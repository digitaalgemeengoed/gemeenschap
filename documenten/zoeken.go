package main

// Zoeken, met OpenSearch (ZOEK_URL) als index.
//
// De index is een afgeleide: alles staat ook in het register en de opslag.
// Hij zit niet in de back-up; is hij weg (of verandert de opbouw: zoekIndex),
// dan bouwt de dienst hem vanzelf opnieuw op.
//
// Bijhouden: triggers in Postgres zetten elk gewijzigd item in zoek_wachtrij
// (ook alles onder een map die een andere naam of plek krijgt, of verwijderd
// wordt, en alles van een ruimte die verandert). Eén taak verwerkt de rij:
// weg uit de index wat weg is, anders het item (opnieuw) erin. De tekst uit
// een document (tekst.go) wordt alleen opnieuw gelezen bij een nieuwe versie
// (zoek_stand). Elke zes uur vergelijkt de dienst het register met
// zoek_stand, voor wat gemist is.
//
// Rechten: OpenSearch bepaalt nooit wat iemand ziet. De zoekvraag beperkt
// zich tot de ruimtes die je ziet; daarna controleert de dienst elk resultaat
// met dezelfde rechten als de verkenner (ook besloten mappen), en of het in
// het register nog bestaat. Een index die achterloopt, toont dus nooit iets
// wat je niet mag zien.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lib/pq"
)

// zoekIndex: verander het nummer als de opbouw (mapping) verandert; de dienst
// maakt dan een nieuwe index en vult die opnieuw.
const zoekIndex = "documenten-1"

type Zoeker struct {
	URL   string
	HTTP  *http.Client
	klaar atomic.Bool // de index bestaat en is bereikbaar
}

// --- OpenSearch --------------------------------------------------------------------

