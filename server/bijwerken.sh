#!/usr/bin/env bash
# Zet de versie die nu in /opt/gemeenschap staat neer: images ophalen en
# bouwen, Keycloak gelijk trekken met keycloak/realm/, WordPress (met plugins,
# vertalingen en CiviCRM) en Redmine bijwerken. Wachtwoorden in .env blijven.
#
# Normaal start server/uitrol.sh dit bij een nieuwe versie-tag. Met de hand
# (bijvoorbeeld na een wijziging in .env):
#
#   sudo /opt/gemeenschap/server/bijwerken.sh
#
# Stopt met een foutcode als iets wezenlijks mislukt; uitrol.sh gaat dan
# terug naar de vorige versie.

HIER="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
. "$HIER/lib.sh"
moet_root
cd "$DG_MAP" || exit 1
[ -s .env ] || stop "Nog niet geïnstalleerd. Gebruik installeer.sh."

meld "Bijwerken naar $(git describe --tags --always 2>/dev/null || echo onbekend)"
vul_env_aan .env CIVICRM_VERSIE 6.20.0
vul_env_aan .env UITROL_MELDEN ""
dc pull --quiet --ignore-buildable --ignore-pull-failures 2>/dev/null || true
dc --profile gereedschap pull --quiet keycloak-config wpcli 2>/dev/null || true
dc build --quiet || stop "Bouwen mislukt."
dc up -d --remove-orphans || stop "Starten mislukt."
if dc exec -T proxy caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile >/dev/null 2>&1; then
  ok "Caddy herladen"
else
  waarschuw "Caddy herladen lukte niet; kijk met: sudo docker compose -f $DG_MAP/$COMPOSE logs proxy"
fi

wacht "Keycloak" 48 keycloak_klaar || stop "Keycloak start niet. Kijk met: sudo docker compose -f $DG_MAP/$COMPOSE logs keycloak"
meld "Keycloak: realm gemeenschap gelijk trekken met keycloak/realm/"
dc run --rm -T keycloak-config >/tmp/dg-keycloak-config.log 2>&1 \
  || { tail -n 30 /tmp/dg-keycloak-config.log; stop "Keycloak inrichten mislukt (zie hierboven)."; }
ok "Realm bijgewerkt"

wacht "WordPress" 36 wordpress_klaar || stop "WordPress start niet. Kijk met: sudo docker compose -f $DG_MAP/$COMPOSE logs wordpress"
meld "WordPress bijwerken (BuddyPress, bbPress, CiviCRM, thema)"
dc run --rm -T wpcli dg-inrichten || stop "WordPress bijwerken mislukt; kijk in de uitvoer hierboven."

wacht "Redmine" 60 redmine_klaar || stop "Redmine start niet. Kijk met: sudo docker compose -f $DG_MAP/$COMPOSE logs redmine"
meld "Redmine bijwerken"
dc exec -T -u redmine redmine bundle exec rails runner /usr/local/share/dg/inrichten.rb >/dev/null || stop "Redmine inrichten mislukt. Kijk met: sudo docker compose -f $DG_MAP/$COMPOSE logs redmine"
dc restart redmine-groepen >/dev/null
docker image prune -f >/dev/null
ok "Bijgewerkt"
echo
