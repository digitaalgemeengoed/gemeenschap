# Beheer (dg-beheer): ontwerp

`beheer.<domein>`: één plek voor het dagelijkse beheer van het platform, zonder
de console van Keycloak. Voor de **beheerders van het platform** (`/site/beheer`)
en de **kerngroep** van de coöperatie (`/netwerk/kerngroep`). Op de site: de
menuoptie *Beheer* op je eigen profiel (alleen voor wie in een van die groepen
zit).

## Wat er kan

| | kerngroep | beheer |
|---|---|---|
| Leden zoeken en bekijken (account, rollen, organisatie, sessies, logboek) | ✓ | ✓ |
| Rollen contributor en member geven en afnemen | ✓ | ✓ |
| Rollen kerngroep, beheerder, redactie geven en afnemen | | ✓ |
| Account blokkeren en weer aanzetten, overal uitloggen | | ✓ |
| Organisaties bekijken (domeinen, leden) | ✓ | ✓ |
| Organisaties toevoegen, wijzigen, leden koppelen en ontkoppelen | | ✓ |
| Platform: versie, diensten, back-up, schijf, geheugen, uitrol | ✓ | ✓ |
| Logboek | ✓ | ✓ |

Beide groepen staan los van elkaar; wie in beide zit, mag wat beide mogen.
`/netwerk/gebruikers` (standaardgroep) en groepen buiten `/netwerk` en `/site`
zijn hier niet te wijzigen.

**Waarborgen**: elke wijziging vraagt een reden en komt in het logboek (wie,
wanneer, wat, waarom). De laatste actieve beheerder kan niet uit de
beheergroep en niet worden geblokkeerd; je blokkeert jezelf niet.

## Hoe het werkt

- **Een eigen dienst** (Go, geen afhankelijkheden van buiten), zoals
  Documenten. Niet in WordPress: dan zou de site (met al zijn plugins) een
  sleutel krijgen waarmee je Keycloak kunt wijzigen.
- **Inloggen** via Keycloak (client `beheer`, code flow met PKCE). Wie al op de
  site is ingelogd, merkt daar niets van (single sign-on). Sessies alleen in
  het geheugen: twee uur, een half uur zonder iets te doen; een herstart logt
  iedereen uit.
- **Wie wat mag**, wordt bij elk verzoek in Keycloak nagekeken (hooguit een
  minuut oud), niet alleen bij het inloggen. Haal je iemand uit de beheergroep,
  dan is hij er binnen een minuut uit, ook midden in een sessie.
- **Keycloak wijzigen** met het service account van de client
  `beheer-keycloak`: `view-users`, `query-users`, `query-groups`,
  `manage-users` en `manage-realm`. De dienst doet alleen de aanroepen in
  `keycloak.go`.
- **Platform**: de dienst kan niet bij Docker of de schijf (en hoort dat niet te
  kunnen). `server/status.sh` schrijft elke vijf minuten (systemd-timer
  `dg-status`) `/var/lib/dg-gemeenschap/status/status.json`; dat is
  alleen-lezen gekoppeld.
- **Logboek**: één JSON-regel per wijziging in het volume `beheerdata`
  (`/data/logboek.jsonl`). De back-up neemt het mee (`beheer-logboek.jsonl`);
  `herstellen.sh <map> beheer` voegt het weer samen.
- CSP zonder scripts, CSRF-sleutel per sessie, cookies `__Host-`,
  `SameSite=Lax`, `HttpOnly`; container alleen-lezen, zonder capabilities.

## Risico's (eerlijk)

- **`manage-realm`** is nodig voor de organisaties: Keycloak geeft de
  organisaties alleen met dat recht vrij, ook om ze te lezen (nagekeken op
  26.8.2; `view-realm` is niet genoeg). Daarmee kan het service account ook
  clients, inlogstromen en instellingen van de realm wijzigen. **Wie de
  container of het geheim `BEHEER_KEYCLOAK_SECRET` in handen krijgt, is in
  feite beheerder van de realm.** Met alleen `manage-users` (nodig voor rollen
  en blokkeren) kon dat al bijna: dan kun je iedereen beheerder maken en
  wachtwoorden zetten. Het geheim staat alleen in `.env` op de server en in de
  container; de dienst is niet van buiten bereikbaar behalve via Caddy.
  Afslanken kan later met de fijnmazige rechten van Keycloak (FGAP v2: alleen
  het lidmaatschap van bepaalde groepen), en de organisaties terug naar de
  console.
- **Geen MFA** in de eerste fase (besluit: de drempel is nu hoger dan het
  risico). Wie het wachtwoord van een beheerder heeft, komt hier binnen. Het
  logboek laat achteraf zien wat er gebeurde. Volgende stap: OTP of passkey
  verplicht voor `/site/beheer` en de kerngroep (een conditionele stap in de
  inlogstroom van Keycloak, of `acr_values` vanuit deze dienst).
- **Uitloggen** beëindigt de sessies in Keycloak. Een sessie op de site of in
  Redmine die al open staat, loopt door tot die zelf afloopt of ververst.
- Wijzigingen in de console van Keycloak staan niet in dit logboek.

## Later

- MFA voor beheer (zie boven), en bij gevoelige handelingen opnieuw inloggen
  (`max_age`).
- Uitnodigen en een mail voor een nieuw wachtwoord sturen
  (`execute-actions-email`).
- Leden van projecten (Redmine) en werkgroepen (BuddyPress) per lid laten zien.
