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

package beta

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
)

// testMRD builds a ManagedResourceDefinition for tests. The spec embeds
// CustomResourceDefinitionSpec, and promoted fields cannot be set in a
// composite literal before go1.27.
func testMRD(group, kind, plural string) *v1alpha1.ManagedResourceDefinition {
	mrd := &v1alpha1.ManagedResourceDefinition{}
	mrd.Spec.Group = group
	mrd.Spec.Names = extv1.CustomResourceDefinitionNames{Kind: kind, Plural: plural}
	mrd.Spec.Versions = []v1alpha1.CustomResourceDefinitionVersion{{Name: "v1", Served: true, Storage: true}}
	return mrd
}

// TestGenerateMRName tests that the name generation is stable and consistent.
func TestGenerateMRName(t *testing.T) {
	type args struct {
		kind         string
		externalName string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Simple bucket name",
			args: args{
				kind:         "Bucket",
				externalName: "my-bucket",
			},
		},
		{
			name: "Name with special characters",
			args: args{
				kind:         "Bucket",
				externalName: "bucket-prod-2025-01-15",
			},
		},
		{
			name: "Different kinds produce different prefixes",
			args: args{
				kind:         "Database",
				externalName: "db-prod",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate name twice to test idempotency
			got1 := generateMRName(tt.args.kind, tt.args.externalName, nil)
			got2 := generateMRName(tt.args.kind, tt.args.externalName, nil)

			if got1 != got2 {
				t.Errorf("generateMRName not idempotent: first=%q, second=%q", got1, got2)
			}

			// Verify format (should be <kind>-<hash>)
			if len(got1) < len(tt.args.kind)+9 { // kind + dash + 8-char hash
				t.Errorf("generateMRName produced too short name: %q", got1)
			}
		})
	}
}

// TestUnmanagedResources tests the identity-based difference between discovered
// resources and existing managed resources.
func TestUnmanagedResources(t *testing.T) {
	tests := []struct {
		name     string
		external []externalResource
		existing []existingResource
		expected []string // External names of the unmanaged resources.
	}{
		{
			name:     "Empty",
			external: nil,
			existing: nil,
			expected: nil,
		},
		{
			name:     "All unmanaged",
			external: []externalResource{{Name: "bucket-1"}, {Name: "bucket-2"}},
			existing: nil,
			expected: []string{"bucket-1", "bucket-2"},
		},
		{
			name:     "All managed",
			external: []externalResource{{Name: "bucket-1"}},
			existing: []existingResource{{Name: "bucket-1"}},
			expected: nil,
		},
		{
			name: "Same name in another scope is unmanaged",
			external: []externalResource{
				{Name: "record-1", Scope: map[string]string{"zone": "a.example"}},
				{Name: "record-1", Scope: map[string]string{"zone": "b.example"}},
			},
			existing: []existingResource{
				{Name: "record-1", ForProvider: map[string]any{"zone": "a.example"}},
			},
			expected: []string{"record-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unmanagedResources(tt.external, tt.existing)

			var names []string
			for _, r := range got {
				names = append(names, r.Name)
			}

			if diff := cmp.Diff(tt.expected, names); diff != "" {
				t.Errorf("unmanagedResources(...): -want, +got:\n%s", diff)
			}
		})
	}
}

// TestListExternalResources tests extracting unmanaged resources from a DiscoveryReport.
func TestListExternalResources(t *testing.T) {
	// Create a sample DiscoveryReport with unmanaged resources
	dr := &v1alpha1.DiscoveryReport{
		ObjectMeta: metav1.ObjectMeta{Name: "test-discovery"},
		Status: v1alpha1.DiscoveryReportStatus{
			DiscoveredResourceCount: new(int64(5)),
			UnmanagedResourceCount:  new(int64(2)),
			UnmanagedResources: []v1alpha1.ExternalResource{
				{ExternalName: "unmanaged-bucket-1"},
				{ExternalName: "unmanaged-bucket-2"},
			},
			LastDiscoveryTime: &metav1.Time{Time: metav1.Now().Time},
		},
	}

	// Extract the resources from the DiscoveryReport status
	// This mirrors what listExternalResources() does in the CLI
	resources := dr.Status.UnmanagedResources

	if len(resources) != 2 {
		t.Errorf("expected 2 unmanaged resources, got %d", len(resources))
	}

	if resources[0].ExternalName != "unmanaged-bucket-1" {
		t.Errorf("expected first resource 'unmanaged-bucket-1', got %q", resources[0].ExternalName)
	}

	if resources[1].ExternalName != "unmanaged-bucket-2" {
		t.Errorf("expected second resource 'unmanaged-bucket-2', got %q", resources[1].ExternalName)
	}

	// Verify the counts match the resource list
	expectedCount := int64(len(resources))
	if *dr.Status.UnmanagedResourceCount != expectedCount {
		t.Errorf("expected unmanaged count %d, got %d", expectedCount, *dr.Status.UnmanagedResourceCount)
	}
}

