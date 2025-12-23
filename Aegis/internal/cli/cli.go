package cli

// import (
// 	"bufio"
// 	"fmt"
// 	"gatekeeper/internal/config"
// 	"log/slog"
// 	"os"
// 	"strconv"
// 	"strings"
// 	"time"

// 	"gopkg.in/yaml.v3"
// )

// var reader = bufio.NewReader(os.Stdin)

// func readInput(prompt, defaultValue string) string {
// 	fmt.Printf("%s [%s]: ", prompt, defaultValue)

// 	input, _ := reader.ReadString('\n')
// 	input = strings.TrimSpace(input)

// 	if input == "" {
// 		return defaultValue
// 	}

// 	return input
// }

// func AddRouteWizzard(configPath string) {
// 	slog.Info("--- Starte Route-Erstellungs-Assistent ---")

// 	cfg, err := config.LoadConfig(configPath)
// 	if err != nil {
// 		slog.Error("Fehler beim Laden der Konfigurationsdatei", slog.String("path", configPath), slog.Any("error", err))
// 		os.Exit(1)
// 	}

// 	path := readInput("Pfad der neuen Route (z.B. /api/v2/data/*)", "")
// 	if path == "" {
// 		slog.Error("Pfad ist erforderlich. Assistent abgebrochen.")
// 		os.Exit(1)
// 	}

// 	targetURL := readInput("Ziel-URL (Downstream Service, z.B. http://service-a:8081)", "")
// 	if targetURL == "" {
// 		slog.Error("Ziel-URL ist erforderlich. Assistent abgebrochen.")
// 		os.Exit(1)
// 	}

// 	rolesStr := readInput("Erforderliche Rollen (Komma-separiert, leer für Public)", "")

// 	limitStr := readInput("Rate Limit (Anzahl Anfragen, 0 für deaktiviert)", "0")
// 	limit, err := strconv.ParseUint(limitStr, 10, 32)
// 	if err != nil {
// 		slog.Error("Ungültige Zahl für Limit", slog.Any("error", err))
// 		os.Exit(1)
// 	}
// 	window := "1m"
// 	if limit > 0 {
// 		window = readInput("Rate Limit Fenster (z.B. 1m, 5s)", "1m")
// 		if _, err := time.ParseDuration(window); err != nil {
// 			slog.Error("Ungültiges Rate Limit Fenster. Assistent abgebrochen.", slog.String("window", window))
// 			os.Exit(1)
// 		}
// 	}

// 	cacheTTLStr := readInput("Cache TTL (z.B. 30s, 5m, 0 für deaktiviert)", "0s")
// 	var cacheTTL time.Duration
// 	if cacheTTLStr != "0" && cacheTTLStr != "0s" {
// 		cacheTTL, err = time.ParseDuration(cacheTTLStr)
// 		if err != nil {
// 			slog.Error("Ungültige Cache TTL. Assistent abgebrochen.", slog.String("ttl", cacheTTLStr))
// 			os.Exit(1)
// 		}
// 	}

// 	cbThresholdStr := readInput("Circuit Breaker (Fehlerschwelle, 0 für deaktiviert)", "0")
// 	cbThreshold, err := strconv.Atoi(cbThresholdStr)
// 	if err != nil {
// 		slog.Error("Ungültige Zahl für Fehlerschwelle", slog.Any("error", err))
// 		os.Exit(1)
// 	}
// 	var cbTimeout time.Duration
// 	if cbThreshold > 0 {
// 		cbTimeoutStr := readInput("Circuit Breaker Open Timeout (z.B. 15s, 1m)", "15s")
// 		cbTimeout, err = time.ParseDuration(cbTimeoutStr)
// 		if err != nil {
// 			slog.Error("Ungültiger Circuit Breaker Timeout. Assistent abgebrochen.", slog.String("timeout", cbTimeoutStr))
// 			os.Exit(1)
// 		}
// 	}

// 	var roles []string
// 	if rolesStr != "" {
// 		for _, part := range strings.Split(rolesStr, ",") {
// 			trimmed := strings.TrimSpace(part)
// 			if trimmed != "" {
// 				roles = append(roles, trimmed)
// 			}
// 		}
// 	}

// 	newRoute := config.RouteConfig{
// 		Path:          path,
// 		TargetURL:     targetURL,
// 		RequiredRoles: roles,
// 		RateLimit: config.RateLimitConfig{
// 			Limit:  uint32(limit),
// 			Window: window,
// 		},
// 		CacheTTL: cacheTTL.String(),
// 		CircuitBreaker: config.CircuitBreakerConfig{
// 			FailureThreshold: cbThreshold,
// 			OpenTimeout:      cbTimeout.String(),
// 		},
// 	}

// 	cfg.Routes = append(cfg.Routes, newRoute)

// 	data, err := yaml.Marshal(cfg)
// 	if err != nil {
// 		slog.Error("Fehler beim Serialisieren der Konfiguration", slog.Any("error", err))
// 		os.Exit(1)
// 	}

// 	err = os.WriteFile(configPath, data, 0644)
// 	if err != nil {
// 		slog.Error("Fehler beim Speichern der Konfigurationsdatei", slog.Any("error", err))
// 		os.Exit(1)
// 	}

// 	slog.Info("Route erfolgreich hinzugefügt. Konfiguration wird live neu geladen.", slog.String("path", newRoute.Path))
// }
