#!/usr/bin/env bash
# Chat (Matrix): de configuratie van Synapse, MAS en Element maken uit .env, en
# eenmalig het account waarmee Documenten de kamers bijhoudt. Wordt gelezen
# door bijwerken.sh (functies matrix_bestanden en matrix_bot).
#
#   matrix.<domein>       Synapse (de chatserver), buiten de afscherming
#   matrix-auth.<domein>  MAS: inloggen, alleen via Keycloak
#   chat.<domein>         Element Web (de chat in de browser)
#
# Gebruikers heten @naam:<domein>. Federatie (praten met andere
# Matrix-servers) staat uit, tenzij MATRIX_FEDERATIE in .env iets zegt:
#   leeg                       dicht (standaard)
#   *                          open
#   gemeente.nl,andere.nl      alleen met deze servers
#
# De gegenereerde bestanden staan in $DG_STAAT/matrix (alleen root kan erin):
# synapse/ (homeserver.yaml en log.yaml), mas.yaml, element.json en de
# sleutels van MAS (sleutels/, één keer gemaakt; weg = iedereen opnieuw inloggen).

# Vaste id van Keycloak als bron in MAS (een ULID); staat ook in
# keycloak/realm/gemeenschap.json (redirect-URI van de client matrix-auth).
MAS_KEYCLOAK_ID=01JCQ8DGGKC0RS1M0000000000

# Een waarde uit .env (zonder het bestand als script uit te voeren).
env_waarde() { sed -n "s/^$1=//p" "$DG_MAP/.env" | tail -n1; }
# Voor tussen dubbele aanhalingstekens in YAML.
yaml_tekst() { printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'; }

matrix_bestanden() {
  local m="$DG_STAAT/matrix"
  local DOMEIN MATRIX_FEDERATIE SMTP_HOST SMTP_POORT SMTP_GEBRUIKER SMTP_WACHTWOORD SMTP_AFZENDER
  local MATRIX_DB_WACHTWOORD MAS_DB_WACHTWOORD MATRIX_MAS_GEHEIM MAS_VERSLEUTEL MATRIX_KC_SECRET
  for v in DOMEIN MATRIX_FEDERATIE SMTP_HOST SMTP_POORT SMTP_GEBRUIKER SMTP_AFZENDER \
           MATRIX_DB_WACHTWOORD MAS_DB_WACHTWOORD MATRIX_MAS_GEHEIM MAS_VERSLEUTEL MATRIX_KC_SECRET; do
    printf -v "$v" '%s' "$(env_waarde "$v")"
  done
  SMTP_WACHTWOORD="$(yaml_tekst "$(env_waarde SMTP_WACHTWOORD)")"
  SMTP_GEBRUIKER="$(yaml_tekst "$SMTP_GEBRUIKER")"
  install -d -m 700 "$m"
  install -d -m 755 "$m/sleutels" # de map erboven is alleen voor root
  install -d -m 755 "$m/synapse"
  # De sleutels van MAS (ondertekenen van tokens): één keer maken.
  [ -s "$m/sleutels/rsa.pem" ] || openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$m/sleutels/rsa.pem" 2>/dev/null
  [ -s "$m/sleutels/ec.pem" ] || openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out "$m/sleutels/ec.pem" 2>/dev/null
  chmod 644 "$m/sleutels/"*.pem # de container (andere gebruiker) moet ze kunnen lezen; de map zelf is alleen voor root

  local federatie
  case "${MATRIX_FEDERATIE:-}" in
    "") federatie="federation_domain_whitelist: []          # dicht: geen federatie" ;;
    "*") federatie="# federatie open" ;;
    *) federatie="federation_domain_whitelist: [$(printf '%s' "$MATRIX_FEDERATIE" | sed 's/[^A-Za-z0-9.,:-]//g; s/,/, /g')]" ;;
  esac
  local mail=""
  if [ -n "${SMTP_HOST:-}" ]; then
    mail="email:
  smtp_host: \"$SMTP_HOST\"
  smtp_port: ${SMTP_POORT:-587}
  smtp_user: \"${SMTP_GEBRUIKER:-}\"
  smtp_pass: \"${SMTP_WACHTWOORD:-}\"
  force_tls: $([ "${SMTP_POORT:-587}" = 465 ] && echo true || echo false)
  require_transport_security: true
  notif_from: \"Chat · Digitaal GemeenGoed <${SMTP_AFZENDER:-noreply@$DOMEIN}>\"
  app_name: Chat · Digitaal GemeenGoed
  enable_notifs: true
  notif_for_new_users: true
  client_base_url: https://chat.$DOMEIN
  notif_delay_before_mail: 15m"
  fi

  umask 022
  cat > "$m/synapse/homeserver.yaml" <<EOF
