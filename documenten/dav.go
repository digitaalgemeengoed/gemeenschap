package main

// WebDAV: dezelfde ruimtes, mappen en documenten als in de browser, voor
// Verkenner, Finder, Word, LibreOffice, Cyberduck en rclone.
//
//	/dav/                    de ruimtes die je ziet (alleen lezen)
//	/dav/Mijn documenten/    je eigen ruimte (zo nodig aangemaakt)
//	/dav/<ruimte>/<pad>      mappen en documenten
//	/o/<sleutel>/<naam>      één document, met een tijdelijke link (openen in Word)
//
// Inloggen op /dav/ met je e-mailadres en een app-wachtwoord (Basic, alleen
// over https). De rechten zijn dezelfde als in de browser, met je groepen live
// uit Keycloak (groepen.go).
//
// Opslaan maakt een nieuwe versie. Slaat dezelfde persoon binnen tien minuten
// opnieuw op, dan vervangt dat de laatste versie (Word en LibreOffice slaan
// tussentijds op). Bestanden die besturingssystemen en programma's zelf
// neerzetten (._*, .DS_Store, Thumbs.db, desktop.ini, ~$*, .~lock.*), komen
// niet in het register: die bewaart de dienst even in het geheugen.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/digitaalgemeengoed/gemeenschap/documenten/internal/webdav"
)

// --- Wie, binnen één verzoek ------------------------------------------------------

type davSleutel struct{}

type davVerzoek struct {
	s      *Sessie
	booms  map[int64]*Boom
	alleen *OpenLink // bij een tijdelijke link: alleen dit document

	eigen        *Ruimte // Mijn documenten (nil: geen)
	eigenGezocht bool
}

// mijnNaam: zo heet je eigen ruimte in WebDAV (de sleutel mijn-… werkt ook).
const mijnNaam = "Mijn documenten"

func davVan(ctx context.Context) *davVerzoek {
	v, _ := ctx.Value(davSleutel{}).(*davVerzoek)
	return v
}

// --- Vluchtige bestanden ------------------------------------------------------------

func isVluchtig(naam string) bool {
	l := strings.ToLower(naam)
	return strings.HasPrefix(naam, "._") || l == ".ds_store" || l == "thumbs.db" || l == "desktop.ini" ||
		strings.HasPrefix(naam, "~$") || strings.HasPrefix(naam, ".~lock.") || l == ".localized"
}

type vluchtigBestand struct {
	data []byte
	tijd time.Time
}

type Vluchtig struct {
	mu sync.Mutex
	m  map[string]*vluchtigBestand
}

func (v *Vluchtig) sleutel(ruimte, ouder int64, naam string) string {
	return fmt.Sprintf("%d/%d/%s", ruimte, ouder, strings.ToLower(naam))
}

func (v *Vluchtig) haal(k string) *vluchtigBestand {
	v.mu.Lock()
	defer v.mu.Unlock()
	for s, b := range v.m { // opruimen: twee uur
		if time.Since(b.tijd) > 2*time.Hour {
			delete(v.m, s)
		}
	}
	return v.m[k]
}

func (v *Vluchtig) zet(k string, data []byte) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.m == nil {
		v.m = map[string]*vluchtigBestand{}
	}
	v.m[k] = &vluchtigBestand{data: data, tijd: time.Now()}
}

func (v *Vluchtig) weg(k string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, ok := v.m[k]
	delete(v.m, k)
	return ok
}

// --- Recent: voor "veilig opslaan" ------------------------------------------------
//
// Windows, Word en andere programma's slaan vaak op als: schrijf een tijdelijk
// bestand, hernoem of verwijder het origineel, hernoem het tijdelijke bestand
// naar de oude naam. Om de geschiedenis bij één document te houden onthoudt de
// dienst twee minuten welk document een naam net verliet; krijgt een ander
// document die naam, dan wordt dat een nieuwe versie van het oude.

type recentNaam struct {
	doc  int64
	wie  string
	tijd time.Time
}

type Recent struct {
	mu        sync.Mutex
	verlaten  map[string]recentNaam // ruimte/ouder/naam → document dat die naam had
	opgenomen map[string]time.Time  // namen die door opnemen verdwenen (verwijderen = gelukt)
}

const recentDuur = 2 * time.Minute

func recentSleutel(ruimte, ouder int64, naam string) string {
	return fmt.Sprintf("%d/%d/%s", ruimte, ouder, strings.ToLower(naam))
}

