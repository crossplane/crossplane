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

package v1alpha1

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestExternalInventoryRegistered(t *testing.T) {
	s := runtime.NewScheme()
	if err := AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme(...): unexpected error: %v", err)
	}

	if _, err := s.New(ExternalInventoryGroupVersionKind); err != nil {
		t.Errorf("s.New(%v): unexpected error: %v", ExternalInventoryGroupVersionKind, err)
	}

	if _, err := s.New(SchemeGroupVersion.WithKind(ExternalInventoryKind + "List")); err != nil {
		t.Errorf("s.New(%sList): unexpected error: %v", ExternalInventoryKind, err)
	}
}

func TestExternalInventoryJSONRoundTrip(t *testing.T) {
	in := &ExternalInventory{
		Spec: ExternalInventorySpec{},
		Status: ExternalInventoryStatus{
			Scopes: []ExternalInventoryScopeStatus{
				{
					Scope: ExternalInventoryScope{"zone": "abc"},
					Items: []ExternalInventoryItem{
						{ExternalName: "023e105f", Scope: ExternalInventoryScope{"zone": "abc"}},
					},
					NextPageToken: "page-2",
				},
				{
					Scope: ExternalInventoryScope{"zone": "def"},
					Error: "rate limited",
				},
			},
		},
	}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("json.Marshal(...): unexpected error: %v", err)
	}

	out := &ExternalInventory{}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("json.Unmarshal(...): unexpected error: %v", err)
	}

	if diff := cmp.Diff(in.Status, out.Status); diff != "" {
		t.Errorf("round-tripped status: -want, +got:\n%s", diff)
	}
}

func TestExternalInventoryDeepCopyIsIndependent(t *testing.T) {
	in := &ExternalInventory{
		Status: ExternalInventoryStatus{
			Scopes: []ExternalInventoryScopeStatus{
				{Scope: ExternalInventoryScope{"zone": "abc"}},
			},
		},
	}

	out := in.DeepCopy()
	out.Status.Scopes[0].Scope["zone"] = "changed"

	if got := in.Status.Scopes[0].Scope["zone"]; got != "abc" {
		t.Errorf("DeepCopy shares scope map with the original: got %q, want %q", got, "abc")
	}
}
