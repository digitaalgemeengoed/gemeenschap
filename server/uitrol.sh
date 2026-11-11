#!/usr/bin/env bash
# Rolt uit wat er op GitHub verandert, in twee sporen:
#
#   Inhoud          De map inhoud/ op main. Na elke samenvoeging op main staan
#                   de pagina's binnen vijf minuten op de site.
#   Infrastructuur  Al het andere (images, Keycloak, WordPress, Redmine,
#                   Caddy, deze scripts). Alleen bij een nieuwe versie-tag
#                   (vX.Y.Z) op main. Eerst een back-up; mislukt de controle
#                   daarna, dan gaat de vorige versie terug.
#
# Draait elke vijf minuten via de systemd-timer dg-uitrol. Met de hand:
#
#   sudo /opt/gemeenschap/server/uitrol.sh            wat er nieuw is
#   sudo /opt/gemeenschap/server/uitrol.sh --status   wat er nu staat
#
# Pauzeren (bijvoorbeeld tijdens onderhoud): sudo touch /var/lib/dg-gemeenschap/pauze
# Een mislukte versie opnieuw proberen: haal hem uit /var/lib/dg-gemeenschap/mislukt

# Dit script kan zichzelf vervangen (een nieuwe tag). Bash leest een script
# terwijl het loopt; daarom draait het vanaf een kopie.
if [ -z "${DG_UITROL_KOPIE:-}" ]; then
  kopie="$(mktemp /run/dg-uitrol.XXXXXX)"
  cp "$0" "$kopie"
  DG_UITROL_KOPIE="$kopie" DG_UITROL_BRON="$(cd "$(dirname "$0")" && pwd)" exec bash "$kopie" "$@"
fi
trap 'rm -f "$DG_UITROL_KOPIE"' EXIT

# shellcheck source=lib.sh
. "$DG_UITROL_BRON/lib.sh"
moet_root
install -d -m 700 "$DG_STAAT"
exec 9>"$DG_STAAT/slot"
flock -n 9 || { echo "Er loopt al een uitrol."; exit 0; }
cd "$DG_MAP" || exit 1
[ -s .env ] || stop "Nog niet geïnstalleerd: sudo $DG_MAP/server/installeer.sh"

infra_nu()  { cat "$DG_STAAT/infra" 2>/dev/null || true; }
inhoud_nu() { cat "$DG_STAAT/inhoud" 2>/dev/null || true; }
noteer()    { printf '%s %s\n' "$(date -Iseconds)" "$*" >> "$DG_STAAT/logboek"; echo "$*"; }

if [ "${1:-}" = "--status" ]; then
  echo "Infrastructuur: $(infra_nu || true)"; echo "Inhoud (main):  $(inhoud_nu | cut -c1-12)"
  [ -f "$DG_STAAT/pauze" ] && echo "Gepauzeerd ($DG_STAAT/pauze)"
  [ -s "$DG_STAAT/mislukt" ] && echo "Mislukt, niet opnieuw geprobeerd: $(tr '\n' ' ' < "$DG_STAAT/mislukt")"
  echo; tail -n 10 "$DG_STAAT/logboek" 2>/dev/null || true
  exit 0
fi
if [ -f "$DG_STAAT/pauze" ]; then echo "Gepauzeerd."; exit 0; fi

# Een melding per mail aan UITROL_MELDEN (in .env), als dat is ingesteld.
meld_per_mail() {
  local onderwerp="$1" tekst="$2"
  ( lees_env .env
    [ -n "${UITROL_MELDEN:-}" ] && [ -n "${SMTP_HOST:-}" ] || exit 0
    ONDERWERP="$onderwerp" TEKST="$tekst" python3 -I - <<'PY' || true
import os, smtplib, ssl
from email.message import EmailMessage
e = os.environ
m = EmailMessage()
m["From"] = e.get("SMTP_AFZENDER") or "noreply@" + e["DOMEIN"]
m["To"] = e["UITROL_MELDEN"]
m["Subject"] = "[gemeenschap] " + e["ONDERWERP"]
m.set_content(e["TEKST"] + "\n\nLogboek: sudo /opt/gemeenschap/server/uitrol.sh --status\n")
poort = int(e.get("SMTP_POORT") or 587)
if poort == 465:
    s = smtplib.SMTP_SSL(e["SMTP_HOST"], poort, context=ssl.create_default_context(), timeout=30)
else:
    s = smtplib.SMTP(e["SMTP_HOST"], poort, timeout=30); s.starttls(context=ssl.create_default_context())
if e.get("SMTP_GEBRUIKER"):
    s.login(e["SMTP_GEBRUIKER"], e.get("SMTP_WACHTWOORD", ""))
s.send_message(m); s.quit()
PY
  )
}

