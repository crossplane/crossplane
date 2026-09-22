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

package dependency

import (
	"fmt"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	configMap = schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	pod       = schema.GroupVersionKind{Version: "v1", Kind: "Pod"}
)

func object(gvk schema.GroupVersionKind, namespace, name string, lbls map[string]string) client.Object {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetNamespace(namespace)
	u.SetName(name)
	u.SetLabels(lbls)

	return u
}

func key(namespace, name string) client.ObjectKey {
	return client.ObjectKey{Namespace: namespace, Name: name}
}

// reqs wraps references as requirements, for tests that don't care about the
// step or requirement name.
func reqs(refs ...Reference) []Requirement {
	out := make([]Requirement, len(refs))
	for i, r := range refs {
		out[i] = Requirement{Reference: r}
	}
	return out
}

// A track is a single Track call, used to set up a Tracker's state.
type track struct {
	xr       client.ObjectKey
	composed []Reference
	required []Reference
}

var sortKeys = cmpopts.SortSlices(func(a, b client.ObjectKey) bool { return a.String() < b.String() })

func TestDependants(t *testing.T) {
	type args struct {
		tracks []track
		obj    client.Object
	}
	type want struct {
		deps []client.ObjectKey
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MatchByName": {
			reason: "An XR that requires a resource by name depends on that resource.",
			args: args{
				tracks: []track{{
					xr:       key("ns", "xr"),
					required: []Reference{{GVK: configMap, Namespace: "ns", Name: "cm"}},
				}},
				obj: object(configMap, "ns", "cm", nil),
			},
			want: want{deps: []client.ObjectKey{key("ns", "xr")}},
		},
		"NoMatchByName": {
			reason: "An XR that requires a different name doesn't depend on the object.",
			args: args{
				tracks: []track{{
					xr:       key("ns", "xr"),
					required: []Reference{{GVK: configMap, Namespace: "ns", Name: "other"}},
				}},
				obj: object(configMap, "ns", "cm", nil),
			},
			want: want{},
		},
		"NoMatchByGVK": {
			reason: "An XR that requires a different kind doesn't depend on the object.",
			args: args{
				tracks: []track{{
					xr:       key("ns", "xr"),
					required: []Reference{{GVK: pod, Namespace: "ns", Name: "cm"}},
				}},
				obj: object(configMap, "ns", "cm", nil),
			},
			want: want{},
		},
		"ComposedMatchByName": {
			reason: "An XR that composes a resource depends on it.",
			args: args{
				tracks: []track{{
					xr:       key("ns", "xr"),
					composed: []Reference{{GVK: configMap, Namespace: "ns", Name: "cm"}},
				}},
				obj: object(configMap, "ns", "cm", nil),
			},
			want: want{deps: []client.ObjectKey{key("ns", "xr")}},
		},
		"MatchByLabel": {
			reason: "An XR that requires resources by label depends on a matching resource.",
			args: args{
				tracks: []track{{
					xr:       key("ns", "xr"),
					required: []Reference{{GVK: configMap, Namespace: "ns", Labels: map[string]string{"k": "v"}}},
				}},
				obj: object(configMap, "ns", "cm", map[string]string{"k": "v", "other": "label"}),
			},
			want: want{deps: []client.ObjectKey{key("ns", "xr")}},
		},
		"NoMatchByLabel": {
			reason: "An XR that requires resources by a label the object lacks doesn't depend on it.",
			args: args{
				tracks: []track{{
					xr:       key("ns", "xr"),
					required: []Reference{{GVK: configMap, Namespace: "ns", Labels: map[string]string{"k": "v"}}},
				}},
				obj: object(configMap, "ns", "cm", map[string]string{"k": "different"}),
			},
			want: want{},
		},
		"MatchAll": {
			reason: "An XR that requires resources with no name or labels depends on every resource of the kind.",
			args: args{
				tracks: []track{{
					xr:       key("ns", "xr"),
					required: []Reference{{GVK: configMap, Namespace: "ns"}},
				}},
				obj: object(configMap, "ns", "cm", nil),
			},
			want: want{deps: []client.ObjectKey{key("ns", "xr")}},
		},
		"LabelMatchesAnyNamespace": {
			reason: "A reference with no namespace matches resources in any namespace.",
			args: args{
				tracks: []track{{
					xr:       key("ns", "xr"),
					required: []Reference{{GVK: configMap, Labels: map[string]string{"k": "v"}}},
				}},
				obj: object(configMap, "other", "cm", map[string]string{"k": "v"}),
			},
			want: want{deps: []client.ObjectKey{key("ns", "xr")}},
		},
		"LabelScopedToNamespace": {
			reason: "A reference with a namespace doesn't match resources in another namespace.",
			args: args{
				tracks: []track{{
					xr:       key("ns", "xr"),
					required: []Reference{{GVK: configMap, Namespace: "ns", Labels: map[string]string{"k": "v"}}},
				}},
				obj: object(configMap, "other", "cm", map[string]string{"k": "v"}),
			},
			want: want{},
		},
		"MultipleDependants": {
			reason: "Every XR that depends on a resource is returned.",
			args: args{
				tracks: []track{
					{xr: key("ns", "a"), required: []Reference{{GVK: configMap, Namespace: "ns", Name: "cm"}}},
					{xr: key("ns", "b"), required: []Reference{{GVK: configMap, Namespace: "ns", Labels: map[string]string{"k": "v"}}}},
					{xr: key("ns", "c"), required: []Reference{{GVK: configMap, Namespace: "ns", Name: "other"}}},
				},
				obj: object(configMap, "ns", "cm", map[string]string{"k": "v"}),
			},
			want: want{deps: []client.ObjectKey{key("ns", "a"), key("ns", "b")}},
		},
		"Deduplicated": {
			reason: "An XR that depends on a resource by both name and label is only returned once.",
			args: args{
				tracks: []track{{
					xr: key("ns", "xr"),
					required: []Reference{
						{GVK: configMap, Namespace: "ns", Name: "cm"},
						{GVK: configMap, Namespace: "ns", Labels: map[string]string{"k": "v"}},
					},
				}},
				obj: object(configMap, "ns", "cm", map[string]string{"k": "v"}),
			},
			want: want{deps: []client.ObjectKey{key("ns", "xr")}},
		},
		"ReplacedByLaterTrack": {
			reason: "Tracking an XR again replaces its previous references.",
			args: args{
				tracks: []track{
					{xr: key("ns", "xr"), required: []Reference{{GVK: configMap, Namespace: "ns", Name: "cm"}}},
					{xr: key("ns", "xr"), required: []Reference{{GVK: configMap, Namespace: "ns", Name: "other"}}},
				},
				obj: object(configMap, "ns", "cm", nil),
			},
			want: want{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tr := NewInMemory()
			for _, tk := range tc.args.tracks {
				tr.Track(tk.xr, tk.composed, reqs(tk.required...))
			}

			got := tr.Dependants(tc.args.obj)

			if diff := cmp.Diff(tc.want.deps, got, sortKeys, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nDependants(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGVKs(t *testing.T) {
	cases := map[string]struct {
		reason string
		tracks []track
		want   []schema.GroupVersionKind
	}{
		"Empty": {
			reason: "A tracker with no references has no GVKs.",
			want:   nil,
		},
		"ComposedAndRequired": {
			reason: "GVKs returns the kinds of both composed and required references.",
			tracks: []track{{
				xr:       key("ns", "xr"),
				composed: []Reference{{GVK: pod, Namespace: "ns", Name: "p"}},
				required: []Reference{{GVK: configMap, Namespace: "ns", Name: "cm"}},
			}},
			want: []schema.GroupVersionKind{pod, configMap},
		},
		"Deduplicated": {
			reason: "A GVK referenced by multiple XRs is returned once.",
			tracks: []track{
				{xr: key("ns", "a"), required: []Reference{{GVK: configMap, Namespace: "ns", Name: "cm"}}},
				{xr: key("ns", "b"), required: []Reference{{GVK: configMap, Namespace: "ns", Name: "other"}}},
			},
			want: []schema.GroupVersionKind{configMap},
		},
		"DroppedWhenLastReferenceReplaced": {
			reason: "A GVK is dropped once no reference to it remains.",
			tracks: []track{
				{xr: key("ns", "xr"), required: []Reference{{GVK: configMap, Namespace: "ns", Name: "cm"}}},
				{xr: key("ns", "xr"), required: []Reference{{GVK: pod, Namespace: "ns", Name: "p"}}},
			},
			want: []schema.GroupVersionKind{pod},
		},
		"RetainedWhileAnotherXRReferences": {
			reason: "A GVK is retained while any XR still references it.",
			tracks: []track{
				{xr: key("ns", "a"), required: []Reference{{GVK: configMap, Namespace: "ns", Name: "cm"}}},
				{xr: key("ns", "b"), required: []Reference{{GVK: configMap, Namespace: "ns", Name: "other"}}},
				{xr: key("ns", "a"), required: []Reference{{GVK: pod, Namespace: "ns", Name: "p"}}},
			},
			want: []schema.GroupVersionKind{configMap, pod},
		},
	}

	sortGVKs := cmpopts.SortSlices(func(a, b schema.GroupVersionKind) bool { return a.String() < b.String() })

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tr := NewInMemory()
			for _, tk := range tc.tracks {
				tr.Track(tk.xr, tk.composed, reqs(tk.required...))
			}

			got := tr.GVKs()

			if diff := cmp.Diff(tc.want, got, sortGVKs, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nGVKs(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRequirements(t *testing.T) {
	type args struct {
		xr   client.ObjectKey
		step string
	}

	cases := map[string]struct {
		reason   string
		required []Requirement
		args     args
		want     []Requirement
	}{
		"ReturnsStepRequirements": {
			reason: "Requirements returns only the named step's requirements.",
			required: []Requirement{
				{Step: "a", Name: "cm", Reference: Reference{GVK: configMap, Namespace: "ns", Name: "cm"}},
				{Step: "b", Name: "cm", Reference: Reference{GVK: configMap, Namespace: "ns", Name: "other"}},
			},
			args: args{xr: key("ns", "xr"), step: "a"},
			want: []Requirement{{Step: "a", Name: "cm", Reference: Reference{GVK: configMap, Namespace: "ns", Name: "cm"}}},
		},
		"UnknownXR": {
			reason: "Requirements returns nil for an XR that was never tracked.",
			args:   args{xr: key("ns", "xr"), step: "a"},
			want:   nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tr := NewInMemory()
			if len(tc.required) > 0 {
				tr.Track(tc.args.xr, nil, tc.required)
			}

			got := tr.Requirements(tc.args.xr, tc.args.step)

			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nRequirements(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestForget(t *testing.T) {
	tr := NewInMemory()
	tr.Track(key("ns", "a"), nil, reqs(Reference{GVK: configMap, Namespace: "ns", Name: "cm"}))
	tr.Track(key("ns", "b"), nil, reqs(Reference{GVK: configMap, Namespace: "ns", Name: "cm"}))

	tr.Forget(key("ns", "a"))

	// b still depends on the ConfigMap; a no longer does.
	got := tr.Dependants(object(configMap, "ns", "cm", nil))
	want := []client.ObjectKey{key("ns", "b")}
	if diff := cmp.Diff(want, got, sortKeys); diff != "" {
		t.Errorf("\nForget(a) should leave b's dependency intact: -want, +got:\n%s", diff)
	}

	tr.Forget(key("ns", "b"))

	if diff := cmp.Diff([]schema.GroupVersionKind(nil), tr.GVKs(), cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("\nForgetting all XRs should drop all GVKs: -want, +got:\n%s", diff)
	}
}

func TestTrackers(t *testing.T) {
	t.Run("GetIsStablePerController", func(t *testing.T) {
		r := NewTrackers(DefaultNewTracker)

		a1 := r.Get("a")
		a2 := r.Get("a")
		b := r.Get("b")

		if a1 != a2 {
			t.Errorf("Get(\"a\") returned different Trackers on successive calls")
		}
		if a1 == b {
			t.Errorf("Get(\"a\") and Get(\"b\") returned the same Tracker")
		}
	})

	t.Run("DeleteResetsTracker", func(t *testing.T) {
		r := NewTrackers(DefaultNewTracker)

		a1 := r.Get("a")
		r.Delete("a")
		a2 := r.Get("a")

		if a1 == a2 {
			t.Errorf("Get(\"a\") after Delete(\"a\") returned the same Tracker")
		}
	})

	t.Run("UsesInjectedFactory", func(t *testing.T) {
		want := NopTracker{}
		r := NewTrackers(func() Tracker { return want })

		if got := r.Get("a"); got != Tracker(want) {
			t.Errorf("Get(\"a\") = %T, want the injected Tracker %T", got, want)
		}
	})
}

// TestInMemoryConcurrency exercises an InMemory Tracker from many goroutines at
// once. The Tracker is read on every watch event and written on every reconcile
// from separate goroutines, so it must be safe for concurrent use.
//
// Run it under `CGO_ENABLED=1 go test -race` for a rigorous check - the race
// detector will pinpoint any unsynchronized access. Without -race it's weaker
// but not worthless: because the Tracker's state is entirely maps, a missing or
// wrong lock tends to trip the runtime's built-in "concurrent map access" fatal,
// and the end-state assertions below catch corruption that survives to a
// consistent-looking state.
func TestInMemoryConcurrency(t *testing.T) {
	const (
		writers    = 50
		iterations = 100
		readers    = 10
	)

	// Each writer owns a disjoint XR, so the end state is deterministic: after
	// every writer has run, every XR is tracked, composing one ConfigMap (by
	// name) and requiring Pods (by label).
	composed := []Reference{{GVK: configMap, Namespace: "ns", Name: "cm"}}
	required := []Requirement{{
		Step:      "grumpy",
		Name:      "pods",
		Reference: Reference{GVK: pod, Namespace: "ns", Labels: map[string]string{"app": "x"}},
	}}

	tr := NewInMemory()

	var wg sync.WaitGroup

	// Writers churn each XR's references, ending on Track so the XR is tracked.
	for i := range writers {
		xr := key("ns", fmt.Sprintf("xr-%d", i))
		wg.Go(func() {
			for range iterations {
				tr.Track(xr, composed, required)
				tr.Requirements(xr, "grumpy")
				tr.Forget(xr)
				tr.Track(xr, composed, required)
			}
		})
	}

	// Readers hammer the read paths against the writers.
	cm := object(configMap, "ns", "cm", nil)
	p := object(pod, "ns", "p", map[string]string{"app": "x"})
	for range readers {
		wg.Go(func() {
			for range writers * iterations {
				tr.Dependants(cm)
				tr.Dependants(p)
				tr.GVKs()
			}
		})
	}

	wg.Wait()

	// Every writer ended on Track, so every XR depends on both kinds.
	if got := len(tr.Dependants(cm)); got != writers {
		t.Errorf("Dependants(ConfigMap): got %d dependants, want %d", got, writers)
	}
	if got := len(tr.Dependants(p)); got != writers {
		t.Errorf("Dependants(Pod): got %d dependants, want %d", got, writers)
	}
	if got := len(tr.GVKs()); got != 2 {
		t.Errorf("GVKs(): got %d, want 2 (ConfigMap and Pod)", got)
	}
}