# Gemaakt door server/matrix.sh uit .env. Niet hier wijzigen.
server_name: "$DOMEIN"
public_baseurl: "https://matrix.$DOMEIN/"
pid_file: /data/homeserver.pid
listeners:
  - port: 8008
    tls: false
    type: http
    x_forwarded: true
    bind_addresses: ["0.0.0.0"]
    resources:
      - names: [client, federation]
        compress: false
database:
  name: psycopg2
  args:
    user: synapse
    password: "$MATRIX_DB_WACHTWOORD"
    database: synapse
    host: matrix-db
    cp_min: 5
    cp_max: 10
log_config: /config/log.yaml
media_store_path: /data/media_store
max_upload_size: 50M
signing_key_path: /data/signing.key
report_stats: false
trusted_key_servers: []
suppress_key_server_warning: true
$federatie
allow_public_rooms_over_federation: false
allow_public_rooms_without_auth: false
# Inloggen en accounts: alleen via MAS (en dus Keycloak).
enable_registration: false
password_config:
  enabled: false
matrix_authentication_service:
  enabled: true
  endpoint: "http://mas:8080/"
  secret: "$MATRIX_MAS_GEHEIM"
# Geen voorbeelden van links laden: dat zou de server naar willekeurige sites laten gaan.
url_preview_enabled: false
require_auth_for_profile_requests: true
user_directory:
  enabled: true
  search_all_users: true
  prefer_local_users: true
presence:
  enabled: true
$mail
EOF
  cat > "$m/synapse/log.yaml" <<'EOF'
version: 1
formatters:
  kort:
    format: "%(asctime)s %(levelname)s %(name)s: %(message)s"
handlers:
  console:
    class: logging.StreamHandler
    formatter: kort
loggers:
  synapse.storage.SQL:
    level: WARNING
root:
  level: INFO
  handlers: [console]
disable_existing_loggers: false
EOF

  local smtp="transport: blackhole"
  if [ -n "${SMTP_HOST:-}" ]; then
    smtp="transport: smtp
  mode: $([ "${SMTP_POORT:-587}" = 465 ] && echo tls || echo starttls)
  hostname: \"$SMTP_HOST\"
  port: ${SMTP_POORT:-587}
  username: \"${SMTP_GEBRUIKER:-}\"
  password: \"${SMTP_WACHTWOORD:-}\"
  from: '\"Chat · Digitaal GemeenGoed\" <${SMTP_AFZENDER:-noreply@$DOMEIN}>'
  reply_to: '\"Digitaal GemeenGoed\" <${SMTP_AFZENDER:-noreply@$DOMEIN}>'"
  fi
  local realm="https://aanmelden.$DOMEIN/realms/gemeenschap" intern="http://keycloak:8080/realms/gemeenschap/protocol/openid-connect"
  umask 077
  cat > "$m/mas.yaml" <<EOF
# Gemaakt door server/matrix.sh uit .env. Niet hier wijzigen.
http:
  public_base: "https://matrix-auth.$DOMEIN/"
  listeners:
    - name: web
      resources:
        - name: discovery
        - name: human
        - name: oauth
        - name: compat
        - name: graphql
        - name: assets
      binds:
        - address: "0.0.0.0:8080"
database:
  uri: "postgresql://mas:$MAS_DB_WACHTWOORD@matrix-db/mas"
secrets:
  encryption: "$MAS_VERSLEUTEL"
  keys:
    - kid: rsa1
      key_file: /sleutels/rsa.pem
    - kid: ec1
      key_file: /sleutels/ec.pem
matrix:
  kind: synapse
  homeserver: "$DOMEIN"
  endpoint: "http://synapse:8008/"
  secret: "$MATRIX_MAS_GEHEIM"
# Alleen inloggen via Keycloak; geen eigen wachtwoorden.
passwords:
  enabled: false
account:
  email_change_allowed: false
  displayname_change_allowed: true
  password_registration_enabled: false
email:
  $smtp
branding:
  service_name: "Chat · Digitaal GemeenGoed"
  policy_uri: "https://$DOMEIN/privacy/"
  tos_uri: "https://$DOMEIN/voorwaarden/"
