package main

// Beheer (/beheer): voor de beheerders van het platform en de kerngroep.
// Alle ruimtes met omvang, quotum en status; het quotum wijzigen; per ruimte
// de prullenbak (terugzetten of definitief verwijderen) en het logboek. Van
// persoonlijke ruimtes zie je alleen de omvang, niet de inhoud.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
)

func magBeheren(s *Sessie) bool { return s.In(groepBeheer) || s.In(groepKerngroep) }

type ruimteRegel struct {
	Ruimte
	EigenaarNaam string
	Gebruik      int64
	Aantal       int64
	InPrullenbak int64
}

func (r *Register) Overzicht(ctx context.Context) ([]ruimteRegel, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+prefix("r.", ruimteKolommen)+`, coalesce(p.naam, ''),
		coalesce((SELECT sum(v.grootte) FROM versie v JOIN item i ON i.id = v.item_id WHERE i.ruimte_id = r.id AND i.verwijderd_op IS NULL), 0),
		(SELECT count(*) FROM item i WHERE i.ruimte_id = r.id AND i.soort = 'document' AND i.verwijderd_op IS NULL),
		(SELECT count(*) FROM item i WHERE i.ruimte_id = r.id AND i.verwijderd_op IS NOT NULL)
		FROM ruimte r LEFT JOIN persoon p ON p.email = r.eigenaar
		ORDER BY r.status = 'weg', array_position(ARRAY['cooperatie','werkgroep','project','persoonlijk'], r.soort), lower(coalesce(p.naam, r.naam))`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var uit []ruimteRegel
	for rows.Next() {
		var x ruimteRegel
		if err := rows.Scan(&x.ID, &x.Sleutel, &x.Soort, &x.Naam, &x.RedmineID, &x.Status, &x.Eigenaar, &x.Quotum, &x.BevrorenSinds,
			&x.EigenaarNaam, &x.Gebruik, &x.Aantal, &x.InPrullenbak); err != nil {
			return nil, err
		}
		uit = append(uit, x)
	}
	return uit, rows.Err()
}

func prefix(p, kolommen string) string {
	delen := strings.Split(kolommen, ", ")
	for i := range delen {
		delen[i] = p + delen[i]
	}
	return strings.Join(delen, ", ")
}

type weggegooid struct {
	ID             int64
	Naam, Soort    string
	Pad            string
	VerwijderdOp   time.Time
	VerwijderdDoor string
	Onder          int64 // aantal items eronder die mee weggingen
}

