package middleware

import (
	"log/slog"
	"net/http"
	"os"
)

func InternalAuth(next http.Handler) http.Handler {
	internalSecret := os.Getenv("ATHENA_INTERNAL_SECRET")
	if internalSecret == "" {
		slog.Error("FATAL: ATHENA_INTERNAL_SECRET ist nicht gesetzt. Interne API ist deaktiviert.")
		// Gib einen Handler zurück, der immer Fehler wirft
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Interne Authentifizierung nicht konfiguriert", http.StatusInternalServerError)
		})
	}
	
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providedSecret := r.Header.Get("X-Internal-Secret")
		
		if providedSecret != internalSecret {
			slog.WarnContext(r.Context(), "Ungültiger interner Secret-Versuch", "remote_addr", r.RemoteAddr)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}