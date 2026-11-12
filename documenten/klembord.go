package main

// Knippen, kopiëren en plakken in de verkenner (het contextmenu). Het
// klembord zelf staat in de browser (sessionStorage); de server krijgt bij
// het plakken wat er geknipt of gekopieerd is, en controleert alles opnieuw.
//
//   - Knippen en plakken = verplaatsen, binnen dezelfde ruimte (zoals op de
//     netwerkschijf). Vraagt Bewerken op het item en op de map waar het heen gaat.
//   - Kopiëren en plakken: een document (de huidige versie), ook naar een
//     andere ruimte. Vraagt Lezen op het document en Bewerken op de map waar
//     het heen gaat. Staat er al iets met die naam, dan heet de kopie
//     "naam (kopie).ext". Een map kopiëren kan (nog) niet hier, wel via de
//     netwerkschijf.

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
)

func (a *App) plakken(w http.ResponseWriter, r *http.Request, s *Sessie, p *Plek) {
	ctx := r.Context()
	doelMap, _ := strconv.ParseInt(r.FormValue("doel"), 10, 64)
	bronID, _ := strconv.ParseInt(r.FormValue("bron"), 10, 64)
	actie := r.FormValue("actie")
	terugNaar := veiligTerug(r.FormValue("terug"))
	if terugNaar == "/" {
		terugNaar = mapURL(p.Ruimte, doelMap)
	}
	if !p.Boom.Bestaat(doelMap) || (doelMap != 0 && p.Boom.IsDoc(doelMap)) {
		terug(w, r, terugNaar, "fout", "Die map bestaat niet (meer).")
		return
	}
	if p.Boom.Niveau(doelMap) < Bewerken {
		terug(w, r, terugNaar, "fout", "Je kunt hier niets neerzetten: je hebt in deze map het recht "+niveauNaam(p.Boom.Niveau(doelMap))+".")
		return
	}

	// De bron: in deze ruimte of (alleen bij kopiëren) een andere.
	bronRuimte, bronBoom := p.Ruimte, p.Boom
	if sl := r.FormValue("bron_ruimte"); sl != "" && sl != p.Ruimte.Sleutel {
		rm, err := a.Reg.Ruimte(ctx, sl)
		if err != nil {
			terug(w, r, terugNaar, "fout", "Wat je knipte of kopieerde, bestaat niet meer.")
			return
		}
		b, err := a.Reg.laadBoom(ctx, rm, s)
		if err != nil {
			a.interneFout(w, r, s, err)
			return
		}
		bronRuimte, bronBoom = rm, b
	}
	m := bronBoom.mappen[bronID]
	if m == nil || bronBoom.Niveau(bronID) < Lezen && !bronBoom.Zichtbaar(bronID) {
		terug(w, r, terugNaar, "fout", "Wat je knipte of kopieerde, bestaat niet meer of is niet meer voor jou.")
		return
	}

	switch actie {
	case "knippen":
		if bronRuimte.ID != p.Ruimte.ID {
			terug(w, r, terugNaar, "fout", "Verplaatsen kan alleen binnen een ruimte. Kopieer het document en verwijder het daarna op de oude plek.")
			return
		}
		if bronBoom.Niveau(bronID) < Bewerken {
			terug(w, r, terugNaar, "fout", "Je kunt "+m.Naam+" niet verplaatsen: daarvoor is Bewerken nodig.")
			return
		}
		if m.Ouder == doelMap {
			terug(w, r, terugNaar, "melding", m.Naam+" staat hier al.")
			return
		}
		err := a.Reg.Verplaats(ctx, p.Ruimte.ID, bronID, doelMap, m.Naam, s.Email)
		switch {
		case errors.Is(err, errBestaatAl):
			terug(w, r, terugNaar, "fout", "Hier staat al iets met de naam "+m.Naam+". Wijzig eerst een van de namen.")
		case errors.Is(err, errOngeldig):
			terug(w, r, terugNaar, "fout", "Een map kan niet in zichzelf of in een map eronder.")
		case err != nil:
			a.interneFout(w, r, s, err)
		default:
			terug(w, r, terugNaar, "melding", m.Naam+" is verplaatst.")
		}
	case "kopieren":
		if !m.Doc {
			terug(w, r, terugNaar, "fout", "Een map kopiëren kan hier (nog) niet; via de netwerkschijf wel. Een document kopiëren kan wel.")
			return
		}
		if bronBoom.Niveau(bronID) < Lezen {
			terug(w, r, terugNaar, "fout", "Je kunt dit document niet lezen, en dus niet kopiëren.")
			return
		}
		naam := vrijeNaam(p.Boom, doelMap, m.Naam)
		if err := a.kopieer(r, s, bronRuimte, bronID, p.Ruimte, doelMap, naam); err != nil {
			switch {
			case errors.Is(err, errVol):
				terug(w, r, terugNaar, "fout", "Kopiëren lukt niet: "+err.Error()+".")
			case errors.Is(err, errTeGroot):
				terug(w, r, terugNaar, "fout", "Kopiëren lukt niet: het document is te groot.")
			default:
				a.interneFout(w, r, s, err)
			}
			return
		}
		terug(w, r, terugNaar, "melding", "Gekopieerd als "+naam+".")
	default:
		terug(w, r, terugNaar, "fout", "Onbekende opdracht.")
	}
}

// kopieer: de huidige versie van een document als nieuw document elders.
func (a *App) kopieer(r *http.Request, s *Sessie, van *Ruimte, id int64, naar *Ruimte, ouder int64, naam string) error {
	ctx := r.Context()
	x, err := a.Reg.Item(ctx, van.ID, id)
	if err != nil {
		return err
	}
	v, err := a.Reg.Versie(ctx, x.ID, x.HuidigeVersie)
	if err != nil {
		return err
	}
	rd, _, err := a.Opslag.Haal(ctx, v.Object)
	if err != nil {
		return err
	}
	defer rd.Close()
	return a.bewaar(ctx, naar, ouder, naam, rd, s)
}

// vrijeNaam: "notulen.docx", of "notulen (kopie).docx", "notulen (kopie 2).docx" …
func vrijeNaam(b *Boom, ouder int64, naam string) string {
	if b.Zoek(ouder, naam) == 0 {
		return naam
	}
	ext := path.Ext(naam)
	stam := strings.TrimSuffix(naam, ext)
	for i := 1; i < 1000; i++ {
		kandidaat := stam + " (kopie)" + ext
		if i > 1 {
			kandidaat = fmt.Sprintf("%s (kopie %d)%s", stam, i, ext)
		}
		if b.Zoek(ouder, kandidaat) == 0 {
			return kandidaat
		}
	}
	return stam + " (kopie " + nieuwID()[:8] + ")" + ext
}
