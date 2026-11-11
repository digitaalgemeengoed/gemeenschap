---
titel: LISA — Informatiebeveiliging
omschrijving: Layered Information Security Architecture: het Security Control Framework in één overzicht.
labels: demo, security, bio, lisa
---


# LISA — Informatiebeveiliging

**Layered Information Security Architecture.** Een demo die het Security Control Framework (SCF) in één overzicht laat zien: de zes lagen van LISA, wat er in de SCF-vault staat, en hoe de sturings- en operationele processen moeten lopen. Voor de CISO, security officers, teamleiders en security- en privacyspecialisten.

LISA hoort bij XENA, naast MIRA, TINA, ELSA, MAIA, VERA en IRIS.

> Een demo, geen product. De voorbeelden komen uit de vault; het voorstel voor rollen en de beoordeling van de processen zijn van deze demo.

**[Broncode en uitleg op GitHub](https://github.com/EHa-1999/LISA)** (open daar `index.html` in je browser)

## De vault

De inhoud zelf staat in een Obsidian-vault:

- [scf-vault-nl](https://github.com/EHa-1999/scf-vault-nl): de Nederlandse vault, de bron.
- [scf-vault-en](https://github.com/EHa-1999/scf-vault-en), [scf-vault-de](https://github.com/EHa-1999/scf-vault-de), [scf-vault-fr](https://github.com/EHa-1999/scf-vault-fr): vertalingen in het Engels, Duits en Frans, met dezelfde ID's en velden.

## Wat de demo laat zien

| Weergave | Wat je ziet |
|---|---|
| **Lagen** | De zes lagen, van governance-kaders tot audits. Met één maatregel (MFA voor beheerders in XENA) die je volgt van NIS2 tot de bevinding uit de audit. |
| **Vault** | Wat er per map in de vault staat, hoeveel, voor wie, met een link naar GitHub. En wat aandacht vraagt. |
| **Processen** | Vijf sturingsprocessen en elf operationele processen, stap voor stap per rol, met per proces of de vault het beschrijft. Met de jaarcyclus. |
| **Rollen** | Per rol de taken, dashboards en het ritme, en een voorstel voor een RACI-matrix. |
| **Meten** | De implementatiescore en de compliancescore om mee te rekenen, met de drempels en wat er te besluiten valt. |
| **Groeipad** | Acht stappen van vault naar werkend stelsel. |
| **Help** | Weergaven, veelgestelde vragen en begrippen. |

Met de keuze **Bekijk als** bovenaan markeert de demo wat voor een rol het meest telt.

## Veelgestelde vragen

### Begrippen

**Wat is het verschil tussen LISA en de SCF-vault?**  
LISA is het raamwerk: de lagen en de regels. De SCF-vault is de uitwerking ervan in Obsidian, met de controls, systemen en dashboards van de gemeente.

**Wat is een baseline-control?**  
Een maatregel die altijd verplicht is. Wie er niet aan voldoet, moet uitleggen waarom en een uitzondering laten goedkeuren door de CISO, met een vervaldatum.

**Waarom zowel CIS-controls als eigen controls?**  
CIS dekt vooral techniek. De eigen controls vullen aan waar de BIO, ISO 27001 en de AVG meer vragen: privacy, organisatie, personeel, fysieke beveiliging, leveranciers en continuïteit.

**Wat is een SCB?**  
Een System Component Breakdown: de opbouw van een systeem uit componenten, met versies, gegevensstromen en grenzen. Het is de spil van de intake.

### Gebruik

**Moet iedereen met Obsidian werken?**  
Nee. Wie beheert, wel. Wie alleen leest, kan de vault op GitHub bekijken of een export van de dashboards gebruiken.

**Waar begin ik als systeemeigenaar?**  
Bij de intakeprocedure in Documentatie, en daarna het voorbeeld in Voorbeelden.

### Scores

**Waarom twee scores?**  
Omdat ingericht en voldoende niet hetzelfde zijn. De MFA voor XENA is ingericht, maar een audit vond beheerders zonder MFA.

**Welke score gaat naar het bestuur?**  
De baselinescore per systeem en per kader, met het aantal open uitzonderingen en bevindingen ernaast.

### Processen

**Welke processen ontbreken?**  
Wijzigingen, incidenten en meldplicht, kwetsbaarheden en patches, en uitfasering. Ze worden genoemd, maar niet beschreven.

### Talen

**Komt de vault in andere talen?**  
De Nederlandse vault is de bron. Een Engelse, Duitse en Franse versie zijn gepland, met dezelfde ID's en velden.

## Begrippen

| Begrip | Betekenis |
|---|---|
| LISA | Layered Information Security Architecture: het gelaagde raamwerk. |
| SCF | Security Control Framework: de vault met de uitwerking. |
| SR | Security requirement: een van de dertien kerngebieden. |
| Control | Een beveiligingsmaatregel. |
| Benchmark | Hardeningrichtlijn per technologie, met maatregelen op niveau L1 en L2. |
| Baseline | De verplichte set maatregelen. |
| Implementatie | Een control toegepast op een component binnen een systeem. |
| Definition of Done | Meetbare criteria waarmee een implementatie af is. |
| BIV | Beschikbaarheid, integriteit en vertrouwelijkheid: de classificatie van een systeem. |
| BBN | Beveiligingsniveau volgens de BIO, van 1 tot 3. |
| SCB | System Component Breakdown. |
| Self-assessment | Zelfbeoordeling door de eigenaar, per component of systeem. |
| Bevinding | Een tekortkoming uit een audit, met ernst en termijn. |
| RACI | Wie voert uit, wie is eindverantwoordelijk, wie wordt geraadpleegd, wie geïnformeerd. |
| Dataview | Plug-in voor Obsidian die de dashboards maakt. |

---

*Overgenomen uit de demo LISA (README en Help) van Erik Hoekstra, Gemeente Haarlem; EUPL 1.2. Een demo, geen product: alle voorbeeldgegevens zijn verzonnen, en een deel is toekomstbeeld. Wijzigt de demo, dan is [de repo](https://github.com/EHa-1999/LISA) leidend.*
