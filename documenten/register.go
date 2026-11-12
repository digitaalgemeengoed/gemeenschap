package main

// Het register: ruimtes, leden, mappen en documenten, versies, het logboek en
// de sessies. De bytes staan in de objectopslag; hier staat wat ze zijn.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

// schema: elke stap één keer, in volgorde. Alleen achteraan toevoegen.
var schema = []string{
	`CREATE TABLE ruimte (
		id          bigserial PRIMARY KEY,
		sleutel     text NOT NULL UNIQUE,           -- in de URL: cooperatie, of de identifier van het project
		soort       text NOT NULL CHECK (soort IN ('cooperatie','project')),
		naam        text NOT NULL,
		redmine_id  integer UNIQUE,
		status      text NOT NULL DEFAULT 'actief' CHECK (status IN ('actief','gesloten','weg')),
		aangemaakt  timestamptz NOT NULL DEFAULT now()
	)`,
	`CREATE TABLE lid (
		ruimte_id   bigint NOT NULL REFERENCES ruimte(id) ON DELETE CASCADE,
		email       text NOT NULL,                  -- klein geschreven
		PRIMARY KEY (ruimte_id, email)
	)`,
	`CREATE TABLE item (
		id               bigserial PRIMARY KEY,
		ruimte_id        bigint NOT NULL REFERENCES ruimte(id),
		ouder_id         bigint REFERENCES item(id),
		soort            text NOT NULL CHECK (soort IN ('map','document')),
		naam             text NOT NULL,
		aangemaakt       timestamptz NOT NULL DEFAULT now(),
		aangemaakt_door  text NOT NULL,
		gewijzigd        timestamptz NOT NULL DEFAULT now(),
		gewijzigd_door   text NOT NULL,
		verwijderd_op    timestamptz,
		verwijderd_door  text,
		-- metadata (documenten)
		titel            text NOT NULL DEFAULT '',
		omschrijving     text NOT NULL DEFAULT '',
		documentsoort    text NOT NULL DEFAULT '',
		trefwoorden      text[] NOT NULL DEFAULT '{}',
		huidige_versie   integer NOT NULL DEFAULT 0
	)`,
	// Eén naam per map (hoofdletters tellen niet), alleen voor wat niet verwijderd is.
	`CREATE UNIQUE INDEX item_naam ON item (ruimte_id, coalesce(ouder_id, 0), lower(naam)) WHERE verwijderd_op IS NULL`,
	`CREATE INDEX item_ouder ON item (ruimte_id, ouder_id)`,
	`CREATE TABLE versie (
		item_id      bigint NOT NULL REFERENCES item(id),
		nummer       integer NOT NULL,
		object       text NOT NULL UNIQUE,           -- sleutel in de objectopslag
		sha256       text NOT NULL,
		grootte      bigint NOT NULL,
		mediatype    text NOT NULL,
		door         text NOT NULL,
		op           timestamptz NOT NULL DEFAULT now(),
		PRIMARY KEY (item_id, nummer)
	)`,
	`CREATE TABLE logboek (
		id         bigserial PRIMARY KEY,
		op         timestamptz NOT NULL DEFAULT now(),
		wie        text NOT NULL,
		ruimte_id  bigint REFERENCES ruimte(id),
		item_id    bigint REFERENCES item(id),
		actie      text NOT NULL,
		details    text NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE sessie (
		id         text PRIMARY KEY,                -- SHA-256 van de cookie, niet de cookie zelf
		email      text NOT NULL,
		naam       text NOT NULL,
		groepen    text[] NOT NULL,
		id_token   text NOT NULL,
		csrf       text NOT NULL,
		verloopt   timestamptz NOT NULL
	)`,
	`INSERT INTO ruimte (sleutel, soort, naam) VALUES ('cooperatie', 'cooperatie', 'Coöperatie')`,
	// Namen bij e-mailadressen, voor de weergave (bijgewerkt bij elke login).
	`CREATE TABLE persoon (email text PRIMARY KEY, naam text NOT NULL)`,
	// Rechten (0.10): drie niveaus per persoon, groep of de projectleden, op een
	// map of op de ruimte zelf (item_id leeg). Een map kan de overerving stoppen.
	`ALTER TABLE item ADD COLUMN eigen_rechten boolean NOT NULL DEFAULT false`,
	`ALTER TABLE lid ADD COLUMN niveau smallint NOT NULL DEFAULT 2`,
	`CREATE TABLE recht (
		id         bigserial PRIMARY KEY,
		ruimte_id  bigint NOT NULL REFERENCES ruimte(id) ON DELETE CASCADE,
		item_id    bigint REFERENCES item(id),
		soort      text NOT NULL CHECK (soort IN ('groep','persoon','rol')),
		wie        text NOT NULL,                   -- groepspad, e-mailadres, of 'redmine'
		niveau     smallint NOT NULL CHECK (niveau BETWEEN 1 AND 3),
		door       text NOT NULL,
		op         timestamptz NOT NULL DEFAULT now()
	)`,
	`CREATE UNIQUE INDEX recht_uniek ON recht (ruimte_id, coalesce(item_id, 0), soort, wie)`,
	// Groepen die bij het inloggen voorbijkwamen: om toegang aan te kunnen geven.
	`CREATE TABLE groep (pad text PRIMARY KEY, gezien timestamptz NOT NULL DEFAULT now())`,
	// WebDAV (0.11): de groepen van de laatste login (een WebDAV-programma
	// heeft geen token uit Keycloak), app-wachtwoorden en tijdelijke links.
	`ALTER TABLE persoon ADD COLUMN groepen text[] NOT NULL DEFAULT '{}'`,
	`CREATE TABLE app_wachtwoord (
		id          bigserial PRIMARY KEY,
		email       text NOT NULL,
		naam        text NOT NULL,                  -- het apparaat, zoals de gebruiker het noemt
		hash        text NOT NULL UNIQUE,           -- SHA-256 van het wachtwoord
		aangemaakt  timestamptz NOT NULL DEFAULT now(),
		gebruikt    timestamptz
	)`,
	`CREATE TABLE open_link (
		hash        text PRIMARY KEY,               -- SHA-256 van de sleutel in de link
		email       text NOT NULL,
		ruimte_id   bigint NOT NULL REFERENCES ruimte(id) ON DELETE CASCADE,
		item_id     bigint NOT NULL REFERENCES item(id),
		verloopt    timestamptz NOT NULL
	)`,
	// Werkgroepen en persoonlijke ruimtes (0.11); quota; de rol "leden" is
	// algemeen (projectleden uit Redmine, werkgroepleden uit WordPress).
	`ALTER TABLE ruimte DROP CONSTRAINT ruimte_soort_check`,
	`ALTER TABLE ruimte ADD CONSTRAINT ruimte_soort_check CHECK (soort IN ('cooperatie','project','werkgroep','persoonlijk'))`,
	`ALTER TABLE ruimte ADD COLUMN wp_groep_id integer UNIQUE, ADD COLUMN eigenaar text UNIQUE, ADD COLUMN quotum bigint, ADD COLUMN bevroren_sinds timestamptz`,
	`UPDATE recht SET wie = 'leden' WHERE soort = 'rol'`,
	// Wie in de kerngroep zit (uit de groep "Netwerk: Kerngroep" in Redmine).
	`CREATE TABLE kerngroep (email text PRIMARY KEY)`,
	// Ook de beheerders van het platform hebben altijd recht op een
	// persoonlijke ruimte: de tabel bevat nu beide groepen uit Keycloak.
	`ALTER TABLE kerngroep RENAME TO vaste_leden`,
	// Zoeken (0.14): wat opnieuw in de index moet (zoeken.go). Triggers
	// houden de rij bij, zodat geen enkele wijziging buiten de index blijft.
	`CREATE TABLE zoek_wachtrij (item_id bigint PRIMARY KEY, sinds timestamptz NOT NULL DEFAULT clock_timestamp());
	CREATE TABLE zoek_stand (item_id bigint PRIMARY KEY, versie integer NOT NULL, op timestamptz NOT NULL DEFAULT now());
	CREATE FUNCTION zoek_item() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN
		IF TG_OP = 'DELETE' THEN
			INSERT INTO zoek_wachtrij (item_id) VALUES (OLD.id)
				ON CONFLICT (item_id) DO UPDATE SET sinds = clock_timestamp();
			RETURN OLD;
		END IF;
		INSERT INTO zoek_wachtrij (item_id) VALUES (NEW.id)
			ON CONFLICT (item_id) DO UPDATE SET sinds = clock_timestamp();
		-- Een map met een andere naam of plek, of (terug uit de) prullenbak: alles eronder.
		IF TG_OP = 'UPDATE' AND NEW.soort = 'map' AND (NEW.naam IS DISTINCT FROM OLD.naam
				OR NEW.ouder_id IS DISTINCT FROM OLD.ouder_id OR NEW.verwijderd_op IS DISTINCT FROM OLD.verwijderd_op) THEN
			INSERT INTO zoek_wachtrij (item_id)
				WITH RECURSIVE onder AS (
					SELECT id FROM item WHERE ouder_id = NEW.id
					UNION SELECT i.id FROM item i JOIN onder o ON i.ouder_id = o.id)
				SELECT id FROM onder
				ON CONFLICT (item_id) DO UPDATE SET sinds = clock_timestamp();
		END IF;
		RETURN NEW;
	END $$;
	CREATE TRIGGER zoek_item AFTER INSERT OR UPDATE OR DELETE ON item FOR EACH ROW EXECUTE FUNCTION zoek_item();
	CREATE FUNCTION zoek_ruimte() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN
		IF NEW.sleutel IS DISTINCT FROM OLD.sleutel OR NEW.status IS DISTINCT FROM OLD.status THEN
			INSERT INTO zoek_wachtrij (item_id) SELECT id FROM item WHERE ruimte_id = NEW.id
				ON CONFLICT (item_id) DO UPDATE SET sinds = clock_timestamp();
		END IF;
		RETURN NEW;
	END $$;
	CREATE TRIGGER zoek_ruimte AFTER UPDATE ON ruimte FOR EACH ROW EXECUTE FUNCTION zoek_ruimte();
	INSERT INTO zoek_wachtrij (item_id) SELECT id FROM item WHERE verwijderd_op IS NULL`,
}

