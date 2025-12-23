package circuit

import (
	"log"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// --- Zustandskonstanten ---
type State int
const (
	Closed State = iota
	Open
	HalfOpen
)

const (
	namespace = "gatekeeper"
	subsystem = "circuitbreaker"
)

var stateGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name: "state",
		Help: "Aktueller Zustand des Circuit Breakers (0=Closed, 1=Open, 2=HalfOpen).",
	},
	[]string{"service"},
)

func init() {
	prometheus.MustRegister(stateGauge)
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{Namespace: namespace, Subsystem: subsystem, Name: "state_closed_value"},
		func() float64 { return float64(Closed) },
	))
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{Namespace: namespace, Subsystem: subsystem, Name: "state_open_value"},
		func() float64 { return float64(Open) },
	))
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{Namespace: namespace, Subsystem: subsystem, Name: "state_half_open_value"},
		func() float64 { return float64(HalfOpen) },
	))
}


type Breaker struct {
	Name string
	State State
	FailureCount int
	LastFailureTime time.Time
	mu sync.Mutex
	Timeout time.Duration
	FailureThreshold int
	
	
	stateGauge prometheus.Gauge
}

func registerBreakerMetrics(b *Breaker) {
	gauge := stateGauge.WithLabelValues(b.Name)
	b.stateGauge = gauge
	b.stateGauge.Set(float64(Closed))
	log.Printf("CIRCUIT METRICS: Breaker Metriken für Service '%s' registriert.", b.Name)
}

func NewBreaker(name string, threshold int, timeout time.Duration) *Breaker {
	b := &Breaker{
		Name: name,
		State: Closed,
		FailureThreshold: threshold,
		Timeout: timeout,
	}
	
	registerBreakerMetrics(b)
	
	return b
}

func (b *Breaker) checkState() State {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.State == Open && time.Since(b.LastFailureTime) > b.Timeout {
		b.State = HalfOpen
		b.FailureCount = 0 
		b.stateGauge.Set(float64(HalfOpen)) 
		
		log.Printf("CIRCUIT BREAKER: Service %s wechselt zu HALF-OPEN.", b.Name)
	}
	return b.State
}

func (b *Breaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if b.State != Closed {
		b.State = Closed
		b.FailureCount = 0
		b.stateGauge.Set(float64(Closed))
		
		log.Printf("CIRCUIT BREAKER: Service %s wechselt zu CLOSED (Erfolgreich zurückgesetzt).", b.Name)
	}
}

func (b *Breaker) Fail() {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if b.State != Open {
		b.FailureCount++
		b.LastFailureTime = time.Now()

		if b.FailureCount >= b.FailureThreshold || b.State == HalfOpen {
			b.State = Open
			b.FailureCount = 0
			b.stateGauge.Set(float64(Open))

			log.Printf("CIRCUIT BREAKER: Service %s wechselt zu OPEN (Schwellenwert erreicht oder Half-Open Test fehlgeschlagen).", b.Name)
		}
	}
}

var BreakerRegistry = struct {
	sync.Mutex
	breakers map[string]*Breaker
}{
	breakers: make(map[string]*Breaker),
}

func GetBreaker(name string, threshold int, timeout time.Duration) *Breaker {
	BreakerRegistry.Lock()
	defer BreakerRegistry.Unlock()

	if b, ok := BreakerRegistry.breakers[name]; ok {
		return b
	}

	b := NewBreaker(name, threshold, timeout)
	BreakerRegistry.breakers[name] = b
	return b
}
