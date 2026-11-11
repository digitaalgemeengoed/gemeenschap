# Wijzigingen

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
