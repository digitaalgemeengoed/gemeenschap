#!/usr/bin/env bash
# Hoe staat het gemeenschapsplatform erbij? Containers, de drie adressen, de
# uitrol vanuit GitHub en de laatste back-up.
#
#   sudo /opt/gemeenschap/server/controleer.sh

HIER="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
. "$HIER/lib.sh"
moet_root
cd "$DG_MAP" || exit 1
[ -s .env ] || { echo "  Nog niet geïnstalleerd."; exit 0; }

meld "Containers"
dc ps --format '  {{.Service}}: {{.Status}}'

meld "Adressen"
DOMEIN="$(sed -n 's/^DOMEIN=//p' .env)"
for naam in "kennis.$DOMEIN" "documenten.$DOMEIN" "architectuur.$DOMEIN"; do
  wijst_hier "$naam" || waarschuw "$naam wijst nog niet naar deze server (DNS); dat deel is nog niet bereikbaar"
done
for adres in $(controle_adressen "$DOMEIN"); do
  code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 8 "https://$adres")"
  if [[ "$code" =~ ^[23] ]]; then ok "https://$adres ($code)"
  elif [ "$code" = 401 ] && grep -q '^DG_TOEGANG=dicht' .env; then ok "https://$adres (afgeschermd met wachtwoord)"
  else waarschuw "https://$adres antwoordt niet goed ($code)"; fi
done

meld "Uitrol vanuit GitHub"
if systemctl is-active -q dg-uitrol.timer; then ok "Timer actief"; else waarschuw "Timer staat uit: sudo systemctl enable --now dg-uitrol.timer"; fi

meld "Architectuurmodellen"
if systemctl is-active -q dg-architectuur.timer; then ok "Timer actief"; else waarschuw "Timer staat uit: sudo systemctl enable --now dg-architectuur.timer"; fi
if [ -s /var/lib/dg-gemeenschap/architectuur/lijst ]; then
  while IFS='|' read -r naam titel _ datum; do
    if [ -f "/srv/dg-architectuur/$naam/index.html" ]; then ok "$titel (bijgewerkt $datum)"; else waarschuw "$titel: nog geen rapport (zie sudo grep architectuur /var/lib/dg-gemeenschap/logboek)"; fi
  done < /var/lib/dg-gemeenschap/architectuur/lijst
else
  ok "Nog geen modellen in architectuur/repos.txt"
fi
"$DG_MAP/server/uitrol.sh" --status | sed 's/^/  /'

meld "Back-up"
LAATSTE="$(find /var/backups/dg-gemeenschap -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort | tail -1)"
if [ -n "$LAATSTE" ]; then ok "Laatste: $LAATSTE ($(du -sh "$LAATSTE" | cut -f1))"; else waarschuw "Nog geen back-up"; fi
echo
