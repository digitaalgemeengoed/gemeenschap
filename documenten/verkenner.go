package main

// De verkenner: links de ruimtes en de mappenboom, in het midden de inhoud
// van één map, rechts (als je iets kiest) de eigenschappen of de toegang.
// Alles wordt op de server opgebouwd; JavaScript maakt het alleen prettiger.

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"unicode"
)

// Plek: de ruimte en de boom, voor één verzoek.
type Plek struct {
	Ruimte *Ruimte
	Boom   *Boom
}

func (a *App) metRuimte(h func(http.ResponseWriter, *http.Request, *Sessie, *Plek)) http.HandlerFunc {
	return a.metSessie(func(w http.ResponseWriter, r *http.Request, s *Sessie) {
		rm, err := a.Reg.Ruimte(r.Context(), r.PathValue("ruimte"))
		if errors.Is(err, errNietGevonden) {
			a.fout(w, r, s, http.StatusNotFound, "Deze ruimte bestaat niet, of je hebt er geen toegang toe.")
			return
		}
		if err != nil {
			a.interneFout(w, r, s, err)
			return
		}
		b, err := a.Reg.laadBoom(r.Context(), rm, s)
		if err != nil {
			a.interneFout(w, r, s, err)
			return
		}
		if !b.Zichtbaar(0) {
			a.fout(w, r, s, http.StatusNotFound, "Deze ruimte bestaat niet, of je hebt er geen toegang toe.")
			return
		}
		if r.Method == http.MethodPost && !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") && !a.csrfGoed(r, s) {
			a.fout(w, r, s, http.StatusForbidden, "Verlopen formulier. Ga terug, ververs de pagina en probeer het opnieuw.")
			return
		}
		h(w, r, s, &Plek{Ruimte: rm, Boom: b})
	})
}

// mag: heeft deze persoon dit niveau in deze map? Zo niet: een nette fout.
func (a *App) mag(w http.ResponseWriter, r *http.Request, s *Sessie, p *Plek, mapID int64, niveau int) bool {
	if !p.Boom.Bestaat(mapID) {
		a.fout(w, r, s, http.StatusNotFound, "Deze map bestaat niet (meer).")
		return false
	}
	heeft := p.Boom.Niveau(mapID)
	if heeft >= niveau {
		return true
	}
	if heeft == Geen {
		a.fout(w, r, s, http.StatusNotFound, "Deze map bestaat niet, of je hebt er geen toegang toe.")
		return false
	}
	tekst := "Je hebt hier het recht " + niveauNaam(heeft) + "; hiervoor is " + niveauNaam(niveau) + " nodig."
	if p.Ruimte.Status != "actief" {
		tekst = "Dit project is gesloten: je kunt alleen nog lezen."
	}
	a.fout(w, r, s, http.StatusForbidden, tekst)
	return false
}

func padID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

// --- De verkenner -------------------------------------------------------------

type ruimteKaart struct {
	Ruimte
	Niveau  int
	Aantal  int64
	Grootte int64
	Actief  bool
}

type regelRegel struct {
	Regel
	Naam, SoortLabel string
}

type Verkenner struct {
	Ruimtes  []ruimteKaart
	Ruimte   *Ruimte
	Knopen   []*Knoop
	Pad      []*Item // mappen van boven naar hier
	Map      *Item   // nil = de bovenste laag
	MapID    int64
	Niveau   int
	Doorgang bool
	Besloten bool
	Inhoud   []*Item
	Selectie *Item
	Versies  []Versie
	Paneel   string // "", "eigenschappen", "toegang"
	Hier     string
	MaxMB    int64
	Soorten  []string
	// Toegang
	Regels         []regelRegel
	Groepen        []Keuze
	Personen       []Keuze
	SelNiveau      int // recht op de map van de selectie
	Bekijkbaar     bool
	Redmine        string
	BeslotenMappen map[int64]bool
}

func (v *Verkenner) MapURL(id int64) string { return mapURL(v.Ruimte, id) }

