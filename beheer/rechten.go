package main

// Wie mag wat. Twee groepen geven toegang, los van elkaar; wie in beide zit,
// mag wat beide mogen.
//
//   /site/beheer        de beheerders van het platform: alles hieronder.
//   /netwerk/kerngroep  de kerngroep van de coöperatie: leden bekijken en de
//                       rollen in het netwerk geven of afnemen (contributor,
//                       member). Niet: kerngroep, beheer of redactie; geen
//                       accounts blokkeren; organisaties alleen bekijken.
//
// Waarborgen voor iedereen: de laatste beheerder blijft beheerder, en je
// blokkeert jezelf niet. Elke wijziging vraagt een reden en komt in het
// logboek.

import (
	"context"
	"strings"
	"sync"
	"time"
)

const (
	groepBeheer     = "/site/beheer"
	groepKerngroep  = "/netwerk/kerngroep"
	groepGebruikers = "/netwerk/gebruikers"
)

// Wat de kerngroep mag geven en afnemen.
var kerngroepGroepen = map[string]bool{
	"/netwerk/contributors": true,
	"/netwerk/members":      true,
}

type Rol struct {
	Beheer    bool
	Kerngroep bool
}

func rolUit(groepen []string) Rol {
	var r Rol
	for _, g := range groepen {
		r.Beheer = r.Beheer || g == groepBeheer
		r.Kerngroep = r.Kerngroep || g == groepKerngroep
	}
	return r
}

func (r Rol) Toegang() bool { return r.Beheer || r.Kerngroep }

func (r Rol) Naam() string {
	switch {
	case r.Beheer && r.Kerngroep:
		return "beheerder en kerngroep"
	case r.Beheer:
		return "beheerder"
	case r.Kerngroep:
		return "kerngroep"
	}
	return ""
}

// MagGroep: mag deze rol iemand in of uit deze groep zetten?
func (r Rol) MagGroep(pad string) bool {
	if pad == groepGebruikers {
		return false // standaardgroep: iedereen zit erin
	}
	if r.Beheer {
		return strings.HasPrefix(pad, "/netwerk/") || strings.HasPrefix(pad, "/site/")
	}
	return r.Kerngroep && kerngroepGroepen[pad]
}

func (r Rol) MagBlokkeren() bool    { return r.Beheer }
func (r Rol) MagUitloggen() bool    { return r.Beheer }
func (r Rol) MagOrganisaties() bool { return r.Beheer }

// rollenCache: de groepen van wie er ingelogd is, kort bewaard. Zo telt een
// wijziging in Keycloak (iemand uit de beheergroep) binnen een minuut, ook
// midden in een sessie.
type rollenCache struct {
	mu   sync.Mutex
	alle map[string]rolMoment
}

type rolMoment struct {
	rol Rol
	tot time.Time
}

const rolDuur = time.Minute

func (a *App) rolVan(ctx context.Context, sub string) (Rol, error) {
	a.rollen.mu.Lock()
	m, ok := a.rollen.alle[sub]
	a.rollen.mu.Unlock()
	if ok && time.Now().Before(m.tot) {
		return m.rol, nil
	}
	groepen, err := a.KC.GroepenVan(ctx, sub)
	if err != nil {
		return Rol{}, err
	}
	r := rolUit(groepen)
	a.rollen.mu.Lock()
	if a.rollen.alle == nil {
		a.rollen.alle = map[string]rolMoment{}
	}
	a.rollen.alle[sub] = rolMoment{rol: r, tot: time.Now().Add(rolDuur)}
	a.rollen.mu.Unlock()
	return r, nil
}

// vergeetRol: na een wijziging van iemands groepen meteen opnieuw kijken.
func (a *App) vergeetRol(sub string) {
	a.rollen.mu.Lock()
	delete(a.rollen.alle, sub)
	a.rollen.mu.Unlock()
}
