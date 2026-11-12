package main

// De bestandsbeheerder in de browser.

import (
	"crypto/subtle"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	_ "time/tzdata"
)

//go:embed sjablonen/*.html
var sjabloonBestanden embed.FS

//go:embed statisch
var statischeBestanden embed.FS

type App struct {
	Reg               *Register
	Opslag            *S3
	OIDC              *OIDC
	Basis             string
	Geheim            []byte
	MaxGrootte        int64
	Projecten         string     // publieke URL van Redmine, voor links
	Berichten         *WordPress // nil: geen berichten bij delen en geen werkgroepen
	QuotumPersoonlijk int64
	Keycloak          *KeycloakBeheer // de kerngroep, en de groepen van wie inlogt (groepen.go)
	groepen           groepenCache
	Zoeken            *Zoeker // nil: zoeken staat uit (zoeken.go)
	sjablonen         map[string]*template.Template
}

const (
	groepBeheer    = "/site/beheer"
	groepKerngroep = "/netwerk/kerngroep"
	groepMembers   = "/netwerk/members"
)

// Documentsoorten: een korte keuzelijst.
var documentsoorten = []string{"notitie", "verslag", "presentatie", "ontwerp", "overeenkomst", "overig"}

var amsterdam, _ = time.LoadLocation("Europe/Amsterdam")

func init() {
	for ext, t := range map[string]string{
		".docx":      "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xlsx":      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".pptx":      "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".odt":       "application/vnd.oasis.opendocument.text",
		".ods":       "application/vnd.oasis.opendocument.spreadsheet",
		".odp":       "application/vnd.oasis.opendocument.presentation",
		".md":        "text/markdown; charset=utf-8",
		".archimate": "application/xml",
	} {
		mime.AddExtensionType(ext, t)
	}
}

func (a *App) Routes() http.Handler {
	a.laadSjablonen()
	mux := http.NewServeMux()
	statisch, _ := fs.Sub(statischeBestanden, "statisch")
	mux.Handle("GET /statisch/", http.StripPrefix("/statisch/", http.FileServerFS(statisch)))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := a.Reg.db.PingContext(r.Context()); err != nil {
			http.Error(w, "database", http.StatusServiceUnavailable)
			return
		}
		io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("GET /inloggen", a.inloggen)
	mux.HandleFunc("GET /oidc/terug", a.terugVanKeycloak)
	mux.HandleFunc("POST /uitloggen", a.metSessie(a.uitloggen))

	mux.HandleFunc("GET /{$}", a.metSessie(a.thuis))
	mux.HandleFunc("GET /r/{ruimte}", a.metRuimte(a.toonMap))
	mux.HandleFunc("GET /r/{ruimte}/m/{id}", a.metRuimte(a.toonMap))
	mux.HandleFunc("GET /r/{ruimte}/toegang", a.metRuimte(a.toonMap))
	mux.HandleFunc("GET /r/{ruimte}/info", a.metRuimte(a.toonMap))
	mux.HandleFunc("GET /r/{ruimte}/delen", a.metRuimte(a.toonMap))
	mux.HandleFunc("GET /r/{ruimte}/m/{id}/info", a.metRuimte(a.toonMap))
	mux.HandleFunc("GET /r/{ruimte}/m/{id}/delen", a.metRuimte(a.toonMap))
	mux.HandleFunc("GET /r/{ruimte}/d/{id}/versies", a.metRuimte(a.toonDocument))
	mux.HandleFunc("GET /r/{ruimte}/d/{id}/delen", a.metRuimte(a.toonDocument))
	mux.HandleFunc("POST /r/{ruimte}/plakken", a.metRuimte(a.plakken))
	mux.HandleFunc("GET /r/{ruimte}/m/{id}/toegang", a.metRuimte(a.toonMap))
	mux.HandleFunc("GET /r/{ruimte}/d/{id}", a.metRuimte(a.toonDocument))
	mux.HandleFunc("GET /r/{ruimte}/d/{id}/toegang", a.metRuimte(a.toonDocument))
	mux.HandleFunc("POST /r/{ruimte}/i/{id}/delen", a.metRuimte(a.delen))
	mux.HandleFunc("GET /r/{ruimte}/d/{id}/download", a.metRuimte(a.download))
	mux.HandleFunc("POST /r/{ruimte}/map", a.metRuimte(a.nieuweMap))
	mux.HandleFunc("POST /r/{ruimte}/upload", a.metRuimte(a.upload))
	mux.HandleFunc("POST /r/{ruimte}/d/{id}/metadata", a.metRuimte(a.metadata))
	mux.HandleFunc("POST /r/{ruimte}/i/{id}/hernoem", a.metRuimte(a.hernoem))
	mux.HandleFunc("POST /r/{ruimte}/i/{id}/verwijder", a.metRuimte(a.verwijder))
	mux.HandleFunc("POST /r/{ruimte}/toegang", a.metRuimte(a.toegang))
	mux.HandleFunc("POST /r/{ruimte}/i/{id}/openen", a.metRuimte(a.openen))
	mux.HandleFunc("GET /r/mijn", a.metSessie(a.mijn))
	mux.HandleFunc("GET /beheer", a.metSessie(a.beheer))
	mux.HandleFunc("POST /beheer", a.metSessie(a.beheer))
	mux.HandleFunc("GET /beheer/r/{id}", a.metSessie(a.beheerRuimte))
	mux.HandleFunc("POST /beheer/r/{id}", a.metSessie(a.beheerRuimte))
	mux.HandleFunc("GET /zoeken", a.metSessie(a.zoeken))
	mux.HandleFunc("GET /koppelen", a.metSessie(a.koppelen))
	mux.HandleFunc("POST /koppelen", a.metSessie(a.koppelen))
	a.davRoutes(mux)
	return veilig(mux, a.OIDC.Uitgever)
}

