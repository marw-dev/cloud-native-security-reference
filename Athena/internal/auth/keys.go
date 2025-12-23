package auth

import (
	"athena/internal/config"
	"crypto/rsa"
	"fmt"
	"log/slog"
	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/vault/api"
)

// createVaultClient ist eine interne Hilfsfunktion
func createVaultClient(cfg *config.Config) (*api.Client, error) {
	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = cfg.VaultAddr
	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Erstellen des Vault-Clients: %w", err)
	}

	slog.Info("Führe Vault AppRole-Login aus...")
	appRoleData := map[string]any{
		"role_id":   cfg.VaultAppRoleRoleID,
		"secret_id": cfg.VaultAppRoleSecretID,
	}

	resp, err := client.Logical().Write("auth/approle/login", appRoleData)

	if err != nil {
		return nil, fmt.Errorf("fehler beim Vault AppRole-Login: %w", err)
	}
	if resp == nil || resp.Auth == nil {
		return nil, fmt.Errorf("vault AppRole-Login gab keine Authentifizierungsdaten zurück")
	}

	client.SetToken(resp.Auth.ClientToken)
	slog.Info("Vault AppRole-Login erfolgreich. Kurzlebiger Token gesetzt.")
	return client, nil
}

// readKV2SecretData liest Daten spezifisch aus der KV v2 Engine (für JWTs)
func readKV2SecretData(client *api.Client, path string) (map[string]any, error) {
	secret, err := client.Logical().Read(path)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Lesen des Secrets von Vault %s: %w", path, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("keine daten gefunden unter Vault-Pfad: %s", path)
	}

	// Bei KV v2 sind die Daten in einem "data"-Feld verschachtelt
	secretData, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		// Fallback: Versuche es direkt (falls versehentlich KV v1 oder falscher Pfad)
		return secret.Data, nil 
	}
	return secretData, nil
}

type AthenaSecrets struct {
	PrivateKey         		*rsa.PrivateKey
	PublicKey          		*rsa.PublicKey
	DatabaseURL        		string
	RegistrationSecret 		string
	AthenaInternalSecret	string
}

func LoadAthenaSecretsFromVault(cfg *config.Config) (*AthenaSecrets, error) {
	slog.Debug("Erstelle Vault-Client für Athena-Secrets...", slog.String("address", cfg.VaultAddr))
	client, err := createVaultClient(cfg)
	if err != nil {
		return nil, err
	}

	secrets := &AthenaSecrets{}

	// --- 1. JWT-Schlüssel (KV v2 Engine) ---
	slog.Debug("Lese JWT/Registrierungs-Secrets aus Vault", slog.String("path", cfg.VaultJWTSekretPath))
	jwtSecretData, err := readKV2SecretData(client, cfg.VaultJWTSekretPath)
	if err != nil {
		slog.Error("Fehler beim Lesen der JWT-Daten", slog.Any("error", err))
		return nil, err
	}

	privateKeyPEM, _ := jwtSecretData["private_key"].(string)
	publicKeyPEM, _ := jwtSecretData["public_key"].(string)
	regSecret, _ := jwtSecretData["registration_secret"].(string)
	internalSecret, _ := jwtSecretData["athena_internal_secret"].(string)

	if privateKeyPEM == "" || publicKeyPEM == "" {
		return nil, fmt.Errorf("private_key oder public_key fehlen im Vault Secret")
	}

	secrets.PrivateKey, err = jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("fehler beim Parsen des privaten Schlüssels: %w", err)
	}
	secrets.PublicKey, err = jwt.ParseRSAPublicKeyFromPEM([]byte(publicKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("fehler beim Parsen des öffentlichen Schlüssels: %w", err)
	}
	secrets.RegistrationSecret = regSecret
	slog.Info("JWT-Schlüssel erfolgreich geladen.")

	// --- 2. Datenbank (Database Engine) ---
	slog.Debug("Lese dynamische DB-Credentials aus Vault", slog.String("path", cfg.VaultDBCredsPath))
	
	dbSecret, err := client.Logical().Read(cfg.VaultDBCredsPath)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Lesen der DB-Credentials: %w", err)
	}
	if dbSecret == nil || dbSecret.Data == nil {
		return nil, fmt.Errorf("keine Credentials von Vault unter %s erhalten", cfg.VaultDBCredsPath)
	}

	// Die Felder heißen direkt "username" und "password"
	username, ok1 := dbSecret.Data["username"].(string)
	password, ok2 := dbSecret.Data["password"].(string)

	if !ok1 || !ok2 {
		return nil, fmt.Errorf("vault antwort enthielt keine username/password felder")
	}

	// DSN aus Umgebungsvariablen + Vault-Daten bauen
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" { dbHost = "db" }
	
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" { dbPort = "3306" }

	dbName := os.Getenv("DB_NAME")
	if dbName == "" { dbName = "my_go_db" }

	// DSN Format: user:pass@tcp(host:port)/dbname?param=value
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", username, password, dbHost, dbPort, dbName)
	
	secrets.DatabaseURL = dsn
	slog.Info("Dynamische DB-Credentials geladen und DSN generiert", slog.String("db_user", username))

	secrets.AthenaInternalSecret = internalSecret

	return secrets, nil
}
