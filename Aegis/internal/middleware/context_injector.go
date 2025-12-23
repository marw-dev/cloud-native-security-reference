package middleware

import (
	"gatekeeper/internal/config"
	"log/slog"
	"net"
	"net/http"
)

// ContextInjectorMiddleware prüft den Host und fügt X-Project-ID hinzu
func ContextInjectorMiddleware(cfg *config.GatewayConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// Host extrahieren (z.B. "webshop.com:8080" -> "webshop.com")
			host := r.Host
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}

			// 1. Prüfen, ob es der Admin-Host ist
			// (Admin-Requests bekommen NIE eine Projekt-ID)
			if cfg.AdminHost != "" && host == cfg.AdminHost {
				slog.DebugContext(r.Context(), "ContextInjector: Admin-Host erkannt", "host", host)
				next.ServeHTTP(w, r)
				return
			}

			// 2. Prüfen, ob es ein gemappter Kunden-Host ist
			if projectID, ok := cfg.ContextMap[host]; ok {
				slog.DebugContext(r.Context(), "ContextInjector: Host gemappt", "host", host, "project_id", projectID)
				r.Header.Set("X-Project-ID", projectID)
			}
			
			// 3. (Fallback für die AthenaUI)
            // Erlaube der AthenaUI, die ID manuell zu setzen, um
            // Projekte zu verwalten.
			if r.Header.Get("X-Project-ID") == "" { // Nur wenn nicht schon per Host gesetzt
				if manualPID := r.Header.Get("X-Manual-Project-ID"); manualPID != "" {
					slog.DebugContext(r.Context(), "ContextInjector: Manueller Header erkannt", "header_value", manualPID)
					r.Header.Set("X-Project-ID", manualPID)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}