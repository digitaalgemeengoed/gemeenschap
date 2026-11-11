package main

// Sessies in het geheugen. Een herstart van de dienst logt iedereen uit; voor
// een beheerscherm is dat prima, en zo staat er nergens een sessie op schijf.
// Kort: twee uur in totaal, een half uur zonder iets te doen.

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

const (
	sessieDuur = 2 * time.Hour
	sessieRust = 30 * time.Minute
)

type Sessie struct {
	Sub     string // de id van de gebruiker in Keycloak
	Email   string
	Naam    string
	IDToken string
	CSRF    string
	Rol     Rol // bij elk verzoek opnieuw bepaald (zie App.metSessie)
	tot     time.Time
	laatst  time.Time
}

type Sessies struct {
	mu   sync.Mutex
	alle map[string]*Sessie
}

func NieuweSessies() *Sessies { return &Sessies{alle: map[string]*Sessie{}} }

func sessieSleutel(cookie string) string {
	h := sha256.Sum256([]byte(cookie))
	return hex.EncodeToString(h[:])
}

func (ss *Sessies) Nieuw(p *Persoon) (cookie string) {
	cookie = willekeurig(32)
	nu := time.Now()
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for k, s := range ss.alle {
		if nu.After(s.tot) || nu.Sub(s.laatst) > sessieRust {
			delete(ss.alle, k)
		}
	}
	ss.alle[sessieSleutel(cookie)] = &Sessie{Sub: p.Sub, Email: p.Email, Naam: p.Naam, IDToken: p.IDToken, CSRF: willekeurig(24), tot: nu.Add(sessieDuur), laatst: nu}
	return cookie
}

// Zoek: de sessie bij de cookie (een kopie), of nil.
func (ss *Sessies) Zoek(cookie string) *Sessie {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	k := sessieSleutel(cookie)
	s := ss.alle[k]
	nu := time.Now()
	if s == nil || nu.After(s.tot) || nu.Sub(s.laatst) > sessieRust {
		delete(ss.alle, k)
		return nil
	}
	s.laatst = nu
	kopie := *s
	return &kopie
}

func (ss *Sessies) Weg(cookie string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	delete(ss.alle, sessieSleutel(cookie))
}
