# Pagina's van digitaalgemeengoed.nl

Elke pagina van de site is hier een bestand in `paginas/`. Wijzig je een
bestand op `main`, dan staat de wijziging binnen vijf minuten op de site. In
WordPress staan deze pagina's op slot; daar zie je bij elke pagina de link
**Bewerken op GitHub**.

Nieuws, berichten en de fora horen hier niet: die maak je gewoon in WordPress.

## Een pagina wijzigen

1. Open het bestand op GitHub en klik op het potlood.
2. Wijzig de tekst. Met **Preview** zie je ongeveer hoe het wordt.
3. Kies onderaan **Create a new branch** en dan **Propose changes**. Zo komt
   er een pull request, en kijkt iemand anders mee voordat het live gaat.
4. Na **Merge** staat het binnen vijf minuten op de site.

## Een nieuwe pagina

Maak een bestand `paginas/<naam>.md`. Bovenaan staat een kopje:

```
---
titel: Over het netwerk
adres: over            (optioneel; anders de bestandsnaam)
ouder: meedoen         (optioneel: het adres van de pagina erboven)
volgorde: 2            (optioneel: de plek in het menu)
status: publiceren     (of: concept)
voorpagina: ja         (alleen voor de startpagina)
---
```

Daaronder de tekst, in Markdown:

| Schrijf je | Dan krijg je |
|---|---|
| `## Kopje` en `### Kleiner kopje` | kopjes (de titel komt uit het kopje bovenaan) |
| een witregel | een nieuwe alinea |
| `- punt` of `1. punt` | een lijst |
| `**vet**` en `*cursief*` | **vet** en *cursief* |
| `[tekst](https://…)` | een link |
| `[tekst](privacy.md)` | een link naar een andere pagina hier |
| `![beschrijving](media/foto.jpg)` op een eigen regel | een afbeelding uit `media/` |
| `> tekst` | een citaat |
| een tabel met `\|` | een tabel |
| `---` | een lijn |
| `[[Doe mee]](meedoen.md)` op een eigen regel | een knop (meer regels onder elkaar: meer knoppen naast elkaar) |
| `[dg_lid_worden]` op een eigen regel | de knop *Lid worden* (alleen voor wie niet is ingelogd) |

HTML werkt niet: het verschijnt als tekst.

`[dg_dashboard]` op een eigen regel (alleen op de startpagina): het dashboard
voor leden. Wie is ingelogd, ziet het dashboard en niet wat eronder staat;
wie niet is ingelogd, ziet juist alleen wat eronder staat.

Met `dashboard: samenwerkingen` in het kopje van een pagina komen de kaarten
van die pagina (kop, label, icoon, eerste regel) ook op het dashboard, zodra
de pagina gepubliceerd is.

### Vakken

Voor een opvallender opmaak zet je tekst tussen `::: naam` en `:::`:

```
::: kaarten
### Eerste kaart
Tekst van de eerste kaart.

### Tweede kaart
Tekst van de tweede kaart.
:::
```

| Vak | Wat het doet |
|---|---|
| `held` | de opening van de startpagina over de hele breedte: grote kop, linten, inleiding, knoppen, stadsgezicht, en rechts een foto |
| `kenmerken` | een strook met per `###` kop een groot icoon en een korte regel |
| `kaarten` | elke `###` kop met de tekst eronder wordt een kaart; de kaarten staan naast elkaar |
| `stappen` | als kaarten, maar genummerd: een proces in stappen |
| `projecten` | kaarten met een tegel (icoon of beeld), een label en een pijl naar de pagina |
| `kader` | tekst in een lichtblauw vlak, bijvoorbeeld een oproep; een lijst erin krijgt groene vinkjes |
| `band` | een donkerblauwe balk met een kop, een regel en een knop rechts |
| `feiten` | regels `Sleutel: waarde` als een tabelletje, zoals Fase, Werkwijze en Financiering op een projectpagina |

Een vak in een vak kan niet.

Direct onder de `###` kop van een kaart (of bovenin een `held`) kun je regels
`sleutel: waarde` zetten:

| Regel | Wat het doet |
|---|---|
| `icoon: lagen` | een icoon: open, lagen, mensen, raket, plant, document, mail, werkplek, forum, boek, schild, stad, puzzel, hand, kompas, code |
| `label: Common Ground` | een gekleurd label (Common Ground, Integratie, Platform en Verkenning hebben een eigen kleur); in een `held` worden labels de blauwe en groene linten |
| `link: xena.md` | de hele kaart wordt een link, met een pijl |
| `beeld: media/foto.jpg` | een foto: in een `held` rechts over de hele hoogte, op een kaart als tegel |
| `soort: oproep` | in `projecten`: een brede balk onder de kaarten, voor een oproep met knop |
| `doel: 50000`, `toegezegd: 31000`, `deelnemers: 4 gemeenten` | crowdfunding met vaste bedragen: een voortgangsbalk. Alleen echte toezeggingen invullen. |
| `campagne: anonimiseren-2027`, `doel: 60000`, `einddatum: 2027-03-01` | crowdfunding waarbij gemeenten **op de site** toezeggen (zie hieronder). Gebruik dan geen `toegezegd:` of `deelnemers:`; die komen uit de toezeggingen. |

### Toezeggingen per gemeente

Met `campagne:` op een kaart in een `kaarten`-vak kan een ingelogd lid
namens zijn gemeente een **intentie** opgeven: bedrag, eventueel de voorwaarde
"alleen als het streefbedrag gehaald wordt", en wie bij de gemeente mag
tekenen. Dat kan alleen wie via het e-maildomein aan een gemeente gekoppeld
is (de organisaties in Keycloak). Per gemeente is er één toezegging per
campagne; een collega kan hem wijzigen of intrekken.

Een intentie is niet bindend. De kerngroep zet hem in WordPress onder
**Toezeggingen** op *bevestigd* zodra de samenwerkingsovereenkomst getekend
is, met het kenmerk van de overeenkomst. De getekende overeenkomst zelf hoort
in het archief, niet in WordPress.

De balk toont bevestigde bedragen vol en intenties gestreept. Op de site staat
bij elke gemeente of het een intentie of bevestigd is; het bedrag staat er
alleen bij als het bevestigd is.

De naam van de campagne (`campagne:`) bestaat uit kleine letters, cijfers en
streepjes en verander je niet meer als er toezeggingen zijn.

`[dg_fase project=xena]` (bijvoorbeeld in een `feiten`-vak) toont de fase van
het project zoals die in Redmine staat, in het veld Fase.

Een pagina weghalen: verwijder het bestand. Op de site wordt het dan een
concept; het verdwijnt niet.

## Als het misgaat

Staat er na tien minuten nog niets? Dan klopt er iets niet in een bestand,
zoals een onbekend veld in het kopje of een link naar een pagina die niet
bestaat. De site blijft dan zoals hij was. Wie de server beheert, ziet de
melding met `sudo /opt/gemeenschap/server/uitrol.sh --status` (en krijgt een
mail als dat is ingesteld).
