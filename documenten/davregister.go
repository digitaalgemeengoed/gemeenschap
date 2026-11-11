package main

// Het register voor WebDAV: verplaatsen, versies overnemen bij het
// "veilig opslaan" van Windows en Office, app-wachtwoorden en tijdelijke
// links om een document in Word of LibreOffice te openen.

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/lib/pq"
)

func hashSleutel(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// Verplaats: een item een andere naam en/of een andere map geven (binnen de ruimte).
func (r *Register) Verplaats(ctx context.Context, ruimteID, id, nieuweOuder int64, naam, wie string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := controleerOuder(ctx, tx, ruimteID, nieuweOuder); err != nil {
		return err
	}
	// Niet in zichzelf of in een map eronder.
	for o := nieuweOuder; o != 0; {
		if o == id {
			return errOngeldig
		}
		if err := tx.QueryRowContext(ctx, `SELECT coalesce(ouder_id, 0) FROM item WHERE id = $1`, o).Scan(&o); err != nil {
			return err
		}
	}
	var oud string
	if err := tx.QueryRowContext(ctx, `SELECT naam FROM item WHERE id = $1 AND ruimte_id = $2 AND verwijderd_op IS NULL FOR UPDATE`, id, ruimteID).Scan(&oud); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNietGevonden
		}
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE item SET ouder_id = $3, naam = $4, gewijzigd = now(), gewijzigd_door = $5 WHERE id = $1 AND ruimte_id = $2`,
		id, ruimteID, nullOuder(nieuweOuder), naam, wie)
	if isUniek(err) {
		return errBestaatAl
	}
	if err != nil {
		return err
	}
	if err := logboek(ctx, tx, wie, ruimteID, id, "verplaatst", oud+" → "+naam); err != nil {
		return err
	}
	return tx.Commit()
}

// Opnemen: de laatste versie van bron wordt een nieuwe versie van doel; doel
// krijgt (weer) de naam en de map, en wordt zo nodig uit de prullenbak gehaald.
// Bron gaat naar de prullenbak. Zo blijft de geschiedenis bij één document
// als een programma opslaat via "tijdelijk bestand, oud weg, tijdelijk
// hernoemen".
func (r *Register) Opnemen(ctx context.Context, ruimteID, bron, doel, ouder int64, naam, wie string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var bronVersie, doelVersie int
	if err := tx.QueryRowContext(ctx, `SELECT huidige_versie FROM item WHERE id = $1 AND ruimte_id = $2 AND soort = 'document' AND verwijderd_op IS NULL FOR UPDATE`,
		bron, ruimteID).Scan(&bronVersie); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT huidige_versie FROM item WHERE id = $1 AND ruimte_id = $2 AND soort = 'document' FOR UPDATE`,
		doel, ruimteID).Scan(&doelVersie); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE item SET verwijderd_op = now(), verwijderd_door = $2 WHERE id = $1`, bron, wie); err != nil {
		return err
	}
	if bronVersie > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE versie SET item_id = $3, nummer = $4 WHERE item_id = $1 AND nummer = $2`,
			bron, bronVersie, doel, doelVersie+1); err != nil {
			return err
		}
		doelVersie++
	}
	_, err = tx.ExecContext(ctx, `UPDATE item SET verwijderd_op = NULL, verwijderd_door = NULL, ouder_id = $2, naam = $3,
		huidige_versie = $4, gewijzigd = now(), gewijzigd_door = $5 WHERE id = $1`, doel, nullOuder(ouder), naam, doelVersie, wie)
	if isUniek(err) {
		return errBestaatAl
	}
	if err != nil {
		return err
	}
	if err := logboek(ctx, tx, wie, ruimteID, doel, "nieuwe versie", naam+" (opgeslagen via WebDAV)"); err != nil {
		return err
	}
	return tx.Commit()
}

// --- App-wachtwoorden ---------------------------------------------------------------

type AppWachtwoord struct {
	ID         int64
	Naam       string
	Aangemaakt time.Time
	Gebruikt   sql.NullTime
}