func (z *Zoeker) vraag(ctx context.Context, methode, pad string, lichaam any, doel any) (int, error) {
	var r io.Reader
	if lichaam != nil {
		b, err := json.Marshal(lichaam)
		if err != nil {
			return 0, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, methode, strings.TrimRight(z.URL, "/")+pad, r)
	if err != nil {
		return 0, err
	}
	if lichaam != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := z.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		return resp.StatusCode, fmt.Errorf("opensearch %s %s: %s: %.300s", methode, pad, resp.Status, data)
	}
	if doel != nil && resp.StatusCode < 300 {
		if err := json.Unmarshal(data, doel); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

var zoekOpbouw = map[string]any{
	"settings": map[string]any{
		"number_of_shards":   1,
		"number_of_replicas": 0, // één knooppunt
		"analysis": map[string]any{
			"filter": map[string]any{
				"nl_stop": map[string]any{"type": "stop", "stopwords": "_dutch_"},
				"nl_stam": map[string]any{"type": "stemmer", "language": "dutch"},
			},
			"analyzer": map[string]any{
				// Nederlands: "archiveren" vindt ook "archief"; é = e.
				"nl":     map[string]any{"tokenizer": "standard", "filter": []string{"lowercase", "asciifolding", "nl_stop", "nl_stam"}},
				"gewoon": map[string]any{"tokenizer": "standard", "filter": []string{"lowercase", "asciifolding"}},
			},
		},
	},
	"mappings": map[string]any{
		"dynamic": "strict",
		"properties": map[string]any{
			"ruimte_id":     map[string]any{"type": "long"},
			"ruimte":        map[string]any{"type": "keyword"},
			"item_id":       map[string]any{"type": "long"},
			"soort":         map[string]any{"type": "keyword"},
			"naam":          tekstVeld(true),
			"pad":           map[string]any{"type": "text", "analyzer": "gewoon"},
			"mappen":        map[string]any{"type": "long"}, // de mappen boven het item
			"titel":         tekstVeld(false),
			"omschrijving":  tekstVeld(false),
			"documentsoort": map[string]any{"type": "keyword"},
			"trefwoorden":   map[string]any{"type": "text", "analyzer": "nl", "fields": map[string]any{"kw": map[string]any{"type": "keyword"}}},
			"extensie":      map[string]any{"type": "keyword"},
			"mediatype":     map[string]any{"type": "keyword"},
			"grootte":       map[string]any{"type": "long"},
			"gewijzigd":     map[string]any{"type": "date"},
			"door":          map[string]any{"type": "keyword"},
			"door_naam":     map[string]any{"type": "text", "analyzer": "gewoon"},
			"versie":        map[string]any{"type": "integer"},
			"inhoud":        map[string]any{"type": "text", "analyzer": "nl", "index_options": "offsets", "fields": map[string]any{"gewoon": map[string]any{"type": "text", "analyzer": "gewoon", "index_options": "offsets"}}},
			"inhoud_status": map[string]any{"type": "keyword"}, // gelezen, geen, fout, te-groot
		},
	},
}

func tekstVeld(kw bool) map[string]any {
	v := map[string]any{"type": "text", "analyzer": "nl", "fields": map[string]any{"gewoon": map[string]any{"type": "text", "analyzer": "gewoon"}}}
	if kw {
		v["fields"].(map[string]any)["kw"] = map[string]any{"type": "keyword", "ignore_above": 512}
	}
	return v
}

// zorgVoorIndex: de index bestaat; nieuw = alles opnieuw in de wachtrij.
func (a *App) zorgVoorIndex(ctx context.Context) error {
	z := a.Zoeken
	code, err := z.vraag(ctx, http.MethodHead, "/"+zoekIndex, nil, nil)
	if err != nil {
		return err
	}
	if code == http.StatusOK {
		return nil
	}
	if _, err := z.vraag(ctx, http.MethodPut, "/"+zoekIndex, zoekOpbouw, nil); err != nil {
		return err
	}
	log.Printf("zoeken: index %s aangemaakt; alles gaat opnieuw in de index", zoekIndex)
	if _, err := a.Reg.db.ExecContext(ctx, `TRUNCATE zoek_stand;
		INSERT INTO zoek_wachtrij (item_id) SELECT id FROM item WHERE verwijderd_op IS NULL ON CONFLICT DO NOTHING`); err != nil {
		return err
	}
	// Oude indexen (een eerdere opbouw) opruimen.
	var oud []struct {
		Index string `json:"index"`
	}
	if _, err := z.vraag(ctx, http.MethodGet, "/_cat/indices/documenten-*?format=json", nil, &oud); err == nil {
		for _, o := range oud {
			if o.Index != zoekIndex && strings.HasPrefix(o.Index, "documenten-") {
				z.vraag(ctx, http.MethodDelete, "/"+url.PathEscape(o.Index), nil, nil)
				log.Printf("zoeken: oude index %s verwijderd", o.Index)
			}
		}
	}
	return nil
}

// --- Bijhouden ---------------------------------------------------------------------

// zoekBijhouden: draait zolang de dienst draait.
func (a *App) zoekBijhouden(ctx context.Context) {
	z := a.Zoeken
	gemeld := time.Time{}
	wacht := func(d time.Duration) bool {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(d):
			return true
		}
	}
	for {
		var gezond struct {
			Status string `json:"status"`
		}
		_, err := z.vraag(ctx, http.MethodGet, "/_cluster/health?wait_for_status=yellow&timeout=5s", nil, &gezond)
		if err == nil && gezond.Status != "red" && gezond.Status != "" {
			err = a.zorgVoorIndex(ctx)
		} else if err == nil {
			err = fmt.Errorf("status %q", gezond.Status)
		}
		if err == nil {
			break
		}
		z.klaar.Store(false)
		if time.Since(gemeld) > 10*time.Minute {
			log.Printf("zoeken: OpenSearch nog niet klaar (%v); opnieuw over 30 seconden", err)
			gemeld = time.Now()
		}
		if !wacht(30 * time.Second) {
			return
		}
	}
	z.klaar.Store(true)
	log.Printf("zoeken: OpenSearch klaar (%s)", zoekIndex)

	vergeleken := time.Time{}
	for {
		if time.Since(vergeleken) > 6*time.Hour {
			if err := a.Reg.zoekVergelijk(ctx); err != nil {
				log.Printf("zoeken: vergelijken: %v", err)
			}
			vergeleken = time.Now()
		}
		n, err := a.zoekVerwerk(ctx, 25)
		if err != nil {
			if time.Since(gemeld) > 10*time.Minute {
				log.Printf("zoeken: bijwerken: %v", err)
				gemeld = time.Now()
			}
			// Is de index weg (OpenSearch opnieuw begonnen)? Dan opnieuw opbouwen.
			if code, e := z.vraag(ctx, http.MethodHead, "/"+zoekIndex, nil, nil); e == nil && code == http.StatusNotFound {
				if e := a.zorgVoorIndex(ctx); e != nil {
					log.Printf("zoeken: index opnieuw: %v", e)
				}
			}
			if !wacht(30 * time.Second) {
				return
			}
			continue
		}
		if n == 0 && !wacht(3*time.Second) {
			return
		}
	}
}

// zoekVergelijk: wat in het register anders is dan in de index, in de wachtrij.
func (r *Register) zoekVergelijk(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO zoek_wachtrij (item_id)
		SELECT i.id FROM item i LEFT JOIN zoek_stand z ON z.item_id = i.id
		WHERE i.verwijderd_op IS NULL AND (z.item_id IS NULL OR z.versie <> i.huidige_versie)
		ON CONFLICT DO NOTHING;
		INSERT INTO zoek_wachtrij (item_id)
		SELECT z.item_id FROM zoek_stand z LEFT JOIN item i ON i.id = z.item_id
		WHERE i.id IS NULL OR i.verwijderd_op IS NOT NULL
		ON CONFLICT DO NOTHING`)
	return err
}

// zoekVerwerk: hooguit n items uit de wachtrij. Geeft het aantal terug.
func (a *App) zoekVerwerk(ctx context.Context, n int) (int, error) {
	rows, err := a.Reg.db.QueryContext(ctx, `SELECT item_id, sinds FROM zoek_wachtrij ORDER BY sinds LIMIT $1`, n)
	if err != nil {
		return 0, err
	}
	type rij struct {
		id    int64
		sinds time.Time
	}
	var rijen []rij
	for rows.Next() {
		var x rij
		if err := rows.Scan(&x.id, &x.sinds); err != nil {
			rows.Close()
			return 0, err
		}
		rijen = append(rijen, x)
	}
	rows.Close()
	for _, x := range rijen {
		if err := a.zoekItem(ctx, x.id); err != nil {
			return 0, fmt.Errorf("item %d: %w", x.id, err)
		}
		// Alleen weg als er intussen niets nieuws gebeurde (dan is sinds anders).
		if _, err := a.Reg.db.ExecContext(ctx, `DELETE FROM zoek_wachtrij WHERE item_id = $1 AND sinds = $2`, x.id, x.sinds); err != nil {
			return 0, err
		}
	}
	return len(rijen), nil
}

type zoekDoc struct {
	RuimteID      int64     `json:"ruimte_id"`
	Ruimte        string    `json:"ruimte"`
	ItemID        int64     `json:"item_id"`
	Soort         string    `json:"soort"`
	Naam          string    `json:"naam"`
	Pad           string    `json:"pad"`
	Mappen        []int64   `json:"mappen"`
	Titel         string    `json:"titel"`
	Omschrijving  string    `json:"omschrijving"`
	Documentsoort string    `json:"documentsoort,omitempty"`
	Trefwoorden   []string  `json:"trefwoorden"`
	Extensie      string    `json:"extensie,omitempty"`
	Mediatype     string    `json:"mediatype,omitempty"`
	Grootte       int64     `json:"grootte"`
	Gewijzigd     time.Time `json:"gewijzigd"`
	Door          string    `json:"door"`
	DoorNaam      string    `json:"door_naam"`
	Versie        int       `json:"versie"`
	Inhoud        *string   `json:"inhoud,omitempty"`
	InhoudStatus  string    `json:"inhoud_status,omitempty"`
}

// zoekItem: één item in of uit de index.
func (a *App) zoekItem(ctx context.Context, id int64) error {
	z := a.Zoeken
	docPad := "/" + zoekIndex + "/_doc/" + strconv.FormatInt(id, 10)
	weg := func() error {
		if _, err := z.vraag(ctx, http.MethodDelete, docPad, nil, nil); err != nil {
			return err
		}
		_, err := a.Reg.db.ExecContext(ctx, `DELETE FROM zoek_stand WHERE item_id = $1`, id)
		return err
	}

	// Het item, met de mappen erboven (van boven naar beneden).
	rows, err := a.Reg.db.QueryContext(ctx, `
		WITH RECURSIVE op AS (
			SELECT id, ouder_id, naam, verwijderd_op, 0 AS diepte FROM item WHERE id = $1
			UNION ALL
			SELECT i.id, i.ouder_id, i.naam, i.verwijderd_op, op.diepte + 1 FROM item i JOIN op ON i.id = op.ouder_id WHERE op.diepte < 100)
		SELECT id, naam, verwijderd_op IS NOT NULL FROM op ORDER BY diepte DESC`, id)
	if err != nil {
		return err
	}
	var mappen []int64
	var namen []string
	verwijderd := false
	for rows.Next() {
		var mid int64
		var naam string
		var v bool
		if err := rows.Scan(&mid, &naam, &v); err != nil {
			rows.Close()
			return err
		}
		verwijderd = verwijderd || v
		if mid != id {
			mappen = append(mappen, mid)
			namen = append(namen, naam)
		}
	}
	rows.Close()

	var x struct {
		ruimteID                   int64
		sleutel, ruimteStatus      string
		soort, naam, titel, omschr string
		dsoort, door, doorNaam     string
		trefwoorden                []string
		versie                     int
		gewijzigd                  time.Time
		object, mediatype          sql.NullString
		grootte                    sql.NullInt64
	}
	err = a.Reg.db.QueryRowContext(ctx, `
		SELECT i.ruimte_id, r.sleutel, r.status, i.soort, i.naam, i.titel, i.omschrijving, i.documentsoort,
		       i.gewijzigd_door, coalesce(p.naam, i.gewijzigd_door), i.trefwoorden, i.huidige_versie, i.gewijzigd,
		       v.object, v.mediatype, v.grootte
		FROM item i JOIN ruimte r ON r.id = i.ruimte_id
		LEFT JOIN versie v ON v.item_id = i.id AND v.nummer = i.huidige_versie
		LEFT JOIN persoon p ON p.email = i.gewijzigd_door
		WHERE i.id = $1`, id).Scan(&x.ruimteID, &x.sleutel, &x.ruimteStatus, &x.soort, &x.naam, &x.titel, &x.omschr, &x.dsoort,
		&x.door, &x.doorNaam, pq.Array(&x.trefwoorden), &x.versie, &x.gewijzigd, &x.object, &x.mediatype, &x.grootte)
	if errors.Is(err, sql.ErrNoRows) || verwijderd || (err == nil && x.ruimteStatus == "weg") {
		return weg()
	}
	if err != nil {
		return err
	}

	d := zoekDoc{RuimteID: x.ruimteID, Ruimte: x.sleutel, ItemID: id, Soort: x.soort, Naam: x.naam,
		Pad: strings.Join(namen, " / "), Mappen: mappen, Titel: x.titel, Omschrijving: x.omschr,
		Documentsoort: x.dsoort, Trefwoorden: x.trefwoorden, Door: x.door, DoorNaam: x.doorNaam,
		// UTC in milliseconden: het datumveld van OpenSearch leest dat zeker.
		Gewijzigd: x.gewijzigd.UTC().Truncate(time.Millisecond),
		Versie: x.versie}
	if d.Mappen == nil {
		d.Mappen = []int64{}
	}
	if d.Trefwoorden == nil {
		d.Trefwoorden = []string{}
	}
	if x.soort == "document" {
		d.Extensie = strings.TrimPrefix(strings.ToLower(path.Ext(x.naam)), ".")
		d.Mediatype, d.Grootte = x.mediatype.String, x.grootte.Int64
	}

	// Alleen de tekst opnieuw lezen bij een nieuwe versie.
	var gelezen sql.NullInt64
	a.Reg.db.QueryRowContext(ctx, `SELECT versie FROM zoek_stand WHERE item_id = $1`, id).Scan(&gelezen)
	if x.soort == "document" && gelezen.Valid && int(gelezen.Int64) == x.versie {
		code, err := z.vraag(ctx, http.MethodPost, "/"+zoekIndex+"/_update/"+strconv.FormatInt(id, 10), map[string]any{"doc": d}, nil)
		if err != nil {
			return err
		}
		if code != http.StatusNotFound {
			return nil
		}
		// Niet (meer) in de index: hieronder helemaal.
	}
	if x.soort == "document" {
		t, status := a.zoekTekst(ctx, x.object.String, x.grootte.Int64, x.naam, x.mediatype.String)
		d.Inhoud, d.InhoudStatus = &t, status
	}
	if _, err := z.vraag(ctx, http.MethodPut, docPad, d, nil); err != nil {
		return err
	}
	_, err = a.Reg.db.ExecContext(ctx, `INSERT INTO zoek_stand (item_id, versie) VALUES ($1, $2)
		ON CONFLICT (item_id) DO UPDATE SET versie = $2, op = now()`, id, x.versie)
	return err
}

// zoekTekst: de tekst van een versie; een fout is geen reden om niet te indexeren.
func (a *App) zoekTekst(ctx context.Context, object string, grootte int64, naam, mediatype string) (string, string) {
	if object == "" || tekstSoort(naam, mediatype) == "" {
		return "", "geen"
	}
	if grootte > maxTekstBron {
		return "", "te-groot"
	}
	r, _, err := a.Opslag.Haal(ctx, object)
	if err != nil {
		log.Printf("zoeken: %s ophalen: %v", naam, err)
		return "", "fout"
	}
	defer r.Close()
	f, err := os.CreateTemp("", "zoek-*")
	if err != nil {
		return "", "fout"
	}
	defer os.Remove(f.Name())
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(r, maxTekstBron+1))
	if err != nil {
		return "", "fout"
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", "fout"
	}
	t, err := haalTekst(ctx, f, n, naam, mediatype)
	if err != nil && t == "" {
		if !errors.Is(err, errGeenTekst) {
			log.Printf("zoeken: tekst uit %s: %v", naam, err)
			return "", "fout"
		}
		return "", "geen"
	}
	return t, "gelezen"
}

// --- Zoeken ------------------------------------------------------------------------

type Zoekresultaat struct {
	Ruimte    ruimteKaart
	ID        int64
	Soort     string
	Naam      string
	NaamHTML  template.HTML
	Pad       string
	MapID     int64
	Fragment  []template.HTML
	Gewijzigd time.Time
	DoorNaam  string
	Grootte   int64
	Mediatype string
	Versie    int
	URL       string
}

type Zoekpagina struct {
	Vraag      string
	Ruimtes    []ruimteKaart
	In         string // sleutel van de ruimte (leeg = overal)
	InNaam     string
	MapID      int64
	MapNaam    string
	Soort      string // "", document, map, pdf, word, rekenblad, presentatie
	Resultaten []Zoekresultaat
	Totaal     int  // treffers in de index (voor de rechtencontrole)
	Meer       bool // er zijn er meer dan getoond
	Uit        bool // zoeken staat uit of OpenSearch is er niet
	Gezocht    bool
}

var zoekSoorten = map[string][]string{
	"pdf":         {"pdf"},
	"word":        {"docx", "docm", "doc", "odt", "rtf"},
	"rekenblad":   {"xlsx", "xlsm", "xls", "ods", "csv"},
	"presentatie": {"pptx", "pptm", "ppt", "odp"},
}

const (
	markBegin = "\x01"
	markEind  = "\x02"
	zoekToon  = 50
)

func markeer(s string) template.HTML {
	s = html.EscapeString(s)
	s = strings.ReplaceAll(s, markBegin, "<mark>")
	s = strings.ReplaceAll(s, markEind, "</mark>")
	return template.HTML(s) // alleen onze eigen <mark>; de rest is ge-escaped
}

func (a *App) zoeken(w http.ResponseWriter, r *http.Request, s *Sessie) {
	ctx := r.Context()
	q := r.URL.Query()
	p := &Zoekpagina{Vraag: strings.TrimSpace(q.Get("q")), In: q.Get("in"), Soort: q.Get("soort")}
	if len([]rune(p.Vraag)) > 200 {
		p.Vraag = string([]rune(p.Vraag)[:200])
	}
	p.MapID, _ = strconv.ParseInt(q.Get("m"), 10, 64)

	kaarten, err := a.ruimteKaarten(ctx, s, nil)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	p.Ruimtes = kaarten
	booms := map[int64]*Boom{}
	perID := map[int64]ruimteKaart{}
	var zichtbaar []int64
	for _, k := range kaarten {
		perID[k.ID] = k
		zichtbaar = append(zichtbaar, k.ID)
		if k.Sleutel == p.In {
			p.InNaam = k.Naam
		}
	}
	if p.InNaam == "" {
		p.In, p.MapID = "", 0
	}
	boom := func(rid int64) *Boom {
		if b, ok := booms[rid]; ok {
			return b
		}
		k := perID[rid]
		b, err := a.Reg.laadBoom(ctx, &k.Ruimte, s)
		if err != nil {
			b = nil
		}
		booms[rid] = b
		return b
	}
	if p.MapID != 0 {
		var rid int64
		for _, k := range kaarten {
			if k.Sleutel == p.In {
				rid = k.ID
			}
		}
		if b := boom(rid); b != nil && b.mappen[p.MapID] != nil && !b.mappen[p.MapID].Doc && b.Zichtbaar(p.MapID) {
			p.MapNaam = b.mappen[p.MapID].Naam
		} else {
			p.MapID = 0
		}
	}

	if a.Zoeken == nil || !a.Zoeken.klaar.Load() {
		p.Uit = true
		a.toon(w, r, "zoeken", http.StatusOK, pagina{Titel: "Zoeken", Sessie: s, Data: p})
		return
	}
	if p.Vraag == "" || len(zichtbaar) == 0 {
		a.toon(w, r, "zoeken", http.StatusOK, pagina{Titel: "Zoeken", Sessie: s, Data: p})
		return
	}
	p.Gezocht = true

	filters := []any{map[string]any{"terms": map[string]any{"ruimte_id": zichtbaar}}}
	if p.In != "" {
		filters = append(filters, map[string]any{"term": map[string]any{"ruimte": p.In}})
	}
	if p.MapID != 0 {
		filters = append(filters, map[string]any{"term": map[string]any{"mappen": p.MapID}})
	}
	switch p.Soort {
	case "map", "document":
		filters = append(filters, map[string]any{"term": map[string]any{"soort": p.Soort}})
	default:
		if ext, ok := zoekSoorten[p.Soort]; ok {
			filters = append(filters, map[string]any{"terms": map[string]any{"extensie": ext}})
		} else {
			p.Soort = ""
		}
	}
	vraag := map[string]any{
		"size":    zoekToon * 2, // ruimte voor wat de rechtencontrole wegfiltert
		"_source": map[string]any{"excludes": []string{"inhoud"}},
		"query": map[string]any{"bool": map[string]any{
			"filter": filters,
			"should": []any{
				map[string]any{"multi_match": map[string]any{"query": p.Vraag, "type": "cross_fields", "operator": "and",
					"fields": []string{"naam^6", "naam.gewoon^6", "titel^4", "titel.gewoon^4", "trefwoorden^4", "omschrijving^2", "pad^1.5", "door_naam", "inhoud", "inhoud.gewoon"}}},
				// Het begin van een woord in de naam ("notul" vindt "notulen").
				map[string]any{"match_bool_prefix": map[string]any{"naam.gewoon": map[string]any{"query": p.Vraag, "operator": "and", "boost": 3}}},
				map[string]any{"match_phrase": map[string]any{"inhoud.gewoon": map[string]any{"query": p.Vraag, "boost": 2}}},
			},
			"minimum_should_match": 1,
		}},
		"highlight": map[string]any{
			"pre_tags": []string{markBegin}, "post_tags": []string{markEind}, "encoder": "default",
			"fields": map[string]any{
				"naam":          map[string]any{"number_of_fragments": 0},
				"naam.gewoon":   map[string]any{"number_of_fragments": 0},
				"inhoud":        map[string]any{"fragment_size": 170, "number_of_fragments": 2},
				"inhoud.gewoon": map[string]any{"fragment_size": 170, "number_of_fragments": 2},
				"omschrijving":  map[string]any{"number_of_fragments": 0},
			},
		},
	}
	var antwoord struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []struct {
				Bron      zoekDoc             `json:"_source"`
				Markering map[string][]string `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if _, err := a.Zoeken.vraag(ctx, http.MethodPost, "/"+zoekIndex+"/_search", vraag, &antwoord); err != nil {
		logFout(r, err)
		p.Uit = true
		a.toon(w, r, "zoeken", http.StatusOK, pagina{Titel: "Zoeken", Sessie: s, Data: p})
		return
	}
	p.Totaal = antwoord.Hits.Total.Value
	for _, h := range antwoord.Hits.Hits {
		d := h.Bron
		k, ok := perID[d.RuimteID]
		b := boom(d.RuimteID)
		if !ok || b == nil {
			continue
		}
		// Dezelfde rechten als de verkenner; en bestaat het nog?
		m := b.mappen[d.ItemID]
		if m == nil {
			continue
		}
		if m.Doc && b.Niveau(d.ItemID) < Lezen || !m.Doc && !b.Zichtbaar(d.ItemID) {
			continue
		}
		if len(p.Resultaten) == zoekToon {
			p.Meer = true
			break
		}
		res := Zoekresultaat{Ruimte: k, ID: d.ItemID, Soort: d.Soort, Naam: d.Naam, Pad: d.Pad, Gewijzigd: d.Gewijzigd,
			DoorNaam: d.DoorNaam, Grootte: d.Grootte, Mediatype: d.Mediatype, Versie: d.Versie}
		if len(d.Mappen) > 0 {
			res.MapID = d.Mappen[len(d.Mappen)-1]
		}
		res.NaamHTML = template.HTML(html.EscapeString(d.Naam))
		for _, veld := range []string{"naam.gewoon", "naam"} {
			if f := h.Markering[veld]; len(f) > 0 {
				res.NaamHTML = markeer(f[0])
				break
			}
		}
		for _, veld := range []string{"inhoud.gewoon", "inhoud", "omschrijving"} {
			if f := h.Markering[veld]; len(f) > 0 {
				for _, x := range f {
					res.Fragment = append(res.Fragment, markeer(x))
				}
				break
			}
		}
		if d.Soort == "map" {
			res.URL = "/r/" + k.Sleutel + "/m/" + strconv.FormatInt(d.ItemID, 10)
		} else {
			res.URL = "/r/" + k.Sleutel + "/d/" + strconv.FormatInt(d.ItemID, 10)
		}
		p.Resultaten = append(p.Resultaten, res)
	}
	a.toon(w, r, "zoeken", http.StatusOK, pagina{Titel: "Zoeken: " + p.Vraag, Sessie: s, Data: p})
}
