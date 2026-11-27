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

package discovery

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

func TestDiscoverExternal(t *testing.T) {
	mrd := &v1alpha1.ManagedResourceDefinition{}
	mrd.Name = "records.dns.example.org"

	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	inventory := func(pc *xpv2.Reference, scopes ...v1alpha1.ExternalInventoryScopeStatus) *v1alpha1.ExternalInventory {
		inv := &v1alpha1.ExternalInventory{}
		inv.Name = mrd.Name
		inv.Spec.ProviderConfigRef = pc
		inv.Status.Scopes = scopes
		return inv
	}

	pcDefault := &xpv2.Reference{Name: "default"}

	cases := map[string]struct {
		objects []*v1alpha1.ExternalInventory
		pcName  string
		want    []v1alpha1.ExternalResource
		wantErr string
	}{
		"NoInventory": {
			pcName:  "default",
			wantErr: "cannot get ExternalInventory",
		},
		"ListedWithOtherProviderConfig": {
			objects: []*v1alpha1.ExternalInventory{inventory(&xpv2.Reference{Name: "other"})},
			pcName:  "default",
			wantErr: "was listed with ProviderConfig \"other\"",
		},
		"ScopeFailed": {
			objects: []*v1alpha1.ExternalInventory{inventory(pcDefault,
				v1alpha1.ExternalInventoryScopeStatus{Scope: v1alpha1.ExternalInventoryScope{"zone": "a.example"}, Error: "boom"},
				v1alpha1.ExternalInventoryScopeStatus{Scope: v1alpha1.ExternalInventoryScope{"zone": "b.example"}},
			)},
			pcName:  "default",
			wantErr: "1 of 2 scopes failed to list",
		},
		"ItemsFromAllScopes": {
			objects: []*v1alpha1.ExternalInventory{inventory(pcDefault,
				v1alpha1.ExternalInventoryScopeStatus{
					Scope: v1alpha1.ExternalInventoryScope{"zone": "a.example"},
					Items: []v1alpha1.ExternalInventoryItem{{ExternalName: "record-1", Scope: v1alpha1.ExternalInventoryScope{"zone": "a.example"}}},
				},
				v1alpha1.ExternalInventoryScopeStatus{
					Scope: v1alpha1.ExternalInventoryScope{"zone": "b.example"},
					Items: []v1alpha1.ExternalInventoryItem{{ExternalName: "record-1", Scope: v1alpha1.ExternalInventoryScope{"zone": "b.example"}}},
				},
			)},
			pcName: "default",
			want: []v1alpha1.ExternalResource{
				{ExternalName: "record-1", Scope: map[string]string{"zone": "a.example"}},
				{ExternalName: "record-1", Scope: map[string]string{"zone": "b.example"}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			b := fake.NewClientBuilder().WithScheme(scheme)
			for _, o := range tc.objects {
				b = b.WithObjects(o)
			}

			r := &Reconciler{client: b.Build()}

			got, err := r.discoverExternal(context.Background(), logging.NewNopLogger(), mrd, tc.pcName)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("discoverExternal(): want error containing %q, got none", tc.wantErr)
				}
				if msg := err.Error(); !strings.Contains(msg, tc.wantErr) {
					t.Fatalf("discoverExternal(): want error containing %q, got %q", tc.wantErr, msg)
				}
				return
			}

			if err != nil {
				t.Fatalf("discoverExternal(): unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("discoverExternal(): -want, +got:\n%s", diff)
			}
		})
	}
}
