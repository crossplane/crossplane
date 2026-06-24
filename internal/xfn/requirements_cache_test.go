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
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

var errGate = errors.New("required resource not satisfied")

func sel(name string) *fnv1.ResourceSelector {
	return &fnv1.ResourceSelector{
		ApiVersion: "example.org/v1",
		Kind:       "Thing",
		Match:      &fnv1.ResourceSelector_MatchName{MatchName: name},
	}
}

func reqs(names ...string) *fnv1.Requirements {
	r := &fnv1.Requirements{Resources: map[string]*fnv1.ResourceSelector{}}
	for _, n := range names {
		r.Resources[n] = sel(n)
	}
	return r
}

func TestRequirementsCache(t *testing.T) {
	xrKind := schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "XR"}

	c := NewRequirementsCache()

	// An empty cache returns nothing.
	if _, ok := c.Get("xr-uid", "fn"); ok {
		t.Fatal("Get() on empty cache returned ok=true")
	}

	// Setting then getting round-trips a clone.
	want := reqs("a", "b")
	c.Set("xr-uid", xrKind, "fn", want)
	got, ok := c.Get("xr-uid", "fn")
	if !ok {
		t.Fatal("Get() after Set() returned ok=false")
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("Get() after Set(): -want, +got:\n%s", diff)
	}

	// The cache stores a clone - mutating what we stored doesn't change it.
	want.Resources["c"] = sel("c")
	got, _ = c.Get("xr-uid", "fn")
	if _, leaked := got.GetResources()["c"]; leaked {
		t.Error("Get() returned a value mutated through the caller's reference; cache didn't clone")
	}

	// Entries are scoped per (XR UID, function).
	if _, ok := c.Get("xr-uid", "other-fn"); ok {
		t.Error("Get() for a different function returned ok=true")
	}
	if _, ok := c.Get("other-uid", "fn"); ok {
		t.Error("Get() for a different XR returned ok=true")
	}

	// Setting empty requirements deletes the entry rather than storing it.
	c.Set("xr-uid", xrKind, "fn", &fnv1.Requirements{})
	if _, ok := c.Get("xr-uid", "fn"); ok {
		t.Error("Get() after Set() with empty requirements returned ok=true; entry not deleted")
	}

	// RetainForKind forgets XRs of a kind whose UID isn't live.
	c.Set("xr-uid", xrKind, "fn1", reqs("a"))
	c.Set("xr-uid", xrKind, "fn2", reqs("b"))
	c.RetainForKind(xrKind, map[string]bool{}) // No live UIDs.
	if _, ok := c.Get("xr-uid", "fn1"); ok {
		t.Error("Get() after RetainForKind() returned ok=true")
	}
	if _, ok := c.Get("xr-uid", "fn2"); ok {
		t.Error("Get() after RetainForKind() returned ok=true")
	}
}

func TestRequirementsCacheRetainForKind(t *testing.T) {
	xrKind := schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "XR"}
	otherKind := schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "Other"}

	c := NewRequirementsCache()
	c.Set("live", xrKind, "fn", reqs("a"))
	c.Set("dead", xrKind, "fn", reqs("b"))
	c.Set("other", otherKind, "fn", reqs("c"))

	// Retain only "live" among XRs of xrKind. "dead" is evicted; "other" is a
	// different kind and untouched even though it's not in the live set.
	c.RetainForKind(xrKind, map[string]bool{"live": true})

	if _, ok := c.Get("live", "fn"); !ok {
		t.Error("RetainForKind() evicted a live XR's requirements")
	}
	if _, ok := c.Get("dead", "fn"); ok {
		t.Error("RetainForKind() didn't evict a dead XR's requirements")
	}
	if _, ok := c.Get("other", "fn"); !ok {
		t.Error("RetainForKind() evicted an XR of a different kind")
	}
}

// xrReq builds a RunFunctionRequest whose observed composite resource has the
// supplied UID, so the FetchingFunctionRunner can key the RequirementsCache. The
// XR is an example.org/v1 XR.
func xrReq(uid string) *fnv1.RunFunctionRequest {
	return &fnv1.RunFunctionRequest{
		Observed: &fnv1.State{
			Composite: &fnv1.Resource{
				Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
					"apiVersion": structpb.NewStringValue("example.org/v1"),
					"kind":       structpb.NewStringValue("XR"),
					"metadata": structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
						"uid": structpb.NewStringValue(uid),
					}}),
				}},
			},
		},
	}
}

