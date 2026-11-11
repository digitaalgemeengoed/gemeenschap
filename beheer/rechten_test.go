package main

import "testing"

func TestRechten(t *testing.T) {
	beheer := rolUit([]string{"/netwerk/gebruikers", "/site/beheer"})
	kern := rolUit([]string{"/netwerk/kerngroep"})
	beide := rolUit([]string{"/site/beheer", "/netwerk/kerngroep"})
	niks := rolUit([]string{"/netwerk/members"})
	if niks.Toegang() || !beheer.Toegang() || !kern.Toegang() {
		t.Fatal("toegang")
	}
	gevallen := []struct {
		rol  Rol
		pad  string
		goed bool
	}{
		{kern, "/netwerk/members", true},
		{kern, "/netwerk/contributors", true},
		{kern, "/netwerk/kerngroep", false},
		{kern, "/site/beheer", false},
		{kern, "/site/redactie", false},
		{beheer, "/netwerk/kerngroep", true},
		{beheer, "/site/beheer", true},
		{beheer, "/netwerk/gebruikers", false},
		{beheer, "/eigen-groep", false},
		{beide, "/site/redactie", true},
		{niks, "/netwerk/members", false},
	}
	for _, g := range gevallen {
		if g.rol.MagGroep(g.pad) != g.goed {
			t.Errorf("%s %s: verwacht %v", g.rol.Naam(), g.pad, g.goed)
		}
	}
	if kern.MagBlokkeren() || kern.MagOrganisaties() || !beide.MagOrganisaties() {
		t.Error("blokkeren of organisaties")
	}
}

func TestDomeinen(t *testing.T) {
	d, err := domeinenUit("Haarlem.nl, @zandvoort.nl\nhaarlem.nl")
	if err != nil || len(d) != 2 || d[0].Naam != "haarlem.nl" || d[1].Naam != "zandvoort.nl" {
		t.Fatalf("%v %v", d, err)
	}
	if _, err := domeinenUit("geen domein"); err == nil {
		t.Fatal("geen fout")
	}
	if a := aliasVan("Gemeente 's-Hertogenbosch"); a != "s-hertogenbosch" {
		t.Fatal(a)
	}
}
