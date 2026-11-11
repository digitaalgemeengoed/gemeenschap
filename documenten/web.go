package main

// De bestandsbeheerder in de browser.

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode"
)

//go:embed sjablonen/*.html
var sjabloonBestanden embed.FS

//go:embed statisch
var statischeBestanden embed.FS

type App struct {
	Reg        *Register
	Opslag     *S3
	OIDC       *OIDC
	Basis      string
	Geheim     []byte
	MaxGrootte int64
	Projecten  string // publieke URL van Redmine, voor links
	sjablonen  map[string]*template.Template
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

	mux.HandleFunc("GET /{$}", a.metSessie(a.overzicht))
	mux.HandleFunc("GET /r/{ruimte}", a.metRuimte(false, a.toonMap))
	mux.HandleFunc("GET /r/{ruimte}/m/{id}", a.metRuimte(false, a.toonMap))
	mux.HandleFunc("GET /r/{ruimte}/d/{id}", a.metRuimte(false, a.toonDocument))
	mux.HandleFunc("GET /r/{ruimte}/d/{id}/download", a.metRuimte(false, a.download))
	mux.HandleFunc("POST /r/{ruimte}/map", a.metRuimte(true, a.nieuweMap))
	mux.HandleFunc("POST /r/{ruimte}/upload", a.metRuimte(true, a.upload))
	mux.HandleFunc("POST /r/{ruimte}/d/{id}/metadata", a.metRuimte(true, a.metadata))
	mux.HandleFunc("POST /r/{ruimte}/i/{id}/hernoem", a.metRuimte(true, a.hernoem))
	mux.HandleFunc("POST /r/{ruimte}/i/{id}/verwijder", a.metRuimte(true, a.verwijder))
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
		"join": strings.Join,
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
	for _, naam := range []string{"overzicht", "map", "document", "fout"} {
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
	a.toon(w, r, "fout", status, pagina{Titel: http.StatusText(status), Sessie: s, Data: tekst})
}

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

// --- Rechten ----------------------------------------------------------------

type Toegang struct {
	Ruimte    *Ruimte
	Schrijven bool
}

func (a *App) rechten(ctx context.Context, s *Sessie, rm *Ruimte) (lezen, schrijven bool, err error) {
	open := rm.Status == "actief"
	if s.In(groepBeheer) {
		return true, open, nil
	}
	switch rm.Soort {
	case "cooperatie":
		return s.In(groepMembers) || s.In(groepKerngroep), open && s.In(groepKerngroep), nil
	case "project":
		lid, err := a.Reg.Lid(ctx, rm.ID, s.Email)
		return lid, lid && open, err
	}
	return false, false, nil
}

func (a *App) metRuimte(schrijven bool, h func(http.ResponseWriter, *http.Request, *Sessie, *Toegang)) http.HandlerFunc {
	return a.metSessie(func(w http.ResponseWriter, r *http.Request, s *Sessie) {
		rm, err := a.Reg.Ruimte(r.Context(), r.PathValue("ruimte"))
		if errors.Is(err, errNietGevonden) {
			a.fout(w, r, s, http.StatusNotFound, "Deze ruimte bestaat niet, of je bent er geen lid van.")
			return
		}
		if err != nil {
			a.interneFout(w, r, s, err)
			return
		}
		lezen, mag, err := a.rechten(r.Context(), s, rm)
		if err != nil {
			a.interneFout(w, r, s, err)
			return
		}
		if !lezen {
			a.fout(w, r, s, http.StatusNotFound, "Deze ruimte bestaat niet, of je bent er geen lid van.")
			return
		}
		if schrijven {
			if !mag {
				a.fout(w, r, s, http.StatusForbidden, "Je kunt in deze ruimte alleen lezen.")
				return
			}
			// Bij een upload staat het csrf-veld in de multipart; daar controleert upload() zelf.
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") && !a.csrfGoed(r, s) {
				a.fout(w, r, s, http.StatusForbidden, "Verlopen formulier. Ga terug, ververs de pagina en probeer het opnieuw.")
				return
			}
		}
		h(w, r, s, &Toegang{Ruimte: rm, Schrijven: mag})
	})
}

func padID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

// --- Pagina's ---------------------------------------------------------------

