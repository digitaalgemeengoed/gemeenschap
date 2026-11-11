package main

// Inloggen bij Keycloak (OpenID Connect, code flow met PKCE). De browser gaat
// naar het publieke adres; tokens en sleutels haalt de dienst rechtstreeks
// bij Keycloak, net als oauth2-proxy.

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type OIDC struct {
	Uitgever     string // https://aanmelden.<domein>/realms/gemeenschap
	Intern       string // http://keycloak:8080/realms/gemeenschap
	ClientID     string
	ClientGeheim string
	TerugURL     string // https://beheer.<domein>/oidc/terug
	HTTP         *http.Client
	mu           sync.Mutex
	sleutels     map[string]*rsa.PublicKey
	sleutelsTijd time.Time
}

func (o *OIDC) authURL() string  { return o.Uitgever + "/protocol/openid-connect/auth" }
func (o *OIDC) tokenURL() string { return o.Intern + "/protocol/openid-connect/token" }
func (o *OIDC) jwksURL() string  { return o.Intern + "/protocol/openid-connect/certs" }
func (o *OIDC) uitlogURL() string {
	return o.Uitgever + "/protocol/openid-connect/logout"
}

func willekeurigeBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func willekeurig(n int) string { return base64.RawURLEncoding.EncodeToString(willekeurigeBytes(n)) }

func jsonMarshal(v any) ([]byte, error)   { return json.Marshal(v) }
func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

// Poging: wat tussen de doorverwijzing naar Keycloak en de terugkeer bewaard
// blijft, in een ondertekende cookie.
type Poging struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Terug    string `json:"t"`
	Tot      int64  `json:"e"`
}

func onderteken(geheim []byte, data []byte) string {
	h := hmac.New(sha256.New, geheim)
	h.Write(data)
	return base64.RawURLEncoding.EncodeToString(data) + "." + base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func controleer(geheim []byte, waarde string) ([]byte, bool) {
	d, s, ok := strings.Cut(waarde, ".")
	if !ok {
		return nil, false
	}
	data, err1 := base64.RawURLEncoding.DecodeString(d)
	sig, err2 := base64.RawURLEncoding.DecodeString(s)
	if err1 != nil || err2 != nil {
		return nil, false
	}
	h := hmac.New(sha256.New, geheim)
	h.Write(data)
	return data, hmac.Equal(sig, h.Sum(nil))
}

// Begin: de URL naar Keycloak en de cookie met de poging.
func (o *OIDC) Begin(terug string) (string, Poging) {
	p := Poging{State: willekeurig(24), Nonce: willekeurig(24), Verifier: willekeurig(48), Terug: terug, Tot: time.Now().Add(30 * time.Minute).Unix()}
	h := sha256.Sum256([]byte(p.Verifier))
	q := url.Values{
		"client_id":             {o.ClientID},
		"response_type":         {"code"},
		"scope":                 {"openid email profile"},
		"redirect_uri":          {o.TerugURL},
		"state":                 {p.State},
		"nonce":                 {p.Nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(h[:])},
		"code_challenge_method": {"S256"},
	}
	return o.authURL() + "?" + q.Encode(), p
}

type Persoon struct {
	Sub     string
	Email   string
	Naam    string
	Groepen []string
	IDToken string
}

// Wissel: de code inruilen voor tokens en het ID-token controleren.
func (o *OIDC) Wissel(ctx context.Context, code string, p Poging) (*Persoon, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {o.TerugURL},
		"code_verifier": {p.Verifier},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, o.tokenURL(), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(url.QueryEscape(o.ClientID), url.QueryEscape(o.ClientGeheim))
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.IDToken == "" {
		return nil, errors.New("token: geen id_token")
	}
	claims, err := o.verifieer(ctx, tok.IDToken)
	if err != nil {
		return nil, err
	}
	if claims.Nonce != p.Nonce {
		return nil, errors.New("id_token: nonce klopt niet")
	}
	if claims.Sub == "" || claims.Email == "" {
		return nil, errors.New("id_token: geen e-mailadres")
	}
	naam := claims.Naam
	if naam == "" {
		naam = claims.Email
	}
	return &Persoon{Sub: claims.Sub, Email: strings.ToLower(claims.Email), Naam: naam, Groepen: claims.Groepen, IDToken: tok.IDToken}, nil
}

type idClaims struct {
	Iss     string          `json:"iss"`
	Sub     string          `json:"sub"`
	Aud     json.RawMessage `json:"aud"`
	Exp     int64           `json:"exp"`
	Nonce   string          `json:"nonce"`
	Email   string          `json:"email"`
	Naam    string          `json:"name"`
	Groepen []string        `json:"groups"`
}

func (o *OIDC) verifieer(ctx context.Context, jwt string) (*idClaims, error) {
	delen := strings.Split(jwt, ".")
	if len(delen) != 3 {
		return nil, errors.New("id_token: geen JWT")
	}
	kopB, err := base64.RawURLEncoding.DecodeString(delen[0])
	if err != nil {
		return nil, err
	}
	var kop struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(kopB, &kop); err != nil {
		return nil, err
	}
	if kop.Alg != "RS256" {
		return nil, fmt.Errorf("id_token: algoritme %q niet toegestaan", kop.Alg)
	}
	sleutel, err := o.sleutel(ctx, kop.Kid)
	if err != nil {
		return nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(delen[2])
	if err != nil {
		return nil, err
	}
	h := sha256.Sum256([]byte(delen[0] + "." + delen[1]))
	if err := rsa.VerifyPKCS1v15(sleutel, crypto.SHA256, h[:], sig); err != nil {
		return nil, errors.New("id_token: handtekening klopt niet")
	}
	inhoud, err := base64.RawURLEncoding.DecodeString(delen[1])
	if err != nil {
		return nil, err
	}
	var c idClaims
	if err := json.Unmarshal(inhoud, &c); err != nil {
		return nil, err
	}
	if c.Iss != o.Uitgever {
		return nil, fmt.Errorf("id_token: uitgever %q, verwacht %q", c.Iss, o.Uitgever)
	}
	var auds []string
	if err := json.Unmarshal(c.Aud, &auds); err != nil {
		var een string
		if json.Unmarshal(c.Aud, &een) == nil {
			auds = []string{een}
		}
	}
	bedoeld := false
	for _, a := range auds {
		bedoeld = bedoeld || a == o.ClientID
	}
	if !bedoeld {
		return nil, errors.New("id_token: niet voor deze client")
	}
	if time.Now().Unix() > c.Exp+60 {
		return nil, errors.New("id_token: verlopen")
	}
	return &c, nil
}

func (o *OIDC) sleutel(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if k := o.sleutels[kid]; k != nil {
		return k, nil
	}
	// Onbekende sleutel: opnieuw ophalen (Keycloak kan sleutels wisselen), hooguit eens per 10 seconden.
	if time.Since(o.sleutelsTijd) < 10*time.Second {
		return nil, errors.New("id_token: onbekende sleutel")
	}
	o.sleutelsTijd = time.Now()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, o.jwksURL(), nil)
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var set struct {
		Keys []struct {
			Kid, Kty, Use, N, E string
		} `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&set); err != nil {
		return nil, err
	}
	o.sleutels = map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" || (k.Use != "" && k.Use != "sig") {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil {
			continue
		}
		o.sleutels[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	if k := o.sleutels[kid]; k != nil {
		return k, nil
	}
	return nil, errors.New("id_token: onbekende sleutel")
}
