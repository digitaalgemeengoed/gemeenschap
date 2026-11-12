package main

// WebDAV over HTTP: inloggen met een app-wachtwoord (/dav/) of een
// tijdelijke link (/o/), en de pagina's om een netwerkschijf te koppelen en
// een document in Word of LibreOffice te openen.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/digitaalgemeengoed/gemeenschap/documenten/internal/webdav"
)

func (a *App) davRoutes(mux *http.ServeMux) {
	fs := &DavFS{a: a, vluchtig: &Vluchtig{}, recent: &Recent{}, props: &DodeProps{}}
	logger := func(r *http.Request, err error) {
		if err != nil && !errors.Is(err, errNietGevonden) && r.Method != "PROPFIND" {
			log.Printf("dav %s %s: %v", r.Method, r.URL.Path, err)
		}
	}
	dav := &webdav.Handler{Prefix: "/dav", FileSystem: fs, LockSystem: webdav.NewMemLS(), Logger: logger}
	open := &webdav.Handler{Prefix: "/o", FileSystem: fs, LockSystem: webdav.NewMemLS(), Logger: logger}

	mux.HandleFunc("/dav", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/dav/"
		a.davMetWachtwoord(dav).ServeHTTP(w, r)
	})
	mux.Handle("/dav/", a.davMetWachtwoord(dav))
	mux.HandleFunc("/o/", func(w http.ResponseWriter, r *http.Request) {
		// /o/<sleutel>/<naam>: de sleutel is de toegang.
		rest := strings.TrimPrefix(r.URL.Path, "/o/")
		sleutel, _, _ := strings.Cut(rest, "/")
		if sleutel == "" {
			http.NotFound(w, r)
			return
		}
		l, err := a.Reg.OpenLinkVan(r.Context(), sleutel)
		if err == nil && !a.actueel(r.Context(), l.Sessie) {
			err = errNietGevonden
		}
		if err != nil {
			http.Error(w, "Deze link is verlopen. Open het document opnieuw vanuit Documenten.", http.StatusNotFound)
			return
		}
		ctx := context.WithValue(r.Context(), davSleutel{}, &davVerzoek{s: l.Sessie, booms: map[int64]*Boom{}, alleen: l})
		open.ServeHTTP(w, r.WithContext(ctx))
	})
	// Windows vraagt bij het koppelen eerst OPTIONS op de hoogste map.
	mux.HandleFunc("OPTIONS /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("DAV", "1, 2")
		w.Header().Set("MS-Author-Via", "DAV")
		w.Header().Set("Allow", "OPTIONS, GET, HEAD")
	})
}

