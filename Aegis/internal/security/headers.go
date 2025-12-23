package security

import "net/http"

func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		
		w.Header().Set("X-Frame-Options", "DENY") 
		w.Header().Set("X-Content-Type-Options", "nosniff") 
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin") 

		csp := "default-src 'self'; " +
			"script-src 'self'; " +
			"style-src 'self' 'unsafe-inline'; " +
			"frame-ancestors 'none';" 
		w.Header().Set("Content-Security-Policy", csp)

		// HTTPS (HSTS) - Auskommentiert, da TLS/HSTS nur bei HTTPS-Start sinnvoll ist
		// w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

const MaxPayloadBytes = 1048576 // 1MB Limit

func PayloadSizeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, MaxPayloadBytes)

		next.ServeHTTP(w, r)
	})
}
