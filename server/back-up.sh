#!/usr/bin/env bash
# Dagelijkse back-up van het gemeenschapsplatform: de vijf databases
# (Keycloak, WordPress met CiviCRM, Redmine, de kennisbank, de documenten), de
# uploads van WordPress en de bestanden van Redmine en de kennisbank. Bewaart
# 14 dagen in /var/backups/dg-gemeenschap. De documenten zelf (de objecten)
# staan in één spiegel: /var/backups/dg-gemeenschap-objecten.
# Draait elke nacht (cron) en vlak voor elke nieuwe versie (uitrol.sh).
#
# Kopieer beide mappen ook naar een plek buiten deze server (bijvoorbeeld met
# restic of rclone naar opslag bij een andere aanbieder). Een back-up op
# dezelfde server beschermt niet tegen het verlies van de server.
#
# Niet meegenomen: .env (de wachtwoorden). Bewaar die apart, in je wachtwoordmanager.

HIER="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
. "$HIER/lib.sh"
moet_root
cd "$DG_MAP" || exit 1

DOEL=/var/backups/dg-gemeenschap/$(date +%F-%H%M)
install -d -m 700 "$DOEL"
umask 077

dc exec -T keycloak-db pg_dump -U keycloak -Fc keycloak > "$DOEL/keycloak.dump"
dc exec -T redmine-db pg_dump -U redmine -Fc redmine > "$DOEL/redmine.dump"
dc exec -T wiki-db pg_dump -U wiki -Fc wiki > "$DOEL/kennisbank.dump"
dc exec -T wiki tar -C /wiki/data -czf - content > "$DOEL/kennisbank-bestanden.tar.gz"
# Wachtwoord via de omgeving van de container, niet op de opdrachtregel.
dc exec -T wp-db sh -c 'exec mariadb-dump -uroot -p"$MARIADB_ROOT_PASSWORD" --single-transaction --routines --triggers --databases wordpress' | gzip > "$DOEL/wordpress.sql.gz"
dc exec -T wordpress tar -C /var/www/html -czf - wp-content/uploads > "$DOEL/wordpress-uploads.tar.gz"
dc exec -T redmine tar -C /usr/src/redmine -czf - files > "$DOEL/redmine-bestanden.tar.gz"
# Documenten: eerst het register, dan de objecten. Objecten veranderen nooit;
# daarom één spiegel voor alle nachten, die alleen aangroeit (los van de
# mappen hierboven, die na 14 dagen verdwijnen).
dc exec -T documenten-db pg_dump -U documenten -Fc documenten > "$DOEL/documenten.dump"
install -d -m 700 "$DOC_SPIEGEL"
dc run --rm -T --no-deps -u 0 -v "$DOC_SPIEGEL:/spiegel" documenten spiegel /spiegel

find /var/backups/dg-gemeenschap -mindepth 1 -maxdepth 1 -type d -mtime +14 -exec rm -rf {} +
ok "Back-up in $DOEL ($(du -sh "$DOEL" | cut -f1))"
