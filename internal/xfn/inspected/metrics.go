/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package inspected

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	_ Metrics = &NopMetrics{}
	_ Metrics = &PrometheusMetrics{}
)

// NopMetrics does nothing.
type NopMetrics struct{}

// ErrorOnRequest does nothing.
func (n NopMetrics) ErrorOnRequest(_ string) {}

// ErrorOnResponse does nothing.
func (n NopMetrics) ErrorOnResponse(_ string) {}

// PrometheusMetrics for the pipeline inspector.
type PrometheusMetrics struct {
	errors *prometheus.CounterVec
}

// NewPrometheusMetrics creates a new PrometheusMetrics.
func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		errors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Subsystem: "function",
				Name:      "run_function_pipeline_inspector_errors_total",
				Help:      "Total number of errors encountered emitting request/response data.",
			},
			[]string{"function_name", "type"},
		),
	}
}

// ErrorOnRequest records errors encountered emitting request/response data.
func (m *PrometheusMetrics) ErrorOnRequest(name string) {
	m.errors.With(prometheus.Labels{"function_name": name, "type": "request"}).Inc()
}

// ErrorOnResponse records errors encountered emitting request/response data.
func (m *PrometheusMetrics) ErrorOnResponse(name string) {
	m.errors.With(prometheus.Labels{"function_name": name, "type": "response"}).Inc()
}

// Describe describes the Prometheus metrics.
func (m *PrometheusMetrics) Describe(ch chan<- *prometheus.Desc) {
	m.errors.Describe(ch)
}

// Collect collects the Prometheus metrics.
func (m *PrometheusMetrics) Collect(ch chan<- prometheus.Metric) {
	m.errors.Collect(ch)
}
