# Gemeenschapsplatform Digitaal GemeenGoed

Het platform van Digitaal GemeenGoed en NL-Dit voor het netwerk rond de
[Office Assistent](https://github.com/digitaalgemeengoed): lid worden,
profielen, fora, nieuwsbrief, meldingen en projecten. Deze repo is de bron
voor de hele server: de software, de instellingen en de pagina's van de site.
Wat hier op `main` en in versie-tags staat, staat binnen vijf minuten live.

| Adres | Dienst | Waarvoor |
|---|---|---|
| `digitaalgemeengoed.nl` | WordPress met BuddyPress, bbPress en CiviCRM | site, leden en groepen, fora onder `/forum/`, nieuwsbrief en contacten |
| `aanmelden.digitaalgemeengoed.nl` | Keycloak, realm `gemeenschap` | lid worden, inloggen, wachtwoord vergeten |
| `projecten.digitaalgemeengoed.nl` | Redmine | meldingen, wensen, roadmap, portfolio |
| `forum.digitaalgemeengoed.nl` | (doorverwijzing) | naar `/forum/` |
| `kennis.digitaalgemeengoed.nl` | Wiki.js 2 | de kennisbank, alleen voor leden; pagina's ook in de repo `kennisbank` |
| `architectuur.digitaalgemeengoed.nl` | Archi-rapporten achter oauth2-proxy | de architectuurmodellen, alleen voor leden |

## Een pagina wijzigen

Zie [inhoud/README.md](inhoud/README.md). Kort: wijzig het bestand in
`inhoud/paginas/`, open een pull request, voeg samen. Binnen vijf minuten
staat het op de site.

## Hoe de server zichzelf bijwerkt

De server kijkt elke vijf minuten op GitHub (`server/uitrol.sh`, systemd-timer
`dg-uitrol`). GitHub heeft geen toegang tot de server; de server haalt op.

| Wat | Wanneer live | Hoe |
|---|---|---|
| `inhoud/` (pagina's, menu's, afbeeldingen) | na samenvoegen op `main` | `wp dg inhoud`: Markdown naar WordPress-blokken. Een fout in een bestand verandert niets op de site. |
| al het andere | bij een nieuwe versie-tag `vX.Y.Z` op `main` | back-up, `server/bijwerken.sh`, controle of de site antwoordt; zo niet, terug naar de vorige tag |

Een nieuwe versie uitrollen:

```
git tag v0.2.0 && git push origin v0.2.0
```

of op GitHub via **Releases → Draft a new release**. Een tag die niet op
`main` staat, wordt genegeerd; een mislukte versie wordt niet opnieuw
geprobeerd (zie `/var/lib/dg-gemeenschap/mislukt` op de server). Zet een
nieuwe versie liever eerst op een testserver als er migraties in zitten:
terugzetten herstelt de containers en instellingen, niet de database.

**Bescherm de repo.** Wie op `main` mag samenvoegen, verandert de site; wie een
tag mag zetten, verandert de server. Zet in **Settings → Rules** een regel
voor `main` (alleen via een pull request, één goedkeuring) en een regel dat
alleen beheerders tags `v*` mogen maken.

## Wie is de bron van wat

| Wat | Bron | Waar je het wijzigt |
|---|---|---|
| Pagina's, menu's | `inhoud/` | GitHub (in WordPress op slot) |
| Nieuws, fora, berichten | WordPress | WordPress |
| Groepen, profielvelden, clients, mailinstellingen van Keycloak | `keycloak/realm/gemeenschap.json` | GitHub (bij elke versie gelijkgetrokken met [keycloak-config-cli](https://github.com/adorsys/keycloak-config-cli)) |
| Leden, organisaties, wie in welke groep | Keycloak | Terug naar een eerdere versie (containers en instellingen, geen databases) | `sudo /opt/gemeenschap/server/terugzetten.sh` (lijst) / `… terugzetten.sh v0.8.0` |
| Een back-up terugzetten (databases en bestanden) | `sudo /opt/gemeenschap/server/herstellen.sh` (lijst) / `… herstellen.sh --proef <map>` / `… herstellen.sh <map> [keycloak] [wordpress] [redmine] [kennisbank]` |
| Keycloak-beheer |
| Thema (BuddyX met ons kindthema: logo, kleuren, voet), plugins, instellingen van WordPress, CiviCRM en Redmine | `wordpress/`, `redmine/` | GitHub. Kleuren en voet staan vast in `wordpress/thema/digitaalgemeengoed/functions.php`; wat in de Customizer staat, telt daarvoor niet. |
| Pagina's van de kennisbank | Wiki.js (database) | in de wiki; met de Git-opslag ook als Markdown in de repo `kennisbank` (beide richtingen, elke vijf minuten) |
| Instellingen van de kennisbank (inloggen, groepen, kleuren, Git-opslag) | `wiki/inrichten.py` | GitHub; bij elke versie gelijkgetrokken |
| Architectuurmodellen | één repo per project (coArchi), lijst in `architectuur/repos.txt` | in Archi met coArchi; de lijst via een pull request (na samenvoegen live, zonder versie) |
| Wachtwoorden en geheimen | `/opt/gemeenschap/.env` en `/opt/gemeenschap/geheimen/` op de server | alleen op de server, nooit in deze repo |

Keycloak is de bron voor wie lid is. WordPress, CiviCRM, Redmine en de
kennisbank nemen dat over bij het inloggen (Redmine elke tien minuten). De
architectuurmodellen ziet iedereen in `/netwerk/gebruikers`.

| In Keycloak | Wordt in WordPress | In bbPress | In Redmine | In de kennisbank |
|---|---|---|---|---|
| `/netwerk/gebruikers` (standaard) | abonnee, ledentype Gebruiker | deelnemer | groep Netwerk: Gebruikers | lezen en schrijven |
| `/netwerk/contributors` | ledentype Contributor | deelnemer | groep Netwerk: Contributors | (als gebruikers) |
| `/netwerk/members` | ledentype Member | deelnemer | groep Netwerk: Members | (als gebruikers) |
| `/netwerk/kerngroep` | ledentype Kerngroep | moderator | groep Netwerk: Kerngroep | (als gebruikers) |
| `/site/redactie` | redacteur | moderator | — | ook verplaatsen en verwijderen |
| `/site/beheer` | beheerder | keymaster | beheerder | beheerder |

Profielvelden uit het aanmeldformulier (organisatie, functie, werkveld,
toestemming voor de nieuwsbrief) komen in het BuddyPress-profiel (alleen
zichtbaar voor ingelogde leden) en in CiviCRM, met een activiteit bij elke
wijziging van de toestemming.

## Bestanden

```
inhoud/                     pagina's, menu's en afbeeldingen van de site
ontwerp/                    logo, ontwerpvisualisaties en LICENTIES.md (herkomst van elke afbeelding)
keycloak/realm/             de realm gemeenschap (keycloak-config-cli)
keycloak/themes/            kleuren van het aanmeld- en inlogscherm
wordpress/dg-gemeenschap.php  mu-plugin: inloggen, rollen, profiel, CiviCRM
wordpress/dg-inhoud.php       mu-plugin: pagina's uit inhoud/, op slot
wordpress/thema/            kindthema van BuddyX: logo, kleuren, voet
wordpress/inrichten.sh      wp-cli: WordPress, plugins, CiviCRM
redmine/                    image met redmine_oauth, inrichten, groepssync
wiki/                       de kennisbank: image (vertalingen, logo, offline), inrichten.py, startinhoud/
architectuur/repos.txt      de architectuurmodellen (één repo per project)
caddy/Caddyfile             de adressen, certificaten, afscherming
compose.yml                 alle containers
server/                     installeren, bijwerken, uitrollen, back-up, controleren
```

## Kennisbank

Wiki.js 2 op `kennis.<domein>`. Alleen leden: wie niet is ingelogd, gaat naar
Keycloak. Alle leden kunnen lezen en schrijven; de groepen komen bij elke
login uit Keycloak. Het noodaccount `beheer@<domein>` (wachtwoord
`WIKI_BEHEER_WACHTWOORD` in `.env`) staat niet op het inlogscherm; het is voor
`wiki/inrichten.py` en voor nood.

De wiki draait offline: de Nederlandse en Engelse teksten zitten in de image,
hij haalt niets op bij de dienst van Wiki.js.

**Startinhoud.** `wiki/startinhoud/` bevat de eerste pagina's: een algemene
sectie (werkwijze, rollen, meldingen, bijdragen, crowdfunding, privacy, FAQ),
de projecten en de demo's. `wiki/inrichten.py` zet een bestand alleen in de
wiki als die pagina er nog niet is; daarna wijzig je hem in de wiki (of in de
repo `kennisbank`). Een nieuw bestand hier komt er bij de volgende versie bij.

**Pagina's ook in GitHub.** Eenmalig:

1. Maak de repo `digitaalgemeengoed/kennisbank`, **privé** (de kennisbank is
   alleen voor leden), met een eerste commit op `main` (bijvoorbeeld een README).
2. Zet de publieke sleutel van de server als **deploy key met schrijfrechten**
   bij die repo (Settings → Deploy keys). `bijwerken.sh` toont hem zolang de
   koppeling er niet is; anders: `sudo cat /opt/gemeenschap/geheimen/kennisbank.pub`.
3. Zet in `.env`: `KENNISBANK_REPO=git@github.com:digitaalgemeengoed/kennisbank.git`
   en draai `sudo /opt/gemeenschap/server/bijwerken.sh`.

Daarna staat elke pagina als Markdown in de repo (met wie hem schreef), en
komt een wijziging in de repo binnen vijf minuten in de wiki.

## Architectuurmodellen

`architectuur.<domein>` toont het HTML-rapport van Archi voor elk model, alleen
voor leden. Eén repo per project of domein, in het formaat van coArchi.

1. Maak de repo (bijvoorbeeld `digitaalgemeengoed/architectuur-xena`) en zet
   het model erin met coArchi in Archi.
2. Voeg een regel toe aan `architectuur/repos.txt` (zie de uitleg daar) en
   voeg samen op `main`. Een nieuwe versie is niet nodig.
3. Binnen een kwartier staat het rapport op `architectuur.<domein>/<naam>/`.
   Bij elke commit in de modelrepo wordt het opnieuw gebouwd.

Privé-repo's: zet `ARCH_GITHUB_TOKEN` in `.env`, een *fine-grained* token met
alleen *Contents: read* op de modelrepo's. Wat er gebeurde:
`sudo grep architectuur /var/lib/dg-gemeenschap/logboek`; alles opnieuw bouwen:
`sudo /opt/gemeenschap/server/architectuur.sh --opnieuw`.

Zet in een model geen netwerkinrichting, beveiligingsmaatregelen of
leveranciersafspraken van een gemeente (BIO); referentie- en
oplossingsarchitectuur wel.

## Installeren

Op een nieuwe VPS, met de basis uit
[oa-uitrol](https://github.com/digitaalgemeengoed/oa-uitrol) (INRICHTING.md,
stap 12): `inrichten.sh` (kies 4), `ssh-afsluiten.sh`, `installeer.sh`.

## Beheer op de server

| Wat | Opdracht |
|---|---|
| Hoe staat het ervoor | `sudo /opt/gemeenschap/server/controleer.sh` |
| Wat is er uitgerold | `sudo /opt/gemeenschap/server/uitrol.sh --status` |
| Nu meteen uitrollen | `sudo /opt/gemeenschap/server/uitrol.sh` |
| Pauzeren / hervatten | `sudo touch /var/lib/dg-gemeenschap/pauze` / `sudo rm …/pauze` |
| Na een wijziging in `.env` | `sudo /opt/gemeenschap/server/bijwerken.sh` |
| Afschermen met wachtwoord (zolang het niet openbaar is) | `sudo /opt/gemeenschap/server/afschermen.sh aan` / `… uit` |
| Keycloak-beheer | `ssh -L 8080:127.0.0.1:8080 beheer@<server>`, dan `http://localhost:8080/admin` |

Met `UITROL_MELDEN=<adres>` in `.env` krijg je een mail bij elke nieuwe
versie en bij elke fout.

**Terugzetten of herstellen?** Na een nieuwe versie gaat er iets mis: eerst
`terugzetten.sh` naar de vorige versie. De nieuwere versie komt dan niet
vanzelf terug; een volgende, nieuwe versie rolt gewoon uit. Veranderde de
nieuwe versie ook iets in een database, of zijn er gegevens kwijt: daarna
`herstellen.sh` met de back-up van vlak voor die versie (`uitrol.sh` maakt die
altijd). Beide maken eerst een back-up van nu, dus beide kun je ongedaan maken.
Pagina's (`inhoud/`) zet je terug met een revert op GitHub.

Back-ups: elke nacht en vlak voor elke nieuwe versie, naar
`/var/backups/dg-gemeenschap`, 14 dagen. Kopieer die map naar een andere
aanbieder; `.env` hoort in je wachtwoordmanager, niet in de back-up.

## Privacy

Digitaal GemeenGoed en NL-Dit zijn samen verantwoordelijk (AVG artikel 26).
De privacyverklaring staat in `inhoud/paginas/privacy.md`; elke versie is
terug te vinden in de geschiedenis van die pagina.

## Licentie

EUPL 1.2, zie [LICENSE](LICENSE).
