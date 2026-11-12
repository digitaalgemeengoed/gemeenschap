#!/usr/bin/env bash
# Het gemeenschapsplatform installeren op VPS 4: Keycloak (realm gemeenschap),
# WordPress met BuddyPress, bbPress en CiviCRM, en Redmine. Daarna rolt de
# server zelf uit wat er op GitHub verandert (server/uitrol.sh).
#
# Richt de server eerst in met oa-uitrol (server/inrichten.sh, kies 4, en
# ssh-afsluiten.sh); oa-uitrol/server/installeer.sh haalt deze repo op en
# start dit script. Zelf starten kan ook:
#
#   sudo /opt/gemeenschap/server/installeer.sh
#
# Schrijft de eerste keer .env met nieuwe, willekeurige wachtwoorden en houdt
# die bij een volgende keer. Opnieuw draaien kan zonder schade.

HIER="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
. "$HIER/lib.sh"
moet_root
command -v docker >/dev/null && docker compose version >/dev/null 2>&1 \
  || stop "Docker ontbreekt. Richt de server eerst in met oa-uitrol: server/inrichten.sh, kies 4."
[ "$DG_MAP" = "$(cd "$HIER/.." && pwd)" ] || stop "Deze repo hoort in $DG_MAP te staan."
cd "$DG_MAP" || exit 1

meld "Gemeenschapsplatform installeren"

# --- Domein en DNS --------------------------------------------------------------
DOMEIN="$(sed -n 's/^DOMEIN=//p' .env 2>/dev/null || true)"
while [ -z "${DOMEIN:-}" ]; do
  read -r -p "  Domein van het platform (bijvoorbeeld digitaalgemeengoed.nl): " DOMEIN
  DOMEIN="${DOMEIN,,}"; DOMEIN="${DOMEIN#https://}"; DOMEIN="${DOMEIN%/}"
  geldig_domein "$DOMEIN" || { waarschuw "Dat is geen geldig domein."; DOMEIN=""; }
done
IP="$(publiek_ip)"
ONTBREEKT=0
for naam in "$DOMEIN" "www.$DOMEIN" "aanmelden.$DOMEIN" "projecten.$DOMEIN" "forum.$DOMEIN" "kennis.$DOMEIN" "architectuur.$DOMEIN" "documenten.$DOMEIN" "beheer.$DOMEIN"; do
  gevonden="$(getent ahostsv4 "$naam" | awk 'NR==1{print $1}')"
  if [ "$gevonden" = "$IP" ]; then ok "$naam wijst naar deze server"; else waarschuw "$naam wijst naar '${gevonden:-niets}', niet naar $IP"; ONTBREEKT=1; fi
done
for naam in "$DOMEIN" "aanmelden.$DOMEIN" "projecten.$DOMEIN"; do
  gevonden="$(getent ahostsv4 "$naam" | awk 'NR==1{print $1}')"
  [ "$gevonden" = "$IP" ] || stop "Maak eerst in je DNS A-records naar $IP voor $DOMEIN, aanmelden.$DOMEIN en projecten.$DOMEIN (en liefst www. en forum.), wacht een kwartier en probeer opnieuw. Zonder die records kan Caddy geen certificaten halen."
done
[ "$ONTBREEKT" = 0 ] || waarschuw "www. en forum. zijn alleen doorverwijzingen, kennis., architectuur., documenten. en beheer. kunnen later; Caddy haalt het certificaat zodra de naam naar deze server wijst."

# --- .env -----------------------------------------------------------------------
if [ -s .env ]; then
  ok ".env bestaat al; wachtwoorden blijven gelijk"
else
  umask 077
  {
    echo "# Geschreven door server/installeer.sh op $(date -I). Bevat wachtwoorden: niet delen."
    echo "DOMEIN=$DOMEIN"
    echo "KC_DB_WACHTWOORD=$(geheim 24)"
    echo "KC_BEHEERDER=beheer"
    echo "KC_BEHEERDER_WACHTWOORD=$(geheim 12)"
    echo "WP_DB_WACHTWOORD=$(geheim 24)"
    echo "WP_DB_ROOT_WACHTWOORD=$(geheim 24)"
    echo "WP_CLIENT_SECRET=$(geheim 32)"
    echo "WP_BEHEER_WACHTWOORD=$(geheim 12)"
    echo "REDMINE_DB_WACHTWOORD=$(geheim 24)"
    echo "REDMINE_CLIENT_SECRET=$(geheim 32)"
    echo "REDMINE_SYNC_SECRET=$(geheim 32)"
    echo "REDMINE_SECRET_KEY_BASE=$(geheim 64)"
    echo "REDMINE_API_KEY=$(geheim 20)"
    echo "REDMINE_SITE_API_KEY=$(geheim 20)"
    echo "REDMINE_ADMIN_WACHTWOORD=$(geheim 12)"
    echo "CIVICRM_VERSIE=6.20.0"
    echo "SMTP_HOST="
    echo "SMTP_POORT=587"
    echo "SMTP_GEBRUIKER="
    echo "SMTP_WACHTWOORD="
    echo "SMTP_AFZENDER=noreply@$DOMEIN"
  } > .env
  umask 022
  ok ".env geschreven met nieuwe wachtwoorden"
fi