// Prullenbak: wat in één keer werd verwijderd (de bovenste van elke keer).
func (r *Register) Prullenbak(ctx context.Context, ruimteID int64) ([]weggegooid, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT i.id, i.naam, i.soort, i.verwijderd_op, coalesce(p.naam, i.verwijderd_door, ''),
		(SELECT count(*) FROM item k WHERE k.ruimte_id = i.ruimte_id AND k.verwijderd_op = i.verwijderd_op AND k.id <> i.id),
		coalesce((WITH RECURSIVE pad AS (SELECT id, ouder_id, naam FROM item WHERE id = i.ouder_id
			UNION ALL SELECT o.id, o.ouder_id, o.naam FROM item o JOIN pad ON o.id = pad.ouder_id)
			SELECT string_agg(naam, ' / ' ORDER BY id) FROM pad), '')
		FROM item i LEFT JOIN item o ON o.id = i.ouder_id LEFT JOIN persoon p ON p.email = i.verwijderd_door
		WHERE i.ruimte_id = $1 AND i.verwijderd_op IS NOT NULL
		  AND (o.id IS NULL OR o.verwijderd_op IS NULL OR o.verwijderd_op <> i.verwijderd_op)
		ORDER BY i.verwijderd_op DESC LIMIT 500`, ruimteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var uit []weggegooid
	for rows.Next() {
		var x weggegooid
		if err := rows.Scan(&x.ID, &x.Naam, &x.Soort, &x.VerwijderdOp, &x.VerwijderdDoor, &x.Onder, &x.Pad); err != nil {
			return nil, err
		}
		uit = append(uit, x)
	}
	return uit, rows.Err()
}

// Terugzetten: het item en wat er tegelijk mee wegging. Staat de map erboven
// zelf in de prullenbak, dan komt het bovenaan in de ruimte.
func (r *Register) Terugzetten(ctx context.Context, ruimteID, id int64, wie string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var op time.Time
	var ouder sql.NullInt64
	var ouderWeg bool
	err = tx.QueryRowContext(ctx, `SELECT i.verwijderd_op, i.ouder_id, coalesce(o.verwijderd_op IS NOT NULL, false) FROM item i LEFT JOIN item o ON o.id = i.ouder_id
		WHERE i.id = $1 AND i.ruimte_id = $2 AND i.verwijderd_op IS NOT NULL FOR UPDATE OF i`, id, ruimteID).Scan(&op, &ouder, &ouderWeg)
	if errors.Is(err, sql.ErrNoRows) {
		return errNietGevonden
	}
	if err != nil {
		return err
	}
	if ouderWeg {
		if _, err := tx.ExecContext(ctx, `UPDATE item SET ouder_id = NULL WHERE id = $1`, id); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `WITH RECURSIVE boom AS (
			SELECT id FROM item WHERE id = $1
			UNION ALL SELECT i.id FROM item i JOIN boom b ON i.ouder_id = b.id WHERE i.verwijderd_op = $2)
		UPDATE item SET verwijderd_op = NULL, verwijderd_door = NULL WHERE id IN (SELECT id FROM boom)`, id, op)
	if isUniek(err) {
		return errBestaatAl
	}
	if err != nil {
		return err
	}
	if err := logboek(ctx, tx, wie, ruimteID, id, "teruggezet", "uit de prullenbak"); err != nil {
		return err
	}
	return tx.Commit()
}

// DefinitiefWeg: het item en alles eronder uit het register; de objecten
// komen terug om uit de opslag te halen.
func (r *Register) DefinitiefWeg(ctx context.Context, ruimteID, id int64, wie string) ([]string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var ids []int64
	rows, err := tx.QueryContext(ctx, `WITH RECURSIVE boom AS (
			SELECT id FROM item WHERE id = $1 AND ruimte_id = $2 AND verwijderd_op IS NOT NULL
			UNION ALL SELECT i.id FROM item i JOIN boom b ON i.ouder_id = b.id)
		SELECT id FROM boom`, id, ruimteID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x int64
		rows.Scan(&x)
		ids = append(ids, x)
	}
	rows.Close()
	if len(ids) == 0 {
		return nil, errNietGevonden
	}
	var objecten []string
	rows, err = tx.QueryContext(ctx, `DELETE FROM versie WHERE item_id = ANY($1) RETURNING object`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var o string
		rows.Scan(&o)
		objecten = append(objecten, o)
	}
	rows.Close()
	for _, q := range []string{
		`DELETE FROM recht WHERE item_id = ANY($1)`,
		`DELETE FROM open_link WHERE item_id = ANY($1)`,
		`UPDATE logboek SET item_id = NULL WHERE item_id = ANY($1)`,
		`DELETE FROM item WHERE id = ANY($1)`,
	} {
		if _, err := tx.ExecContext(ctx, q, pq.Array(ids)); err != nil {
			return nil, err
		}
	}
	r.logboekTx(ctx, tx, wie, ruimteID, "definitief verwijderd", fmt.Sprintf("%d item(s), %d versie(s)", len(ids), len(objecten)))
	return objecten, tx.Commit()
}

func (r *Register) logboekTx(ctx context.Context, tx *sql.Tx, wie string, ruimteID int64, actie, details string) {
	tx.ExecContext(ctx, `INSERT INTO logboek (wie, ruimte_id, actie, details) VALUES ($1, $2, $3, $4)`, wie, ruimteID, actie, details)
}

type logRegel struct {
	Op                  time.Time
	Wie, Actie, Details string
	Item                string
}

func (r *Register) Logboek(ctx context.Context, ruimteID int64) ([]logRegel, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT l.op, coalesce(p.naam, l.wie), l.actie, l.details, coalesce(i.naam, '')
		FROM logboek l LEFT JOIN persoon p ON p.email = l.wie LEFT JOIN item i ON i.id = l.item_id
		WHERE l.ruimte_id = $1 ORDER BY l.id DESC LIMIT 300`, ruimteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var uit []logRegel
	for rows.Next() {
		var x logRegel
		if err := rows.Scan(&x.Op, &x.Wie, &x.Actie, &x.Details, &x.Item); err != nil {
			return nil, err
		}
		uit = append(uit, x)
	}
	return uit, rows.Err()
}

// --- Pagina's ------------------------------------------------------------------------

