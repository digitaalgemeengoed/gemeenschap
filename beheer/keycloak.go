package main

// De Admin API van Keycloak, met het service account van de client
// beheer-keycloak. Alleen de aanroepen hieronder; de dienst kan meer
// (manage-users, manage-realm), maar doet niet meer.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type Keycloak struct {
	Realm    string // http://keycloak:8080/realms/gemeenschap
	ClientID string
	Geheim   string
	HTTP     *http.Client

	mu       sync.Mutex
	token    string
	tokenTot time.Time
}

var errNietGevonden = errors.New("niet gevonden")

// KeycloakFout: Keycloak weigerde of gaf een fout; Bericht is wat Keycloak zei.
type KeycloakFout struct {
	Status  int
	Bericht string
}

func (f *KeycloakFout) Error() string { return fmt.Sprintf("keycloak %d: %s", f.Status, f.Bericht) }

func (k *Keycloak) beheerURL() string {
	return strings.Replace(k.Realm, "/realms/", "/admin/realms/", 1)
}

func (k *Keycloak) haalToken(ctx context.Context) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.token != "" && time.Now().Before(k.tokenTot) {
		return k.token, nil
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
		Token  string `json:"access_token"`
		Duur   int    `json:"expires_in"`
		Fout   string `json:"error"`
		Uitleg string `json:"error_description"`
	}
	json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&tok)
	if resp.StatusCode != http.StatusOK || tok.Token == "" {
		return "", fmt.Errorf("keycloak token: %s %s %s", resp.Status, tok.Fout, tok.Uitleg)
	}
	k.token = tok.Token
	k.tokenTot = time.Now().Add(time.Duration(max(tok.Duur-30, 10)) * time.Second)
	return k.token, nil
}

func (k *Keycloak) doe(ctx context.Context, methode, pad string, in, uit any) error {
	token, err := k.haalToken(ctx)
	if err != nil {
		return err
	}
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, methode, k.beheerURL()+pad, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := k.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return errNietGevonden
	case resp.StatusCode >= 300:
		var f struct {
			Error            string `json:"error"`
			ErrorMessage     string `json:"errorMessage"`
			ErrorDescription string `json:"error_description"`
		}
		json.Unmarshal(b, &f)
		bericht := strings.TrimSpace(f.ErrorMessage + " " + f.ErrorDescription)
		if bericht == "" {
			bericht = f.Error
		}
		if bericht == "" {
			bericht = strings.TrimSpace(string(b))
		}
		return &KeycloakFout{Status: resp.StatusCode, Bericht: bericht}
	}
	if uit != nil && len(b) > 0 {
		return json.Unmarshal(b, uit)
	}
	return nil
}

func pe(s string) string { return url.PathEscape(s) }

// --- Leden ----------------------------------------------------------------------

type Lid struct {
	ID             string              `json:"id"`
	Gebruikersnaam string              `json:"username"`
	Email          string              `json:"email"`
	Voornaam       string              `json:"firstName"`
	Achternaam     string              `json:"lastName"`
	Actief         bool                `json:"enabled"`
	EmailBevestigd bool                `json:"emailVerified"`
	Aangemaakt     int64               `json:"createdTimestamp"`
	Kenmerken      map[string][]string `json:"attributes"`
}

func (l Lid) Naam() string {
	n := strings.TrimSpace(l.Voornaam + " " + l.Achternaam)
	if n == "" {
		n = l.Email
	}
	if n == "" {
		n = l.Gebruikersnaam
	}
	return n
}

func (l Lid) Kenmerk(naam string) string { return strings.Join(l.Kenmerken[naam], ", ") }

func (l Lid) AangemaaktOp() time.Time { return time.UnixMilli(l.Aangemaakt) }

func (k *Keycloak) ZoekLeden(ctx context.Context, zoek string, eerste, max int) ([]Lid, error) {
	q := url.Values{"first": {fmt.Sprint(eerste)}, "max": {fmt.Sprint(max)}, "briefRepresentation": {"false"}}
	if zoek != "" {
		q.Set("search", zoek)
	}
	var uit []Lid
	return uit, k.doe(ctx, http.MethodGet, "/users?"+q.Encode(), nil, &uit)
}

