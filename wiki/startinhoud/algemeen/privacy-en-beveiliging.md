---
titel: Privacy en beveiliging
omschrijving: Welke gegevens waar staan, wie ze ziet, en wat je niet deelt
labels: algemeen, privacy, avg, bio
---

# Privacy en beveiliging

Digitaal GemeenGoed en NL-Dit zijn samen verantwoordelijk voor de persoonsgegevens op het platform (AVG artikel 26). De volledige uitleg komt in de [privacyverklaring](https://digitaalgemeengoed.nl/privacy/); die is nog een concept en wordt juridisch getoetst.

## Wat waar staat

| Gegevens | Waar | Wie ziet het |
|---|---|---|
| naam, e-mailadres, wachtwoord, tweede factor | Keycloak (je account) | alleen jij en de beheerders; het wachtwoord niemand |
| organisatie, functie, werkveld | Keycloak, overgenomen in het ledenprofiel | ingelogde leden |
| toestemming voor de nieuwsbrief | Keycloak en CiviCRM, met datum | de beheerders |
| berichten in de fora | WordPress | ingelogde leden |
| meldingen en reacties | Redmine | **iedereen**, met je naam |
| pagina's en opmerkingen in de kennisbank | Wiki.js, en de besloten repo op GitHub | ingelogde leden; in GitHub ook je e-mailadres |
| toezeggingen voor crowdfunding | WordPress, CiviCRM | zie [Crowdfunding](/algemeen/crowdfunding) |

Het platform gebruikt geen trackers en geen advertenties, en vraagt geen profielfoto's op bij Gravatar.

## Wat je niet deelt

- **Persoonsgegevens** van anderen: niet in de fora, niet in Redmine, niet in de kennisbank, niet in schermafdrukken. Gebruik testgegevens.
- **Wachtwoorden, sleutels en tokens**, ook niet "even" in een bericht.
- **De technische inrichting van een gemeente**: netwerkplaten, beveiligingsmaatregelen, kwetsbaarheden, leveranciersafspraken. Dat is informatie die volgens de BIO beschermd moet worden.

## Een fout of een lek gezien?

- Een kwetsbaarheid in de software: meld haar **niet** in Redmine (dat is openbaar), maar rechtstreeks bij de kerngroep. [In te vullen: het adres voor beveiligingsmeldingen.]
- Persoonsgegevens op een plek waar ze niet horen: meld het bij de kerngroep, dan halen we ze weg.

## Beveiliging van je account

- Kies een sterk wachtwoord dat je nergens anders gebruikt.
- Zet een tweede factor aan via *Mijn account*.
- Na herhaaldelijk mislukte pogingen wordt inloggen tijdelijk geblokkeerd.
