package main

// De groepen van wie Documenten gebruikt, live uit Keycloak (hooguit een
// minuut oud), niet alleen van het moment van inloggen. Zo telt een nieuwe
// rol meteen (Mijn documenten, de ruimte van de coöperatie), en een rol die
// iemand kwijtraakt ook: niet pas als de sessie na acht uur verloopt. Een
// geblokkeerd of verwijderd account is meteen uitgelogd, ook op de
// netwerkschijf en in de links voor Word en LibreOffice.
//
// Is Keycloak even niet bereikbaar, dan gelden de groepen die we al hadden
// (van de vorige controle of van het inloggen): Documenten blijft dan
// werken, met hooguit verouderde rechten. Dat staat in het log.

import (
	"context"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/lib/pq"
)

const groepenDuur = time.Minute

type groepenCache struct {
	mu          sync.Mutex
	alle        map[string]groepenMoment
	laatsteFout time.Time
}

type groepenMoment struct {
	groepen []string
	actief  bool
	tot     time.Time
}

// actueel: zet de groepen van s op wat Keycloak nu zegt. Onwaar: het account
// is geblokkeerd of bestaat niet meer.
func (a *App) actueel(ctx context.Context, s *Sessie) bool {
	if a.Keycloak == nil || s == nil {
		return true
	}
	c := &a.groepen
	c.mu.Lock()
	m, ok := c.alle[s.Email]
	c.mu.Unlock()
	if !ok || time.Now().After(m.tot) {
		groepen, actief, err := a.Keycloak.GroepenVan(ctx, s.Email)
		if err != nil {
			c.mu.Lock()
			if time.Since(c.laatsteFout) > time.Minute {
				c.laatsteFout = time.Now()
				log.Printf("groepen uit Keycloak: %v (de bekende groepen gelden)", err)
			}
			c.mu.Unlock()
			return true
		}
		m = groepenMoment{groepen: groepen, actief: actief, tot: time.Now().Add(groepenDuur)}
		c.mu.Lock()
		if c.alle == nil {
			c.alle = map[string]groepenMoment{}
		}
		c.alle[s.Email] = m
		c.mu.Unlock()
		if actief && !zelfdeGroepen(s.Groepen, groepen) {
			// Ook in het register: de andere sessies en de netwerkschijf.
			if err := a.Reg.ZetGroepen(ctx, s.Email, groepen); err != nil {
				log.Printf("groepen van %s bijwerken: %v", s.Email, err)
			}
		}
	}
	if !m.actief {
		return false
	}
	s.Groepen = m.groepen
	return true
}

func zelfdeGroepen(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// ZetGroepen: de groepen van deze persoon, in zijn sessies en in persoon
// (die de netwerkschijf gebruikt).
func (r *Register) ZetGroepen(ctx context.Context, email string, groepen []string) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE persoon SET groepen = $2 WHERE email = $1`, email, pq.Array(groepen)); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE sessie SET groepen = $2 WHERE email = $1`, email, pq.Array(groepen)); err != nil {
		return err
	}
	for _, g := range groepen {
		if _, err := r.db.ExecContext(ctx, `INSERT INTO groep (pad) VALUES ($1) ON CONFLICT (pad) DO UPDATE SET gezien = now()`, g); err != nil {
			return err
		}
	}
	return nil
}