func (r *Recent) verlaat(ruimte, ouder int64, naam string, doc int64, wie string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.verlaten == nil {
		r.verlaten, r.opgenomen = map[string]recentNaam{}, map[string]time.Time{}
	}
	r.verlaten[recentSleutel(ruimte, ouder, naam)] = recentNaam{doc, wie, time.Now()}
}

func (r *Recent) zoek(ruimte, ouder int64, naam, wie string) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := recentSleutel(ruimte, ouder, naam)
	x, ok := r.verlaten[k]
	if !ok || x.wie != wie || time.Since(x.tijd) > recentDuur {
		return 0
	}
	delete(r.verlaten, k)
	return x.doc
}

func (r *Recent) neemOp(ruimte, ouder int64, naam string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.opgenomen == nil {
		r.verlaten, r.opgenomen = map[string]recentNaam{}, map[string]time.Time{}
	}
	r.opgenomen[recentSleutel(ruimte, ouder, naam)] = time.Now()
}

func (r *Recent) vergeetOpgenomen(ruimte, ouder int64, naam string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.opgenomen, recentSleutel(ruimte, ouder, naam))
}

func (r *Recent) wasOpgenomen(ruimte, ouder int64, naam string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.opgenomen[recentSleutel(ruimte, ouder, naam)]
	return ok && time.Since(t) < recentDuur
}

// --- Het bestandssysteem -----------------------------------------------------------

type DavFS struct {
	a        *App
	vluchtig *Vluchtig
	recent   *Recent
	props    *DodeProps
}

// doodVoor: de sleutel voor de dode eigenschappen van een plek.
func (fs *DavFS) doodVoor(p *plek) dood {
	switch {
	case p.rm == nil:
		return dood{fs.props, "/"}
	case p.vluchtig:
		return dood{fs.props, fs.vluchtig.sleutel(p.rm.ID, p.ouder, p.naam)}
	default:
		return dood{fs.props, fmt.Sprintf("%d#%d", p.rm.ID, p.id)}
	}
}

// plek: waar een pad naar wijst.
type plek struct {
	rm       *Ruimte // nil = de lijst met ruimtes
	b        *Boom
	id       int64  // het item (0 = de ruimte zelf)
	ouder    int64  // de map waarin het staat (of zou komen)
	naam     string // de laatste naam in het pad
	bestaat  bool
	vluchtig bool
}

func (fs *DavFS) boom(ctx context.Context, rm *Ruimte) (*Boom, error) {
	v := davVan(ctx)
	if b := v.booms[rm.ID]; b != nil {
		return b, nil
	}
	b, err := fs.a.Reg.laadBoom(ctx, rm, v.s)
	if err == nil {
		v.booms[rm.ID] = b
	}
	return b, err
}

// vergeet: na een wijziging de boom opnieuw laden.
func (fs *DavFS) vergeet(ctx context.Context, rm *Ruimte) { delete(davVan(ctx).booms, rm.ID) }

// eigen: de persoonlijke ruimte van wie er is, aangemaakt als hij er recht op
// heeft (zoals bij de eerste klik op Mijn documenten in de browser). Nil: geen.
func (fs *DavFS) eigen(ctx context.Context) (*Ruimte, error) {
	v := davVan(ctx)
	if v.eigenGezocht {
		return v.eigen, nil
	}
	rm, err := fs.a.Reg.eigenRuimte(ctx, v.s.Email)
	if err != nil {
		return nil, err
	}
	if rm == nil || rm.Status == "weg" {
		mag, err := fs.a.magPersoonlijk(ctx, v.s)
		if err != nil {
			return nil, err
		}
		rm = nil
		if mag {
			if rm, err = fs.a.Reg.PersoonlijkeRuimte(ctx, v.s.Email, fs.a.QuotumPersoonlijk); err != nil {
				return nil, err
			}
		}
	}
	v.eigen, v.eigenGezocht = rm, true
	return rm, nil
}

// isEigen: is dit de persoonlijke ruimte van wie er is?
func isEigen(rm *Ruimte, s *Sessie) bool {
	return rm != nil && rm.Soort == "persoonlijk" && strings.EqualFold(rm.Eigenaar.String, s.Email)
}

// ruimteVan: het eerste deel van een pad; "Mijn documenten" is je eigen ruimte.
func (fs *DavFS) ruimteVan(ctx context.Context, naam string) (*Ruimte, error) {
	if strings.EqualFold(naam, mijnNaam) {
		rm, err := fs.eigen(ctx)
		if err == nil && rm == nil {
			err = os.ErrNotExist
		}
		return rm, err
	}
	rm, err := fs.a.Reg.Ruimte(ctx, naam)
	if errors.Is(err, errNietGevonden) {
		return nil, os.ErrNotExist
	}
	return rm, err
}

