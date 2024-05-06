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
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var _ TrackingInformers = &MockTrackingInformers{}

type MockTrackingInformers struct {
	cache.Informers

	MockActiveInformers func() []schema.GroupVersionKind
	MockGetInformer     func(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error)
	MockRemoveInformer  func(ctx context.Context, obj client.Object) error
}

func (m *MockTrackingInformers) ActiveInformers() []schema.GroupVersionKind {
	return m.MockActiveInformers()
}

func (m *MockTrackingInformers) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	return m.MockGetInformer(ctx, obj, opts...)
}

func (m *MockTrackingInformers) RemoveInformer(ctx context.Context, obj client.Object) error {
	return m.MockRemoveInformer(ctx, obj)
}

var _ manager.Manager = &MockManager{}

type MockManager struct {
	manager.Manager

	MockElected   func() <-chan struct{}
	MockGetScheme func() *runtime.Scheme
}

func (m *MockManager) Elected() <-chan struct{} {
	return m.MockElected()
}

func (m *MockManager) GetScheme() *runtime.Scheme {
	return m.MockGetScheme()
}

var _ WatchGarbageCollector = &MockWatchGarbageCollector{}

type MockWatchGarbageCollector struct {
	MockGarbageCollectWatches func(ctx context.Context, interval time.Duration)
}

func (m *MockWatchGarbageCollector) GarbageCollectWatches(ctx context.Context, interval time.Duration) {
	m.MockGarbageCollectWatches(ctx, interval)
}

var _ kcontroller.Controller = &MockController{}

type MockController struct {
	kcontroller.Controller

	MockStart func(ctx context.Context) error
	MockWatch func(src source.Source) error
}

func (m *MockController) Start(ctx context.Context) error {
	return m.MockStart(ctx)
}

func (m *MockController) Watch(src source.Source) error {
	return m.MockWatch(src)
}

