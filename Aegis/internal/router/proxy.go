package router

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings" // WICHTIGER IMPORT
	"time"

	"gatekeeper/internal/circuit"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
)

// NewReverseProxy (aus server.go verschoben)
func NewReverseProxy(targetURL string, breaker *circuit.Breaker, timeout time.Duration) http.Handler {
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Fatalf("Ungültige Ziel-URL: %s", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	transport := http.DefaultTransport.(*http.Transport).Clone()

	if timeout > 0 {
		transport.ResponseHeaderTimeout = timeout
		transport.DialContext = (&net.Dialer{ Timeout: 5 * time.Second }).DialContext
	}

	proxy.Transport = otelhttp.NewTransport(transport,
		otelhttp.WithClientTrace(nil),
		otelhttp.WithPropagators(propagation.TraceContext{}),
	)

	// Director MUSS angepasst werden, um StripPrefix korrekt zu unterstützen
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host // Wichtig für Host-Header-Routing
		
		// Diese Zeile ist entscheidend für http.StripPrefix
		req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy-Fehler zu %s: %v", target.Host, err)
		if breaker != nil { breaker.Fail() }
		if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
			http.Error(w, "Downstream Service Timeout.", http.StatusGatewayTimeout) // 504
			return
		}
		http.Error(w, "Downstream Service nicht erreichbar.", http.StatusServiceUnavailable) // 503
	}
	return proxy
}

// singleJoiningSlash (Hilfsfunktion für den Proxy Director)
func singleJoiningSlash(a, b string) string {
    aslash := strings.HasSuffix(a, "/")
    bslash := strings.HasPrefix(b, "/")
    switch {
    case aslash && bslash:
        return a + b[1:]
    case !aslash && !bslash:
        return a + "/" + b
    }
    return a + b
}