func delenVan(naam string) []string {
	naam = strings.Trim(path.Clean("/"+naam), "/")
	if naam == "" {
		return nil
	}
	return strings.Split(naam, "/")
}

// zoek: zet een pad om in een plek. Het laatste deel mag ontbreken (dan
// bestaat=false, voor aanmaken); een ontbrekende map onderweg is een fout.
func (fs *DavFS) zoek(ctx context.Context, naam string) (*plek, error) {
	v := davVan(ctx)
	d := delenVan(naam)
	if v.alleen != nil {
		return fs.zoekAlleen(ctx, d)
	}
	if len(d) == 0 {
		return &plek{bestaat: true}, nil
	}
	rm, err := fs.ruimteVan(ctx, d[0])
	if err != nil {
		return nil, err
	}
	b, err := fs.boom(ctx, rm)
	if err != nil {
		return nil, err
	}
	if !b.Zichtbaar(0) {
		return nil, os.ErrNotExist
	}
	p := &plek{rm: rm, b: b, bestaat: true}
	for i, deel := range d[1:] {
		laatste := i == len(d)-2
		if !p.bestaat || (p.id != 0 && b.IsDoc(p.id)) {
			return nil, os.ErrNotExist
		}
		ouder := p.id
		id := b.Zoek(ouder, deel)
		zichtbaar := id != 0 && ((b.IsDoc(id) && b.Niveau(id) >= Lezen) || (!b.IsDoc(id) && b.Zichtbaar(id)))
		if id != 0 && !zichtbaar && b.IsDoc(id) && laatste {
			// Bestaat, maar je mag het niet zien: niet overschrijven, niet aanmaken.
			return nil, os.ErrPermission
		}
		if !zichtbaar {
			if !laatste {
				return nil, os.ErrNotExist
			}
			p = &plek{rm: rm, b: b, ouder: ouder, naam: deel, vluchtig: isVluchtig(deel)}
			if p.vluchtig && fs.vluchtig.haal(fs.vluchtig.sleutel(rm.ID, ouder, deel)) != nil {
				p.bestaat = true
			}
			return p, nil
		}
		p = &plek{rm: rm, b: b, id: id, ouder: ouder, naam: b.mappen[id].Naam, bestaat: true}
	}
	if len(d) == 1 {
		p.naam = rm.Sleutel
		if strings.EqualFold(d[0], mijnNaam) {
			p.naam = mijnNaam
		}
	}
	return p, nil
}

// zoekAlleen: bij een tijdelijke link is er één map (de sleutel) met één document.
func (fs *DavFS) zoekAlleen(ctx context.Context, d []string) (*plek, error) {
	l := davVan(ctx).alleen
	rm, err := fs.a.Reg.RuimteVanID(ctx, l.RuimteID)
	if err != nil {
		return nil, os.ErrNotExist
	}
	b, err := fs.boom(ctx, rm)
	if err != nil {
		return nil, err
	}
	m := b.mappen[l.ItemID]
	if m == nil || !m.Doc || b.Niveau(l.ItemID) < Lezen {
		return nil, os.ErrNotExist
	}
	switch {
	case len(d) <= 1: // de lijst, of de map van de sleutel
		return &plek{bestaat: true, naam: "document"}, nil
	case len(d) == 2 && strings.EqualFold(d[1], m.Naam):
		return &plek{rm: rm, b: b, id: l.ItemID, ouder: m.Ouder, naam: m.Naam, bestaat: true}, nil
	case len(d) == 2 && isVluchtig(d[1]):
		p := &plek{rm: rm, b: b, ouder: m.Ouder, naam: d[1], vluchtig: true}
		p.bestaat = fs.vluchtig.haal(fs.vluchtig.sleutel(rm.ID, m.Ouder, d[1])) != nil
		return p, nil
	}
	return nil, os.ErrPermission
}

// inDeLijst: in /dav/ zelf staan alleen de ruimtes. Iets neerzetten dat geen
// ruimte is, is "geen toegang" (403), niet "bestaat niet" (409 Conflict, wat
// een bestandsbeheer als onbegrijpelijke fout toont).
func inDeLijst(naam string, err error) error {
	if errors.Is(err, os.ErrNotExist) && len(delenVan(naam)) == 1 {
		return os.ErrPermission
	}
	return err
}

func (p *plek) isMap() bool {
	return p.rm == nil || p.id == 0 || !p.b.IsDoc(p.id)
}

