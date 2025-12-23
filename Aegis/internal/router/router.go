package router

import (
	"crypto/rsa"
	"log"
	"log/slog"
	"net/http"
	"strings"

	"gatekeeper/internal/config"
	"gatekeeper/internal/middleware"
	"gatekeeper/internal/security"

	"github.com/go-chi/chi/v5"
	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Dependencies bündelt die Abhängigkeiten für den Router
type Dependencies struct {
	Config      *config.GatewayConfig
	PublicKey   *rsa.PublicKey
	RedisClient *redis.Client
}

// SetupRouter erstellt und konfiguriert den gesamten Chi-Router
func SetupRouter(deps *Dependencies) *chi.Mux {
	log.Println("Registriere Routen...")
	newRouter := chi.NewRouter()

	// 1. Globale Middlewares
	newRouter.Use(middleware.ContextInjectorMiddleware(deps.Config))
	
	newRouter.Use(func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, "HTTP Request")
	})
	corsCfg := security.DefaultCORSConfig
	if len(deps.Config.Cors.AllowedOrigins) > 0 {
		corsCfg.AllowedOrigins = deps.Config.Cors.AllowedOrigins
		slog.Info("CORS AllowedOrigins konfiguriert", slog.Any("origins", corsCfg.AllowedOrigins))
	}
	newRouter.Use(security.CORSMiddleware(corsCfg))
	newRouter.Use(security.SecurityHeadersMiddleware)
	newRouter.Use(security.PayloadSizeMiddleware)
	newRouter.Use(middleware.RequestLogger)

	// 2. Statische Routen
	
	// /api/auth (unauthentifiziert)
	authHandler, err := buildStaticAuthProxy()
	if err != nil {
		log.Fatalf("FATAL: /api/auth proxy-Fehler: %v", err)
	}
	newRouter.Mount("/api/auth", authHandler)
	log.Println("Registriere statische Auth-Routen (/api/auth/*)")

	// /api/projects (authentifiziert)
	projectsHandler, err := buildStaticAuthenticatedProxy(deps, "/api")
	if err != nil {
		log.Fatalf("FATAL: /api/projects proxy-Fehler: %v", err)
	}
	newRouter.Mount("/api/projects", projectsHandler)
	log.Println("Registriere statische Admin-Routen (/api/projects/*)")

	// /api/users (authentifiziert)
	usersHandler, err := buildStaticAuthenticatedProxy(deps, "/api")
	if err != nil {
		log.Fatalf("FATAL: /api/users proxy-Fehler: %v", err)
	}
	newRouter.Mount("/api/users", usersHandler)
	log.Println("Registriere statische Admin-Routen (/api/users/*)")

	// NEU: /api/admin (authentifiziert)
	adminHandler, err := buildStaticAuthenticatedProxy(deps, "/api")
	if err != nil {
		log.Fatalf("FATAL: /api/admin proxy-Fehler: %v", err)
	}
	newRouter.Mount("/api/admin", adminHandler)
	log.Println("Registriere statische Admin-Routen (/api/admin/*)")


	// 3. Health Check
	newRouter.Get("/health", healthCheckHandler(deps))

	// 4. Dynamische Projekt-Routen
	log.Println("Registriere dynamische Projekt-Routen von Athena...")
	for _, route := range deps.Config.Routes {
		// WICHTIG: /api/admin hier auch überspringen
		if strings.HasPrefix(route.Path, "/api/auth/") ||
			strings.HasPrefix(route.Path, "/api/projects/") ||
			strings.HasPrefix(route.Path, "/api/users/") ||
			strings.HasPrefix(route.Path, "/api/admin/") {
			log.Printf("Überspringe dynamische Route (wird statisch verwaltet): %s", route.Path)
			continue
		}
		
		handler := buildRouteHandler(deps, route)

		// 5. Pfad-Präfix DYNAMISCH entfernen
		registrationPath := route.Path
		if strings.HasSuffix(registrationPath, "/*") {
			mountPrefix := strings.TrimSuffix(registrationPath, "/*")
			
			lastSlashIndex := strings.LastIndex(mountPrefix, "/")
			var stripPrefix string
			if lastSlashIndex > 0 {
				stripPrefix = mountPrefix[:lastSlashIndex]
			} else if mountPrefix != "" {
				stripPrefix = mountPrefix
			} else {
				stripPrefix = ""
			}

			strippedHandler := handler
			if stripPrefix != "" {
				strippedHandler = http.StripPrefix(stripPrefix, handler)
			}
			
			if strings.HasSuffix(mountPrefix, "/auth") {
				strippedHandler = blockStandaloneOtp(stripPrefix, strippedHandler)
			}

			newRouter.Mount(mountPrefix, strippedHandler)
			log.Printf("Route registriert (Mount): %s/* -> %s (Stripping '%s', ...)",
				mountPrefix, route.TargetURL, stripPrefix)

		} else {
			newRouter.Handle(registrationPath, handler)
			log.Printf("Route registriert (Handle Exact): %s -> %s",
				registrationPath, route.TargetURL)
		}
	}

	log.Println("Routen erfolgreich (neu) geladen.")
	return newRouter
}