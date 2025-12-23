package server

import (
	"log/slog"
	"net/http"
	"time"
)

func RequestLogger(next http.Handler) http.Handler {
 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
 		start := time.Now()
 
 		lw := &loggingResponseWriter{ResponseWriter: w}
 		next.ServeHTTP(lw, r)
 
 		duration := time.Since(start)
 
 		attrs := []slog.Attr{
 			slog.String("method", r.Method),
 			slog.String("path", r.URL.Path),
 			slog.String("remote_addr", r.RemoteAddr),
 			slog.Int("status", lw.status),
 			slog.Duration("duration", duration),
 		}

 		if lw.status >= 500 {
 			slog.LogAttrs(r.Context(), slog.LevelError, "HTTP Request (Server Error)", attrs...)
 		} else if lw.status >= 400 {
 			slog.LogAttrs(r.Context(), slog.LevelWarn, "HTTP Request (Client Error)", attrs...)
 		} else {
 			slog.LogAttrs(r.Context(), slog.LevelInfo, "HTTP Request (Success)", attrs...)
 		}
 	})
}
 
type loggingResponseWriter struct {
 	http.ResponseWriter
 	status int
}
 
func (lrw *loggingResponseWriter) WriteHeader(code int) {
 	lrw.status = code
 	lrw.ResponseWriter.WriteHeader(code)
}
 
func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
 	if lrw.status == 0 {
 		lrw.status = http.StatusOK
 	}
 
 	return lrw.ResponseWriter.Write(b)
}