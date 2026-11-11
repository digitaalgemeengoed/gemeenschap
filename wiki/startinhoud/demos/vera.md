---
titel: VERA — Waarmerken
omschrijving: Verified Electronic Records Attestation: gekwalificeerde zegels en tijdstempels, met de zegelsleutel in eigen beheer.
labels: demo, waarmerken, eidas, vera
---


# VERA — Waarmerken

**Verified Electronic Records Attestation.** Een demo van waarmerken in XENA: gekwalificeerde elektronische zegels en tijdstempels, met de zegelsleutel in eigen beheer. De naam komt van het Latijnse *verus*: waar, echt.

VERA hoort bij XENA, naast de demo over opslag, MIRA (de assistent in het kantoorpakket), TINA (het zoekportaal voor inwoners), ELSA (e-mailarchivering) en MAIA (chatarchivering).

> Een demo, geen product. Bedragen en aantallen zijn aannames om mee te rekenen.

**[Broncode en uitleg op GitHub](https://github.com/EHa-1999/VERA)** (open daar `index.html` in je browser)

## Wat de demo laat zien

Aan de voorkant is er weinig te zien: de Office Assistent heeft een knop Waarmerken. Deze demo gaat over wat daarachter gebeurt.

| Weergave | Wat je ziet |
|---|---|
| **Overzicht** | Voor betrokkenen en besluitvormers: tijdstempel, zegel en handtekening naast elkaar. Wat elk bewijst, wie het zet, hoe de medewerker bevestigt (niets, de tweede factor van de eigen inlog, of de app van de TSP) en wat het kost, met voorbeelden uit de gemeente. |
| **Architectuur** | De lagen van verzoek tot bewijs: wie om een waarmerk vraagt, de integratielaag, de zegeldienst in eigen beheer met het HSM-paar, en wat er naar buiten gaat (alleen hashes). Per handeling de stappen: een document of e-mailbericht waarmerken, automatisch bij binnenkomst, controleren, bewaren op lange termijn en de sleutelceremonie. |
| **Techniek** | Zegel, handtekening en tijdstempel naast elkaar, de componenten met wat vaststaat en wat kandidaat is, drie manieren om de sleutel te bewaren, de formaten per soort object, een rekenvoorbeeld met aannames, de sleutelceremonie en de standaarden. |
| **Groeipad** | Acht stappen, te beginnen met een kleine groep bevoegden die op verzoek zegelt via een dienst op afstand, tot een HSM-paar in eigen beheer dat met andere gemeenten wordt gedeeld. |
| **Help** | De weergaven, de schakelaar, veelgestelde vragen en de begrippen. |

Met de schakelaar bovenaan kies je het scenario: alleen op verzoek, of ook een automatisch tijdstempel op elk zakelijk object bij binnenkomst.

## Veelgestelde vragen

### Begrippen

**Kan ik ondertekenen met mijn gewone inlog?**  
Voor een zegel wel, met de rol zegelbevoegd en een extra bevestiging. Voor een gekwalificeerde handtekening niet: dan bevestig je in de app van de TSP, omdat alleen jij de sleutel mag bedienen.

**Waarom heeft de assistent drie verschillende knoppen?**  
Omdat het drie verschillende dingen zijn: een tijdstempel gaat automatisch, een zegel vraagt een bevoegde, en een handtekening vraagt jouw eigen bevestiging.

**Wat is het verschil tussen een zegel en een handtekening?**  
Een zegel is van de organisatie, een handtekening van een persoon. Een zegel mag een server automatisch zetten; een handtekening vraagt dat de persoon zelf de sleutel bedient.

**Wat bewijst een tijdstempel?**  
Dat een stuk op een moment in deze vorm bestond. Niet wie het heeft gemaakt.

### Privacy

**Gaan documenten naar een externe partij?**  
Nee. Voor het tijdstempel, en bij een dienst op afstand ook voor het zegel, gaat alleen een hash naar buiten: een vingerafdruk waaruit het document niet terug te halen is.

### Kosten

**Waarom niet meteen een eigen HSM?**  
Omdat het pas loont bij grote aantallen. Met een kleine groep bevoegden is een dienst op afstand goedkoper. Het rekenvoorbeeld in Techniek laat het omslagpunt zien.

**Waarom niet een USB-token op de server?**  
Omdat er dan maar één apparaat is, zonder uitwijk, en de pincode in de configuratie moet staan. Voor een proef kan het, voor productie niet.

### Bewaren

**Blijft een zegel altijd geldig?**  
Niet vanzelf. Certificaten verlopen en algoritmen verouderen. De bewaardienst zet op tijd een archieftijdstempel over het bewijs, zodat het geldig blijft zolang het stuk wordt bewaard.

### Controle

**Hoe controleert een ontvanger een waarmerk?**  
Bij een pdf in elke pdf-lezer, zoals bij een uittreksel van KVK. Bij een container op de openbare controlepagina, of met de controledienst van de EU.

### Bevoegdheid

**Wie mag zegelen?**  
In het begin een kleine groep: griffie, Juridische Zaken en informatiebeheer. De rol wordt toegekend door de gemeentesecretaris en vastgelegd in de integratielaag.

## Begrippen

| Begrip | Betekenis |
|---|---|
| Sole control | De eis dat alleen de ondertekenaar de sleutel kan gebruiken. |
| Europese wallet | De Europese digitale identiteitswallet, waarmee ook ondertekenen mogelijk wordt. |
| Zegel | Elektronisch bewijs dat een stuk van de organisatie komt en niet is gewijzigd. |
| Handtekening | Elektronisch bewijs dat een persoon met het stuk instemt. |
| Tijdstempel | Bewijs dat een stuk op een moment bestond. |
| QSCD | Gekwalificeerd middel om een zegel of handtekening te maken; de sleutel kan er niet uit. |
| HSM | Beveiligde hardware voor sleutels. Een gecertificeerde HSM is een QSCD. |
| TSP | Verlener van vertrouwensdiensten: geeft certificaten en tijdstempels uit. |
| PKIoverheid | Het stelsel van certificaten voor de Nederlandse overheid. |
| LTV | Langetermijnvalidatie: alles wat nodig is om het zegel later te controleren, zit in het stuk. |
| ASiC-E | Een container met bestanden en een zegel erover. |
| Hashboom | Een boom van vingerafdrukken waarmee één tijdstempel voor veel objecten geldt. |
| Sleutelceremonie | Het vastgelegde moment waarop de zegelsleutel in de HSM ontstaat, in bijzijn van de TSP. |

---

*Overgenomen uit de demo VERA (README en Help) van Erik Hoekstra, Gemeente Haarlem; EUPL 1.2. Een demo, geen product: alle voorbeeldgegevens zijn verzonnen, en een deel is toekomstbeeld. Wijzigt de demo, dan is [de repo](https://github.com/EHa-1999/VERA) leidend.*
