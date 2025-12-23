package main

import (
	"context"
	"crypto/rsa"
	"fmt"
	"log"
	"log/slog"
	"os"

	"gatekeeper/internal/config"
	"gatekeeper/internal/server" // Importiert server
	"gatekeeper/internal/telementry"

	"github.com/golang-jwt/jwt/v5"
	_ "gopkg.in/yaml.v3"
)

// loadPublicKey (bleibt hier, da es vor der Server-Initialisierung benötigt wird)
func loadPublicKey(filePath string) (*rsa.PublicKey, error) {
	slog.Debug("Lade öffentlichen RSA-Schlüssel", slog.String("path", filePath))
	keyData, err := os.ReadFile(filePath)
	if err != nil {
		slog.Error("Fehler beim Lesen der öffentlichen Schlüsseldatei", slog.String("path", filePath), slog.Any("error", err))
		return nil, fmt.Errorf("fehler beim Lesen der öffentlichen Schlüsseldatei %s: %w", filePath, err)
	}

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(keyData)
	if err != nil {
		slog.Error("Fehler beim Parsen des öffentlichen RSA-Schlüssels", slog.String("path", filePath), slog.Any("error", err))
		return nil, fmt.Errorf("fehler beim Parsen des öffentlichen RSA-Schlüssels aus %s: %w", filePath, err)
	}
	slog.Info("Öffentlicher RSA-Schlüssel erfolgreich geladen", slog.String("path", filePath))

	return publicKey, nil
}

func main() {
	// 1. Logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// 2. Initialkonfiguration
	athenaAPIURL := os.Getenv("ATHENA_CONFIG_URL")
	athenaAPISecret := os.Getenv("ATHENA_INTERNAL_SECRET")
	athenaContextMapURL := os.Getenv("ATHENA_CONTEXT_MAP_URL")

	cfg, err := config.LoadConfigFromAPI(athenaAPIURL, athenaContextMapURL, athenaAPISecret)
	if err != nil {
		log.Fatalf("Fehler beim Laden der initialen Konfiguration von Athena: %v", err)
	}

	publicKey, err := loadPublicKey(cfg.JwtPublicKeyPath)
	if err != nil {
		log.Fatalf("Fehler beim Laden des JWT Public Key: %v", err)
	}

	// 3. Telemetrie
	tpShutdown := telementry.InitTracerProvider("gatekeeper")
	defer func() {
		if err := tpShutdown(context.Background()); err != nil {
			log.Printf("Fehler beim Shutdown des Tracer Providers: %v", err)
		}
	}()

	// 4. Abhängigkeiten bündeln
	deps := &server.Dependencies{
		Config:           cfg,
		PublicKey:        publicKey,
		TracerShutdown:   tpShutdown,
		AthenaAPIURL:     athenaAPIURL,
		AthenaAPISecret:  athenaAPISecret,
		AthenaContextMapURL: athenaContextMapURL,
	}

	// 5. Server erstellen
	s := server.NewServer(deps)

	// 6. Server starten
	go s.Start()

	// 7. Auf Shutdown warten
	s.WaitForShutdown()
}