# Redmine met inloggen via Keycloak (plugin redmine_oauth van Kontron, GPL-3.0).
ARG REDMINE_VERSIE=6.1.6
FROM redmine:${REDMINE_VERSIE}
ARG REDMINE_OAUTH_VERSIE=v4.2.3
USER root
# Zoals de officiële image: de gems installeren met PostgreSQL als adapter en
# het Gemfile.lock per adapter bewaren. Bij het starten kopieert het entrypoint
# Gemfile.lock.postgresql naar Gemfile.lock; staan de gems van de plugin daar
# niet in, dan wil het bij elke start opnieuw installeren (en dat mislukte).
RUN set -eux; \
    apt-get update; apt-get install -y --no-install-recommends git build-essential libpq-dev; \
    git clone --depth 1 --branch "${REDMINE_OAUTH_VERSIE}" https://github.com/kontron/redmine_oauth.git plugins/redmine_oauth; \
    rm -rf plugins/redmine_oauth/.git; \
    echo "$RAILS_ENV:" > config/database.yml; \
    echo "  adapter: postgresql" >> config/database.yml; \
    bundle install --jobs 4; \
    cp Gemfile.lock Gemfile.lock.postgresql; \
    rm config/database.yml; \
    apt-get purge -y --auto-remove build-essential git; rm -rf /var/lib/apt/lists/*; \
    chown -R redmine:redmine plugins
# Mail via de SMTP-dienst uit .env (configuration.yml wordt met ERB gelezen).
COPY --chown=redmine:redmine configuration.yml config/configuration.yml
COPY --chown=redmine:redmine inrichten.rb /usr/local/share/dg/inrichten.rb
# Geen USER redmine: het entrypoint van de officiële image start als root,
# zet de rechten goed en gaat dan zelf over op de gebruiker redmine.
