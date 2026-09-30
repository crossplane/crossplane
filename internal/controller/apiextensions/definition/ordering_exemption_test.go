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
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

func TestPendingDependencyExemption(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "XR"}
	target := types.NamespacedName{Namespace: "default", Name: "xr"}

	// The XR is holding back subnet, which waits on vpc, and a database that
	// waits on a required resource called vpc - which isn't the composed vpc -
	// and is deleting old, which waits on nothing composed.
	xr := map[string]any{
		"status": map[string]any{"crossplane": map[string]any{"pendingResources": []any{
			map[string]any{
				"resourceName": "subnet",
				"operation":    "Create",
				"dependsOn":    []any{map[string]any{"name": "vpc"}},
			},
			map[string]any{
				"resourceName": "database",
				"operation":    "Create",
				"dependsOn":    []any{map[string]any{"type": "RequiredResource", "name": "db-config", "requirement": map[string]any{"name": "vpc"}}},
			},
			map[string]any{
				"resourceName": "old",
				"operation":    "Delete",
			},
		}}},
	}
	c := &test.MockClient{MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
		u, _ := obj.(*kunstructured.Unstructured)
		u.Object = xr
		return nil
	}}

	composed := func(name string) client.Object {
		u := &kunstructured.Unstructured{}
		u.SetUID(types.UID("uid-" + name))
		if name != "" {
			u.SetAnnotations(map[string]string{annotationCompositionResourceName: name})
		}
		return u
	}

	cases := map[string]struct {
		reason string
		obj    client.Object
		want   bool
	}{
		"DependencyOfPendingCreate": {
			reason: "An event from something a pending creation waits on is what releases the next wave, so it's exempt.",
			obj:    composed("vpc"),
			want:   true,
		},
		"NotADependency": {
			reason: "An event from a composed resource nothing pending waits on is metered like any other.",
			obj:    composed("gateway"),
			want:   false,
		},
		"PendingItself": {
			reason: "A resource that's itself pending isn't a dependency of anything, so its events aren't exempt.",
			obj:    composed("subnet"),
			want:   false,
		},
		"NotComposed": {
			reason: "Something without a composition resource name isn't a composed resource, and can't be one a pending creation depends on.",
			obj:    composed(""),
			want:   false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := NewPendingDependencyExemption(c, gvk)
			if diff := cmp.Diff(tc.want, e(context.Background(), tc.obj, target)); diff != "" {
				t.Errorf("\n%s\nexempt: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPendingDependencyExemptionIsRateLimited(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "XR"}
	c := &test.MockClient{MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
		u, _ := obj.(*kunstructured.Unstructured)
		u.Object = map[string]any{"status": map[string]any{"crossplane": map[string]any{"pendingResources": []any{
			map[string]any{"resourceName": "subnet", "operation": "Create", "dependsOn": []any{map[string]any{"name": "vpc"}}},
		}}}}
		return nil
	}}
	vpc := &kunstructured.Unstructured{}
	vpc.SetUID("uid-vpc")
	vpc.SetAnnotations(map[string]string{annotationCompositionResourceName: "vpc"})

	e := NewPendingDependencyExemption(c, gvk)
	a := types.NamespacedName{Namespace: "default", Name: "a"}
	b := types.NamespacedName{Namespace: "default", Name: "b"}

	// A dependency that flaps without becoming ready must not wake its XR
	// unmetered: the first event is exempt, the next within the interval isn't.
	if !e(context.Background(), vpc, a) {
		t.Fatal("first event: want exempt")
	}
	if e(context.Background(), vpc, a) {
		t.Error("second event within the interval: want metered, got exempt")
	}

	// The limit is per XR, so the same source waking another XR isn't held
	// back by the first.
	if !e(context.Background(), vpc, b) {
		t.Error("same source, another XR: want exempt")
	}
}
