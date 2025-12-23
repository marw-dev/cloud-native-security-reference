#!/bin/bash
set -e

echo "---> Konfiguriere MySQL Secrets Engine..."

vault secrets enable database 2>/dev/null || true

read -s -p "Gib das MySQL Root-Passwort ein: " DB_ROOT_PASS
echo ""

vault write database/config/athena-db \
    plugin_name=mysql-database-plugin \
    connection_url="{{username}}:{{password}}@tcp(db:3306)/" \
    allowed_roles="athena-role" \
    username="root" \
    password="$DB_ROOT_PASS"

vault write database/roles/athena-role \
    db_name=athena-db \
    creation_statements="CREATE USER '{{name}}'@'%' IDENTIFIED BY '{{password}}'; GRANT SELECT, INSERT, UPDATE, DELETE ON my_go_db.* TO '{{name}}'@'%';" \
    default_ttl="1h" \
    max_ttl="24h"

echo "--> Setup abgeschlossen. Vault kann nun User für MySQL erstellen."