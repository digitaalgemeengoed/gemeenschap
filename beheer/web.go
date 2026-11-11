package main

// De webpagina's: inloggen, sessies, sjablonen en de beveiliging eromheen.

import (
	"crypto/subtle"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	_ "time/tzdata"
)

//go:embed sjablonen/*.html
var sjabloonBestanden embed.FS

//go:embed statisch
var statischeBestanden embed.FS

// Links: de andere onderdelen van het platform, voor de pagina Platform.
type Links struct {
	Site, Aanmelden, Projecten, Kennisbank, Documenten string
}

type App struct {
	KC        *Keycloak
	OIDC      *OIDC
	Sessies   *Sessies
	Logboek   *Logboek
	Basis     string // https://beheer.<domein>
	Geheim    []byte
	StatusPad string // het statusbestand van de server (server/status.sh)
	Links     Links
	rollen    rollenCache
	sjablonen map[string]*template.Template
}

var amsterdam, _ = time.LoadLocation("Europe/Amsterdam")

const cookieSessie = "dg_beheer"

func (a *App) Routes() http.Handler {
	a.laadSjablonen()
	mux := http.NewServeMux()
	statisch, _ := fs.Sub(statischeBestanden, "statisch")
	mux.Handle("GET /statisch/", http.StripPrefix("/statisch/", http.FileServerFS(statisch)))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok\n") })
	mux.HandleFunc("GET /inloggen", a.inloggen)
	mux.HandleFunc("GET /oidc/terug", a.terugVanKeycloak)
	mux.HandleFunc("POST /uitloggen", a.metSessie(a.uitloggen))

	mux.HandleFunc("GET /{$}", a.metToegang(a.start))
	mux.HandleFunc("GET /leden", a.metToegang(a.leden))
	mux.HandleFunc("GET /leden/{id}", a.metToegang(a.lid))
	mux.HandleFunc("POST /leden/{id}/groepen", a.metToegang(a.lidGroepen))
	mux.HandleFunc("POST /leden/{id}/actief", a.metToegang(a.lidActief))
	mux.HandleFunc("POST /leden/{id}/uitloggen", a.metToegang(a.lidUitloggen))
	mux.HandleFunc("GET /organisaties", a.metToegang(a.organisaties))
	mux.HandleFunc("POST /organisaties", a.metToegang(a.nieuweOrganisatie))
	mux.HandleFunc("GET /organisaties/{id}", a.metToegang(a.organisatie))
	mux.HandleFunc("POST /organisaties/{id}", a.metToegang(a.wijzigOrganisatie))
	mux.HandleFunc("POST /organisaties/{id}/leden", a.metToegang(a.organisatieLeden))
	mux.HandleFunc("GET /platform", a.metToegang(a.platform))
	mux.HandleFunc("GET /logboek", a.metToegang(a.logboek))
	return veilig(mux, a.OIDC.Uitgever)
}

func veilig(h http.Handler, uitgever string) http.Handler {
	keycloak := uitgever
	if u, err := url.Parse(uitgever); err == nil {
		keycloak = u.Scheme + "://" + u.Host
	}
	csp := "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; frame-ancestors 'none'; form-action 'self' " + keycloak + "; base-uri 'none'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k := w.Header()
		k.Set("Content-Security-Policy", csp)
		k.Set("X-Content-Type-Options", "nosniff")
		k.Set("Referrer-Policy", "same-origin")
		k.Set("X-Frame-Options", "DENY")
		k.Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

// --- Sjablonen --------------------------------------------------------------

var maanden = []string{"jan", "feb", "mrt", "apr", "mei", "jun", "jul", "aug", "sep", "okt", "nov", "dec"}

func datum(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	t = t.In(amsterdam)
	return fmt.Sprintf("%d %s %d, %02d:%02d", t.Day(), maanden[t.Month()-1], t.Year(), t.Hour(), t.Minute())
}

func dag(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	t = t.In(amsterdam)
	return fmt.Sprintf("%d %s %d", t.Day(), maanden[t.Month()-1], t.Year())
}

func grootte(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d bytes", n)
	case n < 1<<20:
		return fmt.Sprintf("%.0f kB", float64(n)/1024)
	case n < 1<<30:
		return strings.Replace(fmt.Sprintf("%.1f MB", float64(n)/(1<<20)), ".", ",", 1)
	default:
		return strings.Replace(fmt.Sprintf("%.1f GB", float64(n)/(1<<30)), ".", ",", 1)
	}
}

func (a *App) laadSjablonen() {
	functies := template.FuncMap{
		"datum":     datum,
		"dag":       dag,
		"grootte":   grootte,
		"join":      strings.Join,
		"groepNaam": groepNaam,
		"procent": func(a, b int64) int64 {
			if b <= 0 {
				return 0
			}
			return a * 100 / b
		},
	}
	a.sjablonen = map[string]*template.Template{}
	for _, naam := range []string{"start", "leden", "lid", "organisaties", "organisatie", "platform", "logboek", "fout"} {
		a.sjablonen[naam] = template.Must(template.New("").Funcs(functies).ParseFS(sjabloonBestanden, "sjablonen/basis.html", "sjablonen/"+naam+".html"))
	}
}

