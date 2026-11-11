---
titel: MIRA — Office Assistent
omschrijving: Modular Information & Records Assistant: de assistent in het kantoorpakket die zorgt voor kwaliteit aan de bron.
labels: demo, office-assistent, mira
---


# MIRA — Office Assistent

**Modular Information & Records Assistant**, de Office Assistent uit XENA Deel II.

Een interactieve demo van een assistentpaneel in kantoorsoftware dat zorgt voor kwaliteit aan de bron: metadateren, toegankelijkheid, persoonsgegevens, classificeren, waarmerken en workflow, voor tekstdocumenten, rekenbladen, presentaties en e-mail. Hetzelfde paneel werkt in verschillende kantoorpakketten, en geneste onderdelen (een ingesloten tabel, een gekoppeld bereik, een bijlage) houden hun herkomst.

> **Een demo, geen product.** Alle voorbeeldgegevens zijn verzonnen: personen, dossiers en zaaknummers bestaan niet. Een deel van wat de demo toont is toekomstbeeld; de schakelaar *Tijdvak* laat zien wat vandaag al kan.

De demo hoort bij *XENA Deel II, De Office Assistent*, de architectuurvisie waarop zij is gebaseerd. Zij is de tegenhanger van de demo [Archiefwaardige opslag voor de medewerker](https://github.com/EHa-1999/XENA).

**[Open de demo](https://eha-1999.github.io/MIRA/) · [Broncode en uitleg op GitHub](https://github.com/EHa-1999/MIRA)**

## Wat de demo laat zien

| Weergave | Wat je ziet |
|---|---|
| **Tekst** | Een raadsvoorstel in de tekstverwerker, met het paneel van de assistent ernaast. Het meest uitgewerkte stuk; begin hier. |
| **Rekenblad** | De bron van de cijfers. Sla een nieuwe versie op en zie onder *Gebruikt in* welke stukken achterlopen. |
| **Presentatie** | Drie dia's met sprekersnotities. Het paneel volgt de dia die in beeld is. |
| **E-mail** | Een bericht met twee bijlagen. Bij het verzenden stelt de assistent één vraag per bijlage. |
| **Architectuur** | De vier omgevingen, het contract ertussen en per handeling de stappen. |
| **Techniek** | Per functie wat zij vraagt en van wie: de werkomgeving, de leverancier of het platform. |
| **Groeipad** | In vier stappen invoeren, waarvan drie zonder op de markt te wachten. |
| **Help** | De weergaven, een route om te proberen, veelgestelde vragen en begrippen. |

Voor de documentweergaven gelden drie schakelaars: **Tijdvak** (huidige praktijk, vandaag met de assistent, later), **Werkomgeving** (drie kantoorpakketten of mailomgevingen) en **Bekijk als** (opsteller, of een inwoner of ontvanger). De balk onderaan, *Onder de motorkap*, toont bij elke handeling welke aanroep of gebeurtenis erbij hoort.

## Veelgestelde vragen

### Algemeen

**Is dit een product?**  
Nee. Het is een demo van een architectuur. Alle gegevens zijn verzonnen, en een deel van wat je ziet is toekomstbeeld. De schakelaar Tijdvak laat zien wat vandaag al kan.

**Vervangt de assistent het zaaksysteem of het documentbeheer?**  
Nee. De assistent draagt een stuk over en registreert niet zelf. Zij bewaart ook niets buiten het platform: er ontstaat geen tweede archief.

**Wat moet de medewerker extra doen?**  
Zo weinig mogelijk. De assistent leidt af wat af te leiden is, stelt voor wat zij kan voorstellen, en vraagt alleen wat alleen de medewerker weet, op het moment dat zij het antwoord nog weet.

### Onderdelen

**Wat is het verschil tussen ingesloten en gekoppeld?**  
Een ingesloten onderdeel is een kopie in het stuk. Een gekoppeld onderdeel is een eigen object waarnaar het stuk verwijst. Met de assistent krijgt ook een kopie een vastgelegde herkomst: bron, versie en moment.

**Wat gebeurt er als de bron wijzigt nadat het stuk is vastgesteld?**  
Aan het stuk verandert niets. Een vastgesteld stuk blijft tonen wat is vastgesteld. Wel wordt zichtbaar dat de bron inmiddels een nieuwere versie heeft.

**Waarom stelt de assistent een vraag bij het vaststellen?**  
Als de bron van een onderdeel korter wordt bewaard dan het stuk, moet iemand kiezen: de waarde overnemen, of aanvaarden dat er later een leemte ontstaat. Dat is het moment waarop iemand het antwoord nog weet.

### Modules

**Beslist de assistent wat er wordt gelakt?**  
Nee. Het programma signaleert en stelt voor. Een medewerker beslist per passage, en of iets openbaar wordt is een besluit van een bevoegde functionaris.

**Wat als een controle niet werkt?**  
Dan meldt het paneel dat er niet is gecontroleerd. Dat is iets anders dan dat er niets is gevonden. Het werk wordt er niet door tegengehouden.

**Waarom blijft een waarmerk geldig als de bron wijzigt?**  
Het waarmerk dekt het stuk en de versies van de onderdelen zoals ze zijn vastgezet. Een nieuwe versie van de bron verandert daar niets aan.

### Werkomgevingen

**Werkt dit alleen in Word?**  
Nee. De modules praten met een contract, niet met een kantoorpakket. Per werkomgeving is er een dunne adapter die dat contract invult.

**Waarom gedraagt Collabora zich soms anders dan Word?**  
Werkomgevingen kunnen niet allemaal hetzelfde. Waar iets ontbreekt, bereikt de assistent hetzelfde op een andere manier, bijvoorbeeld door het document opnieuw te laden, en zegt zij dat.

**Wat als het platform niet bereikbaar is?**  
Dan biedt de assistent niet aan wat zij niet kan waarmaken, en zegt zij dat. De medewerker kan gewoon doorwerken.

### Invoering

**Moeten we wachten op de leveranciers?**  
Nee. De eerste drie stappen van het groeipad vragen niets van de leveranciers en leveren het merendeel van de baten.

**Hoe verhoudt dit zich tot Common Ground?**  
De assistent past het principe van gegevens bij de bron toe op documenten. Zij voegt geen principe toe en vervangt geen voorziening.

## Begrippen

| Begrip | Betekenis |
|---|---|
| Informatieobject | Een stuk met een eigen identificatie in het platform, los van de plek waar het bestand staat. |
| Onderdeel | Iets in een stuk dat ergens anders vandaan komt: een tabel, een grafiek, een kaart, een bijlage. |
| Binding | De vastgelegde relatie tussen een stuk en een onderdeel: welke bron, welke versie, op welke plek. |
| Herkomst | Waar een onderdeel vandaan komt, wie de bron beheert en wanneer het is opgehaald. |
| Vaststellen | Het moment waarop vaststaat dat dit de versie is waarover is besloten. Daarna staat het stuk stil. |
| Bewering | Een waarde met erbij wie haar heeft bepaald, wanneer, waarom en waarop. |
| Gezag | Hoe zwaar een bewering weegt: een voorstel van een programma, overgenomen uit een bron, afgeleid door een regel, of besloten door een bevoegde functionaris. |
| Vermogen | Iets wat een werkomgeving kan, zoals een deel van het document vervangen of melden wat in beeld is. |
| Adapter | De dunne schakel tussen een werkomgeving en de assistent. Er is er één per werkomgeving. |
| Contract | De afspraak tussen adapter en assistent: wat de werkomgeving levert en wat het paneel kan tonen en vragen. |
| Beoordelingsweergave | Het eigen venster van de assistent voor beslissingen vóór openbaarmaking, in elke werkomgeving gelijk. |
| Gelakte versie | Een afgeleide van het stuk waarin passages onleesbaar zijn gemaakt. Het origineel blijft ongewijzigd. |
| Weigeringsgrond | De wettelijke grond waarop een passage niet openbaar wordt. Zij staat zichtbaar bij de passage. |
| Waarmerk | Het bewijs dat een versie van het stuk en haar samenstelling authentiek en ongewijzigd zijn sinds het moment van waarmerken. Het kan op elke versie, namens de auteur, een afdeling of de organisatie. |
| Trap | Een niveau op de ladder van wat een onderdeel kan, van losse kopie tot samengestelde weergave. |
| Zichtbare leemte | De aanduiding op de plek van een onderdeel dat een lezer niet mag zien, met de grondslag erbij. |

---

*Overgenomen uit de demo MIRA (README en Help) van Erik Hoekstra, Gemeente Haarlem; EUPL 1.2. Een demo, geen product: alle voorbeeldgegevens zijn verzonnen, en een deel is toekomstbeeld. Wijzigt de demo, dan is [de repo](https://github.com/EHa-1999/MIRA) leidend.*