upstream_oauth2:
  providers:
    - id: $MAS_KEYCLOAK_ID
      human_name: "je DGG-account"
      issuer: "$realm"
      # De browser gaat naar het openbare adres; tokens haalt MAS binnen het
      # Docker-netwerk op (de afscherming houdt verkeer van buiten tegen).
      discovery_mode: disabled
      authorization_endpoint: "$realm/protocol/openid-connect/auth"
      token_endpoint: "$intern/token"
      jwks_uri: "$intern/certs"
      pkce_method: always
      token_endpoint_auth_method: client_secret_basic
      client_id: matrix-auth
      client_secret: "$MATRIX_KC_SECRET"
      scope: "openid profile email"
      claims_imports:
        # Het e-mailadres is de gebruikersnaam in Keycloak; de chatnaam is wat
        # voor de @ staat (kan bij de eerste keer worden aangepast als die al bestaat).
        localpart:
          action: suggest
          template: "{{ (user.preferred_username | split('@'))[0] | lower }}"
        displayname:
          action: force
          template: "{{ user.name }}"
        email:
          action: force
          template: "{{ user.email }}"
EOF
  chmod 644 "$m/mas.yaml" # gelezen door de container; de map zelf is alleen voor root

  umask 022
  cat > "$m/element.json" <<EOF
{
  "default_server_config": {
    "m.homeserver": { "base_url": "https://matrix.$DOMEIN", "server_name": "$DOMEIN" }
  },
  "brand": "Chat · Digitaal GemeenGoed",
  "disable_custom_urls": true,
  "disable_guests": true,
  "disable_3pid_login": true,
  "default_country_code": "NL",
  "default_theme": "light",
  "show_labs_settings": false,
  "room_directory": { "servers": ["$DOMEIN"] },
  "integrations_ui_url": "",
  "integrations_rest_url": "",
  "integrations_widgets_urls": [],
  "privacy_policy_url": "https://$DOMEIN/privacy/",
  "terms_and_conditions_links": [
    { "url": "https://$DOMEIN/voorwaarden/", "text": "Voorwaarden" },
    { "url": "https://$DOMEIN/privacy/", "text": "Privacy" }
  ],
  "setting_defaults": {
    "UIFeature.registration": false,
    "UIFeature.passwordReset": false,
    "UIFeature.identityServer": false,
    "UIFeature.thirdPartyId": false,
    "UIFeature.voip": false,
    "UIFeature.widgets": false,
    "UIFeature.feedback": false,
    "UIFeature.shareSocial": false
  }
}
EOF
}

# Het account waarmee Documenten de kamers bijhoudt (@documenten:<domein>),
# met een token met beheerrechten in Synapse. Eén keer; daarna staat het
# token in .env (MATRIX_BOT_TOKEN).
matrix_bot() {
  grep -qE '^MATRIX_BOT_TOKEN=.+' .env && return 0
  wacht "MAS" 30 mas_klaar || { waarschuw "MAS draait niet; de kamers worden (nog) niet bijgehouden. Kijk met: sudo docker compose -f $DG_MAP/$COMPOSE logs mas"; return 0; }
  dc exec -T mas mas-cli manage register-user --username documenten --display-name "Digitaal GemeenGoed" --no-admin --yes >/dev/null 2>&1 || true
  local token
  token="$(dc exec -T mas mas-cli manage issue-compatibility-token documenten --device-id DOCUMENTEN --yes-i-want-to-grant-synapse-admin-privileges 2>&1 | grep -oE 'mct_[A-Za-z0-9_]+' | head -n1)"
  if [ -z "$token" ]; then
    waarschuw "Het token voor het chataccount kon niet worden gemaakt; de kamers worden (nog) niet bijgehouden. Zie README, Chat."
    return 0
  fi
  sed -i "s|^MATRIX_BOT_TOKEN=.*|MATRIX_BOT_TOKEN=$token|" .env
  grep -q '^MATRIX_BOT_TOKEN=' .env || echo "MATRIX_BOT_TOKEN=$token" >> .env
  ok "Chataccount @documenten:$DOMEIN klaar; Documenten houdt de kamers bij"
  dc up -d documenten >/dev/null
}

synapse_klaar() { dc exec -T synapse curl -fs -o /dev/null http://localhost:8008/health; }
mas_klaar() { dc exec -T synapse curl -fs -o /dev/null http://mas:8080/health; }