func (a *App) ruimteKaarten(ctx context.Context, s *Sessie, huidig *Ruimte) ([]ruimteKaart, error) {
	alle, err := a.Reg.Ruimtes(ctx)
	if err != nil {
		return nil, err
	}
	tel, err := a.Reg.Tellingen(ctx)
	if err != nil {
		return nil, err
	}
	var uit []ruimteKaart
	for i := range alle {
		rm := &alle[i]
		b, err := a.Reg.laadBoom(ctx, rm, s)
		if err != nil {
			return nil, err
		}
		if !b.Zichtbaar(0) {
			continue
		}
		uit = append(uit, ruimteKaart{Ruimte: *rm, Niveau: b.Niveau(0), Aantal: tel[rm.ID][0], Grootte: tel[rm.ID][1],
			Actief: huidig != nil && huidig.ID == rm.ID})
	}
	return uit, nil
}

func (a *App) thuis(w http.ResponseWriter, r *http.Request, s *Sessie) {
	kaarten, err := a.ruimteKaarten(r.Context(), s, nil)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	a.toon(w, r, "verkenner", http.StatusOK, pagina{Titel: "Documenten", Sessie: s, Data: &Verkenner{
		Ruimtes: kaarten, Hier: "/", Redmine: a.Projecten,
	}})
}

// verkenner bouwt het scherm voor een map, eventueel met een selectie of paneel.
func (a *App) verkenner(ctx context.Context, s *Sessie, p *Plek, mapID int64) (*Verkenner, error) {
	kaarten, err := a.ruimteKaarten(ctx, s, p.Ruimte)
	if err != nil {
		return nil, err
	}
	v := &Verkenner{Ruimtes: kaarten, Ruimte: p.Ruimte, MapID: mapID, Niveau: p.Boom.Niveau(mapID),
		Besloten: p.Boom.Besloten(mapID), MaxMB: a.MaxGrootte >> 20, Soorten: documentsoorten, Redmine: a.Projecten}
	v.Doorgang = v.Niveau < Lezen
	opPad := map[int64]bool{}
	if mapID != 0 {
		v.Pad, err = a.Reg.Pad(ctx, p.Ruimte.ID, mapID)
		if err != nil {
			return nil, err
		}
		v.Map = v.Pad[len(v.Pad)-1]
		for _, m := range v.Pad {
			opPad[m.ID] = true
		}
	}
	v.Knopen = p.Boom.Knopen(0, mapID, opPad)
	inhoud, err := a.Reg.Inhoud(ctx, p.Ruimte.ID, mapID)
	if err != nil {
		return nil, err
	}
	v.BeslotenMappen = map[int64]bool{}
	for _, x := range inhoud {
		if x.Soort == "map" {
			if p.Boom.Zichtbaar(x.ID) {
				v.Inhoud = append(v.Inhoud, x)
				v.BeslotenMappen[x.ID] = p.Boom.Besloten(x.ID)
			}
		} else if !v.Doorgang {
			v.Inhoud = append(v.Inhoud, x)
		}
	}
	return v, nil
}

func (a *App) toonMap(w http.ResponseWriter, r *http.Request, s *Sessie, p *Plek) {
	mapID := padID(r)
	if !p.Boom.Bestaat(mapID) || !p.Boom.Zichtbaar(mapID) {
		a.fout(w, r, s, http.StatusNotFound, "Deze map bestaat niet (meer), of je hebt er geen toegang toe.")
		return
	}
	v, err := a.verkenner(r.Context(), s, p, mapID)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	v.Hier = mapURL(p.Ruimte, mapID)
	if strings.HasSuffix(r.URL.Path, "/toegang") && !v.Doorgang {
		v.Paneel = "toegang"
		namen, err := a.Reg.Namen(r.Context())
		if err != nil {
			a.interneFout(w, r, s, err)
			return
		}
		for _, x := range p.Boom.Geldend(mapID) {
			n, sl := regelLabel(x, namen)
			v.Regels = append(v.Regels, regelRegel{Regel: x, Naam: n, SoortLabel: sl})
		}
		if v.Niveau >= Beheren {
			v.Groepen, v.Personen, err = a.Reg.Kandidaten(r.Context(), p.Ruimte)
			if err != nil {
				a.interneFout(w, r, s, err)
				return
			}
		}
	}
	titel := p.Ruimte.Naam
	if v.Map != nil {
		titel = v.Map.Naam + " · " + p.Ruimte.Naam
	}
	a.toon(w, r, "verkenner", http.StatusOK, pagina{Titel: titel, Sessie: s, Data: v})
}

