#!/usr/bin/env bash
# Terug naar een eerdere versie van het platform (de infrastructuur: images,
# thema, Keycloak, WordPress, Redmine, de kennisbank en deze scripts).
#
#   sudo /opt/gemeenschap/server/terugzetten.sh            welke versies er zijn
#   sudo /opt/gemeenschap/server/terugzetten.sh v0.8.0     terug naar v0.8.0
#
# Wat het doet:
#   1. een back-up van nu (zo kun je ook dit weer ongedaan maken);
#   2. de gekozen versie neerzetten met haar eigen bijwerken.sh;
#   3. controleren of de adressen antwoorden; zo niet, terug naar waar je was;
#   4. de nieuwere versies noteren als "niet opnieuw proberen", zodat de
#      automatische uitrol ze niet meteen weer neerzet. Een volgende, nieuwe
#      versie rolt daarna gewoon uit.
#
# Wat het niet doet: de databases terugzetten. Veranderde een nieuwere versie
# iets in een database, gebruik dan ook herstellen.sh met de back-up van vlak
# voor die versie (uitrol.sh maakt die altijd).
#
# De pagina's (inhoud/) volgen main; daarvoor is dit script niet nodig: draai
# een wijziging terug op GitHub (revert), dan staat het binnen vijf minuten.

# Dit script vervangt zichzelf (een oudere versie heeft het misschien niet eens);
# daarom draait het vanaf een kopie, net als uitrol.sh.
if [ -z "${DG_TERUG_KOPIE:-}" ]; then
  kopie="$(mktemp /run/dg-terugzetten.XXXXXX)"
  cp "$0" "$kopie"
  DG_TERUG_KOPIE="$kopie" DG_TERUG_BRON="$(cd "$(dirname "$0")" && pwd)" exec bash "$kopie" "$@"
fi
trap 'rm -f "$DG_TERUG_KOPIE"' EXIT

# shellcheck source=lib.sh
. "$DG_TERUG_BRON/lib.sh"
moet_root
install -d -m 700 "$DG_STAAT"
cd "$DG_MAP" || exit 1
[ -s .env ] || stop "Nog niet geïnstalleerd."

NU="$(cat "$DG_STAAT/infra" 2>/dev/null || true)"
noteer() { printf '%s %s\n' "$(date -Iseconds)" "$*" >> "$DG_STAAT/logboek"; echo "  $*"; }

git fetch --quiet --tags origin || waarschuw "GitHub niet bereikbaar; ik gebruik de versies die hier al bekend zijn."
mapfile -t VERSIES < <(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' | sort -V)

DOEL="${1:-}"
if [ -z "$DOEL" ]; then
  meld "Versies"
  for v in "${VERSIES[@]}"; do
    if [ "$v" = "$NU" ]; then echo "  $v   ← staat nu"; elif grep -qx "$v" "$DG_STAAT/mislukt" 2>/dev/null; then echo "  $v   (niet opnieuw proberen)"; else echo "  $v"; fi
  done
  echo
  echo "  Terug naar een versie: sudo $DG_MAP/server/terugzetten.sh <versie>"
  exit 0
fi
[[ "$DOEL" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || stop "Geef een versie zoals v0.8.0."
printf '%s\n' "${VERSIES[@]}" | grep -qx "$DOEL" || stop "Versie $DOEL bestaat niet. Zonder versie erachter zie je welke er zijn."
[ "$DOEL" != "$NU" ] || { ok "$DOEL staat al"; exit 0; }

# Niet tegelijk met een uitrol: dezelfde vergrendeling als uitrol.sh.
exec 9>"$DG_STAAT/slot"
flock -n 9 || stop "Er loopt net een uitrol. Probeer het over een paar minuten opnieuw."

meld "Terug van ${NU:-(onbekend)} naar $DOEL"
echo "  Containers en instellingen gaan terug; databases niet (daarvoor: herstellen.sh)."
if [ -t 0 ]; then vraag_ja "  Doorgaan?" || stop "Niets gedaan."; fi

"$DG_MAP/server/back-up.sh" || stop "Back-up mislukt; niets gedaan."

git -c advice.detachedHead=false checkout --quiet --force "$DOEL"
if "$DG_MAP/server/bijwerken.sh" && wacht "de site" 24 gezond; then
  echo "$DOEL" > "$DG_STAAT/infra"
  # Alle nieuwere versies niet automatisch opnieuw neerzetten.
  for v in "${VERSIES[@]}"; do
    if [ "$(printf '%s\n%s\n' "$DOEL" "$v" | sort -V | tail -1)" = "$v" ] && [ "$v" != "$DOEL" ]; then
      grep -qx "$v" "$DG_STAAT/mislukt" 2>/dev/null || echo "$v" >> "$DG_STAAT/mislukt"
    fi
  done
  noteer "Met de hand teruggezet: ${NU:-(onbekend)} → $DOEL. Nieuwere versies staan in $DG_STAAT/mislukt; een volgende nieuwe versie rolt gewoon uit."
  ok "$DOEL staat"
  echo
  echo "  Een nieuwere versie toch weer neerzetten: haal haar uit $DG_STAAT/mislukt"
  echo "  en draai sudo $DG_MAP/server/uitrol.sh"
else
  reden="$(gezond 2>&1 || true)"
  noteer "Terugzetten naar $DOEL mislukt (${reden:-bijwerken gaf een fout})."
  if [ -n "$NU" ]; then
    waarschuw "Terugzetten mislukt; ik zet $NU weer neer."
    git -c advice.detachedHead=false checkout --quiet --force "$NU"
    "$DG_MAP/server/bijwerken.sh" || true
    noteer "Weer terug op $NU."
  fi
  stop "Terugzetten naar $DOEL is niet gelukt (${reden:-bijwerken gaf een fout}). Kijk in de uitvoer hierboven."
fi
