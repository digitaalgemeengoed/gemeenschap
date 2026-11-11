package main

// Wie in de kerngroep zit, rechtstreeks uit Keycloak (met de client
// redmine-sync, die de leden van groepen mag lezen). Nodig voor de
// persoonlijke ruimtes: in het token zit het ook, maar alleen bij inloggen.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type KeycloakBeheer struct {
	Realm    string // http://keycloak:8080/realms/gemeenschap
	ClientID string
	Geheim   string
	HTTP     *http.Client
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

// Leden: de e-mailadressen (klein) van de leden van een groep (pad).
func (k *KeycloakBeheer) Leden(ctx context.Context, pad string) ([]string, error) {
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {k.ClientID}, "client_secret": {k.Geheim}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, k.Realm+"/protocol/openid-connect/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := k.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	var tok struct {
		Token string `json:"access_token"`
	}
	err = json.NewDecoder(resp.Body).Decode(&tok)
	resp.Body.Close()
	if err != nil || tok.Token == "" {
		return nil, fmt.Errorf("keycloak token: %s", resp.Status)
	}
	var groep struct {
		ID string `json:"id"`
	}
	if err := k.haal(ctx, tok.Token, "/group-by-path"+pad, &groep); err != nil {
		return nil, err
	}
	var uit []string
	for eerste := 0; ; eerste += 100 {
		var deel []struct {
			Email   string `json:"email"`
			Enabled bool   `json:"enabled"`
		}
		if err := k.haal(ctx, tok.Token, fmt.Sprintf("/groups/%s/members?first=%d&max=100&briefRepresentation=true", groep.ID, eerste), &deel); err != nil {
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

func (k *KeycloakBeheer) SynchroniseerKerngroep(ctx context.Context, reg *Register) error {
	leden, err := k.Leden(ctx, groepKerngroep)
	if err != nil {
		return err
	}
	return reg.ZetKerngroep(ctx, leden)
}