func (a *App) toonDocument(w http.ResponseWriter, r *http.Request, s *Sessie, p *Plek) {
	doc, err := a.Reg.Item(r.Context(), p.Ruimte.ID, padID(r))
	if errors.Is(err, errNietGevonden) || (err == nil && doc.Soort != "document") {
		a.fout(w, r, s, http.StatusNotFound, "Dit document bestaat niet (meer).")
		return
	}
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	if !a.mag(w, r, s, p, doc.OuderID.Int64, Lezen) {
		return
	}
	v, err := a.verkenner(r.Context(), s, p, doc.OuderID.Int64)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	v.Selectie, v.Paneel, v.SelNiveau = doc, "eigenschappen", v.Niveau
	v.Bekijkbaar = bekijkbaar(doc.Mediatype)
	v.Hier = fmt.Sprintf("/r/%s/d/%d", p.Ruimte.Sleutel, doc.ID)
	if v.Versies, err = a.Reg.Versies(r.Context(), doc.ID); err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	a.toon(w, r, "verkenner", http.StatusOK, pagina{Titel: doc.Naam + " · " + p.Ruimte.Naam, Sessie: s, Data: v})
}

// bekijkbaar: in de browser te openen zonder risico.
func bekijkbaar(mt string) bool {
	switch mt {
	case "application/pdf", "image/png", "image/jpeg", "image/gif", "image/webp", "text/plain; charset=utf-8":
		return true
	}
	return false
}

