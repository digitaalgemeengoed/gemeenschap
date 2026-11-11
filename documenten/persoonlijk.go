package main

// Persoonlijke ruimtes ("Mijn documenten"): voor wie in de kerngroep, een
// werkgroep of een project zit, en voor de beheerders van het platform. Aangemaakt bij het eerste gebruik, met een
// quotum. Alleen de eigenaar kan erin (ook de beheerders van het platform
// niet); hij kan wel delen.
//
// Is de eigenaar nergens meer lid, dan wordt de ruimte bevroren: 90 dagen
// alleen lezen, ook geen nieuwe documenten. Wordt hij weer lid, dan gaat hij
// weer open. Na 90 dagen gaat alles naar de prullenbak.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const bevriesDuur = 90 * 24 * time.Hour

// MagPersoonlijk: zit dit e-mailadres in de kerngroep, een werkgroep of een
// project (met een rol die toegang geeft)?
func (r *Register) MagPersoonlijk(ctx context.Context, email string) (bool, error) {
	var ja bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM vaste_leden WHERE email = $1)
		OR EXISTS (SELECT 1 FROM lid l JOIN ruimte r ON r.id = l.ruimte_id
			WHERE l.email = $1 AND l.niveau > 0 AND r.status = 'actief' AND r.soort IN ('project', 'werkgroep'))`,
		strings.ToLower(email)).Scan(&ja)
	return ja, err
}

func (a *App) magPersoonlijk(ctx context.Context, s *Sessie) (bool, error) {
	if s.In(groepKerngroep) || s.In(groepBeheer) {
		return true, nil
	}
	return a.Reg.MagPersoonlijk(ctx, s.Email)
}

// PersoonlijkeRuimte: de ruimte van deze persoon, zo nodig aangemaakt of
// (na het verlopen van een eerdere) weer geopend.
func (r *Register) PersoonlijkeRuimte(ctx context.Context, email string, quotum int64) (*Ruimte, error) {
	email = strings.ToLower(email)
	var x Ruimte
	err := scanRuimte(r.db.QueryRowContext(ctx, `SELECT `+ruimteKolommen+` FROM ruimte WHERE eigenaar = $1`, email), &x)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		sleutel := "mijn-" + hashSleutel(email)[:10]
		var q any
		if quotum > 0 {
			q = quotum
		}
		if _, err := r.db.ExecContext(ctx, `INSERT INTO ruimte (sleutel, soort, naam, eigenaar, quotum, status)
			VALUES ($1, 'persoonlijk', 'Mijn documenten', $2, $3, 'actief') ON CONFLICT DO NOTHING`, sleutel, email, q); err != nil {
			return nil, err
		}
		log.Printf("persoonlijke ruimte %s aangemaakt", sleutel)
		return r.PersoonlijkeRuimte(ctx, email, quotum)
	case err != nil:
		return nil, err
	case x.Status == "weg":
		if _, err := r.db.ExecContext(ctx, `UPDATE ruimte SET status = 'actief', bevroren_sinds = NULL WHERE id = $1`, x.ID); err != nil {
			return nil, err
		}
		x.Status, x.BevrorenSinds = "actief", sql.NullTime{}
	}
	return &x, nil
}

// mijn: /r/mijn → de eigen ruimte.
func (a *App) mijn(w http.ResponseWriter, r *http.Request, s *Sessie) {
	bestaand, err := a.Reg.eigenRuimte(r.Context(), s.Email)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	if bestaand == nil || bestaand.Status == "weg" {
		mag, err := a.magPersoonlijk(r.Context(), s)
		if err != nil {
			a.interneFout(w, r, s, err)
			return
		}
		if !mag {
			a.fout(w, r, s, http.StatusForbidden, "Mijn documenten is er voor wie in de kerngroep, een werkgroep of een project zit, en voor de beheerders van het platform. Word lid van een werkgroep op de site, of vraag de projectleider je in Redmine aan een project toe te voegen.")
			return
		}
	}
	rm, err := a.Reg.PersoonlijkeRuimte(r.Context(), s.Email, a.QuotumPersoonlijk)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	http.Redirect(w, r, "/r/"+rm.Sleutel, http.StatusSeeOther)
}

func (r *Register) eigenRuimte(ctx context.Context, email string) (*Ruimte, error) {
	var x Ruimte
	err := scanRuimte(r.db.QueryRowContext(ctx, `SELECT `+ruimteKolommen+` FROM ruimte WHERE eigenaar = $1`, strings.ToLower(email)), &x)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &x, err
}

// levenscyclus: bevriezen, ontdooien en na 90 dagen opruimen.
func (a *App) levenscyclus(ctx context.Context) error {
	rows, err := a.Reg.db.QueryContext(ctx, `SELECT `+ruimteKolommen+` FROM ruimte WHERE soort = 'persoonlijk' AND status <> 'weg'`)
	if err != nil {
		return err
	}
	var ruimtes []Ruimte
	for rows.Next() {
		var x Ruimte
		if err := scanRuimte(rows, &x); err != nil {
			rows.Close()
			return err
		}
		ruimtes = append(ruimtes, x)
	}
	rows.Close()
	for _, rm := range ruimtes {
		eigenaar := rm.Eigenaar.String
		mag, err := a.Reg.MagPersoonlijk(ctx, eigenaar)
		if err != nil {
			return err
		}
		switch {
		case rm.Status == "actief" && !mag:
			if _, err := a.Reg.db.ExecContext(ctx, `UPDATE ruimte SET status = 'gesloten', bevroren_sinds = now() WHERE id = $1`, rm.ID); err != nil {
				return err
			}
			a.Reg.logboekLos(ctx, "systeem", rm.ID, "bevroren", eigenaar+" is geen lid meer van de kerngroep, een werkgroep of een project, en geen beheerder")
			log.Printf("persoonlijke ruimte %s bevroren", rm.Sleutel)
			a.systeembericht(ctx, eigenaar, "Je persoonlijke documenten zijn bevroren",
				fmt.Sprintf("Je zit niet meer in de kerngroep, een werkgroep of een project (en bent geen beheerder). Daarom staat <a href=\"%s/r/%s\">Mijn documenten</a> nu op alleen-lezen. Over 90 dagen (%s) gaan de documenten naar de prullenbak. Download wat je wilt bewaren. Word je weer lid, dan gaat de ruimte vanzelf weer open.",
					a.Basis, rm.Sleutel, time.Now().Add(bevriesDuur).In(amsterdam).Format("2-1-2006")))
		case rm.Status == "gesloten" && mag:
			if _, err := a.Reg.db.ExecContext(ctx, `UPDATE ruimte SET status = 'actief', bevroren_sinds = NULL WHERE id = $1`, rm.ID); err != nil {
				return err
			}
			a.Reg.logboekLos(ctx, "systeem", rm.ID, "ontdooid", eigenaar+" is weer lid")
			log.Printf("persoonlijke ruimte %s weer open", rm.Sleutel)
		case rm.Status == "gesloten" && rm.BevrorenSinds.Valid && time.Since(rm.BevrorenSinds.Time) > bevriesDuur:
			res, err := a.Reg.db.ExecContext(ctx, `UPDATE item SET verwijderd_op = now(), verwijderd_door = 'systeem' WHERE ruimte_id = $1 AND verwijderd_op IS NULL`, rm.ID)
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			if _, err := a.Reg.db.ExecContext(ctx, `UPDATE ruimte SET status = 'weg' WHERE id = $1`, rm.ID); err != nil {
				return err
			}
			a.Reg.logboekLos(ctx, "systeem", rm.ID, "opgeruimd", fmt.Sprintf("na 90 dagen bevroren: %d item(s) naar de prullenbak", n))
			log.Printf("persoonlijke ruimte %s opgeruimd (%d items)", rm.Sleutel, n)
		}
	}
	return nil
}

func (r *Register) logboekLos(ctx context.Context, wie string, ruimteID int64, actie, details string) {
	r.db.ExecContext(ctx, `INSERT INTO logboek (wie, ruimte_id, actie, details) VALUES ($1, $2, $3, $4)`, wie, ruimteID, actie, details)
}

// systeembericht: een bericht van de site (afzender: de beheerder).
func (a *App) systeembericht(ctx context.Context, aan, onderwerp, inhoud string) {
	if a.Berichten == nil {
		return
	}
	if _, err := a.Berichten.stuur(ctx, map[string]any{"systeem": true, "aan": []string{aan}, "onderwerp": onderwerp, "inhoud": inhoud}); err != nil {
		log.Printf("bericht aan %s: %v", aan, err)
	}
}