type ruimteKaart struct {
	Ruimte
	Schrijven bool
	Aantal    int64
	Grootte   int64
}

func (a *App) overzicht(w http.ResponseWriter, r *http.Request, s *Sessie) {
	alle, err := a.Reg.Ruimtes(r.Context())
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	tel, err := a.Reg.Tellingen(r.Context())
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	lidVan, err := a.Reg.LidVan(r.Context(), s.Email)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	var coop, projecten []ruimteKaart
	for _, rm := range alle {
		var lezen, schrijven bool
		open := rm.Status == "actief"
		switch {
		case s.In(groepBeheer):
			lezen, schrijven = true, open
		case rm.Soort == "cooperatie":
			lezen, schrijven = s.In(groepMembers) || s.In(groepKerngroep), open && s.In(groepKerngroep)
		default:
			lezen, schrijven = lidVan[rm.ID], lidVan[rm.ID] && open
		}
		if !lezen {
			continue
		}
		k := ruimteKaart{Ruimte: rm, Schrijven: schrijven, Aantal: tel[rm.ID][0], Grootte: tel[rm.ID][1]}
		if rm.Soort == "cooperatie" {
			coop = append(coop, k)
		} else {
			projecten = append(projecten, k)
		}
	}
	a.toon(w, r, "overzicht", http.StatusOK, pagina{Titel: "Documenten", Sessie: s, Data: map[string]any{
		"Cooperatie": coop, "Projecten": projecten, "Redmine": a.Projecten,
	}})
}

func (a *App) toonMap(w http.ResponseWriter, r *http.Request, s *Sessie, t *Toegang) {
	ouder := padID(r)
	var pad []*Item
	if ouder != 0 {
		var err error
		pad, err = a.Reg.Pad(r.Context(), t.Ruimte.ID, ouder)
		if errors.Is(err, errNietGevonden) || (err == nil && pad[len(pad)-1].Soort != "map") {
			a.fout(w, r, s, http.StatusNotFound, "Deze map bestaat niet (meer).")
			return
		}
		if err != nil {
			a.interneFout(w, r, s, err)
			return
		}
	}
	inhoud, err := a.Reg.Inhoud(r.Context(), t.Ruimte.ID, ouder)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	titel, kop := t.Ruimte.Naam, t.Ruimte.Naam
	if len(pad) > 0 {
		kop = pad[len(pad)-1].Naam
		titel = kop + " · " + t.Ruimte.Naam
	}
	a.toon(w, r, "map", http.StatusOK, pagina{Titel: titel, Sessie: s, Data: map[string]any{
		"Ruimte": t.Ruimte, "Schrijven": t.Schrijven, "Pad": pad, "Kop": kop, "Ouder": ouder, "Inhoud": inhoud,
		"MaxMB": a.MaxGrootte >> 20, "Hier": r.URL.Path,
	}})
}

func (a *App) toonDocument(w http.ResponseWriter, r *http.Request, s *Sessie, t *Toegang) {
	pad, err := a.Reg.Pad(r.Context(), t.Ruimte.ID, padID(r))
	if errors.Is(err, errNietGevonden) || (err == nil && pad[len(pad)-1].Soort != "document") {
		a.fout(w, r, s, http.StatusNotFound, "Dit document bestaat niet (meer).")
		return
	}
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	doc := pad[len(pad)-1]
	versies, err := a.Reg.Versies(r.Context(), doc.ID)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	a.toon(w, r, "document", http.StatusOK, pagina{Titel: doc.Naam + " · " + t.Ruimte.Naam, Sessie: s, Data: map[string]any{
		"Ruimte": t.Ruimte, "Schrijven": t.Schrijven, "Pad": pad[:len(pad)-1], "Doc": doc, "Versies": versies,
		"Soorten": documentsoorten, "Bekijkbaar": bekijkbaar(doc.Mediatype), "Hier": r.URL.Path,
	}})
}

// bekijkbaar: in de browser te openen zonder risico.
func bekijkbaar(mt string) bool {
	switch mt {
	case "application/pdf", "image/png", "image/jpeg", "image/gif", "image/webp", "text/plain; charset=utf-8":
		return true
	}
	return false
}

