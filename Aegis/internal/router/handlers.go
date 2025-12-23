package router

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"gatekeeper/internal/auth"
	"gatekeeper/internal/cache"
	"gatekeeper/internal/circuit"
	"gatekeeper/internal/config"
	"gatekeeper/internal/ratelimit"
	"gatekeeper/internal/security"
)

// healthCheckHandler (aus server.go/setupRoutes extrahiert)
func healthCheckHandler(deps *Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		_, err := deps.RedisClient.Ping(ctx).Result()
		if err != nil {
			log.Printf("HEALTH CHECK FEHLER: Redis nicht erreichbar: %v", err)
			http.Error(w, "Gatekeeper Status: NOK (Redis nicht erreichbar)", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Gatekeeper Status: OK"))
	}
}

// blockStandaloneOtp (aus server.go/setupRoutes extrahiert)
func blockStandaloneOtp(stripPrefix string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPathAfterStrip := "/auth/login/otp-standalone"
		if r.URL.Path == expectedPathAfterStrip {
			slog.WarnContext(r.Context(), "Blockierter Versuch (otp-standalone)", slog.String("original_path_prefix", stripPrefix), slog.String("path_after_strip", r.URL.Path), slog.String("remote_addr", r.RemoteAddr))
			http.Error(w, "Dieser Endpunkt ist über das Gateway nicht verfügbar.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// buildStaticAuthProxy (aus server.go/setupRoutes extrahiert)
func buildStaticAuthProxy() (http.Handler, error) {
	athenaURL := os.Getenv("ATHENA_SERVICE_URL")
	if athenaURL == "" {
		log.Fatal("FATAL: ATHENA_SERVICE_URL ist in der .env-Datei nicht gesetzt!")
	}

    
    // (Verwendet NewReverseProxy aus proxy.go)
	proxy := NewReverseProxy(athenaURL, nil, 0) // Kein Breaker oder Timeout für Auth-Proxy

	baseProxyHandler := http.StripPrefix("/api", proxy)

	// Wende den Blocker auf den Auth-Handler an
	return blockStandaloneOtp("/api/auth", baseProxyHandler), nil
}

func buildStaticAuthenticatedProxy(deps *Dependencies, stripPrefix string) (http.Handler, error) {
	athenaURL := os.Getenv("ATHENA_SERVICE_URL")
	if athenaURL == "" {
		log.Fatal("FATAL: ATHENA_SERVICE_URL ist in der .env-Datei nicht gesetzt!")
	}
	proxy := NewReverseProxy(athenaURL, nil, 0)
	
	// 1. Zuerst den Proxy in StripPrefix einpacken
	handler := http.StripPrefix(stripPrefix, proxy)
	
	// 2. WICHTIG: Die Claim/Cleaning-Middleware (wie bei dynamischen Routen)
	// Diese Middleware liest den Body NICHT, sie injiziert nur Header aus dem Kontext.
	handler = security.ClaimAndCleaningMiddleware(handler)
	
	// 3. Auth-Middleware (validiert Token, füllt Kontext für ClaimAndCleaningMiddleware)
	handler = auth.AuthMiddleware(deps.PublicKey)(handler)
	
	return handler, nil
}

// buildRouteHandler (NEU: Extrahiert aus der Schleife in server.go/setupRoutes)
// Erstellt die Middleware-Kette für eine einzelne dynamische Route
func buildRouteHandler(deps *Dependencies, route config.RouteConfig) http.Handler {
	// 1. Reverse Proxy erstellen (Ziel)
	var breaker *circuit.Breaker
	var timeoutDuration time.Duration = 0

	if route.ProxyTimeout != "" {
		var err error
		timeoutDuration, err = time.ParseDuration(route.ProxyTimeout)
		if err != nil {
			log.Fatalf("Ungültiges ProxyTimeout '%s' für Pfad %s: %v", route.ProxyTimeout, route.Path, err)
		}
	}
	cbCfg := route.CircuitBreaker
	if cbCfg.FailureThreshold > 0 {
		cbTimeout, err := time.ParseDuration(cbCfg.OpenTimeout)
		if err != nil {
			log.Fatalf("Ungültiges CircuitBreaker.OpenTimeout '%s' für Pfad %s: %v", cbCfg.OpenTimeout, route.Path, err)
		}
		serviceName := route.TargetURL
		breaker = circuit.GetBreaker(serviceName, cbCfg.FailureThreshold, cbTimeout)
	}
	// (Verwendet NewReverseProxy aus proxy.go)
	handler := NewReverseProxy(route.TargetURL, breaker, timeoutDuration)

	// 2. Routen-spezifische Middlewares anwenden (von innen nach außen)
	if breaker != nil {
		handler = circuit.CircuitBreakerMiddleware(breaker)(handler)
	}
	if route.WebhookSecret != "" {
		if route.WebhookSignatureHeader == "" {
			route.WebhookSignatureHeader = "X-Hub-Signature-256"
		}
		handler = security.WebhookSignatureMiddleware(route.WebhookSecret, route.WebhookSignatureHeader)(handler)
	}
	if len(route.RequiredRoles) > 0 {
		handler = security.ClaimAndCleaningMiddleware(handler)
		handler = auth.ACLMiddleware(route.RequiredRoles)(handler)
		handler = auth.AuthMiddleware(deps.PublicKey)(handler)
	}
	if route.RateLimit.Limit > 0 {
		window, err := time.ParseDuration(route.RateLimit.Window)
		if err != nil { log.Fatalf("Ungültiges RateLimit.Window '%s' für Pfad %s: %v", route.RateLimit.Window, route.Path, err) }
		limitInt := int(route.RateLimit.Limit)
		handler = ratelimit.RateLimitMiddleware(deps.RedisClient, limitInt, window)(handler)
	}
	if route.CacheTTL != "" && route.CacheTTL != "0" && route.CacheTTL != "0s" {
		ttl, err := time.ParseDuration(route.CacheTTL)
		if err != nil { log.Fatalf("Ungültiges CacheTTL '%s' für Pfad %s: %v", route.CacheTTL, route.Path, err) }
		handler = cache.CacheMiddleware(deps.RedisClient, ttl)(handler)
	}
	
	return handler
}