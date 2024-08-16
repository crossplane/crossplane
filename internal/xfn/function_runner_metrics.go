/*
Copyright 2023 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package xfn

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	fnv1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
)

// Metrics are requests, errors, and duration (RED) metrics for composition
// function runs.
type Metrics struct {
	requests  *prometheus.CounterVec
	responses *prometheus.CounterVec
	duration  *prometheus.HistogramVec
}

// NewMetrics creates metrics for composition function runs.
func NewMetrics() *Metrics {
	return &Metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "composition",
			Name:      "run_function_request_total",
			Help:      "Total number of RunFunctionRequests sent.",
		}, []string{"function_name", "function_package", "grpc_target", "grpc_method"}),

		responses: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "composition",
			Name:      "run_function_response_total",
			Help:      "Total number of RunFunctionResponses received.",
		}, []string{"function_name", "function_package", "grpc_target", "grpc_method", "grpc_code", "result_severity"}),

		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: "composition",
			Name:      "run_function_seconds",
			Help:      "Histogram of RunFunctionResponse latency (seconds).",
			Buckets:   prometheus.DefBuckets,
		}, []string{"function_name", "function_package", "grpc_target", "grpc_method", "grpc_code", "result_severity"}),
	}
}

// Describe sends the super-set of all possible descriptors of metrics
// collected by this Collector to the provided channel and returns once
// the last descriptor has been sent.
func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	m.requests.Describe(ch)
	m.responses.Describe(ch)
	m.duration.Describe(ch)
}

// Collect is called by the Prometheus registry when collecting
// metrics. The implementation sends each collected metric via the
// provided channel and returns once the last metric has been sent.
func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	m.requests.Collect(ch)
	m.responses.Collect(ch)
	m.duration.Collect(ch)
}

// CreateInterceptor returns a gRPC UnaryClientInterceptor for the named
// function. The supplied package (pkg) should be the package's OCI reference.
func (m *Metrics) CreateInterceptor(name, pkg string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		l := prometheus.Labels{"function_name": name, "function_package": pkg, "grpc_target": cc.Target(), "grpc_method": method}

		m.requests.With(l).Inc()

		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		duration := time.Since(start)

		s, _ := status.FromError(err)
		l["grpc_code"] = s.Code().String()

		// We consider the 'severity' of the response to be that of the most
		// severe result in the response. A response with no results, or only
		// normal results, has severity "Normal". A response with warnings, but
		// no fatal results, has severity "Warning". A response with fatal
		// results has severity "Fatal".
		l["result_severity"] = "Normal"
		if rsp, ok := reply.(*fnv1.RunFunctionResponse); ok {
			for _, r := range rsp.GetResults() {
				// Keep iterating if we see a warning result - we might still
				// see a fatal result.
				if r.GetSeverity() == fnv1.Severity_SEVERITY_WARNING {
					l["result_severity"] = "Warning"
				}
				// Break if we see a fatal result, to ensure we don't downgrade
				// the severity to warning.
				if r.GetSeverity() == fnv1.Severity_SEVERITY_FATAL {
					l["result_severity"] = "Fatal"
					break
				}
			}
		}

		m.responses.With(l).Inc()
		m.duration.With(l).Observe(duration.Seconds())

		return err
	}
}
