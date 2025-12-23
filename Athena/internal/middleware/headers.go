package middleware

import "net/http"

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Erzwingt HTTPS für 1 Jahr, Production auskommentieren
		// w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// Verhindert Clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Verhindert das "Raten" von MIME-Typen
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Minimal restriktive CSP für eine API (verhindert Laden von UI-Elementen)
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		next.ServeHTTP(w, r)
	})
}