# --- Mail ----------------------------------------------------------------------
# Zonder SMTP-dienst gaat er geen mail weg: geen bevestiging van het e-mailadres
# bij het aanmelden, geen nieuwsbrief, geen meldingen uit Redmine.
if ! grep -qE '^SMTP_HOST=.+' .env; then
  echo
  echo "  Mail gaat via een SMTP-dienst (bij voorkeur een Europese aanbieder). Zonder"
  echo "  SMTP kan niemand zich aanmelden: Keycloak stuurt een mail om het adres te bevestigen."
  if vraag_ja "  Nu de SMTP-gegevens invullen?"; then
    read -r -p "  Server (bijvoorbeeld smtp.aanbieder.nl): " H
    read -r -p "  Poort [587]: " P; P="${P:-587}"
    read -r -p "  Gebruiker: " U
    read -r -s -p "  Wachtwoord (je ziet niets terwijl je typt): " W; echo
    read -r -p "  Afzender [noreply@$DOMEIN]: " A; A="${A:-noreply@$DOMEIN}"
    for v in "$H" "$P" "$U" "$W" "$A"; do
      [[ "$v" =~ ^[A-Za-z0-9+/=.:_,@-]*$ ]] || stop "De SMTP-gegevens bevatten tekens die .env niet aankan (alleen letters, cijfers en + / = . : _ , @ -). Gebruik een ander wachtwoord of vul .env zelf in."
    done
    zet_env .env SMTP_HOST "$H"; zet_env .env SMTP_POORT "$P"; zet_env .env SMTP_GEBRUIKER "$U"
    zet_env .env SMTP_WACHTWOORD "$W"; zet_env .env SMTP_AFZENDER "$A"
    ok "SMTP ingesteld"
  else
    waarschuw "Geen SMTP: vul SMTP_HOST en de rest later in $DG_MAP/.env in en draai dan: sudo $DG_MAP/server/bijwerken.sh"
  fi
fi

# --- Versie ------------------------------------------------------------------
# De nieuwste versie-tag op main; zonder tags de laatste stand van main.
git fetch --quiet --tags origin || stop "GitHub niet bereikbaar."
TAG="$(nieuwste_tag)"
if [ -n "$TAG" ]; then
  git -c advice.detachedHead=false checkout --quiet "$TAG"
  ok "Versie $TAG"
else
  git checkout --quiet main && git merge --ff-only --quiet origin/main
  waarschuw "Nog geen versie-tag; ik gebruik de laatste stand van main"
fi

# --- Neerzetten en inrichten ------------------------------------------------------
meld "Images ophalen en bouwen (de eerste keer tien tot vijftien minuten)"
"$DG_MAP/server/bijwerken.sh" || stop "Installeren mislukt. Opnieuw draaien kan: sudo $DG_MAP/server/installeer.sh"
install -d -m 700 "$DG_STAAT"
[ -n "$TAG" ] && echo "$TAG" > "$DG_STAAT/infra"

# --- Back-up en uitrol --------------------------------------------------------------
echo "30 3 * * * root $DG_MAP/server/back-up.sh >/var/log/dg-back-up.log 2>&1" > /etc/cron.d/dg-gemeenschap
ok "Dagelijkse back-up om 03:30"
install -m 644 "$DG_MAP/server/systemd/dg-uitrol.service" "$DG_MAP/server/systemd/dg-uitrol.timer" /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now dg-uitrol.timer >/dev/null 2>&1
ok "Uitrol vanuit GitHub elke vijf minuten (systemd-timer dg-uitrol)"
meld "Pagina's uit inhoud/"
"$DG_MAP/server/uitrol.sh" || waarschuw "De pagina's zijn niet gelukt; kijk met: sudo $DG_MAP/server/uitrol.sh --status"

KC_BEHEERDER_WACHTWOORD="$(sed -n 's/^KC_BEHEERDER_WACHTWOORD=//p' .env)"
WP_BEHEER_WACHTWOORD="$(sed -n 's/^WP_BEHEER_WACHTWOORD=//p' .env)"
REDMINE_ADMIN_WACHTWOORD="$(sed -n 's/^REDMINE_ADMIN_WACHTWOORD=//p' .env)"
cat <<EOF

  Klaar. Het platform staat op:
      https://$DOMEIN                 site, leden, fora
      https://$DOMEIN/lid-worden/     aanmelden
      https://projecten.$DOMEIN       Redmine

  Noodaccounts (bewaar ze in je wachtwoordmanager; ze staan ook in $DG_MAP/.env):
      Keycloak-beheer (via de SSH-tunnel):  beheer / $KC_BEHEERDER_WACHTWOORD
      WordPress (alleen met wp-cli):        beheer / $WP_BEHEER_WACHTWOORD
      Redmine (Inloggen met wachtwoord):    admin / $REDMINE_ADMIN_WACHTWOORD

  Volgende stappen (oa-uitrol INRICHTING.md, stap 12):
    1. Meld jezelf aan via /lid-worden/ en zet jezelf in Keycloak in de groepen
       /site/beheer en /netwerk/kerngroep.
    2. Maak in Keycloak per gemeente een organisatie met het e-maildomein.
    3. Vul Voorwaarden en Privacyverklaring in, op GitHub in inhoud/paginas/,
       en zet daar status: publiceren.
    4. Zet SPF, DKIM en DMARC voor $DOMEIN bij je SMTP-aanbieder.
    5. Wil je een mail bij elke uitrol? Zet UITROL_MELDEN=<adres> in $DG_MAP/.env.
EOF