func (a *App) beheer(w http.ResponseWriter, r *http.Request, s *Sessie) {
	if !magBeheren(s) {
		a.fout(w, r, s, http.StatusForbidden, "Beheer is voor de kerngroep en de beheerders van het platform.")
		return
	}
	if r.Method == http.MethodPost {
		if !a.csrfGoed(r, s) {
			a.fout(w, r, s, http.StatusForbidden, "Verlopen formulier. Ververs de pagina en probeer het opnieuw.")
			return
		}
		id, _ := strconv.ParseInt(r.FormValue("ruimte"), 10, 64)
		mb := strings.TrimSpace(r.FormValue("mb"))
		var q any
		if mb != "" {
			n, err := strconv.ParseInt(mb, 10, 64)
			if err != nil || n < 1 {
				terug(w, r, "/beheer", "fout", "Geef het quotum in hele MB, of laat het leeg voor geen quotum.")
				return
			}
			q = n << 20
		}
		if _, err := a.Reg.db.ExecContext(r.Context(), `UPDATE ruimte SET quotum = $2 WHERE id = $1`, id, q); err != nil {
			a.interneFout(w, r, s, err)
			return
		}
		a.Reg.logboekLos(r.Context(), s.Email, id, "quotum", fmt.Sprintf("%v MB", mb))
		terug(w, r, "/beheer", "melding", "Quotum bijgewerkt.")
		return
	}
	lijst, err := a.Reg.Overzicht(r.Context())
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	var totaal int64
	for _, x := range lijst {
		totaal += x.Gebruik
	}
	a.toon(w, r, "beheer", http.StatusOK, pagina{Titel: "Beheer", Sessie: s, Data: map[string]any{"Lijst": lijst, "Totaal": totaal, "Bevries": bevriesDuur}})
}

// beheerRuimte: de prullenbak en het logboek van één ruimte. Een
// persoonlijke ruimte alleen voor de eigenaar zelf.
func (a *App) beheerRuimte(w http.ResponseWriter, r *http.Request, s *Sessie) {
	id := padID(r)
	rm, err := a.Reg.RuimteVanID(r.Context(), id)
	if errors.Is(err, errNietGevonden) {
		// Ook een ruimte die weg is, heeft een prullenbak.
		var x Ruimte
		if err := scanRuimte(a.Reg.db.QueryRowContext(r.Context(), `SELECT `+ruimteKolommen+` FROM ruimte WHERE id = $1`, id), &x); err != nil {
			a.fout(w, r, s, http.StatusNotFound, "Deze ruimte bestaat niet.")
			return
		}
		rm, err = &x, nil
	}
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	eigen := rm.Soort == "persoonlijk" && strings.EqualFold(rm.Eigenaar.String, s.Email)
	if !eigen && (rm.Soort == "persoonlijk" || !magBeheren(s)) {
		a.fout(w, r, s, http.StatusForbidden, "De prullenbak en het logboek zijn voor de kerngroep en de beheerders; van een persoonlijke ruimte alleen voor de eigenaar.")
		return
	}
	doel := fmt.Sprintf("/beheer/r/%d", id)
	if r.Method == http.MethodPost {
		if !a.csrfGoed(r, s) {
			a.fout(w, r, s, http.StatusForbidden, "Verlopen formulier. Ververs de pagina en probeer het opnieuw.")
			return
		}
		item, _ := strconv.ParseInt(r.FormValue("item"), 10, 64)
		switch r.FormValue("actie") {
		case "terug":
			err := a.Reg.Terugzetten(r.Context(), rm.ID, item, s.Email)
			switch {
			case errors.Is(err, errBestaatAl):
				terug(w, r, doel, "fout", "Er staat al iets met die naam op die plek. Hernoem dat eerst.")
			case err != nil:
				a.interneFout(w, r, s, err)
			default:
				terug(w, r, doel, "melding", "Teruggezet.")
			}
			return
		case "weg":
			objecten, err := a.Reg.DefinitiefWeg(r.Context(), rm.ID, item, s.Email)
			if err != nil {
				a.interneFout(w, r, s, err)
				return
			}
			mislukt := 0
			for _, o := range objecten {
				if err := a.Opslag.Verwijder(r.Context(), o); err != nil {
					mislukt++
					logFout(r, err)
				}
			}
			tekst := fmt.Sprintf("Definitief verwijderd (%d versie(s)).", len(objecten))
			if mislukt > 0 {
				tekst += fmt.Sprintf(" %d object(en) bleven in de opslag staan; dat is onschadelijk.", mislukt)
			}
			terug(w, r, doel, "melding", tekst)
			return
		}
	}
	prullenbak, err := a.Reg.Prullenbak(r.Context(), rm.ID)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	log, err := a.Reg.Logboek(r.Context(), rm.ID)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	naam := rm.Naam
	if rm.Soort == "persoonlijk" {
		if namen, err := a.Reg.Namen(r.Context()); err == nil && namen[rm.Eigenaar.String] != "" {
			naam = "Mijn documenten van " + namen[rm.Eigenaar.String]
		}
	}
	a.toon(w, r, "beheerruimte", http.StatusOK, pagina{Titel: "Beheer · " + naam, Sessie: s, Data: map[string]any{
		"Ruimte": rm, "Naam": naam, "Prullenbak": prullenbak, "Logboek": log, "Persoonlijk": rm.Soort == "persoonlijk", "Beheer": magBeheren(s),
	}})
}
