#!/usr/bin/env bash
# Schrijft de stand van de server naar een klein JSON-bestand voor het
# beheerscherm (beheer.<domein>, pagina Platform): de versie, de diensten,
# de laatste back-up, de schijf en het geheugen. De beheerdienst leest het
# bestand alleen (gekoppeld als alleen-lezen); zo hoeft die niet bij Docker.
# Draait elke vijf minuten via de systemd-timer dg-status.

HIER="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
. "$HIER/lib.sh"
moet_root

MAP="$DG_STAAT/status"
install -d -m 755 "$MAP"

# JSON-tekst: aanhalingstekens en backslashes ontsnappen, stuurtekens weg.
js() { local s="${1//\\/\\\\}"; s="${s//\"/\\\"}"; s="$(printf '%s' "$s" | tr -d '\000-\037')"; printf '"%s"' "$s"; }
lijst() { local eerst=1 r; printf '['; while IFS= read -r r; do [ -n "$r" ] || continue; [ $eerst = 1 ] || printf ','; eerst=0; js "$r"; done; printf ']'; }

# Diensten: docker compose ps geeft (afhankelijk van de versie) een JSON-lijst
# of één JSON-object per regel.
diensten="$(dc ps -a --format json 2>/dev/null || true)"
case "$diensten" in
  \[*) ;;
  "") diensten="[]" ;;
  *) diensten="[$(printf '%s\n' "$diensten" | grep -v '^$' | paste -sd, -)]" ;;
esac

backup_map="$(find "${DG_BACKUPS:-/var/backups/dg-gemeenschap}" -mindepth 1 -maxdepth 1 -type d -printf '%T@ %p\n' 2>/dev/null | sort -n | tail -1 | cut -d' ' -f2- || true)"
backup_tijd=""; backup_bytes=0
if [ -n "$backup_map" ]; then
  backup_tijd="$(date -Iseconds -r "$backup_map")"
  backup_bytes="$(du -sb "$backup_map" | cut -f1)"
fi
spiegel_bytes="$(du -sb "$DOC_SPIEGEL" 2>/dev/null | cut -f1 || true)"
read -r schijf_totaal schijf_vrij < <(df -B1 --output=size,avail / | tail -1)
geheugen_totaal="$(awk '/^MemTotal:/{print $2*1024}' /proc/meminfo)"
geheugen_vrij="$(awk '/^MemAvailable:/{print $2*1024}' /proc/meminfo)"

tijdelijk="$(mktemp "$MAP/status.XXXXXX")"
{
  printf '{"tijd":%s' "$(js "$(date -Iseconds)")"
  printf ',"infra":%s' "$(js "$(cat "$DG_STAAT/infra" 2>/dev/null || echo onbekend)")"
  printf ',"inhoud":%s' "$(js "$(cut -c1-12 "$DG_STAAT/inhoud" 2>/dev/null || true)")"
  if [ -f "$DG_STAAT/pauze" ]; then printf ',"pauze":true'; else printf ',"pauze":false'; fi
  printf ',"mislukt":%s' "$(lijst < <(cat "$DG_STAAT/mislukt" 2>/dev/null || true))"
  printf ',"uitrol":%s' "$(lijst < <(tail -n 10 "$DG_STAAT/logboek" 2>/dev/null || true))"
  printf ',"diensten":%s' "$diensten"
  printf ',"backup":{"map":%s,"tijd":%s,"bytes":%s}' "$(js "$(basename "${backup_map:-}")")" "$(js "${backup_tijd:-0001-01-01T00:00:00Z}")" "${backup_bytes:-0}"
  printf ',"spiegel_bytes":%s' "${spiegel_bytes:-0}"
  printf ',"schijf":{"totaal":%s,"vrij":%s}' "${schijf_totaal:-0}" "${schijf_vrij:-0}"
  printf ',"geheugen":{"totaal":%s,"beschikbaar":%s}' "${geheugen_totaal:-0}" "${geheugen_vrij:-0}"
  printf ',"opgestart":%s}\n' "$(js "$(uptime -s 2>/dev/null || true)")"
} > "$tijdelijk"
chmod 644 "$tijdelijk"
mv "$tijdelijk" "$MAP/status.json"
