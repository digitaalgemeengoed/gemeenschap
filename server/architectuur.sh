#!/usr/bin/env bash
# Bouwt de HTML-rapporten van de architectuurmodellen voor
# architectuur.<domein>. Draait elk kwartier (systemd-timer dg-architectuur)
# en bouwt alleen een model dat sinds de vorige keer is gewijzigd.
#
#   sudo /opt/gemeenschap/server/architectuur.sh            nu bouwen wat nieuw is
#   sudo /opt/gemeenschap/server/architectuur.sh --opnieuw  alles opnieuw bouwen
#
# De lijst met modellen staat in architectuur/repos.txt op main. Elk model is
# een Archi-repo in het formaat van coArchi; het rapport maakt Archi zelf (de
# image archimate-ci-image, zonder netwerk). De rapporten staan in
# /srv/dg-architectuur; Caddy toont ze alleen aan leden (oauth2-proxy).

HIER="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
. "$HIER/lib.sh"
moet_root
cd "$DG_MAP" || exit 1
[ -s .env ] || stop "Nog niet geïnstalleerd."

BRON="$DG_STAAT/architectuur"
SITE=/srv/dg-architectuur
IMAGE="woozymasta/archimate-ci-image:$(sed -n 's/^ARCHI_CI_VERSIE=//p' .env | grep . || echo 5.7.0-1.0.6)"
TOKEN="$(sed -n 's/^ARCH_GITHUB_TOKEN=//p' .env)"
OPNIEUW=0; if [ "${1:-}" = --opnieuw ]; then OPNIEUW=1; fi

install -d -m 700 "$DG_STAAT" "$BRON"
install -d -m 755 "$SITE"
exec 8>"$DG_STAAT/architectuur.slot"
flock -n 8 || { echo "Er wordt al gebouwd."; exit 0; }
export GIT_TERMINAL_PROMPT=0
noteer() { printf '%s architectuur: %s\n' "$(date -Iseconds)" "$*" >> "$DG_STAAT/logboek"; echo "  $*"; }
# Een fout per model maar één keer in het logboek, niet elk kwartier opnieuw.
fout() {
  local naam="$1"; shift
  if [ "$(cat "$BRON/$naam.melding" 2>/dev/null)" != "$*" ]; then noteer "$naam: $*"; printf '%s' "$*" > "$BRON/$naam.melding"; else echo "  $naam: $*"; fi
}

# De lijst van main (uitrol.sh haalt elke vijf minuten op); anders die hier.
LIJST="$(git show origin/main:architectuur/repos.txt 2>/dev/null || cat architectuur/repos.txt 2>/dev/null || true)"

