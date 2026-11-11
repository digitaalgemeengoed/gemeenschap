# Wijzigingen

## 0.11.0 — 2026-11-11

**Documenten: werkgroepen, Mijn documenten, quota en beheer.**
- **Werkgroepen** krijgen elk een ruimte (`werkgroep-<slug>`). De leden komen elke vijf minuten uit de groepen van BuddyPress op de site: lid Bewerken, beheerder of moderator van de groep Beheren, de kerngroep Beheren.
- **Mijn documenten**: een persoonlijke ruimte voor wie in de kerngroep, een werkgroep of een project zit, aangemaakt bij het eerste gebruik, met een quotum van 1 GB (`DOC_QUOTUM_PERSOONLIJK_MB`). Alleen de eigenaar kan erin, ook de beheerders van het platform niet; hij kan wel delen (bij de ontvanger: *Gedeeld met jou → Van <naam>*). Is de eigenaar nergens meer lid, dan wordt de ruimte **bevroren**: 90 dagen alleen-lezen, met een bericht; daarna gaat alles naar de prullenbak. Wordt hij weer lid, dan gaat hij weer open. Wie in de kerngroep zit, komt rechtstreeks uit Keycloak (client `redmine-sync`). De levenscyclus draait alleen als Keycloak, Redmine en WordPress alle drie antwoordden.
- **Eén bucket, een voorvoegsel per ruimte**: nieuwe bestanden heten `objecten/r<ruimte>/<uuid>`.
- **Beheer** (`/beheer`, voor de kerngroep en `/site/beheer`): alle ruimtes met omvang, aantal documenten, status en quotum (in te stellen); per ruimte de prullenbak (terugzetten, definitief verwijderen: ook uit de opslag) en het logboek. Van persoonlijke ruimtes alleen de omvang; hun prullenbak is voor de eigenaar.
- Navigatie in Documenten: Persoonlijk, Coöperatie, Werkgroepen, Projecten en Gedeeld met jou.
- **Op de site**: een tab *Documenten* bij elke werkgroep (voor de leden), *Mijn documenten* op je eigen profiel, en de shortcode `[dg_documenten project=…]` voor de projectpagina's.

## 0.10.0 — 2026-11-11

**Documenten: een verkenner, rechten per map en document, delen, WebDAV en openen in Word of LibreOffice.**
- Het scherm is nu een verkenner in de stijl van de XENA-demo: een opdrachtbalk (Nieuwe map, Uploaden, Downloaden, Naam wijzigen, Verwijderen, Toegang, Eigenschappen), een adresbalk, links de ruimtes met de mappenboom, in het midden alleen de inhoud van de gekozen map, rechts de eigenschappen of de toegang, onderaan een statusbalk. Bestanden sleep je op de lijst; een rij aanklikken kiest het document.
- **Rechten** in drie niveaus, zoals in de XENA-demo: Lezen, Bewerken, Beheren. Per persoon, per groep uit Keycloak of voor de projectleden, op een ruimte of een map. Een map neemt de toegang van boven over; met *Overerving stoppen* wordt hij besloten. Wie Beheren heeft, kan toegang doorgeven, ook Beheren.
- Standaard: de kerngroep Beheren in elke ruimte; in een project de projectleden volgens hun rol in Redmine (Kerngroep en Manager Beheren, Contributor en Member Bewerken). `/site/beheer` kan altijd overal bij.
- Rechten ook op **losse documenten**, met dezelfde overerving en *Overerving stoppen*.
- **Delen** van een map of document met een persoon, groep of de projectleden, met Lezen (standaard) of Bewerken. De ontvanger krijgt een bericht met de link via de berichten van de site (en de e-mailmelding daarvan); bij een groep elk lid een eigen bericht. Delen vraagt Beheren. De link kopiëren of doorsturen (LinkedIn, Bluesky, X, WhatsApp, e-mail, *Delen via…*) kan iedereen die het kan lezen; de link werkt alleen voor wie toegang heeft.
- WordPress: het eindpunt `/wp-json/dg/v1/bericht` (alleen met het geheim `DOC_BERICHT_GEHEIM`, en Caddy laat het niet van buiten toe).
- Verwijderen vraagt nu Beheren (was: iedereen die mocht schrijven).
- **Openen in Word / Excel / PowerPoint / LibreOffice** bij een document: het programma opent het rechtstreeks (via een tijdelijke link van tien uur, alleen voor dat document) en slaat ook rechtstreeks op. Wie alleen mag lezen, krijgt het alleen-lezen.
- **WebDAV** op `/dav/`: Documenten als netwerkschijf in Verkenner, Finder, Bestanden of rclone, met dezelfde ruimtes en rechten. Inloggen met je e-mailadres en een **app-wachtwoord** (pagina *Netwerkschijf koppelen*; één per apparaat, in te trekken).
  - Elke keer opslaan is een versie; slaat dezelfde persoon binnen tien minuten opnieuw op, dan vervangt dat de laatste versie (Word en LibreOffice slaan tussentijds op).
  - "Veilig opslaan" (tijdelijk bestand, origineel weg, tijdelijk bestand hernoemen) houdt de geschiedenis bij één document.
  - Bestanden die systemen zelf neerzetten (`._*`, `.DS_Store`, `Thumbs.db`, `desktop.ini`, `~$*`, `.~lock.*`) komen niet in het register.
  - Vergrendelen (LOCK) zoals Word en LibreOffice dat doen. Getest met litmus: basis, kopiëren en verplaatsen, eigenschappen en vergrendelen werken, op twee bekende beperkingen van de Go-bibliotheek na (geen gedeelde vergrendeling, strenge XML-controle).
  - Caddy laat `/dav/` en `/o/` buiten de afscherming met wachtwoord: die loggen zelf in.

