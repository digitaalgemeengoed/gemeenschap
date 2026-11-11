---
titel: Bijdragen
omschrijving: Aan de code, aan de site en aan de kennisbank, via GitHub
labels: algemeen, github, bijdragen
---

# Bijdragen

Alles wat het netwerk maakt, staat op [GitHub](https://github.com/digitaalgemeengoed), onder de EUPL 1.2: de software, de site en de inrichting van het platform.

## Aan de code

1. Kijk in Redmine of er al een melding is; maak er anders een.
2. Bij iets groters: overleg eerst in het forum *Development* of in de werkgroep *Developers & contributors*. Duur werk ontwerpen we eerst op papier.
3. Maak een **fork** of een **branch**, en een **pull request** naar `main`. Beschrijf wat je veranderde en waarom, en verwijs naar de melding.
4. Iemand anders bekijkt het (review). Pas daarna wordt het samengevoegd.

Afspraken:

- **Alleen testgegevens.** Nooit echte persoonsgegevens, ook niet in tests of schermafdrukken.
- **Geen geheimen** (wachtwoorden, sleutels, tokens) in de code of in een commit.
- **Nederlands** in de interface en de documentatie van de gemeenschap; code mag Engels of Nederlands, zolang een repo het consequent doet.
- Een nieuwe versie heeft een nummer (`v0.8.0`), een regel in de `CHANGELOG.md` en een release op GitHub.

## Aan de site

De pagina's van digitaalgemeengoed.nl staan als Markdown in de repo [gemeenschap](https://github.com/digitaalgemeengoed/gemeenschap/tree/main/inhoud). In WordPress zie je bij elke pagina de link **Bewerken op GitHub**.

1. Open het bestand op GitHub en klik op het potlood.
2. Wijzig de tekst. Met **Preview** zie je ongeveer hoe het wordt.
3. Kies **Create a new branch** en **Propose changes**. Zo komt er een pull request, en kijkt iemand mee.
4. Na samenvoegen staat het binnen vijf minuten op de site.

De opmaak (vakken, kaarten, knoppen) staat in `inhoud/README.md` in die repo.

## Aan de kennisbank

Gewoon hier, in de wiki: zie [Werken in de kennisbank](/algemeen/kennisbank). Wie liever in Markdown en Git werkt: de pagina's staan ook in de besloten repo `kennisbank`, en een wijziging daar staat binnen vijf minuten in de wiki.

## Aan de server

De inrichting van het platform (Keycloak, WordPress, Redmine, de wiki) staat ook in de repo `gemeenschap`. Een wijziging daarin gaat pas live bij een nieuwe versie, met eerst een back-up, en terug naar de vorige versie als het misgaat. Wie op `main` mag samenvoegen of een versie mag maken, bepaalt de kerngroep.
