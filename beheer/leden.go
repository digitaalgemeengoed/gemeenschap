package main

// Leden en rollen: zoeken, bekijken, rollen (groepen) geven en afnemen,
// accounts blokkeren en overal uitloggen.

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const perPagina = 25

func (a *App) leden(w http.ResponseWriter, r *http.Request, s *Sessie) {
	q := r.URL.Query()
	zoek := strings.TrimSpace(q.Get("zoek"))
	groepPad := q.Get("groep")
	nr, _ := strconv.Atoi(q.Get("pagina"))
	nr = max(nr, 1)
	groepen, err := a.KC.Groepen(r.Context())
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	var lijst []Lid
	totaal := -1
	if groepPad != "" {
		i := slices.IndexFunc(groepen, func(g Groep) bool { return g.Pad == groepPad })
		if i < 0 {
			a.fout(w, r, s, http.StatusNotFound, "Die groep bestaat niet (meer).")
			return
		}
		lijst, err = a.KC.GroepLeden(r.Context(), groepen[i].ID, (nr-1)*perPagina, perPagina+1)
		if err == nil && zoek != "" {
			z := strings.ToLower(zoek)
			lijst = slices.DeleteFunc(lijst, func(l Lid) bool {
				return !strings.Contains(strings.ToLower(l.Naam()+" "+l.Email+" "+l.Kenmerk("organisatie")), z)
			})
		}
	} else {
		lijst, err = a.KC.ZoekLeden(r.Context(), zoek, (nr-1)*perPagina, perPagina+1)
		if err == nil {
			totaal, err = a.KC.AantalLeden(r.Context(), zoek)
		}
	}
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	verder := len(lijst) > perPagina
	if verder {
		lijst = lijst[:perPagina]
	}
	pag := func(n int) string {
		v := url.Values{"pagina": {strconv.Itoa(n)}}
		if zoek != "" {
			v.Set("zoek", zoek)
		}
		if groepPad != "" {
			v.Set("groep", groepPad)
		}
		return "/leden?" + v.Encode()
	}
	data := map[string]any{"Lijst": lijst, "Zoek": zoek, "Groep": groepPad, "Groepen": groepen, "Totaal": totaal, "Pagina": nr}
	if nr > 1 {
		data["Vorige"] = pag(nr - 1)
	}
	if verder {
		data["Volgende"] = pag(nr + 1)
	}
	a.toon(w, r, "leden", http.StatusOK, pagina{Titel: "Leden en rollen", Hier: "leden", Sessie: s, Data: data})
}

type groepKeuze struct {
	Groep
	Erin bool
	Mag  bool
}

func (a *App) lid(w http.ResponseWriter, r *http.Request, s *Sessie) {
	l, err := a.KC.Lid(r.Context(), r.PathValue("id"))
	if errors.Is(err, errNietGevonden) {
		a.fout(w, r, s, http.StatusNotFound, "Dit lid bestaat niet (meer).")
		return
	}
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	alle, err := a.KC.Groepen(r.Context())
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	van, err := a.KC.GroepenVan(r.Context(), l.ID)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	var keuzes []groepKeuze
	for _, g := range alle {
		if g.Pad == "/netwerk" || g.Pad == "/site" {
			continue // alleen de groepen eronder geven toegang
		}
		keuzes = append(keuzes, groepKeuze{Groep: g, Erin: slices.Contains(van, g.Pad), Mag: s.Rol.MagGroep(g.Pad)})
	}
	data := map[string]any{"Lid": l, "Groepen": keuzes, "Zelf": l.ID == s.Sub}
	if orgs, err := a.KC.OrganisatiesVan(r.Context(), l.ID); err == nil {
		data["Organisaties"] = orgs
	}
	if n, err := a.KC.Sessies(r.Context(), l.ID); err == nil {
		data["Sessies"] = n
	}
	if l.Email != "" {
		data["Logboek"], _ = a.Logboek.Laatste(20, l.Email)
	}
	a.toon(w, r, "lid", http.StatusOK, pagina{Titel: l.Naam(), Hier: "leden", Sessie: s, Data: data})
}

func (a *App) lidGroepen(w http.ResponseWriter, r *http.Request, s *Sessie) {
	ctx := r.Context()
	id := r.PathValue("id")
	doel := "/leden/" + url.PathEscape(id)
	l, err := a.KC.Lid(ctx, id)
	if err != nil {
		a.lidFout(w, r, s, err)
		return
	}
	reden, ok := reden(r)
	if !ok {
		terug(w, r, doel, "fout", zonderReden)
		return
	}
	alle, err := a.KC.Groepen(ctx)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	van, err := a.KC.GroepenVan(ctx, l.ID)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	r.ParseForm()
	gekozen := r.PostForm["groep"]
	type wijziging struct {
		g    Groep
		erin bool
	}
	var wijzigingen []wijziging
	for _, g := range alle {
		erin, wil := slices.Contains(van, g.Pad), slices.Contains(gekozen, g.Pad)
		if erin == wil {
			continue
		}
		if !s.Rol.MagGroep(g.Pad) {
			if wil {
				terug(w, r, doel, "fout", "Je mag "+groepNaam(g.Pad)+" niet geven.")
				return
			}
			continue // niet in het formulier: niet aan te passen
		}
		wijzigingen = append(wijzigingen, wijziging{g, wil})
	}
	if len(wijzigingen) == 0 {
		terug(w, r, doel, "melding", "Er is niets gewijzigd.")
		return
	}
	// Waarborg: de beheergroep wordt nooit leeg.
	for _, x := range wijzigingen {
		if x.g.Pad == groepBeheer && !x.erin && l.Actief {
			n, err := a.KC.ActieveLeden(ctx, x.g.ID)
			if err != nil {
				a.interneFout(w, r, s, err)
				return
			}
			if n <= 1 {
				terug(w, r, doel, "fout", "Dit is de laatste beheerder van het platform. Maak eerst iemand anders beheerder.")
				return
			}
		}
	}
	var gedaan []string
	for _, x := range wijzigingen {
		if err := a.KC.InGroep(ctx, l.ID, x.g.ID, x.erin); err != nil {
			a.vergeetRol(l.ID)
			if len(gedaan) > 0 {
				a.noteer(s, "rollen", lidLabel(l), strings.Join(gedaan, ", ")+" (daarna mislukt)", reden)
			}
			a.interneFout(w, r, s, err)
			return
		}
		if x.erin {
			gedaan = append(gedaan, "+ "+groepNaam(x.g.Pad))
		} else {
			gedaan = append(gedaan, "− "+groepNaam(x.g.Pad))
		}
	}
	a.vergeetRol(l.ID)
	a.noteer(s, "rollen", lidLabel(l), strings.Join(gedaan, ", "), reden)
	terug(w, r, doel, "melding", "Opgeslagen: "+strings.Join(gedaan, ", ")+". De wijziging geldt bij de volgende keer inloggen (in Redmine en Documenten binnen een paar minuten).")
}

