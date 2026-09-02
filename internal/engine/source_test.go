/*
Copyright 2020 The Crossplane Authors.

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

package engine

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	kcache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ cache.Informer = &MockInformer{}

type MockInformer struct {
	cache.Informer

	MockAddEventHandler    func(handler kcache.ResourceEventHandler) (kcache.ResourceEventHandlerRegistration, error)
	MockRemoveEventHandler func(handle kcache.ResourceEventHandlerRegistration) error
	MockIsStopped          func() bool
}

func (m *MockInformer) AddEventHandler(handler kcache.ResourceEventHandler) (kcache.ResourceEventHandlerRegistration, error) {
	return m.MockAddEventHandler(handler)
}

func (m *MockInformer) RemoveEventHandler(handle kcache.ResourceEventHandlerRegistration) error {
	return m.MockRemoveEventHandler(handle)
}

func (m *MockInformer) IsStopped() bool {
	return m.MockIsStopped()
}

func TestStartSource(t *testing.T) {
	type params struct {
		inf cache.Informer
		h   handler.EventHandler
		ps  []predicate.Predicate
	}

	type args struct {
		ctx context.Context
		q   workqueue.TypedRateLimitingInterface[reconcile.Request]
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"AddEventHandlerError": {
			reason: "Start should return an error if it can't add an event handler to the informer.",
			params: params{
				inf: &MockInformer{
					MockAddEventHandler: func(_ kcache.ResourceEventHandler) (kcache.ResourceEventHandlerRegistration, error) {
						return nil, errors.New("boom")
					},
				},
			},
			args: args{
				ctx: context.Background(),
				q:   nil, // Not called, just plumbed down to the event handler.
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"SuccessfulStart": {
			reason: "Start should return nil if it successfully starts the source.",
			params: params{
				inf: &MockInformer{
					MockAddEventHandler: func(_ kcache.ResourceEventHandler) (kcache.ResourceEventHandlerRegistration, error) {
						return nil, nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				q:   nil, // Not called, just plumbed down to the event handler.
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := NewStoppableSource(tc.params.inf, tc.params.h, tc.params.ps...)

			err := s.Start(tc.args.ctx, tc.args.q)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Start(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

var _ kcache.ResourceEventHandlerRegistration = &MockRegistration{}

type MockRegistration struct{}

func (m *MockRegistration) HasSynced() bool { return true }

func TestStopSource(t *testing.T) {
	type params struct {
		inf cache.Informer
		h   handler.EventHandler
		ps  []predicate.Predicate
	}

	type args struct {
		ctx context.Context
		q   workqueue.TypedRateLimitingInterface[reconcile.Request]
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"SuccessfulStop": {
			reason: "Stop should return nil if it successfully stops the source.",
			params: params{
				inf: &MockInformer{
					MockAddEventHandler: func(_ kcache.ResourceEventHandler) (kcache.ResourceEventHandlerRegistration, error) {
						return &MockRegistration{}, nil
					},
					MockRemoveEventHandler: func(_ kcache.ResourceEventHandlerRegistration) error {
						return nil
					},
					MockIsStopped: func() bool { return false },
				},
			},
			args: args{
				ctx: context.Background(),
				q:   nil, // Not called, just plumbed down to the event handler.
			},
			want: want{
				err: nil,
			},
		},
		"RemoveEventHandlerError": {
			reason: "Stop should return an error if it can't remove the source's event handler.",
			params: params{
				inf: &MockInformer{
					MockAddEventHandler: func(_ kcache.ResourceEventHandler) (kcache.ResourceEventHandlerRegistration, error) {
						return &MockRegistration{}, nil
					},
					MockRemoveEventHandler: func(_ kcache.ResourceEventHandlerRegistration) error {
						return errors.New("boom")
					},
					MockIsStopped: func() bool { return false },
				},
			},
			args: args{
				ctx: context.Background(),
				q:   nil, // Not called, just plumbed down to the event handler.
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := NewStoppableSource(tc.params.inf, tc.params.h, tc.params.ps...)

			err := s.Start(tc.args.ctx, tc.args.q)
			if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Start(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			err = s.Stop(tc.args.ctx)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Start(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestStartStoppedSource(t *testing.T) {
	type args struct {
		stopFirst bool
	}

	type want struct {
		err   error
		added bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NotStopped": {
			reason: "Start should add an event handler to the informer.",
			args: args{
				stopFirst: false,
			},
			want: want{
				err:   nil,
				added: true,
			},
		},
		"StoppedFirst": {
			reason: "Start should not add an event handler once the source has been stopped, because nothing would remove it.",
			args: args{
				stopFirst: true,
			},
			want: want{
				err:   nil,
				added: false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			added := false
			inf := &MockInformer{
				MockAddEventHandler: func(_ kcache.ResourceEventHandler) (kcache.ResourceEventHandlerRegistration, error) {
					added = true
					return &MockRegistration{}, nil
				},
				MockRemoveEventHandler: func(_ kcache.ResourceEventHandlerRegistration) error {
					return nil
				},
				MockIsStopped: func() bool { return false },
			}

			s := NewStoppableSource(inf, nil)

			if tc.args.stopFirst {
				// The engine stops a watch it started, whether or not the
				// controller got around to starting the source behind it.
				err := s.Stop(context.Background())
				if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("\n%s\ns.Stop(...): -want error, +got error:\n%s", tc.reason, diff)
				}
			}

			err := s.Start(context.Background(), nil)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Start(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.added, added); diff != "" {
				t.Errorf("\n%s\ns.Start(...) added an event handler: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// Start holds the source lock while it registers the handler, so a Stop that
// arrives in the middle has to wait for the registration before it can remove
// it. Without that, Stop finds nothing to remove and the handler is left behind.
func TestConcurrentStartStopSource(t *testing.T) {
	var added, removed atomic.Int32

	registering := make(chan struct{})
	finishRegistering := make(chan struct{})

	inf := &MockInformer{
		MockAddEventHandler: func(_ kcache.ResourceEventHandler) (kcache.ResourceEventHandlerRegistration, error) {
			added.Add(1)
			close(registering)
			<-finishRegistering

			return &MockRegistration{}, nil
		},
		MockRemoveEventHandler: func(_ kcache.ResourceEventHandlerRegistration) error {
			removed.Add(1)
			return nil
		},
		MockIsStopped: func() bool { return false },
	}

	s := NewStoppableSource(inf, nil)

	started := make(chan error, 1)
	go func() { started <- s.Start(context.Background(), nil) }()

	<-registering

	stopped := make(chan error, 1)
	go func() { stopped <- s.Stop(context.Background()) }()

	// Give Stop a moment to reach the lock. It waits there however long we take,
	// so this only decides whether the two calls overlap, not what they do. A
	// Stop that ran to completion here would find no handler to remove, which is
	// what this test is looking for.
	time.Sleep(100 * time.Millisecond)

	close(finishRegistering)

	if diff := cmp.Diff(nil, <-started, cmpopts.EquateErrors()); diff != "" {
		t.Errorf("s.Start(...): -want error, +got error:\n%s", diff)
	}
	if diff := cmp.Diff(nil, <-stopped, cmpopts.EquateErrors()); diff != "" {
		t.Errorf("s.Stop(...): -want error, +got error:\n%s", diff)
	}

	if got := added.Load(); got != 1 {
		t.Errorf("s.Start(...) added %d event handler(s), want 1", got)
	}
	if got := removed.Load(); got != 1 {
		t.Errorf("s.Stop(...) removed %d event handler(s), want 1", got)
	}
}
