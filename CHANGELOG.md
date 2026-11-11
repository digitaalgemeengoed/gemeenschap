# Wijzigingen

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