func (a *App) download(w http.ResponseWriter, r *http.Request, s *Sessie, t *Toegang) {
	doc, err := a.Reg.Item(r.Context(), t.Ruimte.ID, padID(r))
	if errors.Is(err, errNietGevonden) || (err == nil && doc.Soort != "document") {
		a.fout(w, r, s, http.StatusNotFound, "Dit document bestaat niet (meer).")
		return
	}
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	nummer := doc.HuidigeVersie
	if v := r.URL.Query().Get("versie"); v != "" {
		nummer, _ = strconv.Atoi(v)
	}
	v, err := a.Reg.Versie(r.Context(), doc.ID, nummer)
	if errors.Is(err, errNietGevonden) {
		a.fout(w, r, s, http.StatusNotFound, "Deze versie bestaat niet.")
		return
	}
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	etag := `"` + v.SHA256 + `"`
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	body, lengte, err := a.Opslag.Haal(r.Context(), v.Object)
	if err != nil {
		a.interneFout(w, r, s, fmt.Errorf("object %s: %w", v.Object, err))
		return
	}
	defer body.Close()
	k := w.Header()
	k.Set("Content-Type", v.Mediatype)
	if lengte >= 0 {
		k.Set("Content-Length", strconv.FormatInt(lengte, 10))
	}
	k.Set("ETag", etag)
	k.Set("Cache-Control", "private, no-cache")
	wijze := "attachment"
	if r.URL.Query().Get("bekijken") == "1" && bekijkbaar(v.Mediatype) {
		wijze = "inline"
		k.Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; object-src 'self'")
	}
	k.Set("Content-Disposition", mime.FormatMediaType(wijze, map[string]string{"filename": doc.Naam}))
	io.Copy(w, body)
}

// --- Handelingen ------------------------------------------------------------

// geldigeNaam: wat ook in Windows, macOS en WebDAV kan.
func geldigeNaam(n string) (string, error) {
	n = strings.TrimSpace(n)
	switch {
	case n == "" || n == "." || n == "..":
		return "", errors.New("Geef een naam.")
	case len(n) > 200:
		return "", errors.New("De naam is te lang (hooguit 200 tekens).")
	case strings.ContainsAny(n, `/\:*?"<>|`):
		return "", errors.New(`Een naam mag geen / \ : * ? " < > of | bevatten.`)
	case strings.HasSuffix(n, "."):
		return "", errors.New("Een naam mag niet op een punt eindigen.")
	}
	for _, c := range n {
		if unicode.IsControl(c) {
			return "", errors.New("De naam bevat een ongeldig teken.")
		}
	}
	return n, nil
}

func mapURL(rm *Ruimte, ouder int64) string {
	if ouder == 0 {
		return "/r/" + rm.Sleutel
	}
	return fmt.Sprintf("/r/%s/m/%d", rm.Sleutel, ouder)
}

func (a *App) nieuweMap(w http.ResponseWriter, r *http.Request, s *Sessie, t *Toegang) {
	ouder, _ := strconv.ParseInt(r.FormValue("ouder"), 10, 64)
	naam, err := geldigeNaam(r.FormValue("naam"))
	if err != nil {
		terug(w, r, mapURL(t.Ruimte, ouder), "fout", err.Error())
		return
	}
	id, err := a.Reg.NieuweMap(r.Context(), t.Ruimte.ID, ouder, naam, s.Email)
	switch {
	case errors.Is(err, errBestaatAl):
		terug(w, r, mapURL(t.Ruimte, ouder), "fout", "Hier staat al iets met de naam "+naam+".")
	case errors.Is(err, errNietGevonden):
		a.fout(w, r, s, http.StatusNotFound, "Deze map bestaat niet (meer).")
	case err != nil:
		a.interneFout(w, r, s, err)
	default:
		http.Redirect(w, r, mapURL(t.Ruimte, id), http.StatusSeeOther)
	}
}