# Git met het token als de repo op GitHub staat en er een token is; het token
# komt niet in .git/config.
git_bron() {
  local url="$1"; shift
  if [ -n "$TOKEN" ] && [[ "$url" == https://github.com/* ]]; then
    git -c http.extraHeader="Authorization: Basic $(printf 'x-access-token:%s' "$TOKEN" | base64 -w0)" "$@"
  else
    git "$@"
  fi
}

html() { sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g' -e 's/"/\&quot;/g' <<<"$1"; }

: > "$BRON/.lijst.nieuw"
while read -r naam url titel; do
  [[ -z "$naam" || "$naam" == \#* ]] && continue
  if ! [[ "$naam" =~ ^[a-z0-9][a-z0-9-]{0,40}$ ]] || ! [[ "$url" =~ ^(https://|git@)[A-Za-z0-9._@:/-]+$ ]]; then
    noteer "regel overgeslagen (naam of adres klopt niet): $naam"; continue
  fi
  titel="${titel:-$naam}"
  map="$BRON/$naam"
  if [ -d "$map/.git" ] && [ "$(git -C "$map" remote get-url origin)" = "$url" ]; then
    git_bron "$url" -C "$map" fetch --quiet --depth 1 origin HEAD || { fout "$naam" "ophalen mislukt"; continue; }
    git -C "$map" reset --quiet --hard FETCH_HEAD
  else
    rm -rf "$map"
    git_bron "$url" clone --quiet --depth 1 "$url" "$map" 2>/dev/null || { fout "$naam" "klonen mislukt (bestaat de repo, is hij privé zonder ARCH_GITHUB_TOKEN?)"; continue; }
  fi
  commit="$(git -C "$map" rev-parse HEAD)"
  datum="$(git -C "$map" log -1 --format=%cs)"
  printf '%s\n' "$naam|$titel|$url|$datum" >> "$BRON/.lijst.nieuw"

  if [ "$OPNIEUW" = 0 ] && [ -f "$SITE/$naam/index.html" ] && [ "$(cat "$map.commit" 2>/dev/null)" = "$commit" ]; then
    continue
  fi
  if [ "$OPNIEUW" = 0 ] && [ "$(cat "$map.mislukt" 2>/dev/null)" = "$commit" ]; then
    continue  # deze versie lukte al niet; pas weer bij een nieuwe commit
  fi
  if [ ! -f "$map/model/folder.xml" ]; then
    noteer "$naam: geen coArchi-model gevonden (model/folder.xml ontbreekt)"; echo "$commit" > "$map.mislukt"; continue
  fi

  werk="$(mktemp -d /var/tmp/dg-archi.XXXXXX)"
  cp -a "$map" "$werk/project"
  install -d -m 777 "$werk/report"
  if docker run --rm --network none --memory 2g \
       -v "$werk/project:/archi/project" -v "$werk/report:/archi/report" \
       -e ARCHI_HTML_REPORT_ENABLED=true -e ARCHI_JASPER_REPORT_ENABLED=false \
       -e ARCHI_CSV_REPORT_ENABLED=false -e ARCHI_EXPORT_MODEL_ENABLED=false \
       "$IMAGE" >"$werk/log" 2>&1 && [ -f "$werk/report/html/index.html" ]; then
    rm -rf "$SITE/$naam.nieuw" "$SITE/$naam.oud"
    mv "$werk/report/html" "$SITE/$naam.nieuw"
    chmod -R a+rX,go-w "$SITE/$naam.nieuw"
    if [ -d "$SITE/$naam" ]; then mv "$SITE/$naam" "$SITE/$naam.oud"; fi
    mv "$SITE/$naam.nieuw" "$SITE/$naam"
    rm -rf "$SITE/$naam.oud"
    echo "$commit" > "$map.commit"; rm -f "$map.mislukt" "$map.melding"
    noteer "$naam: rapport gebouwd ($(git -C "$map" log -1 --format='%h %s' | cut -c1-80))"
  else
    echo "$commit" > "$map.mislukt"
    noteer "$naam: rapport bouwen mislukt; zie hieronder"
    tail -n 15 "$werk/log" | sed 's/^/      /'   # de vorige versie van het rapport blijft staan
  fi
  rm -rf "$werk"
done <<<"$LIJST"
mv "$BRON/.lijst.nieuw" "$BRON/lijst"

# Rapporten van modellen die niet meer in de lijst staan, weghalen.
for d in "$SITE"/*/; do
  [ -d "$d" ] || continue
  n="$(basename "$d")"
  if ! grep -q "^$n|" "$BRON/lijst"; then rm -rf "$d"; noteer "$n: niet meer in de lijst, rapport weggehaald"; fi
done

# --- De overzichtspagina ------------------------------------------------------
{
  cat <<'KOP'
<!doctype html>
<html lang="nl">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>Architectuurmodellen · Digitaal GemeenGoed</title>
<style>
  :root { --marine: #0a2654; --blauw: #14478f; --link: #1a56a8; --rand: #dbe4ee; --grijs: #4a5a6c; }
  * { box-sizing: border-box; }
  body { margin: 0; font: 16px/1.55 system-ui, -apple-system, "Segoe UI", Roboto, sans-serif; color: #1c2733; background: #f5f8fb; }
  header { background: var(--marine); color: #fff; padding: 1.4rem 1rem; }
  header div, main { max-width: 64rem; margin: 0 auto; }
  header h1 { margin: 0; font-size: 1.5rem; font-weight: 800; }
  header p { margin: .3rem 0 0; opacity: .85; }
  main { padding: 2rem 1rem 3rem; }
  .modellen { display: grid; grid-template-columns: repeat(auto-fit, minmax(17rem, 1fr)); gap: 1.25rem; padding: 0; list-style: none; }
  .modellen li { background: #fff; border: 1px solid var(--rand); border-radius: 14px; padding: 1.2rem 1.3rem; }
  .modellen h2 { margin: 0 0 .4rem; font-size: 1.15rem; }
  .modellen h2 a { color: var(--marine); text-decoration: none; }
  .modellen h2 a:hover { color: var(--link); text-decoration: underline; }
  .modellen p { margin: 0; color: var(--grijs); font-size: .92rem; }
  .modellen p a { color: var(--link); }
  .leeg { background: #fff; border: 1px dashed var(--rand); border-radius: 14px; padding: 1.5rem; color: var(--grijs); }
  footer { max-width: 64rem; margin: 0 auto; padding: 0 1rem 2rem; color: var(--grijs); font-size: .9rem; }
  footer a { color: var(--link); }
</style>
</head>
<body>
<header><div><h1>Architectuurmodellen</h1><p>De ArchiMate-modellen van de projecten van Digitaal GemeenGoed, alleen voor leden.</p></div></header>
<main>
KOP
  if [ -s "$BRON/lijst" ]; then
    echo '<ul class="modellen">'
    while IFS='|' read -r naam titel url datum; do
      [ -f "$SITE/$naam/index.html" ] || continue
      printf '<li><h2><a href="%s/">%s</a></h2><p>Bijgewerkt op %s · <a href="%s">model op GitHub</a></p></li>\n' \
        "$(html "$naam")" "$(html "$titel")" "$(html "$datum")" "$(html "${url%.git}")"
    done < "$BRON/lijst"
    echo '</ul>'
  else
    echo '<p class="leeg">Er staan nog geen modellen. Een model komt erbij met een regel in <code>architectuur/repos.txt</code> in de repo gemeenschap.</p>'
  fi
  cat <<'VOET'
</main>
<footer>Uitleg bij de modellen staat in de kennisbank. Elk model wordt elk kwartier bijgewerkt vanuit GitHub; het rapport maakt Archi.</footer>
</body>
</html>
VOET
} > "$SITE/index.html.nieuw"
chmod 644 "$SITE/index.html.nieuw"
mv "$SITE/index.html.nieuw" "$SITE/index.html"
