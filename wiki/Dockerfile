# De kennisbank: Wiki.js 2 (AGPL-3.0) met de vertalingen in de image, het logo
# van Digitaal GemeenGoed en offline-modus. Offline: de wiki haalt geen
# vertalingen of updates op bij de dienst van Wiki.js; wat hij nodig heeft,
# zit in de image. Gebouwd vanuit de hoofdmap van de repo (compose.yml).
ARG WIKI_VERSIE=2
FROM ghcr.io/requarks/wiki:${WIKI_VERSIE}
# De vertalingen (Requarks/wiki-localization), vastgezet op één commit.
ARG VERTALINGEN=c4803134075eaa6a8806add09aaa8e9c9404a836
USER root
RUN set -eux; \
    mkdir -p /wiki/data/sideload /wiki/assets/dgg; \
    cd /wiki/data/sideload; \
    for f in locales.json en.json nl.json; do \
      wget -q -O "$f" "https://raw.githubusercontent.com/Requarks/wiki-localization/${VERTALINGEN}/$f"; \
    done; \
    node -e 'const fs=require("fs"),f="locales.json",d=JSON.parse(fs.readFileSync(f));d.data.localization.locales=d.data.localization.locales.filter(l=>["en","nl"].includes(l.code));fs.writeFileSync(f,JSON.stringify(d))'; \
    chown -R node:node /wiki/data
COPY --chown=node:node wordpress/thema/digitaalgemeengoed/assets/icoon.png /wiki/assets/dgg/icoon.png
COPY --chown=node:node wiki/config.yml /wiki/config.yml
USER node