// TestGenerateManifests tests YAML generation for adoption.
func TestGenerateManifests(t *testing.T) {
	type args struct {
		mrd           *v1alpha1.ManagedResourceDefinition
		externalNames []externalResource
	}

	type want struct {
		count       int
		kind        string
		apiVersion  string
		external    string
		policies    []any
		providerRef string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SingleAdoption": {
			reason: "A single unmanaged resource should produce one managed resource with conservative management policies.",
			args: args{
				mrd:           testMRD("s3.aws.crossplane.io", "Bucket", "buckets"),
				externalNames: resources("my-bucket"),
			},
			want: want{
				count:       1,
				kind:        "Bucket",
				apiVersion:  "s3.aws.crossplane.io/v1",
				external:    "my-bucket",
				policies:    []any{"Observe", "LateInitialize"},
				providerRef: "default",
			},
		},
		"MultipleAdoptionsSorted": {
			reason: "Multiple unmanaged resources should each produce a managed resource, in deterministic sorted order.",
			args: args{
				mrd:           testMRD("s3.aws.crossplane.io", "Bucket", "buckets"),
				externalNames: resources("bucket-c", "bucket-a", "bucket-b"),
			},
			want: want{count: 3, kind: "Bucket", apiVersion: "s3.aws.crossplane.io/v1", external: "bucket-a", policies: []any{"Observe", "LateInitialize"}, providerRef: "default"},
		},
		"CustomProviderConfig": {
			reason: "The requested ProviderConfig should be set, rather than relying on the CRD default of 'default'.",
			args: args{
				mrd:           testMRD("s3.aws.crossplane.io", "Bucket", "buckets"),
				externalNames: resources("my-bucket"),
			},
			want: want{count: 1, kind: "Bucket", apiVersion: "s3.aws.crossplane.io/v1", external: "my-bucket", policies: []any{"Observe", "LateInitialize"}, providerRef: "custom-pc"},
		},
		"NoResources": {
			reason: "No unmanaged resources should produce no manifests.",
			args: args{
				mrd:           testMRD("s3.aws.crossplane.io", "Bucket", "buckets"),
				externalNames: resources(),
			},
			want: want{count: 0},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pc := "default"
			if tc.want.providerRef != "" {
				pc = tc.want.providerRef
			}
			c := &discoverCmd{Namespace: "default", ProviderConfig: pc}

			got, err := c.generateManifests(tc.args.mrd, tc.args.externalNames)
			if err != nil {
				t.Fatalf("\n%s\ngenerateManifests(...): unexpected error: %v", tc.reason, err)
			}

			if diff := cmp.Diff(tc.want.count, len(got)); diff != "" {
				t.Errorf("\n%s\ngenerateManifests(...): -want count, +got count:\n%s", tc.reason, diff)
			}

			if tc.want.count == 0 {
				return
			}

			first := got[0]
			if first.GetKind() != tc.want.kind {
				t.Errorf("\n%s\nwant kind %q, got %q", tc.reason, tc.want.kind, first.GetKind())
			}
			if first.GetAPIVersion() != tc.want.apiVersion {
				t.Errorf("\n%s\nwant apiVersion %q, got %q", tc.reason, tc.want.apiVersion, first.GetAPIVersion())
			}
			if got := first.GetAnnotations()["crossplane.io/external-name"]; got != tc.want.external {
				t.Errorf("\n%s\nwant external-name %q, got %q", tc.reason, tc.want.external, got)
			}

			// The management policies must survive generation. If they are
			// missing the CRD defaults them to "*" and Crossplane will delete
			// the external resource along with the managed resource.
			policies, found, err := unstructured.NestedSlice(first.Object, "spec", "managementPolicies")
			if err != nil || !found {
				t.Fatalf("\n%s\nmanagementPolicies missing from generated object: found=%v err=%v", tc.reason, found, err)
			}
			if diff := cmp.Diff(tc.want.policies, policies); diff != "" {
				t.Errorf("\n%s\nmanagementPolicies: -want, +got:\n%s", tc.reason, diff)
			}
			for _, p := range policies {
				if p == "*" {
					t.Errorf("\n%s\ngenerated full-management policy %q; adoption must never be fully managed", tc.reason, p)
				}
			}

			// The ProviderConfig must be set explicitly, for the same reason.
			pcName, found, err := unstructured.NestedString(first.Object, "spec", "providerConfigRef", "name")
			if err != nil || !found {
				t.Fatalf("\n%s\nproviderConfigRef missing from generated object: found=%v err=%v", tc.reason, found, err)
			}
			if pcName != tc.want.providerRef {
				t.Errorf("\n%s\nwant providerConfigRef %q, got %q", tc.reason, tc.want.providerRef, pcName)
			}
		})
	}
}