func (a *App) upload(w http.ResponseWriter, r *http.Request, s *Sessie, t *Toegang) {
	// Ruim boven de grens, voor de overige velden; per bestand telt de grens zelf.
	r.Body = http.MaxBytesReader(w, r.Body, a.MaxGrootte*4+(1<<20))
	mr, err := r.MultipartReader()
	if err != nil {
		a.fout(w, r, s, http.StatusBadRequest, "Geen bestanden ontvangen.")
		return
	}
	var ouder int64
	var doc *Item // een nieuwe versie van dit document, onder zijn eigen naam
	csrfOK := false
	var gelukt, mislukt []string
delen:
	for {
		deel, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			mislukt = append(mislukt, "de upload brak af")
			break
		}
		switch deel.FormName() {
		case "csrf":
			b, _ := io.ReadAll(io.LimitReader(deel, 200))
			csrfOK = subtle.ConstantTimeCompare(b, []byte(s.CSRF)) == 1
			continue
		case "ouder":
			b, _ := io.ReadAll(io.LimitReader(deel, 30))
			ouder, _ = strconv.ParseInt(string(b), 10, 64)
			continue
		case "document":
			b, _ := io.ReadAll(io.LimitReader(deel, 30))
			id, _ := strconv.ParseInt(string(b), 10, 64)
			doc, err = a.Reg.Item(r.Context(), t.Ruimte.ID, id)
			if err != nil || doc.Soort != "document" {
				a.fout(w, r, s, http.StatusNotFound, "Dit document bestaat niet (meer).")
				return
			}
			ouder = doc.OuderID.Int64
			continue
		case "bestanden":
		default:
			continue
		}
		if !csrfOK {
			a.fout(w, r, s, http.StatusForbidden, "Verlopen formulier. Ga terug, ververs de pagina en probeer het opnieuw.")
			return
		}
		naam := path.Base(strings.ReplaceAll(deel.FileName(), "\\", "/"))
		if deel.FileName() == "" {
			continue
		}
		if doc != nil {
			naam = doc.Naam
		}
		naam, err = geldigeNaam(naam)
		if err != nil {
			mislukt = append(mislukt, deel.FileName()+": "+err.Error())
			continue
		}
		if err := a.bewaar(r.Context(), t.Ruimte, ouder, naam, deel, s); err != nil {
			var mbe *http.MaxBytesError
			switch {
			case errors.Is(err, errTeGroot) || errors.As(err, &mbe):
				mislukt = append(mislukt, fmt.Sprintf("%s: groter dan %d MB", naam, a.MaxGrootte>>20))
			case errors.Is(err, errIsMap):
				mislukt = append(mislukt, naam+": "+err.Error())
			case errors.Is(err, errNietGevonden):
				a.fout(w, r, s, http.StatusNotFound, "Deze map bestaat niet (meer).")
				return
			default:
				log.Printf("upload %s: %v", naam, err)
				mislukt = append(mislukt, naam+": niet gelukt, probeer het opnieuw")
			}
			if errors.As(err, &mbe) {
				break delen
			}
			continue
		}
		gelukt = append(gelukt, naam)
	}
	doel := mapURL(t.Ruimte, ouder)
	if doc != nil {
		doel = fmt.Sprintf("/r/%s/d/%d", t.Ruimte.Sleutel, doc.ID)
	}
	switch {
	case len(mislukt) > 0:
		terug(w, r, doel, "fout", "Niet bewaard: "+strings.Join(mislukt, "; ")+".")
	case len(gelukt) == 1:
		terug(w, r, doel, "melding", gelukt[0]+" is bewaard.")
	case len(gelukt) > 1:
		terug(w, r, doel, "melding", fmt.Sprintf("%d bestanden bewaard.", len(gelukt)))
	default:
		terug(w, r, doel, "fout", "Kies eerst een of meer bestanden.")
	}
}

var errTeGroot = errors.New("te groot")

// bewaar: eerst naar een tijdelijk bestand (met SHA-256), dan naar de
// opslag, dan in het register. Lukt het register niet, dan blijft er een
// object over dat niemand kent; dat is onschadelijk.
func (a *App) bewaar(ctx context.Context, rm *Ruimte, ouder int64, naam string, inhoud io.Reader, s *Sessie) error {
	f, err := os.CreateTemp("", "upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	h := sha256.New()
	kop := make([]byte, 512)
	n, err := io.ReadFull(inhoud, kop)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return err
	}
	kop = kop[:n]
	f.Write(kop)
	h.Write(kop)
	rest, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(inhoud, a.MaxGrootte-int64(n)+1))
	if err != nil {
		return err
	}
	lengte := int64(n) + rest
	if lengte > a.MaxGrootte {
		return errTeGroot
	}
	mt := mime.TypeByExtension(strings.ToLower(path.Ext(naam)))
	if mt == "" {
		mt = http.DetectContentType(kop)
	}
	som := hex.EncodeToString(h.Sum(nil))
	object := "objecten/" + nieuwID()
	if err := a.Opslag.Zet(ctx, object, f, lengte, som, mt); err != nil {
		return err
	}
	v := Versie{Object: object, SHA256: som, Grootte: lengte, Mediatype: mt}
	_, _, err = a.Reg.NieuweVersie(ctx, rm.ID, ouder, naam, s.Email, v)
	if errors.Is(err, errBestaatAl) { // tegelijk aangemaakt: nog één keer
		_, _, err = a.Reg.NieuweVersie(ctx, rm.ID, ouder, naam, s.Email, v)
	}
	return err
}