# Antwoorden de drie adressen? Via Caddy op deze server, zonder DNS.
gezond() {
  local domein code adres
  domein="$(sed -n 's/^DOMEIN=//p' .env)"
  for adres in "$domein/" "aanmelden.$domein/realms/gemeenschap/.well-known/openid-configuration" "projecten.$domein/login"; do
    code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 15 --resolve "${adres%%/*}:443:127.0.0.1" "https://$adres")"
    [[ "$code" =~ ^[23] ]] || { echo "https://$adres gaf $code"; return 1; }
  done
}

git fetch --quiet --tags --prune --prune-tags origin || stop "GitHub niet bereikbaar."

# --- Infrastructuur: een nieuwe tag op main --------------------------------
NIEUW="$(nieuwste_tag)"
OUD="$(infra_nu)"
if [ -n "$NIEUW" ] && [ "$NIEUW" != "$OUD" ] && ! grep -qx "$NIEUW" "$DG_STAAT/mislukt" 2>/dev/null; then
  # Nooit terug naar een oudere versie omdat een tag is weggehaald.
  if [ -n "$OUD" ] && [ "$(printf '%s\n%s\n' "$OUD" "$NIEUW" | sort -V | tail -1)" != "$NIEUW" ]; then
    noteer "Nieuwste tag $NIEUW is ouder dan $OUD; niets gedaan."
  else
    noteer "Infrastructuur: ${OUD:-(eerste keer)} → $NIEUW"
    "$DG_MAP/server/back-up.sh" >/dev/null || stop "Back-up mislukt; $NIEUW niet uitgerold."
    git -c advice.detachedHead=false checkout --quiet --force "$NIEUW"
    if "$DG_MAP/server/bijwerken.sh" && wacht "de site" 24 gezond; then
      echo "$NIEUW" > "$DG_STAAT/infra"
      noteer "Infrastructuur $NIEUW staat."
      meld_per_mail "$NIEUW uitgerold" "Versie $NIEUW van het gemeenschapsplatform staat op $(hostname)."
    else
      echo "$NIEUW" >> "$DG_STAAT/mislukt"
      reden="$(gezond 2>&1 || true)"
      if [ -n "$OUD" ]; then
        noteer "$NIEUW mislukt (${reden:-bijwerken gaf een fout}); terug naar $OUD."
        git -c advice.detachedHead=false checkout --quiet --force "$OUD"
        "$DG_MAP/server/bijwerken.sh" || true
        meld_per_mail "$NIEUW mislukt, terug naar $OUD" "Uitrollen van $NIEUW lukte niet (${reden:-bijwerken gaf een fout}). Versie $OUD staat weer. Database-migraties van $NIEUW gaan niet vanzelf terug; de back-up van vlak ervoor staat in /var/backups/dg-gemeenschap."
      else
        noteer "$NIEUW mislukt (${reden:-bijwerken gaf een fout}); er was geen vorige versie."
        meld_per_mail "$NIEUW mislukt" "Uitrollen van $NIEUW lukte niet (${reden:-bijwerken gaf een fout}). Er was geen vorige versie om naar terug te gaan."
      fi
    fi
  fi
fi

# --- Inhoud: inhoud/ op main ---------------------------------------------------
REF="$(git rev-parse --quiet --verify origin/main:inhoud || true)"
if [ -n "$REF" ] && [ "$REF" != "$(inhoud_nu)" ]; then
  nieuw="$DG_STAAT/inhoud.nieuw"
  rm -rf "$nieuw"; mkdir -p "$nieuw"
  git archive origin/main inhoud | tar -x -C "$nieuw" --strip-components=1
  chmod -R a+rX "$nieuw"
  rm -rf "$DG_STAAT/inhoud.oud"
  [ -d "$DG_STAAT/inhoud.map" ] && mv "$DG_STAAT/inhoud.map" "$DG_STAAT/inhoud.oud"
  mv "$nieuw" "$DG_STAAT/inhoud.map"
  if uitvoer="$(DG_INHOUD="$DG_STAAT/inhoud.map" dc run --rm -T wpcli wp dg inhoud /inhoud 2>&1)"; then
    echo "$REF" > "$DG_STAAT/inhoud"
    noteer "Inhoud $(git rev-parse --short origin/main): $(echo "$uitvoer" | tail -1)"
  else
    echo "$uitvoer"
    # Niet elke vijf minuten opnieuw: pas weer bij de volgende wijziging op main.
    echo "$REF" > "$DG_STAAT/inhoud"
    noteer "Inhoud $(git rev-parse --short origin/main) niet gelukt: $(echo "$uitvoer" | grep -E 'Error|Warning' | head -3 | tr '\n' ' ')"
    meld_per_mail "pagina's niet bijgewerkt" "De pagina's van $(git rev-parse --short origin/main) konden niet worden bijgewerkt. Er is niets veranderd op de site.

$uitvoer"
  fi
fi