func (fs *DavFS) Stat(ctx context.Context, naam string) (os.FileInfo, error) {
	p, err := fs.zoek(ctx, naam)
	if err != nil {
		return nil, err
	}
	if !p.bestaat {
		// De reservekopie van veilig opslaan is al opgenomen; laat het
		// programma denken dat hij er nog is, zodat verwijderen lukt.
		if p.rm != nil && fs.recent.wasOpgenomen(p.rm.ID, p.ouder, p.naam) {
			return &davInfo{naam: p.naam, tijd: time.Now()}, nil
		}
		return nil, os.ErrNotExist
	}
	return fs.info(ctx, p)
}

func (fs *DavFS) info(ctx context.Context, p *plek) (os.FileInfo, error) {
	if p.vluchtig {
		b := fs.vluchtig.haal(fs.vluchtig.sleutel(p.rm.ID, p.ouder, p.naam))
		if b == nil {
			return nil, os.ErrNotExist
		}
		return &davInfo{naam: p.naam, grootte: int64(len(b.data)), tijd: b.tijd}, nil
	}
	if p.rm == nil || p.id == 0 {
		return &davInfo{naam: p.naam, map_: true, tijd: time.Now()}, nil
	}
	x, err := fs.a.Reg.Item(ctx, p.rm.ID, p.id)
	if err != nil {
		return nil, os.ErrNotExist
	}
	return itemInfo(x, fs.a.Reg, p.rm.ID), nil
}

func itemInfo(x *Item, reg *Register, ruimte int64) *davInfo {
	return &davInfo{naam: x.Naam, map_: x.Soort == "map", grootte: x.Grootte, tijd: x.Gewijzigd, mediatype: x.Mediatype,
		reg: reg, ruimte: ruimte, id: x.ID, versie: x.HuidigeVersie}
}

func (fs *DavFS) Mkdir(ctx context.Context, naam string, _ os.FileMode) error {
	p, err := fs.zoek(ctx, naam)
	if err != nil {
		return inDeLijst(naam, err)
	}
	if p.bestaat {
		return os.ErrExist
	}
	if p.rm == nil || davVan(ctx).alleen != nil || p.vluchtig {
		return os.ErrPermission
	}
	if p.b.Niveau(p.ouder) < Bewerken {
		return os.ErrPermission
	}
	n, err := geldigeNaam(p.naam)
	if err != nil {
		return os.ErrInvalid
	}
	_, err = fs.a.Reg.NieuweMap(ctx, p.rm.ID, p.ouder, n, davVan(ctx).s.Email)
	fs.vergeet(ctx, p.rm)
	if errors.Is(err, errBestaatAl) {
		return os.ErrExist
	}
	return err
}

