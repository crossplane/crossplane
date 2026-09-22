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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite/dependency"
	"github.com/crossplane/crossplane/v2/internal/engine"
)

var _ ControllerEngine = &MockEngine{}

type MockEngine struct {
	MockGetWatches  func(name string) ([]engine.WatchID, error)
	MockStopWatches func(ctx context.Context, name string, ws ...engine.WatchID) (int, error)
}

func (m *MockEngine) GetWatches(name string) ([]engine.WatchID, error) {
	return m.MockGetWatches(name)
}

func (m *MockEngine) StopWatches(ctx context.Context, name string, ws ...engine.WatchID) (int, error) {
	return m.MockStopWatches(ctx, name, ws...)
}

func TestGarbageCollectWatchesNow(t *testing.T) {
	errBoom := errors.New("boom")

	dependedOn := schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "DependedOn"}
	collectMe := schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "GarbageCollectMe"}

	// tracker returns a Tracker that reports the supplied GVKs as depended on.
	tracker := func(gvks ...schema.GroupVersionKind) dependency.Tracker {
		tr := dependency.NewInMemory()
		refs := make([]dependency.Reference, len(gvks))
		for i, g := range gvks {
			refs[i] = dependency.Reference{GVK: g}
		}
		tr.Track(client.ObjectKey{Name: "xr"}, refs, nil)
		return tr
	}

	type params struct {
		name    string
		ce      ControllerEngine
		tracker dependency.Tracker
		o       []GarbageCollectorOption
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
				},
				tracker: tracker(),
			},
			want: want{err: cmpopts.AnyError},
		},
		"NoDependencyWatches": {
			reason: "The method should do nothing if there are no dependency watches to potentially garbage collect.",
			params: params{
				ce: &MockEngine{
					MockGetWatches: func(_ string) ([]engine.WatchID, error) {
						return []engine.WatchID{
							{Type: engine.WatchTypeCompositeResource},
							{Type: engine.WatchTypeCompositionRevision},
						}, nil
					},
					// StopWatches isn't mocked; it would panic if called.
				},
				tracker: tracker(),
			},
			want: want{err: nil},
		},
		"KeepDependedOnWatch": {
			reason: "The method shouldn't stop a watch for a kind an XR still depends on.",
			params: params{
				ce: &MockEngine{
					MockGetWatches: func(_ string) ([]engine.WatchID, error) {
						return []engine.WatchID{
							{Type: engine.WatchTypeDependency, GVK: dependedOn},
						}, nil
					},
					// StopWatches isn't mocked; it would panic if called.
				},
				tracker: tracker(dependedOn),
			},
			want: want{err: nil},
		},
		"StopWatchesError": {
			reason: "The method should return an error if it can't stop watches.",
			params: params{
				ce: &MockEngine{
					MockGetWatches: func(_ string) ([]engine.WatchID, error) {
						return []engine.WatchID{
							{Type: engine.WatchTypeDependency, GVK: collectMe},
						}, nil
					},
					MockStopWatches: func(_ context.Context, _ string, _ ...engine.WatchID) (int, error) {
						return 0, errBoom
					},
				},
				tracker: tracker(),
			},
			want: want{err: cmpopts.AnyError},
		},
		"UndependedWatchesStopped": {
			reason: "The method should stop dependency watches for kinds no XR depends on, and keep the rest.",
			params: params{
				ce: &MockEngine{
					MockGetWatches: func(_ string) ([]engine.WatchID, error) {
						return []engine.WatchID{
							{Type: engine.WatchTypeDependency, GVK: dependedOn},
							{Type: engine.WatchTypeDependency, GVK: collectMe},
						}, nil
					},
					MockStopWatches: func(_ context.Context, _ string, ws ...engine.WatchID) (int, error) {
						want := []engine.WatchID{{Type: engine.WatchTypeDependency, GVK: collectMe}}
						if diff := cmp.Diff(want, ws); diff != "" {
							t.Errorf("\nMockStopWatches(...): -want, +got:\n%s", diff)
						}

						return 1, nil
					},
				},
				tracker: tracker(dependedOn),
				o:       []GarbageCollectorOption{WithLogger(logging.NewNopLogger())},
			},
			want: want{err: nil},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gc := NewGarbageCollector(tc.params.name, tc.params.ce, tc.params.tracker, tc.params.o...)

			err := gc.GarbageCollectWatchesNow(tc.args.ctx)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ngc.GarbageCollectWatchesNow(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