// groepNaam: een leesbare naam voor de bekende groepen.
func groepNaam(pad string) string {
	switch pad {
	case "/netwerk/gebruikers":
		return "Gebruiker"
	case "/netwerk/contributors":
		return "Contributor"
	case "/netwerk/members":
		return "Member"
	case "/netwerk/kerngroep":
		return "Kerngroep"
	case "/site/redactie":
		return "Redactie van de site"
	case "/site/beheer":
		return "Beheerder van het platform"
	}
	return pad
}

type pagina struct {
	Titel   string
	Hier    string // het onderdeel in het menu
	Sessie  *Sessie
	Melding string
	Fout    string
	Data    any
}

func (a *App) toon(w http.ResponseWriter, r *http.Request, sjabloon string, status int, p pagina) {
	if p.Melding == "" {
		p.Melding = r.URL.Query().Get("melding")
	}
	if p.Fout == "" {
		p.Fout = r.URL.Query().Get("fout")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := a.sjablonen[sjabloon].ExecuteTemplate(w, "basis", p); err != nil {
		log.Printf("sjabloon %s: %v", sjabloon, err)
	}
}

func (a *App) fout(w http.ResponseWriter, r *http.Request, s *Sessie, status int, tekst string) {
	a.toon(w, r, "fout", status, pagina{Titel: foutTitel(status), Sessie: s, Data: tekst})
}

func foutTitel(status int) string {
	switch status {
	case http.StatusForbidden:
		return "Geen toegang"
	case http.StatusNotFound:
		return "Niet gevonden"
	case http.StatusUnauthorized:
		return "Niet ingelogd"
	case http.StatusBadRequest:
		return "Dat ging niet goed"
	default:
		return "Er ging iets mis"
	}
}

func (a *App) interneFout(w http.ResponseWriter, r *http.Request, s *Sessie, err error) {
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	a.fout(w, r, s, http.StatusInternalServerError, "Er ging iets mis. Probeer het opnieuw; blijft het misgaan, kijk dan in de logboeken van de server.")
}

// terug: na een handeling terug naar de pagina, met een melding of fout.
func terug(w http.ResponseWriter, r *http.Request, doel, soort, tekst string) {
	if tekst != "" {
		sep := "?"
		if strings.Contains(doel, "?") {
			sep = "&"
		}
		doel += sep + soort + "=" + url.QueryEscape(tekst)
	}
	http.Redirect(w, r, doel, http.StatusSeeOther)
}

// --- Inloggen ---------------------------------------------------------------

func (a *App) veilig() bool { return strings.HasPrefix(a.Basis, "https://") }

func (a *App) cookieNaam(n string) string {
	if a.veilig() {
		return "__Host-" + n
	}
	return n
}

func (a *App) zetCookie(w http.ResponseWriter, naam, waarde string, duur time.Duration) {
	c := &http.Cookie{Name: a.cookieNaam(naam), Value: waarde, Path: "/", HttpOnly: true, Secure: a.veilig(), SameSite: http.SameSiteLaxMode}
	if duur < 0 {
		c.MaxAge = -1
	} else {
		c.MaxAge = int(duur.Seconds())
	}
	http.SetCookie(w, c)
}

func veiligTerug(t string) string {
	if !strings.HasPrefix(t, "/") || strings.HasPrefix(t, "//") || strings.Contains(t, "\\") {
		return "/"
	}
	return t
}

func (a *App) inloggen(w http.ResponseWriter, r *http.Request) {
	doel, p := a.OIDC.Begin(veiligTerug(r.URL.Query().Get("terug")))
	data, _ := jsonMarshal(p)
	a.zetCookie(w, "dg_beheer_poging", onderteken(a.Geheim, data), 30*time.Minute)
	http.Redirect(w, r, doel, http.StatusFound)
}

func (a *App) terugVanKeycloak(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	c, err := r.Cookie(a.cookieNaam("dg_beheer_poging"))
	var p Poging
	ok := err == nil
	if ok {
		var data []byte
		data, ok = controleer(a.Geheim, c.Value)
		ok = ok && jsonUnmarshal(data, &p) == nil && p.Tot > time.Now().Unix() &&
			subtle.ConstantTimeCompare([]byte(p.State), []byte(q.Get("state"))) == 1
	}
	if !ok {
		http.Redirect(w, r, "/inloggen", http.StatusFound)
		return
	}
	a.zetCookie(w, "dg_beheer_poging", "", -1)
	if e := q.Get("error"); e != "" {
		a.fout(w, r, nil, http.StatusForbidden, "Inloggen is niet gelukt ("+e+").")
		return
	}
	persoon, err := a.OIDC.Wissel(r.Context(), q.Get("code"), p)
	if err != nil {
		log.Printf("inloggen: %v", err)
		a.fout(w, r, nil, http.StatusBadGateway, "Inloggen is niet gelukt. Probeer het opnieuw.")
		return
	}
	a.vergeetRol(persoon.Sub)
	cookie := a.Sessies.Nieuw(persoon)
	a.zetCookie(w, cookieSessie, cookie, sessieDuur)
	log.Printf("ingelogd: %s", persoon.Email)
	http.Redirect(w, r, p.Terug, http.StatusSeeOther)
}

func (a *App) uitloggen(w http.ResponseWriter, r *http.Request, s *Sessie) {
	if !a.csrfGoed(r, s) {
		a.fout(w, r, s, http.StatusForbidden, "Verlopen formulier. Probeer het opnieuw.")
		return
	}
	if c, err := r.Cookie(a.cookieNaam(cookieSessie)); err == nil {
		a.Sessies.Weg(c.Value)
	}
	a.zetCookie(w, cookieSessie, "", -1)
	q := url.Values{"id_token_hint": {s.IDToken}, "post_logout_redirect_uri": {a.Links.Site + "/"}}
	http.Redirect(w, r, a.OIDC.uitlogURL()+"?"+q.Encode(), http.StatusSeeOther)
}

func (a *App) csrfGoed(r *http.Request, s *Sessie) bool {
	return subtle.ConstantTimeCompare([]byte(r.FormValue("csrf")), []byte(s.CSRF)) == 1
}

func (a *App) metSessie(h func(http.ResponseWriter, *http.Request, *Sessie)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var s *Sessie
		if c, err := r.Cookie(a.cookieNaam(cookieSessie)); err == nil {
			s = a.Sessies.Zoek(c.Value)
		}
		if s == nil {
			if r.Method == http.MethodGet {
				http.Redirect(w, r, "/inloggen?terug="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
			} else {
				a.fout(w, r, nil, http.StatusUnauthorized, "Je bent niet (meer) ingelogd. Log opnieuw in en probeer het nog eens.")
			}
			return
		}
		h(w, r, s)
	}
}

// metToegang: alleen voor beheerders en de kerngroep, telkens opnieuw
// nagekeken in Keycloak (hooguit een minuut oud). Bij een POST ook de
// CSRF-sleutel.
func (a *App) metToegang(h func(http.ResponseWriter, *http.Request, *Sessie)) http.HandlerFunc {
	return a.metSessie(func(w http.ResponseWriter, r *http.Request, s *Sessie) {
		rol, err := a.rolVan(r.Context(), s.Sub)
		if err != nil {
			a.interneFout(w, r, s, err)
			return
		}
		if !rol.Toegang() {
			a.fout(w, r, s, http.StatusForbidden, "Beheer is er voor de beheerders van het platform en de kerngroep van de coöperatie. Hoor je daarbij en zie je dit toch, log dan uit en weer in.")
			return
		}
		s.Rol = rol
		if r.Method == http.MethodPost && !a.csrfGoed(r, s) {
			a.fout(w, r, s, http.StatusForbidden, "Verlopen formulier. Ga terug, laad de pagina opnieuw en probeer het nog eens.")
			return
		}
		h(w, r, s)
	})
}

// reden: de verplichte reden bij een wijziging.
func reden(r *http.Request) (string, bool) {
	t := strings.Join(strings.Fields(r.FormValue("reden")), " ")
	if len([]rune(t)) > 500 {
		t = string([]rune(t)[:500])
	}
	return t, len([]rune(t)) >= 5
}

const zonderReden = "Geef een reden (minstens een paar woorden); die komt in het logboek."

func (a *App) noteer(s *Sessie, actie, onderwerp, details, reden string) {
	if err := a.Logboek.Schrijf(Regel{Wie: s.Email, Naam: s.Naam, Actie: actie, Onderwerp: onderwerp, Details: details, Reden: reden}); err != nil {
		log.Printf("LOGBOEK SCHRIJVEN MISLUKT (%s %s %s): %v", s.Email, actie, onderwerp, err)
	}
	log.Printf("beheer: %s %s %s %s (%s)", s.Email, actie, onderwerp, details, reden)
}

// --- Start en logboek ----------------------------------------------------------

func (a *App) start(w http.ResponseWriter, r *http.Request, s *Sessie) {
	recent, err := a.Logboek.Laatste(8, "")
	if err != nil {
		log.Printf("logboek lezen: %v", err)
	}
	st, _ := leesStatus(a.StatusPad)
	a.toon(w, r, "start", http.StatusOK, pagina{Titel: "Beheer", Hier: "start", Sessie: s, Data: map[string]any{"Recent": recent, "Status": st}})
}

func (a *App) logboek(w http.ResponseWriter, r *http.Request, s *Sessie) {
	filter := strings.TrimSpace(r.URL.Query().Get("zoek"))
	regels, err := a.Logboek.Laatste(300, filter)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	a.toon(w, r, "logboek", http.StatusOK, pagina{Titel: "Logboek", Hier: "logboek", Sessie: s, Data: map[string]any{"Regels": regels, "Zoek": filter}})
}
