#!/usr/bin/env bash
# Het platform afschermen met één gebruikersnaam en wachtwoord (HTTP Basic
# Auth), bijvoorbeeld zolang het nog niet openbaar is. Geldt voor de site,
# aanmelden. en projecten.; verkeer van de server zelf en uit de containers
# hoeft niet in te loggen.
#
#   sudo /opt/gemeenschap/server/afschermen.sh aan   vraagt gebruikersnaam en wachtwoord
#   sudo /opt/gemeenschap/server/afschermen.sh uit   weer openbaar
#   sudo /opt/gemeenschap/server/afschermen.sh       hoe staat het
#
# In .env komt alleen een hash van het wachtwoord (bcrypt, base64), nooit het
# wachtwoord zelf.

HIER="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
. "$HIER/lib.sh"
moet_root
cd "$DG_MAP" || exit 1
[ -s .env ] || stop "Nog niet geïnstalleerd."

case "${1:-}" in
  aan)
    read -r -p "  Gebruikersnaam: " GEBRUIKER
    [[ "$GEBRUIKER" =~ ^[A-Za-z0-9._-]{1,64}$ ]] || stop "Alleen letters, cijfers, punt, liggend streepje en koppelteken."
    read -r -s -p "  Wachtwoord (je ziet niets terwijl je typt): " W1; echo
    read -r -s -p "  Nog een keer: " W2; echo
    [ "$W1" = "$W2" ] || stop "De twee wachtwoorden zijn niet gelijk."
    [ "${#W1}" -ge 8 ] || stop "Neem een wachtwoord van minstens 8 tekens."
    HASH="$(printf '%s\n' "$W1" | dc exec -T proxy caddy hash-password 2>/dev/null | tail -1)"
    unset W1 W2
    [[ "$HASH" == \$2* ]] || stop "Een hash maken lukte niet; draait de proxy? (sudo docker compose -f $DG_MAP/$COMPOSE ps proxy)"
    zet_env .env DG_TOEGANG_GEBRUIKER "$GEBRUIKER"
    zet_env .env DG_TOEGANG_HASH "$(printf '%s' "$HASH" | base64 -w0)"
    zet_env .env DG_TOEGANG dicht
    dc up -d proxy >/dev/null 2>&1 || stop "De proxy herstarten lukte niet; kijk met: sudo docker compose -f $DG_MAP/$COMPOSE logs proxy"
    ok "Afgeschermd: wie de site, aanmelden. of projecten. opent, moet eerst inloggen als $GEBRUIKER"
    ;;
  uit)
    zet_env .env DG_TOEGANG open
    dc up -d proxy >/dev/null 2>&1 || stop "De proxy herstarten lukte niet; kijk met: sudo docker compose -f $DG_MAP/$COMPOSE logs proxy"
    ok "Niet meer afgeschermd: het platform is openbaar"
    ;;
  "")
    if grep -q '^DG_TOEGANG=dicht' .env; then
      ok "Afgeschermd (gebruiker $(sed -n 's/^DG_TOEGANG_GEBRUIKER=//p' .env))"
    else
      ok "Niet afgeschermd"
    fi
    ;;
  *) stop "Gebruik: afschermen.sh aan | uit" ;;
esac
