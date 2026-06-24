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

package xfn

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

func TestRequiredGVKs(t *testing.T) {
	type args struct {
		r *fnv1.Requirements
	}
	type want struct {
		gvks []schema.GroupVersionKind
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Nil": {
			reason: "Nil requirements select no GVKs.",
			args:   args{r: nil},
			want:   want{gvks: []schema.GroupVersionKind{}},
		},
		"Resources": {
			reason: "We return the GVK of each required resource selector.",
			args: args{r: &fnv1.Requirements{
				Resources: map[string]*fnv1.ResourceSelector{
					"a": {ApiVersion: "example.org/v1", Kind: "Thing"},
				},
			}},
			want: want{gvks: []schema.GroupVersionKind{
				{Group: "example.org", Version: "v1", Kind: "Thing"},
			}},
		},
		"Deduplicates": {
			reason: "Two selectors for the same GVK collapse to one watch.",
			args: args{r: &fnv1.Requirements{
				Resources: map[string]*fnv1.ResourceSelector{
					"by-name":  {ApiVersion: "example.org/v1", Kind: "Thing", Match: &fnv1.ResourceSelector_MatchName{MatchName: "x"}},
					"by-label": {ApiVersion: "example.org/v1", Kind: "Thing"},
				},
			}},
			want: want{gvks: []schema.GroupVersionKind{
				{Group: "example.org", Version: "v1", Kind: "Thing"},
			}},
		},
		"IgnoresSchemas": {
			reason: "Schema requirements don't correspond to watchable resources.",
			args: args{r: &fnv1.Requirements{
				Schemas: map[string]*fnv1.SchemaSelector{
					"s": {ApiVersion: "example.org/v1", Kind: "Thing"},
				},
			}},
			want: want{gvks: []schema.GroupVersionKind{}},
		},
		"IgnoresEmptySelector": {
			reason: "A selector with no apiVersion or kind yields no GVK.",
			args: args{r: &fnv1.Requirements{
				Resources: map[string]*fnv1.ResourceSelector{
					"empty": {},
				},
			}},
			want: want{gvks: []schema.GroupVersionKind{}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := requiredGVKs(tc.args.r)
			if diff := cmp.Diff(tc.want.gvks, got, cmpopts.SortSlices(func(a, b schema.GroupVersionKind) bool {
				return a.String() < b.String()
			}), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nrequiredGVKs(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRequiredResourceWatcherRegistry(t *testing.T) {
	errBoom := errors.New("boom")

	xrGVK := schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "XR"}
	required := []schema.GroupVersionKind{{Group: "example.org", Version: "v1", Kind: "Thing"}}

	t.Run("DispatchesToRegisteredWatcher", func(t *testing.T) {
		var gotXR schema.GroupVersionKind
		var gotRequired []schema.GroupVersionKind

		reg := NewRequiredResourceWatcherRegistry()
		reg.Register(xrGVK, RequiredResourceWatcherFn(func(_ context.Context, xr schema.GroupVersionKind, req []schema.GroupVersionKind) error {
			gotXR = xr
			gotRequired = req
			return nil
		}))

		if err := reg.WatchRequiredResources(context.Background(), xrGVK, required); err != nil {
			t.Fatalf("WatchRequiredResources() error: %v", err)
		}
		if diff := cmp.Diff(xrGVK, gotXR); diff != "" {
			t.Errorf("dispatched XR GVK: -want, +got:\n%s", diff)
		}
		if diff := cmp.Diff(required, gotRequired); diff != "" {
			t.Errorf("dispatched required GVKs: -want, +got:\n%s", diff)
		}
	})

	t.Run("IgnoresUnregisteredKind", func(t *testing.T) {
		// An XR kind with no registered watcher is silently ignored - the common
		// case, since required resource watches are opt-in.
		reg := NewRequiredResourceWatcherRegistry()
		if err := reg.WatchRequiredResources(context.Background(), xrGVK, required); err != nil {
			t.Errorf("WatchRequiredResources() for unregistered kind: want nil, got %v", err)
		}
	})

	t.Run("PropagatesWatcherError", func(t *testing.T) {
		reg := NewRequiredResourceWatcherRegistry()
		reg.Register(xrGVK, RequiredResourceWatcherFn(func(_ context.Context, _ schema.GroupVersionKind, _ []schema.GroupVersionKind) error {
			return errBoom
		}))
		err := reg.WatchRequiredResources(context.Background(), xrGVK, required)
		if diff := cmp.Diff(errBoom, err, cmpopts.EquateErrors()); diff != "" {
			t.Errorf("WatchRequiredResources(): -want error, +got error:\n%s", diff)
		}
	})

	t.Run("DeregisterStopsDispatch", func(t *testing.T) {
		called := false
		reg := NewRequiredResourceWatcherRegistry()
		reg.Register(xrGVK, RequiredResourceWatcherFn(func(_ context.Context, _ schema.GroupVersionKind, _ []schema.GroupVersionKind) error {
			called = true
			return nil
		}))
		reg.Deregister(xrGVK)
		if err := reg.WatchRequiredResources(context.Background(), xrGVK, required); err != nil {
			t.Fatalf("WatchRequiredResources() error: %v", err)
		}
		if called {
			t.Error("watcher was called after Deregister")
		}
	})
}
