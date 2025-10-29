/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package circuit

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	_ Metrics              = &NopMetrics{}
	_ Metrics              = &PrometheusMetrics{}
	_ prometheus.Collector = &PrometheusMetrics{}
)

// NopMetrics does nothing.
type NopMetrics struct{}

// IncOpen does nothing.
func (m *NopMetrics) IncOpen(_ string) {}

// IncClose does nothing.
func (m *NopMetrics) IncClose(_ string) {}

// IncEvent does nothing.
func (m *NopMetrics) IncEvent(_ string, _ string) {}

// PrometheusMetrics records circuit breaker transitions and results.
type PrometheusMetrics struct {
	opens  *prometheus.CounterVec
	closes *prometheus.CounterVec
	events *prometheus.CounterVec
}

// NewPrometheusMetrics creates circuit breaker metrics backed by Prometheus.
func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		opens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "circuit_breaker",
			Name:      "opens_total",
			Help:      "Number of times the XR circuit breaker transitioned from closed to open.",
		}, []string{"controller"}),

		closes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "circuit_breaker",
			Name:      "closes_total",
			Help:      "Number of times the XR circuit breaker transitioned from open to closed.",
		}, []string{"controller"}),

		events: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "circuit_breaker",
			Name:      "events_total",
			Help:      "Number of XR watch events handled by the circuit breaker, labelled by outcome.",
		}, []string{"controller", "result"}),
	}
}

// IncOpen records a circuit breaker open transition.
func (m *PrometheusMetrics) IncOpen(controller string) {
	m.opens.With(prometheus.Labels{"controller": controller}).Inc()
}

// IncClose records a circuit breaker close transition.
func (m *PrometheusMetrics) IncClose(controller string) {
	m.closes.With(prometheus.Labels{"controller": controller}).Inc()
}

// IncEvent records a circuit breaker event outcome.
func (m *PrometheusMetrics) IncEvent(controller, result string) {
	m.events.With(prometheus.Labels{
		"controller": controller,
		"result":     result,
	}).Inc()
}

// Describe sends the super-set of all possible descriptors of metrics
// collected by this Collector to the provided channel and returns once
// the last descriptor has been sent.
func (m *PrometheusMetrics) Describe(ch chan<- *prometheus.Desc) {
	m.opens.Describe(ch)
	m.closes.Describe(ch)
	m.events.Describe(ch)
}

// Collect is called by the Prometheus registry when collecting metrics.
// The implementation sends each collected metric via the provided channel
// and returns once the last metric has been sent.
func (m *PrometheusMetrics) Collect(ch chan<- prometheus.Metric) {
	m.opens.Collect(ch)
	m.closes.Collect(ch)
	m.events.Collect(ch)
}