func (a *App) lidActief(w http.ResponseWriter, r *http.Request, s *Sessie) {
	ctx := r.Context()
	id := r.PathValue("id")
	doel := "/leden/" + url.PathEscape(id)
	if !s.Rol.MagBlokkeren() {
		a.fout(w, r, s, http.StatusForbidden, "Alleen de beheerders van het platform kunnen accounts blokkeren.")
		return
	}
	l, err := a.KC.Lid(ctx, id)
	if err != nil {
		a.lidFout(w, r, s, err)
		return
	}
	reden, ok := reden(r)
	if !ok {
		terug(w, r, doel, "fout", zonderReden)
		return
	}
	actief := r.FormValue("actief") == "ja"
	if actief == l.Actief {
		terug(w, r, doel, "melding", "Er is niets gewijzigd.")
		return
	}
	if !actief {
		if l.ID == s.Sub {
			terug(w, r, doel, "fout", "Je kunt je eigen account niet blokkeren.")
			return
		}
		van, err := a.KC.GroepenVan(ctx, l.ID)
		if err != nil {
			a.interneFout(w, r, s, err)
			return
		}
		if slices.Contains(van, groepBeheer) {
			g, err := a.KC.GroepMetPad(ctx, groepBeheer)
			if err != nil {
				a.interneFout(w, r, s, err)
				return
			}
			if n, err := a.KC.ActieveLeden(ctx, g.ID); err != nil || n <= 1 {
				if err != nil {
					a.interneFout(w, r, s, err)
					return
				}
				terug(w, r, doel, "fout", "Dit is de laatste beheerder van het platform; dat account blijft aan.")
				return
			}
		}
	}
	if err := a.KC.ZetActief(ctx, l.ID, actief); err != nil {
		a.lidFout(w, r, s, err)
		return
	}
	actie, melding := "gedeblokkeerd", "Het account staat weer aan."
	if !actief {
		actie, melding = "geblokkeerd", "Het account is geblokkeerd en overal uitgelogd."
		if err := a.KC.LogUit(ctx, l.ID); err != nil {
			melding = "Het account is geblokkeerd; uitloggen lukte niet (" + err.Error() + ")."
		}
	}
	a.vergeetRol(l.ID)
	a.noteer(s, actie, lidLabel(l), "", reden)
	terug(w, r, doel, "melding", melding)
}

func (a *App) lidUitloggen(w http.ResponseWriter, r *http.Request, s *Sessie) {
	id := r.PathValue("id")
	doel := "/leden/" + url.PathEscape(id)
	if !s.Rol.MagUitloggen() {
		a.fout(w, r, s, http.StatusForbidden, "Alleen de beheerders van het platform kunnen iemand overal uitloggen.")
		return
	}
	l, err := a.KC.Lid(r.Context(), id)
	if err != nil {
		a.lidFout(w, r, s, err)
		return
	}
	reden, ok := reden(r)
	if !ok {
		terug(w, r, doel, "fout", zonderReden)
		return
	}
	if err := a.KC.LogUit(r.Context(), l.ID); err != nil {
		a.lidFout(w, r, s, err)
		return
	}
	a.noteer(s, "overal uitgelogd", lidLabel(l), "", reden)
	terug(w, r, doel, "melding", "Uitgelogd bij Keycloak. Een sessie die al open staat op de site of in Redmine loopt nog door tot die afloopt of ververst.")
}

func lidLabel(l *Lid) string {
	if l.Email != "" {
		return fmt.Sprintf("%s <%s>", l.Naam(), l.Email)
	}
	return l.Naam()
}

func (a *App) lidFout(w http.ResponseWriter, r *http.Request, s *Sessie, err error) {
	var kf *KeycloakFout
	switch {
	case errors.Is(err, errNietGevonden):
		a.fout(w, r, s, http.StatusNotFound, "Dit lid bestaat niet (meer).")
	case errors.As(err, &kf) && kf.Status < 500:
		a.fout(w, r, s, http.StatusBadRequest, "Keycloak weigerde dit: "+kf.Bericht)
	default:
		a.interneFout(w, r, s, err)
	}
}
