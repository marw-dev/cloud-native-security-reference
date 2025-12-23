HashiCorp Vault Konfiguration für Aegis & Athena

Diese Dokumentation beschreibt die Einrichtung und Konfiguration von HashiCorp Vault für die Microservices Athena (Auth & Config Service) und Aegis (API Gateway).

Das System verwendet den AppRole-Authentifizierungsmechanismus, um Containern sicheren Zugriff auf Geheimnisse zu gewähren, ohne statische Tokens im Code zu hinterlegen.

1. Architektur & Übersicht

Vault dient als "Source of Truth" für zwei Arten von Geheimnissen:

Statische Geheimnisse (KV v2):

secret/data/athena/jwt: Enthält RSA-Schlüssel für JWT-Signierung und das Registrierungs-Secret.

secret/data/aegis/config: Enthält Konfigurationsdaten für das Gateway (falls verwendet).

Dynamische Geheimnisse (Database Engine):

database/creds/athena-role: Generiert temporäre Datenbank-Benutzer für Athena.

2. Voraussetzungen

Stellen Sie sicher, dass Vault läuft und entsperrt ("unsealed") ist. Sie benötigen das Root-Token oder ein Token mit entsprechenden Rechten.

```bash
export VAULT_ADDR='[http://127.0.0.1:8200](http://127.0.0.1:8200)'
export VAULT_TOKEN='<Ihr-Root-Token>'
```

3. Datenbank-Engine & Rollen Setup

Bevor die AppRoles funktionieren, muss die Datenbank-Engine konfiguriert sein.

3.1. MySQL-Verbindung konfigurieren

Verbindet Vault mit der MySQL-Datenbank unter Verwendung eines dedizierten vault_admin Benutzers.

vault secrets enable database

# Konfiguration der Verbindung (User muss in MySQL existieren)

```bash
vault write database/config/athena-db \
 plugin_name=mysql-database-plugin \
 connection_url="{{username}}:{{password}}@tcp(db:3306)/" \
 allowed_roles="athena-role" \
 username="vault_admin" \
 password="secure-vault-password"
```

3.2. Athena-Rolle erstellen

Diese Rolle definiert, welche Rechte die temporären Datenbank-User erhalten, die für den Athena-Service generiert werden. Beachten Sie das revocation_statement zur Vermeidung von MySQL-Fehlern beim Löschen.

```bash
vault write database/roles/athena-role \
    db_name=athena-db \
    creation_statements="CREATE USER '{{name}}'@'%' IDENTIFIED BY '{{password}}'; GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, ALTER, INDEX, REFERENCES ON athena_db.* TO '{{name}}'@'%';" \
    revocation_statements="DROP USER '{{name}}'@'%';" \
    default_ttl="1h" \
    max_ttl="24h"
```

4. Richtlinien (Policies) erstellen

Richtlinien definieren genau, wer was lesen darf. Wir erstellen getrennte Richtlinien für Athena und Aegis.

4.1. Athena Policy

Athena benötigt Zugriff auf JWT-Schlüssel und muss Datenbank-Credentials generieren können.

# Policy-Datei erstellen

```bash
echo '
# Lesezugriff auf JWT-Schlüssel und Registrierungs-Secret
path "secret/data/athena/jwt" {
    capabilities = ["read"]
}
```

# Dynamische DB-Secrets generieren

```bash
echo '
path "database/creds/athena-role" {
  capabilities = ["read"]
}
' > /tmp/athena-policy.hcl

# Policy in Vault schreiben

vault policy write athena /tmp/athena-policy.hcl
```

4.2. Aegis Policy

Aegis benötigt Lesezugriff auf seine eigene Konfiguration.

# Policy-Datei erstellen

```bash
echo '

# Lesezugriff auf Gateway-Konfigurationen

path "secret/data/aegis/config" {
capabilities = ["read"]
}
' > /tmp/aegis-policy.hcl

# Policy in Vault schreiben

vault policy write aegis /tmp/aegis-policy.hcl
```

5. AppRole Authentifizierung einrichten

Wir aktivieren die AppRole-Methode und verknüpfen die oben erstellten Richtlinien mit Rollen.

5.1. AppRole aktivieren

```bash
vault auth enable approle
```

5.2. Rollen definieren

# Athena Rolle erstellen (verknüpft mit 'athena' Policy)

```bash
vault write auth/approle/role/athena token_policies="athena" token_ttl=1h
```

# Aegis Rolle erstellen (verknüpft mit 'aegis' Policy)

```bash
vault write auth/approle/role/aegis token_policies="aegis" token_ttl=1h
```

6. Credentials abrufen (Für Deployment)

Damit die Container starten können, benötigen sie die RoleID und die SecretID. Diese müssen in die .env Datei eingetragen werden.

Für Athena:

# Role ID abrufen

```bash
vault read auth/approle/role/athena/role-id
```

# Secret ID generieren

```bash
vault write -f auth/approle/role/athena/secret-id
```

Kopieren Sie die Werte in Aegis/.env unter ATHENA_APPROLE_ROLE_ID und ATHENA_APPROLE_SECRET_ID.

Für Aegis:

# Role ID abrufen

```bash
vault read auth/approle/role/aegis/role-id
```

# Secret ID generieren

```bash
vault write -f auth/approle/role/aegis/secret-id
```

Kopieren Sie die Werte in Aegis/.env unter AEGIS_APPROLE_ROLE_ID und AEGIS_APPROLE_SECRET_ID.

7. Statische Secrets initialisieren

Vergessen Sie nicht, die JWT-Schlüssel initial in Vault zu schreiben, damit Athena sie lesen kann.

# Beispiel (Pfade zu Ihren generierten PEM-Dateien anpassen)

```bash
vault kv put secret/athena/jwt \
 private_key=@Athena/keys/private.pem \
 public_key=@Athena/keys/public.pem \
 registration_secret="IhrAdminSecret"
```