// NieuwAppWachtwoord: geeft het wachtwoord één keer terug; bewaard wordt alleen de hash.
func (r *Register) NieuwAppWachtwoord(ctx context.Context, email, naam string) (string, error) {
	b := willekeurigeBytes(20)
	ww := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
	ww = ww[0:8] + "-" + ww[8:16] + "-" + ww[16:24] + "-" + ww[24:32]
	_, err := r.db.ExecContext(ctx, `INSERT INTO app_wachtwoord (email, naam, hash) VALUES ($1, $2, $3)`, strings.ToLower(email), naam, hashSleutel(ww))
	return ww, err
}

func (r *Register) AppWachtwoorden(ctx context.Context, email string) ([]AppWachtwoord, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, naam, aangemaakt, gebruikt FROM app_wachtwoord WHERE email = $1 ORDER BY aangemaakt DESC`, strings.ToLower(email))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var uit []AppWachtwoord
	for rows.Next() {
		var x AppWachtwoord
		if err := rows.Scan(&x.ID, &x.Naam, &x.Aangemaakt, &x.Gebruikt); err != nil {
			return nil, err
		}
		uit = append(uit, x)
	}
	return uit, rows.Err()
}

func (r *Register) AppWachtwoordWeg(ctx context.Context, email string, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM app_wachtwoord WHERE id = $1 AND email = $2`, id, strings.ToLower(email))
	return err
}

// MetAppWachtwoord: wie hoort bij dit e-mailadres en wachtwoord? Met de
// groepen van de laatste keer dat hij in de browser inlogde.
func (r *Register) MetAppWachtwoord(ctx context.Context, email, ww string) (*Sessie, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var id int64
	var gebruikt sql.NullTime
	err := r.db.QueryRowContext(ctx, `SELECT id, gebruikt FROM app_wachtwoord WHERE hash = $1 AND email = $2`, hashSleutel(strings.TrimSpace(ww)), email).Scan(&id, &gebruikt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNietGevonden
	}
	if err != nil {
		return nil, err
	}
	if !gebruikt.Valid || time.Since(gebruikt.Time) > time.Hour {
		r.db.ExecContext(ctx, `UPDATE app_wachtwoord SET gebruikt = now() WHERE id = $1`, id)
	}
	return r.persoonSessie(ctx, email)
}

func (r *Register) persoonSessie(ctx context.Context, email string) (*Sessie, error) {
	s := &Sessie{Email: email}
	err := r.db.QueryRowContext(ctx, `SELECT naam, groepen FROM persoon WHERE email = $1`, email).Scan(&s.Naam, pq.Array(&s.Groepen))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNietGevonden
	}
	return s, err
}

// --- Tijdelijke links (openen in Word of LibreOffice) -------------------------------

const openDuur = 10 * time.Hour

func (r *Register) NieuweOpenLink(ctx context.Context, email string, ruimteID, itemID int64) (string, error) {
	sleutel := willekeurig(24)
	r.db.ExecContext(ctx, `DELETE FROM open_link WHERE verloopt < now()`)
	_, err := r.db.ExecContext(ctx, `INSERT INTO open_link (hash, email, ruimte_id, item_id, verloopt) VALUES ($1, $2, $3, $4, $5)`,
		hashSleutel(sleutel), email, ruimteID, itemID, time.Now().Add(openDuur))
	return sleutel, err
}

type OpenLink struct {
	Sessie   *Sessie
	RuimteID int64
	ItemID   int64
}

func (r *Register) OpenLinkVan(ctx context.Context, sleutel string) (*OpenLink, error) {
	var l OpenLink
	var email string
	err := r.db.QueryRowContext(ctx, `SELECT email, ruimte_id, item_id FROM open_link WHERE hash = $1 AND verloopt > now()`, hashSleutel(sleutel)).
		Scan(&email, &l.RuimteID, &l.ItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNietGevonden
	}
	if err != nil {
		return nil, err
	}
	l.Sessie, err = r.persoonSessie(ctx, email)
	return &l, err
}

func (r *Register) RuimteVanID(ctx context.Context, id int64) (*Ruimte, error) {
	var x Ruimte
	err := r.db.QueryRowContext(ctx, `SELECT id, sleutel, soort, naam, redmine_id, status FROM ruimte WHERE id = $1 AND status <> 'weg'`, id).
		Scan(&x.ID, &x.Sleutel, &x.Soort, &x.Naam, &x.RedmineID, &x.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNietGevonden
	}
	return &x, err
}