func TestFetchingFunctionRunnerPreSatisfy(t *testing.T) {
	// A function with a single, stable, dynamic requirement. It returns the same
	// requirement every call. Without pre-satisfaction this costs two calls: one
	// to discover the requirement, one to confirm it's satisfied.
	stable := reqs("needed")

	t.Run("CollapsesDoubleCallWhenRemembered", func(t *testing.T) {
		calls := 0
		wrapped := FunctionRunnerFn(func(_ context.Context, _ string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
			calls++
			// The function gates on its requirement: it only does real work once
			// the resource is present. We assert it's present on the very first
			// call, proving we pre-satisfied it.
			if _, ok := req.GetRequiredResources()["needed"]; !ok {
				return nil, errGate
			}
			return &fnv1.RunFunctionResponse{Requirements: stable}, nil
		})

		fetched := RequiredResourcesFetcherFn(func(_ context.Context, _ *fnv1.ResourceSelector) (*fnv1.Resources, error) {
			return &fnv1.Resources{}, nil
		})

		cache := NewRequirementsCache()
		// Pre-seed the cache as though a previous reconcile already learned the
		// requirement.
		cache.Set("xr-uid", schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "XR"}, "fn", stable)

		r := NewFetchingFunctionRunner(wrapped, fetched, NopRequiredSchemasFetcher{}, WithRequirementsRecorder(cache))

		_, err := r.RunFunction(context.Background(), "fn", xrReq("xr-uid"))
		if err != nil {
			t.Fatalf("RunFunction() error: %v", err)
		}
		if calls != 1 {
			t.Errorf("expected 1 function call when requirements were remembered, got %d", calls)
		}
	})

	t.Run("LearnsAndRemembersOnFirstReconcile", func(t *testing.T) {
		calls := 0
		wrapped := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
			calls++
			return &fnv1.RunFunctionResponse{Requirements: stable}, nil
		})
		fetched := RequiredResourcesFetcherFn(func(_ context.Context, _ *fnv1.ResourceSelector) (*fnv1.Resources, error) {
			return &fnv1.Resources{}, nil
		})

		cache := NewRequirementsCache()
		r := NewFetchingFunctionRunner(wrapped, fetched, NopRequiredSchemasFetcher{}, WithRequirementsRecorder(cache))

		// First reconcile: cache is cold, so it costs the usual two calls.
		if _, err := r.RunFunction(context.Background(), "fn", xrReq("xr-uid")); err != nil {
			t.Fatalf("RunFunction() error: %v", err)
		}
		if calls != 2 {
			t.Errorf("expected 2 function calls on a cold cache, got %d", calls)
		}

		// It should have remembered the requirement.
		got, ok := cache.Get("xr-uid", "fn")
		if !ok {
			t.Fatal("requirement not remembered after first reconcile")
		}
		if diff := cmp.Diff(stable, got, protocmp.Transform()); diff != "" {
			t.Errorf("remembered requirement: -want, +got:\n%s", diff)
		}

		// Second reconcile (fresh request, same XR): now it costs one call.
		calls = 0
		if _, err := r.RunFunction(context.Background(), "fn", xrReq("xr-uid")); err != nil {
			t.Fatalf("RunFunction() error: %v", err)
		}
		if calls != 1 {
			t.Errorf("expected 1 function call on a warm cache, got %d", calls)
		}
	})

	t.Run("SelfCorrectsWhenRequirementsChange", func(t *testing.T) {
		// We remembered an old requirement, but the function now wants a
		// different one. The runner must fall back to its iterative loop.
		old := reqs("old")
		current := reqs("new")

		calls := 0
		wrapped := FunctionRunnerFn(func(_ context.Context, _ string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
			calls++
			// Gate on the current requirement being satisfied.
			if _, ok := req.GetRequiredResources()["new"]; ok {
				return &fnv1.RunFunctionResponse{Requirements: current}, nil
			}
			return &fnv1.RunFunctionResponse{Requirements: current}, nil
		})
		fetched := RequiredResourcesFetcherFn(func(_ context.Context, _ *fnv1.ResourceSelector) (*fnv1.Resources, error) {
			return &fnv1.Resources{}, nil
		})

		cache := NewRequirementsCache()
		cache.Set("xr-uid", schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "XR"}, "fn", old)

		r := NewFetchingFunctionRunner(wrapped, fetched, NopRequiredSchemasFetcher{}, WithRequirementsRecorder(cache))
		if _, err := r.RunFunction(context.Background(), "fn", xrReq("xr-uid")); err != nil {
			t.Fatalf("RunFunction() error: %v", err)
		}

		// Two calls: the first returns the new (unexpected) requirement, the
		// second confirms it after we fetch it.
		if calls != 2 {
			t.Errorf("expected 2 calls when remembered requirements were stale, got %d", calls)
		}

		// The cache should now hold the corrected requirement.
		got, _ := cache.Get("xr-uid", "fn")
		if diff := cmp.Diff(current, got, protocmp.Transform()); diff != "" {
			t.Errorf("corrected requirement: -want, +got:\n%s", diff)
		}
	})
}

