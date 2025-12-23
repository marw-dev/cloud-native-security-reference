package circuit

import (
	"net/http"
)

type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriterInterceptor(w http.ResponseWriter) *responseWriterInterceptor {
	return &responseWriterInterceptor{
		ResponseWriter: w,
		statusCode:	http.StatusOK,
	}
}

func (rwi *responseWriterInterceptor) WriteHeader(statusCode int) {
	rwi.statusCode = statusCode
	rwi.ResponseWriter.WriteHeader(statusCode)
}

func (rwi *responseWriterInterceptor) Write(b []byte) (int, error) {
	return rwi.ResponseWriter.Write(b)
}

func CircuitBreakerMiddleware(breaker *Breaker) func(http.Handler) http.Handler {
	if breaker == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			state := breaker.checkState()

			if state == Open {
				http.Error(w, "Service nicht verfügbar (Circuit Breaker OPEN)", http.StatusServiceUnavailable)
				return
			}

			// Half-Open Test: Wir erlauben den ersten Request. Bei Fehlschlag schaltet der Breaker auf Open.
			if state == HalfOpen {
				// Hier könnte man zusätzlich eine Logik implementieren, um nur EINEN Test-Request
				// durchzulassen, aber die aktuelle Logik im Breaker.Fail() fängt dies ab.
			}

			rwi := newResponseWriterInterceptor(w)
			next.ServeHTTP(rwi, r)

			// Nur 5xx Fehler lösen den Breaker aus
			if rwi.statusCode >= 500 {
				breaker.Fail()
			} else {
				breaker.Success()
			}
		})
	}
}
