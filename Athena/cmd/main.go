package main

import (
	"athena/internal/auth"
	"athena/internal/config"
	"athena/internal/database"
	"athena/internal/handlers"
	"athena/internal/router"
	"athena/internal/server"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// 1. Setup (Logging & Config)
	err := godotenv.Load()
	if err != nil {
		slog.Warn("Fehler beim Laden der .env-Datei (ignoriert)", slog.Any("error", err))
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Fehler beim Laden der Konfiguration", slog.Any("error", err))
		os.Exit(1)
	}

	// 2. Abhängigkeiten (Secrets & DB)
	athenaSecrets, err := auth.LoadAthenaSecretsFromVault(cfg)
	if err != nil {
		slog.Error("Fehler beim Laden der Secrets aus Vault", slog.Any("error", err))
		os.Exit(1)
	}
	
	// Verwende die Secrets
	privateKey := athenaSecrets.PrivateKey
	publicKey := athenaSecrets.PublicKey
	databaseURL := athenaSecrets.DatabaseURL
	cfg.RegistrationSecret = athenaSecrets.RegistrationSecret


	db, err := database.ConnectDB(databaseURL)
	if err != nil {
		slog.Error("Fehler beim Verbinden zur Datenbank", slog.Any("error", err), slog.String("used_url_source", "Vault"))
		os.Exit(1)
	}
	defer db.Close()

	// Migrations
	if err := database.RunMigrations(db.DB.DB, "athena_db"); err != nil {
		slog.Error("Fehler bei der Datenbank-Migration", slog.Any("error", err))
		os.Exit(1)
	}

	// 3. Handler & Repositories
	userRepo := database.NewUserRepository(db)
	projectRepo := database.NewProjectRepository(db)
	routeRepo := database.NewRouteRepository(db)
	tokenRepo := database.NewTokenRepository(db)

	adminHandlers := handlers.NewAdminHandlers(
		userRepo,
		tokenRepo,
		cfg.OTPIssuerName,
		privateKey,
		cfg.JWTAccessTokenTTL,
		cfg.JWTRefreshTokenTTL,
	)

	handlers := router.HandlerDependencies{
		UserRepo:    userRepo,
		ProjectRepo: projectRepo,
		RouteRepo:   routeRepo,
		TokenRepo:   tokenRepo,
		DBPinger:    db,
		PrivateKey:  privateKey,
		PublicKey:   publicKey,
		Config:      cfg,
		AdminHandlers: adminHandlers,
	}

	// 4. Router & Server
	r := router.SetupRouter(handlers)

	srv := server.NewServer(cfg.Port, r)

	// 5. Starten & Graceful Shutdown
	server.StartAndShutdown(srv, db)
}