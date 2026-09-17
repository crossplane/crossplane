/*
Copyright 2025 The Crossplane Authors.

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

package composite

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/reference"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// ref returns a composed resource reference of the shape core writes.
func ref(resourceName string, dependsOn ...string) reference.Composed {
	return reference.Composed{
		APIVersion:   "example.org/v1",
		Kind:         "Thing",
		Name:         "cool-xr-" + resourceName,
		ResourceName: resourceName,
		DependsOn:    dependsOn,
	}
}

// teardownXR returns an XR whose references record the supplied graph.
func teardownXR(refs ...reference.Composed) *composite.Unstructured {
	xr := composite.New()
	xr.SetName("cool-xr")
	xr.SetComposedResourceReferences(refs)

	return xr
}

// observing returns an observer that reports the named resources as existing.
func observing(names ...string) ComposedResourceObserver {
	return ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
		out := ComposedResourceStates{}
		for _, n := range names {
			out[ResourceName(n)] = state("Thing", n)
		}

		return out, nil
	})
}

// recordingGC records what it was asked to delete.
func recordingGC(into *[]string) ComposedResourceGarbageCollector {
	return ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, observed, _ ComposedResourceStates) error {
		for n := range observed {
			*into = append(*into, string(n))
		}

		return nil
	})
}

func TestTeardown(t *testing.T) {
	// vpc <- subnet <- instance.
	graph := []reference.Composed{
		ref("vpc"),
		ref("subnet", "vpc"),
		ref("instance", "subnet"),
	}

	type want struct {
		done    bool
		deleted []string
		err     bool
		// message, if set, must appear in the Deleting condition.
		message string
	}

	cases := map[string]struct {
		reason   string
		observer ComposedResourceObserver
		gc       bool
		xr       *composite.Unstructured
		want     want
	}{
		"NotConfigured": {
			reason: "With no observer and collector, teardown is a no-op and the caller drops its finalizer immediately.",
			xr:     teardownXR(graph...),
			want:   want{done: true},
		},
		"NoGraph": {
			reason:   "An XR whose references record no ordering tears down the way it always has.",
			observer: observing("vpc", "subnet"),
			gc:       true,
			xr:       teardownXR(ref("vpc"), ref("subnet")),
			want:     want{done: true},
		},
		"NothingLeft": {
			reason:   "Once every composed resource is gone the XR is done deleting.",
			observer: observing(),
			gc:       true,
			xr:       teardownXR(graph...),
			want:     want{done: true},
		},
		"DeletesLeavesFirst": {
			reason:   "The first wave deletes only the leaf, not the resources it depends on.",
			observer: observing("vpc", "subnet", "instance"),
			gc:       true,
			xr:       teardownXR(graph...),
			want:     want{done: false, deleted: []string{"instance"}, message: "Deleting 1 composed resource"},
		},
		"DeletesNextWave": {
			reason:   "Once the leaf is gone the resource it depended on becomes deletable.",
			observer: observing("vpc", "subnet"),
			gc:       true,
			xr:       teardownXR(graph...),
			want:     want{done: false, deleted: []string{"subnet"}},
		},
		"BlocksWhenNothingIsDeletable": {
			reason:   "A resource that will not go away holds the XR, which reports that intervention is needed.",
			observer: observing("vpc", "subnet"),
			gc:       true,
			xr:       teardownXR(ref("vpc", "subnet"), ref("subnet", "vpc")),
			want:     want{done: false, message: "manual intervention is required"},
		},
		"ObserveError": {
			reason: "We can't tear down in order if we can't see what's left, and must not drop the finalizer.",
			observer: ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
				return nil, errors.New("boom")
			}),
			gc:   true,
			xr:   teardownXR(graph...),
			want: want{done: false, err: true},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			deleted := []string{}

			r := &Reconciler{log: logging.NewNopLogger(), conditions: conditions.ObservedGenerationPropagationManager{}}
			if tc.observer != nil {
				r.observer = tc.observer
			}

			if tc.gc {
				r.gc = recordingGC(&deleted)
			}

			status := r.conditions.For(tc.xr)

			done, err := r.teardown(context.Background(), tc.xr, status)

			if tc.want.err != (err != nil) {
				t.Fatalf("\n%s\nteardown(...): want error: %v, got: %v", tc.reason, tc.want.err, err)
			}

			if diff := cmp.Diff(tc.want.done, done); diff != "" {
				t.Errorf("\n%s\nteardown(...): -want done, +got done:\n%s", tc.reason, diff)
			}

			if tc.want.deleted != nil {
				if diff := cmp.Diff(tc.want.deleted, deleted); diff != "" {
					t.Errorf("\n%s\nteardown(...): -want deleted, +got deleted:\n%s", tc.reason, diff)
				}
			}

			if tc.want.message != "" {
				// Deleting() is the Ready condition with reason Deleting.
				got := tc.xr.GetCondition(xpv2.TypeReady).Message
				if !strings.Contains(got, tc.want.message) {
					t.Errorf("\n%s\nteardown(...): Deleting condition message %q does not contain %q", tc.reason, got, tc.want.message)
				}
			}
		})
	}
}

// TestTeardownWaitingMessage covers the three states teardown can be in while
// it still has work to do. The distinction is the point: the graph nominates
// the same leaf every reconcile whether its deletion is merely slow or will
// never finish, so an XR stuck forever must not read like one about to
// succeed. An e2e caught that exact defect; these pin it.
func TestTeardownWaitingMessage(t *testing.T) {
	deleting := func(kind, name string, since time.Duration) ComposedResourceState {
		s := state(kind, name)
		ts := metav1.NewTime(time.Now().Add(-since))
		s.Resource.SetDeletionTimestamp(&ts)

		return s
	}

	cases := map[string]struct {
		reason   string
		observed ComposedResourceStates
		deleting []string
		// contains are substrings the message must have; omits are ones it
		// must not.
		contains []string
		omits    []string
	}{
		"AllNewlyRequested": {
			reason: "A wave that has only just asked is ordinary progress and shouldn't suggest anything is wrong.",
			observed: ComposedResourceStates{
				"first":  state("Thing", "xr-first"),
				"second": state("Thing", "xr-second"),
			},
			deleting: []string{"second"},
			contains: []string{"Deleting 1 composed resource(s) in dependency order: second", "2 remaining"},
			omits:    []string{"manual intervention", "already asked"},
		},
		"AllAlreadyAsked": {
			reason: "Everything this wave can delete has been asked and is still here, so teardown isn't progressing and someone may need to look.",
			observed: ComposedResourceStates{
				"first":  state("Thing", "xr-first"),
				"second": deleting("Thing", "xr-second", 90*time.Second),
			},
			deleting: []string{"second"},
			contains: []string{"already asked to delete", "second (deleting for ", "manual intervention"},
			omits:    []string{"in dependency order"},
		},
		"Mixed": {
			reason: "A wave that is partly moving should report both halves rather than hiding the stuck one behind the progress.",
			observed: ComposedResourceStates{
				"first":  state("Thing", "xr-first"),
				"second": deleting("Thing", "xr-second", 30*time.Second),
				"third":  state("Thing", "xr-third"),
			},
			deleting: []string{"second", "third"},
			contains: []string{"Deleting 1 composed resource(s) in dependency order: third", "Still waiting on: second (deleting for "},
		},
		"UnobservedNamesAreIgnored": {
			reason: "A name the graph nominated but that has already gone shouldn't appear at all.",
			observed: ComposedResourceStates{
				"first": state("Thing", "xr-first"),
			},
			deleting: []string{"first", "vanished"},
			contains: []string{"first"},
			omits:    []string{"vanished"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := teardownWaitingMessage(tc.observed, tc.deleting)

			for _, want := range tc.contains {
				if !strings.Contains(got, want) {
					t.Errorf("\n%s\nteardownWaitingMessage(...) = %q\nwant it to contain %q", tc.reason, got, want)
				}
			}

			for _, unwanted := range tc.omits {
				if strings.Contains(got, unwanted) {
					t.Errorf("\n%s\nteardownWaitingMessage(...) = %q\nwant it NOT to contain %q", tc.reason, got, unwanted)
				}
			}
		})
	}
}