// nieuwID: een willekeurige UUID (versie 4).
func nieuwID() string {
	b := willekeurigeBytes(16)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	x := hex.EncodeToString(b)
	return x[0:8] + "-" + x[8:12] + "-" + x[12:16] + "-" + x[16:20] + "-" + x[20:]
}

func (a *App) metadata(w http.ResponseWriter, r *http.Request, s *Sessie, t *Toegang) {
	id := padID(r)
	doel := fmt.Sprintf("/r/%s/d/%d", t.Ruimte.Sleutel, id)
	titel := strings.TrimSpace(r.FormValue("titel"))
	omschrijving := strings.TrimSpace(r.FormValue("omschrijving"))
	soort := r.FormValue("documentsoort")
	geldig := soort == ""
	for _, d := range documentsoorten {
		geldig = geldig || d == soort
	}
	if !geldig || len(titel) > 300 || len(omschrijving) > 4000 {
		terug(w, r, doel, "fout", "Controleer de velden: de titel hooguit 300 tekens, de omschrijving hooguit 4000.")
		return
	}
	var trefwoorden []string
	gezien := map[string]bool{}
	for _, t := range strings.Split(r.FormValue("trefwoorden"), ",") {
		t = strings.TrimSpace(t)
		if t != "" && len(t) <= 60 && !gezien[strings.ToLower(t)] && len(trefwoorden) < 20 {
			gezien[strings.ToLower(t)] = true
			trefwoorden = append(trefwoorden, t)
		}
	}
	err := a.Reg.Metadata(r.Context(), t.Ruimte.ID, id, titel, omschrijving, soort, trefwoorden, s.Email)
	switch {
	case errors.Is(err, errNietGevonden):
		a.fout(w, r, s, http.StatusNotFound, "Dit document bestaat niet (meer).")
	case err != nil:
		a.interneFout(w, r, s, err)
	default:
		terug(w, r, doel, "melding", "Bewaard.")
	}
}

func (a *App) hernoem(w http.ResponseWriter, r *http.Request, s *Sessie, t *Toegang) {
	id := padID(r)
	doel := veiligTerug(r.FormValue("terug"))
	naam, err := geldigeNaam(r.FormValue("naam"))
	if err != nil {
		terug(w, r, doel, "fout", err.Error())
		return
	}
	err = a.Reg.Hernoem(r.Context(), t.Ruimte.ID, id, naam, s.Email)
	switch {
	case errors.Is(err, errBestaatAl):
		terug(w, r, doel, "fout", "Hier staat al iets met de naam "+naam+".")
	case errors.Is(err, errNietGevonden):
		a.fout(w, r, s, http.StatusNotFound, "Dit bestaat niet (meer).")
	case err != nil:
		a.interneFout(w, r, s, err)
	default:
		terug(w, r, doel, "melding", "Hernoemd naar "+naam+".")
	}
}

func (a *App) verwijder(w http.ResponseWriter, r *http.Request, s *Sessie, t *Toegang) {
	id := padID(r)
	x, err := a.Reg.Item(r.Context(), t.Ruimte.ID, id)
	if errors.Is(err, errNietGevonden) {
		a.fout(w, r, s, http.StatusNotFound, "Dit bestaat niet (meer).")
		return
	}
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	if err := a.Reg.Verwijder(r.Context(), t.Ruimte.ID, id, s.Email); err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	terug(w, r, mapURL(t.Ruimte, x.OuderID.Int64), "melding", x.Naam+" staat in de prullenbak. De beheerder kan het terugzetten.")
}
