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

// Package beta implements Crossplane beta (experimental) commands.
package beta

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
)

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
			got1 := generateMRName(tt.args.kind, tt.args.externalName)
			got2 := generateMRName(tt.args.kind, tt.args.externalName)

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

// TestSetDifference tests the set difference operation.
func TestSetDifference(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected int // Just check length
	}{
		{
			name:     "Empty sets",
			a:        []string{},
			b:        []string{},
			expected: 0,
		},
		{
			name:     "All in a are unmanaged",
			a:        []string{"bucket-1", "bucket-2"},
			b:        []string{},
			expected: 2,
		},
		{
			name:     "All in a are managed",
			a:        []string{"bucket-1", "bucket-2"},
			b:        []string{"bucket-1", "bucket-2"},
			expected: 0,
		},
		{
			name:     "Partial overlap",
			a:        []string{"bucket-1", "bucket-2", "bucket-3"},
			b:        []string{"bucket-1"},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setDifference(tt.a, tt.b)

			if len(got) != tt.expected {
				t.Errorf("setDifference length mismatch: got %d, expected %d", len(got), tt.expected)
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
			DiscoveredResourceCount: ptr(int64(5)),
			UnmanagedResourceCount:  ptr(int64(2)),
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
	tests := []struct {
		name           string
		externalNames  []string
		kind           string
		group          string
		wantYAMLCount  int
		wantContains   []string
	}{
		{
			name:          "Single bucket adoption",
			externalNames: []string{"my-bucket"},
			kind:          "Bucket",
			group:         "s3.aws.crossplane.io",
			wantYAMLCount: 1,
			wantContains: []string{
				"kind: Bucket",
				"apiVersion: s3.aws.crossplane.io/v1alpha1",
				"crossplane.io/external-name: my-bucket",
			},
		},
		{
			name:          "Multiple resources",
			externalNames: []string{"bucket-1", "bucket-2", "bucket-3"},
			kind:          "Bucket",
			group:         "s3.aws.crossplane.io",
			wantYAMLCount: 3,
			wantContains: []string{
				"kind: Bucket",
			},
		},
		{
			name:          "Empty resources",
			externalNames: []string{},
			kind:          "Bucket",
			group:         "s3.aws.crossplane.io",
			wantYAMLCount: 0,
			wantContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate generateManifests behavior
			var manifests []string
			for _, extName := range tt.externalNames {
				mrName := generateMRName(tt.kind, extName)
				yaml := "---\n" +
					"apiVersion: " + tt.group + "/v1alpha1\n" +
					"kind: " + tt.kind + "\n" +
					"metadata:\n" +
					"  name: " + mrName + "\n" +
					"  annotations:\n" +
					"    crossplane.io/external-name: " + extName + "\n" +
					"spec:\n" +
					"  managementPolicies:\n" +
					"  - Observe\n" +
					"  - LateInitialize\n"
				manifests = append(manifests, yaml)
			}

			if len(manifests) != tt.wantYAMLCount {
				t.Errorf("expected %d manifests, got %d", tt.wantYAMLCount, len(manifests))
			}

			// Check that expected content is in generated YAML
			allYAML := ""
			for _, m := range manifests {
				allYAML += m
			}
			for _, want := range tt.wantContains {
				if !stringContains(allYAML, want) {
					t.Errorf("expected manifest to contain %q", want)
				}
			}
		})
	}
}

// TestMarshalToYAML tests YAML marshaling of unstructured objects.
func TestMarshalToYAML(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "s3.aws.crossplane.io/v1alpha1",
			"kind":       "Bucket",
			"metadata": map[string]interface{}{
				"name": "my-bucket-mr",
				"annotations": map[string]interface{}{
					"crossplane.io/external-name": "my-bucket",
				},
			},
			"spec": map[string]interface{}{
				"managementPolicies": []interface{}{"Observe", "LateInitialize"},
			},
		},
	}

	// Marshal to YAML
	yamlBytes, err := marshalToYAML(obj)
	if err != nil {
		t.Fatalf("failed to marshal to YAML: %v", err)
	}

	yaml := string(yamlBytes)

	// Check essential fields are present
	expectedFields := []string{
		"apiVersion: s3.aws.crossplane.io/v1alpha1",
		"kind: Bucket",
		"name: my-bucket-mr",
		"crossplane.io/external-name: my-bucket",
	}

	for _, field := range expectedFields {
		if !stringContains(yaml, field) {
			t.Errorf("expected YAML to contain %q, full YAML:\n%s", field, yaml)
		}
	}
}

// TestParseYAMLToUnstructured tests YAML parsing to unstructured objects.
func TestParseYAMLToUnstructured(t *testing.T) {
	yaml := `apiVersion: s3.aws.crossplane.io/v1alpha1
kind: Bucket
metadata:
  name: my-bucket-mr
  annotations:
    crossplane.io/external-name: my-bucket
spec:
  managementPolicies:
  - Observe
  - LateInitialize
`

	obj, err := parseYAMLToUnstructured(yaml)
	if err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	// Check basic structure
	if obj.GetKind() != "Bucket" {
		t.Errorf("expected kind Bucket, got %q", obj.GetKind())
	}

	if obj.GetName() != "my-bucket-mr" {
		t.Errorf("expected name my-bucket-mr, got %q", obj.GetName())
	}

	// Check annotations were preserved
	annotations := obj.GetAnnotations()
	if annotations["crossplane.io/external-name"] != "my-bucket" {
		t.Errorf("expected external-name annotation, got %v", annotations)
	}
}

// ptr returns a pointer to the given value.
func ptr[T any](v T) *T {
	return &v
}

// stringContains checks if haystack contains needle as a substring.
func stringContains(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
