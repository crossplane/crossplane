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

package composite

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite/dependency"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

func TestReferenceFromSelector(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "Thing"}
	ns := "cool-ns"

	cases := map[string]struct {
		reason string
		s      *fnv1.ResourceSelector
		want   dependency.Reference
	}{
		"MatchName": {
			reason: "A match-name selector becomes a reference matched by name.",
			s: &fnv1.ResourceSelector{
				ApiVersion: "example.org/v1",
				Kind:       "Thing",
				Namespace:  &ns,
				Match:      &fnv1.ResourceSelector_MatchName{MatchName: "foo"},
			},
			want: dependency.Reference{GVK: gvk, Namespace: ns, Name: "foo"},
		},
		"MatchLabels": {
			reason: "A match-labels selector becomes a reference matched by labels.",
			s: &fnv1.ResourceSelector{
				ApiVersion: "example.org/v1",
				Kind:       "Thing",
				Match:      &fnv1.ResourceSelector_MatchLabels{MatchLabels: &fnv1.MatchLabels{Labels: map[string]string{"k": "v"}}},
			},
			want: dependency.Reference{GVK: gvk, Labels: map[string]string{"k": "v"}},
		},
		"MatchAll": {
			reason: "A selector with no match becomes a reference that matches every resource of its kind.",
			s: &fnv1.ResourceSelector{
				ApiVersion: "example.org/v1",
				Kind:       "Thing",
				Namespace:  &ns,
			},
			want: dependency.Reference{GVK: gvk, Namespace: ns},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ReferenceFromSelector(tc.s)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nreferenceFromSelector(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSelectorFromReference(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "Thing"}
	ns := "cool-ns"

	cases := map[string]struct {
		reason string
		r      dependency.Reference
		want   *fnv1.ResourceSelector
	}{
		"MatchName": {
			reason: "A reference matched by name becomes a match-name selector.",
			r:      dependency.Reference{GVK: gvk, Namespace: ns, Name: "foo"},
			want: &fnv1.ResourceSelector{
				ApiVersion: "example.org/v1",
				Kind:       "Thing",
				Namespace:  &ns,
				Match:      &fnv1.ResourceSelector_MatchName{MatchName: "foo"},
			},
		},
		"MatchLabels": {
			reason: "A reference matched by labels becomes a match-labels selector.",
			r:      dependency.Reference{GVK: gvk, Labels: map[string]string{"k": "v"}},
			want: &fnv1.ResourceSelector{
				ApiVersion: "example.org/v1",
				Kind:       "Thing",
				Match:      &fnv1.ResourceSelector_MatchLabels{MatchLabels: &fnv1.MatchLabels{Labels: map[string]string{"k": "v"}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := SelectorFromReference(tc.r)
			if diff := cmp.Diff(tc.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nselectorFromReference(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
