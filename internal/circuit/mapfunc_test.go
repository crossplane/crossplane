package circuit

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane/v2/internal/metrics"
)

type fakeBreaker struct {
	state             State
	recordAllowedCall int
}

func (f *fakeBreaker) GetState(context.Context, types.NamespacedName) State {
	return f.state
}

func (f *fakeBreaker) RecordEvent(context.Context, types.NamespacedName, EventSource) {}

func (f *fakeBreaker) RecordAllowed(context.Context, types.NamespacedName) {
	f.recordAllowedCall++
}

func TestNewMapFuncMetrics(t *testing.T) {
	t.Parallel()

	baseObj := &unstructured.Unstructured{}
	baseObj.SetGroupVersionKind(schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "Dummy"})
	baseObj.SetName("source")
	baseObj.SetNamespace("default")

	controller := "composite/tests.example.org"

	request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "target", Namespace: "default"}}
	wrapped := func(context.Context, client.Object) []reconcile.Request {
		return []reconcile.Request{request}
	}

	now := time.Now()

	cases := map[string]struct {
		breakerState      State
		expectLen         int
		expectedResult    string
		wantRecordAllowed int
	}{
		"AllowedWhenClosed": {
			breakerState:   State{IsOpen: false},
			expectLen:      1,
			expectedResult: metrics.CircuitBreakerResultAllowed,
		},
		"DroppedWhenOpen": {
			breakerState:   State{IsOpen: true, NextAllowedAt: now.Add(time.Minute)},
			expectLen:      0,
			expectedResult: metrics.CircuitBreakerResultDropped,
		},
		"HalfOpenAllowed": {
			breakerState:      State{IsOpen: true, NextAllowedAt: now.Add(-time.Second)},
			expectLen:         1,
			expectedResult:    metrics.CircuitBreakerResultHalfOpenAllowed,
			wantRecordAllowed: 1,
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			reg := prometheus.NewRegistry()
			cbm := metrics.NewPrometheusMetrics()
			reg.MustRegister(cbm)

			fb := &fakeBreaker{state: tc.breakerState}

			mapFn := NewMapFunc(wrapped, fb, cbm, controller)
			result := mapFn(context.Background(), baseObj.DeepCopy())

			if len(result) != tc.expectLen {
				t.Fatalf("expected %d requests, got %d", tc.expectLen, len(result))
			}

			labels := map[string]string{
				"controller": controller,
				"result":     tc.expectedResult,
			}
			if got := metricValue(t, reg, "crossplane_circuit_breaker_events_total", labels); got != 1 {
				t.Fatalf("expected event counter 1 for result %q, got %.0f", tc.expectedResult, got)
			}
			if fb.recordAllowedCall != tc.wantRecordAllowed {
				t.Fatalf("expected RecordAllowed to be called %d times, got %d", tc.wantRecordAllowed, fb.recordAllowedCall)
			}
		})
	}
}