func (fs *DavFS) OpenFile(ctx context.Context, naam string, flag int, _ os.FileMode) (webdav.File, error) {
	// PROPPATCH opent met alleen O_RDWR: dat is geen schrijven van inhoud.
	schrijven := flag&(os.O_WRONLY|os.O_CREATE|os.O_TRUNC) != 0
	p, err := fs.zoek(ctx, naam)
	if err != nil {
		if schrijven {
			return nil, inDeLijst(naam, err)
		}
		return nil, err
	}
	if !schrijven {
		if !p.bestaat {
			return nil, os.ErrNotExist
		}
		// Eigenschappen zetten (PROPPATCH) vraagt Bewerken.
		if flag&os.O_RDWR != 0 && p.rm != nil && !p.vluchtig && p.b.Niveau(p.id) < Bewerken {
			return nil, os.ErrPermission
		}
		if p.vluchtig {
			b := fs.vluchtig.haal(fs.vluchtig.sleutel(p.rm.ID, p.ouder, p.naam))
			if b == nil {
				return nil, os.ErrNotExist
			}
			return &davGeheugen{dood: fs.doodVoor(p), Reader: bytes.NewReader(b.data), info: &davInfo{naam: p.naam, grootte: int64(len(b.data)), tijd: b.tijd}}, nil
		}
		if p.isMap() {
			return fs.openMap(ctx, p)
		}
		x, err := fs.a.Reg.Item(ctx, p.rm.ID, p.id)
		if err != nil {
			return nil, os.ErrNotExist
		}
		v, err := fs.a.Reg.Versie(ctx, x.ID, x.HuidigeVersie)
		if err != nil {
			return nil, os.ErrNotExist
		}
		return &davLezer{dood: fs.doodVoor(p), fs: fs, object: v.Object, grootte: v.Grootte, info: itemInfo(x, fs.a.Reg, p.rm.ID)}, nil
	}
	// Schrijven
	if p.rm == nil || (p.bestaat && p.isMap() && !p.vluchtig) {
		return nil, os.ErrPermission
	}
	if p.vluchtig {
		return &davSchrijver{naam: p.naam, klaar: func(data []byte, _ *os.File, _ int64, _ string) error {
			if len(data) > 1<<20 {
				return errTeGroot
			}
			fs.vluchtig.zet(fs.vluchtig.sleutel(p.rm.ID, p.ouder, p.naam), data)
			return nil
		}, geheugen: true}, nil
	}
	nodig := p.ouder
	if p.bestaat {
		nodig = p.id
	}
	if p.b.Niveau(nodig) < Bewerken {
		return nil, os.ErrPermission
	}
	n, err := geldigeNaam(p.naam)
	if err != nil {
		return nil, os.ErrInvalid
	}
	s := davVan(ctx).s
	rm, ouder := p.rm, p.ouder
	tmp, err := os.CreateTemp("", "dav-*")
	if err != nil {
		return nil, err
	}
	return &davSchrijver{naam: n, tmp: tmp, max: fs.a.MaxGrootte, klaar: func(_ []byte, f *os.File, lengte int64, som string) error {
		mt := mime.TypeByExtension(strings.ToLower(path.Ext(n)))
		if mt == "" {
			kop := make([]byte, 512)
			k, _ := f.ReadAt(kop, 0)
			mt = http.DetectContentType(kop[:k])
		}
		if err := fs.a.Reg.PastErBij(ctx, rm, lengte); err != nil {
			return err
		}
		object := objectSleutel(rm)
		if err := fs.a.Opslag.Zet(ctx, object, f, lengte, som, mt); err != nil {
			return err
		}
		_, _, err := fs.a.Reg.nieuweVersie(ctx, rm.ID, ouder, n, s.Email, Versie{Object: object, SHA256: som, Grootte: lengte, Mediatype: mt}, true)
		fs.vergeet(ctx, rm)
		return err
	}}, nil
}

func (fs *DavFS) openMap(ctx context.Context, p *plek) (webdav.File, error) {
	m := &davMap{dood: fs.doodVoor(p), info: &davInfo{naam: p.naam, map_: true, tijd: time.Now()}}
	v := davVan(ctx)
	switch {
	case v.alleen != nil:
		rm, err := fs.a.Reg.RuimteVanID(ctx, v.alleen.RuimteID)
		if err != nil {
			return nil, os.ErrNotExist
		}
		if x, err := fs.a.Reg.Item(ctx, rm.ID, v.alleen.ItemID); err == nil {
			m.kinderen = append(m.kinderen, itemInfo(x, fs.a.Reg, rm.ID))
		}
	case p.rm == nil:
		if _, err := fs.eigen(ctx); err != nil { // zo nodig aanmaken
			return nil, err
		}
		alle, err := fs.a.Reg.Ruimtes(ctx)
		if err != nil {
			return nil, err
		}
		for i := range alle {
			b, err := fs.boom(ctx, &alle[i])
			if err != nil {
				return nil, err
			}
			if b.Zichtbaar(0) {
				naam := alle[i].Sleutel
				if isEigen(&alle[i], v.s) {
					naam = mijnNaam
				}
				m.kinderen = append(m.kinderen, &davInfo{naam: naam, map_: true, tijd: time.Now()})
			}
		}
	default:
		if p.b.Niveau(p.id) < Lezen && !p.b.Zichtbaar(p.id) {
			return nil, os.ErrNotExist
		}
		inhoud, err := fs.a.Reg.Inhoud(ctx, p.rm.ID, p.id)
		if err != nil {
			return nil, err
		}
		for _, x := range inhoud {
			if (x.Soort == "map" && p.b.Zichtbaar(x.ID)) || (x.Soort == "document" && p.b.Niveau(x.ID) >= Lezen) {
				m.kinderen = append(m.kinderen, itemInfo(x, fs.a.Reg, p.rm.ID))
			}
		}
	}
	return m, nil
}

