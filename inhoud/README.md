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
| `held` | de opening van een pagina: grote kop, inleiding en knoppen, met de uil op de achtergrond |
| `kaarten` | elke `###` kop met de tekst eronder wordt een kaart; de kaarten staan naast elkaar |
| `stappen` | als kaarten, maar genummerd: een proces in stappen |
| `kader` | tekst in een lichtblauw vlak, bijvoorbeeld een oproep |

Een vak in een vak kan niet.

Een pagina weghalen: verwijder het bestand. Op de site wordt het dan een
concept; het verdwijnt niet.

## Als het misgaat

Staat er na tien minuten nog niets? Dan klopt er iets niet in een bestand,
zoals een onbekend veld in het kopje of een link naar een pagina die niet
bestaat. De site blijft dan zoals hij was. Wie de server beheert, ziet de
melding met `sudo /opt/gemeenschap/server/uitrol.sh --status` (en krijgt een
mail als dat is ingesteld).
