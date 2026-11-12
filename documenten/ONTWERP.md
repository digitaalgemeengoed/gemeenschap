# Documenten: een lichte XENA voor het netwerk

Ontwerp, 11 november 2026. Status: fase 1 (0.9), verkenner, rechten, delen en WebDAV (0.10).

## Vraag

Leden willen documenten uploaden en bewaren, voor de coöperatie zelf en
binnen projecten, en ze kunnen openen via WebDAV en een lichte
bestandsbeheerder in de browser.

## Uitgangspunten

- **Een werkruimte, op het fundament van XENA.** Geen archief: geen
  bewaartermijnen, geen vernietiging, geen waarmerken. Wel de kern van XENA:
  een register (wat, van wie, welke versie, welke metadata) los van de
  objectopslag (de bytes, nooit overschreven).
- **Alleen werkdocumenten van het netwerk**: notities, verslagen,
  presentaties, ontwerpen. Geen persoonsgegevens en geen vertrouwelijke
  gemeentelijke stukken (Voorwaarden).
- **Licht.** Eén kleine dienst (Go, één binary) plus Garage als S3-opslag.
  Geen Nextcloud: te zwaar voor deze club.
- **Rechten volgen wat er al is.** Keycloak voor wie je bent en je rol in het
  netwerk, Redmine voor wie in welk project zit. Niemand houdt rechten met de
  hand bij.

## Opbouw

```
browser ──► Caddy ──► dg-documenten ──► documenten-db (Postgres: register)
WebDAV  ──►  (documenten.<domein>)   └─► garage (S3, alleen intern: objecten)
                     │
                     ├─ Keycloak (inloggen, groepen)
                     └─ Redmine API (projecten en hun leden, elke 5 min)
```

**Garage** (Deuxfleurs, v2.5) draait als één node zonder replicatie
(`--single-node --default-bucket`); sleutel en bucket komen uit `.env`.
MinIO is als open source gestopt en valt daarom af. Garage is alleen binnen
het Docker-netwerk bereikbaar. Overstappen naar S3 bij een aanbieder kan
later zonder dat de dienst verandert (alleen het eindpunt en de sleutels).

**dg-documenten** heeft geen afhankelijkheden buiten de Go-standaardbibliotheek
en de Postgres-driver: OIDC en S3 (SigV4) zitten er zelf in, klein en
getest.

## Register (Postgres)

| Tabel | Wat |
|---|---|
| `ruimte` | Coöperatie, en één per Redmine-project (`project:<identifier>`), met status (actief, gesloten = alleen lezen, weg) |
| `lid` | wie in een projectruimte mag (uit Redmine), per ronde ververst |
| `item` | de boom: mappen en documenten, met ouder, naam en voor documenten de metadata en de huidige versie; verwijderen = `verwijderd_op` zetten |
| `versie` | elke upload: objectsleutel, SHA-256, grootte, type, wie en wanneer |
| `logboek` | elke handeling: wie, wat, welk item, wanneer |
| `sessie` | ingelogde browsers |
| `app_wachtwoord` | (fase 2) voor WebDAV, gehasht, per apparaat intrekbaar |

Een object in Garage heet `objecten/<uuid>` en wordt nooit overschreven.
Hernoemen en verplaatsen veranderen alleen het register. Een nieuwe versie
is een nieuw object; de oude blijven.

**Metadata** (licht): titel, omschrijving, documentsoort (notitie, verslag,
presentatie, ontwerp, overeenkomst, overig), trefwoorden; automatisch:
auteur, aangemaakt, gewijzigd, grootte, type, SHA-256. In fase 3 komt de
status *concept* / *definitief* (definitief = vergrendeld).

## Rechten

Drie niveaus, zoals in de XENA-demo (venster Toegang):

| Niveau | Mag |
|---|---|
| Lezen | zien en downloaden |
| Bewerken | ook uploaden, nieuwe versies, metadata, hernoemen, mappen maken |
| Beheren | ook verwijderen en de toegang wijzigen, ook Beheren doorgeven |

Een **regel** geeft een persoon (e-mailadres), een groep uit Keycloak of de
projectleden (per rol in Redmine) een niveau, op de ruimte, een map of een
document (tabel `recht`).

**Overerving.** Een map of document neemt de regels van de map erboven over en
kan er regels bij krijgen. *Overerving stoppen* (`item.eigen_rechten`) kopieert de
geldende regels naar de map; daarna gelden alleen de eigen regels. Zo maak je
een besloten map. `/site/beheer` heeft altijd Beheren, zodat niemand zich
buitensluit.

**Standaardregels** op elke ruimte (niet te wijzigen, wel weg te laten in een
besloten map):

