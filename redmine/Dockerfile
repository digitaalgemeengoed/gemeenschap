# Redmine met inloggen via Keycloak (plugin redmine_oauth van Kontron, GPL-3.0).
ARG REDMINE_VERSIE=6.1.6
FROM redmine:${REDMINE_VERSIE}
ARG REDMINE_OAUTH_VERSIE=v4.2.3
USER root
RUN set -eux; \
    apt-get update; apt-get install -y --no-install-recommends git build-essential libpq-dev; \
    git clone --depth 1 --branch "${REDMINE_OAUTH_VERSIE}" https://github.com/kontron/redmine_oauth.git plugins/redmine_oauth; \
    rm -rf plugins/redmine_oauth/.git; \
    bundle config set --local without 'development test'; \
    bundle install --jobs 4; \
    apt-get purge -y --auto-remove build-essential git; rm -rf /var/lib/apt/lists/*; \
    chown -R redmine:redmine plugins
# Mail via de SMTP-dienst uit .env (configuration.yml wordt met ERB gelezen).
COPY --chown=redmine:redmine configuration.yml config/configuration.yml
COPY --chown=redmine:redmine inrichten.rb /usr/local/share/dg/inrichten.rb
USER redmine
