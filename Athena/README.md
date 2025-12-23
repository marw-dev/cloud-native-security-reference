# Athena - Zentraler Authentifizierungs- und Konfigurationsdienst

Athena ist ein hochsicherer, "Vault-First" Backend-Dienst, geschrieben in Go. Es dient als zentrales Nervensystem für das Aegis API Gateway, indem es Benutzerauthentifizierung, Multi-Tenancy-Projektmanagement und die dynamische Konfiguration von Gateway-Routen verwaltet.

Das Projekt ist auf **maximale Sicherheit** und **strikte Trennung von Geheimnissen** ausgelegt.

---

## Tech Stack

- **Kern:** Go (mit Chi-Router)
- **Datenbank:** MySQL (mit `sqlx`)
- **Secrets:** HashiCorp Vault (via AppRole, **zwingend erforderlich**)
- **Observability-Stack (LGTM):**
  - **L**oki (Logs)
  - **G**rafana (Dashboards)
  - **T**empo (Traces)
  - **M**etriken (Prometheus)
- **Deployment:** Docker, `docker-compose`

---

## Hauptfunktionen

- **Zentrale Authentifizierung:**
  - **JWT (RS256):** Erstellt signierte RSA-Tokens (Access, Refresh, Grace).
  - **Passwort-Sicherheit:** Verwendet `bcrypt` für das Hashen von Passwörtern.
  - **Token-Rotation:** Implementiert einen sicheren Refresh-Token-Flow mit Rotation.
- **Multi-Tenancy-Architektur:**
  - **Projekt-Management:** Kapselt Benutzer und Routen in "Projekte".
  - **Host-basiertes Routing:** Weist Domains (z.B. `webshop.com`) zu Projekt-IDs zu.
- **Granulares Sicherheitsmanagement:**
  - **Rollenbasiert (ACL):** Definiert globale Admin-Rollen (`is_admin`) und Projekt-Rollen (z.B. `owner`, `admin`, `user`).
  - **Zwei-Faktor-Authentifizierung (2FA):** Bietet 2FA auf globaler Ebene (für Admins) und pro Projekt (kann erzwungen werden).
- **Dynamische Gateway-Konfiguration:**
  - **Interne API:** Stellt eine (mit `X-Internal-Secret`) gesicherte API für Aegis bereit.
  - **Config-Endpoint:** Liefert alle Projekt-Routen (`/internal/v1/routes/config`).
  - **Context-Map-Endpoint:** Liefert die Host-zu-Projekt-Map (`/internal/v1/context-map`).
- **Volle Beobachtbarkeit:**
  - **Strukturiertes Logging:** Verwendet `slog` für alle Logs.
  - **Audit Trail:** Erstellt dedizierte Audit-Logs für sicherheitsrelevante Aktionen (z.B. Login, OTP-Setup, Rollenänderung).

---

## Technische Features (Übersicht)

Athena ist ein API-Dienst, der die Logik für Authentifizierung und Konfiguration bereitstellt. Im Gegensatz zu Aegis (einem reinen Middleware-Proxy) implementiert Athena die Business-Logik.

### 1. Vault-First Architektur

Das Kernprinzip von Athena ist, dass **keine** sensiblen Geheimnisse (wie DB-Passwörter oder JWT-Schlüssel) jemals in Umgebungsvariablen oder `.env`-Dateien gespeichert werden.

- **Startvorgang:** Beim Start ruft `main.go` die Funktion `auth.LoadAthenaSecretsFromVault` auf.
- **AppRole-Login:** Diese Funktion nutzt die bereitgestellten `ATHENA_APPROLE_ROLE_ID` und `ATHENA_APPROLE_SECRET_ID`, um sich per AppRole bei Vault anzumelden.
- **Secret-Abruf:** Nach erfolgreichem Login liest Athena die Geheimnisse aus den in Vault konfigurierten Pfaden:
    1.  **JWT-Geheimnisse** (`VAULT_JWT_SECRET_PATH`): Lädt `private_key`, `public_key` und `registration_secret`.
    2.  **DB-Geheimnisse** (`VAULT_DB_CREDS_PATH`): Lädt die `database_url`.
- **Fehlschlag:** Wenn einer dieser Schritte fehlschlägt, bricht der Dienst den Start ab.

### 2. Multi-Tenancy Authentifizierungs-Fluss

Athena unterscheidet zwischen zwei Haupt-Login-Szenarien, die durch den von Aegis bereitgestellten `X-Project-ID`-Header gesteuert werden:

1.  **Globaler Login (Admin-UI):**
    -   Aegis erkennt den Admin-Host (z.B. `athena.meinefirma.de`) und setzt *keinen* `X-Project-ID`-Header.
    -   Athena's `LoginHandler` erkennt das Fehlen der Projekt-ID.
    -   Er prüft, ob der Benutzer `is_admin` ist und ob globales 2FA aktiv ist.
    -   Der ausgestellte JWT enthält `is_admin: true`, aber keine Projekt-Rollen.

2.  **Projekt-Login (Kunden-App):**
    -   Aegis erkennt einen Kunden-Host (z.B. `webshop.com`), schlägt ihn in der `ContextMap` nach und injiziert den `X-Project-ID`-Header.
    -   Athena's Middleware (`ProjectIDValidator`) liest diesen Header.
    -   Der `LoginHandler` wechselt in den `handleProjectLogin`-Modus.
    -   Er prüft Anmeldedaten *und* ob der Benutzer Mitglied dieses Projekts ist.
    -   Er prüft Projekt-spezifisches 2FA (auch `force_2fa`-Einstellungen).
    -   Der ausgestellte JWT enthält die Rollen des Benutzers *innerhalb dieses Projekts* (z.B. `user`).

### 3. Interne API (Für Aegis)

Athena stellt eine separate, interne API unter `/internal/v1` bereit. Diese wird von Aegis genutzt, um seine Konfiguration zu laden.

-   **Sicherheit:** Diese Routen werden durch die `InternalAuth`-Middleware geschützt.
-   **Geteiltes Geheimnis:** Diese Middleware prüft auf ein statisches `X-Internal-Secret`-Header. Dies ist das *einzige* Geheimnis, das zwischen Aegis und Athena über Umgebungsvariablen geteilt wird (`ATHENA_INTERNAL_SECRET`). Es wird *nicht* aus Vault geladen, um einen Henne-Ei-Konflikt beim Start von Aegis zu vermeiden.

---

## Schnellstart (Lokal mit Observability)

Sie benötigen `go`, `docker` und `docker-compose`.

Das Starten von Athena ist Teil des gesamten `docker-compose`-Setups im `Aegis`-Verzeichnis.

### 1. Monitoring-Stack starten

Starten Sie Grafana und die dazugehörigen Datenbanken (Loki, Tempo, Prometheus).

```bash
# (Im Aegis-Verzeichnis ausf)
docker compose up -d
