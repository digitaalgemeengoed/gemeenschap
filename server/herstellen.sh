#!/usr/bin/env bash
# Een back-up terugzetten: de databases en bestanden uit één map in
# /var/backups/dg-gemeenschap (gemaakt door back-up.sh, elke nacht en vlak
# voor elke nieuwe versie).
#
#   sudo /opt/gemeenschap/server/herstellen.sh                       welke back-ups er zijn
#   sudo /opt/gemeenschap/server/herstellen.sh <map> [onderdelen]    terugzetten
#   sudo /opt/gemeenschap/server/herstellen.sh --proef <map> [...]   alleen laten zien wat er zou gebeuren
#
# Onderdelen: keycloak (leden, groepen, organisaties), wordpress (site, fora,
# CiviCRM, uploads), redmine (meldingen, bestanden), kennisbank (wiki, bestanden),
# documenten (het register; ontbrekende objecten komen uit de spiegel in
# /var/backups/dg-gemeenschap-objecten), beheer (het logboek van beheer;
# samengevoegd, er gaat niets verloren).
# Zonder onderdelen vraagt het per onderdeel.
#
# Eerst maakt het een back-up van nu, zodat ook het herstellen ongedaan kan.
# Tijdens het herstellen staat de automatische uitrol op pauze.
#
# Let op bij de kennisbank: staat de Git-opslag aan, dan is de repo kennisbank
# op GitHub ook een bron. Na het herstellen haalt de wiki bij de volgende
# synchronisatie (binnen vijf minuten) de nieuwere pagina's weer uit GitHub.
# Wil je echt terug, zet de Git-opslag dan eerst uit (KENNISBANK_REPO leeg in
# .env, bijwerken.sh) of zet de repo ook terug.

HIER="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
. "$HIER/lib.sh"
moet_root
cd "$DG_MAP" || exit 1
[ -s .env ] || stop "Nog niet geïnstalleerd."

BACKUPS="${DG_BACKUPS:-/var/backups/dg-gemeenschap}"
PROEF=0
if [ "${1:-}" = --proef ]; then PROEF=1; shift; fi

# doe: voert uit, of laat in de proef alleen zien wat er zou gebeuren.
doe() {
  if [ "$PROEF" = 1 ]; then printf '    (proef) %s\n' "$*"; else eval "$@"; fi
}