// TestManagedVersion tests that the storage version of the MRD is used, rather
// than assuming v1.
func TestManagedVersion(t *testing.T) {
	cases := map[string]struct {
		versions []v1alpha1.CustomResourceDefinitionVersion
		want     string
		wantErr  bool
	}{
		"StorageVersionV1Alpha1": {
			versions: []v1alpha1.CustomResourceDefinitionVersion{
				{Name: "v1alpha1", Served: true, Storage: true},
				{Name: "v1beta1", Served: true, Storage: false},
			},
			want: "v1alpha1",
		},
		"NoStorageVersion": {
			versions: []v1alpha1.CustomResourceDefinitionVersion{
				{Name: "v1", Served: true, Storage: false},
			},
			wantErr: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mrd := testMRD("s3.aws.crossplane.io", "Bucket", "buckets")
			mrd.Spec.Versions = tc.versions

			got, err := managedVersion(mrd)
			if (err != nil) != tc.wantErr {
				t.Fatalf("managedVersion(...): unexpected error state: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("managedVersion(...): -want, +got:\n%s", diff)
			}
		})
	}
}

// TestRenderManifestsRoundTrip verifies that rendering to YAML and parsing back
// preserves the spec. This is the regression test for the auto-import bug where
// the hand-written parser silently dropped managementPolicies, causing the CRD
// default of "*" to grant full management over adopted resources.
func TestRenderManifestsRoundTrip(t *testing.T) {
	mrd := testMRD("s3.aws.crossplane.io", "Bucket", "buckets")
	c := &discoverCmd{Namespace: "default", ProviderConfig: "default"}

	objs, err := c.generateManifests(mrd, resources("bucket-a", "bucket-b"))
	if err != nil {
		t.Fatalf("generateManifests(...): unexpected error: %v", err)
	}

	rendered, err := renderManifests(objs)
	if err != nil {
		t.Fatalf("renderManifests(...): unexpected error: %v", err)
	}

	if got, want := strings.Count(rendered, "\n---\n")+1, 2; got != want {
		t.Errorf("renderManifests(...): want %d documents, got %d", want, got)
	}

	for doc := range strings.SplitSeq(rendered, "\n---\n") {
		parsed := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(doc), &parsed.Object); err != nil {
			t.Fatalf("cannot parse rendered manifest: %v", err)
		}

		policies, found, err := unstructured.NestedSlice(parsed.Object, "spec", "managementPolicies")
		if err != nil || !found {
			t.Fatalf("managementPolicies lost in round-trip: found=%v err=%v", found, err)
		}
		if diff := cmp.Diff([]any{"Observe", "LateInitialize"}, policies); diff != "" {
			t.Errorf("managementPolicies lost in round-trip: -want, +got:\n%s", diff)
		}

		if _, found, _ := unstructured.NestedString(parsed.Object, "spec", "providerConfigRef", "name"); !found {
			t.Error("providerConfigRef lost in round-trip")
		}
	}
}

// resources returns external resources with the given names and no scope.
func resources(names ...string) []externalResource {
	out := make([]externalResource, 0, len(names))
	for _, n := range names {
		out = append(out, externalResource{Name: n})
	}

	return out
}
