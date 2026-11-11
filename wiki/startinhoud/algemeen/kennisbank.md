---
titel: Werken in de kennisbank
omschrijving: Schrijven, ordenen, en architectuurmodellen toevoegen
labels: algemeen, kennisbank, architectuur
---

# Werken in de kennisbank

## Een pagina wijzigen of maken

- **Wijzigen:** klik op het potlood rechtsonder. Je schrijft in Markdown, met een voorbeeld ernaast. Klik op **Opslaan**.
- **Nieuw:** klik bovenaan op **Nieuwe pagina** en kies de plek (het pad), bijvoorbeeld `projecten/office-assistent/installeren`. Kies de editor *Markdown*.
- **Terug:** onder *Geschiedenis* zie je elke versie, met wie hem maakte. Je kunt een oude versie terugzetten.
- **Praten over een pagina:** met *Opmerkingen* onderaan, of in het forum.

## Waar iets hoort

| Pad | Wat |
|---|---|
| `algemeen/…` | hoe het netwerk werkt: processen, afspraken, de FAQ |
| `projecten/<project>/…` | alles over één project: ontwerp, handleidingen, besluiten, ervaringen |
| `demos/…` | uitleg bij de demo's |

Een pagina die bij een project hoort, zet je onder dat project. Twijfel je, zet hem dan onder `algemeen` en vraag de redactie in het forum.

## Goed schrijven

- Schrijf voor een collega bij een andere gemeente die het onderwerp niet kent.
- Begin met het antwoord, daarna de uitleg.
- Leg een afkorting uit de eerste keer dat je hem gebruikt.
- Zet een datum bij iets dat kan verouderen, zoals een versie of een stand van zaken.
- Verwijs naar de bron: een besluit, een norm, een repo, een melding in Redmine.

## Wat er niet in mag

- **Persoonsgegevens**, wachtwoorden of sleutels.
- **Vertrouwelijke stukken** van een gemeente.
- **De technische inrichting van een gemeente**: netwerk, beveiligingsmaatregelen, leveranciersafspraken (BIO). Referentie- en oplossingsarchitectuur mag wel.

## De pagina's in GitHub

Elke pagina staat ook als Markdown in de besloten repo `digitaalgemeengoed/kennisbank`, met wie hem schreef. Dat werkt in beide richtingen: een wijziging in de repo staat binnen vijf minuten in de wiki. Handig voor ontwikkelaars, en een extra kopie.

## Architectuurmodellen

De ArchiMate-modellen staan op [architectuur.digitaalgemeengoed.nl](https://architectuur.digitaalgemeengoed.nl), als doorklikbaar rapport van Archi, ook alleen voor leden.

- **Eén repo per project of domein**, bijvoorbeeld `architectuur-xena`. Je werkt in **Archi** met de plugin **coArchi**, die het model in Git zet.
- **Een model toevoegen:** een regel in `architectuur/repos.txt` in de repo `gemeenschap` (via een pull request). Binnen een kwartier na samenvoegen staat het rapport online.
- **Bijwerken:** na elke commit in de modelrepo wordt het rapport binnen een kwartier opnieuw gemaakt.
- **Uitleg bij een model** hoort hier, onder het project, met een link naar de view in het rapport.
- Werk met coArchi samen in één model; voeg nooit met de hand XML-bestanden samen.

Dezelfde regel als hierboven: geen netwerkinrichting of beveiliging van een gemeente in een model.
