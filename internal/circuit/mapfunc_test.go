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

package circuit

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// mockBreaker is a mock implementation of Breaker for mapfunc tests.
type mockBreaker struct {
	NopBreaker
	resetTargetCalls []types.NamespacedName
}

func (m *mockBreaker) ResetTarget(_ context.Context, target types.NamespacedName) {
	m.resetTargetCalls = append(m.resetTargetCalls, target)
}

func TestNewSelfDeleteResetMapFunc(t *testing.T) {
	now := metav1.Now()

	target := types.NamespacedName{Name: "test-xr", Namespace: "default"}
	requests := []reconcile.Request{{NamespacedName: target}}

	inner := func(_ context.Context, _ client.Object) []reconcile.Request {
		return requests
	}

	type args struct {
		wrapped handler.MapFunc
		obj     client.Object
	}

	cases := map[string]struct {
		reason           string
		args             args
		wantRequests     []reconcile.Request
		wantResetTargets []types.NamespacedName
	}{
		"DeletingObjectResetsTarget": {
			reason: "When the watched object has a deletionTimestamp, ResetTarget should be called for each mapped request.",
			args: args{
				wrapped: inner,
				obj: func() client.Object {
					u := &unstructured.Unstructured{}
					u.SetName("test-xr")
					u.SetNamespace("default")
					u.SetDeletionTimestamp(&now)
					return u
				}(),
			},
			wantRequests:     requests,
			wantResetTargets: []types.NamespacedName{target},
		},
		"NonDeletingObjectDoesNotResetTarget": {
			reason: "When the watched object does not have a deletionTimestamp, ResetTarget should not be called.",
			args: args{
				wrapped: inner,
				obj: func() client.Object {
					u := &unstructured.Unstructured{}
					u.SetName("test-xr")
					u.SetNamespace("default")
					return u
				}(),
			},
			wantRequests:     requests,
			wantResetTargets: nil,
		},
		"DeletingObjectMultipleRequests": {
			reason: "ResetTarget should be called for each mapped request when the object is deleting.",
			args: args{
				wrapped: func(_ context.Context, _ client.Object) []reconcile.Request {
					return []reconcile.Request{
						{NamespacedName: types.NamespacedName{Name: "xr-1", Namespace: "ns-1"}},
						{NamespacedName: types.NamespacedName{Name: "xr-2", Namespace: "ns-2"}},
					}
				},
				obj: func() client.Object {
					u := &unstructured.Unstructured{}
					u.SetName("some-resource")
					u.SetNamespace("default")
					u.SetDeletionTimestamp(&now)
					return u
				}(),
			},
			wantRequests: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: "xr-1", Namespace: "ns-1"}},
				{NamespacedName: types.NamespacedName{Name: "xr-2", Namespace: "ns-2"}},
			},
			wantResetTargets: []types.NamespacedName{
				{Name: "xr-1", Namespace: "ns-1"},
				{Name: "xr-2", Namespace: "ns-2"},
			},
		},
		"EmptyRequests": {
			reason: "When the wrapped function returns no requests, ResetTarget should not be called even if the object is deleting.",
			args: args{
				wrapped: func(_ context.Context, _ client.Object) []reconcile.Request {
					return nil
				},
				obj: func() client.Object {
					u := &unstructured.Unstructured{}
					u.SetName("test-xr")
					u.SetNamespace("default")
					u.SetDeletionTimestamp(&now)
					return u
				}(),
			},
			wantRequests:     nil,
			wantResetTargets: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mb := &mockBreaker{}
			fn := NewSelfDeleteResetMapFunc(tc.args.wrapped, mb)

			got := fn(context.Background(), tc.args.obj)

			if diff := cmp.Diff(tc.wantRequests, got); diff != "" {
				t.Errorf("%s\nNewSelfDeleteResetMapFunc(...) requests: -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.wantResetTargets, mb.resetTargetCalls); diff != "" {
				t.Errorf("%s\nNewSelfDeleteResetMapFunc(...) ResetTarget calls: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// openBreaker is fully open: it drops every event, and counts what it's asked
// to record.
type openBreaker struct {
	NopBreaker
	recorded int
}

func (b *openBreaker) GetState(_ context.Context, _ types.NamespacedName) State {
	return State{IsOpen: true, NextAllowedAt: time.Now().Add(time.Hour)}
}

func (b *openBreaker) RecordEvent(_ context.Context, _ types.NamespacedName, _ EventSource, _ EventType) {
	b.recorded++
}

func TestNewMapFuncExemption(t *testing.T) {
	target := types.NamespacedName{Namespace: "default", Name: "xr"}
	wrapped := func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: target}}
	}

	cases := map[string]struct {
		reason       string
		opts         []MapFuncOption
		wantRequests int
		wantRecorded int
	}{
		"NoExemption": {
			reason:       "With the breaker open and no exemption, the event should be dropped, and recorded as dropped.",
			wantRequests: 0,
			wantRecorded: 1,
		},
		"Exempt": {
			reason: "An exempt event should reach its target through an open breaker, and not be recorded, since recording charges it.",
			opts: []MapFuncOption{WithExemption(func(_ context.Context, _ client.Object, _ types.NamespacedName) bool {
				return true
			})},
			wantRequests: 1,
			wantRecorded: 0,
		},
		"NotExempt": {
			reason: "An event the exemption doesn't match should be treated as any other.",
			opts: []MapFuncOption{WithExemption(func(_ context.Context, _ client.Object, _ types.NamespacedName) bool {
				return false
			})},
			wantRequests: 0,
			wantRecorded: 1,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			b := &openBreaker{}
			got := NewMapFunc(wrapped, b, tc.opts...)(context.Background(), &unstructured.Unstructured{})

			if diff := cmp.Diff(tc.wantRequests, len(got)); diff != "" {
				t.Errorf("\n%s\nrequests: -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.wantRecorded, b.recorded); diff != "" {
				t.Errorf("\n%s\nevents recorded: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
