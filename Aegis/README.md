# HashiCorp Vault Configuration Guide

This guide details the security infrastructure setup for **Aegis (API Gateway)** and **Athena (Identity Service)** using HashiCorp Vault.

The architecture enforces a **Zero-Trust** model using **AppRole Authentication** to inject secrets into the containers at runtime. No static secrets or credentials are stored in the source code.

---

## Architecture Overview

Vault acts as the central "Source of Truth" managing two types of secrets:

1.  **Static Secrets (KV v2 Engine):**

    - `secret/data/athena/jwt`: Stores RSA Keypairs (Private/Public) for JWT signing and the generic registration secret.
    - `secret/data/aegis/config`: Stores gateway-specific configuration (if applicable).

2.  **Dynamic Secrets (Database Engine):**
    - `database/creds/athena-role`: Generates **ephemeral, short-lived** MySQL database credentials for the Athena service.

---

## 1. Prerequisites

Ensure your Vault instance is initialized and unsealed.

```bash
# Set environment variables for CLI access
export VAULT_ADDR='[http://127.0.0.1:8200](http://127.0.0.1:8200)'
export VAULT_TOKEN='<YOUR-ROOT-TOKEN>'

```

## 2. Infrastructure Setup

### 2.1 Enable Secret Engines

Enable the necessary secret engines for Key-Value storage and Database generation.

```bash
# Enable KV v2 (Static Secrets)
vault secrets enable -path=secret kv-v2

# Enable Database Engine (Dynamic Secrets)
vault secrets enable database
```

### 2.2 Configure MySQL Connection

Connect Vault to the MySQL container using a privileged bootstrap user (e.g., vault_admin).

```bash
vault write database/config/athena-db \
    plugin_name=mysql-database-plugin \
    connection_url="{{username}}:{{password}}@tcp(db:3306)/" \
    allowed_roles="athena-role" \
    username="vault_admin" \
    password="secure-vault-password"
```

### 2.3 Create Database Role (Athena)

Define the permissions for the ephemeral users created by Athena.

- **Default TTL:** 1 hour (Credentials expire automatically).
- **Revocation:** Ensures the user is dropped from MySQL when the lease expires.

## 3. Access Control (Policies)

We apply the Principle of Least Privilege. Each service gets its own specific policy.

### 3.1 Athena Policy

Athena requires access to RSA keys and the ability to generate DB credentials.

```bash

# Create policy file
cat <<EOF > /tmp/athena-policy.hcl
# Read RSA Keys and Registration Secret
path "secret/data/athena/jwt" {
    capabilities = ["read"]
}

# Generate dynamic MySQL credentials
path "database/creds/athena-role" {
    capabilities = ["read"]
}
EOF

# Apply policy
vault policy write athena /tmp/athena-policy.hcl
```

### 3.2 Aegis Policy

Aegis only requires read access to its specific configuration.

```bash
# Create policy file
cat <<EOF > /tmp/aegis-policy.hcl
path "secret/data/aegis/config" {
    capabilities = ["read"]
}
EOF

# Apply policy
vault policy write aegis /tmp/aegis-policy.hcl
```

## 4. AppRole Authentication Setup

AppRole is the standard authentication method for machine-to-machine communication.

### 4.1 Enable AppRole Auth Method

```bash
vault auth enable approle
```

### 4.2 Define Service Roles

Link the AppRoles to the policies created in Step 3.

```bash
# Create Athena Role
vault write auth/approle/role/athena \
    token_policies="athena" \
    token_ttl=1h

# Create Aegis Role
vault write auth/approle/role/aegis \
    token_policies="aegis" \
    token_ttl=1h
```

## 5. Deployment & Seed Data

### 5.1 Seed Initial Secrets (RSA Keys)

Upload your generated RSA keys to Vault so Athena can fetch them on startup.

> Note: Ensure your .pem files are located in Athena/keys/.

```bash
vault kv put secret/athena/jwt \
    private_key=@Athena/keys/private.pem \
    public_key=@Athena/keys/public.pem \
    registration_secret="YourSuperSecureAdminSecret"
```

### 5.2 Retrieve Credentials for .env

To start the containers, you need to inject the RoleID (Static Identity) and SecretID (One-Time Password) into your environment configuration.

**For Athena Service:**

```bash
# Get Role ID
vault read auth/approle/role/athena/role-id

# Generate Secret ID
vault write -f auth/approle/role/athena/secret-id
```

Copy these values to Aegis/.env as ATHENA_APPROLE_ROLE_ID and ATHENA_APPROLE_SECRET_ID.

**For Aegis Service:**

```bash
# Get Role ID
vault read auth/approle/role/aegis/role-id

# Generate Secret ID
vault write -f auth/approle/role/aegis/secret-id
```

Copy these values to Aegis/.env as AEGIS_APPROLE_ROLE_ID and AEGIS_APPROLE_SECRET_ID.
