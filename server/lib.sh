# Gedeelde hulpfuncties voor de serverscripts van het gemeenschapsplatform.
# Wordt ingelezen, niet gestart.
# shellcheck shell=bash

set -euo pipefail

# De map van deze repo op de server, en waar de uitrol bijhoudt wat er staat.
DG_MAP="${DG_MAP:-/opt/gemeenschap}"
DG_STAAT="${DG_STAAT:-/var/lib/dg-gemeenschap}"
# shellcheck disable=SC2034  # gebruikt in de scripts die dit inlezen
DG_REPO="${DG_REPO:-https://github.com/digitaalgemeengoed/gemeenschap}"
COMPOSE=compose.yml

meld()     { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
ok()       { printf '  \033[32m✓\033[0m %s\n' "$*"; }
waarschuw(){ printf '  \033[33m!\033[0m %s\n' "$*"; }
stop()     { printf '\n\033[1;31mGestopt:\033[0m %s\n\n' "$*" >&2; exit 1; }

moet_root() {
  [ "$(id -u)" -eq 0 ] || stop "Start dit script met sudo ervoor, bijvoorbeeld: sudo $0"
}

dc() { docker compose -f "$DG_MAP/$COMPOSE" --project-directory "$DG_MAP" --env-file "$DG_MAP/.env" "$@"; }

# Het publieke IPv4-adres: het bronadres van de route naar buiten.
publiek_ip() {
  ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i < NF; i++) if ($i == "src") { print $(i + 1); exit }}'
}

vraag_ja() {
  local a
  read -r -p "$1 [ja/nee] " a
  [[ "${a,,}" =~ ^(j|ja|y|yes)$ ]]
}

# Willekeurig geheim, alleen hexadecimale tekens: veilig in .env, JSON en URL's.
geheim() { openssl rand -hex "${1:-32}"; }

# Leest een KEY=waarde-bestand zonder het als script uit te voeren.
lees_env() {
  local bestand="$1" regel naam waarde
  [ -f "$bestand" ] || return 0
  while IFS= read -r regel || [ -n "$regel" ]; do
    regel="${regel%$'\r'}"
    [[ -z "$regel" || "$regel" == \#* ]] && continue
    [[ "$regel" =~ ^([A-Z][A-Z0-9_]*)=([A-Za-z0-9+/=.:_,@-]*)$ ]] || stop "Onverwachte regel in $bestand: $regel"
    naam="${BASH_REMATCH[1]}"
    waarde="${BASH_REMATCH[2]}"
    printf -v "$naam" '%s' "$waarde"
    export "${naam?}"
  done < "$bestand"
}

# Voegt KEY=waarde aan .env toe als KEY er nog niet in staat.
vul_env_aan() {
  local bestand="$1" naam="$2" waarde="$3"
  grep -q "^${naam}=" "$bestand" 2>/dev/null && return 0
  echo "${naam}=${waarde}" >> "$bestand"
  ok "Nieuwe instelling in .env: $naam"
}

# Zet KEY=waarde in .env: vervangt een bestaande regel of voegt toe.
zet_env() {
  local bestand="$1" naam="$2" waarde="$3" tijdelijk
  tijdelijk="$(mktemp "$bestand.XXXXXX")"
  grep -v "^${naam}=" "$bestand" > "$tijdelijk" || true
  printf '%s=%s\n' "$naam" "$waarde" >> "$tijdelijk"
  chmod 600 "$tijdelijk"
  mv "$tijdelijk" "$bestand"
}

geldig_domein() { [[ "$1" =~ ^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$ ]]; }

# Wacht tot een opdracht slaagt; $2 is het aantal pogingen van 5 seconden.
wacht() {
  local omschrijving="$1" n="$2"; shift 2
  printf '  wachten op %s ' "$omschrijving"
  for _ in $(seq "$n"); do
    if "$@" >/dev/null 2>&1; then echo; ok "$omschrijving antwoordt"; return 0; fi
    printf '.'; sleep 5
  done
  echo; return 1
}

keycloak_klaar()  { curl -fsS -o /dev/null http://127.0.0.1:8080/realms/master/.well-known/openid-configuration; }
wordpress_klaar() { dc exec -T wordpress curl -fsS -o /dev/null http://localhost/wp-includes/images/blank.gif; }
redmine_klaar()   { dc exec -T redmine ruby -rnet/http -e 'exit Net::HTTP.get_response(URI("http://localhost:3000/login")).code.to_i < 400'; }

# De nieuwste versie-tag (vX.Y.Z) die op main staat; leeg als er geen is.
nieuwste_tag() {
  git -C "$DG_MAP" tag -l 'v[0-9]*.[0-9]*.[0-9]*' --merged origin/main | sort -V | tail -1
}
