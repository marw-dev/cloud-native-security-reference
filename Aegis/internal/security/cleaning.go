package security

import (
	"net/http"
	"strings"

	"gatekeeper/internal/auth"
)

var cleanHeadersToStrip = []string{
	"X-Forwarded-For",
	"X-Real-IP", 
	"X-User-ID",      
	"X-User-Roles",   
}

func ClaimAndCleaningMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		for _, header := range cleanHeadersToStrip {
			r.Header.Del(header)
		}

		claims, ok := r.Context().Value(auth.ContextKey).(*auth.CustomClaims)
		if ok {
			r.Header.Set("X-User-ID", claims.UserID)
			
			r.Header.Set("X-User-Roles", strings.Join(claims.Roles, ","))
		}

		next.ServeHTTP(w, r)
	})
}