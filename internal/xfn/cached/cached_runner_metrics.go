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

package cached

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	_ Metrics = &NopMetrics{}
	_ Metrics = &PrometheusMetrics{}
)

// NopMetrics does nothing.
type NopMetrics struct{}

// Hit does nothing.
func (m *NopMetrics) Hit(_ string) {}

// Miss does nothing.
func (m *NopMetrics) Miss(_ string) {}

// Error does nothing.
func (m *NopMetrics) Error(_ string) {}

// Write does nothing.
func (m *NopMetrics) Write(_ string) {}

// Delete does nothing.
func (m *NopMetrics) Delete(_ string) {}

// WroteBytes does nothing.
func (m *NopMetrics) WroteBytes(_ string, _ int) {}

// DeletedBytes does nothing.
func (m *NopMetrics) DeletedBytes(_ string, _ int) {}

// ReadDuration does nothing.
func (m *NopMetrics) ReadDuration(_ string, _ time.Duration) {}

// WriteDuration does nothing.
func (m *NopMetrics) WriteDuration(_ string, _ time.Duration) {}

// PrometheusMetrics for the function response cache.
type PrometheusMetrics struct {
	hits   *prometheus.CounterVec
	misses *prometheus.CounterVec
	errors *prometheus.CounterVec

	writes  *prometheus.CounterVec
	deletes *prometheus.CounterVec

	bytesWritten *prometheus.CounterVec
	bytesDeleted *prometheus.CounterVec

	readDuration  *prometheus.HistogramVec
	writeDuration *prometheus.HistogramVec
}

// NewPrometheusMetrics exposes function response cache metrics via Prometheus.
func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		hits: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "composition",
			Name:      "run_function_response_cache_hits_total",
			Help:      "Total number of RunFunctionResponse cache hits.",
		}, []string{"function_name"}),

		misses: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "composition",
			Name:      "run_function_response_cache_misses_total",
			Help:      "Total number of RunFunctionResponse cache misses.",
		}, []string{"function_name"}),

		errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "composition",
			Name:      "run_function_response_cache_errors_total",
			Help:      "Total number of RunFunctionResponse cache errors.",
		}, []string{"function_name"}),

		writes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "composition",
			Name:      "run_function_response_cache_writes_total",
			Help:      "Total number of RunFunctionResponses cache writes.",
		}, []string{"function_name"}),

		deletes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "composition",
			Name:      "run_function_response_cache_deletes_total",
			Help:      "Total number of RunFunctionResponses cache deletes.",
		}, []string{"function_name"}),

		bytesWritten: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "composition",
			Name:      "run_function_response_cache_bytes_written_total",
			Help:      "Total number of RunFunctionResponse bytes written.",
		}, []string{"function_name"}),

		bytesDeleted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "composition",
			Name:      "run_function_response_cache_bytes_deleted_total",
			Help:      "Total number of RunFunctionResponse bytes deleted.",
		}, []string{"function_name"}),

		readDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: "composition",
			Name:      "run_function_response_cache_read_seconds",
			Help:      "Histogram of RunFunctionResponse cache read time (seconds).",
			Buckets:   prometheus.DefBuckets,
		}, []string{"function_name"}),

		writeDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: "composition",
			Name:      "run_function_response_cache_write_seconds",
			Help:      "Histogram of RunFunctionResponse cache write time (seconds).",
			Buckets:   prometheus.DefBuckets,
		}, []string{"function_name"}),
	}
}

// Hit records a cache hit.
func (m *PrometheusMetrics) Hit(name string) {
	m.hits.With(prometheus.Labels{"function_name": name}).Inc()
}

// Miss records a cache miss.
func (m *PrometheusMetrics) Miss(name string) {
	m.misses.With(prometheus.Labels{"function_name": name}).Inc()
}

// Error records a cache error.
func (m *PrometheusMetrics) Error(name string) {
	m.errors.With(prometheus.Labels{"function_name": name}).Inc()
}

// Write records a cache write.
func (m *PrometheusMetrics) Write(name string) {
	m.writes.With(prometheus.Labels{"function_name": name}).Inc()
}

// Delete records a cache delete, i.e. due to garbage collection.
func (m *PrometheusMetrics) Delete(name string) {
	m.deletes.With(prometheus.Labels{"function_name": name}).Inc()
}

// WroteBytes records bytes written from the cache.
func (m *PrometheusMetrics) WroteBytes(name string, b int) {
	m.bytesWritten.With(prometheus.Labels{"function_name": name}).Add(float64(b))
}

// DeletedBytes records bytes deleted from the cache.
func (m *PrometheusMetrics) DeletedBytes(name string, b int) {
	m.bytesDeleted.With(prometheus.Labels{"function_name": name}).Add(float64(b))
}

// ReadDuration records the time taken by a cache hit.
func (m *PrometheusMetrics) ReadDuration(name string, d time.Duration) {
	m.readDuration.With(prometheus.Labels{"function_name": name}).Observe(d.Seconds())
}

// WriteDuration records the time taken to write to cache.
func (m *PrometheusMetrics) WriteDuration(name string, d time.Duration) {
	m.writeDuration.With(prometheus.Labels{"function_name": name}).Observe(d.Seconds())
}

// Describe sends the super-set of all possible descriptors of metrics
// collected by this Collector to the provided channel and returns once
// the last descriptor has been sent.
func (m *PrometheusMetrics) Describe(ch chan<- *prometheus.Desc) {
	m.hits.Describe(ch)
	m.misses.Describe(ch)
	m.errors.Describe(ch)
	m.writes.Describe(ch)
	m.deletes.Describe(ch)
	m.bytesWritten.Describe(ch)
	m.bytesDeleted.Describe(ch)
	m.readDuration.Describe(ch)
	m.writeDuration.Describe(ch)
}

// Collect is called by the Prometheus registry when collecting
// metrics. The implementation sends each collected metric via the
// provided channel and returns once the last metric has been sent.
func (m *PrometheusMetrics) Collect(ch chan<- prometheus.Metric) {
	m.hits.Collect(ch)
	m.misses.Collect(ch)
	m.errors.Collect(ch)
	m.writes.Collect(ch)
	m.deletes.Collect(ch)
	m.bytesWritten.Collect(ch)
	m.bytesDeleted.Collect(ch)
	m.readDuration.Collect(ch)
	m.writeDuration.Collect(ch)
}
