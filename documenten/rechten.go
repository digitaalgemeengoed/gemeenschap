package main

// Rechten: drie niveaus, zoals in de XENA-demo.
//
//	Lezen     zien en downloaden
//	Bewerken  ook uploaden, nieuwe versies, metadata, hernoemen, mappen maken
//	Beheren   ook verwijderen en de toegang wijzigen (ook aan anderen geven)
//
// Een regel geeft een persoon, een groep uit Keycloak of de projectleden
// (volgens hun rol in Redmine) een niveau, op de ruimte of op een map. Een map
// neemt de regels van de map erboven over en kan er regels bij krijgen. Met
// "overerving stoppen" gelden alleen de eigen regels van de map: zo maak je
// een besloten map. Beheerders van het platform (/site/beheer) kunnen altijd
// overal bij, zodat niemand zich buitensluit.
//
// Standaard op de ruimte:
//
//	Coöperatie  members Lezen, kerngroep Beheren
//	Project     projectleden volgens hun rol in Redmine (Gebruiker niets,
//	            Contributor en Member Bewerken, Kerngroep en Manager Beheren),
//	            en de kerngroep Beheren
//
// Een gesloten project kan alleen nog gelezen worden.

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
)

const (
	Geen     = 0
	Lezen    = 1
	Bewerken = 2
	Beheren  = 3
)

var niveauNamen = []string{"Geen toegang", "Lezen", "Bewerken", "Beheren"}

func niveauNaam(n int) string {
	if n < 0 || n >= len(niveauNamen) {
		return ""
	}
	return niveauNamen[n]
}

// Regel: wie welk niveau heeft. Bron zegt waar hij vandaan komt.
type Regel struct {
	ID     int64  // 0 voor de vaste en standaardregels
	Soort  string // groep, persoon, rol
	Wie    string
	Niveau int
	MapID  int64  // waar de regel staat (0 = de ruimte)
	Bron   string // "Standaard", of de naam van de map of ruimte
	Vast   bool   // niet te wijzigen (beheerders van het platform, de standaardregels)
	Eigen  bool   // staat op de map waar je naar kijkt
}

type mapInfo struct {
	ID    int64
	Ouder int64
	Naam  string
	Eigen bool
}

// Boom: de mappen en regels van één ruimte, bekeken door één persoon.
type Boom struct {
	Ruimte    *Ruimte
	s         *Sessie
	lidNiveau int // uit Redmine
	mappen    map[int64]*mapInfo
	kinderen  map[int64][]int64
	regels    map[int64][]Regel // eigen regels per map (0 = de ruimte)
	niveaus   map[int64]int
	zichtbaar map[int64]bool
}

