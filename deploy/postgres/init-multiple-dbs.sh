#!/bin/sh
# Creates the two databases ULPF needs on top of the default postgres image,
# which only supports POSTGRES_DB=<one database> out of the box.
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "postgres" <<-EOSQL
    CREATE DATABASE ulpf_meta OWNER $POSTGRES_USER;
    CREATE DATABASE hive_metastore OWNER $POSTGRES_USER;
EOSQL
