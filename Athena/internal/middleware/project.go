package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// PermissiveProjectIDInjector liest den X-Project-ID Header,
// erzwingt ihn aber nicht (im Gegensatz zu ProjectIDValidator).
// Dies wird für /login und /register benötigt.
func PermissiveProjectIDInjector(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID := r.Header.Get("X-Project-ID") // Kommt von Aegis

		if projectID != "" {
			// Nur wenn der Header vorhanden ist, in den Kontext injizieren
			ctx = context.WithValue(ctx, ProjectIDContextKey, projectID)
			slog.DebugContext(ctx, "Middleware: X-Project-ID Header gefunden und injiziert", "project_id", projectID)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ProjectIDValidator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID := r.Header.Get("X-Project-ID")

		if projectID == "" {
			slog.WarnContext(ctx, "Middleware: X-Project-ID Header fehlt oder ist leer")
			
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "X-Project-ID Header fehlt oder ist leer"})
			return
		}

		ctx = context.WithValue(ctx, ProjectIDContextKey, projectID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}