- Coöperatie: `/netwerk/members` Lezen, `/netwerk/kerngroep` Beheren.
- Project: de projectleden volgens hun rol in Redmine, en `/netwerk/kerngroep`
  Beheren. Elk project heeft in Redmine de netwerkgroepen als lid; iemand die
  zich net aanmeldt (Gebruiker) ziet daarom niets. Rollen: Gebruiker 0,
  Contributor en Member Bewerken, Kerngroep, Manager, Projectmanager en
  Projectleider Beheren; een andere rol Bewerken (`DOC_ROLLEN`).

Een gesloten project kan alleen nog gelezen worden. Wie toegang heeft tot een
map diep in een ruimte, ziet de ruimte en de mappen erboven als doorgang.

Een map verwijderen kan alleen met Beheren op de map en op alles eronder.
Uploaden met de naam van een bestaand document is een nieuwe versie daarvan:
daarvoor is Bewerken op dat document nodig.

## Delen

Delen = toegang geven plus een bericht. Alleen met Beheren. Je kiest een
persoon, groep of de projectleden en Lezen (standaard) of Bewerken; er komt een
regel op de map of het document (een bestaand hoger recht blijft). Het bericht
gaat via BuddyPress: de dienst roept `/wp-json/dg/v1/bericht` aan in de
mu-plugin `dg-gemeenschap.php`, met een gedeeld geheim; WordPress zoekt de
ontvangers op e-mailadres of op groep (de groepen uit Keycloak staan bij elke
login in `dg_groepen`) en stuurt elk een eigen bericht. Caddy laat dit adres
niet van buiten toe.

De link is altijd de besloten link: alleen wie toegang heeft, kan hem openen.
Kopiëren en doorsturen (LinkedIn, Bluesky, X, WhatsApp, e-mail, Web Share)
kan iedereen die het kan lezen. Openbare links (iedereen met de link) zijn er
bewust niet: dat is openbaar maken, en vraagt een eigen besluit.

## Inloggen

- Browser: OIDC bij Keycloak (client `documenten`), code flow met PKCE. Inloggen
  via het publieke adres, tokens en sleutels rechtstreeks bij Keycloak
  (zoals oauth2-proxy). De groepen komen bij elke login mee, en worden daarna
  bij elk verzoek in Keycloak nagekeken (hooguit een minuut oud; client
  `redmine-sync`, zie `groepen.go`). Een nieuwe of afgenomen rol telt dus
  zonder opnieuw in te loggen; een geblokkeerd account is meteen uitgelogd.
  Is Keycloak even weg, dan gelden de groepen die er al waren.
- WebDAV (fase 2): app-wachtwoorden, in de browser aan te maken.

## Grenzen

- 500 MB per bestand; geen quotum per ruimte zolang er ruimte is.
- Garage zonder replicatie: de back-up is de tweede kopie.

## Back-up en herstel

- `documenten.dump`: het register (pg_dump), per nacht.
- `documenten-objecten/`: een spiegel van alle objecten, gedeeld door alle
  nachten. Objecten veranderen nooit, dus de spiegel groeit alleen aan
  (`dg-documenten spiegel`).
- Herstellen: het register terug, daarna ontbrekende objecten terug naar
  Garage (`dg-documenten terugspiegelen`). Extra objecten in Garage zijn
  onschadelijk.

## WebDAV en openen in een programma (fase 2)

- `/dav/<ruimte>/<pad>`: de ruimtes, mappen en documenten als bestandssysteem
  (`internal/webdav`, overgenomen uit `golang.org/x/net/webdav`). Inloggen met
  Basic: e-mailadres + app-wachtwoord (tabel `app_wachtwoord`, alleen de
  SHA-256). De groepen komen live uit Keycloak, zoals in de browser
  (`persoon.groepen` als laatst bekende stand).
- `/dav/` zelf toont alleen de ruimtes, als mappen met de sleutel als naam
  (die blijft gelijk als een project van naam verandert). Uitzondering: je
  eigen persoonlijke ruimte heet `Mijn documenten` (de sleutel `mijn-…`
  werkt ook) en wordt bij het openen van de schijf aangemaakt als je er recht
  op hebt. Iets neerzetten in `/dav/` zelf geeft 403.
- Foutcodes (aanpassing in `internal/webdav`, zie HERKOMST.md): geen toegang
  is 403, een ongeldige naam 400, vol of te groot 507.
- `/o/<sleutel>/<naam>`: één document, met een tijdelijke link (tabel
  `open_link`, tien uur). De knoppen *Openen in Word / LibreOffice* starten
  het programma met `ms-word:ofe|u|<link>` of
  `vnd.libreoffice.command:ofe|u|<link>` (`ofv` als je alleen mag lezen).
  Afweging: de sleutel in de link werkt als een wachtwoord voor dat ene
  document, zolang hij geldig is; daar staat tegenover dat het programma
  zonder vragen opent en opslaat. Een app-wachtwoord blijft de weg voor een
  gekoppelde schijf.