func (a *App) download(w http.ResponseWriter, r *http.Request, s *Sessie, p *Plek) {
	doc, err := a.Reg.Item(r.Context(), p.Ruimte.ID, padID(r))
	if errors.Is(err, errNietGevonden) || (err == nil && doc.Soort != "document") {
		a.fout(w, r, s, http.StatusNotFound, "Dit document bestaat niet (meer).")
		return
	}
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	if !a.mag(w, r, s, p, doc.OuderID.Int64, Lezen) {
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

// --- Handelingen --------------------------------------------------------------

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

func mapURL(rm *Ruimte, id int64) string {
	if rm == nil {
		return "/"
	}
	if id == 0 {
		return "/r/" + rm.Sleutel
	}
	return fmt.Sprintf("/r/%s/m/%d", rm.Sleutel, id)
}

func (a *App) nieuweMap(w http.ResponseWriter, r *http.Request, s *Sessie, p *Plek) {
	ouder, _ := strconv.ParseInt(r.FormValue("ouder"), 10, 64)
	if !a.mag(w, r, s, p, ouder, Bewerken) {
		return
	}
	naam, err := geldigeNaam(r.FormValue("naam"))
	if err != nil {
		terug(w, r, mapURL(p.Ruimte, ouder), "fout", err.Error())
		return
	}
	id, err := a.Reg.NieuweMap(r.Context(), p.Ruimte.ID, ouder, naam, s.Email)
	switch {
	case errors.Is(err, errBestaatAl):
		terug(w, r, mapURL(p.Ruimte, ouder), "fout", "Hier staat al iets met de naam "+naam+".")
	case errors.Is(err, errNietGevonden):
		a.fout(w, r, s, http.StatusNotFound, "Deze map bestaat niet (meer).")
	case err != nil:
		a.interneFout(w, r, s, err)
	default:
		http.Redirect(w, r, mapURL(p.Ruimte, id), http.StatusSeeOther)
	}
}

var errTeGroot = errors.New("te groot")

func (a *App) upload(w http.ResponseWriter, r *http.Request, s *Sessie, p *Plek) {
	// Ruim boven de grens, voor de overige velden; per bestand telt de grens zelf.
	r.Body = http.MaxBytesReader(w, r.Body, a.MaxGrootte*4+(1<<20))
	mr, err := r.MultipartReader()
	if err != nil {
		a.fout(w, r, s, http.StatusBadRequest, "Geen bestanden ontvangen.")
		return
	}
	var ouder int64
	var doc *Item // een nieuwe versie van dit document, onder zijn eigen naam
	csrfOK, rechtOK := false, false
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
			doc, err = a.Reg.Item(r.Context(), p.Ruimte.ID, id)
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
		if !rechtOK {
			if !a.mag(w, r, s, p, ouder, Bewerken) {
				return
			}
			rechtOK = true
		}
		if deel.FileName() == "" {
			continue
		}
		naam := path.Base(strings.ReplaceAll(deel.FileName(), "\\", "/"))
		if doc != nil {
			naam = doc.Naam
		}
		naam, err = geldigeNaam(naam)
		if err != nil {
			mislukt = append(mislukt, deel.FileName()+": "+err.Error())
			continue
		}
		if err := a.bewaar(r.Context(), p.Ruimte, ouder, naam, deel, s); err != nil {
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
	doel := mapURL(p.Ruimte, ouder)
	if doc != nil {
		doel = fmt.Sprintf("/r/%s/d/%d", p.Ruimte.Sleutel, doc.ID)
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

func (a *App) metadata(w http.ResponseWriter, r *http.Request, s *Sessie, p *Plek) {
	id := padID(r)
	doc, err := a.Reg.Item(r.Context(), p.Ruimte.ID, id)
	if err != nil || doc.Soort != "document" {
		a.fout(w, r, s, http.StatusNotFound, "Dit document bestaat niet (meer).")
		return
	}
	if !a.mag(w, r, s, p, doc.OuderID.Int64, Bewerken) {
		return
	}
	doel := fmt.Sprintf("/r/%s/d/%d", p.Ruimte.Sleutel, id)
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
	trefwoorden := []string{} // niet nil: dan wordt het NULL in de database
	gezien := map[string]bool{}
	for _, t := range strings.Split(r.FormValue("trefwoorden"), ",") {
		t = strings.TrimSpace(t)
		if t != "" && len(t) <= 60 && !gezien[strings.ToLower(t)] && len(trefwoorden) < 20 {
			gezien[strings.ToLower(t)] = true
			trefwoorden = append(trefwoorden, t)
		}
	}
	if err := a.Reg.Metadata(r.Context(), p.Ruimte.ID, id, titel, omschrijving, soort, trefwoorden, s.Email); err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	terug(w, r, doel, "melding", "Bewaard.")
}

// item: het item uit de URL, met de map waar het in staat.
func (a *App) item(w http.ResponseWriter, r *http.Request, s *Sessie, p *Plek) *Item {
	x, err := a.Reg.Item(r.Context(), p.Ruimte.ID, padID(r))
	if errors.Is(err, errNietGevonden) {
		a.fout(w, r, s, http.StatusNotFound, "Dit bestaat niet (meer).")
		return nil
	}
	if err != nil {
		a.interneFout(w, r, s, err)
		return nil
	}
	return x
}

func (a *App) hernoem(w http.ResponseWriter, r *http.Request, s *Sessie, p *Plek) {
	x := a.item(w, r, s, p)
	if x == nil || !a.mag(w, r, s, p, x.OuderID.Int64, Bewerken) {
		return
	}
	doel := veiligTerug(r.FormValue("terug"))
	naam, err := geldigeNaam(r.FormValue("naam"))
	if err != nil {
		terug(w, r, doel, "fout", err.Error())
		return
	}
	err = a.Reg.Hernoem(r.Context(), p.Ruimte.ID, x.ID, naam, s.Email)
	switch {
	case errors.Is(err, errBestaatAl):
		terug(w, r, doel, "fout", "Hier staat al iets met de naam "+naam+".")
	case err != nil:
		a.interneFout(w, r, s, err)
	default:
		terug(w, r, doel, "melding", "Hernoemd naar "+naam+".")
	}
}

func (a *App) verwijder(w http.ResponseWriter, r *http.Request, s *Sessie, p *Plek) {
	x := a.item(w, r, s, p)
	if x == nil || !a.mag(w, r, s, p, x.OuderID.Int64, Beheren) {
		return
	}
	// Een map met alles erin: ook in elke map eronder moet je Beheren hebben
	// (een besloten map eronder verwijder je niet zomaar).
	if x.Soort == "map" {
		var controleer func(id int64) bool
		controleer = func(id int64) bool {
			if p.Boom.Niveau(id) < Beheren {
				return false
			}
			for _, k := range p.Boom.kinderen[id] {
				if !controleer(k) {
					return false
				}
			}
			return true
		}
		if !controleer(x.ID) {
			terug(w, r, mapURL(p.Ruimte, x.ID), "fout", "Je kunt deze map niet verwijderen: in een map eronder heb je niet het recht Beheren.")
			return
		}
	}
	if err := a.Reg.Verwijder(r.Context(), p.Ruimte.ID, x.ID, s.Email); err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	terug(w, r, mapURL(p.Ruimte, x.OuderID.Int64), "melding", x.Naam+" staat in de prullenbak. De beheerder kan het terugzetten.")
}

// toegang: regels toevoegen, wijzigen, weghalen; overerving stoppen of herstellen.
func (a *App) toegang(w http.ResponseWriter, r *http.Request, s *Sessie, p *Plek) {
	mapID, _ := strconv.ParseInt(r.FormValue("map"), 10, 64)
	if !a.mag(w, r, s, p, mapID, Beheren) {
		return
	}
	doel := mapURL(p.Ruimte, mapID) + "/toegang"
	if mapID == 0 {
		doel = "/r/" + p.Ruimte.Sleutel + "/toegang"
	}
	var err error
	melding := ""
	switch r.FormValue("actie") {
	case "toevoegen":
		soort, wie, _ := strings.Cut(r.FormValue("wie"), "|")
		niveau, _ := strconv.Atoi(r.FormValue("niveau"))
		if soort == "rol" && (p.Ruimte.Soort != "project" || wie != "redmine") || soort == "groep" && wie == groepBeheer {
			err = errOngeldig
		} else if soort == "persoon" {
			wie = strings.ToLower(strings.TrimSpace(wie))
		}
		if err == nil {
			err = a.Reg.RegelToevoegen(r.Context(), p.Ruimte, mapID, soort, wie, niveau, s.Email)
			melding = "Toegang bijgewerkt."
		}
	case "weg":
		id, _ := strconv.ParseInt(r.FormValue("regel"), 10, 64)
		err = a.Reg.RegelWeg(r.Context(), p.Ruimte, mapID, id, s.Email)
		melding = "Toegang ingetrokken."
	case "stoppen":
		err = a.Reg.OverervingStoppen(r.Context(), p.Boom, mapID, s.Email)
		melding = "De overerving is gestopt. De regels van boven staan nu als eigen regels bij deze map; haal weg wie er niet bij hoort."
	case "herstellen":
		err = a.Reg.OverervingHerstellen(r.Context(), p.Ruimte, mapID, s.Email)
		melding = "Deze map neemt de toegang van boven weer over (de eigen regels blijven erbij)."
	default:
		err = errOngeldig
	}
	switch {
	case errors.Is(err, errOngeldig):
		terug(w, r, doel, "fout", "Kies wie en welk recht.")
	case errors.Is(err, errNietGevonden):
		terug(w, r, doel, "fout", "Die regel bestaat niet (meer).")
	case err != nil:
		a.interneFout(w, r, s, err)
	default:
		// Heeft iemand zichzelf buitengesloten? Zeg het.
		b, lerr := a.Reg.laadBoom(r.Context(), p.Ruimte, s)
		if lerr == nil && b.Niveau(mapID) < Beheren {
			melding += " Let op: je hebt hier zelf nu het recht " + niveauNaam(b.Niveau(mapID)) + "."
			if b.Niveau(mapID) < Lezen {
				doel = "/r/" + p.Ruimte.Sleutel
			}
		}
		terug(w, r, doel, "melding", melding)
	}
}
