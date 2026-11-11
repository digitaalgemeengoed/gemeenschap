# Afbeeldingen: herkomst en licenties

Hier staat waar elke afbeelding van de site vandaan komt en onder welke
voorwaarden we hem mogen gebruiken. Voeg je een afbeelding toe aan de site
(`inhoud/media/` of het thema), zet hem dan ook in deze lijst.

Deze map (`ontwerp/`) is het archief: de originelen en de ontwerpen. De site
gebruikt hem niet zelf.

## Op de site

| Bestand | Wat | Herkomst | Licentie en voorwaarden |
|---|---|---|---|
| `wordpress/thema/digitaalgemeengoed/assets/uil.png` | logo (de uil) in de kop en de voet | het logo van DGG, aangeleverd door Erik Hoekstra; origineel in `ontwerp/logo/dgg-logo.png` | rechten bij de rechthebbende van het logo [IN TE VULLEN: wie, en onder welke voorwaarden] |
| `wordpress/thema/digitaalgemeengoed/assets/icoon.png` | sitepictogram (favicon) | uitsnede van het logo | als het logo |
| `keycloak/themes/digitaalgemeengoed/login/resources/img/uil.png` | logo op het aanmeldscherm | kopie van het logo | als het logo |
| `wordpress/thema/digitaalgemeengoed/assets/stad.svg`, `netwerk.svg` | stadsgezicht en netwerklijnen in de opening | eigen tekening, gemaakt voor deze repo | EUPL-1.2, zoals de repo |
| `wordpress/thema/digitaalgemeengoed/assets/iconen/*.svg` | iconen | eigen tekening, gemaakt voor deze repo | EUPL-1.2, zoals de repo |
| `inhoud/media/held-voorlopig.jpg` | foto in de opening van de startpagina | **voorlopig**: uitsnede van de AI-visualisatie `ontwerp/visualisaties/startpagina-2.png` | zie *Ontwerpvisualisaties*. Vervangen door een echte foto voordat de site openbaar wordt. |

## Ontwerpvisualisaties (`ontwerp/visualisaties/`)

Vier complete mock-ups van de site, met AI gemaakt [IN TE VULLEN: met welke
dienst] als ontwerprichting. Het zijn geen foto's: de mensen erop bestaan niet, en er
zitten geen losse fotobestanden in.

| Bestand | Oorspronkelijke naam | Wat |
|---|---|---|
| `startpagina-1.png` | `a_detailed_realistic_website_homepage_mockup_for.png` | startpagina met kaarten voor Common Ground, Crowdfunding, Open source, Kennisdeling en Implementatie |
| `dashboard.png` | `a_vibrant_dashboard_landing_page_ui_for_digitaal.png` | dashboard na inloggen; basis voor `[dg_dashboard]` |
| `startpagina-2.png` | `a_vibrant_modern_website_homepage_mockup_for_dig.png` | startpagina met kenmerken en actuele projecten; basis voor de huidige opmaak en voor `held-voorlopig.jpg` |
| `startpagina-3.png` | `digitaal_gemeengoed_samenwerken_aan_publieke_waard.png` | startpagina met "Waarom Digitaal GemeenGoed?" |

Gebruik: als ontwerp en ter illustratie. [IN TE VULLEN: in de voorwaarden
van die dienst nagaan of wij deze beelden op de openbare site mogen
gebruiken.] Omdat de mensen
erop niet bestaan, horen ze niet op de site als beeld van onze community.

## Foto's van Unsplash (nog niet in de repo)

Het HTML-prototype (de paginaset) laadde drie foto's rechtstreeks van
Unsplash; zie `unsplash-fotolinks.csv`. Ze zitten niet in de repo en de site
gebruikt ze niet.

| Referentie | Unsplash-id | Gebruikt in het prototype op |
|---|---|---|
| foto-samenwerking-1 | `photo-1521737711867-e3b97375f902` | startpagina |
| foto-samenwerking-2 | `photo-1556761175-b413da4baf72` | startpagina, aanmelden |
| foto-overleg-3 | `photo-1552664730-d307ca884978` | kennisbank, open source, implementatie |

**Unsplash License** (https://unsplash.com/license): gratis te gebruiken, ook
commercieel, kopiëren en aanpassen mag, naamsvermelding is niet verplicht
maar wordt gewaardeerd. Niet toegestaan: de foto's (vrijwel) ongewijzigd
verkopen, of verzamelen voor een concurrerende fotodienst. Bij herkenbare
personen of merken kunnen portret- of merkrechten gelden (zie de
[uitleg van Unsplash](https://help.unsplash.com/en/articles/2646379-what-if-there-s-a-brand-or-identifiable-person-depicted-in-an-image-that-i-download)).

Gaan we ze gebruiken, dan:
1. download het origineel via de pagina van de foto op unsplash.com;
2. zet het in `inhoud/media/` (niet rechtstreeks van Unsplash laden: dat
   stuurt het IP-adres van elke bezoeker naar een dienst buiten de EU);
3. vul hierboven onder *Op de site* een regel in, met de fotograaf en de link
   naar de pagina op Unsplash. [IN TE VULLEN: fotograaf per foto.]