func (k *Keycloak) AantalLeden(ctx context.Context, zoek string) (int, error) {
	q := url.Values{}
	if zoek != "" {
		q.Set("search", zoek)
	}
	var n int
	return n, k.doe(ctx, http.MethodGet, "/users/count?"+q.Encode(), nil, &n)
}

func (k *Keycloak) Lid(ctx context.Context, id string) (*Lid, error) {
	var l Lid
	if err := k.doe(ctx, http.MethodGet, "/users/"+pe(id), nil, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

func (k *Keycloak) LidMetEmail(ctx context.Context, email string) (*Lid, error) {
	var uit []Lid
	if err := k.doe(ctx, http.MethodGet, "/users?exact=true&email="+url.QueryEscape(email), nil, &uit); err != nil {
		return nil, err
	}
	if len(uit) == 0 {
		return nil, errNietGevonden
	}
	return &uit[0], nil
}

// ZetActief: een account aan- of uitzetten. Alleen dit veld; de rest van het
// account blijft zoals het is.
func (k *Keycloak) ZetActief(ctx context.Context, id string, actief bool) error {
	var vol map[string]any
	if err := k.doe(ctx, http.MethodGet, "/users/"+pe(id), nil, &vol); err != nil {
		return err
	}
	vol["enabled"] = actief
	return k.doe(ctx, http.MethodPut, "/users/"+pe(id), vol, nil)
}

func (k *Keycloak) Sessies(ctx context.Context, id string) (int, error) {
	var uit []json.RawMessage
	err := k.doe(ctx, http.MethodGet, "/users/"+pe(id)+"/sessions", nil, &uit)
	return len(uit), err
}

func (k *Keycloak) LogUit(ctx context.Context, id string) error {
	return k.doe(ctx, http.MethodPost, "/users/"+pe(id)+"/logout", nil, nil)
}

// --- Groepen ------------------------------------------------------------------

type Groep struct {
	ID   string `json:"id"`
	Naam string `json:"name"`
	Pad  string `json:"path"`
}

// Groepen: alle groepen (met subgroepen), op pad gesorteerd.
func (k *Keycloak) Groepen(ctx context.Context) ([]Groep, error) {
	type knoop struct {
		Groep
		Aantal int     `json:"subGroupCount"`
		Sub    []knoop `json:"subGroups"`
	}
	var top []knoop
	if err := k.doe(ctx, http.MethodGet, "/groups?briefRepresentation=true&max=200", nil, &top); err != nil {
		return nil, err
	}
	var uit []Groep
	var loop func(ouder string, lijst []knoop) error
	loop = func(ouder string, lijst []knoop) error {
		for _, g := range lijst {
			uit = append(uit, g.Groep)
			sub := g.Sub
			if len(sub) == 0 && g.Aantal > 0 {
				if err := k.doe(ctx, http.MethodGet, "/groups/"+pe(g.ID)+"/children?briefRepresentation=true&max=200", nil, &sub); err != nil {
					return err
				}
			}
			if err := loop(g.Pad, sub); err != nil {
				return err
			}
		}
		return nil
	}
	if err := loop("", top); err != nil {
		return nil, err
	}
	sort.Slice(uit, func(i, j int) bool { return uit[i].Pad < uit[j].Pad })
	return uit, nil
}

func (k *Keycloak) GroepMetPad(ctx context.Context, pad string) (*Groep, error) {
	var g Groep
	if err := k.doe(ctx, http.MethodGet, "/group-by-path"+pad, nil, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

// GroepenVan: de paden van de groepen van een lid.
func (k *Keycloak) GroepenVan(ctx context.Context, id string) ([]string, error) {
	var gs []Groep
	if err := k.doe(ctx, http.MethodGet, "/users/"+pe(id)+"/groups?briefRepresentation=true&max=500", nil, &gs); err != nil {
		return nil, err
	}
	uit := make([]string, 0, len(gs))
	for _, g := range gs {
		uit = append(uit, g.Pad)
	}
	sort.Strings(uit)
	return uit, nil
}

func (k *Keycloak) GroepLeden(ctx context.Context, groepID string, eerste, max int) ([]Lid, error) {
	var uit []Lid
	return uit, k.doe(ctx, http.MethodGet, fmt.Sprintf("/groups/%s/members?first=%d&max=%d&briefRepresentation=false", pe(groepID), eerste, max), nil, &uit)
}

// ActieveLeden: hoeveel actieve accounts in een groep zitten.
func (k *Keycloak) ActieveLeden(ctx context.Context, groepID string) (int, error) {
	n := 0
	for eerste := 0; ; eerste += 200 {
		deel, err := k.GroepLeden(ctx, groepID, eerste, 200)
		if err != nil {
			return 0, err
		}
		for _, l := range deel {
			if l.Actief {
				n++
			}
		}
		if len(deel) < 200 {
			return n, nil
		}
	}
}

func (k *Keycloak) InGroep(ctx context.Context, lidID, groepID string, erin bool) error {
	m := http.MethodPut
	if !erin {
		m = http.MethodDelete
	}
	return k.doe(ctx, m, "/users/"+pe(lidID)+"/groups/"+pe(groepID), nil, nil)
}

// --- Organisaties (Keycloak Organizations) -----------------------------------------

type Domein struct {
	Naam         string `json:"name"`
	Geverifieerd bool   `json:"verified"`
}

type Organisatie struct {
	ID           string              `json:"id,omitempty"`
	Naam         string              `json:"name"`
	Alias        string              `json:"alias,omitempty"`
	Actief       bool                `json:"enabled"`
	Beschrijving string              `json:"description,omitempty"`
	Domeinen     []Domein            `json:"domains"`
	Kenmerken    map[string][]string `json:"attributes,omitempty"`
	Leden        int                 `json:"-"`
}

func (o Organisatie) DomeinNamen() []string {
	uit := make([]string, 0, len(o.Domeinen))
	for _, d := range o.Domeinen {
		uit = append(uit, d.Naam)
	}
	return uit
}

func (k *Keycloak) Organisaties(ctx context.Context) ([]Organisatie, error) {
	var uit []Organisatie
	if err := k.doe(ctx, http.MethodGet, "/organizations?briefRepresentation=false&max=500", nil, &uit); err != nil {
		return nil, err
	}
	sort.Slice(uit, func(i, j int) bool { return strings.ToLower(uit[i].Naam) < strings.ToLower(uit[j].Naam) })
	return uit, nil
}

func (k *Keycloak) Organisatie(ctx context.Context, id string) (*Organisatie, error) {
	var o Organisatie
	if err := k.doe(ctx, http.MethodGet, "/organizations/"+pe(id), nil, &o); err != nil {
		return nil, err
	}
	return &o, nil
}

func (k *Keycloak) NieuweOrganisatie(ctx context.Context, o Organisatie) error {
	return k.doe(ctx, http.MethodPost, "/organizations", o, nil)
}

func (k *Keycloak) WijzigOrganisatie(ctx context.Context, o Organisatie) error {
	return k.doe(ctx, http.MethodPut, "/organizations/"+pe(o.ID), o, nil)
}

func (k *Keycloak) OrganisatieLeden(ctx context.Context, id string) ([]Lid, error) {
	var uit []Lid
	return uit, k.doe(ctx, http.MethodGet, "/organizations/"+pe(id)+"/members?max=500&briefRepresentation=false", nil, &uit)
}

func (k *Keycloak) AantalOrganisatieLeden(ctx context.Context, id string) (int, error) {
	var n int
	return n, k.doe(ctx, http.MethodGet, "/organizations/"+pe(id)+"/members/count", nil, &n)
}

func (k *Keycloak) OrganisatieLid(ctx context.Context, orgID, lidID string, erin bool) error {
	if erin {
		return k.doe(ctx, http.MethodPost, "/organizations/"+pe(orgID)+"/members", lidID, nil)
	}
	return k.doe(ctx, http.MethodDelete, "/organizations/"+pe(orgID)+"/members/"+pe(lidID), nil, nil)
}

// OrganisatiesVan: de organisaties waar een lid bij hoort.
func (k *Keycloak) OrganisatiesVan(ctx context.Context, lidID string) ([]Organisatie, error) {
	var uit []Organisatie
	return uit, k.doe(ctx, http.MethodGet, "/organizations/members/"+pe(lidID)+"/organizations?briefRepresentation=true", nil, &uit)
}
