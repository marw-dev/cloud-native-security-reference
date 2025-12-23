package ratelimit

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Define Konstanten für Metrik-Namen
const (
	namespace = "gatekeeper"
	subsystem = "ratelimit"
)

var rateLimitRequests = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name: "requests_total",
		Help: "Gesamtzahl der Rate-Limit-Anfragen, nach Ergebnis (allowed/denied).",
	},
	[]string{"result"},
)

func init() {
	// Registriere den CounterVec beim globalen Prometheus Registry
	prometheus.MustRegister(rateLimitRequests)
}