func (r *Register) laadBoom(ctx context.Context, rm *Ruimte, s *Sessie) (*Boom, error) {
	b := &Boom{Ruimte: rm, s: s, mappen: map[int64]*mapInfo{}, kinderen: map[int64][]int64{},
		regels: map[int64][]Regel{}, niveaus: map[int64]int{}, zichtbaar: map[int64]bool{}}
	if rm.Soort == "project" {
		err := r.db.QueryRowContext(ctx, `SELECT niveau FROM lid WHERE ruimte_id = $1 AND email = $2`, rm.ID, s.Email).Scan(&b.lidNiveau)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, coalesce(ouder_id, 0), naam, eigen_rechten FROM item
		WHERE ruimte_id = $1 AND soort = 'map' AND verwijderd_op IS NULL ORDER BY lower(naam)`, rm.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		m := &mapInfo{}
		if err := rows.Scan(&m.ID, &m.Ouder, &m.Naam, &m.Eigen); err != nil {
			rows.Close()
			return nil, err
		}
		b.mappen[m.ID] = m
	}
	rows.Close()
	for _, m := range b.mappen {
		// Een map onder een verwijderde map hoort niet in de boom.
		if m.Ouder == 0 || b.mappen[m.Ouder] != nil {
			b.kinderen[m.Ouder] = append(b.kinderen[m.Ouder], m.ID)
		}
	}
	for k := range b.kinderen {
		sort.Slice(b.kinderen[k], func(i, j int) bool {
			return strings.ToLower(b.mappen[b.kinderen[k][i]].Naam) < strings.ToLower(b.mappen[b.kinderen[k][j]].Naam)
		})
	}
	rows, err = r.db.QueryContext(ctx, `SELECT id, coalesce(item_id, 0), soort, wie, niveau FROM recht WHERE ruimte_id = $1 ORDER BY soort, wie`, rm.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var x Regel
		if err := rows.Scan(&x.ID, &x.MapID, &x.Soort, &x.Wie, &x.Niveau); err != nil {
			return nil, err
		}
		b.regels[x.MapID] = append(b.regels[x.MapID], x)
	}
	return b, rows.Err()
}

// standaard: de regels die op elke ruimte staan.
func (b *Boom) standaard() []Regel {
	var uit []Regel
	if b.Ruimte.Soort == "cooperatie" {
		uit = append(uit, Regel{Soort: "groep", Wie: groepMembers, Niveau: Lezen})
	} else {
		uit = append(uit, Regel{Soort: "rol", Wie: "redmine", Niveau: Beheren})
	}
	uit = append(uit, Regel{Soort: "groep", Wie: groepKerngroep, Niveau: Beheren})
	for i := range uit {
		uit[i].Bron, uit[i].Vast = "Standaard", true
	}
	return uit
}

var vasteRegel = Regel{Soort: "groep", Wie: groepBeheer, Niveau: Beheren, Bron: "Altijd", Vast: true}

// Geldend: de regels die op deze map gelden (0 = de bovenste laag van de
// ruimte), met waar ze vandaan komen. De vaste regel voor de beheerders van
// het platform staat er altijd bij.
func (b *Boom) Geldend(id int64) []Regel {
	uit := append([]Regel{vasteRegel}, b.geldend(id, id)...)
	return uit
}

func (b *Boom) geldend(id, kijk int64) []Regel {
	var uit []Regel
	if id == 0 {
		uit = b.standaard()
	} else if m := b.mappen[id]; m != nil && !m.Eigen {
		uit = b.geldend(m.Ouder, kijk)
	}
	bron := b.Ruimte.Naam
	if m := b.mappen[id]; m != nil {
		bron = m.Naam
	}
	for _, r := range b.regels[id] {
		r.Bron, r.Eigen = bron, id == kijk
		uit = append(uit, r)
	}
	return uit
}

// geeft: welk niveau deze regel deze persoon geeft.
func (b *Boom) geeft(r Regel) int {
	switch r.Soort {
	case "groep":
		if b.s.In(r.Wie) {
			return r.Niveau
		}
	case "persoon":
		if strings.EqualFold(r.Wie, b.s.Email) {
			return r.Niveau
		}
	case "rol":
		return min(r.Niveau, b.lidNiveau)
	}
	return Geen
}

// Niveau: wat deze persoon in deze map mag (0 = de bovenste laag).
func (b *Boom) Niveau(id int64) int {
	if n, ok := b.niveaus[id]; ok {
		return n
	}
	n := Geen
	for _, r := range b.Geldend(id) {
		n = max(n, b.geeft(r))
	}
	if b.Ruimte.Status != "actief" {
		n = min(n, Lezen)
	}
	b.niveaus[id] = n
	return n
}

// Zichtbaar: de map zelf mag gelezen worden, of er ligt een map onder die
// gelezen mag worden (dan is deze map een doorgang: je ziet de naam, niet de
// inhoud).
func (b *Boom) Zichtbaar(id int64) bool {
	if z, ok := b.zichtbaar[id]; ok {
		return z
	}
	z := b.Niveau(id) >= Lezen
	for _, k := range b.kinderen[id] {
		if b.Zichtbaar(k) {
			z = true
		}
	}
	b.zichtbaar[id] = z
	return z
}

func (b *Boom) Bestaat(id int64) bool { return id == 0 || b.mappen[id] != nil }
func (b *Boom) Besloten(id int64) bool {
	m := b.mappen[id]
	return m != nil && m.Eigen
}

// Knoop: een map in de boom links.
type Knoop struct {
	ID       int64
	Naam     string
	Kinderen []*Knoop
	Open     bool // op het pad naar de map waar je bent
	Actief   bool // de map waar je bent
	Doorgang bool // alleen de naam, niet de inhoud
	Besloten bool
}

// Knopen: de zichtbare mappen onder id, met het pad naar hier opengeklapt.
func (b *Boom) Knopen(id int64, hier int64, pad map[int64]bool) []*Knoop {
	var uit []*Knoop
	for _, k := range b.kinderen[id] {
		if !b.Zichtbaar(k) {
			continue
		}
		n := &Knoop{ID: k, Naam: b.mappen[k].Naam, Open: pad[k], Actief: k == hier,
			Doorgang: b.Niveau(k) < Lezen, Besloten: b.mappen[k].Eigen}
		n.Kinderen = b.Knopen(k, hier, pad)
		uit = append(uit, n)
	}
	return uit
}

// --- Wijzigen ------------------------------------------------------------------

var errOngeldig = errors.New("ongeldig")

func (r *Register) RegelToevoegen(ctx context.Context, rm *Ruimte, mapID int64, soort, wie string, niveau int, door string) error {
	if niveau < Lezen || niveau > Beheren || (soort != "groep" && soort != "persoon" && soort != "rol") || wie == "" {
		return errOngeldig
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO recht (ruimte_id, item_id, soort, wie, niveau, door) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (ruimte_id, coalesce(item_id, 0), soort, wie) DO UPDATE SET niveau = EXCLUDED.niveau, door = EXCLUDED.door, op = now()`,
		rm.ID, nullOuder(mapID), soort, wie, niveau, door)
	if err != nil {
		return err
	}
	if err := logboek(ctx, tx, door, rm.ID, mapID, "toegang gegeven", soort+" "+wie+": "+niveauNaam(niveau)); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Register) RegelWeg(ctx context.Context, rm *Ruimte, mapID, regelID int64, door string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var soort, wie string
	err = tx.QueryRowContext(ctx, `DELETE FROM recht WHERE id = $1 AND ruimte_id = $2 AND coalesce(item_id, 0) = $3 RETURNING soort, wie`,
		regelID, rm.ID, mapID).Scan(&soort, &wie)
	if errors.Is(err, sql.ErrNoRows) {
		return errNietGevonden
	}
	if err != nil {
		return err
	}
	if err := logboek(ctx, tx, door, rm.ID, mapID, "toegang ingetrokken", soort+" "+wie); err != nil {
		return err
	}
	return tx.Commit()
}

