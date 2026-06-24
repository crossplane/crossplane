/*
Copyright 2026 The Crossplane Authors.

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

package definition

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/internal/engine"
)

// watchStarterFn adapts a function to the WatchStarter interface.
type watchStarterFn func(ctx context.Context, name string, ws ...engine.Watch) error

func (fn watchStarterFn) StartWatches(ctx context.Context, name string, ws ...engine.Watch) error {
	return fn(ctx, name, ws...)
}

func watchForGVK(gvk schema.GroupVersionKind) engine.Watch {
	u := &kunstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	return engine.WatchFor(u, engine.WatchTypeRequiredResource, nil)
}

func TestRequiredResourceWatchStarterWatchRequiredResources(t *testing.T) {
	type args struct {
		xr       schema.GroupVersionKind
		required []schema.GroupVersionKind
	}
	type want struct {
		watches []engine.Watch
		err     error
	}

	thing := schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "Thing"}
	other := schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "Other"}

	cases := map[string]struct {
		reason string
		err    error // returned by the StartWatches stub
		args   args
		want   want
	}{
		"NoRequiredResources": {
			reason: "We should not start any watches when there are no required resources.",
			args:   args{xr: thing, required: nil},
			want:   want{watches: nil},
		},
		"StartsWatchPerRequiredKind": {
			reason: "We should start a required resource watch for each required kind.",
			args: args{
				xr:       thing,
				required: []schema.GroupVersionKind{thing, other},
			},
			want: want{
				watches: []engine.Watch{watchForGVK(thing), watchForGVK(other)},
			},
		},
		"StartWatchesError": {
			reason: "We should wrap and return an error from StartWatches.",
			err:    errors.New("boom"),
			args: args{
				xr:       thing,
				required: []schema.GroupVersionKind{thing},
			},
			want: want{
				watches: []engine.Watch{watchForGVK(thing)},
				err:     cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var got []engine.Watch
			startFn := watchStarterFn(func(_ context.Context, _ string, ws ...engine.Watch) error {
				got = ws
				return tc.err
			})

			s := NewRequiredResourceWatchStarter("ctrl", startFn, nil, logging.NewNopLogger())
			err := s.WatchRequiredResources(context.Background(), tc.args.xr, tc.args.required)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nWatchRequiredResources(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.watches, got, cmp.AllowUnexported(engine.Watch{}), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nStarted watches: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