- **Versies.** Elke PUT is een versie. Dezelfde persoon binnen tien minuten:
  de laatste versie wordt vervangen (het oude object blijft in de opslag,
  zonder verwijzing). **Veilig opslaan** (tijdelijk bestand → origineel
  hernoemd of verwijderd → tijdelijk bestand krijgt de oude naam): de dienst
  onthoudt twee minuten welk document een naam verliet; krijgt een ander
  document die naam, dan wordt dat een nieuwe versie van het oude
  (`Register.Opnemen`). Het verwijderen van de reservekopie daarna lukt
  gewoon.
- **Vluchtig.** `._*`, `.DS_Store`, `Thumbs.db`, `desktop.ini`, `~$*`,
  `.~lock.*` blijven in het geheugen (tot 1 MB, twee uur), buiten het register
  en de back-up.
- **Vergrendelen** en dode eigenschappen (PROPPATCH) in het geheugen; bij een
  herstart zijn ze weg. Eén exemplaar van de dienst, dus dat is genoeg.
- Verwijderen via WebDAV vraagt Beheren, behalve een document dat je zelf
  minder dan tien minuten geleden aanmaakte (tijdelijke bestanden).
- Caddy laat `/dav/` en `/o/` buiten de afscherming met wachtwoord.

## Ruimtes, opslag en beheer (0.11)

Besloten op 11 november 2026:

1. **Werkgroepen** zijn de groepen van BuddyPress op de site. Elke werkgroep
   krijgt een ruimte (`werkgroep-<slug>`); de leden komen elke vijf minuten
   uit WordPress (`/wp-json/dg/v1/werkgroepen`, met het gedeelde geheim).
   Lid: Bewerken; beheerder of moderator van de groep: Beheren; de kerngroep:
   Beheren.
2. **Eén bucket, een voorvoegsel per ruimte**: nieuwe objecten heten
   `objecten/r<ruimte-id>/<uuid>`. Zo is de omvang per ruimte te meten en kan
   één ruimte apart worden gekopieerd. Bestaande objecten houden hun naam.
3. **Persoonlijke ruimte** (*Mijn documenten*) voor wie in de kerngroep, een
   werkgroep of een project zit, en (besloten op 11 november, 's avonds) voor
   de beheerders van het platform (`/site/beheer`). Aangemaakt bij het eerste gebruik. Alleen de
   eigenaar (Beheren); hij kan delen. Ook de beheerders van het platform
   kunnen er niet in (de vaste regel geldt hier niet); in Beheer zien zij
   alleen de omvang. Is iemand nergens meer lid, dan wordt de ruimte
   **bevroren**: 90 dagen alleen-lezen (ook geen nieuwe documenten), met een
   bericht. Wordt hij weer lid, dan gaat hij weer open. Na 90 dagen gaat
   alles naar de prullenbak. Wie in de kerngroep zit, komt uit de groep
   *Netwerk: Kerngroep* in Redmine (elke tien minuten gelijk met Keycloak).
   De levenscyclus draait alleen na een geslaagde synchronisatie van Redmine
   én WordPress, zodat een storing niemands ruimte bevriest.
4. **Quotum alleen persoonlijk**: 1 GB (`DOC_QUOTUM_PERSOONLIJK_MB`). Alle
   versies van wat niet in de prullenbak staat, tellen mee. Andere ruimtes
   hebben geen quotum, tenzij Beheer er een zet.

**Beheer** (`/beheer`, voor `/site/beheer` en de kerngroep): alle ruimtes
met omvang, quotum en status; het quotum wijzigen; de prullenbak per ruimte
(terugzetten, definitief verwijderen: dan gaan ook de objecten uit de
opslag, niet uit de back-upspiegel); het logboek.

**Op de site**: een regel *Documenten* in de feiten van elke projectpagina
(`[dg_documenten project=…]`, alleen voor wie is ingelogd), een tab
*Documenten* bij elke werkgroep en *Mijn documenten* op je eigen profiel.

## Fasering

1. Garage en de dienst: ruimtes, mappen, uploaden en downloaden in de
   browser, metadata, versies, inloggen via Keycloak, back-up.
2. WebDAV en app-wachtwoorden, openen in Word en LibreOffice (klaar in 0.10;
   nog te testen met Verkenner, Finder, Word en LibreOffice zelf).
3. Definitief (vergrendelen), zoeken, de prullenbak terughalen, het logboek
   per document.

Tussendoor (0.10): de verkenner in de stijl van de XENA-demo en rechten per
persoon of groep op mappen.

Later: werkgroepen, de Office Assistent schrijft via WebDAV, bewaartermijnen.

## Afgewogen

- **Nextcloud**: alles erin, maar te zwaar en te veel onderhoud.
- **OpenCloud**: lichter, Spaces passen, maar lidmaatschap koppelen kan alleen
  via de API, en het register is niet van ons.
- **SFTPGo + eigen metadata**: minder eigen code, maar metadata blijft een
  aanhangsel en hernoemen op S3 is kopiëren. Het register als kern is het
  hele punt.
