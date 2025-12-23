# Athena - Identity & Configuration Service

Athena is a hardened, **"Vault-First"** backend service written in Go. It acts as the authoritative control plane for the Aegis API Gateway, managing user identities, multi-tenant project isolation, and dynamic routing configurations.

The architecture is designed for **defense-in-depth** and **strict separation of secrets**.

---

## Tech Stack

- **Core:** Go (using `chi` router)
- **Persistence:** MySQL (via `sqlx`)
- **Secret Management:** HashiCorp Vault (AppRole Auth **enforced**)
- **Observability (LGTM Stack):**
  - **L**oki (Structured Logs via `slog`)
  - **G**rafana (Visualizations)
  - **T**empo (Distributed Tracing via OTel)
  - **M**etrics (Prometheus)
- **Infrastructure:** Docker, `docker-compose`

---

## Key Features

### Centralized Authentication

- **Asymmetric JWTs (RS256):** Issues cryptographically signed RSA tokens (Access, Refresh, and Grace tokens).
- **Password Security:** Implements `bcrypt` with high work factors for credential hashing.
- **Secure Token Rotation:** Features a robust refresh token flow with automatic reuse detection (token family invalidation).

### Multi-Tenancy Architecture

- **Project Isolation:** Encapsulates users, roles, and routes within logical "Projects".
- **Domain-Based Routing:** Dynamically maps incoming Host headers (e.g., `shop.client.com`) to specific Project IDs.

### Granular Access Control (RBAC)

- **Role Management:** Distinguishes between Global Admin roles (`is_admin`) and scoped Project Roles (e.g., `owner`, `maintainer`, `viewer`).
- **Two-Factor Authentication (2FA):** Supports TOTP-based 2FA globally (for admins) and enforces it per-project based on policy.

### Dynamic Gateway Configuration

- **Control Plane API:** Exposes secured endpoints for Aegis to fetch configurations.
- **Hot-Reloading Support:** Provides endpoints for Route Configs (`/internal/v1/routes/config`) and Context Maps (`/internal/v1/context-map`).

### Deep Observability

- **Structured Logging:** JSON-formatted logs correlated with Trace IDs for seamless debugging.
- **Audit Trails:** Immutable audit logs for all security-critical actions (Login, OTP setup, Permission changes).

---

## Technical Deep Dive

Athena serves as the business logic layer, while Aegis acts as the edge proxy.

### 1. Vault-First Architecture

Athena adopts a **Zero-Trust approach** to bootstrapping. No sensitive secrets (database credentials, private keys) are ever stored in environment variables or configuration files.

1.  **Bootstrapping:** On startup, `main.go` initializes the Vault client using `ATHENA_APPROLE_ROLE_ID` & `SECRET_ID`.
2.  **AppRole Login:** Authenticates against HashiCorp Vault to obtain a temporary client token.
3.  **Secret Injection:** Fetches secrets directly into memory:
    - **PKI:** `private_key`, `public_key`, and `registration_secret` from `VAULT_JWT_SECRET_PATH`.
    - **Database:** Generates ephemeral MySQL credentials via `VAULT_DB_CREDS_PATH`.
4.  **Fail-Safe:** If Vault is unreachable or authentication fails, the service refuses to start (Fail Fast).

### 2. Multi-Tenant Authentication Flow

Athena determines the login context based on headers injected by the Aegis Gateway:

- **Global Context (Admin UI):**

  - _Trigger:_ No `X-Project-ID` header present.
  - _Logic:_ Authenticates against the global user table. Checks for `is_admin` privileges and global 2FA requirements.
  - _Result:_ JWT contains global scope but no project claims.

- **Project Context (Tenant App):**
  - _Trigger:_ Aegis resolves the Host to a Tenant and injects `X-Project-ID`.
  - _Logic:_ Athena validates credentials AND membership within the specific project. Enforces project-specific policies (e.g., `force_2fa`).
  - _Result:_ JWT contains scoped roles (e.g., `role: "admin"`) valid only for that project.

### 3. Internal Control API

Communication between Aegis (Gateway) and Athena (Control Plane) occurs over a secured internal network.

- **Security:** Protected by the `InternalAuth` middleware.
- **The "Bootstrap Secret":** Uses a static `X-Internal-Secret` header.
  - _Note:_ This is the **only** secret shared via environment variables (`ATHENA_INTERNAL_SECRET`). This architectural decision avoids a "chicken-and-egg" problem where the Gateway would need to fetch Vault secrets before being able to route requests.

---

## Quick Start

Ensure `go`, `docker`, and `docker-compose` are installed.

To start Athena as part of the complete system (including Vault and Observability), run the compose file from the root directory:

```bash
# From the project root (Aegis/)
docker compose up -d
```
