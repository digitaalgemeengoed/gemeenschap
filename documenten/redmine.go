package main

// Projecten en hun leden uit Redmine. Elk project krijgt een ruimte met de
// identifier als sleutel; wie lid is van het project (direct of via een
// groep), mag in de ruimte. Gekoppeld op e-mailadres.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lib/pq"
)

type Redmine struct {
	URL     string // http://redmine:3000
	Sleutel string // API-sleutel van het noodaccount admin
	HTTP    *http.Client
}

func (rm *Redmine) haal(ctx context.Context, pad string, q url.Values, doel any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(rm.URL, "/")+pad+"?"+q.Encode(), nil)
	req.Header.Set("X-Redmine-API-Key", rm.Sleutel)
	resp, err := rm.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("redmine %s: %s", pad, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(doel)
}

// alles haalt een lijst op in delen van 100.
func alles[T any](ctx context.Context, rm *Redmine, pad, veld string, q url.Values) ([]T, error) {
	var uit []T
	for offset := 0; ; offset += 100 {
		qq := url.Values{}
		for k, v := range q {
			qq[k] = v
		}
		qq.Set("limit", "100")
		qq.Set("offset", fmt.Sprint(offset))
		var deel map[string]json.RawMessage
		if err := rm.haal(ctx, pad, qq, &deel); err != nil {
			return nil, err
		}
		var lijst []T
		if err := json.Unmarshal(deel[veld], &lijst); err != nil {
			return nil, err
		}
		uit = append(uit, lijst...)
		var totaal int
		json.Unmarshal(deel["total_count"], &totaal)
		if len(lijst) == 0 || offset+len(lijst) >= totaal {
			return uit, nil
		}
	}
}

type rmProject struct {
	ID         int    `json:"id"`
	Naam       string `json:"name"`
	Identifier string `json:"identifier"`
	Status     int    `json:"status"`
}

type rmLidmaatschap struct {
	Gebruiker *struct{ ID int } `json:"user"`
	Groep     *struct{ ID int } `json:"group"`
}

// Synchroniseer neemt projecten en leden over. Projecten die er niet meer
// zijn (of gearchiveerd), krijgen status "weg"; hun documenten blijven.
func (rm *Redmine) Synchroniseer(ctx context.Context, reg *Register) error {
	gebruikers, err := alles[struct {
		ID   int    `json:"id"`
		Mail string `json:"mail"`
	}](ctx, rm, "/users.json", "users", url.Values{"status": {"1"}})
	if err != nil {
		return err
	}
	mail := map[int]string{}
	for _, g := range gebruikers {
		if g.Mail != "" {
			mail[g.ID] = strings.ToLower(g.Mail)
		}
	}
	groepLeden := map[int][]string{}
	ledenVanGroep := func(id int) ([]string, error) {
		if l, ok := groepLeden[id]; ok {
			return l, nil
		}
		var g struct {
			Groep struct {
				Users []struct{ ID int } `json:"users"`
			} `json:"group"`
		}
		if err := rm.haal(ctx, fmt.Sprintf("/groups/%d.json", id), url.Values{"include": {"users"}}, &g); err != nil {
			return nil, err
		}
		var l []string
		for _, u := range g.Groep.Users {
			if m := mail[u.ID]; m != "" {
				l = append(l, m)
			}
		}
		groepLeden[id] = l
		return l, nil
	}

	// Actief (1) en gesloten (5); gearchiveerd (9) komt niet mee.
	var projecten []rmProject
	for _, st := range []string{"1", "5"} {
		p, err := alles[rmProject](ctx, rm, "/projects.json", "projects", url.Values{"status": {st}})
		if err != nil {
			return err
		}
		projecten = append(projecten, p...)
	}

	gezien := map[int]bool{}
	for _, p := range projecten {
		if gezien[p.ID] {
			continue
		}
		gezien[p.ID] = true
		lms, err := alles[rmLidmaatschap](ctx, rm, fmt.Sprintf("/projects/%d/memberships.json", p.ID), "memberships", nil)
		if err != nil {
			return err
		}
		leden := map[string]bool{}
		for _, lm := range lms {
			switch {
			case lm.Gebruiker != nil:
				if m := mail[lm.Gebruiker.ID]; m != "" {
					leden[m] = true
				}
			case lm.Groep != nil:
				l, err := ledenVanGroep(lm.Groep.ID)
				if err != nil {
					return err
				}
				for _, m := range l {
					leden[m] = true
				}
			}
		}
		status := "actief"
		if p.Status == 5 {
			status = "gesloten"
		}
		if err := reg.ZetProject(ctx, p, status, leden); err != nil {
			return fmt.Errorf("project %s: %w", p.Identifier, err)
		}
	}
	return reg.WegBehalve(ctx, gezien)
}

func (r *Register) ZetProject(ctx context.Context, p rmProject, status string, leden map[string]bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM ruimte WHERE redmine_id = $1`, p.ID).Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) {
		// Nieuw. De sleutel is de identifier; "cooperatie" is bezet.
		sleutel := p.Identifier
		if sleutel == "cooperatie" {
			sleutel = "project-cooperatie"
		}
		if err := tx.QueryRowContext(ctx, `INSERT INTO ruimte (sleutel, soort, naam, redmine_id, status) VALUES ($1, 'project', $2, $3, $4) RETURNING id`,
			sleutel, p.Naam, p.ID, status).Scan(&id); err != nil {
			return err
		}
		log.Printf("ruimte %s aangemaakt (Redmine-project %d)", sleutel, p.ID)
	} else if _, err := tx.ExecContext(ctx, `UPDATE ruimte SET naam = $2, status = $3 WHERE id = $1`, id, p.Naam, status); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM lid WHERE ruimte_id = $1`, id); err != nil {
		return err
	}
	for m := range leden {
		if _, err := tx.ExecContext(ctx, `INSERT INTO lid (ruimte_id, email) VALUES ($1, $2)`, id, m); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Register) WegBehalve(ctx context.Context, gezien map[int]bool) error {
	ids := make([]int64, 0, len(gezien))
	for id := range gezien {
		ids = append(ids, int64(id))
	}
	_, err := r.db.ExecContext(ctx, `UPDATE ruimte SET status = 'weg' WHERE soort = 'project' AND status <> 'weg' AND NOT (redmine_id = ANY($1))`, pq.Array(ids))
	return err
}

// SynchroniseerSteeds: meteen, en daarna elke interval.
func (rm *Redmine) SynchroniseerSteeds(ctx context.Context, reg *Register, interval time.Duration) {
	for {
		if err := rm.Synchroniseer(ctx, reg); err != nil {
			log.Printf("Redmine: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}