// OverervingStoppen: de regels die nu gelden, worden eigen regels van de map
// (dan verandert er nog niets); daarna kun je ze weghalen of beperken.
func (r *Register) OverervingStoppen(ctx context.Context, b *Boom, mapID int64, door string) error {
	if mapID == 0 || b.mappen[mapID] == nil {
		return errOngeldig
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, x := range b.Geldend(mapID) {
		if x.Eigen || (x.Soort == vasteRegel.Soort && x.Wie == vasteRegel.Wie && x.Bron == vasteRegel.Bron) {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO recht (ruimte_id, item_id, soort, wie, niveau, door) VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (ruimte_id, coalesce(item_id, 0), soort, wie) DO UPDATE SET niveau = GREATEST(recht.niveau, EXCLUDED.niveau)`,
			b.Ruimte.ID, mapID, x.Soort, x.Wie, x.Niveau, door); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE item SET eigen_rechten = true WHERE id = $1 AND ruimte_id = $2`, mapID, b.Ruimte.ID); err != nil {
		return err
	}
	if err := logboek(ctx, tx, door, b.Ruimte.ID, mapID, "overerving gestopt", ""); err != nil {
		return err
	}
	return tx.Commit()
}

// OverervingHerstellen: de map neemt de regels van boven weer over; de eigen
// regels blijven erbij.
func (r *Register) OverervingHerstellen(ctx context.Context, rm *Ruimte, mapID int64, door string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE item SET eigen_rechten = false WHERE id = $1 AND ruimte_id = $2`, mapID, rm.ID); err != nil {
		return err
	}
	if err := logboek(ctx, tx, door, rm.ID, mapID, "overerving hersteld", ""); err != nil {
		return err
	}
	return tx.Commit()
}

// --- Wie er is -------------------------------------------------------------------

type Keuze struct {
	Waarde string // soort|wie
	Label  string
}

var groepLabels = map[string]string{
	"/netwerk/gebruikers":   "Alle gebruikers",
	"/netwerk/contributors": "Contributors",
	"/netwerk/members":      "Members",
	"/netwerk/kerngroep":    "Kerngroep",
	"/site/redactie":        "Redactie van de site",
	"/site/beheer":          "Beheerders van het platform",
}

// Kandidaten: wie je toegang kunt geven: groepen en personen die het platform
// kent (wie ooit ingelogd heeft of in Redmine bij een project staat).
func (r *Register) Kandidaten(ctx context.Context, rm *Ruimte) (groepen, personen []Keuze, err error) {
	gezien := map[string]bool{}
	for pad := range groepLabels {
		gezien[pad] = true
	}
	rows, err := r.db.QueryContext(ctx, `SELECT pad FROM groep`)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var p string
		rows.Scan(&p)
		gezien[p] = true
	}
	rows.Close()
	for p := range gezien {
		if p == groepBeheer {
			continue // die kan altijd overal bij
		}
		groepen = append(groepen, Keuze{"groep|" + p, groepLabel(p)})
	}
	sort.Slice(groepen, func(i, j int) bool { return groepen[i].Label < groepen[j].Label })
	if rm.Soort == "project" {
		groepen = append([]Keuze{{"rol|redmine", "Projectleden (volgens hun rol in Redmine)"}}, groepen...)
	}
	rows, err = r.db.QueryContext(ctx, `SELECT email, naam FROM persoon
		UNION SELECT email, email FROM lid WHERE email NOT IN (SELECT email FROM persoon)
		ORDER BY 2`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e, n string
		rows.Scan(&e, &n)
		label := n
		if n != e {
			label = n + " (" + e + ")"
		}
		personen = append(personen, Keuze{"persoon|" + e, label})
	}
	return groepen, personen, rows.Err()
}

func groepLabel(p string) string {
	if l, ok := groepLabels[p]; ok {
		return l
	}
	return p
}

// Namen: e-mailadres → naam, voor de weergave van regels.
func (r *Register) Namen(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT email, naam FROM persoon`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	uit := map[string]string{}
	for rows.Next() {
		var e, n string
		rows.Scan(&e, &n)
		uit[e] = n
	}
	return uit, rows.Err()
}

// Wie: hoe een regel heet in het scherm.
func regelLabel(x Regel, namen map[string]string) (naam, soort string) {
	switch x.Soort {
	case "groep":
		return groepLabel(x.Wie), "groep"
	case "rol":
		return "Projectleden", "volgens hun rol in Redmine"
	default:
		if n := namen[x.Wie]; n != "" {
			return n, "persoon"
		}
		return x.Wie, "persoon"
	}
}
