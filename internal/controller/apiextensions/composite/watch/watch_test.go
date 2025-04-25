/*
Copyright 2024 The Crossplane Authors.

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

package watch

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/internal/engine"
)

var _ ControllerEngine = &MockEngine{}

type MockEngine struct {
	MockGetWatches  func(name string) ([]engine.WatchID, error)
	MockStopWatches func(ctx context.Context, name string, ws ...engine.WatchID) (int, error)
	MockGetCached   func() client.Client
	MockGetUncached func() client.Client
}

func (m *MockEngine) GetWatches(name string) ([]engine.WatchID, error) {
	return m.MockGetWatches(name)
}

func (m *MockEngine) StopWatches(ctx context.Context, name string, ws ...engine.WatchID) (int, error) {
	return m.MockStopWatches(ctx, name, ws...)
}

func (m *MockEngine) GetCached() client.Client {
	return m.MockGetCached()
}

func (m *MockEngine) GetUncached() client.Client {
	return m.MockGetUncached()
}

func TestGarbageCollectWatchesNow(t *testing.T) {
	errBoom := errors.New("boom")

	type params struct {
		name string
		of   resource.CompositeKind
		ce   ControllerEngine
		o    []GarbageCollectorOption
	}
	type args struct {
		ctx context.Context
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
		"GetWatchesError": {
			reason: "The method should return an error if it can't get watches.",
			params: params{
				ce: &MockEngine{
					MockGetWatches: func(_ string) ([]engine.WatchID, error) {
						return nil, errBoom
					},
					MockGetCached: func() client.Client {
						return &test.MockClient{
							MockList: test.NewMockListFn(nil),
						}
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"NoComposedResourceWatches": {
			reason: "The method should return early if there's no composed resource watches to potentially GC.",
			params: params{
				ce: &MockEngine{
					MockGetWatches: func(_ string) ([]engine.WatchID, error) {
						w := []engine.WatchID{{
							Type: engine.WatchTypeCompositeResource,
						}}
						return w, nil
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"ListXRsError": {
			reason: "The method should return an error if it can't list XRs.",
			params: params{
				ce: &MockEngine{
					MockGetWatches: func(_ string) ([]engine.WatchID, error) {
						w := []engine.WatchID{{
							Type: engine.WatchTypeComposedResource,
						}}
						return w, nil
					},
					MockGetCached: func() client.Client {
						return &test.MockClient{
							MockList: test.NewMockListFn(errBoom),
						}
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"StopWatchesError": {
			reason: "The method should return an error if it can't stop watches.",
			params: params{
				ce: &MockEngine{
					MockGetWatches: func(_ string) ([]engine.WatchID, error) {
						w := []engine.WatchID{
							{
								Type: engine.WatchTypeComposedResource,
								GVK:  schema.GroupVersionKind{},
							},
						}
						return w, nil
					},
					MockGetCached: func() client.Client {
						return &test.MockClient{
							MockList: test.NewMockListFn(nil),
						}
					},
					MockGetUncached: func() client.Client {
						return &test.MockClient{
							MockList: test.NewMockListFn(nil),
						}
					},
					MockStopWatches: func(_ context.Context, _ string, _ ...engine.WatchID) (int, error) {
						return 0, errBoom
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"NothingToStop": {
			reason: "The method shouldn't list from the uncached client if the cached client indicates there's no watches to stop.",
			params: params{
				ce: &MockEngine{
					MockGetCached: func() client.Client {
						return &test.MockClient{
							MockList: test.NewMockListFn(nil),
						}
					},
					// A list from uncached would panic,
					// since it's not mocked.
					MockGetWatches: func(_ string) ([]engine.WatchID, error) {
						w := []engine.WatchID{
							{
								Type: engine.WatchTypeCompositeResource,
								GVK:  schema.GroupVersionKind{},
							},
							{
								Type: engine.WatchTypeClaim,
								GVK:  schema.GroupVersionKind{},
							},
							{
								Type: engine.WatchTypeCompositionRevision,
								GVK:  schema.GroupVersionKind{},
							},
						}
						return w, nil
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"UnneededWatchesStopped": {
			reason: "StopWatches shouldn't be called if there's no watches to stop.",
			params: params{
				ce: &MockEngine{
					MockGetCached: func() client.Client {
						return &test.MockClient{
							MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
								xr := composite.New()
								xr.SetResourceReferences([]corev1.ObjectReference{
									{
										APIVersion: "example.org/v1",
										Kind:       "StillComposed",
										// Name doesn't matter.
									},
								})

								obj.(*unstructured.UnstructuredList).Items = []unstructured.Unstructured{xr.Unstructured}

								return nil
							}),
						}
					},
					// Uncached result matches cached
					// result.
					MockGetUncached: func() client.Client {
						return &test.MockClient{
							MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
								xr := composite.New()
								xr.SetResourceReferences([]corev1.ObjectReference{
									{
										APIVersion: "example.org/v1",
										Kind:       "StillComposed",
										// Name doesn't matter.
									},
								})

								obj.(*unstructured.UnstructuredList).Items = []unstructured.Unstructured{xr.Unstructured}

								return nil
							}),
						}
					},
					MockGetWatches: func(_ string) ([]engine.WatchID, error) {
						w := []engine.WatchID{
							// We want to keep this one.
							{
								Type: engine.WatchTypeComposedResource,
								GVK: schema.GroupVersionKind{
									Group:   "example.org",
									Version: "v1",
									Kind:    "StillComposed",
								},
							},
							// We want to GC this one.
							{
								Type: engine.WatchTypeComposedResource,
								GVK: schema.GroupVersionKind{
									Group:   "example.org",
									Version: "v1",
									Kind:    "GarbageCollectMe",
								},
							},
						}
						return w, nil
					},
					MockStopWatches: func(_ context.Context, _ string, ws ...engine.WatchID) (int, error) {
						want := []engine.WatchID{
							{
								Type: engine.WatchTypeComposedResource,
								GVK: schema.GroupVersionKind{
									Group:   "example.org",
									Version: "v1",
									Kind:    "GarbageCollectMe",
								},
							},
						}

						if diff := cmp.Diff(want, ws); diff != "" {
							t.Errorf("\nMockStopWatches(...) -want, +got:\n%s", diff)
						}

						return 0, nil
					},
				},
				o: []GarbageCollectorOption{
					WithLogger(logging.NewNopLogger()),
				},
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gc := NewGarbageCollector(tc.params.name, tc.params.of, tc.params.ce, tc.params.o...)
			err := gc.GarbageCollectWatchesNow(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ngc.GarbageCollectWatchesNow(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