func (fs *DavFS) RemoveAll(ctx context.Context, naam string) error {
	if davVan(ctx).alleen != nil {
		p, err := fs.zoek(ctx, naam)
		if err == nil && p.vluchtig {
			fs.vluchtig.weg(fs.vluchtig.sleutel(p.rm.ID, p.ouder, p.naam))
			return nil
		}
		return os.ErrPermission
	}
	p, err := fs.zoek(ctx, naam)
	if err != nil {
		return err
	}
	if p.rm == nil || p.id == 0 && !p.vluchtig && p.bestaat {
		return os.ErrPermission // de lijst of een hele ruimte: niet via WebDAV
	}
	if p.vluchtig {
		if !fs.vluchtig.weg(fs.vluchtig.sleutel(p.rm.ID, p.ouder, p.naam)) {
			return os.ErrNotExist
		}
		return nil
	}
	if !p.bestaat {
		if fs.recent.wasOpgenomen(p.rm.ID, p.ouder, p.naam) {
			fs.recent.vergeetOpgenomen(p.rm.ID, p.ouder, p.naam)
			return nil // de reservekopie van veilig opslaan: al verwerkt
		}
		return os.ErrNotExist
	}
	b := p.b
	var mag func(id int64) bool
	mag = func(id int64) bool {
		if b.Niveau(id) < Beheren {
			return false
		}
		for _, k := range b.kinderen[id] {
			if !mag(k) {
				return false
			}
		}
		for _, d := range b.docs[id] {
			if b.Niveau(d) < Beheren {
				return false
			}
		}
		return true
	}
	// Verwijderen vraagt Beheren. Uitzondering: een document dat je zelf in
	// de afgelopen tien minuten hebt aangemaakt (tijdelijke bestanden van
	// programma's), daarvoor is Bewerken genoeg.
	s := davVan(ctx).s
	if !mag(p.id) {
		x, err := fs.a.Reg.Item(ctx, p.rm.ID, p.id)
		if err != nil || x.Soort != "document" || b.Niveau(p.id) < Bewerken || x.AangemaaktDoor != s.Email || time.Since(x.Aangemaakt) > 10*time.Minute {
			return os.ErrPermission
		}
	}
	if b.IsDoc(p.id) {
		fs.recent.verlaat(p.rm.ID, p.ouder, p.naam, p.id, s.Email)
	}
	err = fs.a.Reg.Verwijder(ctx, p.rm.ID, p.id, s.Email)
	fs.vergeet(ctx, p.rm)
	return err
}

func (fs *DavFS) Rename(ctx context.Context, oud, nieuw string) error {
	if davVan(ctx).alleen != nil {
		return os.ErrPermission
	}
	van, err := fs.zoek(ctx, oud)
	if err != nil {
		return err
	}
	naar, err := fs.zoek(ctx, nieuw)
	if err != nil {
		return err
	}
	if !van.bestaat || van.rm == nil || van.id == 0 && !van.vluchtig {
		return os.ErrPermission
	}
	if naar.rm == nil || naar.rm.ID != van.rm.ID {
		return os.ErrPermission // niet tussen ruimtes: kopieer dan
	}
	if naar.bestaat {
		return os.ErrExist // webdav heeft het doel bij Overwrite al verwijderd
	}
	if van.vluchtig || naar.vluchtig {
		if van.vluchtig && naar.vluchtig {
			k := fs.vluchtig.sleutel(van.rm.ID, van.ouder, van.naam)
			if x := fs.vluchtig.haal(k); x != nil {
				fs.vluchtig.zet(fs.vluchtig.sleutel(naar.rm.ID, naar.ouder, naar.naam), x.data)
				fs.vluchtig.weg(k)
				return nil
			}
		}
		return os.ErrPermission
	}
	b := van.b
	if b.Niveau(van.id) < Bewerken || b.Niveau(naar.ouder) < Bewerken {
		return os.ErrPermission
	}
	n, err := geldigeNaam(naar.naam)
	if err != nil {
		return os.ErrInvalid
	}
	s := davVan(ctx).s
	defer fs.vergeet(ctx, van.rm)
	if b.IsDoc(van.id) {
		// Had een ander document deze naam net? Dan is dit een nieuwe versie daarvan.
		if doel := fs.recent.zoek(van.rm.ID, naar.ouder, n, s.Email); doel != 0 && doel != van.id {
			tijdelijk := b.mappen[doel]
			if err := fs.a.Reg.Opnemen(ctx, van.rm.ID, van.id, doel, naar.ouder, n, s.Email); err == nil {
				// Het doel had tijdelijk een andere naam (de reservekopie van
				// veilig opslaan); die naam is nu weg, en verwijderen ervan is
				// dus al gelukt.
				if tijdelijk != nil && !strings.EqualFold(tijdelijk.Naam, n) {
					fs.recent.neemOp(van.rm.ID, tijdelijk.Ouder, tijdelijk.Naam)
				}
				return nil
			}
		}
		fs.recent.verlaat(van.rm.ID, van.ouder, van.naam, van.id, s.Email)
	}
	err = fs.a.Reg.Verplaats(ctx, van.rm.ID, van.id, naar.ouder, n, s.Email)
	switch {
	case errors.Is(err, errBestaatAl):
		return os.ErrExist
	case errors.Is(err, errOngeldig):
		return os.ErrInvalid
	}
	return err
}