**Hersteld:**
- In 0.9 kreeg iedereen die zich aanmeldde toegang tot elk project: in Redmine is de groep *Netwerk: Gebruikers* lid van elk project. Nu telt de rol: Gebruiker geeft geen toegang.
- De gegevens van een document bewaren zonder trefwoorden gaf een fout.

## 0.9.1 — 2026-11-11

**Hersteld:** 0.8.1 en 0.9.0 rolden niet uit. Het inrichten van de kennisbank stopte bij het bijwerken van de startpagina: `EACCES: permission denied, open '/wiki/data/repo/home.md'`. De volumes voor de Git-werkkopie en de uploads van de wiki waren van root, terwijl de wiki als `node` draait. Daardoor werkte ook de Git-opslag naar de repo `kennisbank` sinds 0.8.0 niet (de fout kwam alleen in het log van de wiki).
- `bijwerken.sh` zet de eigenaar van `/wiki/data/repo` en `/wiki/data/content` goed, vóór het inrichten.
- De image maakt die mappen zelf aan, zodat een nieuw volume meteen van `node` is.

Verder alles van 0.8.1 en 0.9.0 (hieronder): de startinhoud van de kennisbank, de opening op de hoofdpagina's, terugzetten en herstellen, en Documenten.

## 0.9.0 — 2026-11-11

**Documenten** op `documenten.<domein>` (nieuw): documenten bewaren en delen, voor de coöperatie en per project. Een lichte XENA: een register naast een objectopslag. Ontwerp in `documenten/ONTWERP.md`.
- Een ruimte *Coöperatie* (members lezen, de kerngroep schrijft) en een ruimte per project in Redmine (wie lid is van het project); beheerders mogen alles. Projecten en leden komen elke vijf minuten uit Redmine; een gesloten project is alleen-lezen.
- Mappen, uploaden (slepen, meer bestanden tegelijk, hooguit 500 MB per bestand), downloaden, hernoemen, verwijderen naar de prullenbak.
- Elk document houdt zijn versies: een bestand met dezelfde naam wordt een nieuwe versie; oude versies blijven te downloaden. Bij elke versie een SHA-256-controlegetal.
- Lichte metadata: titel, omschrijving, soort document, trefwoorden; auteur en datums vanzelf. Een logboek van elke handeling.
- Inloggen via Keycloak (client `documenten`).
- **Garage** (v2.5) als S3-opslag, één node, alleen intern bereikbaar. MinIO valt af: als open source gestopt.
- **dg-documenten**: een kleine dienst in Go, zonder afhankelijkheden buiten de Postgres-driver (meegeleverd in `vendor/`).
- Back-up: het register per nacht, de objecten in een spiegel die alleen aangroeit. `herstellen.sh documenten` zet het register terug en vult ontbrekende objecten aan uit de spiegel.
- Op het dashboard een tegel *Documenten*.

**Controle na een uitrol:** de kennisbank en de documenten tellen pas mee als hun naam in de DNS naar de server wijst. Zonder DNS-record heeft Caddy geen certificaat, en dan zou elke nieuwe versie onterecht teruggedraaid worden. `controleer.sh` meldt welke namen nog ontbreken.