func migreer(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_stap (stap integer PRIMARY KEY, op timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Niet tegelijk met een tweede exemplaar.
	if _, err := tx.ExecContext(ctx, `LOCK TABLE schema_stap IN EXCLUSIVE MODE`); err != nil {
		return err
	}
	var gedaan int
	if err := tx.QueryRowContext(ctx, `SELECT coalesce(max(stap), 0) FROM schema_stap`).Scan(&gedaan); err != nil {
		return err
	}
	for i := gedaan; i < len(schema); i++ {
		if _, err := tx.ExecContext(ctx, schema[i]); err != nil {
			return fmt.Errorf("schema stap %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_stap (stap) VALUES ($1)`, i+1); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type Ruimte struct {
	ID            int64
	Sleutel       string
	Soort         string // cooperatie, project, werkgroep, persoonlijk
	Naam          string
	RedmineID     sql.NullInt64
	Status        string // actief, gesloten (alleen lezen), weg
	Eigenaar      sql.NullString
	Quotum        sql.NullInt64 // bytes
	BevrorenSinds sql.NullTime
}

const ruimteKolommen = `id, sleutel, soort, naam, redmine_id, status, eigenaar, quotum, bevroren_sinds`

func scanRuimte(s interface{ Scan(...any) error }, x *Ruimte) error {
	return s.Scan(&x.ID, &x.Sleutel, &x.Soort, &x.Naam, &x.RedmineID, &x.Status, &x.Eigenaar, &x.Quotum, &x.BevrorenSinds)
}

type Item struct {
	ID             int64
	RuimteID       int64
	OuderID        sql.NullInt64
	Soort          string
	Naam           string
	Aangemaakt     time.Time
	AangemaaktDoor string
	Gewijzigd      time.Time
	GewijzigdDoor  string
	Titel          string
	Omschrijving   string
	Documentsoort  string
	Trefwoorden    []string
	HuidigeVersie  int
	// Bij een lijst: van de huidige versie.
	Grootte   int64
	Mediatype string
	// Namen bij de e-mailadressen.
	AangemaaktDoorNaam string
	GewijzigdDoorNaam  string
}

type Versie struct {
	ItemID    int64
	Nummer    int
	Object    string
	SHA256    string
	Grootte   int64
	Mediatype string
	Door      string
	DoorNaam  string
	Op        time.Time
}

type Register struct{ db *sql.DB }

func (r *Register) Ruimtes(ctx context.Context) ([]Ruimte, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+ruimteKolommen+` FROM ruimte WHERE status <> 'weg' ORDER BY soort, lower(naam)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var uit []Ruimte
	for rows.Next() {
		var x Ruimte
		if err := scanRuimte(rows, &x); err != nil {
			return nil, err
		}
		uit = append(uit, x)
	}
	return uit, rows.Err()
}

func (r *Register) Ruimte(ctx context.Context, sleutel string) (*Ruimte, error) {
	var x Ruimte
	err := scanRuimte(r.db.QueryRowContext(ctx, `SELECT `+ruimteKolommen+` FROM ruimte WHERE sleutel = $1 AND status <> 'weg'`, sleutel), &x)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNietGevonden
	}
	return &x, err
}

// Lid: staat dit e-mailadres bij deze ruimte (uit Redmine)?
func (r *Register) Lid(ctx context.Context, ruimteID int64, email string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM lid WHERE ruimte_id = $1 AND email = $2`, ruimteID, strings.ToLower(email)).Scan(&n)
	return n > 0, err
}

// LidVan: de ruimtes (id's) waar dit e-mailadres lid van is.
func (r *Register) LidVan(ctx context.Context, email string) (map[int64]bool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT ruimte_id FROM lid WHERE email = $1`, strings.ToLower(email))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	uit := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		uit[id] = true
	}
	return uit, rows.Err()
}

const itemKolommen = `i.id, i.ruimte_id, i.ouder_id, i.soort, i.naam, i.aangemaakt, i.aangemaakt_door, i.gewijzigd, i.gewijzigd_door,
	i.titel, i.omschrijving, i.documentsoort, i.trefwoorden, i.huidige_versie, coalesce(v.grootte, 0), coalesce(v.mediatype, ''),
	coalesce(pa.naam, i.aangemaakt_door), coalesce(pg.naam, i.gewijzigd_door)`

const itemBron = ` FROM item i
	LEFT JOIN versie v ON v.item_id = i.id AND v.nummer = i.huidige_versie
	LEFT JOIN persoon pa ON pa.email = i.aangemaakt_door
	LEFT JOIN persoon pg ON pg.email = i.gewijzigd_door`

func scanItem(s interface{ Scan(...any) error }) (*Item, error) {
	var x Item
	err := s.Scan(&x.ID, &x.RuimteID, &x.OuderID, &x.Soort, &x.Naam, &x.Aangemaakt, &x.AangemaaktDoor, &x.Gewijzigd, &x.GewijzigdDoor,
		&x.Titel, &x.Omschrijving, &x.Documentsoort, pq.Array(&x.Trefwoorden), &x.HuidigeVersie, &x.Grootte, &x.Mediatype,
		&x.AangemaaktDoorNaam, &x.GewijzigdDoorNaam)
	return &x, err
}

// Inhoud van een map (ouder 0 = de bovenste laag): eerst mappen, dan documenten.
func (r *Register) Inhoud(ctx context.Context, ruimteID, ouder int64) ([]*Item, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+itemKolommen+itemBron+`
		WHERE i.ruimte_id = $1 AND coalesce(i.ouder_id, 0) = $2 AND i.verwijderd_op IS NULL
		ORDER BY i.soort DESC, lower(i.naam)`, ruimteID, ouder)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var uit []*Item
	for rows.Next() {
		x, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		uit = append(uit, x)
	}
	return uit, rows.Err()
}

func (r *Register) Item(ctx context.Context, ruimteID, id int64) (*Item, error) {
	x, err := scanItem(r.db.QueryRowContext(ctx, `SELECT `+itemKolommen+itemBron+`
		WHERE i.ruimte_id = $1 AND i.id = $2 AND i.verwijderd_op IS NULL`, ruimteID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNietGevonden
	}
	return x, err
}

// Pad: de mappen van boven naar beneden tot en met dit item.
func (r *Register) Pad(ctx context.Context, ruimteID, id int64) ([]*Item, error) {
	var pad []*Item
	for id != 0 && len(pad) < 100 {
		x, err := r.Item(ctx, ruimteID, id)
		if err != nil {
			return nil, err
		}
		pad = append([]*Item{x}, pad...)
		id = x.OuderID.Int64
	}
	return pad, nil
}

func (r *Register) Versies(ctx context.Context, itemID int64) ([]Versie, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT item_id, nummer, object, sha256, grootte, mediatype, door, coalesce(p.naam, door), op FROM versie LEFT JOIN persoon p ON p.email = door WHERE item_id = $1 ORDER BY nummer DESC`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var uit []Versie
	for rows.Next() {
		var v Versie
		if err := rows.Scan(&v.ItemID, &v.Nummer, &v.Object, &v.SHA256, &v.Grootte, &v.Mediatype, &v.Door, &v.DoorNaam, &v.Op); err != nil {
			return nil, err
		}
		uit = append(uit, v)
	}
	return uit, rows.Err()
}

func (r *Register) Versie(ctx context.Context, itemID int64, nummer int) (*Versie, error) {
	var v Versie
	err := r.db.QueryRowContext(ctx, `SELECT item_id, nummer, object, sha256, grootte, mediatype, door, op FROM versie WHERE item_id = $1 AND nummer = $2`, itemID, nummer).
		Scan(&v.ItemID, &v.Nummer, &v.Object, &v.SHA256, &v.Grootte, &v.Mediatype, &v.Door, &v.Op)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNietGevonden
	}
	return &v, err
}

var errBestaatAl = errors.New("bestaat al")
var errIsMap = errors.New("er staat al een map met die naam")

func logboek(ctx context.Context, tx *sql.Tx, wie string, ruimteID int64, itemID int64, actie, details string) error {
	var item any
	if itemID != 0 {
		item = itemID
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO logboek (wie, ruimte_id, item_id, actie, details) VALUES ($1, $2, $3, $4, $5)`, wie, ruimteID, item, actie, details)
	return err
}

func nullOuder(ouder int64) any {
	if ouder == 0 {
		return nil
	}
	return ouder
}

// controleerOuder: bestaat de map, in deze ruimte, en is het een map?
func controleerOuder(ctx context.Context, tx *sql.Tx, ruimteID, ouder int64) error {
	if ouder == 0 {
		return nil
	}
	var soort string
	err := tx.QueryRowContext(ctx, `SELECT soort FROM item WHERE id = $1 AND ruimte_id = $2 AND verwijderd_op IS NULL`, ouder, ruimteID).Scan(&soort)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && soort != "map") {
		return errNietGevonden
	}
	return err
}

func isUniek(err error) bool {
	var pe *pq.Error
	return errors.As(err, &pe) && pe.Code == "23505"
}

func (r *Register) NieuweMap(ctx context.Context, ruimteID, ouder int64, naam, wie string) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := controleerOuder(ctx, tx, ruimteID, ouder); err != nil {
		return 0, err
	}
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO item (ruimte_id, ouder_id, soort, naam, aangemaakt_door, gewijzigd_door) VALUES ($1, $2, 'map', $3, $4, $4) RETURNING id`,
		ruimteID, nullOuder(ouder), naam, wie).Scan(&id)
	if isUniek(err) {
		return 0, errBestaatAl
	}
	if err != nil {
		return 0, err
	}
	if err := logboek(ctx, tx, wie, ruimteID, id, "map aangemaakt", naam); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// NieuweVersie legt een upload vast: een nieuw document, of een nieuwe versie
// van het document met dezelfde naam in dezelfde map.
func (r *Register) NieuweVersie(ctx context.Context, ruimteID, ouder int64, naam, wie string, v Versie) (itemID int64, nummer int, err error) {
	return r.nieuweVersie(ctx, ruimteID, ouder, naam, wie, v, false)
}

// samenvoegTijd: slaat dezelfde persoon binnen deze tijd opnieuw op via
// WebDAV (Word en LibreOffice slaan vaak tussentijds op), dan vervangt dat de
// laatste versie in plaats van een nieuwe te maken. Het oude object blijft in
// de opslag staan; het register verwijst er alleen niet meer naar.
const samenvoegTijd = "10 minutes"

func (r *Register) nieuweVersie(ctx context.Context, ruimteID, ouder int64, naam, wie string, v Versie, samenvoegen bool) (itemID int64, nummer int, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()
	if err := controleerOuder(ctx, tx, ruimteID, ouder); err != nil {
		return 0, 0, err
	}
	var soort string
	err = tx.QueryRowContext(ctx, `SELECT id, soort, huidige_versie FROM item
		WHERE ruimte_id = $1 AND coalesce(ouder_id, 0) = $2 AND lower(naam) = lower($3) AND verwijderd_op IS NULL FOR UPDATE`,
		ruimteID, ouder, naam).Scan(&itemID, &soort, &nummer)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		titel := naam
		if i := strings.LastIndex(titel, "."); i > 0 {
			titel = titel[:i]
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO item (ruimte_id, ouder_id, soort, naam, aangemaakt_door, gewijzigd_door, titel) VALUES ($1, $2, 'document', $3, $4, $4, $5) RETURNING id`,
			ruimteID, nullOuder(ouder), naam, wie, titel).Scan(&itemID)
		if isUniek(err) {
			return 0, 0, errBestaatAl // tegelijk door een ander aangemaakt; opnieuw proberen
		}
		if err != nil {
			return 0, 0, err
		}
		nummer = 0
	case err != nil:
		return 0, 0, err
	case soort == "map":
		return 0, 0, errIsMap
	}
	if samenvoegen && nummer > 0 {
		res, err := tx.ExecContext(ctx, `UPDATE versie SET object = $3, sha256 = $4, grootte = $5, mediatype = $6, op = now()
			WHERE item_id = $1 AND nummer = $2 AND door = $7 AND op > now() - interval '`+samenvoegTijd+`'`,
			itemID, nummer, v.Object, v.SHA256, v.Grootte, v.Mediatype, wie)
		if err != nil {
			return 0, 0, err
		}
		if n, _ := res.RowsAffected(); n == 1 {
			if _, err := tx.ExecContext(ctx, `UPDATE item SET gewijzigd = now(), gewijzigd_door = $2 WHERE id = $1`, itemID, wie); err != nil {
				return 0, 0, err
			}
			if err := logboek(ctx, tx, wie, ruimteID, itemID, "versie bijgewerkt", fmt.Sprintf("%s, versie %d, %d bytes, sha256 %s (via WebDAV)", naam, nummer, v.Grootte, v.SHA256)); err != nil {
				return 0, 0, err
			}
			return itemID, nummer, tx.Commit()
		}
	}
	nummer++
	if _, err := tx.ExecContext(ctx, `INSERT INTO versie (item_id, nummer, object, sha256, grootte, mediatype, door) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		itemID, nummer, v.Object, v.SHA256, v.Grootte, v.Mediatype, wie); err != nil {
		return 0, 0, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE item SET huidige_versie = $2, gewijzigd = now(), gewijzigd_door = $3 WHERE id = $1`, itemID, nummer, wie); err != nil {
		return 0, 0, err
	}
	actie := "nieuwe versie"
	if nummer == 1 {
		actie = "document toegevoegd"
	}
	if err := logboek(ctx, tx, wie, ruimteID, itemID, actie, fmt.Sprintf("%s, versie %d, %d bytes, sha256 %s", naam, nummer, v.Grootte, v.SHA256)); err != nil {
		return 0, 0, err
	}
	return itemID, nummer, tx.Commit()
}

func (r *Register) Metadata(ctx context.Context, ruimteID, id int64, titel, omschrijving, soort string, trefwoorden []string, wie string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE item SET titel = $3, omschrijving = $4, documentsoort = $5, trefwoorden = $6, gewijzigd = now(), gewijzigd_door = $7
		WHERE id = $1 AND ruimte_id = $2 AND soort = 'document' AND verwijderd_op IS NULL`, id, ruimteID, titel, omschrijving, soort, pq.Array(trefwoorden), wie)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNietGevonden
	}
	if err := logboek(ctx, tx, wie, ruimteID, id, "metadata gewijzigd", ""); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Register) Hernoem(ctx context.Context, ruimteID, id int64, naam, wie string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var oud string
	if err := tx.QueryRowContext(ctx, `SELECT naam FROM item WHERE id = $1 AND ruimte_id = $2 AND verwijderd_op IS NULL FOR UPDATE`, id, ruimteID).Scan(&oud); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNietGevonden
		}
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE item SET naam = $3, gewijzigd = now(), gewijzigd_door = $4 WHERE id = $1 AND ruimte_id = $2`, id, ruimteID, naam, wie)
	if isUniek(err) {
		return errBestaatAl
	}
	if err != nil {
		return err
	}
	if err := logboek(ctx, tx, wie, ruimteID, id, "hernoemd", oud+" → "+naam); err != nil {
		return err
	}
	return tx.Commit()
}

// Verwijder zet een item (en bij een map alles eronder) in de prullenbak.
// Er verdwijnt niets: versies en objecten blijven.
func (r *Register) Verwijder(ctx context.Context, ruimteID, id int64, wie string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `WITH RECURSIVE boom AS (
			SELECT id FROM item WHERE id = $1 AND ruimte_id = $2 AND verwijderd_op IS NULL
			UNION ALL
			SELECT i.id FROM item i JOIN boom b ON i.ouder_id = b.id WHERE i.verwijderd_op IS NULL)
		UPDATE item SET verwijderd_op = now(), verwijderd_door = $3 WHERE id IN (SELECT id FROM boom)`, id, ruimteID, wie)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errNietGevonden
	}
	if err := logboek(ctx, tx, wie, ruimteID, id, "verwijderd", fmt.Sprintf("%d item(s) naar de prullenbak", n)); err != nil {
		return err
	}
	return tx.Commit()
}

// Telling per ruimte: aantal documenten en totale grootte (huidige versies).
func (r *Register) Tellingen(ctx context.Context) (map[int64][2]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT i.ruimte_id, count(*), coalesce(sum(v.grootte), 0) FROM item i
		JOIN versie v ON v.item_id = i.id AND v.nummer = i.huidige_versie
		WHERE i.soort = 'document' AND i.verwijderd_op IS NULL GROUP BY i.ruimte_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	uit := map[int64][2]int64{}
	for rows.Next() {
		var id, n, g int64
		if err := rows.Scan(&id, &n, &g); err != nil {
			return nil, err
		}
		uit[id] = [2]int64{n, g}
	}
	return uit, rows.Err()
}

// Objecten: alle objectsleutels die het register kent.
func (r *Register) Objecten(ctx context.Context) (map[string]Versie, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT object, sha256, grootte FROM versie`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	uit := map[string]Versie{}
	for rows.Next() {
		var v Versie
		if err := rows.Scan(&v.Object, &v.SHA256, &v.Grootte); err != nil {
			return nil, err
		}
		uit[v.Object] = v
	}
	return uit, rows.Err()
}

// objectSleutel: één bucket, een voorvoegsel per ruimte.
func objectSleutel(rm *Ruimte) string {
	return fmt.Sprintf("objecten/r%d/%s", rm.ID, nieuwID())
}

var errVol = errors.New("de ruimte is vol")

// Gebruik: alle versies van wat niet in de prullenbak staat (bytes).
func (r *Register) Gebruik(ctx context.Context, ruimteID int64) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx, `SELECT coalesce(sum(v.grootte), 0) FROM versie v JOIN item i ON i.id = v.item_id
		WHERE i.ruimte_id = $1 AND i.verwijderd_op IS NULL`, ruimteID).Scan(&n)
	return n, err
}

// PastErBij: past er nog zoveel bij binnen het quotum van de ruimte?
func (r *Register) PastErBij(ctx context.Context, rm *Ruimte, erbij int64) error {
	if !rm.Quotum.Valid {
		return nil
	}
	nu, err := r.Gebruik(ctx, rm.ID)
	if err != nil {
		return err
	}
	if nu+erbij > rm.Quotum.Int64 {
		return fmt.Errorf("%w: %s van %s gebruikt", errVol, grootte(nu), grootte(rm.Quotum.Int64))
	}
	return nil
}
