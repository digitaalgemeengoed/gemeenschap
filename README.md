# Gemeenschapsplatform Digitaal Gemeengoed

Het platform van Digitaal Gemeengoed en NL-Dit voor het netwerk rond de
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
| Leden, organisaties, wie in welke groep | Keycloak | Keycloak-beheer |
| Thema, plugins, instellingen van WordPress, CiviCRM en Redmine | `wordpress/`, `redmine/` | GitHub |
| Wachtwoorden en geheimen | `/opt/gemeenschap/.env` op de server | alleen op de server, nooit in deze repo |

Keycloak is de bron voor wie lid is. WordPress, CiviCRM en Redmine nemen
dat over bij het inloggen (Redmine elke tien minuten):

| In Keycloak | Wordt in WordPress | In bbPress | In Redmine |
|---|---|---|---|
| `/netwerk/gebruikers` (standaard) | abonnee, ledentype Gebruiker | deelnemer | groep Netwerk: Gebruikers |
| `/netwerk/contributors` | ledentype Contributor | deelnemer | groep Netwerk: Contributors |
| `/netwerk/members` | ledentype Member | deelnemer | groep Netwerk: Members |
| `/netwerk/kerngroep` | ledentype Kerngroep | moderator | groep Netwerk: Kerngroep |
| `/site/redactie` | redacteur | moderator | — |
| `/site/beheer` | beheerder | keymaster | beheerder |

Profielvelden uit het aanmeldformulier (organisatie, functie, werkveld,
toestemming voor de nieuwsbrief) komen in het BuddyPress-profiel (alleen
zichtbaar voor ingelogde leden) en in CiviCRM, met een activiteit bij elke
wijziging van de toestemming.

## Bestanden

```
inhoud/                     pagina's, menu's en afbeeldingen van de site
keycloak/realm/             de realm gemeenschap (keycloak-config-cli)
keycloak/themes/            kleuren van het aanmeld- en inlogscherm
wordpress/dg-gemeenschap.php  mu-plugin: inloggen, rollen, profiel, CiviCRM
wordpress/dg-inhoud.php       mu-plugin: pagina's uit inhoud/, op slot
wordpress/thema/            thema (Twenty Twenty-Five met eigen kop en voet)
wordpress/inrichten.sh      wp-cli: WordPress, plugins, CiviCRM
redmine/                    image met redmine_oauth, inrichten, groepssync
caddy/Caddyfile             de vier adressen, certificaten
compose.yml                 alle containers
server/                     installeren, bijwerken, uitrollen, back-up, controleren
```

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
| Keycloak-beheer | `ssh -L 8080:127.0.0.1:8080 beheer@<server>`, dan `http://localhost:8080/admin` |

Met `UITROL_MELDEN=<adres>` in `.env` krijg je een mail bij elke nieuwe
versie en bij elke fout.

Back-ups: elke nacht en vlak voor elke nieuwe versie, naar
`/var/backups/dg-gemeenschap`, 14 dagen. Kopieer die map naar een andere
aanbieder; `.env` hoort in je wachtwoordmanager, niet in de back-up.

## Privacy

Digitaal Gemeengoed en NL-Dit zijn samen verantwoordelijk (AVG artikel 26).
De privacyverklaring staat in `inhoud/paginas/privacy.md`; elke versie is
terug te vinden in de geschiedenis van die pagina.

## Licentie

EUPL 1.2, zie [LICENSE](LICENSE).
