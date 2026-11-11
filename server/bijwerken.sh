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
# Sinds 0.8.0: de kennisbank (Wiki.js) en de architectuurmodellen.
vul_env_aan .env WIKI_DB_WACHTWOORD "$(geheim 24)"
vul_env_aan .env WIKI_BEHEER_WACHTWOORD "$(geheim 12)"
vul_env_aan .env WIKI_CLIENT_SECRET "$(geheim 32)"
vul_env_aan .env KENNISBANK_REPO ""
vul_env_aan .env ARCH_CLIENT_SECRET "$(geheim 32)"
vul_env_aan .env ARCH_COOKIE_SECRET "$(geheim 16)"
vul_env_aan .env ARCH_GITHUB_TOKEN ""
# Sinds 0.9.0: documenten (dg-documenten met Garage).
vul_env_aan .env DOC_DB_WACHTWOORD "$(geheim 24)"
vul_env_aan .env DOC_CLIENT_SECRET "$(geheim 32)"
vul_env_aan .env DOC_SESSIE_GEHEIM "$(geheim 32)"
vul_env_aan .env GARAGE_RPC_GEHEIM "$(geheim 32)"
vul_env_aan .env GARAGE_SLEUTEL "GK$(geheim 12)"
vul_env_aan .env GARAGE_GEHEIM "$(geheim 32)"
vul_env_aan .env DOC_BERICHT_GEHEIM "$(geheim 32)"
# De sleutel waarmee de wiki zijn pagina's naar GitHub schrijft (deploy key).
install -d -m 700 "$DG_MAP/geheimen"
if [ ! -f "$DG_MAP/geheimen/kennisbank" ]; then
  ssh-keygen -q -t ed25519 -N "" -C "kennisbank@$(hostname)" -f "$DG_MAP/geheimen/kennisbank"
  ok "Nieuwe sleutel voor de kennisbank-repo (geheimen/kennisbank.pub)"
fi
dc pull --quiet --ignore-buildable --ignore-pull-failures 2>/dev/null || true
dc --profile gereedschap pull --quiet keycloak-config wpcli wiki-inrichten 2>/dev/null || true
dc build --quiet || stop "Bouwen mislukt."
dc up -d --remove-orphans || stop "Starten mislukt."
if dc exec -T proxy caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile >/dev/null 2>&1; then
  ok "Caddy herladen"
else
  waarschuw "Caddy herladen lukte niet; kijk met: sudo docker compose -f $DG_MAP/$COMPOSE logs proxy"
fi

wacht "Keycloak" 48 keycloak_klaar || stop "Keycloak start niet. Kijk met: sudo docker compose -f $DG_MAP/$COMPOSE logs keycloak"
meld "Keycloak: realm gemeenschap gelijk trekken met keycloak/realm/"
# Poort 465: meteen versleuteld (SSL); andere poorten: STARTTLS.
if [ "$(sed -n 's/^SMTP_POORT=//p' "$DG_MAP/.env")" = 465 ]; then SMTP_SSL=true; SMTP_STARTTLS=false; else SMTP_SSL=false; SMTP_STARTTLS=true; fi
dc run --rm -T -e SMTP_SSL="$SMTP_SSL" -e SMTP_STARTTLS="$SMTP_STARTTLS" keycloak-config >/tmp/dg-keycloak-config.log 2>&1 \
  || { tail -n 30 /tmp/dg-keycloak-config.log; stop "Keycloak inrichten mislukt (zie hierboven)."; }
ok "Realm bijgewerkt"

meld "Kennisbank (Wiki.js) bijwerken"
# De volumes voor de Git-werkkopie en de uploads zijn (van vóór 0.9.1) van
# root; de wiki draait als node en moet erin kunnen schrijven.
dc exec -T -u 0 wiki chown -R node:node /wiki/data/repo /wiki/data/content || waarschuw "Eigenaar van de wiki-mappen zetten lukte niet"
dc run --rm -T wiki-inrichten || stop "Kennisbank inrichten mislukt; kijk in de uitvoer hierboven en met: sudo docker compose -f $DG_MAP/$COMPOSE logs wiki"
if ! grep -qE '^KENNISBANK_REPO=.+' .env; then
  waarschuw "De pagina's van de kennisbank gaan nog niet naar GitHub. Zet deze sleutel als deploy key (met schrijfrechten) bij de repo kennisbank:"
  printf '      %s\n' "$(cat "$DG_MAP/geheimen/kennisbank.pub")"
  waarschuw "en zet daarna KENNISBANK_REPO=git@github.com:digitaalgemeengoed/kennisbank.git in .env (README, Kennisbank)."
fi

wacht "WordPress" 36 wordpress_klaar || stop "WordPress start niet. Kijk met: sudo docker compose -f $DG_MAP/$COMPOSE logs wordpress"
meld "WordPress bijwerken (BuddyPress, bbPress, CiviCRM, thema)"
dc run --rm -T wpcli dg-inrichten || stop "WordPress bijwerken mislukt; kijk in de uitvoer hierboven."

wacht "Redmine" 60 redmine_klaar || stop "Redmine start niet. Kijk met: sudo docker compose -f $DG_MAP/$COMPOSE logs redmine"
meld "Redmine bijwerken"
dc exec -T -u redmine redmine bundle exec rails runner /usr/local/share/dg/inrichten.rb >/dev/null || stop "Redmine inrichten mislukt. Kijk met: sudo docker compose -f $DG_MAP/$COMPOSE logs redmine"
dc restart redmine-groepen >/dev/null

wacht "Documenten" 30 documenten_klaar || stop "De documentendienst start niet. Kijk met: sudo docker compose -f $DG_MAP/$COMPOSE logs documenten garage"
ok "Documenten draait"

# Architectuurmodellen: elk kwartier bouwen wat nieuw is (architectuur/repos.txt).
install -d -m 755 /srv/dg-architectuur
install -m 644 "$DG_MAP/server/systemd/dg-architectuur.service" "$DG_MAP/server/systemd/dg-architectuur.timer" /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now dg-architectuur.timer >/dev/null 2>&1 || waarschuw "Timer dg-architectuur starten lukte niet"
systemctl start --no-block dg-architectuur.service || true
ok "Architectuurmodellen: elk kwartier bijwerken (systemd-timer dg-architectuur)"
docker image prune -f >/dev/null
ok "Bijgewerkt"
echo
