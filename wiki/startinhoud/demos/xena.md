---
titel: XENA — Archiefwaardige opslag
omschrijving: Soevereine, archiefwaardige bestandsopslag op Nextcloud en S3, gezien door de ogen van een gewone medewerker.
labels: demo, xena, opslag, archief
---


# XENA — Archiefwaardige opslag

Een interactieve demo van hoe soevereine, archiefwaardige bestandsopslag op **Nextcloud** en **S3-objectopslag** eruitziet voor een gewone medewerker van een gemeente. Bewaartermijnen, legal hold, metagegevens en toegangsregels worden eronder afgedwongen, terwijl medewerkers blijven werken in de vensters die ze al kennen.

> **Een demo, geen product.** Alle voorbeeldgegevens zijn verzonnen: personen, dossiers en zaaknummers bestaan niet. De demo toont de stip op de horizon; de schakelaar *Tijdvak* en de weergave *Groeipad* laten zien wat wanneer beschikbaar komt.

---

**[Open de demo](https://eha-1999.github.io/XENA/) · [Broncode en uitleg op GitHub](https://github.com/EHa-1999/XENA)**

## Wat de demo laat zien

| Weergave | Wat je ziet |
|---|---|
| **Verkenner** | De bestandsverkenner op de pc met de schijven M:, P:, I: en W:. Elke schijf is een bucket in één Nextcloud-omgeving. |
| **Documentbibliotheek** | De bestandenlijst op een samenwerkingssite, bijvoorbeeld SharePoint, met kolommen voor classificatie en bewaring. |
| **Teamkanaal** | Het tabblad Bestanden in een teamkanaal, bijvoorbeeld Teams. Kies de kanaalbestanden of een gekoppelde schijf. |
| **Zoeken** | De zoekpagina in de webinterface: zoeken op naam, kenmerken en inhoud, met filters. In de Woo-modus maak je een Woo-dossier aan en voeg je documenten toe vanuit de resultaten. |
| **Office Assistent** | Een invoegtoepassing voor kantoorpakketten, hier getoond in Word: kenmerken, suggesties, toegankelijkheid en workflow naast het document. Dezelfde opzet werkt in LibreOffice en andere pakketten: één dunne tussenlaag, met per pakket een kleine invoegtoepassing. |
| **Architectuur** | Hoe vensters, Nextcloud, register en objectopslag samenhangen, met voorbeelden van WebDAV- en S3-berichten. |
| **Techniek** | Per functie wat die van Nextcloud vraagt: standaard, uitbreidingspunt, kernwijziging of iets buiten Nextcloud. |
| **Groeipad** | In zes stappen van de netwerkschijven naar de stip op de horizon, met mijlpalen en beslismomenten. |
| **Help** | Functies, waarom de combinatie ertoe doet, veelgestelde vragen, begrippen en een instructie voor beheerders. |

De balk onderaan (*Onder de motorkap*) toont bij elke handeling welk bericht naar de opslag gaat.

## Belangrijkste functies

- **Werken met bestanden:** bestanden, mappen en dossiers aanmaken; openen, hernoemen, kopiëren, knippen en plakken; filteren op kenmerken.
- **Zoeken:** op naam, kenmerk, identificatie en inhoud via de zoekindex van het register, met filters op onder meer classificatie, vertrouwelijkheid en persoonsgegevens. De rechten van de medewerker bepalen het zoekbereik; versleutelde inhoud staat niet in de index.
- **Woo-verzoeken:** een aparte zoekmodus waarin je een Woo-dossier aanmaakt, documenten als verwijzing toevoegt en de zoekslag vastlegt. De opgenomen versie wordt vastgezet in de objectopslag zolang het verzoek loopt; doorwerken blijft mogelijk.
- **Informatiebeheer:** zakelijk of persoonlijk, metagegevens volgens MDTO, dossiers die classificatie en bewaartermijn doorgeven, bewaartermijn afgedwongen met S3 Object Lock, legal hold.
- **Vergissingen herstellen:** een herroepingstermijn na registratie; voor vergrendelde stukken het onleesbaar maken van de inhoud door de sleutel te vernietigen.
- **Versies:** eerdere versies bekijken en terugzetten; terugzetten maakt een nieuwe versie.
- **Delen en toegang:** duurzame links via register en resolver; drie rechten (lezen, bewerken, beheren) per persoon of groep; zichtbaarheid (vindbaar of verborgen) ingesteld door de beheerder van het object; toegang aanvragen.
- **Samenwerken:** vergrendeling "in gebruik door" bij bureaubladapps; samen bewerken in de browser-editor; een workflowpaneel per object.
- **Tijdvak:** wissel tussen *Vandaag*, *Eind 2027* en *Horizon* om te zien wat er wanneer is.
- **Echte WebDAV-server:** koppel een echte server in het losse bestand (zie hieronder).

## Waarom deze combinatie

Elk van de drie bestaat al. Samen in één opslag zijn ze zeldzaam, en pas samen lossen ze het probleem van overheidsorganisaties op:

1. **Archiefwaardige opslag.** De opslag dwingt bewaartermijn en legal hold zelf af, per versie. Zonder dat blijft archiefwaardigheid een belofte van een toepassing.
2. **Functies voor informatiebeheer.** Metagegevens, dossiers, versies, toegangsregels en workflow geven elk bestand de context om het terug te vinden voor een Woo-verzoek, openbaar te maken of verantwoord te vernietigen.
3. **Integratie met vertrouwde vensters.** Zonder die integratie blijven bestanden op de oude schijven en in mailboxen staan.

Op opslag onder eigen beheer met open source, en met een identificatie die los staat van het platform, blijft de organisatie baas over haar informatie bij een volgende migratie. De aanpak vult Common Ground aan: het zaaksysteem blijft leidend voor zaken, deze opslag zorgt voor alle andere documenten.

## Architectuur en groeipad in het kort

- **Presentatielaag:** Verkenner, kantoorpakketten (Word, LibreOffice en andere) met de Office Assistent, documentbibliotheek, teamkanaal, Nextcloud Files in de browser, Nextcloud-client.
- **Nextcloud:** WebDAV-eindpunt, Files, een Governance- en Archief-app, de opslaginterface.
- **Archiefwaardige laag:** objectenregister, resolver en sleutelbeheer naast S3-objectopslag met versiebeheer, Object Lock en legal hold; één bucket per schijf.
- **Zes verzoeken aan Nextcloud:** wijzigingsmelding, gedelegeerde versiegeschiedenis, alleen-lezen metagegevens uit het register, bestuurde verwijdering, duurzame identiteit, en van bestand naar informatieobject. Alleen verzoek 3 en 4 vallen binnen de huidige opdracht.
- **Toegang:** nu rollen (RBAC); in stap 4 centrale beleidsregels die worden vertaald naar toegangsregels in Nextcloud; op de horizon een centraal beslispunt per verzoek (PBAC).

## Veelgestelde vragen

### Zoeken

**Waarom vind ik een bestand niet terug?**  
De zoekpagina toont alleen wat jij mag zien. Verborgen objecten zonder toegang blijven buiten beeld, en van versleutelde bestanden staat de inhoud niet in de zoekindex: die vind je op naam of kenmerken. Zoek je iets van een collega, vraag dan toegang aan.

### Woo

**Hoe verzamel ik documenten voor een Woo-verzoek?**  
Open Zoeken en kies "Zoeken voor een Woo-verzoek". Kies het Woo-dossier of maak het aan, zoek en voeg documenten toe vanuit de resultaten. Het dossier bevat verwijzingen: elk document blijft op zijn eigen plek en de opgenomen versie wordt vastgezet. Leg de zoekslag vast als je klaar bent.

**Valt een persoonlijk bestand onder een Woo-verzoek?**  
Bestanden die als persoonlijk zijn aangemerkt, vallen buiten de zoekslag. Daarom telt de keuze tussen zakelijk en persoonlijk: een werkdocument hoort niet op de persoonlijke schijf. Nog niet gemetadateerde bestanden vallen wel onder de zoekslag.

### Verwijderen

**Waarom kan ik dit bestand niet verwijderen?**  
Het bestand is zakelijk en valt onder een bewaartermijn, of het dossier staat onder legal hold. De opslag weigert het verwijderen; de melding noemt de reden. Klopt de registratie volgens jou niet, kies dan "Vraag aan informatiebeheer".

### Vergissing

**Ik heb een bestand per ongeluk als zakelijk opgeslagen.**  
Binnen de herroepingstermijn kies je in het eigenschappenpaneel "Registratie herroepen". Daarna kan dat niet meer: kies dan "Staat dit bestand hier ten onrechte?" en meld het bij de informatiebeheerder.

**Ik heb iets in het verkeerde dossier gezet.**  
Een vergrendeld bestand kun je niet uit een dossier halen. Meld het bij de informatiebeheerder via "Staat dit bestand hier ten onrechte?". Zolang de herroepingstermijn loopt, kun je het bestand nog zelf verwijderen.

### Kenmerken

**Wat is het verschil tussen zakelijk en persoonlijk?**  
Zakelijk hoort bij het werk van de gemeente en wordt gearchiveerd. Persoonlijk is alleen van jou en valt buiten de archivering. Gebruik voor persoonlijke bestanden je schijf M:.

**Wat betekent "Nog metadateren"?**  
Je hebt nog niet aangegeven of het bestand zakelijk of persoonlijk is. Open het eigenschappenpaneel, kies de aard en, bij zakelijk, een classificatie. Met het filter "Nog metadateren" zie je ze allemaal.

**Waarom moet ik een classificatie kiezen?**  
De classificatie bepaalt hoe lang een zakelijk bestand bewaard blijft. In een dossier hoef je niets te kiezen: het bestand neemt de classificatie van het dossier over.

### Verplaatsen

**Waarom kan ik een bestand niet uit een dossier verplaatsen?**  
Een bestand in een dossier hoort bij dat dossier. Je kunt wel een kopie maken; die wordt een nieuw informatieobject met een eigen identificatie.

**Wat gebeurt er als ik een bestand naar een andere schijf verplaats?**  
Elke schijf is een eigen bucket. De opslag maakt een kopie en zet een verwijdermarkering op het oude object. Is het bestand vergrendeld, dan blijft het oude object staan tot de termijn afloopt. De identificatie en de link blijven gelijk.

### Openen

**Het bestand opent alleen-lezen.**  
Een collega heeft het bestand open in de bureaubladapp; je ziet "In gebruik door". Kies "Meld mij als het vrij is", of werk samen in de browser-editor. Ook pdf-bestanden en bestanden onder legal hold openen alleen-lezen.

### Versies

**Hoe zet ik een eerdere versie terug?**  
Open het eigenschappenpaneel, tabblad Versies, en kies "Terugzetten" bij de versie die je wilt. Er komt een nieuwe versie bij met de oude inhoud.

### Delen

**Hoe deel ik een bestand met iemand buiten de gemeente?**  
Kies Delen en maak een link voor extern gebruik. Die is alleen-lezen en heeft een einddatum. Vertrouwelijke bestanden kun je niet extern delen.

**Blijft een gedeelde link werken als het bestand wordt verplaatst?**  
Ja. De link verwijst naar het register, niet naar de plek van het bestand. Het register weet waar het bestand nu staat.

### Toegang

**Ik zie een dossier, maar niet wat erin zit.**  
Je hebt geen toegang, maar de beheerder heeft het dossier vindbaar gemaakt. Kies "Toegang aanvragen"; de beheerder van het object beslist.

**Waarom is een knop uitgeschakeld?**  
Je recht op dit object is niet voldoende, bijvoorbeeld alleen lezen. In het eigenschappenpaneel zie je je recht en de beheerder. Kies daar "Meer rechten aanvragen".

**Wie bepaalt of anderen een dossier kunnen zien?**  
De beheerder van het object. Die kiest tussen vindbaar en verborgen. Personeelsdossiers en vertrouwelijke dossiers zijn standaard verborgen.

### Schijven

**Ik zie een netwerkschijf niet.**  
Kies "Netwerkschijf koppelen" en vul het WebDAV-adres van de schijf in. Je ziet alleen de dossiers waarvoor je rechten hebt.

**Kan ik een echte WebDAV-server koppelen?**  
Ja, in de losse versie van deze pagina, via "Netwerkschijf koppelen" met adres, gebruikersnaam en wachtwoord. Bladeren en openen werken, maar ook aanmaken, hernoemen en verwijderen gebeuren echt op de server, en verwijderen is definitief. De archieffuncties werken daar niet: een kale WebDAV-server kent geen metagegevens, bewaartermijnen of dossiers, en kopiëren, knippen en plakken zijn uitgeschakeld. Gebruik daarom een testserver met testgegevens. Binnen claude.ai werkt de koppeling niet, omdat de pagina daar geen verbindingen naar buiten mag maken.

**De koppeling met een echte server geeft een foutmelding.**  
Meestal staat de server het verkeer vanuit de browser niet toe (CORS), of wordt het certificaat niet vertrouwd. Onder "Voor beheerders" in deze Help staat wat de beheerder van de server moet instellen.

**Waarom staat een knop grijs met "stap 3"?**  
Het tijdvak bovenaan staat op "Vandaag" of "Eind 2027". Die functie komt pas in een latere stap van het groeipad. Zet het tijdvak op "Horizon" voor het volledige beeld.

### Vertrouwelijk

**Wat gebeurt er met een vertrouwelijk bestand?**  
Het wordt bij opslaan versleuteld met een eigen sleutel. Alleen wie toegang heeft, kan het lezen. Opheffen kan alleen de informatiebeheerder.

## Begrippen

| Begrip | Betekenis |
|---|---|
| Informatieobject | Een bestand, dossier of ander stuk met een eigen identificatie in het register, los van waar het staat. |
| Zakelijk en persoonlijk | Zakelijk hoort bij het werk van de gemeente en wordt gearchiveerd. Persoonlijk is alleen van de medewerker en valt erbuiten. |
| MDTO | Metagegevens Duurzaam Toegankelijke Overheidsinformatie: de landelijke standaard voor archiefmetagegevens. |
| Dossier | Een map met extra kenmerken, bij één project, object, zaak of verzoek. Bestanden die erin staan, nemen classificatie en bewaartermijn over. |
| Verwijzing | Een document dat in een dossier is opgenomen zonder dat het daar staat. Het blijft op zijn eigen plek en wordt niet gekopieerd. Verwijderen van de verwijzing laat het document staan. |
| Classificatie | De categorie uit de selectielijst. Die bepaalt of iets bewaard of vernietigd wordt, en na hoeveel jaar. |
| Bewaartermijn | Hoe lang een zakelijk bestand bewaard moet blijven. De termijn loopt vanaf registratie of vanaf het afsluiten van het dossier. |
| Object Lock | Een functie van S3-objectopslag die een versie vergrendelt tot een datum. Niemand kan die versie eerder verwijderen. |
| Legal hold | Een bevriezing zonder einddatum, bijvoorbeeld tijdens een rechtszaak. Pas na opheffen kan er weer iets veranderen. |
| Register | De administratie van alle informatieobjecten: identificatie, metagegevens, dossier, toegang en plek in de opslag. |
| Resolver | De dienst achter de deellinks. Die zoekt in het register op waar een object nu staat. |
| Herroepingstermijn | De korte periode na registratie waarin je een vergissing nog zelf kunt herstellen, voordat de vergrendeling ingaat. |
| Bucket | Een afgebakende ruimte in de objectopslag. In deze demo is elke schijf een eigen bucket. |
| WebDAV | Het protocol waarmee de Verkenner, Office en andere vensters bestanden lezen en schrijven bij Nextcloud. |
| Rechten | Lezen, bewerken en beheren. Beheren omvat ook het regelen van toegang en zichtbaarheid. |
| Zichtbaarheid | Of wie geen toegang heeft een object toch kan vinden: vindbaar of verborgen, ingesteld door de beheerder. |
| PBAC | Policy-based access control: toegang volgt uit beleidsregels over kenmerken van persoon, object en situatie, in plaats van uit losse toegangslijsten. |

---

*Overgenomen uit de demo XENA (README en Help) van Erik Hoekstra, Gemeente Haarlem; EUPL 1.2. Een demo, geen product: alle voorbeeldgegevens zijn verzonnen, en een deel is toekomstbeeld. Wijzigt de demo, dan is [de repo](https://github.com/EHa-1999/XENA) leidend.*