func veilig(h http.Handler, uitgever string) http.Handler {
	// Uitloggen is een formulier dat doorgaat naar Keycloak.
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
		h.ServeHTTP(w, r)
	})
}

// --- Sjablonen --------------------------------------------------------------

func grootte(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d bytes", n)
	case n < 1<<20:
		return strings.Replace(fmt.Sprintf("%.0f kB", float64(n)/1024), ".", ",", 1)
	case n < 1<<30:
		return strings.Replace(fmt.Sprintf("%.1f MB", float64(n)/(1<<20)), ".", ",", 1)
	default:
		return strings.Replace(fmt.Sprintf("%.2f GB", float64(n)/(1<<30)), ".", ",", 1)
	}
}

var maanden = []string{"jan", "feb", "mrt", "apr", "mei", "jun", "jul", "aug", "sep", "okt", "nov", "dec"}

func datum(t time.Time) string {
	t = t.In(amsterdam)
	return fmt.Sprintf("%d %s %d, %02d:%02d", t.Day(), maanden[t.Month()-1], t.Year(), t.Hour(), t.Minute())
}

// soortNaam: de soort van een bestand in gewone woorden.
func soortNaam(mt, naam string) string {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(naam), "."))
	switch ext {
	case "doc", "docx", "docm", "dotx":
		return "Word-document (." + ext + ")"
	case "xls", "xlsx", "xlsm":
		return "Excel-rekenblad (." + ext + ")"
	case "ppt", "pptx", "pptm":
		return "PowerPoint-presentatie (." + ext + ")"
	case "odt":
		return "OpenDocument-tekst (.odt)"
	case "ods":
		return "OpenDocument-rekenblad (.ods)"
	case "odp":
		return "OpenDocument-presentatie (.odp)"
	case "pdf":
		return "Pdf"
	case "md":
		return "Markdown (.md)"
	case "txt":
		return "Tekst (.txt)"
	case "csv":
		return "Tabel (.csv)"
	}
	switch {
	case strings.HasPrefix(mt, "image/"):
		return "Afbeelding (" + strings.TrimPrefix(mt, "image/") + ")"
	case ext != "":
		return "." + ext + "-bestand"
	}
	return mt
}

func (a *App) laadSjablonen() {
	functies := template.FuncMap{
		"grootte": grootte,
		"datum":   datum,
		"soortIcoon": func(mt, naam string) string {
			ext := strings.ToLower(path.Ext(naam))
			switch {
			case strings.HasPrefix(mt, "image/"):
				return "beeld"
			case mt == "application/pdf":
				return "pdf"
			case strings.Contains(mt, "presentation") || ext == ".pptx" || ext == ".odp":
				return "presentatie"
			case strings.Contains(mt, "sheet") || ext == ".xlsx" || ext == ".ods" || ext == ".csv":
				return "rekenblad"
			default:
				return "tekst"
			}
		},
		"join":       strings.Join,
		"bekijkbaar": bekijkbaar,
		"soortNaam":  soortNaam,
		"niveau":     niveauNaam,
		"programmas": programmasVoor,
		"procent": func(a, b int64) int64 {
			if b <= 0 {
				return 0
			}
			return a * 100 / b
		},
		"mb": func(b int64) int64 { return b >> 20 },
		// dict2: twee waarden aan een deelsjabloon meegeven.
		"dict2": func(a, b any) struct{ A, B any } { return struct{ A, B any }{a, b} },
		// eigenTitel: de titel, als die meer zegt dan de bestandsnaam.
		"eigenTitel": func(x *Item) string {
			standaard := strings.TrimSuffix(x.Naam, path.Ext(x.Naam))
			if x.Titel == "" || x.Titel == standaard || x.Titel == x.Naam {
				return ""
			}
			return x.Titel
		},
	}
	a.sjablonen = map[string]*template.Template{}
	for _, naam := range []string{"verkenner", "fout", "koppelen", "openen", "beheer", "beheerruimte", "zoeken"} {
		a.sjablonen[naam] = template.Must(template.New("").Funcs(functies).ParseFS(sjabloonBestanden, "sjablonen/basis.html", "sjablonen/"+naam+".html"))
	}
}

