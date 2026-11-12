package main

// Werkgroepen: de groepen van BuddyPress op de site. Elke werkgroep krijgt
// een ruimte "werkgroep-<slug>"; lid = Bewerken, beheerder of moderator van
// de groep = Beheren (de kerngroep heeft ook Beheren, zoals overal).

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/lib/pq"
)

type wpWerkgroep struct {
	ID         int      `json:"id"`
	Slug       string   `json:"slug"`
	Naam       string   `json:"naam"`
	Leden      []string `json:"leden"`
	Beheerders []string `json:"beheerders"`
}

func (wp *WordPress) Werkgroepen(ctx context.Context) ([]wpWerkgroep, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(wp.URL, "/")+"/?rest_route=/dg/v1/werkgroepen", nil)
	if err != nil {
		return nil, err
	}
	req.Host = wp.Host
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-DG-Sleutel", wp.Geheim)
	resp, err := wp.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wordpress %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var uit struct {
		Werkgroepen []wpWerkgroep `json:"werkgroepen"`
	}
	if err := json.Unmarshal(b, &uit); err != nil {
		return nil, err
	}
	return uit.Werkgroepen, nil
}

func (wp *WordPress) SynchroniseerWerkgroepen(ctx context.Context, reg *Register) error {
	groepen, err := wp.Werkgroepen(ctx)
	if err != nil {
		return err
	}
	gezien := map[int]bool{}
	for _, g := range groepen {
		if g.ID == 0 || g.Slug == "" {
			continue
		}
		gezien[g.ID] = true
		leden := map[string]int{}
		for _, e := range g.Leden {
			leden[strings.ToLower(e)] = Bewerken
		}
		for _, e := range g.Beheerders {
			leden[strings.ToLower(e)] = Beheren
		}
		if err := reg.ZetWerkgroep(ctx, g, leden); err != nil {
			return fmt.Errorf("werkgroep %s: %w", g.Slug, err)
		}
	}
	ids := make([]int64, 0, len(gezien))
	for id := range gezien {
		ids = append(ids, int64(id))
	}
	_, err = reg.db.ExecContext(ctx, `UPDATE ruimte SET status = 'weg' WHERE soort = 'werkgroep' AND status <> 'weg' AND NOT (wp_groep_id = ANY($1))`, pq.Array(ids))
	return err
}

func (r *Register) ZetWerkgroep(ctx context.Context, g wpWerkgroep, leden map[string]int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// WordPress geeft namen als HTML ("Developers &amp; contributors").
	g.Naam = html.UnescapeString(g.Naam)
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM ruimte WHERE wp_groep_id = $1`, g.ID).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if err := tx.QueryRowContext(ctx, `INSERT INTO ruimte (sleutel, soort, naam, wp_groep_id, status) VALUES ($1, 'werkgroep', $2, $3, 'actief') RETURNING id`,
			"werkgroep-"+g.Slug, g.Naam, g.ID).Scan(&id); err != nil {
			return err
		}
		log.Printf("ruimte werkgroep-%s aangemaakt (werkgroep %d)", g.Slug, g.ID)
	case err != nil:
		return err
	default:
		if _, err := tx.ExecContext(ctx, `UPDATE ruimte SET naam = $2, status = 'actief' WHERE id = $1`, id, g.Naam); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM lid WHERE ruimte_id = $1`, id); err != nil {
		return err
	}
	for e, n := range leden {
		if _, err := tx.ExecContext(ctx, `INSERT INTO lid (ruimte_id, email, niveau) VALUES ($1, $2, $3)`, id, e, n); err != nil {
			return err
		}
	}
	return tx.Commit()
}