func TestFetchingFunctionRunnerWatch(t *testing.T) {
	t.Run("WatchesRequiredResourcesAtStabilization", func(t *testing.T) {
		// A function with a stable requirement on a single kind.
		stable := reqs("needed")

		wrapped := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
			return &fnv1.RunFunctionResponse{Requirements: stable}, nil
		})
		fetched := RequiredResourcesFetcherFn(func(_ context.Context, _ *fnv1.ResourceSelector) (*fnv1.Resources, error) {
			return &fnv1.Resources{}, nil
		})

		var gotXR schema.GroupVersionKind
		var gotRequired []schema.GroupVersionKind
		watcher := RequiredResourceWatcherFn(func(_ context.Context, xr schema.GroupVersionKind, required []schema.GroupVersionKind) error {
			gotXR = xr
			gotRequired = required
			return nil
		})

		r := NewFetchingFunctionRunner(wrapped, fetched, NopRequiredSchemasFetcher{}, WithRequiredResourceWatcher(watcher))
		if _, err := r.RunFunction(context.Background(), "fn", xrReq("xr-uid")); err != nil {
			t.Fatalf("RunFunction() error: %v", err)
		}

		wantXR := schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "XR"}
		if diff := cmp.Diff(wantXR, gotXR); diff != "" {
			t.Errorf("watched XR GVK: -want, +got:\n%s", diff)
		}
		wantRequired := []schema.GroupVersionKind{{Group: "example.org", Version: "v1", Kind: "Thing"}}
		if diff := cmp.Diff(wantRequired, gotRequired); diff != "" {
			t.Errorf("watched required GVKs: -want, +got:\n%s", diff)
		}
	})

	t.Run("PropagatesWatcherError", func(t *testing.T) {
		wrapped := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
			return &fnv1.RunFunctionResponse{Requirements: reqs("needed")}, nil
		})
		fetched := RequiredResourcesFetcherFn(func(_ context.Context, _ *fnv1.ResourceSelector) (*fnv1.Resources, error) {
			return &fnv1.Resources{}, nil
		})
		watcher := RequiredResourceWatcherFn(func(_ context.Context, _ schema.GroupVersionKind, _ []schema.GroupVersionKind) error {
			return errGate
		})

		r := NewFetchingFunctionRunner(wrapped, fetched, NopRequiredSchemasFetcher{}, WithRequiredResourceWatcher(watcher))
		_, err := r.RunFunction(context.Background(), "fn", xrReq("xr-uid"))
		if !errors.Is(err, errGate) {
			t.Errorf("RunFunction(): want wrapped errGate, got %v", err)
		}
	})

	t.Run("DoesNotRememberOrWatchWithoutXRUID", func(t *testing.T) {
		// An XR with no UID can't be keyed in the cache or identified for
		// watching. We must not remember its requirements (which would collide
		// with other unidentifiable XRs under the empty key) or start watches.
		wrapped := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
			return &fnv1.RunFunctionResponse{Requirements: reqs("needed")}, nil
		})
		fetched := RequiredResourcesFetcherFn(func(_ context.Context, _ *fnv1.ResourceSelector) (*fnv1.Resources, error) {
			return &fnv1.Resources{}, nil
		})

		watched := false
		watcher := RequiredResourceWatcherFn(func(_ context.Context, _ schema.GroupVersionKind, _ []schema.GroupVersionKind) error {
			watched = true
			return nil
		})

		cache := NewRequirementsCache()
		r := NewFetchingFunctionRunner(wrapped, fetched, NopRequiredSchemasFetcher{},
			WithRequirementsRecorder(cache),
			WithRequiredResourceWatcher(watcher))

		// A request with no observed composite at all - no UID, no GVK.
		if _, err := r.RunFunction(context.Background(), "fn", &fnv1.RunFunctionRequest{}); err != nil {
			t.Fatalf("RunFunction() error: %v", err)
		}

		if watched {
			t.Error("started watches for an XR with no UID")
		}
		if _, ok := cache.Get("", "fn"); ok {
			t.Error("remembered requirements under the empty XR UID key")
		}
	})
}

