package ratelimit

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"gatekeeper/internal/auth"

	"github.com/go-redis/redis/v8"
)

// RateLimitMiddleware wendet ein Rate Limit auf eingehende Anfragen an.
func RateLimitMiddleware(client *redis.Client, limit int, window time.Duration) func(http.Handler) http.Handler {
	if limit <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			key := getRateLimitKey(r)
			if key == "" {
				// Wenn kein Key bestimmt werden kann (z.B. keine IP), lassen wir den Request durch
				// und zählen ihn als 'allowed' (da das Limit in diesem Fall nicht gilt)
				rateLimitRequests.WithLabelValues("allowed").Inc() // NEU: Metrik für erlaubte Anfragen
				next.ServeHTTP(w, r)
				return
			}

			currentCount, err := client.Incr(context.Background(), key).Result()
			if err != nil {
				log.Printf("Redis-Fehler beim Rate Limiting: %v", err)
				// Fail-Open: Bei Redis-Fehler erlauben wir den Request, um Verfügbarkeit zu gewährleisten.
				next.ServeHTTP(w, r)
				return
			}

			if currentCount == 1 {
				// Setze das Verfallsdatum nur beim ersten Zählerstand
				client.Expire(context.Background(), key, window)
			}

			if int(currentCount) > limit {
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", window.Seconds()))
				http.Error(w, "Rate Limit überschritten", http.StatusTooManyRequests) // HTTP 429
				rateLimitRequests.WithLabelValues("denied").Inc() // NEU: Metrik für abgelehnte Anfragen
				return
			}
			
			rateLimitRequests.WithLabelValues("allowed").Inc() // NEU: Metrik für erlaubte Anfragen
			next.ServeHTTP(w, r)
		})
	}
}

// getRateLimitKey extrahiert den eindeutigen Schlüssel für das Rate Limiting (UserID oder IP).
func getRateLimitKey(r *http.Request) string {
	// 1. Versuche, den Schlüssel über den authentifizierten Benutzer zu erhalten
	if claims, ok := r.Context().Value(auth.ContextKey).(*auth.CustomClaims); ok {
		return "user:" + claims.UserID
	}

	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		return "ip:" + strings.TrimSpace(parts[0])
	}
    
    if r.RemoteAddr != "" {
        return "ip:" + r.RemoteAddr
    }

	return "" 
}
