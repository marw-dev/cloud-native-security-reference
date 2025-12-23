package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port string
	JWTAccessTokenTTL  time.Duration
	JWTRefreshTokenTTL time.Duration
	JWTGraceTokenTTL   time.Duration
	OTPIssuerName      string

	RegistrationSecret string

	// AppRole Credentials
	VaultAddr           string
	VaultAppRoleRoleID  string
	VaultAppRoleSecretID string

	VaultJWTSekretPath  string
	VaultDBCredsPath    string

	GatekeeperIPs []string
}

func LoadConfig() (*Config, error) {
	var cfg Config
	var err error

	cfg.Port = os.Getenv("PORT")
	if cfg.Port == "" {
		cfg.Port = "8081"
	}
	if _, err := strconv.Atoi(cfg.Port); err != nil {
		return nil, fmt.Errorf("ungültiger PORT: %w", err)
	}

	ttlAccessStr := os.Getenv("JWT_ACCESS_TOKEN_TTL")
	if ttlAccessStr == "" {
		ttlAccessStr = "1h" // Default value
	}
	cfg.JWTAccessTokenTTL, err = time.ParseDuration(ttlAccessStr)
	if err != nil {
		return nil, fmt.Errorf("ungültige JWT_ACCESS_TOKEN_TTL: %w", err)
	}

	ttlRefreshStr := os.Getenv("JWT_REFRESH_TOKEN_TTL")
	if ttlRefreshStr == "" {
		ttlRefreshStr = "168h" // Default value
	}
	cfg.JWTRefreshTokenTTL, err = time.ParseDuration(ttlRefreshStr)
	if err != nil {
		return nil, fmt.Errorf("ungültige JWT_REFRESH_TOKEN_TTL: %w", err)
	}

	ttlGraceStr := os.Getenv("JWT_GRACE_TOKEN_TTL")
	if ttlGraceStr == "" {
		ttlGraceStr = "5m"
	}
	cfg.JWTGraceTokenTTL, err = time.ParseDuration(ttlGraceStr)
	if err != nil {
		return nil, fmt.Errorf("ungültige JWT_GRACE_TOKEN_TTL: %w", err)
	}

	cfg.OTPIssuerName = os.Getenv("OTP_ISSUER_NAME")
	if cfg.OTPIssuerName == "" {
		cfg.OTPIssuerName = "Athena" // Default value
	}

	// Vault-Variablen laden
	cfg.VaultAddr = os.Getenv("VAULT_ADDR")
	
	// Lade AppRole Credentials
	cfg.VaultAppRoleRoleID = os.Getenv("ATHENA_APPROLE_ROLE_ID")
	cfg.VaultAppRoleSecretID = os.Getenv("ATHENA_APPROLE_SECRET_ID")

	cfg.VaultJWTSekretPath = os.Getenv("VAULT_JWT_SECRET_PATH")
	cfg.VaultDBCredsPath = os.Getenv("VAULT_DB_CREDS_PATH")

	// Vault-Prüfungen
	if cfg.VaultAddr == "" {
		return nil, fmt.Errorf("konfiguration fehlt: VAULT_ADDR muss gesetzt sein")
	}
	if cfg.VaultAppRoleRoleID == "" {
		return nil, fmt.Errorf("konfiguration fehlt: ATHENA_APPROLE_ROLE_ID muss gesetzt sein")
	}
	if cfg.VaultAppRoleSecretID == "" {
		return nil, fmt.Errorf("konfiguration fehlt: ATHENA_APPROLE_SECRET_ID muss gesetzt sein")
	}
	if cfg.VaultJWTSekretPath == "" {
		return nil, fmt.Errorf("konfiguration fehlt: VAULT_JWT_SECRET_PATH muss gesetzt sein")
	}
	if cfg.VaultDBCredsPath == "" {
		return nil, fmt.Errorf("konfiguration fehlt: VAULT_DB_CREDS_PATH muss gesetzt sein")
	}

	gatekeeperIPsStr := os.Getenv("AEGIS_IPS")
	if gatekeeperIPsStr == "" {
		cfg.GatekeeperIPs = []string{"127.0.0.1", "::1"}
	} else {
		cfg.GatekeeperIPs = strings.Split(gatekeeperIPsStr, ",")
	}

	return &cfg, nil
}