func TestFetchingFunctionRunnerRetag(t *testing.T) {
	// The runner sits above the response cache, which keys on the request's tag.
	// Each iteration's request must carry a tag derived from that iteration's
	// content - including the required resources fetched so far - so the cache
	// keys on the resolved request, and a change to a required resource produces
	// a different tag (a cache miss).
	t.Run("TagReflectsFetchedResources", func(t *testing.T) {
		// Record the tag the wrapped runner sees on each call.
		var tags []string
		wrapped := FunctionRunnerFn(func(_ context.Context, _ string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
			tags = append(tags, req.GetMeta().GetTag())
			return &fnv1.RunFunctionResponse{Requirements: reqs("needed")}, nil
		})

		// The fetched resource's content varies, so the iteration-1 request
		// (which carries it) differs from iteration 0.
		resourceContent := "first"
		fetched := RequiredResourcesFetcherFn(func(_ context.Context, _ *fnv1.ResourceSelector) (*fnv1.Resources, error) {
			return &fnv1.Resources{Items: []*fnv1.Resource{{Resource: MustStruct(map[string]any{"data": resourceContent})}}}, nil
		})

		r := NewFetchingFunctionRunner(wrapped, fetched, NopRequiredSchemasFetcher{}, WithPerIterationTagging())
		if _, err := r.RunFunction(context.Background(), "fn", xrReq("xr-uid")); err != nil {
			t.Fatalf("RunFunction() error: %v", err)
		}

		// Two calls: iteration 0 (no required resources) and iteration 1 (with
		// them). Their tags must differ, and neither may be empty.
		if len(tags) != 2 {
			t.Fatalf("expected 2 calls, got %d", len(tags))
		}
		if tags[0] == "" || tags[1] == "" {
			t.Errorf("expected non-empty tags, got %q and %q", tags[0], tags[1])
		}
		if tags[0] == tags[1] {
			t.Error("iteration 0 and iteration 1 had the same tag; the tag doesn't reflect fetched required resources")
		}

		// Re-running with different required resource content must produce a
		// different iteration-1 tag - this is what invalidates the cache.
		firstIter1 := tags[1]
		tags = nil
		resourceContent = "second"
		if _, err := r.RunFunction(context.Background(), "fn", xrReq("xr-uid")); err != nil {
			t.Fatalf("RunFunction() error: %v", err)
		}
		if tags[1] == firstIter1 {
			t.Error("a changed required resource produced the same tag; the cache wouldn't be invalidated")
		}
	})

	t.Run("DoesNotTagWithoutOption", func(t *testing.T) {
		// Without per-iteration tagging the runner leaves the request's tag
		// alone, preserving behavior for when the cache sits above the runner.
		var tags []string
		wrapped := FunctionRunnerFn(func(_ context.Context, _ string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
			tags = append(tags, req.GetMeta().GetTag())
			return &fnv1.RunFunctionResponse{}, nil
		})
		fetched := RequiredResourcesFetcherFn(func(_ context.Context, _ *fnv1.ResourceSelector) (*fnv1.Resources, error) {
			return &fnv1.Resources{}, nil
		})

		r := NewFetchingFunctionRunner(wrapped, fetched, NopRequiredSchemasFetcher{})
		// The incoming request carries a pre-set tag; the runner must not change it.
		req := xrReq("xr-uid")
		req.Meta = &fnv1.RequestMeta{Tag: "preset"}
		if _, err := r.RunFunction(context.Background(), "fn", req); err != nil {
			t.Fatalf("RunFunction() error: %v", err)
		}
		for _, tag := range tags {
			if tag != "preset" {
				t.Errorf("runner changed the request tag without WithPerIterationTagging: got %q", tag)
			}
		}
	})
}
