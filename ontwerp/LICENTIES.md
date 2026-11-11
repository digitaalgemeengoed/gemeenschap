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
| `inhoud/media/projecten-mizuno-kozuki.jpg` | foto in de opening van *Projecten* | foto van **Mizuno Kozuki** via Unsplash, aangeleverd door Erik Hoekstra; middensectie uitgesneden (16:10) | Unsplash License, zie hieronder. [IN TE VULLEN: link naar de foto op unsplash.com] |
| `inhoud/media/photo-1556761175-b413da4baf72.avif` | foto in de opening van *Over ons* | foto van **Austin Distel** via Unsplash | Unsplash License, zie hieronder |
| `inhoud/media/photo-1552664730-d307ca884978.avif` | foto in de opening van *Het platform* | foto van **Annie Spratt** via Unsplash | Unsplash License, zie hieronder |
| `inhoud/media/community-ai.jpg` | foto in de opening van *Meedoen* (Community) | **AI-beeld**, gemaakt met ChatGPT (OpenAI) door Erik Hoekstra, november 2026; de mensen erop bestaan niet | volgens de voorwaarden van OpenAI is de uitvoer van wie hem liet maken. Vervangen door foto's van echte bijeenkomsten zodra die er zijn. |
| `inhoud/media/werkwijze-ai.jpg` | foto in de opening van *Werkwijze* | **AI-beeld**, gemaakt met ChatGPT (OpenAI) door Erik Hoekstra, november 2026; de mensen erop bestaan niet | als hierboven |
| `inhoud/media/crowdfunding-ai.jpg` | foto in de opening van *Crowdfunding* | **AI-beeld**, gemaakt met ChatGPT (OpenAI) door Erik Hoekstra, november 2026; de mensen erop bestaan niet. Er staan een EU-vlag en een scherm *Funding* op. | als hierboven. Let op: de EU-vlag kan suggereren dat de EU meebetaalt of het steunt; alleen gebruiken als dat zo is. |
| `inhoud/media/contact-ai.jpg` | foto in de opening van *Contact* | **AI-beeld**, gemaakt met ChatGPT (OpenAI) door Erik Hoekstra, november 2026; de mensen erop bestaan niet | als hierboven |
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

## Foto's van Unsplash (`inhoud/media/`)

Het HTML-prototype (de paginaset) laadde drie foto's rechtstreeks van
Unsplash; zie `unsplash-fotolinks.csv`. Ze staan nu in `inhoud/media/`, zodat
de site ze zelf aanbiedt (rechtstreeks van Unsplash laden stuurt het
IP-adres van elke bezoeker naar een dienst buiten de EU). *Over ons* en *Het
platform* gebruiken de foto's van Austin Distel en Annie Spratt (zie *Op de
site*); foto-samenwerking-1 wordt nog niet gebruikt.

| Bestand in `inhoud/media/` | Fotograaf | Gebruikt in het prototype op |
|---|---|---|
| `photo-1521737711867-e3b97375f902.avif` (foto-samenwerking-1) | onbekend: niet achterhaald | startpagina |
| `photo-1556761175-b413da4baf72.avif` (foto-samenwerking-2) | Austin Distel | startpagina, aanmelden |
| `photo-1552664730-d307ca884978.avif` (foto-overleg-3) | Annie Spratt | kennisbank, open source, implementatie |

**Unsplash License** (https://unsplash.com/license): gratis te gebruiken, ook
commercieel, kopiëren en aanpassen mag, naamsvermelding is niet verplicht
maar wordt gewaardeerd. Niet toegestaan: de foto's (vrijwel) ongewijzigd
verkopen, of verzamelen voor een concurrerende fotodienst. Bij herkenbare
personen of merken kunnen portret- of merkrechten gelden (zie de
[uitleg van Unsplash](https://help.unsplash.com/en/articles/2646379-what-if-there-s-a-brand-or-identifiable-person-depicted-in-an-image-that-i-download)).

Gebruik je er een op een pagina, vermeld dan de fotograaf, bijvoorbeeld onder
de foto: *Foto: Annie Spratt via Unsplash*. Van foto-samenwerking-1 is de
fotograaf niet bekend en is dus ook niet na te gaan of de foto echt onder de
Unsplash License valt; gebruik die pas als dat wel bekend is, of kies een
andere foto.
