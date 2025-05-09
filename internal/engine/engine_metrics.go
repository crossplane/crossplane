/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package engine

import (
	"github.com/prometheus/client_golang/prometheus"
)

// NopMetrics does nothing.
type NopMetrics struct{}

// ControllerStarted does nothing.
func (m *NopMetrics) ControllerStarted(_ string) {}

// ControllerStopped does nothing.
func (m *NopMetrics) ControllerStopped(_ string) {}

// WatchStarted does nothing.
func (m *NopMetrics) WatchStarted(_ string, _ WatchType) {}

// WatchStopped does nothing.
func (m *NopMetrics) WatchStopped(_ string, _ WatchType) {}

// PrometheusMetrics for the controller engine.
type PrometheusMetrics struct {
	controllersStarted *prometheus.CounterVec
	controllersStopped *prometheus.CounterVec

	watchesStarted *prometheus.CounterVec
	watchesStopped *prometheus.CounterVec
}

// NewPrometheusMetrics exposes controller engine metrics via Prometheus.
func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		controllersStarted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "composition",
			Name:      "controllers_started_total",
			Help:      "Total number of XR controllers started.",
		}, []string{"controller"}),

		controllersStopped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "composition",
			Name:      "controllers_stopped_total",
			Help:      "Total number of XR controllers stopped.",
		}, []string{"controller"}),

		watchesStarted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "composition",
			Name:      "watches_started_total",
			Help:      "Total number of watches started.",
		}, []string{"controller", "type"}),

		watchesStopped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "composition",
			Name:      "watches_stopped_total",
			Help:      "Total number of watches stopped.",
		}, []string{"controller", "type"}),
	}
}

// ControllerStarted records a controller start.
func (m *PrometheusMetrics) ControllerStarted(name string) {
	m.controllersStarted.With(prometheus.Labels{"controller": name}).Inc()
}

// ControllerStopped records a controller stop.
func (m *PrometheusMetrics) ControllerStopped(name string) {
	m.controllersStopped.With(prometheus.Labels{"controller": name}).Inc()
}

// WatchStarted records a watch start for a controller.
func (m *PrometheusMetrics) WatchStarted(name string, t WatchType) {
	m.watchesStarted.With(prometheus.Labels{"controller": name, "type": string(t)}).Inc()
}

// WatchStopped records a watch stop for a controller.
func (m *PrometheusMetrics) WatchStopped(name string, t WatchType) {
	m.watchesStopped.With(prometheus.Labels{"controller": name, "type": string(t)}).Inc()
}

// Describe sends the super-set of all possible descriptors of metrics
// collected by this Collector to the provided channel and returns once
// the last descriptor has been sent.
func (m *PrometheusMetrics) Describe(ch chan<- *prometheus.Desc) {
	m.controllersStarted.Describe(ch)
	m.controllersStopped.Describe(ch)
	m.watchesStarted.Describe(ch)
	m.watchesStopped.Describe(ch)
}

// Collect is called by the Prometheus registry when collecting
// metrics. The implementation sends each collected metric via the
// provided channel and returns once the last metric has been sent.
func (m *PrometheusMetrics) Collect(ch chan<- prometheus.Metric) {
	m.controllersStarted.Collect(ch)
	m.controllersStopped.Collect(ch)
	m.watchesStarted.Collect(ch)
	m.watchesStopped.Collect(ch)
}
