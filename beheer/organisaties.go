package main

// Organisaties: de deelnemende gemeenten en organisaties (Keycloak
// Organizations). Een domein koppelt wie zich met een e-mailadres op dat
// domein aanmeldt aan de organisatie; de site gebruikt dat bijvoorbeeld bij
// de crowdfunding. Bekijken: beheer en kerngroep. Wijzigen: beheer.

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

func (a *App) organisaties(w http.ResponseWriter, r *http.Request, s *Sessie) {
	lijst, err := a.KC.Organisaties(r.Context())
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	for i := range lijst {
		if n, err := a.KC.AantalOrganisatieLeden(r.Context(), lijst[i].ID); err == nil {
			lijst[i].Leden = n
		} else {
			log.Printf("leden van %s: %v", lijst[i].Alias, err)
		}
	}
	a.toon(w, r, "organisaties", http.StatusOK, pagina{Titel: "Organisaties", Hier: "organisaties", Sessie: s, Data: lijst})
}

var (
	domeinPatroon = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$`)
	aliasPatroon  = regexp.MustCompile(`[^a-z0-9]+`)
)

// domeinenUit: domeinen uit een tekstveld (komma's, spaties of regels).
func domeinenUit(t string) ([]Domein, error) {
	var uit []Domein
	gezien := map[string]bool{}
	for _, d := range strings.FieldsFunc(strings.ToLower(t), func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\r' || r == '\t' }) {
		d = strings.TrimPrefix(strings.TrimSpace(d), "@")
		if d == "" || gezien[d] {
			continue
		}
		if !domeinPatroon.MatchString(d) {
			return nil, fmt.Errorf("%q is geen domein (zoals haarlem.nl)", d)
		}
		gezien[d] = true
		uit = append(uit, Domein{Naam: d})
	}
	return uit, nil
}

func aliasVan(naam string) string {
	a := strings.Trim(aliasPatroon.ReplaceAllString(strings.ToLower(naam), "-"), "-")
	for _, voor := range []string{"gemeente-", "provincie-"} {
		a = strings.TrimPrefix(a, voor)
	}
	if len(a) > 40 {
		a = strings.Trim(a[:40], "-")
	}
	return a
}

func (a *App) nieuweOrganisatie(w http.ResponseWriter, r *http.Request, s *Sessie) {
	if !s.Rol.MagOrganisaties() {
		a.fout(w, r, s, http.StatusForbidden, "Alleen de beheerders van het platform kunnen organisaties toevoegen.")
		return
	}
	naam := strings.Join(strings.Fields(r.FormValue("naam")), " ")
	domeinen, err := domeinenUit(r.FormValue("domeinen"))
	switch {
	case err != nil:
		terug(w, r, "/organisaties", "fout", err.Error())
		return
	case naam == "" || len(naam) > 120:
		terug(w, r, "/organisaties", "fout", "Geef de organisatie een naam.")
		return
	case len(domeinen) == 0:
		terug(w, r, "/organisaties", "fout", "Geef minstens één domein (zoals haarlem.nl).")
		return
	}
	reden, ok := reden(r)
	if !ok {
		terug(w, r, "/organisaties", "fout", zonderReden)
		return
	}
	o := Organisatie{Naam: naam, Alias: aliasVan(naam), Actief: true, Domeinen: domeinen, Beschrijving: strings.TrimSpace(r.FormValue("beschrijving"))}
	if o.Alias == "" {
		terug(w, r, "/organisaties", "fout", "Kies een naam met letters of cijfers.")
		return
	}
	if err := a.KC.NieuweOrganisatie(r.Context(), o); err != nil {
		a.orgFout(w, r, s, "/organisaties", err)
		return
	}
	a.noteer(s, "organisatie toegevoegd", naam, "domeinen: "+strings.Join(o.DomeinNamen(), ", "), reden)
	terug(w, r, "/organisaties", "melding", naam+" is toegevoegd.")
}

func (a *App) organisatie(w http.ResponseWriter, r *http.Request, s *Sessie) {
	o, err := a.KC.Organisatie(r.Context(), r.PathValue("id"))
	if errors.Is(err, errNietGevonden) {
		a.fout(w, r, s, http.StatusNotFound, "Deze organisatie bestaat niet (meer).")
		return
	}
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	leden, err := a.KC.OrganisatieLeden(r.Context(), o.ID)
	if err != nil {
		a.interneFout(w, r, s, err)
		return
	}
	logboek, _ := a.Logboek.Laatste(20, o.Naam)
	a.toon(w, r, "organisatie", http.StatusOK, pagina{Titel: o.Naam, Hier: "organisaties", Sessie: s,
		Data: map[string]any{"Org": o, "Leden": leden, "Logboek": logboek}})
}

func (a *App) wijzigOrganisatie(w http.ResponseWriter, r *http.Request, s *Sessie) {
	id := r.PathValue("id")
	doel := "/organisaties/" + url.PathEscape(id)
	if !s.Rol.MagOrganisaties() {
		a.fout(w, r, s, http.StatusForbidden, "Alleen de beheerders van het platform kunnen organisaties wijzigen.")
		return
	}
	o, err := a.KC.Organisatie(r.Context(), id)
	if err != nil {
		a.orgFout(w, r, s, doel, err)
		return
	}
	naam := strings.Join(strings.Fields(r.FormValue("naam")), " ")
	domeinen, err := domeinenUit(r.FormValue("domeinen"))
	switch {
	case err != nil:
		terug(w, r, doel, "fout", err.Error())
		return
	case naam == "" || len(naam) > 120:
		terug(w, r, doel, "fout", "Geef de organisatie een naam.")
		return
	case len(domeinen) == 0:
		terug(w, r, doel, "fout", "Geef minstens één domein (zoals haarlem.nl).")
		return
	}
	reden, ok := reden(r)
	if !ok {
		terug(w, r, doel, "fout", zonderReden)
		return
	}
	var veranderd []string
	if naam != o.Naam {
		veranderd = append(veranderd, fmt.Sprintf("naam: %s → %s", o.Naam, naam))
	}
	oud, nieuw := o.DomeinNamen(), (&Organisatie{Domeinen: domeinen}).DomeinNamen()
	slices.Sort(oud)
	slices.Sort(nieuw)
	if !slices.Equal(oud, nieuw) {
		veranderd = append(veranderd, fmt.Sprintf("domeinen: %s → %s", strings.Join(oud, ", "), strings.Join(nieuw, ", ")))
	}
	actief := r.FormValue("actief") == "ja"
	if actief != o.Actief {
		veranderd = append(veranderd, map[bool]string{true: "weer actief", false: "uitgezet"}[actief])
	}
	beschrijving := strings.TrimSpace(r.FormValue("beschrijving"))
	if beschrijving != o.Beschrijving {
		veranderd = append(veranderd, "beschrijving")
	}
	if len(veranderd) == 0 {
		terug(w, r, doel, "melding", "Er is niets gewijzigd.")
		return
	}
	// Bestaande domeinen houden hun gegevens (zoals 'geverifieerd').
	for i, d := range domeinen {
		if j := slices.IndexFunc(o.Domeinen, func(x Domein) bool { return x.Naam == d.Naam }); j >= 0 {
			domeinen[i] = o.Domeinen[j]
		}
	}
	o.Naam, o.Domeinen, o.Actief, o.Beschrijving = naam, domeinen, actief, beschrijving
	if err := a.KC.WijzigOrganisatie(r.Context(), *o); err != nil {
		a.orgFout(w, r, s, doel, err)
		return
	}
	a.noteer(s, "organisatie gewijzigd", o.Naam, strings.Join(veranderd, "; "), reden)
	terug(w, r, doel, "melding", "Opgeslagen.")
}

func (a *App) organisatieLeden(w http.ResponseWriter, r *http.Request, s *Sessie) {
	id := r.PathValue("id")
	doel := "/organisaties/" + url.PathEscape(id)
	if !s.Rol.MagOrganisaties() {
		a.fout(w, r, s, http.StatusForbidden, "Alleen de beheerders van het platform kunnen leden aan een organisatie koppelen.")
		return
	}
	o, err := a.KC.Organisatie(r.Context(), id)
	if err != nil {
		a.orgFout(w, r, s, doel, err)
		return
	}
	reden, ok := reden(r)
	if !ok {
		terug(w, r, doel, "fout", zonderReden)
		return
	}
	var l *Lid
	erin := r.FormValue("weg") == ""
	if erin {
		email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
		if !strings.Contains(email, "@") {
			terug(w, r, doel, "fout", "Geef het e-mailadres van het lid.")
			return
		}
		if l, err = a.KC.LidMetEmail(r.Context(), email); errors.Is(err, errNietGevonden) {
			terug(w, r, doel, "fout", "Er is geen account met "+email+".")
			return
		}
	} else {
		l, err = a.KC.Lid(r.Context(), r.FormValue("weg"))
	}
	if err != nil {
		a.orgFout(w, r, s, doel, err)
		return
	}
	if err := a.KC.OrganisatieLid(r.Context(), o.ID, l.ID, erin); err != nil {
		a.orgFout(w, r, s, doel, err)
		return
	}
	actie, melding := "lid gekoppeld", lidLabel(l)+" hoort nu bij "+o.Naam+"."
	if !erin {
		actie, melding = "lid ontkoppeld", lidLabel(l)+" hoort niet meer bij "+o.Naam+"."
	}
	a.noteer(s, actie, o.Naam, lidLabel(l), reden)
	terug(w, r, doel, "melding", melding)
}

func (a *App) orgFout(w http.ResponseWriter, r *http.Request, s *Sessie, doel string, err error) {
	var kf *KeycloakFout
	switch {
	case errors.Is(err, errNietGevonden):
		a.fout(w, r, s, http.StatusNotFound, "Deze organisatie bestaat niet (meer).")
	case errors.As(err, &kf) && kf.Status == http.StatusConflict:
		terug(w, r, doel, "fout", "Dat kan niet: "+kf.Bericht+". (Een domein of naam hoort al bij een andere organisatie, of het lid hoort al bij deze.)")
	case errors.As(err, &kf) && kf.Status < 500:
		terug(w, r, doel, "fout", "Keycloak weigerde dit: "+kf.Bericht)
	default:
		a.interneFout(w, r, s, err)
	}
}