type pagina struct {
	Titel   string
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
	w.Header().Set("Cache-Control", "no-store")
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

func logFout(r *http.Request, err error) { log.Printf("%s %s: %v", r.Method, r.URL.Path, err) }

func (a *App) interneFout(w http.ResponseWriter, r *http.Request, s *Sessie, err error) {
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	a.fout(w, r, s, http.StatusInternalServerError, "Er ging iets mis. Probeer het opnieuw; blijft het misgaan, meld het dan.")
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

// veiligTerug: alleen een pad binnen deze site.
func veiligTerug(t string) string {
	if !strings.HasPrefix(t, "/") || strings.HasPrefix(t, "//") || strings.Contains(t, "\\") {
		return "/"
	}
	return t
}

func (a *App) inloggen(w http.ResponseWriter, r *http.Request) {
	doel, p := a.OIDC.Begin(veiligTerug(r.URL.Query().Get("terug")))
	data, _ := jsonMarshal(p)
	a.zetCookie(w, "dg_poging", onderteken(a.Geheim, data), 30*time.Minute)
	http.Redirect(w, r, doel, http.StatusFound)
}

func (a *App) terugVanKeycloak(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	c, err := r.Cookie(a.cookieNaam("dg_poging"))
	var p Poging
	ok := err == nil
	if ok {
		var data []byte
		data, ok = controleer(a.Geheim, c.Value)
		ok = ok && jsonUnmarshal(data, &p) == nil && p.Tot > time.Now().Unix() &&
			subtle.ConstantTimeCompare([]byte(p.State), []byte(q.Get("state"))) == 1
	}
	if !ok {
		// Verlopen of in een ander tabblad begonnen: gewoon opnieuw.
		http.Redirect(w, r, "/inloggen", http.StatusFound)
		return
	}
	a.zetCookie(w, "dg_poging", "", -1)
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
	cookie, err := a.Reg.NieuweSessie(r.Context(), persoon)
	if err != nil {
		a.interneFout(w, r, nil, err)
		return
	}
	a.zetCookie(w, "dg_documenten", cookie, sessieDuur)
	http.Redirect(w, r, p.Terug, http.StatusSeeOther)
}

func (a *App) uitloggen(w http.ResponseWriter, r *http.Request, s *Sessie) {
	if !a.csrfGoed(r, s) {
		a.fout(w, r, s, http.StatusForbidden, "Verlopen formulier. Probeer het opnieuw.")
		return
	}
	if c, err := r.Cookie(a.cookieNaam("dg_documenten")); err == nil {
		a.Reg.EindSessie(r.Context(), c.Value)
	}
	a.zetCookie(w, "dg_documenten", "", -1)
	q := url.Values{"id_token_hint": {s.IDToken}, "post_logout_redirect_uri": {a.Basis + "/"}}
	http.Redirect(w, r, a.OIDC.uitlogURL()+"?"+q.Encode(), http.StatusSeeOther)
}

func (a *App) csrfGoed(r *http.Request, s *Sessie) bool {
	return subtle.ConstantTimeCompare([]byte(r.FormValue("csrf")), []byte(s.CSRF)) == 1
}

func (a *App) metSessie(h func(http.ResponseWriter, *http.Request, *Sessie)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var s *Sessie
		if c, err := r.Cookie(a.cookieNaam("dg_documenten")); err == nil {
			s, err = a.Reg.Sessie(r.Context(), c.Value)
			if err != nil && !errors.Is(err, errNietGevonden) {
				a.interneFout(w, r, nil, err)
				return
			}
			if s != nil && !a.actueel(r.Context(), s) {
				a.Reg.EindSessie(r.Context(), c.Value) // geblokkeerd of verwijderd
				s = nil
			}
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
