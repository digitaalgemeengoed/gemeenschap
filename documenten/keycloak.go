package main

// Rechtstreeks uit Keycloak, met de client redmine-sync (die gebruikers en
// groepen mag lezen): wie in de kerngroep zit (voor de persoonlijke ruimtes),
// en de groepen van wie Documenten gebruikt (zie groepen.go). In het token
// zit dat ook, maar alleen van het moment van inloggen.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type KeycloakBeheer struct {
	Realm    string // http://keycloak:8080/realms/gemeenschap
	ClientID string
	Geheim   string
	HTTP     *http.Client

	mu     sync.Mutex
	tok    string
	tokTot time.Time
}

func (k *KeycloakBeheer) haal(ctx context.Context, token, pad string, doel any) error {
	beheer := strings.Replace(k.Realm, "/realms/", "/admin/realms/", 1)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, beheer+pad, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := k.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("keycloak %s: %s", pad, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(doel)
}

// token: een toegangstoken van het service account, bewaard tot vlak voor
// het verloopt (de groepen worden bij elk verzoek nagekeken).
func (k *KeycloakBeheer) token(ctx context.Context) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.tok != "" && time.Now().Before(k.tokTot) {
		return k.tok, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {k.ClientID}, "client_secret": {k.Geheim}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, k.Realm+"/protocol/openid-connect/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := k.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var tok struct {
		Token string `json:"access_token"`
		Duur  int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil || tok.Token == "" {
		return "", fmt.Errorf("keycloak token: %s", resp.Status)
	}
	k.tok, k.tokTot = tok.Token, time.Now().Add(time.Duration(max(tok.Duur-30, 10))*time.Second)
	return k.tok, nil
}

// GroepenVan: de groepen (paden) van wie dit e-mailadres heeft, en of het
// account actief is. Een account dat niet (meer) bestaat, is niet actief.
func (k *KeycloakBeheer) GroepenVan(ctx context.Context, email string) (groepen []string, actief bool, err error) {
	token, err := k.token(ctx)
	if err != nil {
		return nil, false, err
	}
	var gebruikers []struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Enabled bool   `json:"enabled"`
	}
	if err := k.haal(ctx, token, "/users?exact=true&briefRepresentation=true&email="+url.QueryEscape(email), &gebruikers); err != nil {
		return nil, false, err
	}
	var id string
	for _, g := range gebruikers {
		if strings.EqualFold(g.Email, email) {
			id, actief = g.ID, g.Enabled
		}
	}
	if id == "" || !actief {
		return nil, false, nil
	}
	var lijst []struct {
		Pad string `json:"path"`
	}
	if err := k.haal(ctx, token, "/users/"+url.PathEscape(id)+"/groups?briefRepresentation=true&max=1000", &lijst); err != nil {
		return nil, false, err
	}
	groepen = []string{}
	for _, g := range lijst {
		groepen = append(groepen, g.Pad)
	}
	return groepen, true, nil
}

// Leden: de e-mailadressen (klein) van de leden van een groep (pad).
func (k *KeycloakBeheer) Leden(ctx context.Context, pad string) ([]string, error) {
	token, err := k.token(ctx)
	if err != nil {
		return nil, err
	}
	var groep struct {
		ID string `json:"id"`
	}
	if err := k.haal(ctx, token, "/group-by-path"+pad, &groep); err != nil {
		return nil, err
	}
	var uit []string
	for eerste := 0; ; eerste += 100 {
		var deel []struct {
			Email   string `json:"email"`
			Enabled bool   `json:"enabled"`
		}
		if err := k.haal(ctx, token, fmt.Sprintf("/groups/%s/members?first=%d&max=100&briefRepresentation=true", groep.ID, eerste), &deel); err != nil {
			return nil, err
		}
		for _, u := range deel {
			if u.Email != "" && u.Enabled {
				uit = append(uit, strings.ToLower(u.Email))
			}
		}
		if len(deel) < 100 {
			return uit, nil
		}
	}
}

// SynchroniseerKerngroep: de kerngroep en de beheerders van het platform
// (samen: wie altijd recht heeft op een persoonlijke ruimte).
func (k *KeycloakBeheer) SynchroniseerKerngroep(ctx context.Context, reg *Register) error {
	var alle []string
	for _, pad := range []string{groepKerngroep, groepBeheer} {
		leden, err := k.Leden(ctx, pad)
		if err != nil {
			return err
		}
		alle = append(alle, leden...)
	}
	return reg.ZetKerngroep(ctx, alle)
}
