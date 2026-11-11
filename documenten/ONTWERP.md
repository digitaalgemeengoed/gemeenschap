# Documenten: een lichte XENA voor het netwerk

Ontwerp, 11 november 2026. Status: fase 1 klaar (0.9), verkenner en rechten (0.10).

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
projectleden (per rol in Redmine) een niveau, op de ruimte of op een map
(tabel `recht`). Rechten staan op mappen, niet op losse documenten: een
besloten document zet je in een besloten map.

**Overerving.** Een map neemt de regels van de map erboven over en kan er
regels bij krijgen. *Overerving stoppen* (`item.eigen_rechten`) kopieert de
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

Een map verwijderen kan alleen met Beheren op de map erboven en op elke map
eronder.

## Inloggen

- Browser: OIDC bij Keycloak (client `documenten`), code flow met PKCE. Inloggen
  via het publieke adres, tokens en sleutels rechtstreeks bij Keycloak
  (zoals oauth2-proxy). De groepen komen bij elke login mee.
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

## Fasering

1. Garage en de dienst: ruimtes, mappen, uploaden en downloaden in de
   browser, metadata, versies, inloggen via Keycloak, back-up.
2. WebDAV (`/dav/`, `golang.org/x/net/webdav` met een eigen bestandssysteem
   op het register) en app-wachtwoorden. Testen met Verkenner, Finder,
   LibreOffice, Word, Cyberduck en rclone.
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
