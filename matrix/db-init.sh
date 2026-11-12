#!/bin/sh
# Eén keer, bij een lege database: een eigen database voor MAS (het inloggen
# van de chat). Synapse heeft de database "synapse" (POSTGRES_DB).
set -e
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<SQL
CREATE ROLE mas LOGIN PASSWORD '$MAS_DB_WACHTWOORD';
CREATE DATABASE mas OWNER mas ENCODING 'UTF8' LC_COLLATE 'C' LC_CTYPE 'C' TEMPLATE template0;
SQL