Na het uitrollen: een A-record voor `documenten.<domein>` naar de server.

## 0.8.1 — 2026-11-11

- **Startinhoud** (29 pagina's, `wiki/startinhoud/`): een algemene sectie over hoe Digitaal GemeenGoed werkt (werkwijze, platform, lid worden en rollen, meldingen, bijdragen, crowdfunding, werken in de kennisbank, privacy en beveiliging, veelgestelde vragen), een pagina per project (de Office Assistent uitgebreid, met het prototype), en een pagina per demo (XENA, MIRA, TINA, ELSA, MAIA, VERA, IRIS, LISA) met de weergaven, uitgangspunten, vragen en begrippen uit de demo zelf. Alleen pagina's die nog niet bestaan; wat leden wijzigen, blijft. Plus een navigatie links.
- De kortere startpagina van 0.8.0 wordt vervangen door de nieuwe, maar alleen als niemand haar heeft gewijzigd.
- De pagina *Het platform* op de site wijst naar de kennisbank en de architectuurmodellen.

**Terugzetten en herstellen** (nieuw, `server/`):
- `terugzetten.sh [versie]`: terug naar een eerdere versie van de infrastructuur, met een back-up vooraf en de controle achteraf; lukt het niet, dan staat de huidige versie weer. Nieuwere versies worden gemarkeerd als *niet opnieuw proberen*, zodat de automatische uitrol ze niet meteen weer neerzet.
- `herstellen.sh [--proef] [map] [onderdelen]`: een back-up terugzetten per onderdeel (Keycloak, WordPress, Redmine, kennisbank). Controleert eerst of de bestanden heel zijn, maakt een back-up van nu, pauzeert de uitrol en vraagt om bevestiging. Met `--proef` zie je alleen wat er zou gebeuren.
- De gezondheidscontrole (`gezond`) staat nu in `lib.sh`, voor `uitrol.sh` en `terugzetten.sh` samen.

**Opmaak:** de hoofdpagina's (Over ons, Werkwijze, Projecten, Het platform, Meedoen, Crowdfunding, Contact) beginnen nu met een opening in de stijl van de startpagina: titel, lint, inleiding, knoppen, stadsgezicht en (waar er een is) een foto. Op zo'n pagina vervalt de titelbalk van BuddyX met het kruimelpad. De opening is daar wat lager dan op de startpagina. Beelden: *Projecten* (foto Mizuno Kozuki), *Over ons* (Austin Distel), *Het platform* (Annie Spratt); *Werkwijze*, *Meedoen*, *Crowdfunding* en *Contact* met een gegenereerde afbeelding (ChatGPT). Herkomst en licenties in `ontwerp/LICENTIES.md`. De knoppen in een opening blijven nu zichtbaar voor wie is ingelogd, behalve op de startpagina.

**Teksten:** het adres info@digitaalgemeengoed.nl staat nu op Contact (ook voor het melden van beveiligingsproblemen), Privacy en Voorwaarden.

## 0.8.0 — 2026-11-11

**Kennisbank** op `kennis.<domein>`: Wiki.js 2, alleen voor leden.
- Inloggen via Keycloak; de groepen komen bij elke login mee (`/netwerk/gebruikers` lezen en schrijven, `/site/redactie` ook verplaatsen en verwijderen, `/site/beheer` alles). Gasten zien niets en gaan meteen naar het inloggen.
- Ingericht als code (`wiki/inrichten.py`, bij elke versie): installatie, titel, logo en kleuren van de site, groepen, Nederlands, een startpagina met spelregels.
- Draait offline: de Nederlandse en Engelse teksten zitten in de image (`wiki/Dockerfile`).
- **Pagina's ook in GitHub**, in beide richtingen, zodra de repo `kennisbank` en de deploy key er zijn (zie README, Kennisbank).
- In het menu en op het dashboard wijst *Kennisbank* nu naar de wiki; in Redmine staat de wiki per project uit (een projectwiki waarin al iets staat, blijft).

**Architectuurmodellen** op `architectuur.<domein>`, alleen voor leden.
- Eén repo per project of domein (coArchi); de lijst staat in `architectuur/repos.txt` en werkt na samenvoegen op `main`, zonder versie.
- `server/architectuur.sh` (systemd-timer, elk kwartier) bouwt het HTML-rapport met Archi (archimate-ci-image, zonder netwerk) als het model is gewijzigd, en maakt een overzichtspagina. Een mislukt rapport laat het vorige staan.
- Inloggen via Keycloak met oauth2-proxy voor Caddy.

Verder: nieuwe Keycloak-clients `wiki` en `architectuur`; back-up van de kennisbank; `controleer.sh` en de controle bij het uitrollen kijken ook naar de kennisbank; de privacyverklaring (concept) noemt de kennisbank en GitHub.

## 0.7.5 — 2026-11-11

- Oplossing: na de bevestigingsmail bij het aanmelden kwam je op een foutpagina (*Invalid state*). Een inlogverzoek bleef maar drie minuten geldig; nu een half uur. En komt WordPress toch een verzoek tegen dat het niet meer kent, dan begint het één keer opnieuw: wie al bij Keycloak is ingelogd, is dan meteen ingelogd.

## 0.7.4 — 2026-11-11

- De opening staat op een breed scherm in het midden: de foto en de netwerklijnen eindigen bij de rechterkant van de inhoud, met een zachte rand, in plaats van door te lopen tot de rand van het scherm. Daardoor leek de pagina rechts uitgelijnd.

## 0.7.3 — 2026-11-11

- Oplossing: mail via poort **465** (meteen versleuteld, zoals bij Migadu) werkte niet in Keycloak en Redmine; die gingen altijd uit van STARTTLS op 587. Nu kiezen Keycloak (bij `bijwerken.sh`) en Redmine zelf: poort 465 met SSL, andere poorten met STARTTLS. WordPress deed dit al goed.

## 0.7.2 — 2026-11-11

Opmaak van de startpagina op brede schermen:
- Eén inhoudsbreedte (1380 px) voor kop, opening en pagina; de tekst van de opening, het logo en de rest beginnen op dezelfde lijn. Alleen de achtergrond van de opening loopt over de hele breedte.
- Het hele hoofdmenu past op één regel (geen *More* meer vanaf 1280 px): de punten staan iets dichter op elkaar en de ondertitel naast het logo verschijnt pas vanaf 1700 px.
- Geen zwarte WordPress-werkbalk meer voor bezoekers en gewone leden; alleen voor wie pagina's of berichten bewerkt.
- Actuele projecten: drie kaarten per rij die samen de hele breedte vullen.

## 0.7.1 — 2026-11-11

- **Afschermen met een wachtwoord** zolang het platform nog niet openbaar is: `sudo /opt/gemeenschap/server/afschermen.sh aan` vraagt een gebruikersnaam en wachtwoord en zet de site, `aanmelden.` en `projecten.` erachter (HTTP Basic Auth in Caddy). In `.env` staat alleen een hash. Verkeer van de server zelf en uit de containers hoeft niet in te loggen, zodat de controles, wp-cron en het inloggen via Keycloak blijven werken. `afschermen.sh uit` maakt het weer openbaar.
- **Geen zijbalk** meer op pagina's, fora en ledenlijsten: BuddyX toonde op een nieuwe installatie de widgets *Recent Posts* en *Recent Comments* naast de opening, waardoor de startpagina werd samengedrukt.
- Oplossing: de image van keycloak-config-cli heet `adorsys/keycloak-config-cli:6.5.1-26` (versie plus de Keycloak-reeks); `6.5.1` bestaat niet, waardoor de eerste installatie stopte bij het inrichten van Keycloak.
- Oplossing: de plugin voor inloggen via Keycloak heet op wordpress.org `daggerhart-openid-connect-generic`; met de oude naam stopte het inrichten van WordPress.
- De tabel voor toezeggingen wordt niet meer aangemaakt voordat WordPress zelf geïnstalleerd is (een foutmelding bij de eerste installatie, zonder gevolgen).
- Oplossing: wp-cli kreeg de PHP-instellingen van de site niet mee (geheugen 128 MB in plaats van 512 MB), waardoor CiviCRM installeren vastliep.
- Oplossing: Redmine startte steeds opnieuw. De image bewaart nu `Gemfile.lock.postgresql` met de gems van de OAuth-plugin, en het entrypoint start weer als root (zoals in de officiële image) en gaat daarna zelf over op de gebruiker redmine.
- Oplossing: Redmine inrichten (`rails runner`) kreeg de geheime sleutel niet mee; `SECRET_KEY_BASE` staat nu in de omgeving van de container, en het inrichten draait als de gebruiker redmine.
- Oplossing: de tekst op de inlogknop van Redmine was te lang (maximaal 40 tekens); nu *Inloggen met je DGG-account*, in het blauw van de site.
- De testschermafdrukken in `gem/` zijn weg.

## 0.7.0 — 2026-11-11

**Toezeggingen voor crowdfunding per gemeente** (`wordpress/dg-crowdfunding.php`):
- Een ingelogd lid dat via zijn e-maildomein aan een gemeente gekoppeld is, geeft op de site een **intentie** op namens die gemeente: bedrag (veelvoud van € 500), wie mag tekenen (naam en functie), en eventueel de voorwaarde dat het streefbedrag gehaald wordt. Hij vinkt aan dat de intentie niet bindend is en dat er een samenwerkingsovereenkomst volgt. Eén toezegging per gemeente per campagne; collega's kunnen hem wijzigen of intrekken.
- De kerngroep zet een intentie in WordPress onder **Toezeggingen** op *bevestigd* als de overeenkomst getekend is (met kenmerk en datum), of op *vervallen*. Een bevestigde toezegging kan het lid niet meer wijzigen.
- Op de kaart ziet een bezoeker alleen het streefbedrag en het totaal dat is toegezegd. Een ingelogd lid ziet ook welke gemeenten meedoen, met een balk voor bevestigd (vol) en intentie (gestreept); het bedrag per gemeente alleen als het bevestigd is.
- Elke wijziging komt als activiteit bij de gemeente in CiviCRM (als CiviCRM draait) en per mail bij de beheerder; wie toezegt, krijgt een bevestiging.
- In Markdown: `campagne:` en `einddatum:` op een kaart. Zie `inhoud/README.md`.
- `wp dg toezeggingen` toont de stand.
- Privacyverklaring (concept): de gegevens van toezeggingen erbij.

Ook: **`ontwerp/`** met het logo, de vier ontwerpvisualisaties en de links naar de Unsplash-foto's uit het prototype, en `ontwerp/LICENTIES.md` met de herkomst en licentie van elke afbeelding.

## 0.6.0 — 2026-11-11

Naar aanleiding van de paginaset (prototype) en met een duidelijke verwijzing naar de VNG en Common Ground:
- **Common Ground**: een eigen pagina (onder Over DGG) en een blok op de startpagina, met links naar het programma Common Ground bij de VNG en naar commonground.nl; ook in de voet en op Over ons.
- **Werkgroepen** als BuddyPress-groepen, elk met een eigen forum: Common Ground, Open Overheid, Records Management, Projectportfolio, Developers & contributors, en Implementatie & beheer. Groepen aanmaken kan alleen een beheerder.
- **Projectpagina's** voor Anonimiseren bij de bron, Open overheid en Datafundament; elke projectpagina heeft nu een feitenblok (fase uit Redmine, werkwijze, financiering) en "Hoe je bijdraagt". Redmine krijgt die drie projecten erbij.
- **Crowdfunding**: kaarten met een voortgangsbalk (`doel:`, `toegezegd:`, `deelnemers:`).
- **Voet** in drie kolommen (Ontdekken, Deelnemen, Goed om te weten) met het logo en een korte omschrijving.
- Startpagina voor gasten: "Niet telkens opnieuw beginnen. Samen verder bouwen." met vinkjes, en de balk "Goede software maken we samen."
- Pagina's **Open source** en **Implementatie** onder Kennisbank.
- Oplossing: de tabellen van de BuddyPress-onderdelen (groepen, activiteit, berichten, meldingen) worden nu echt aangemaakt; alleen de instelling zetten deed dat niet.

## 0.5.0 — 2026-11-11

**Dashboard voor leden** op de startpagina, naar het ontwerp van Erik (shortcode `[dg_dashboard]`, `wordpress/dg-dashboard.php`):
- links: profiel met foto en gemeente, en navigatie (dashboard, community, fora, kennisbank, projecten, samenwerkingen, evenementen, berichten met het aantal ongelezen, instellingen in Keycloak);
- midden: welkom, **projectupdates** (het nieuws van de openbare projecten in Redmine, met de fase van het project), **recente discussies** (de fora), en oproepen voor de kennisbank en om mee te doen;
- rechts: **samenwerkingen** (uit `inhoud/paginas/samenwerkingen.md`, pas zichtbaar als die gepubliceerd is), **actieve leden** (BuddyPress) en **aankomende evenementen** (CiviCRM, met het aantal aanmeldingen).
- Wie is ingelogd, ziet onder de opening het dashboard in plaats van de pagina voor gasten.
- Redmine: het projectveld **Fase** (verkenning, ontwikkeling, pilot, beheer) en de projecten XENA, E-mailarchivering en Object-native werkplek.
- Nieuwe pagina's: Samenwerkingen (concept), Crowdfunding en Contact; in het hoofdmenu Kennisbank, Crowdfunding en Contact.

## 0.4.0 — 2026-11-11

**De opmaak van het ontwerp** (schets van Erik, 11 november):
- Kop met de uil, de naam op twee regels, de ondertitel *Community voor open source en Common Ground in de overheid*, een huisje voor Home, uitklapmenu's (Over DGG, Projecten, Community, Kennisbank) en een knop **Aanmelden**.
- Opening over de hele breedte: grote kop, een blauw en een groen lint, knoppen, een stadsgezicht (eigen lijntekening) en rechts een foto met netwerklijnen. De foto is voorlopig: een uitsnede van de schets (`inhoud/media/held-voorlopig.jpg`).
- Een strook met vijf kenmerken, kaarten voor de actuele projecten met label en pijl, en een oproep *Word onderdeel van DGG*.
- Nieuwe pagina's: XENA – Open DMS/RMA, E-mailarchivering en Object-native werkplek, onder Projecten.
- In Markdown: vakken `kenmerken` en `projecten`, en regels `icoon:`, `label:`, `link:`, `beeld:` en `soort:` in kaarten. Menu's met uitklapmenu's (inspringen). Zestien iconen in het thema.
- Kleuren: marineblauw, blauw en groen, alle tekst volgens WCAG AA.

## 0.3.0 — 2026-11-11

- **De site op basis van de introductiepresentatie** (mei 2025): start (*Samen grenzeloos sterk*), Over ons (de uitdaging, naast wat er al is, organisatie), Werkwijze (vier stappen), Projecten (vijf projecten uit het portfolio van Haarlem en Zandvoort, met de Office Assistent eronder), Het platform (zes onderdelen, met wat er al is en wat nog komt) en Meedoen (als persoon en als gemeente).
- **Vakken in Markdown**: `::: held`, `kaarten`, `stappen` en `kader`, en knoppen met `[[Tekst]](adres)`. Zie `inhoud/README.md`.
- De naam is overal **Digitaal GemeenGoed**, zoals in de presentatie; de ondertitel van de site is *Samen grenzeloos sterk*.

## 0.2.0 — 2026-11-11

- **Thema BuddyX** (5.2.0, wordpress.org) met een kindthema voor Digitaal Gemeengoed: het logo (de uil), kleuren uit het logo (blauw `#0066cc`, donker `#003c7a`; contrast voldoet aan WCAG AA), en een voet met de links uit `inhoud/menu.md`. Leden als kaarten, fora en groepen in de stijl van een gemeenschap, donkere modus. Vervangt het kindthema van Twenty Twenty-Five.
- Logo en sitepictogram komen uit het thema (`assets/`) en worden bij het inrichten in de mediabibliotheek gezet.
- Het aanmeldscherm van Keycloak in dezelfde kleuren, met de uil.
- De menu's uit `inhoud/menu.md` zijn nu gewone WordPress-menu's; het hoofdmenu staat op de plek *Primary* van het thema.
- **Geen Gravatar**: profielfoto's worden niet meer bij Gravatar opgevraagd (dat stuurt een versleuteld e-mailadres van elk lid naar een dienst buiten de EU). Wie geen eigen foto heeft, krijgt het standaardplaatje.

## 0.1.0 — 2026-11-11

De eerste versie, overgenomen uit oa-uitrol 0.21.0 en uitgebreid:

- **Uitrol vanuit GitHub.** De server haalt elke vijf minuten op wat er nieuw is: `inhoud/` na samenvoegen op `main`, de rest bij een versie-tag. Eerst een back-up; mislukt de controle, dan terug naar de vorige versie. Melding per mail als `UITROL_MELDEN` is ingesteld.
- **Pagina's en menu's als Markdown** in `inhoud/`, in WordPress op slot met een link *Bewerken op GitHub*. Eerste pagina's: start, over het netwerk, Office Assistent, meedoen, en voorwaarden en privacyverklaring (concept, nog juridisch te toetsen).
- **Keycloak-instellingen uit de repo**, bij elke versie gelijkgetrokken met keycloak-config-cli 6.5.1. Leden en organisaties blijven staan. De mailinstellingen komen nu uit `.env`.
- **Thema**: Twenty Twenty-Five met eigen kleuren, kop en voet. BuddyPress op `/leden/`, `/groepen/`, `/activiteit/`.
