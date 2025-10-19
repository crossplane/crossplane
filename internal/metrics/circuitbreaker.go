package metrics

import (
	"errors"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// CircuitBreakerResultAllowed indicates the event was processed while the circuit
	// was closed.
	CircuitBreakerResultAllowed = "allowed"
	// CircuitBreakerResultDropped indicates the event was dropped while the circuit
	// was open.
	CircuitBreakerResultDropped = "dropped"
	// CircuitBreakerResultHalfOpenAllowed indicates the event was processed while
	// probing a half-open circuit.
	CircuitBreakerResultHalfOpenAllowed = "halfopen_allowed"
)

// CBMetrics records circuit breaker transitions and event outcomes.
type CBMetrics interface {
	IncOpen(controller string)
	IncClose(controller string)
	IncEvent(controller, result string)
}

type cbMetrics struct {
	opens  *prometheus.CounterVec
	closes *prometheus.CounterVec
	events *prometheus.CounterVec

	controllers sync.Map // map[string]*controllerCounters
}

type controllerCounters struct {
	opens  prometheus.Counter
	closes prometheus.Counter

	mu     sync.RWMutex
	events map[string]prometheus.Counter
}

// NewCBMetrics creates circuit breaker metrics backed by the supplied Prometheus
// registerer. Metrics are registered the first time this function is called with
// a given registerer. Subsequent calls reuse the existing collectors.
func NewCBMetrics(reg prometheus.Registerer) CBMetrics {
	m := &cbMetrics{
		opens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "crossplane",
			Name:      "circuit_breaker_opens_total",
			Help:      "Number of times the XR circuit breaker transitioned from closed to open.",
		}, []string{"controller"}),
		closes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "crossplane",
			Name:      "circuit_breaker_closes_total",
			Help:      "Number of times the XR circuit breaker transitioned from open to closed.",
		}, []string{"controller"}),
		events: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "crossplane",
			Name:      "circuit_breaker_events_total",
			Help:      "Number of XR watch events handled by the circuit breaker, labelled by outcome.",
		}, []string{"controller", "result"}),
	}

	registerCounterVec(reg, &m.opens)
	registerCounterVec(reg, &m.closes)
	registerCounterVec(reg, &m.events)

	return m
}

func registerCounterVec(reg prometheus.Registerer, cv **prometheus.CounterVec) {
	if reg == nil {
		return
	}

	if err := reg.Register(*cv); err != nil {
		var already prometheus.AlreadyRegisteredError
		if errors.As(err, &already) {
			if existing, ok := already.ExistingCollector.(*prometheus.CounterVec); ok {
				*cv = existing
				return
			}
		}
		// Avoid crashing the control plane if registration fails; the collector
		// will continue to function locally even if it's not exported.
		return
	}
}

func (m *cbMetrics) IncOpen(controller string) {
	if m == nil {
		return
	}

	cc := m.getController(controller)
	cc.opens.Inc()
}

func (m *cbMetrics) IncClose(controller string) {
	if m == nil {
		return
	}

	cc := m.getController(controller)
	cc.closes.Inc()
}

func (m *cbMetrics) IncEvent(controller, result string) {
	if m == nil {
		return
	}

	cc := m.getController(controller)

	cc.mu.RLock()
	counter, ok := cc.events[result]
	cc.mu.RUnlock()
	if ok {
		counter.Inc()
		return
	}

	cc.mu.Lock()
	counter = cc.events[result]
	if counter == nil {
		counter = m.events.WithLabelValues(controller, result)
		cc.events[result] = counter
	}
	cc.mu.Unlock()

	counter.Inc()
}

func (m *cbMetrics) getController(controller string) *controllerCounters {
	if controller == "" {
		controller = "unknown"
	}

	if cc, ok := m.controllers.Load(controller); ok {
		if counters, ok := cc.(*controllerCounters); ok {
			return counters
		}
	}

	cc := &controllerCounters{
		opens:  m.opens.WithLabelValues(controller),
		closes: m.closes.WithLabelValues(controller),
		events: make(map[string]prometheus.Counter, 3),
	}

	for _, result := range []string{
		CircuitBreakerResultAllowed,
		CircuitBreakerResultDropped,
		CircuitBreakerResultHalfOpenAllowed,
	} {
		cc.events[result] = m.events.WithLabelValues(controller, result)
	}

	actual, loaded := m.controllers.LoadOrStore(controller, cc)
	if loaded {
		if counters, ok := actual.(*controllerCounters); ok {
			return counters
		}
		return cc
	}

	return cc
}
