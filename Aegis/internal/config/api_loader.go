package config

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
)

type athenaProjectRoute struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Path      string `json:"path"`
	TargetURL string `json:"target_url"`
	CacheTTL  string `json:"cache_ttl"`

	RequiredRoles []string `json:"required_roles"`

	RateLimit struct {
		Limit  int    `json:"limit"`
		Window string `json:"window"`
	} `json:"rate_limit"`

	CircuitBreaker struct {
		FailureThreshold int    `json:"failure_threshold"`
		OpenTimeout      string `json:"open_timeout"`
	} `json:"circuit_breaker"`
}

// fetchFromAthena ist eine wiederverwendbare Helferfunktion
func fetchFromAthena(apiURL, apiSecret string) ([]byte, error) {
	if apiURL == "" {
		return nil, fmt.Errorf("API URL ist leer")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Erstellen der Anfrage: %w", err)
	}
	req.Header.Set("X-Internal-Secret", apiSecret)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Abrufen von Athena (%s): %w", apiURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("athena API (%s) hat mit Status %d geantwortet", apiURL, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// LoadConfigFromAPI (Lädt jetzt Routen UND die Context-Map)
func LoadConfigFromAPI(routesAPIURL, contextMapAPIURL, apiSecret string) (*GatewayConfig, error) {
	slog.Debug("Lade Gateway-Konfiguration von Athena API...", "routes_url", routesAPIURL)

	cfg := loadLocalConfig() // Lädt Ports, Redis, AdminHost etc. aus Env-Vars

	if routesAPIURL == "" || apiSecret == "" {
		return nil, fmt.Errorf("ATHENA_CONFIG_URL oder ATHENA_INTERNAL_SECRET ist nicht gesetzt")
	}

	// 1. Routen laden
	routesData, err := fetchFromAthena(routesAPIURL, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Laden der Routen: %w", err)
	}

	var athenaRoutes []*athenaProjectRoute
	if err := json.Unmarshal(routesData, &athenaRoutes); err != nil {
		return nil, fmt.Errorf("fehler beim Parsen der Routen-Antwort von Athena: %w", err)
	}

	cfg.Routes = make([]RouteConfig, 0, len(athenaRoutes))
	for _, ar := range athenaRoutes {
		aegisRoute := RouteConfig{
			Path:          ar.Path,
			TargetURL:     ar.TargetURL,
			RequiredRoles: ar.RequiredRoles,
			CacheTTL:      ar.CacheTTL,
			RateLimit: RateLimitConfig{
				Limit:  uint32(ar.RateLimit.Limit),
				Window: ar.RateLimit.Window,
			},
			CircuitBreaker: CircuitBreakerConfig{
				FailureThreshold: ar.CircuitBreaker.FailureThreshold,
				OpenTimeout:      ar.CircuitBreaker.OpenTimeout,
			},
		}
		cfg.Routes = append(cfg.Routes, aegisRoute)
	}
	slog.Info("Gateway-Routen erfolgreich von Athena API geladen", slog.Int("routes_loaded", len(cfg.Routes)))

	// 2. Context Map laden
	if contextMapAPIURL != "" {
		slog.Debug("Lade Context Map von Athena...", "url", contextMapAPIURL)
		mapData, err := fetchFromAthena(contextMapAPIURL, apiSecret)
		if err != nil {
			slog.Warn("Fehler beim Laden der Context Map (wird übersprungen)", "error", err)
		} else {
			var contextMap map[string]string
			if err := json.Unmarshal(mapData, &contextMap); err != nil {
				slog.Warn("Fehler beim Parsen der Context Map", "error", err)
			} else {
				cfg.ContextMap = contextMap
				slog.Info("Context Map erfolgreich geladen", slog.Int("hosts", len(cfg.ContextMap)))
			}
		}
	} else {
		slog.Warn("ATHENA_CONTEXT_MAP_URL nicht gesetzt. Domain-Routing ist deaktiviert.")
		cfg.ContextMap = make(map[string]string) // Initialisiere leere Map
	}

	return cfg, nil
}

// loadLocalConfig (Lädt jetzt auch AdminHost und ContextMapURL)
func loadLocalConfig() *GatewayConfig {
	cfg := &GatewayConfig{}
	portStr := os.Getenv("AEGIS_PORT")
	if portStr == "" {
		portStr = "8080"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		slog.Warn("Ungültiger AEGIS_PORT, verwende 8080", "value", portStr, "error", err)
		port = 8080
	}
	cfg.Port = port

	metricsPortStr := os.Getenv("AEGIS_METRICS_PORT")
	if metricsPortStr == "" {
		metricsPortStr = "9090" // Default
	}
	metricsPort, err := strconv.Atoi(metricsPortStr)
	if err != nil {
		slog.Warn("Ungültiger AEGIS_METRICS_PORT, verwende 9090", "value", metricsPortStr, "error", err)
		metricsPort = 9090
	}
	cfg.MetricsPort = metricsPort

	cfg.RedisAddr = os.Getenv("AEGIS_REDIS_ADDR")
	if cfg.RedisAddr == "" {
		slog.Warn("AEGIS_REDIS_ADDR nicht gesetzt, verwende default 'localhost:6379'")
		cfg.RedisAddr = "localhost:6379"
	}

	cfg.JwtPublicKeyPath = os.Getenv("JWT_PUBLIC_KEY_PATH")
	if cfg.JwtPublicKeyPath == "" {
		slog.Warn("JWT_PUBLIC_KEY_PATH ist nicht gesetzt. JWT-Validierung wird fehlschlagen.")
		cfg.JwtPublicKeyPath = "configs/public.pem"
	}

	cfg.Cors = CorsConfig{
		AllowedOrigins: []string{"http://localhost:8082"},
	}

	// NEUE ENV-VARIABLEN
	cfg.ContextMapURL = os.Getenv("ATHENA_CONTEXT_MAP_URL")
	cfg.AdminHost = os.Getenv("AEGIS_ADMIN_HOST") // z.B. "athena.deine-firma.de"

	return cfg
}