func (a *App) davMetWachtwoord(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ww, ok := r.BasicAuth()
		var s *Sessie
		if ok {
			var err error
			s, err = a.Reg.MetAppWachtwoord(r.Context(), email, ww)
			if err != nil && !errors.Is(err, errNietGevonden) {
				log.Printf("dav inloggen: %v", err)
				http.Error(w, "Er ging iets mis.", http.StatusInternalServerError)
				return
			}
			if s != nil && !a.actueel(r.Context(), s) {
				s = nil // geblokkeerd of verwijderd
			}
		}
		if s == nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="Documenten Digitaal GemeenGoed", charset="UTF-8"`)
			http.Error(w, "Log in met je e-mailadres en een app-wachtwoord (Documenten → Netwerkschijf koppelen).", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), davSleutel{}, &davVerzoek{s: s, booms: map[int64]*Boom{}})
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

// --- Netwerkschijf koppelen ------------------------------------------------------

// eenmalig: een nieuw app-wachtwoord, tot het één keer getoond is (per sessie).
var eenmalig sync.Map

type Koppelen struct {
	Adres     string
	Email     string
	Lijst     []AppWachtwoord
	Nieuw     string // het nieuwe wachtwoord, één keer getoond
	NieuwNaam string
}

func (a *App) koppelen(w http.ResponseWriter, r *http.Request, s *Sessie) {
	k := &Koppelen{Adres: a.Basis + "/dav/", Email: s.Email}
	if r.Method == http.MethodPost {
		if !a.csrfGoed(r, s) {
			a.fout(w, r, s, http.StatusForbidden, "Verlopen formulier. Ververs de pagina en probeer het opnieuw.")
			return
		}
		switch r.FormValue("actie") {
		case "nieuw":
			naam := strings.TrimSpace(r.FormValue("naam"))
			if naam == "" {
				naam = "Mijn apparaat"
			}
			if len(naam) > 80 {
				naam = naam[:80]
			}
			ww, err := a.Reg.NieuwAppWachtwoord(r.Context(), s.Email, naam)
			if err != nil {
				a.interneFout(w, r, s, err)
				return
			}
			// Eén keer tonen, na een doorverwijzing (verversen maakt geen nieuw).
			eenmalig.Store(s.CSRF, [2]string{ww, naam})
			http.Redirect(w, r, "/koppelen", http.StatusSeeOther)
			return
		case "weg":
			id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
			if err := a.Reg.AppWachtwoordWeg(r.Context(), s.Email, id); err != nil {
				a.interneFout(w, r, s, err)
				return
			}
			terug(w, r, "/koppelen", "melding", "Het app-wachtwoord is ingetrokken.")
			return
		}
	}
	if x, ok := eenmalig.LoadAndDelete(s.CSRF); ok {
		n := x.([2]string)
		k.Nieuw, k.NieuwNaam = n[0], n[1]
	}
	var err error
	if k.Lijst, err = a.Reg.AppWachtwoorden(r.Context(), s.Email); err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	a.toon(w, r, "koppelen", http.StatusOK, pagina{Titel: "Netwerkschijf koppelen", Sessie: s, Data: k})
}

// --- Openen in Word of LibreOffice -------------------------------------------------

// programma: welke programma's een bestand kunnen openen, en hun URI-schema.
type programma struct {
	Sleutel, Naam, Schema string
}

var programmas = []programma{
	{"word", "Word", "ms-word"},
	{"excel", "Excel", "ms-excel"},
	{"powerpoint", "PowerPoint", "ms-powerpoint"},
	{"libreoffice", "LibreOffice", "vnd.libreoffice.command"},
}

func programmasVoor(naam string) []programma {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(naam), "."))
	var uit []programma
	switch ext {
	case "doc", "docx", "docm", "dot", "dotx", "rtf":
		uit = append(uit, programmas[0])
	case "xls", "xlsx", "xlsm", "csv":
		uit = append(uit, programmas[1])
	case "ppt", "pptx", "pptm":
		uit = append(uit, programmas[2])
	}
	switch ext {
	case "doc", "docx", "docm", "dot", "dotx", "rtf", "xls", "xlsx", "xlsm", "csv", "ppt", "pptx", "pptm",
		"odt", "ods", "odp", "odg", "ott", "ots", "otp", "txt", "md":
		uit = append(uit, programmas[3])
	}
	return uit
}

// openen: maak een tijdelijke link en geef het adres voor het programma terug.
func (a *App) openen(w http.ResponseWriter, r *http.Request, s *Sessie, p *Plek) {
	x := a.item(w, r, s, p)
	if x == nil || x.Soort != "document" || !a.mag(w, r, s, p, x.ID, Lezen) {
		return
	}
	var prog *programma
	for _, q := range programmasVoor(x.Naam) {
		if q.Sleutel == r.FormValue("app") {
			prog = &q
		}
	}
	if prog == nil {
		a.fout(w, r, s, http.StatusBadRequest, "Dit bestand kan niet in dat programma worden geopend.")
		return
	}
	sleutel, err := a.Reg.NieuweOpenLink(r.Context(), s.Email, p.Ruimte.ID, x.ID)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	link := a.Basis + "/o/" + sleutel + "/" + url.PathEscape(x.Naam)
	wijze := "ofe" // openen om te bewerken
	if p.Boom.Niveau(x.ID) < Bewerken {
		wijze = "ofv" // alleen bekijken
	}
	adres := prog.Schema + ":" + wijze + "|u|" + link
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]string{"adres": adres, "programma": prog.Naam})
		return
	}
	a.toon(w, r, "openen", http.StatusOK, pagina{Titel: "Openen in " + prog.Naam, Sessie: s, Data: map[string]any{
		"Adres": template.URL(adres) /* eigen schema; door ons opgebouwd */, "Programma": prog.Naam, "Naam": x.Naam,
		"Terug": fmt.Sprintf("/r/%s/d/%d", p.Ruimte.Sleutel, x.ID),
	}})
}
