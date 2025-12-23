package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
)

// Gatekeeper ist eine Middleware, die nur Anfragen von vertrauenswürdigen IPs zulässt.
func Gatekeeper(allowedIPs []string) func(next http.Handler) http.Handler {
	ipMap := make(map[string]struct{})
	for _, ip := range allowedIPs {
		ipMap[strings.TrimSpace(ip)] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			
			remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				remoteIP = r.RemoteAddr
			}

			if _, ok := ipMap[remoteIP]; !ok {
				slog.WarnContext(ctx, "Aggressive Block: Verbotene IP", 
					slog.String("remote_ip", remoteIP),
					slog.String("path", r.URL.Path),
				)
				// Aggressives Blocken: 403 Forbidden. 
				// Das "Banning" (z.B. per fail2ban) müsste auf Host-Ebene passieren.
				http.Error(w, `{"error": "forbidden"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}