package server

import (
	"context"
	"crypto/rsa"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"gatekeeper/internal/config"
	"gatekeeper/internal/router"

	"github.com/go-chi/chi/v5"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Dependencies bündelt alle statischen Abhängigkeiten für den Server
type Dependencies struct {
	Config           *config.GatewayConfig
	PublicKey        *rsa.PublicKey
	TracerShutdown   func(context.Context) error
	AthenaAPIURL     string
	AthenaAPISecret  string
	AthenaContextMapURL string
}

// Server-Struktur hält den Zustand
type Server struct {
	httpServer  *http.Server
	chiRouter   *chi.Mux
	deps        *Dependencies
	redisClient *redis.Client
	routerMutex sync.RWMutex
}

// NewRedisClient
func NewRedisClient(addr string) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
		Password: "",
		DB: 0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Fehler bei der Verbindung zu Redis unter %s: %v", addr, err)
	}
	log.Println("Erfolgreich mit Redis verbunden.")
	return rdb
}

// NewServer initialisiert den Server, Router und die Abhängigkeiten
func NewServer(deps *Dependencies) *Server {
	redisClient := NewRedisClient(deps.Config.RedisAddr)

	s := &Server{
		deps:        deps,
		redisClient: redisClient,
	}

	// Router-Abhängigkeiten vorbereiten
	routerDeps := &router.Dependencies{
		Config:      deps.Config,
		PublicKey:   deps.PublicKey,
		RedisClient: redisClient,
	}

	// Den ersten Router aufsetzen
	s.chiRouter = router.SetupRouter(routerDeps)

	return s
}

// ServeHTTP (unverändert)
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.routerMutex.RLock()
	router := s.chiRouter
	s.routerMutex.RUnlock()

	router.ServeHTTP(w, r)
}

// Start (Bündelt die Start-Logik aus main.go)
func (s *Server) Start() {
	// Metrik-Server starten
	go s.startMetricsServer(s.deps.Config.MetricsPort)

	// Config Poller starten
	go s.startConfigPoller(s.deps.AthenaAPIURL, s.deps.AthenaAPISecret)

	// Haupt-Server starten
	portStr := strconv.Itoa(s.deps.Config.Port)
	addr := fmt.Sprintf(":%s", portStr)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s, // Leitet an s.ServeHTTP -> chiRouter weiter
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	cfg := s.deps.Config
	if cfg.TLSCertPath != "" && cfg.TLSKeyPath != "" {
		fmt.Printf("Gatekeeper gestartet auf https://localhost%s (TLS/HTTPS)\n", addr)
		if err := s.httpServer.ListenAndServeTLS(cfg.TLSCertPath, cfg.TLSKeyPath); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Fehler beim Starten des HTTPS-Servers: %v", err)
		}
	} else {
		fmt.Printf("Gatekeeper gestartet auf http://localhost%s (Kein TLS. Nur für Entwicklung!)\n", addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Fehler beim Starten des HTTP-Servers: %v", err)
		}
	}
}

// WaitForShutdown (Bündelt Shutdown-Logik aus main.go)
func (s *Server) WaitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Println("Fahre Server herunter (Graceful Shutdown)...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if s.deps.TracerShutdown != nil {
		if err := s.deps.TracerShutdown(ctx); err != nil {
			log.Printf("Fehler beim Shutdown des Tracer Providers: %v", err)
		}
	}

	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Fatalf("Fehler beim Graceful Shutdown des Servers: %v", err)
		}
	}

	log.Println("Server erfolgreich heruntergefahren.")
}

// startMetricsServer (aus main.go verschoben)
func (s *Server) startMetricsServer(port int) {
	addr := fmt.Sprintf(":%d", port)
    metricsMux := http.NewServeMux()
    metricsMux.Handle("/metrics", promhttp.Handler())
    fmt.Printf("Prometheus Metriken gestartet auf http://localhost%s/metrics\n", addr)
    
    err := http.ListenAndServe(addr, metricsMux)
    if err != nil && err != http.ErrServerClosed {
        log.Fatalf("Fehler beim Starten des Metrik-Servers auf %s: %v", addr, err)
    }
}

