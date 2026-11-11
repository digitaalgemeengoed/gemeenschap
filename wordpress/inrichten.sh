#!/bin/sh
# WordPress inrichten of bijwerken voor Digitaal GemeenGoed. Draait in de
# wp-cli-container (compose.yml, service wpcli):
#
#   docker compose -f compose.yml run --rm wpcli dg-inrichten
#
# Opnieuw draaien kan zonder schade: wat er al is, blijft; plugins, WordPress
# en vertalingen worden bijgewerkt, net als de database van CiviCRM.
set -eu
cd /var/www/html
wp() { command wp --path=/var/www/html "$@"; }
log() { printf '  %s\n' "$*"; }

# --- WordPress ----------------------------------------------------------------
if ! wp core is-installed 2>/dev/null; then
  wp core install --url="https://$DOMEIN" --title="Digitaal GemeenGoed" \
    --admin_user=beheer --admin_password="$WP_BEHEER_WACHTWOORD" \
    --admin_email="beheer@$DOMEIN" --skip-email
  log "WordPress geïnstalleerd (noodaccount: beheer)"
fi
wp core update --minor >/dev/null && wp core update-db >/dev/null
wp language core install nl_NL --activate >/dev/null 2>&1 || true
wp option update blogname 'Digitaal GemeenGoed' >/dev/null
wp option update blogdescription 'Samen grenzeloos sterk' >/dev/null
wp option update timezone_string Europe/Amsterdam >/dev/null
wp option update date_format 'j F Y' >/dev/null
wp option update users_can_register 0 >/dev/null
wp option update default_comment_status closed >/dev/null
wp rewrite structure '/%postname%/' >/dev/null
log "WordPress $(wp core version), Nederlands"

# --- Plugins van wordpress.org ------------------------------------------------
for p in openid-connect-generic buddypress bbpress; do
  if wp plugin is-installed "$p"; then wp plugin update "$p" >/dev/null 2>&1 || true; else wp plugin install "$p" >/dev/null; fi
  wp plugin activate "$p" >/dev/null 2>&1 || true
done
wp language plugin install --all nl_NL >/dev/null 2>&1 || true
wp language plugin update --all >/dev/null 2>&1 || true
log "Plugins: OpenID Connect Generic, BuddyPress, bbPress"

# Inloggen via Keycloak: de vaste instellingen; geheimen en adressen komen
# als constanten uit dg-gemeenschap.php (omgeving).
wp option update openid_connect_generic_settings --format=json \
  '{"identity_key":"preferred_username","nickname_key":"preferred_username","email_format":"{email}","displayname_format":"{given_name} {family_name}","identify_with_username":false,"token_refresh_enable":1,"http_request_timeout":10}' >/dev/null

# BuddyPress: leden, profielen, groepen, activiteit, berichten, meldingen.
wp option update bp-active-components --format=json \
  '{"members":1,"xprofile":1,"groups":1,"activity":1,"settings":1,"notifications":1,"messages":1}' >/dev/null
wp eval 'bp_core_add_page_mappings( bp_get_option( "bp-active-components" ), "keep" );' >/dev/null
log "BuddyPress-onderdelen en -pagina's"

# --- CiviCRM ------------------------------------------------------------------
# Een nieuwe versie: CIVICRM_VERSIE in .env ophogen en bijwerken.sh draaien.
GEINSTALLEERD="$(wp plugin get civicrm --field=version 2>/dev/null || true)"
if [ "$GEINSTALLEERD" != "$CIVICRM_VERSIE" ]; then
  log "CiviCRM $CIVICRM_VERSIE ophalen${GEINSTALLEERD:+ (was $GEINSTALLEERD)}"
  wp plugin install "https://download.civicrm.org/civicrm-${CIVICRM_VERSIE}-wordpress.zip" --force >/dev/null
fi
php -r 'copy("https://download.civicrm.org/cv/cv.phar", "/tmp/cv") or exit(1);' && chmod +x /tmp/cv
wp plugin activate civicrm >/dev/null 2>&1 || true
if ! /tmp/cv --cwd=/var/www/html ev 'return 1;' >/dev/null 2>&1; then
  /tmp/cv core:install --cwd=/var/www/html --cms-base-url="https://$DOMEIN" --lang=nl_NL >/dev/null
  log "CiviCRM geïnstalleerd"
else
  /tmp/cv upgrade:db --cwd=/var/www/html >/dev/null 2>&1 || true
  log "CiviCRM bijgewerkt"
fi
# Naam en e-mail van WordPress-gebruikers en CiviCRM-contacten gelijk houden.
if wp plugin is-installed civicrm-wp-profile-sync; then wp plugin update civicrm-wp-profile-sync >/dev/null 2>&1 || true; else wp plugin install civicrm-wp-profile-sync >/dev/null 2>&1 || true; fi
wp plugin activate civicrm-wp-profile-sync >/dev/null 2>&1 || true

# --- Thema: BuddyX met ons kindthema (wordpress/thema/) ------------------------
# Een vaste versie; een nieuwe kies je met BUDDYX_VERSIE in compose.yml.
if [ "$(wp theme get buddyx --field=version 2>/dev/null || true)" != "$BUDDYX_VERSIE" ]; then
  wp theme install buddyx --version="$BUDDYX_VERSIE" --force >/dev/null
fi
wp theme activate digitaalgemeengoed >/dev/null
log "Thema Digitaal GemeenGoed (BuddyX $BUDDYX_VERSIE)"

# --- Onze inrichting: fora, profielvelden, adressen ----------------------------
wp dg inrichten
wp rewrite flush >/dev/null
wp cache flush >/dev/null 2>&1 || true
log "Klaar"
