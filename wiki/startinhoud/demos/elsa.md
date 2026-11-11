---
titel: ELSA — E-mailarchivering
omschrijving: Electronic Letters & Storage Archive: elk bericht in het register en de objectopslag, gelezen via IMAP.
labels: demo, e-mail, archivering, elsa
---


# ELSA — E-mailarchivering

**Electronic Letters & Storage Archive.** Een demo van e-mailarchivering in XENA: elk bericht komt automatisch in het register en de objectopslag, en de medewerker leest het gewoon in zijn eigen mailprogramma via IMAP.

ELSA hoort bij XENA, naast de demo van de opslag, MIRA (de assistent in het kantoorpakket) en TINA (het zoekportaal voor de inwoner).

> Een demo, geen product. Personen, berichten, adressen en nummers zijn verzonnen.

**[Broncode en uitleg op GitHub](https://github.com/EHa-1999/ELSA)** (open daar `index.html` in je browser)

## Wat de demo laat zien

| Weergave | Wat je ziet |
|---|---|
| **Postbus** | Een mailprogramma dat via IMAP uit ELSA leest. Per bericht de keuze zakelijk of persoonlijk, met tien dagen bedenktijd, metagegevens aanvullen en versleutelen. Verwijderen van een zakelijk bericht haalt het uit de map, maar het blijft bewaard. |
| **Doorstroom** | Hoe een bericht via Exchange Online, een transport rule, Kafka en een kleine brug in Nextcloud, en van daar binnen een seconde in het register, de objectopslag en de zoekindex komt, zonder kopie in Exchange. |
| **Filters** | Sieve-filters die in ELSA zelf draaien, met de acties die niet zijn toegestaan omdat ze de scheiding zouden omzeilen. |
| **Adresboek** | Een gemeentelijk adresboek waarin iedereen zelf kiest of en met welke gegevens hij staat, gedeeld via CardDAV. |
| **Sleutels** | Persoonlijke berichten zijn versleuteld met een eigen sleutel. Alleen de gemeentesecretaris kan toegang geven of die bevoegdheid delegeren, en alles wordt gelogd. |
| **Architectuur** | De lagen van werkplek tot de Nextcloud-stack van XENA, de stappen per handeling en de open punten. Eén bericht in de objectopslag, met de IMAP-server en Nextcloud Files als twee ingangen. |
| **Techniek** | Drie manieren om IMAP naast WebDAV te laten draaien, een vergelijking van Apache James, Stalwart en Dovecot, wat er moet worden gebouwd en de open standaarden. |
| **Groeipad** | Vijf stappen, van archief en zoeken naast Exchange tot werken zonder Exchange. |

Met de knoppen bovenaan spring je vooruit in de tijd om te zien wat er na tien dagen gebeurt.

## Uitgangspunten

- Exchange Online verzendt en ontvangt; ELSA bewaart. Er blijft geen kopie in Exchange. Een andere mailprovider vervangt alleen de aanvoer.
- Een medewerker heeft tien dagen om zakelijk en persoonlijk te scheiden. Daarna geldt een bericht als zakelijk.
- Persoonlijk betekent: in de map Persoonlijk, versleuteld met een eigen sleutel en buiten het zoekbereik voor de Woo. De medewerker is daarvoor verantwoordelijk.
- Filters mogen niet weggooien, niet naar Persoonlijk verplaatsen en niet doorsturen buiten de aangesloten gemeenten.
- Opname in het adresboek is een persoonlijke keuze.

## Veelgestelde vragen

### Scheiden

**Wat gebeurt er als ik niets kies?**  
Na tien dagen geldt een bericht als zakelijk. Het komt dan in het archief, met de bewaartermijn uit de classificatie.

**Kan ik een keuze terugdraaien?**  
Binnen de bedenktijd in beide richtingen. Daarna kan zakelijk maken altijd; persoonlijk maken alleen op verzoek bij de informatiebeheerder, met een tweede beoordelaar.

**Waarom kan een bericht soms niet persoonlijk worden?**  
Omdat het aan een zaak of dossier hangt, of onder een lopend Woo-verzoek valt. Ontkoppel eerst; bij een Woo-verzoek kan het niet.

### Privacy

**Wie kan mijn persoonlijke berichten lezen?**  
Alleen jij. Ze staan versleuteld met jouw sleutel. Toegang voor iemand anders kan alleen de gemeentesecretaris geven, voor een omschreven periode en reden, en dat wordt vastgelegd.

### Woo

**Wanneer wordt een bericht openbaar?**  
Met een Woo-categorie uiterlijk twee weken na ontvangst of verzending, na de bedenktijd, het anonimiseren en de beoordeling. Zonder categorie alleen op verzoek.

### Regels

**Wat is het verschil tussen een regel en een filter in Outlook?**  
Een regel in ELSA draait bij het register en werkt dus overal, ook op je telefoon. Een filter in Outlook of Thunderbird werkt alleen als dat programma openstaat.

**Kan een regel mijn eigen keuze overschrijven?**  
Nee. Een regel vult aan wat leeg is. Wat een regel zet, zie je bij het veld en kun je binnen de bedenktijd herzien.

### Mailprogramma

**Welke mailprogramma's ondersteunen de assistent?**  
Thunderbird, Outlook (klassiek), Nextcloud Mail en Roundcube. De nieuwe Outlook en Outlook op het web niet, bij een IMAP-postbus; daar gebruik je de webmail.

**Wat als ik een bericht verwijder?**  
Binnen de bedenktijd geldt dat als niet zakelijk. Is het al zakelijk, dan verdwijnt het alleen uit je map; het object blijft bewaard zolang de bewaartermijn loopt.

### Opslag

**Waar staan de berichten eigenlijk?**  
Eén keer, als EML in de objectopslag van XENA, beschreven in het register. Het mailprogramma ziet ze via IMAP, Nextcloud Files via WebDAV; er is geen tweede kopie.

### Woo

**Wat doet Anonimiseren met het origineel?**  
Niets. Het origineel blijft ongewijzigd in het archief. Er komt een geanonimiseerde weergave naast, met plaatshouders voor de persoonsgegevens.

### Waarmerk

**Waarom een bericht waarmerken?**  
Om later te kunnen aantonen wat de gemeente heeft ontvangen of verstuurd, ongewijzigd en op welk moment. De bijlagen horen erbij, op een vaste versie.

### Toegankelijkheid

**Waarom controleert de assistent alleen bij het opstellen?**  
Een ontvangen bericht verander je niet. Wat je zelf schrijft, kun je nog verbeteren. Bij publicatie maakt XENA een toegankelijke weergave.

### Adresboek

**Moet ik in het gemeentelijk adresboek?**  
Nee, opname is vrijwillig. Je kiest zelf welke velden je deelt en kunt je vermelding altijd intrekken.

## Begrippen

| Begrip | Betekenis |
|---|---|
| Anonimiseren | Persoonsgegevens vervangen door plaatshouders, in een eigen weergave naast het origineel. |
| Waarmerk | Een elektronisch zegel of handtekening die vastlegt dat een bericht op een moment authentiek en ongewijzigd is. |
| Bedenktijd | De tien dagen waarin je een bericht zakelijk of persoonlijk maakt, en je keuze nog kunt herzien. |
| Zakelijk | Een bericht dat bij het werk hoort. Het komt in het archief en kan onder de Woo vallen. |
| Persoonlijk | Een privébericht, versleuteld met je eigen sleutel en buiten het zoekbereik voor de Woo. |
| Register | Waar ELSA van elk bericht de metagegevens bijhoudt: aard, zaak, dossier, classificatie en rechten. |
| Objectopslag | Waar de berichten zelf staan, één keer en onveranderlijk. |
| Regel | Een afspraak die metagegevens zet zodra een bericht binnenkomt: van de organisatie, het team of jezelf. |
| Sieve | De standaard voor filters bij aflevering in de postbus. In ELSA schrijft ELSA het script zelf. |
| Woo-categorie | Een wettelijke categorie van stukken die uit zichzelf openbaar worden gemaakt. |
| Informatiebeheerder | Beheert de regels van de organisatie en beoordeelt verzoeken om een zakelijk bericht persoonlijk te maken. |
| Office Assistent | Hetzelfde paneel in mailprogramma en kantoorpakket. ELSA is de module voor e-mailarchivering. |

---

*Overgenomen uit de demo ELSA (README en Help) van Erik Hoekstra, Gemeente Haarlem; EUPL 1.2. Een demo, geen product: alle voorbeeldgegevens zijn verzonnen, en een deel is toekomstbeeld. Wijzigt de demo, dan is [de repo](https://github.com/EHa-1999/ELSA) leidend.*
