package main

// Delen: iemand (of een groep) toegang geven tot een map of document en
// hem daar een bericht over sturen, via de berichten van de site
// (BuddyPress). De site stuurt zelf de e-mailmelding.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// WordPress: het eindpunt voor berichten in de mu-plugin dg-gemeenschap.php.
type WordPress struct {
	URL    string // http://wordpress
	Host   string // het domein van de site (WordPress kent alleen dat adres)
	Geheim string // DOC_BERICHT_GEHEIM, ook bij WordPress
	HTTP   *http.Client
}

type berichtUitslag struct {
	Verstuurd int      `json:"verstuurd"`
	Onbekend  []string `json:"onbekend"`
}

// Stuur: één bericht per ontvanger (geen groepsgesprek). Ontvangers op
// e-mailadres en op groep (de groepen uit Keycloak die WordPress bij elke
// login bijhoudt).
func (wp *WordPress) Stuur(ctx context.Context, van string, aan, groepen []string, onderwerp, inhoud string) (*berichtUitslag, error) {
	body, _ := json.Marshal(map[string]any{"van": van, "aan": aan, "groepen": groepen, "onderwerp": onderwerp, "inhoud": inhoud})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(wp.URL, "/")+"/?rest_route=/dg/v1/bericht", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Host = wp.Host
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-DG-Sleutel", wp.Geheim)
	resp, err := wp.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		var f struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		json.Unmarshal(b, &f)
		if f.Code == "dg_afzender" {
			return nil, errGeenAfzender
		}
		return nil, fmt.Errorf("wordpress %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var u berichtUitslag
	if err := json.Unmarshal(b, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

var errGeenAfzender = errors.New("afzender onbekend op de site")

// RegelDelen: zoals RegelToevoegen, maar verlaagt een bestaand recht niet.
func (r *Register) RegelDelen(ctx context.Context, rm *Ruimte, id int64, soort, wie string, niveau int, door string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO recht (ruimte_id, item_id, soort, wie, niveau, door) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (ruimte_id, coalesce(item_id, 0), soort, wie) DO UPDATE SET niveau = GREATEST(recht.niveau, EXCLUDED.niveau)`,
		rm.ID, id, soort, wie, niveau, door)
	if err != nil {
		return err
	}
	if err := logboek(ctx, tx, door, rm.ID, id, "gedeeld", soort+" "+wie+": "+niveauNaam(niveau)); err != nil {
		return err
	}
	return tx.Commit()
}

// ProjectLeden: de e-mailadressen van wie in Redmine toegang heeft tot dit project.
func (r *Register) ProjectLeden(ctx context.Context, rm *Ruimte) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT email FROM lid WHERE ruimte_id = $1 AND niveau > 0`, rm.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var uit []string
	for rows.Next() {
		var e string
		rows.Scan(&e)
		uit = append(uit, e)
	}
	return uit, rows.Err()
}

func (a *App) delen(w http.ResponseWriter, r *http.Request, s *Sessie, p *Plek) {
	x := a.item(w, r, s, p)
	if x == nil || !a.mag(w, r, s, p, x.ID, Beheren) {
		return
	}
	doel := veiligTerug(r.FormValue("terug"))
	soort, wie, _ := strings.Cut(r.FormValue("wie"), "|")
	niveau, _ := strconv.Atoi(r.FormValue("niveau"))
	if niveau != Lezen && niveau != Bewerken {
		niveau = Lezen
	}
	switch {
	case soort == "persoon" && strings.Contains(wie, "@"):
		wie = strings.ToLower(strings.TrimSpace(wie))
	case soort == "groep" && strings.HasPrefix(wie, "/") && wie != groepBeheer:
	case soort == "rol" && wie == "redmine" && p.Ruimte.Soort == "project":
	default:
		terug(w, r, doel, "fout", "Kies met wie je deelt.")
		return
	}
	bericht := strings.TrimSpace(r.FormValue("bericht"))
	if len(bericht) > 2000 {
		bericht = bericht[:2000]
	}
	if err := a.Reg.RegelDelen(r.Context(), p.Ruimte, x.ID, soort, wie, niveau, s.Email); err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	namen, _ := a.Reg.Namen(r.Context())
	ontvanger, _ := regelLabel(Regel{Soort: soort, Wie: wie}, namen)
	melding := fmt.Sprintf("Gedeeld met %s (%s).", ontvanger, strings.ToLower(niveauNaam(niveau)))

	if r.FormValue("dm") == "ja" {
		if a.Berichten == nil {
			melding += " Er is geen bericht gestuurd: berichten staan uit (DOC_BERICHT_GEHEIM)."
		} else {
			var aan, groepen []string
			switch soort {
			case "persoon":
				aan = []string{wie}
			case "groep":
				groepen = []string{wie}
			case "rol":
				var err error
				if aan, err = a.Reg.ProjectLeden(r.Context(), p.Ruimte); err != nil {
					a.interneFout(w, r, s, err)
					return
				}
			}
			pad := fmt.Sprintf("/r/%s/d/%d", p.Ruimte.Sleutel, x.ID)
			wat := "het document"
			if x.Soort == "map" {
				pad, wat = mapURL(p.Ruimte, x.ID), "de map"
			}
			link := a.Basis + pad
			inhoud := fmt.Sprintf("%s deelde %s <strong>%s</strong> met je, in %s. Je kunt het %s.",
				html.EscapeString(s.Naam), wat, html.EscapeString(x.Naam), html.EscapeString(p.Ruimte.Naam),
				map[int]string{Lezen: "lezen en downloaden", Bewerken: "lezen en bewerken"}[niveau])
			if bericht != "" {
				inhoud += "\n\n" + html.EscapeString(bericht)
			}
			inhoud += fmt.Sprintf("\n\n<a href=\"%s\">Openen in Documenten</a>", html.EscapeString(link))
			u, err := a.Berichten.Stuur(r.Context(), s.Email, aan, groepen, "Gedeeld: "+x.Naam, inhoud)
			switch {
			case errors.Is(err, errGeenAfzender):
				melding += " Er is geen bericht gestuurd: je hebt nog geen account op de site. Log daar eenmaal in, dan kan het wel."
			case err != nil:
				melding += " Het bericht kon niet worden verstuurd; stuur de link zelf."
				logFout(r, err)
			case u.Verstuurd == 0:
				melding += " Er is geen bericht gestuurd: de ontvanger heeft nog geen account op de site."
			default:
				melding += fmt.Sprintf(" Bericht gestuurd aan %d %s.", u.Verstuurd, map[bool]string{true: "lid", false: "leden"}[u.Verstuurd == 1])
				if len(u.Onbekend) > 0 {
					melding += fmt.Sprintf(" %d nog zonder account op de site.", len(u.Onbekend))
				}
			}
		}
	}
	terug(w, r, doel, "melding", melding)
}
