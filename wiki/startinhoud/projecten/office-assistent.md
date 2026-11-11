---
titel: Office Assistent
omschrijving: Een stuk goed vastleggen op het moment dat het ontstaat; het werkende prototype en hoe je het uitprobeert
labels: projecten, office-assistent, mira, prototype
---

# Office Assistent

De Office Assistent helpt een medewerker een stuk goed vast te leggen **op het moment dat het ontstaat**: in de tekstverwerker, het rekenblad of de presentatie, niet achteraf in een archiefsysteem. Het concept heet **MIRA** (*Modular Information & Records Assistant*) en komt uit *XENA Deel II*; in de werkomgeving heet het gewoon *Assistent*.

- **Het concept laten zien:** de demo [MIRA](/demos/mira).
- **Het werkende prototype:** de repo's op [GitHub](https://github.com/digitaalgemeengoed), met uitleg op [digitaalgemeengoed.github.io](https://digitaalgemeengoed.github.io/).
- **Volgen en melden:** [Redmine](https://projecten.digitaalgemeengoed.nl/projects/office-assistent), en het forum *Office Assistent*.

> **Een prototype, geen product.** Uitproberen mag, maar alleen met testgegevens: geen echte persoonsgegevens of vertrouwelijke stukken.

## Wat de assistent doet

| Module | Wat | Stand in het prototype |
|---|---|---|
| Zakelijk of persoonlijk | Bij een nieuw stuk de keuze: zakelijk (met metadata, in het archief) of persoonlijk (eigen opslag, geen metadata, alleen voor jezelf). Een persoonlijk stuk kan later zakelijk worden. | werkt |
| Metadateren | De gegevens die bij een stuk horen, zoals onderwerp, zaak, status en Woo-informatiecategorie, in een paneel naast het stuk, met voorstellen uit de inhoud. Ontbrekende verplichte velden worden gemeld. | werkt |
| Anonimiseren bij de bron | Persoonsgegevens aanwijzen, nakijken **in het document zelf**, en dan **versleutelen** in plaats van zwart lakken. | werkt |
| Classificeren en bewaren | Een klasse uit de Selectielijst gemeenten (VNG, 2020) voorstellen en de bewaartermijn uitrekenen. Een vastgesteld stuk gaat op slot; een wijziging wordt een voorstel dat een ander (de *vrijgever*) goedkeurt. | werkt |
| Toegankelijkheid | Het bestand controleren op 34 punten uit WCAG 2.1 AA (docx, xlsx, pptx) en, waar het kan, herstellen. | werkt |
| Waarmerken | Een gekwalificeerd zegel of tijdstempel, zie de demo [VERA](/demos/vera). | volgt |
| Workflow | Stappen en taken rond een stuk. | volgt |

Wat de assistent voorstelt, beslist de medewerker. De assistent vraagt alleen wat alleen de medewerker weet.

### Anonimiseren, iets preciezer

- De herkenning doet **OpenAnonymiser** (van Conduction), op de eigen server. Standaard worden afgeschermd: namen, adressen, BSN, IBAN, telefoonnummers, e-mailadressen, kentekens en zaaknummers.
- Wat de assistent vindt, is een **voorbeeld** tot de medewerker bevestigt. Ten onrechte gevonden kan terug, gemist kan erbij.
- Na bevestigen komt op die plek een **plaatshouder** in het document. Het origineel staat versleuteld in een register, met een sleutel per document en per AVG-categorie: gewone persoonsgegevens (art. 6), bijzondere (art. 9) en strafrechtelijke (art. 10).
- Wie welke gegevens weer mag zien, hangt af van het **PPL-niveau** van de medewerker (rollen `ppl-1`, `ppl-2`, `ppl-3` in Keycloak). Inzien wordt vastgelegd.
- Anonimiseren maakt een nieuwe versie; de oorspronkelijke versie blijft, en terugzetten kan met het juiste niveau en een reden.
- Het beheerportaal houdt bij hoe goed de herkenning gaat (ten onrechte gevonden, gemist), om de herkenning bij te stellen.

### Beheerportaal

Een gemeente (of haar leverancier, als de gemeente dat delegeert) beheert de assistent in een eigen portaal:

- het **metadatamodel**: een basismodel kiezen (MDTO, RGBZ of TMLO), velden aan- of uitzetten en eigen velden toevoegen;
- de **classificatie**: welke resultaten uit de Selectielijst de gemeente gebruikt;
- accounts, groepen, audit en instellingen.

## Waar het werkt

| Werkomgeving | Hoe |
|---|---|
| Microsoft Word, Excel en PowerPoint (ook in de browser) | een invoegtoepassing met een eigen tabblad *Assistent* |
| LibreOffice Writer, Calc en Impress op de eigen computer | het paneel in de zijbalk; als alternatief een eigen zijpaneel in LibreOffice zelf |
| Collabora Online | het paneel naast het document in de browser |

Het paneel is in elke werkomgeving **hetzelfde**: het komt van de server. Per werkomgeving is er alleen een dunne *adapter* die de inhoud aanlevert en terugneemt. Kan een werkomgeving iets niet (Collabora kan bijvoorbeeld geen selectie doorgeven), dan valt de assistent zichtbaar terug.

## Hoe het in elkaar zit

| Laag | Repo | Wat |
|---|---|---|
| Werkomgeving en adapter | `oa-adapters` | de dunne schakel per kantoorpakket; geen sleutels, geen gegevens |
| Paneel | `oa-paneel` | React met NL Design System, hetzelfde in elke werkomgeving |
| Assistentkern | `oa-kern` | de modules; onthoudt niets tussen twee aanroepen |
| Contract | `oa-contract` | de afspraak tussen alle lagen |
| Integratielaag | `oa-integratielaag` | stelt vast wie er belt; een **lezende** en een **vastleggende** ingang. Tokens komen nooit in de werkomgeving. |
| Assistent-services | `oa-services`, `oa-wcag`, OpenAnonymiser | lezen en rekenen, nooit vastleggen |
| Platform | `opendop-mini` | objecten, metadata, versies en gebeurtenissen op PostgreSQL en S3-opslag met Object Lock; sleutels in OpenBao. Later te vervangen door OpenDOP of de Nextcloud-stack van XENA. |
| Beheer | `oa-beheer` | het beheerportaal |
| Uitrol | `oa-uitrol` | alles op één machine, of op drie servers |

Belangrijke keuzes:

- **De assistent heeft geen eigen opslag.** Alles wat wordt vastgelegd, staat in het platform; er ontstaat geen tweede archief.
- **Elke vastlegging is een gebeurtenis** in een tabel die alleen toevoegen toestaat. Zo is na te gaan wie wat wanneer deed.
- **Scheiding per gemeente** in de database: een gemeente ziet alleen haar eigen objecten.
- **Drie servers naar vertrouwensgrens**: toegang (wat van internet bereikbaar is), platform (de opslag) en diensten (herkenning, toegankelijkheid, Keycloak, sleutels). Ze praten met elkaar over een versleuteld netwerk (WireGuard).

## Uitproberen en meetesten

- **Op een omgeving die al draait:** volg `TESTEN.md` in de repo [oa-uitrol](https://github.com/digitaalgemeengoed/oa-uitrol) en de downloadpagina van die omgeving.
- **Zelf inrichten:** `server/INRICHTING.md` in oa-uitrol, stap voor stap, ook voor wie geen systeembeheerder is.
- **Wat we zoeken:** testers met Word (Windows en Mac), LibreOffice (Windows, Mac en Linux) en Collabora. Meld bevindingen in Redmine, met de werkomgeving en de versie erbij.

## Ervaringen en handleidingen

*Nog leeg. Heb je de assistent geprobeerd bij je gemeente? Maak hieronder een pagina, bijvoorbeeld `projecten/office-assistent/ervaringen-<gemeente>`.*
