/*
Copyright 2024 The Crossplane Authors.

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

package internal

import (
	"github.com/crossplane/crossplane/cmd/crank/beta/internal/testutils"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	coolResource = map[string]interface{}{
		"apiVersion": "example.org/v1alpha1",
		"kind":       "ComposedResource",
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"crossplane.io/composition-resource-name": "resource-a",
			},
			"name": "test-validate-a",
		},
		"spec": map[string]interface{}{
			"coolField": "I'm cool!",
		},
	}
	coolerResource = map[string]interface{}{
		"apiVersion": "example.org/v1alpha1",
		"kind":       "ComposedResource",
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"crossplane.io/composition-resource-name": "resource-b",
			},
			"name": "test-validate-b",
		},
		"spec": map[string]interface{}{
			"coolerField": "I'm cooler!",
		},
	}
)

func TestFileLoaderLoad(t *testing.T) {
	type args struct {
		Path string
	}
	type want struct {
		resources []*un.Unstructured
		err       error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Successfully load resources from file",
			args: args{
				Path: "testdata/resources.yaml",
			},
			want: want{
				resources: []*un.Unstructured{
					{
						Object: coolResource,
					},
					{
						Object: coolerResource,
					},
				},
			},
		},
		"Error": {
			reason: "Error loading resources from file",
			args: args{
				Path: "testdata/does-not-exist.yaml",
			},
			want: want{
				resources: nil,
				err:       cmpopts.AnyError,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &FileLoader{
				path: tc.args.Path,
			}
			got, err := f.Load()
			if diff := cmp.Diff(tc.want.resources, got); diff != "" {
				t.Errorf("%s\nLoad(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nLoad(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFolderLoaderLoad(t *testing.T) {
	type args struct {
		Path string
	}
	type want struct {
		resources []*un.Unstructured
		err       error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Successfully load resources from folder",
			args: args{
				Path: "testdata/folder",
			},
			want: want{
				resources: []*un.Unstructured{
					{
						Object: coolResource,
					},
					{
						Object: coolerResource,
					},
				},
			},
		},
		"Error": {
			reason: "Error loading resources from folder",
			args: args{
				Path: "testdata/does-not-exist",
			},
			want: want{
				resources: nil,
				err:       cmpopts.AnyError,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &FolderLoader{
				path: tc.args.Path,
			}
			got, err := f.Load()
			if diff := cmp.Diff(tc.want.resources, got); diff != "" {
				t.Errorf("%s\nLoad(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nLoad(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestStreamToUnstructured(t *testing.T) {
	type args struct {
		stream [][]byte
	}
	type want struct {
		resources []*un.Unstructured
		err       error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Successfully parse stream to unstructured resources",
			args: args{
				stream: [][]byte{
					[]byte("apiVersion: v1\nkind: Pod\nmetadata:\n  name: test"),
				},
			},
			want: want{
				resources: []*un.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Pod",
							"metadata": map[string]interface{}{
								"name": "test",
							},
						},
					},
				},
			},
		},
		"Error": {
			reason: "Error parsing stream to unstructured resources",
			args: args{
				stream: [][]byte{
					[]byte("this is not a yaml"),
				},
			},
			want: want{
				resources: nil,
				err:       cmpopts.AnyError,
			},
		},
		"CompositionWithPipelineResources": {
			reason: "Successfully parse Composition with pipeline input resources to unstructured resources",
			args: args{
				stream: [][]byte{
					[]byte(`
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example-composition
spec:
  compositeTypeRef:
    apiVersion: example.crossplane.io/v1alpha1
    kind: ExampleComposite
  pipeline:
    - step: patch-and-transform
      functionRef:
        name: example-function
      input:
        apiVersion: pt.fn.crossplane.io/v1beta1
        kind: Resources
        resources:
          - name: instanceNodeRole
            base:
              apiVersion: iam.aws.upbound.io/v1beta1
              kind: Role
              spec: {}
`),
				},
			},
			want: want{
				resources: []*un.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "pt.fn.crossplane.io/v1beta1",
							"kind":       "Resources",
							"resources": []interface{}{
								map[string]interface{}{
									"name": "instanceNodeRole",
									"base": map[string]interface{}{
										"apiVersion": "iam.aws.upbound.io/v1beta1",
										"kind":       "Role",
										"spec":       map[string]interface{}{},
									},
								},
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.crossplane.io/v1",
							"kind":       "Composition",
							"metadata": map[string]interface{}{
								"name": "example-composition",
							},
							"spec": map[string]interface{}{
								"compositeTypeRef": map[string]interface{}{
									"apiVersion": "example.crossplane.io/v1alpha1",
									"kind":       "ExampleComposite",
								},
								"pipeline": []interface{}{
									map[string]interface{}{
										"step": "patch-and-transform",
										"functionRef": map[string]interface{}{
											"name": "example-function",
										},
										"input": map[string]interface{}{
											"apiVersion": "pt.fn.crossplane.io/v1beta1",
											"kind":       "Resources",
											"resources": []interface{}{
												map[string]interface{}{
													"name": "instanceNodeRole",
													"base": map[string]interface{}{
														"apiVersion": "iam.aws.upbound.io/v1beta1",
														"kind":       "Role",
														"spec":       map[string]interface{}{},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := streamToUnstructured(tc.args.stream)
			if diff := cmp.Diff(tc.want.resources, got); diff != "" {
				t.Errorf("%s\nstreamToUnstructured(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nstreamToUnstructured(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompositeLoader_Load(t *testing.T) {
	// Create temporary test files
	tempDir, err := os.MkdirTemp("", "loader-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file with a single resource
	file1 := filepath.Join(tempDir, "file1.yaml")
	content1 := []byte(`
apiVersion: example.org/v1
kind: Resource1
metadata:
  name: resource-one
spec:
  field: value1
`)
	if err := os.WriteFile(file1, content1, 0600); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Create a file with two resources
	file2 := filepath.Join(tempDir, "file2.yaml")
	content2 := []byte(`
apiVersion: example.org/v1
kind: Resource2
metadata:
  name: resource-two
spec:
  field: value2
---
apiVersion: example.org/v1
kind: Resource3
metadata:
  name: resource-three
spec:
  field: value3
`)
	if err := os.WriteFile(file2, content2, 0600); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Create a folder with a file
	folderPath := filepath.Join(tempDir, "subfolder")
	if err := os.Mkdir(folderPath, 0700); err != nil {
		t.Fatalf("Failed to create subfolder: %v", err)
	}

	folderFile := filepath.Join(folderPath, "folder-file.yaml")
	folderContent := []byte(`
apiVersion: example.org/v1
kind: FolderResource
metadata:
  name: folder-resource
spec:
  field: folder-value
`)
	if err := os.WriteFile(folderFile, folderContent, 0600); err != nil {
		t.Fatalf("Failed to write folder file: %v", err)
	}

	// Save original stdin and restore it after the test
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Prepare stdin content to use in stdin tests
	stdinContent := `
apiVersion: example.org/v1
kind: StdinResource
metadata:
  name: stdin-resource
spec:
  field: stdin-value
`

	type setupFunc func(t *testing.T) func()
	type args struct {
		sources []string
	}
	type want struct {
		resourceCount int
		resourceNames []string
		err           error
		setup         setupFunc // Function to set up any test-specific resources (like stdin)
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SingleFile": {
			reason: "Should load resources from a single file",
			args: args{
				sources: []string{file1},
			},
			want: want{
				resourceCount: 1,
				resourceNames: []string{"resource-one"},
				err:           nil,
			},
		},
		"MultipleFiles": {
			reason: "Should load and combine resources from multiple files",
			args: args{
				sources: []string{file1, file2},
			},
			want: want{
				resourceCount: 3,
				resourceNames: []string{"resource-one", "resource-two", "resource-three"},
				err:           nil,
			},
		},
		"FileAndFolder": {
			reason: "Should load resources from both files and folders",
			args: args{
				sources: []string{file1, folderPath},
			},
			want: want{
				resourceCount: 2,
				resourceNames: []string{"resource-one", "folder-resource"},
				err:           nil,
			},
		},
		"NoSources": {
			reason: "Should return error when no sources are specified",
			args: args{
				sources: []string{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"NonExistentSource": {
			reason: "Should return error for non-existent source",
			args: args{
				sources: []string{"non-existent-file.yaml"},
			},
			want: want{
				resourceCount: 0,
				resourceNames: nil,
				err:           cmpopts.AnyError,
			},
		},
		"StdinOnly": {
			reason: "Should load resources from stdin only",
			args: args{
				sources: []string{"-"},
			},
			want: want{
				resourceCount: 1,
				resourceNames: []string{"stdin-resource"},
				setup: func(t *testing.T) func() {
					t.Helper()
					// Create a pipe to simulate stdin
					r, w, err := os.Pipe()
					if err != nil {
						t.Fatalf("Failed to create pipe: %v", err)
					}

					// Set os.Stdin to our read pipe
					os.Stdin = r

					// Write content to the pipe (simulating stdin input)
					go func() {
						defer w.Close()
						_, err := io.WriteString(w, stdinContent)
						if err != nil {
							t.Errorf("Failed to write to stdin pipe: %v", err)
						}
					}()

					// Return a cleanup function
					return func() {
						// No need to restore os.Stdin here as we do it at the end of the main test
					}
				},
			},
		},
		"FileAndStdin": {
			reason: "Should load resources from file and stdin",
			args: args{
				sources: []string{file1, "-"},
			},
			want: want{
				resourceCount: 2,
				resourceNames: []string{"resource-one", "stdin-resource"},
				setup: func(t *testing.T) func() {
					t.Helper()
					// Create a pipe to simulate stdin
					r, w, err := os.Pipe()
					if err != nil {
						t.Fatalf("Failed to create pipe: %v", err)
					}

					// Set os.Stdin to our read pipe
					os.Stdin = r

					// Write content to the pipe (simulating stdin input)
					go func() {
						defer w.Close()
						_, err := io.WriteString(w, stdinContent)
						if err != nil {
							t.Errorf("Failed to write to stdin pipe: %v", err)
						}
					}()

					// Return a cleanup function
					return func() {
						// No need to restore os.Stdin here as we do it at the end of the main test
					}
				},
			},
		},
		"StdinAndFile": {
			reason: "Should load resources from stdin and file in correct order",
			args: args{
				sources: []string{"-", file1},
			},
			want: want{
				resourceCount: 2,
				resourceNames: []string{"stdin-resource", "resource-one"},
				setup: func(t *testing.T) func() {
					t.Helper()
					// Create a pipe to simulate stdin
					r, w, err := os.Pipe()
					if err != nil {
						t.Fatalf("Failed to create pipe: %v", err)
					}

					// Set os.Stdin to our read pipe
					os.Stdin = r

					// Write content to the pipe (simulating stdin input)
					go func() {
						defer w.Close()
						_, err := io.WriteString(w, stdinContent)
						if err != nil {
							t.Errorf("Failed to write to stdin pipe: %v", err)
						}
					}()

					// Return a cleanup function
					return func() {
						// No need to restore os.Stdin here as we do it at the end of the main test
					}
				},
			},
		},
		"DuplicateStdinMarkers": {
			reason: "Should handle multiple stdin markers by using stdin only once",
			args: args{
				sources: []string{"-", "-", file1},
			},
			want: want{
				resourceCount: 2,
				resourceNames: []string{"stdin-resource", "resource-one"},
				setup: func(t *testing.T) func() {
					t.Helper()
					// Create a pipe to simulate stdin
					r, w, err := os.Pipe()
					if err != nil {
						t.Fatalf("Failed to create pipe: %v", err)
					}

					// Set os.Stdin to our read pipe
					os.Stdin = r

					// Write content to the pipe (simulating stdin input)
					go func() {
						defer w.Close()
						_, err := io.WriteString(w, stdinContent)
						if err != nil {
							t.Errorf("Failed to write to stdin pipe: %v", err)
						}
					}()

					// Return a cleanup function
					return func() {
						// No need to restore os.Stdin here as we do it at the end of the main test
					}
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var cleanup func()
			// Run setup if provided
			if tc.want.setup != nil {
				cleanup = tc.want.setup(t)
				defer cleanup()
			}

			// Create the composite loader
			loader, err := NewCompositeLoader(tc.args.sources)
			if err != nil {
				if tc.want.err != nil {
					// If we expect an error at creation time, that's fine
					return
				}
				t.Fatalf("Failed to create composite loader: %v", err)
			}

			// Load the resources
			resources, err := loader.Load()

			// Check error expectation
			if tc.want.err != nil {
				if err == nil {
					t.Errorf("%s\nLoad(): expected error but got none", tc.reason)
				}
				return
			}

			if err != nil {
				t.Errorf("%s\nLoad(): unexpected error: %v", tc.reason, err)
				return
			}

			// Check resource count
			if got := len(resources); got != tc.want.resourceCount {
				t.Errorf("%s\nLoad(): expected %d resources, got %d", tc.reason, tc.want.resourceCount, got)
			}

			// Check resource names
			gotNames := make([]string, 0, len(resources))
			for _, res := range resources {
				gotNames = append(gotNames, res.GetName())
			}

			// Compare names in order (don't sort)
			if diff := cmp.Diff(tc.want.resourceNames, gotNames); diff != "" {
				t.Errorf("%s\nLoad(): resource names mismatch (-want +got):\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompositeLoader_WithMocks(t *testing.T) {
	// Create a composite loader with mock loaders to test the combining logic
	mockLoader1 := &testutils.MockLoader{
		Resources: []*un.Unstructured{
			{
				Object: map[string]interface{}{
					"apiVersion": "test/v1",
					"kind":       "Test1",
					"metadata": map[string]interface{}{
						"name": "test1",
					},
				},
			},
		},
	}

	mockLoader2 := &testutils.MockLoader{
		Resources: []*un.Unstructured{
			{
				Object: map[string]interface{}{
					"apiVersion": "test/v1",
					"kind":       "Test2",
					"metadata": map[string]interface{}{
						"name": "test2",
					},
				},
			},
			{
				Object: map[string]interface{}{
					"apiVersion": "test/v1",
					"kind":       "Test3",
					"metadata": map[string]interface{}{
						"name": "test3",
					},
				},
			},
		},
	}

	// Create a composite loader with the mocks
	compositeLoader := &CompositeLoader{
		loaders: []Loader{mockLoader1, mockLoader2},
	}

	// Test that resources are properly combined
	resources, err := compositeLoader.Load()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(resources) != 3 {
		t.Errorf("Expected 3 resources, got %d", len(resources))
	}

	// Check that resources are in the expected order (loader1's resources first, then loader2's)
	expectedNames := []string{"test1", "test2", "test3"}
	for i, name := range expectedNames {
		if i >= len(resources) {
			t.Errorf("Missing expected resource at index %d", i)
			continue
		}

		if got := resources[i].GetName(); got != name {
			t.Errorf("Expected resource name %s at index %d, got %s", name, i, got)
		}
	}
}