// --- Bestanden en informatie ------------------------------------------------------

type davInfo struct {
	naam      string
	map_      bool
	grootte   int64
	tijd      time.Time
	mediatype string
	reg       *Register
	ruimte    int64
	id        int64
	versie    int
}

func (i *davInfo) Name() string       { return i.naam }
func (i *davInfo) Size() int64        { return i.grootte }
func (i *davInfo) ModTime() time.Time { return i.tijd }
func (i *davInfo) IsDir() bool        { return i.map_ }
func (i *davInfo) Sys() any           { return nil }
func (i *davInfo) Mode() fs.FileMode {
	if i.map_ {
		return fs.ModeDir | 0o755
	}
	return 0o644
}

func (i *davInfo) ContentType(ctx context.Context) (string, error) {
	if i.map_ || i.mediatype == "" {
		return "", webdav.ErrNotImplemented
	}
	return i.mediatype, nil
}

// ETag: het controlegetal van de huidige versie.
func (i *davInfo) ETag(ctx context.Context) (string, error) {
	if i.map_ || i.reg == nil {
		return "", webdav.ErrNotImplemented
	}
	v, err := i.reg.Versie(ctx, i.id, i.versie)
	if err != nil {
		return "", webdav.ErrNotImplemented
	}
	return `"` + v.SHA256 + `"`, nil
}

// davMap: een map met de inhoud die je mag zien.
type davMap struct {
	dood
	info     *davInfo
	kinderen []os.FileInfo
	pos      int
}

func (m *davMap) Close() error                   { return nil }
func (m *davMap) Read([]byte) (int, error)       { return 0, os.ErrInvalid }
func (m *davMap) Write([]byte) (int, error)      { return 0, os.ErrPermission }
func (m *davMap) Seek(int64, int) (int64, error) { return 0, nil }
func (m *davMap) Stat() (os.FileInfo, error)     { return m.info, nil }
func (m *davMap) Readdir(n int) ([]os.FileInfo, error) {
	if n <= 0 {
		rest := m.kinderen[m.pos:]
		m.pos = len(m.kinderen)
		return rest, nil
	}
	if m.pos >= len(m.kinderen) {
		return nil, io.EOF
	}
	eind := min(m.pos+n, len(m.kinderen))
	rest := m.kinderen[m.pos:eind]
	m.pos = eind
	return rest, nil
}

// davLezer: leest een object uit de opslag, vanaf waar gevraagd (Range).
type davLezer struct {
	dood
	fs      *DavFS
	object  string
	grootte int64
	pos     int64
	rc      io.ReadCloser
	info    *davInfo
}

func (l *davLezer) Read(p []byte) (int, error) {
	if l.pos >= l.grootte {
		return 0, io.EOF
	}
	if l.rc == nil {
		rc, err := l.fs.a.Opslag.HaalVanaf(context.Background(), l.object, l.pos)
		if err != nil {
			return 0, err
		}
		l.rc = rc
	}
	n, err := l.rc.Read(p)
	l.pos += int64(n)
	return n, err
}

func (l *davLezer) Seek(offset int64, waar int) (int64, error) {
	nieuw := offset
	switch waar {
	case io.SeekCurrent:
		nieuw = l.pos + offset
	case io.SeekEnd:
		nieuw = l.grootte + offset
	}
	if nieuw < 0 {
		return 0, os.ErrInvalid
	}
	if nieuw != l.pos && l.rc != nil {
		l.rc.Close()
		l.rc = nil
	}
	l.pos = nieuw
	return nieuw, nil
}

func (l *davLezer) Close() error {
	if l.rc != nil {
		return l.rc.Close()
	}
	return nil
}
func (l *davLezer) Write([]byte) (int, error)          { return 0, os.ErrPermission }
func (l *davLezer) Readdir(int) ([]os.FileInfo, error) { return nil, os.ErrInvalid }
func (l *davLezer) Stat() (os.FileInfo, error)         { return l.info, nil }

// davGeheugen: een vluchtig bestand.
type davGeheugen struct {
	dood
	*bytes.Reader
	info *davInfo
}

