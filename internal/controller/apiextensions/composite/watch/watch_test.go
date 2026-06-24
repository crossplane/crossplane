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

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane/crossplane/v2/internal/engine"
)

var _ ControllerEngine = &MockEngine{}

var _ RequiredResourceProvider = &MockRequiredResourceProvider{}

// MockRequiredResourceProvider is a mock RequiredResourceProvider.
type MockRequiredResourceProvider struct {
	MockRequiredGVKs  func(xrUID string) []schema.GroupVersionKind
	MockRetainForKind func(gvk schema.GroupVersionKind, live map[string]bool)
}

func (m *MockRequiredResourceProvider) RequiredGVKs(xrUID string) []schema.GroupVersionKind {
	return m.MockRequiredGVKs(xrUID)
}

func (m *MockRequiredResourceProvider) RetainForKind(gvk schema.GroupVersionKind, live map[string]bool) {
	if m.MockRetainForKind != nil {
		m.MockRetainForKind(gvk, live)
	}
}

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
		of   schema.GroupVersionKind
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
		"LegacyComposedResourceWatchesUseLegacySchema": {
			reason: "For a legacy XR controller we should read composed resource refs using the legacy schema, so we keep watches the XR still composes.",
			params: params{
				of: schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "LegacyXR"},
				ce: &MockEngine{
					MockGetCached: func() client.Client {
						return &test.MockClient{
							MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
								// A legacy XR composing StillComposed. Legacy XRs
								// store refs at spec.resourceRefs, which only the
								// legacy schema reads.
								xr := composite.New(composite.WithSchema(composite.SchemaLegacy))
								xr.SetResourceReferences([]corev1.ObjectReference{{
									APIVersion: "example.org/v1",
									Kind:       "StillComposed",
								}})
								obj.(*unstructured.UnstructuredList).Items = []unstructured.Unstructured{xr.Unstructured}
								return nil
							}),
						}
					},
					MockGetWatches: func(_ string) ([]engine.WatchID, error) {
						return []engine.WatchID{{
							Type: engine.WatchTypeComposedResource,
							GVK:  schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "StillComposed"},
						}}, nil
					},
					// StopWatches isn't mocked: the test fails if it's called,
					// because StillComposed is still composed and must be kept.
				},
				o: []GarbageCollectorOption{
					WithLogger(logging.NewNopLogger()),
					WithCompositeSchema(composite.SchemaLegacy),
				},
			},
			want: want{
				err: nil,
			},
		},
		"UnneededRequiredResourceWatchesStopped": {
			reason: "We should stop a required resource watch for a kind no XR's function pipeline requires, and keep one that's still required.",
			params: params{
				ce: &MockEngine{
					MockGetCached: func() client.Client {
						return &test.MockClient{
							MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
								xr := composite.New()
								xr.SetUID("xr-uid")
								obj.(*unstructured.UnstructuredList).Items = []unstructured.Unstructured{xr.Unstructured}
								return nil
							}),
						}
					},
					MockGetUncached: func() client.Client {
						return &test.MockClient{
							MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
								xr := composite.New()
								xr.SetUID("xr-uid")
								obj.(*unstructured.UnstructuredList).Items = []unstructured.Unstructured{xr.Unstructured}
								return nil
							}),
						}
					},
					MockGetWatches: func(_ string) ([]engine.WatchID, error) {
						return []engine.WatchID{
							// Still required - keep it.
							{
								Type: engine.WatchTypeRequiredResource,
								GVK:  schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "StillRequired"},
							},
							// No longer required - GC it.
							{
								Type: engine.WatchTypeRequiredResource,
								GVK:  schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "GarbageCollectMe"},
							},
						}, nil
					},
					MockStopWatches: func(_ context.Context, _ string, ws ...engine.WatchID) (int, error) {
						want := []engine.WatchID{
							{
								Type: engine.WatchTypeRequiredResource,
								GVK:  schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "GarbageCollectMe"},
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
					WithRequiredResourceProvider(&MockRequiredResourceProvider{
						MockRequiredGVKs: func(xrUID string) []schema.GroupVersionKind {
							if xrUID != "xr-uid" {
								return nil
							}
							return []schema.GroupVersionKind{{Group: "example.org", Version: "v1", Kind: "StillRequired"}}
						},
						MockRetainForKind: func(_ schema.GroupVersionKind, live map[string]bool) {
							// We evict using the authoritative uncached live set,
							// which contains our one live XR.
							if diff := cmp.Diff(map[string]bool{"xr-uid": true}, live); diff != "" {
								t.Errorf("\nRetainForKind() live: -want, +got:\n%s", diff)
							}
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
			gc := NewGarbageCollector(tc.params.name, tc.params.of, tc.params.ce, tc.params.o...)

			err := gc.GarbageCollectWatchesNow(tc.args.ctx)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ngc.GarbageCollectWatchesNow(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