if [ -z "${1:-}" ]; then
  meld "Back-ups in $BACKUPS"
  for d in $(ls -1d "$BACKUPS"/*/ 2>/dev/null | sort -r); do
    printf '  %-20s %6s   %s\n' "$(basename "$d")" "$(du -sh "$d" | cut -f1)" "$(cd "$d" && ls | tr '\n' ' ')"
  done
  echo
  echo "  Terugzetten: sudo $DG_MAP/server/herstellen.sh <map> [keycloak] [wordpress] [redmine] [kennisbank] [documenten] [beheer]"
  echo "  Eerst kijken wat er zou gebeuren: sudo $DG_MAP/server/herstellen.sh --proef <map>"
  exit 0
fi

MAP="$1"; shift
[[ "$MAP" == /* ]] || MAP="$BACKUPS/$MAP"
MAP="${MAP%/}"
[ -d "$MAP" ] || stop "Geen back-up in $MAP. Zonder map erachter zie je welke er zijn."

# Welke onderdelen, en staan hun bestanden in de back-up?
declare -A BESTANDEN=(
  [keycloak]="keycloak.dump"
  [wordpress]="wordpress.sql.gz wordpress-uploads.tar.gz"
  [redmine]="redmine.dump redmine-bestanden.tar.gz"
  [kennisbank]="kennisbank.dump kennisbank-bestanden.tar.gz"
  [documenten]="documenten.dump"
  [beheer]="beheer-logboek.jsonl"
)
ALLE=(keycloak wordpress redmine kennisbank documenten beheer)
GEKOZEN=()
if [ $# -gt 0 ]; then
  for o in "$@"; do
    [[ -n "${BESTANDEN[$o]:-}" ]] || stop "Onbekend onderdeel: $o (kies uit: ${ALLE[*]})."
    GEKOZEN+=("$o")
  done
else
  for o in "${ALLE[@]}"; do
    [ -f "$MAP/${BESTANDEN[$o]%% *}" ] || continue
    if [ "$PROEF" = 1 ] || vraag_ja "  $o terugzetten uit $(basename "$MAP")?"; then GEKOZEN+=("$o"); fi
  done
fi
[ ${#GEKOZEN[@]} -gt 0 ] || stop "Niets gekozen; niets gedaan."
for o in "${GEKOZEN[@]}"; do
  for b in ${BESTANDEN[$o]}; do
    [ -s "$MAP/$b" ] || stop "$o: $b ontbreekt of is leeg in $MAP; niets gedaan."
    # Is het bestand heel? Liever nu stoppen dan halverwege.
    case "$b" in
      *.gz) gzip -t "$MAP/$b" 2>/dev/null || stop "$o: $b is beschadigd; niets gedaan." ;;
      *.dump) [ "$(head -c 5 "$MAP/$b")" = PGDMP ] || stop "$o: $b is geen PostgreSQL-dump; niets gedaan." ;;
    esac
  done
done

meld "Terugzetten uit $(basename "$MAP"): ${GEKOZEN[*]}"
echo "  Wat er nu in die onderdelen staat, wordt vervangen door de stand van de back-up."
if [ "$PROEF" = 0 ]; then
  read -r -p "  Typ 'herstel' om door te gaan: " a
  [ "$a" = herstel ] || stop "Niets gedaan."
fi

# Tijdens het herstellen geen uitrol.
WAS_GEPAUZEERD=0; [ -f "$DG_STAAT/pauze" ] && WAS_GEPAUZEERD=1
doe "install -d -m 700 '$DG_STAAT' && touch '$DG_STAAT/pauze'"
hervat() { if [ "$WAS_GEPAUZEERD" = 0 ] && [ "$PROEF" = 0 ]; then rm -f "$DG_STAAT/pauze"; fi; }
trap hervat EXIT

meld "Eerst een back-up van nu"
doe "'$DG_MAP/server/back-up.sh'"

# Een PostgreSQL-dump (pg_dump -Fc) terugzetten: alles of niets.
pg_terug() {
  local dienst="$1" gebruiker="$2" db="$3" bestand="$4"
  doe "dc exec -T '$dienst' pg_restore -U '$gebruiker' -d '$db' --clean --if-exists --no-owner --single-transaction --exit-on-error < '$bestand'"
}

for o in "${GEKOZEN[@]}"; do
  meld "$o"
  case "$o" in
    keycloak)
      doe "dc stop keycloak >/dev/null"
      pg_terug keycloak-db keycloak keycloak "$MAP/keycloak.dump"
      doe "dc start keycloak >/dev/null"
      ;;
    wordpress)
      doe "dc exec -T wordpress sh -c 'rm -rf /var/www/html/wp-content/uploads && tar -C /var/www/html -xzf - && chown -R www-data:www-data /var/www/html/wp-content/uploads' < '$MAP/wordpress-uploads.tar.gz'"
      doe "dc stop wordpress >/dev/null"
      # De dump bevat de hele database (mariadb-dump --databases wordpress).
      doe "dc exec -T wp-db sh -c 'exec mariadb -uroot -p\"\$MARIADB_ROOT_PASSWORD\" -e \"DROP DATABASE IF EXISTS wordpress\"'"
      doe "gunzip -c '$MAP/wordpress.sql.gz' | dc exec -T wp-db sh -c 'exec mariadb -uroot -p\"\$MARIADB_ROOT_PASSWORD\"'"
      doe "dc start wordpress >/dev/null"
      ;;
    redmine)
      doe "dc exec -T redmine sh -c 'rm -rf /usr/src/redmine/files && tar -C /usr/src/redmine -xzf -' < '$MAP/redmine-bestanden.tar.gz'"
      doe "dc stop redmine redmine-groepen >/dev/null"
      pg_terug redmine-db redmine redmine "$MAP/redmine.dump"
      doe "dc start redmine redmine-groepen >/dev/null"
      ;;
    kennisbank)
      doe "dc exec -T wiki sh -c 'rm -rf /wiki/data/content/* && tar -C /wiki/data -xzf -' < '$MAP/kennisbank-bestanden.tar.gz'"
      doe "dc stop wiki >/dev/null"
      pg_terug wiki-db wiki wiki "$MAP/kennisbank.dump"
      doe "dc start wiki >/dev/null"
      if grep -qE '^KENNISBANK_REPO=.+' .env; then
        waarschuw "De Git-opslag staat aan: de wiki haalt nieuwere pagina's uit GitHub binnen vijf minuten terug (zie de uitleg bovenin dit script)."
      fi
      ;;
    documenten)
      # Het register terug; objecten blijven in de opslag staan (ze veranderen
      # nooit). Wat het register kent maar de opslag mist, komt uit de spiegel.
      doe "dc stop documenten >/dev/null"
      pg_terug documenten-db documenten documenten "$MAP/documenten.dump"
      doe "dc run --rm -T --no-deps -u 0 -v '$DOC_SPIEGEL:/spiegel:ro' documenten terugspiegelen /spiegel"
      doe "dc start documenten >/dev/null"
      ;;
    beheer)
      # Samenvoegen: regels die er al staan blijven, ontbrekende komen terug.
      doe "dc exec -T beheer dg-beheer logboek-terug < '$MAP/beheer-logboek.jsonl'"
      ;;
  esac
  if [ "$PROEF" = 1 ]; then ok "$o zou worden teruggezet"; else ok "$o teruggezet"; fi
done

if [ "$PROEF" = 1 ]; then
  echo; ok "Proef: er is niets veranderd."
  exit 0
fi
printf '%s herstellen: %s uit %s\n' "$(date -Iseconds)" "${GEKOZEN[*]}" "$(basename "$MAP")" >> "$DG_STAAT/logboek"
wacht "de site" 24 gezond || waarschuw "Nog niet alles antwoordt; kijk met: sudo $DG_MAP/server/controleer.sh"
ok "Klaar. De back-up van vlak hiervoor staat ook in $BACKUPS, voor als je dit ongedaan wilt maken."