func (g *davGeheugen) Close() error                       { return nil }
func (g *davGeheugen) Write([]byte) (int, error)          { return 0, os.ErrPermission }
func (g *davGeheugen) Readdir(int) ([]os.FileInfo, error) { return nil, os.ErrInvalid }
func (g *davGeheugen) Stat() (os.FileInfo, error)         { return g.info, nil }

// davSchrijver: wat binnenkomt gaat naar een tijdelijk bestand (met
// SHA-256); bij Close naar de opslag en het register.
type davSchrijver struct {
	naam     string
	tmp      *os.File
	geheugen bool
	buf      bytes.Buffer
	h        hashSchrijver
	n        int64
	max      int64
	fout     error
	klaar    func(data []byte, f *os.File, lengte int64, som string) error
}

type hashSchrijver struct {
	h interface {
		Write([]byte) (int, error)
		Sum([]byte) []byte
	}
}

func (w *davSchrijver) Write(p []byte) (int, error) {
	if w.h.h == nil {
		w.h.h = sha256.New()
	}
	w.n += int64(len(p))
	if w.max > 0 && w.n > w.max {
		w.fout = errTeGroot
		return 0, pastNiet(errTeGroot)
	}
	w.h.h.Write(p)
	if w.geheugen {
		if w.buf.Len()+len(p) > 1<<20 {
			w.fout = errTeGroot
			return 0, pastNiet(errTeGroot)
		}
		return w.buf.Write(p)
	}
	return w.tmp.Write(p)
}

// pastNiet: vol of te groot wordt 507 (Insufficient Storage), niet 405.
func pastNiet(err error) error {
	if errors.Is(err, errVol) || errors.Is(err, errTeGroot) {
		return fmt.Errorf("%w: %w", webdav.ErrInsufficientStorage, err)
	}
	return err
}

func (w *davSchrijver) Close() error {
	return pastNiet(w.sluit())
}

func (w *davSchrijver) sluit() error {
	if w.tmp != nil {
		defer os.Remove(w.tmp.Name())
		defer w.tmp.Close()
	}
	if w.fout != nil {
		return w.fout
	}
	if w.h.h == nil {
		w.h.h = sha256.New()
	}
	som := hex.EncodeToString(w.h.h.Sum(nil))
	if w.geheugen {
		return w.klaar(w.buf.Bytes(), nil, w.n, som)
	}
	return w.klaar(nil, w.tmp, w.n, som)
}

func (w *davSchrijver) Read([]byte) (int, error)           { return 0, os.ErrInvalid }
func (w *davSchrijver) Seek(int64, int) (int64, error)     { return 0, nil }
func (w *davSchrijver) Readdir(int) ([]os.FileInfo, error) { return nil, os.ErrInvalid }
func (w *davSchrijver) Stat() (os.FileInfo, error) {
	return &davInfo{naam: w.naam, grootte: w.n, tijd: time.Now()}, nil
}

// --- Dode eigenschappen (PROPPATCH) ----------------------------------------------
// Windows zet bij elk bestand tijden en kenmerken (Win32CreationTime …). Die
// bewaart de dienst in het geheugen, zodat het programma tevreden is.

type DodeProps struct {
	mu sync.Mutex
	m  map[string]map[xml.Name]webdav.Property
}

// dood: zit in elk bestand en elke map; k is de sleutel (ruimte/item of pad).
type dood struct {
	dp *DodeProps
	k  string
}

func (d dood) DeadProps() (map[xml.Name]webdav.Property, error) {
	if d.dp == nil {
		return nil, nil
	}
	d.dp.mu.Lock()
	defer d.dp.mu.Unlock()
	uit := map[xml.Name]webdav.Property{}
	for n, p := range d.dp.m[d.k] {
		uit[n] = p
	}
	return uit, nil
}

func (d dood) Patch(patches []webdav.Proppatch) ([]webdav.Propstat, error) {
	st := webdav.Propstat{Status: http.StatusOK}
	if d.dp == nil {
		return []webdav.Propstat{st}, nil
	}
	d.dp.mu.Lock()
	defer d.dp.mu.Unlock()
	if d.dp.m == nil {
		d.dp.m = map[string]map[xml.Name]webdav.Property{}
	}
	if d.dp.m[d.k] == nil {
		d.dp.m[d.k] = map[xml.Name]webdav.Property{}
	}
	for _, p := range patches {
		for _, x := range p.Props {
			st.Props = append(st.Props, webdav.Property{XMLName: x.XMLName})
			if p.Remove {
				delete(d.dp.m[d.k], x.XMLName)
			} else {
				d.dp.m[d.k][x.XMLName] = x
			}
		}
	}
	return []webdav.Propstat{st}, nil
}