// loadPublicKey (Hier dupliziert für den Poller)
func loadPublicKey(filePath string) (*rsa.PublicKey, error) {
	keyData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Lesen der öffentlichen Schlüsseldatei %s: %w", filePath, err)
	}
	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(keyData)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Parsen des öffentlichen RSA-Schlüssels aus %s: %w", filePath, err)
	}
	return publicKey, nil
}


// startConfigPoller (aus main.go verschoben, jetzt mit Key-Reloading)
func (s *Server) startConfigPoller(apiURL, apiSecret string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	log.Printf("Config Poller gestartet. Prüfe Athena API alle 30s auf %s", apiURL)

	contextMapURL := s.deps.AthenaContextMapURL

	for range ticker.C {
		slog.Debug("Config Poller: Prüfe Athena API auf Änderungen...")

		cfg, err := config.LoadConfigFromAPI(apiURL, contextMapURL, apiSecret)
		if err != nil {
			slog.Warn("Config Poller: FEHLER beim Abrufen der Konfig von Athena", "error", err)
			continue
		}

		currentKeyPath := s.GetPublicKeyPath()
		newKeyPath := cfg.JwtPublicKeyPath
		
		var newPubKey *rsa.PublicKey
		
		if currentKeyPath != newKeyPath {
			slog.Info("Config Poller: Pfad des Public Key hat sich geändert. Lade neu.", "old", currentKeyPath, "new", newKeyPath)
			newPubKey, err = loadPublicKey(newKeyPath)
			if err != nil {
				slog.Warn("Config Poller: FEHLER: Neuer Public Key konnte nicht geladen werden. Reload übersprungen.", "path", newKeyPath, "error", err)
				continue
			}
		} else {
			newPubKey = nil
		}

		if err := s.ReloadConfig(cfg, newPubKey); err != nil {
			slog.Warn("Config Poller: Fehler beim Hot Reload", "error", err)
		} else {
			slog.Info("Config Poller: Hot Reload erfolgreich abgeschlossen.")
		}
	}
}

// ReloadConfig (Aktualisiert, um den Router neu zu erstellen)
func (s *Server) ReloadConfig(newCfg *config.GatewayConfig, newPubKey *rsa.PublicKey) error {
	slog.Debug("Hot Reload wird ausgelöst...")
	
	// 1. Abhängigkeiten aktualisieren
	s.routerMutex.Lock()
	s.deps.Config = newCfg
	if newPubKey != nil {
		s.deps.PublicKey = newPubKey
	}
	
	// Prüfen, ob Redis sich geändert hat
	if newCfg.RedisAddr != s.redisClient.Options().Addr {
		slog.Info("Redis-Adresse hat sich geändert", "old", s.redisClient.Options().Addr, "new", newCfg.RedisAddr)
		
		oldRedisClient := s.redisClient
		s.redisClient = NewRedisClient(newCfg.RedisAddr)

		// Alten Client verzögert schließen
		go func() {
			time.Sleep(5 * time.Second)
			if err := oldRedisClient.Close(); err != nil {
				slog.Warn("Fehler beim Schließen des alten Redis-Clients", "error", err)
			}
		}()
	}
	s.routerMutex.Unlock() // Entsperren VOR Router-Setup

	// 2. Neuen Router erstellen
	routerDeps := &router.Dependencies{
		Config:      s.deps.Config,
		PublicKey:   s.deps.PublicKey,
		RedisClient: s.redisClient,
	}
	newRouter := router.SetupRouter(routerDeps)

	// 3. Router atomar austauschen
	s.routerMutex.Lock()
	s.chiRouter = newRouter
	s.routerMutex.Unlock()

	slog.Info("Hot Reload erfolgreich abgeschlossen.")
	return nil
}

// GetPublicKeyPath (Unverändert)
func (s *Server) GetPublicKeyPath() string {
	s.routerMutex.RLock()
	defer s.routerMutex.RUnlock()
	return s.deps.Config.JwtPublicKeyPath
}