func TestStartController(t *testing.T) {
	type params struct {
		mgr  manager.Manager
		infs TrackingInformers
		c    client.Client
		opts []ControllerEngineOption
	}
	type args struct {
		name string
		opts []ControllerOption
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
		"NewControllerError": {
			reason: "Start should return an error if it can't create a new controller",
			params: params{
				mgr: &MockManager{
					MockElected: func() <-chan struct{} {
						e := make(chan struct{})
						close(e)
						return e
					},
				},
				infs: &MockTrackingInformers{},
			},
			args: args{
				name: "cool-controller",
				opts: []ControllerOption{
					WithNewControllerFn(func(_ string, _ manager.Manager, _ kcontroller.Options) (kcontroller.Controller, error) {
						return nil, errors.New("boom")
					}),
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"StartControllerError": {
			reason: "Start won't return an error if it can't start the new controller in a goroutine.",
			params: params{
				mgr: &MockManager{
					MockElected: func() <-chan struct{} {
						e := make(chan struct{})
						close(e)
						return e
					},
				},
				infs: &MockTrackingInformers{},
			},
			args: args{
				name: "cool-controller",
				opts: []ControllerOption{
					WithNewControllerFn(func(_ string, _ manager.Manager, _ kcontroller.Options) (kcontroller.Controller, error) {
						return &MockController{
							MockStart: func(_ context.Context) error {
								return errors.New("boom")
							},
						}, nil
					}),
				},
			},
			// TODO(negz): Test that the error was logged? We usually don't.
			want: want{
				err: nil,
			},
		},
		"SuccessfulStart": {
			reason: "It should be possible to successfully start a controller.",
			params: params{
				mgr: &MockManager{
					MockElected: func() <-chan struct{} {
						e := make(chan struct{})
						close(e)
						return e
					},
				},
				infs: &MockTrackingInformers{},
			},
			args: args{
				name: "cool-controller",
				opts: []ControllerOption{
					WithNewControllerFn(func(_ string, _ manager.Manager, _ kcontroller.Options) (kcontroller.Controller, error) {
						return &MockController{
							MockStart: func(ctx context.Context) error {
								<-ctx.Done()
								return nil
							},
						}, nil
					}),
					WithWatchGarbageCollector(&MockWatchGarbageCollector{
						MockGarbageCollectWatches: func(ctx context.Context, _ time.Duration) {
							<-ctx.Done()
						},
					}),
				},
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := New(tc.params.mgr, tc.params.infs, tc.params.c, tc.params.opts...)
			err := e.Start(tc.args.name, tc.args.opts...)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Start(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			// Starting the controller second time should be a no-op.
			err = e.Start(tc.args.name, tc.args.opts...)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Start(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			// Stop the controller. Will be a no-op if it never started.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err = e.Stop(ctx, tc.args.name)
			if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Stop(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestIsRunning(t *testing.T) {
	type params struct {
		mgr  manager.Manager
		infs TrackingInformers
		c    client.Client
		opts []ControllerEngineOption
	}

	// We need to control how we start the controller.
	type argsStart struct {
		name string
		opts []ControllerOption
	}
	type args struct {
		name string
	}
	type want struct {
		running bool
	}
	cases := map[string]struct {
		reason    string
		params    params
		argsStart argsStart
		args      args
		want      want
	}{
		"SuccessfulStart": {
			reason: "IsRunning should return true if the controller successfully starts.",
			params: params{
				mgr: &MockManager{
					MockElected: func() <-chan struct{} {
						e := make(chan struct{})
						close(e)
						return e
					},
				},
				infs: &MockTrackingInformers{},
			},
			argsStart: argsStart{
				name: "cool-controller",
				opts: []ControllerOption{
					WithNewControllerFn(func(_ string, _ manager.Manager, _ kcontroller.Options) (kcontroller.Controller, error) {
						return &MockController{
							MockStart: func(ctx context.Context) error {
								<-ctx.Done()
								return nil
							},
						}, nil
					}),
				},
			},
			args: args{
				name: "cool-controller",
			},
			want: want{
				running: true,
			},
		},
		"StartControllerError": {
			reason: "IsRunning should return false if the controller didn't successfully start.",
			params: params{
				mgr: &MockManager{
					MockElected: func() <-chan struct{} {
						e := make(chan struct{})
						close(e)
						return e
					},
				},
				infs: &MockTrackingInformers{},
			},
			argsStart: argsStart{
				name: "cool-controller",
				opts: []ControllerOption{
					WithNewControllerFn(func(_ string, _ manager.Manager, _ kcontroller.Options) (kcontroller.Controller, error) {
						return &MockController{
							MockStart: func(_ context.Context) error {
								return errors.New("boom")
							},
						}, nil
					}),
				},
			},
			args: args{
				name: "cool-controller",
			},
			want: want{
				running: false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := New(tc.params.mgr, tc.params.infs, tc.params.c, tc.params.opts...)
			_ = e.Start(tc.args.name, tc.argsStart.opts...)

			// Give the start goroutine a little time to fail.
			time.Sleep(1 * time.Second)

			running := e.IsRunning(tc.args.name)
			if diff := cmp.Diff(tc.want.running, running); diff != "" {
				t.Errorf("\n%s\ne.IsRunning(...): -want, +got:\n%s", tc.reason, diff)
			}

			// Stop the controller. Will be a no-op if it never started.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = e.Stop(ctx, tc.args.name)

			// IsRunning should always be false after the controller is stopped.
			running = e.IsRunning(tc.args.name)
			if diff := cmp.Diff(false, running); diff != "" {
				t.Errorf("\n%s\ne.IsRunning(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestStopController(t *testing.T) {
	type params struct {
		mgr  manager.Manager
		infs TrackingInformers
		c    client.Client
		opts []ControllerEngineOption
	}
	type args struct {
		ctx  context.Context
		name string
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
			reason: "It should be possible to successfully stop a controller.",
			params: params{
				mgr: &MockManager{
					MockElected: func() <-chan struct{} {
						e := make(chan struct{})
						close(e)
						return e
					},
					MockGetScheme: runtime.NewScheme,
				},
				infs: &MockTrackingInformers{
					MockActiveInformers: func() []schema.GroupVersionKind {
						return nil
					},
				},
			},
			args: args{
				ctx:  context.Background(),
				name: "cool-controller",
			},
			want: want{
				err: nil,
			},
		},
		// TODO(negz): Test handling watches that fail to stop? I'm not sure
		// it's worth the amount of complexity making StoppableSource injectable
		// would add. We could make Watch an interface with a GetSource.
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := New(tc.params.mgr, tc.params.infs, tc.params.c, tc.params.opts...)
			err := e.Start(tc.args.name, WithNewControllerFn(func(_ string, _ manager.Manager, _ kcontroller.Options) (kcontroller.Controller, error) {
				return &MockController{
					MockStart: func(ctx context.Context) error {
						<-ctx.Done()
						return nil
					},
					MockWatch: func(_ source.Source) error {
						return nil
					},
				}, nil
			}))
			if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("\n%s\ne.Start(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			// Add a watch for stop to stop.
			u := &unstructured.Unstructured{}
			u.SetAPIVersion("test.crossplane.io/v1")
			u.SetKind("Composed")
			err = e.StartWatches(tc.args.name, WatchFor(u, WatchTypeComposedResource, nil))
			if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.StartWatches(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			// Stop the controller. Will be a no-op if it never started.
			err = e.Stop(tc.args.ctx, tc.args.name)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Stop(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			// Stop should be a no-op when called on a stopped controller.
			err = e.Stop(tc.args.ctx, tc.args.name)
			if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Stop(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestStartWatches(t *testing.T) {
	type params struct {
		mgr  manager.Manager
		infs TrackingInformers
		c    client.Client
		opts []ControllerEngineOption
	}
	// We need to control how we start the controller.
	type argsStart struct {
		name string
		opts []ControllerOption
	}
	type args struct {
		name string
		ws   []Watch
	}
	type want struct {
		err     error
		watches []WatchID
	}
	cases := map[string]struct {
		reason    string
		params    params
		argsStart argsStart
		args      args
		want      want
	}{
		"StartWatchError": {
			reason: "StartWatches should return an error when a watch fails to start.",
			params: params{
				mgr: &MockManager{
					MockElected: func() <-chan struct{} {
						e := make(chan struct{})
						close(e)
						return e
					},
					MockGetScheme: runtime.NewScheme,
				},
				infs: &MockTrackingInformers{
					MockActiveInformers: func() []schema.GroupVersionKind {
						return []schema.GroupVersionKind{
							{
								Group:   "test.crossplane.io",
								Version: "v1",
								Kind:    "Composed",
							},
						}
					},
				},
			},
			argsStart: argsStart{
				name: "cool-controller",
				opts: []ControllerOption{
					WithNewControllerFn(func(_ string, _ manager.Manager, _ kcontroller.Options) (kcontroller.Controller, error) {
						return &MockController{
							MockStart: func(ctx context.Context) error {
								<-ctx.Done()
								return nil
							},
							MockWatch: func(_ source.Source) error {
								return errors.New("boom")
							},
						}, nil
					}),
				},
			},
			args: args{
				name: "cool-controller",
				ws: []Watch{
					func() Watch {
						u := &unstructured.Unstructured{}
						u.SetAPIVersion("test.crossplane.io/v1")
						u.SetKind("Composed")
						return WatchFor(u, WatchTypeComposedResource, nil)
					}(),
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"SuccessfulStartWatches": {
			reason: "StartWatches shouldn't return an error when all watches start successfully.",
			params: params{
				mgr: &MockManager{
					MockElected: func() <-chan struct{} {
						e := make(chan struct{})
						close(e)
						return e
					},
					MockGetScheme: runtime.NewScheme,
				},
				infs: &MockTrackingInformers{
					MockActiveInformers: func() []schema.GroupVersionKind {
						return []schema.GroupVersionKind{
							{
								Group:   "test.crossplane.io",
								Version: "v1",
								Kind:    "Resource",
							},
						}
					},
				},
			},
			argsStart: argsStart{
				name: "cool-controller",
				opts: []ControllerOption{
					WithNewControllerFn(func(_ string, _ manager.Manager, _ kcontroller.Options) (kcontroller.Controller, error) {
						return &MockController{
							MockStart: func(ctx context.Context) error {
								<-ctx.Done()
								return nil
							},
							MockWatch: func(_ source.Source) error {
								return nil
							},
						}, nil
					}),
				},
			},
			args: args{
				name: "cool-controller",
				ws: []Watch{
					func() Watch {
						u := &unstructured.Unstructured{}
						u.SetAPIVersion("test.crossplane.io/v1")
						u.SetKind("Resource")
						return WatchFor(u, WatchTypeComposedResource, nil)
					}(),
					// This should be deduplicated into the above watch.
					func() Watch {
						u := &unstructured.Unstructured{}
						u.SetAPIVersion("test.crossplane.io/v1")
						u.SetKind("Resource")
						return WatchFor(u, WatchTypeComposedResource, nil)
					}(),
					// This shouldn't be deduplicated, because it's a different
					// watch type.
					func() Watch {
						u := &unstructured.Unstructured{}
						u.SetAPIVersion("test.crossplane.io/v1")
						u.SetKind("Resource")
						return WatchFor(u, WatchTypeCompositeResource, nil)
					}(),
				},
			},
			want: want{
				err: nil,
				watches: []WatchID{
					{
						Type: WatchTypeComposedResource,
						GVK: schema.GroupVersionKind{
							Group:   "test.crossplane.io",
							Version: "v1",
							Kind:    "Resource",
						},
					},
					{
						Type: WatchTypeCompositeResource,
						GVK: schema.GroupVersionKind{
							Group:   "test.crossplane.io",
							Version: "v1",
							Kind:    "Resource",
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := New(tc.params.mgr, tc.params.infs, tc.params.c, tc.params.opts...)
			err := e.Start(tc.argsStart.name, tc.argsStart.opts...)
			if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("\n%s\ne.Start(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			err = e.StartWatches(tc.args.name, tc.args.ws...)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.StartWatches(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			// Start the same watches again to exercise the code that ensures we
			// only add each watch once.
			err = e.StartWatches(tc.args.name, tc.args.ws...)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.StartWatches(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			watches, err := e.GetWatches(tc.args.name)
			if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.GetWatches(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.watches, watches,
				cmpopts.EquateEmpty(),
				cmpopts.SortSlices(func(a, b WatchID) bool { return fmt.Sprintf("%s", a) > fmt.Sprintf("%s", b) }),
			); diff != "" {
				t.Errorf("\n%s\ne.StartWatches(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			// Stop the controller. Will be a no-op if it never started.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err = e.Stop(ctx, tc.args.name)
			if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Stop(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestStopWatches(t *testing.T) {
	type params struct {
		mgr  manager.Manager
		infs TrackingInformers
		c    client.Client
		opts []ControllerEngineOption
	}
	type args struct {
		ctx  context.Context
		name string
		ws   []WatchID
	}
	type want struct {
		stopped int
		err     error
		watches []WatchID
	}
	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"NoWatchesToStop": {
			reason: "StopWatches should be a no-op when there's no watches to stop.",
			params: params{
				mgr: &MockManager{
					MockElected: func() <-chan struct{} {
						e := make(chan struct{})
						close(e)
						return e
					},
					MockGetScheme: runtime.NewScheme,
				},
				infs: &MockTrackingInformers{
					MockActiveInformers: func() []schema.GroupVersionKind {
						return nil
					},
				},
			},
			args: args{
				ctx:  context.Background(),
				name: "cool-controller",
				ws: []WatchID{
					{
						Type: WatchTypeCompositeResource,
						GVK: schema.GroupVersionKind{
							Group:   "test.crossplane.io",
							Version: "v1",
							Kind:    "NeverStarted",
						},
					},
				},
			},
			want: want{
				stopped: 0,
				err:     nil,
				watches: []WatchID{
					{
						Type: WatchTypeComposedResource,
						GVK: schema.GroupVersionKind{
							Group:   "test.crossplane.io",
							Version: "v1",
							Kind:    "Resource",
						},
					},
					{
						Type: WatchTypeCompositeResource,
						GVK: schema.GroupVersionKind{
							Group:   "test.crossplane.io",
							Version: "v1",
							Kind:    "Resource",
						},
					},
				},
			},
		},
		"StopOneWatch": {
			reason: "StopWatches should only stop the watches it's asked to.",
			params: params{
				mgr: &MockManager{
					MockElected: func() <-chan struct{} {
						e := make(chan struct{})
						close(e)
						return e
					},
					MockGetScheme: runtime.NewScheme,
				},
				infs: &MockTrackingInformers{
					MockActiveInformers: func() []schema.GroupVersionKind {
						return nil
					},
				},
			},
			args: args{
				ctx:  context.Background(),
				name: "cool-controller",
				ws: []WatchID{
					{
						Type: WatchTypeComposedResource,
						GVK: schema.GroupVersionKind{
							Group:   "test.crossplane.io",
							Version: "v1",
							Kind:    "Resource",
						},
					},
				},
			},
			want: want{
				stopped: 1,
				err:     nil,
				watches: []WatchID{
					{
						Type: WatchTypeCompositeResource,
						GVK: schema.GroupVersionKind{
							Group:   "test.crossplane.io",
							Version: "v1",
							Kind:    "Resource",
						},
					},
				},
			},
		},
		"StopAllWatches": {
			reason: "StopWatches should stop all watches when asked to.",
			params: params{
				mgr: &MockManager{
					MockElected: func() <-chan struct{} {
						e := make(chan struct{})
						close(e)
						return e
					},
					MockGetScheme: runtime.NewScheme,
				},
				infs: &MockTrackingInformers{
					MockActiveInformers: func() []schema.GroupVersionKind {
						return nil
					},
				},
			},
			args: args{
				ctx:  context.Background(),
				name: "cool-controller",
				ws: []WatchID{
					{
						Type: WatchTypeComposedResource,
						GVK: schema.GroupVersionKind{
							Group:   "test.crossplane.io",
							Version: "v1",
							Kind:    "Resource",
						},
					},
					{
						Type: WatchTypeCompositeResource,
						GVK: schema.GroupVersionKind{
							Group:   "test.crossplane.io",
							Version: "v1",
							Kind:    "Resource",
						},
					},
					{
						Type: WatchTypeCompositeResource,
						GVK: schema.GroupVersionKind{
							Group:   "test.crossplane.io",
							Version: "v1",
							Kind:    "NeverStarted",
						},
					},
				},
			},
			want: want{
				stopped: 2,
				err:     nil,
				watches: []WatchID{},
			},
		},
		// TODO(negz): Test handling watches that fail to stop? I'm not sure
		// it's worth the amount of complexity making StoppableSource injectable
		// would add. We could make Watch an interface with a GetSource.
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := New(tc.params.mgr, tc.params.infs, tc.params.c, tc.params.opts...)
			err := e.Start(tc.args.name, WithNewControllerFn(func(_ string, _ manager.Manager, _ kcontroller.Options) (kcontroller.Controller, error) {
				return &MockController{
					MockStart: func(ctx context.Context) error {
						<-ctx.Done()
						return nil
					},
					MockWatch: func(_ source.Source) error {
						return nil
					},
				}, nil
			}))
			if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("\n%s\ne.Start(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			// Add some watches to stop.
			u1 := &unstructured.Unstructured{}
			u1.SetAPIVersion("test.crossplane.io/v1")
			u1.SetKind("Resource")
			err = e.StartWatches(tc.args.name,
				WatchFor(u1, WatchTypeComposedResource, nil),
				WatchFor(u1, WatchTypeCompositeResource, nil),
			)
			if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.StartWatches(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			stopped, err := e.StopWatches(tc.args.ctx, tc.args.name, tc.args.ws...)
			if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.StopWatches(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.stopped, stopped); diff != "" {
				t.Errorf("\n%s\ne.StopWatches(...): -want stopped, +got stopped:\n%s", tc.reason, diff)
			}

			watches, err := e.GetWatches(tc.args.name)
			if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.GetWatches(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.watches, watches,
				cmpopts.EquateEmpty(),
				cmpopts.SortSlices(func(a, b WatchID) bool { return fmt.Sprintf("%s", a) > fmt.Sprintf("%s", b) }),
			); diff != "" {
				t.Errorf("\n%s\ne.StartWatches(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			// Stop the controller. Will be a no-op if it never started.
			err = e.Stop(tc.args.ctx, tc.args.name)
			if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Stop(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
