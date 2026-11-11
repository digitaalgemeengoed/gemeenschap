---
titel: IRIS — Gebruikersbeheer
omschrijving: Identity, Roles & Integrated Sessions: geautomatiseerd gebruikersbeheer van personeelsdossier tot toegangsbesluit.
labels: demo, iam, toegang, iris
---


# IRIS — Gebruikersbeheer

**Identity, Roles & Integrated Sessions.** Een demo van geautomatiseerd gebruikersbeheer voor de systemen uit de XENA-demo's: van het personeelsdossier tot het besluit op een stuk, zonder handwerk per systeem. De naam verwijst naar Iris, de boodschapper van de Griekse goden.

IRIS hoort bij XENA, naast de demo over opslag, MIRA (de assistent in het kantoorpakket), TINA (het zoekportaal voor inwoners), ELSA (e-mailarchivering), MAIA (chatarchivering) en VERA (waarmerken). De demo is bedoeld voor IT-beheer, security en architecten.

> Een demo, geen product. Personen, stukken en aantallen zijn verzonnen.

**[Broncode en uitleg op GitHub](https://github.com/EHa-1999/IRIS)** (open daar `index.html` in je browser)

## Wat de demo laat zien

| Weergave | Wat je ziet |
|---|---|
| **Levenscyclus** | In dienst, interne overstap, extra bevoegdheid, uit dienst en samenwerking met een andere gemeente. Stap voor stap, nu met de hand per systeem en straks geautomatiseerd, met wat er achterblijft. |
| **Toegang** | Kies een persoon, een stuk en een handeling. Het besluit van nu (rollen en ACL's per toepassing) staat naast dat van een beleidsbeslispunt, met de AuthZEN-vraag en het antwoord. |
| **Architectuur** | De lagen: HR, Saviynt voor identity governance, Entra ID als identiteitsbron, Keycloak voor sessies en tokens, een beslispunt voor de besluiten en de toepassingen als handhavingspunt. Met de stappen bij aanmelden, handelen namens de medewerker, een toegangsbeslissing, uit dienst, hercertificering en een inwoner bij TINA. |
| **Techniek** | Componenten, de koppeling per systeem uit de demo's, de modellen van RBAC tot beleid, ontwerpregels en standaarden. |
| **Groeipad** | Acht stappen, beginnend met een rolcatalogus voor de demo-systemen. |
| **Help** | Weergaven, veelgestelde vragen en begrippen. |

## Veelgestelde vragen

### Begrippen

**Wat is het verschil tussen Saviynt en Keycloak?**  
Saviynt bepaalt wie wat mag en waarom: rollen, aanvragen, herzieningen. Keycloak voert het uit bij elke aanmelding: sessies en tokens. Rollen veranderen in Saviynt, nooit in Keycloak.

**Waarom Keycloak tussen Entra ID en de systemen?**  
Zodat de systemen niet aan Entra ID vastzitten. Wordt Entra ID vervangen, dan verandert alleen de koppeling achter Keycloak.

**Wat is een geboorterecht?**  
De rechten die iemand krijgt door zijn functie, zonder aanvraag. Alles daarboven is een extra recht, met een aanvraag, een goedkeurder en een herziening.

### Beheer

**Verdwijnt het werk van functioneel beheer?**  
Het handwerk wel: accounts aanmaken en rechten toekennen per ticket. Wat blijft, is inhoudelijk: rollen koppelen, zaken toewijzen, het beleid mee opstellen en uitzonderingen beoordelen.

**Wie beslist wie zegelbevoegd is?**  
De gemeentesecretaris, via een aanvraag in Saviynt. De zegeldienst kijkt alleen naar de rol in het token.

### Beveiliging

**Hoe snel is iemand die vertrekt eruit?**  
Op het geplande moment gaat het account uit en eindigen alle sessies. Tokens zijn vijf minuten geldig, dus binnen vijf minuten werkt niets meer.

**Heeft de Office Assistent eigen rechten?**  
Nee. De assistent handelt altijd namens de medewerker, met een ingeruild token. Wat de medewerker niet mag, kan de assistent ook niet.

**Mag een beheerder alles zien?**  
Een beheerder ziet de metadata. De inhoud alleen met een vastgelegde reden en een melding aan security.

### Zaaksysteem

**Neemt IAM de rechten van het zaaksysteem over?**  
Nee. Het zaaksysteem blijft leidend voor wie welke zaak behandelt. Het beslispunt vraagt die relatie op.

### Modellen

**Moeten we meteen naar ABAC?**  
Nee. Eerst gaan de regels van nu één op één naar het beslispunt. Daarna komen eigenschappen en relaties erbij, per soort stuk.

### Inwoners

**Staan inwoners ook in Saviynt?**  
Nee. Inwoners melden zich aan met DigiD in een aparte omgeving van Keycloak, en krijgen alleen hun eigen stukken en openbare stukken.

## Begrippen

| Begrip | Betekenis |
|---|---|
| IAM | Identity and access management: wie iemand is en wat die mag. |
| IGA | Identity governance and administration: het beheer van rechten, met aanvragen, goedkeuring en herziening. |
| Geboorterecht | Recht dat bij een functie hoort, zonder aanvraag. |
| Rechtenstapeling | Rechten die zich opstapelen omdat oude niet worden ingetrokken. |
| Hercertificering | Het periodiek herzien van rechten door de leidinggevende. |
| Functiescheiding | Combinaties van rechten die niet bij één persoon mogen liggen. |
| RBAC | Toegang op grond van een rol. |
| ABAC | Toegang op grond van eigenschappen van persoon, stuk en situatie. |
| ReBAC | Toegang op grond van een relatie, zoals behandelaar van een zaak. |
| PBAC | Toegang op grond van beleid als code, dat de andere combineert. |
| Beslispunt | Het onderdeel dat besluit of iets mag (PDP). |
| Handhavingspunt | Het onderdeel dat het besluit uitvoert (PEP). |
| AuthZEN | Standaard voor de vraag van handhavingspunt aan beslispunt. |
| SCIM | Standaard om accounts en groepen aan te maken en op te heffen. |
| Token exchange | Een token inruilen voor een token voor een ander systeem, namens dezelfde persoon. |
| Break-glass | Toegang in uitzonderingen, met een vastgelegde reden en een melding. |

---

*Overgenomen uit de demo IRIS (README en Help) van Erik Hoekstra, Gemeente Haarlem; EUPL 1.2. Een demo, geen product: alle voorbeeldgegevens zijn verzonnen, en een deel is toekomstbeeld. Wijzigt de demo, dan is [de repo](https://github.com/EHa-1999/IRIS) leidend.*
