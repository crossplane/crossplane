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

package diff

import (
	"bytes"
	"context"
	"fmt"
	"github.com/alecthomas/kong"
	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal"
	itu "github.com/crossplane/crossplane/cmd/crank/beta/internal/testutils"
	"github.com/google/go-cmp/cmp"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strings"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	dp "github.com/crossplane/crossplane/cmd/crank/beta/diff/diffprocessor"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCmd_Run(t *testing.T) {
	var buf bytes.Buffer

	// Create a Kong context
	parser, err := kong.New(&struct{}{})
	if err != nil {
		t.Fatalf("Failed to create Kong parser: %v", err)
	}
	kongCtx, err := parser.Parse([]string{})
	if err != nil {
		t.Fatalf("Failed to parse Kong context: %v", err)
	}
	kongCtx.Stdout = &buf
	// Create a buffer to capture output

	type fields struct {
		Namespace string
		Files     []string
		NoColor   bool
		Compact   bool
	}

	type args struct {
		client    cc.ClusterClient
		processor dp.DiffProcessor
		loader    internal.Loader
	}

	tests := map[string]struct {
		fields          fields
		args            args
		setupFiles      func() []string
		wantErr         bool
		wantErrContains string
	}{
		"SuccessfulRun": {
			fields: fields{
				Namespace: "default",
				Files:     []string{},
				NoColor:   false,
				Compact:   false,
			},
			args: args{
				client: tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					Build(),
				processor: tu.NewMockDiffProcessor().
					WithSuccessfulInitialize().
					WithSuccessfulPerformDiff().
					Build(),
				loader: &itu.MockLoader{
					Resources: []*un.Unstructured{},
				},
			},
			setupFiles: func() []string {
				// Create a temporary test file
				tempDir, err := os.MkdirTemp("", "diff-test")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

				tempFile := filepath.Join(tempDir, "test-resource.yaml")
				content := `
apiVersion: test.org/v1alpha1
kind: TestResource
metadata:
  name: test-resource
`
				if err := os.WriteFile(tempFile, []byte(content), 0600); err != nil {
					t.Fatalf("Failed to write temp file: %v", err)
				}

				return []string{tempFile}
			},
			wantErr: false,
		},
		"ClientInitializeError": {
			fields: fields{
				Namespace: "default",
				Files:     []string{},
			},
			args: args{
				client: tu.NewMockClusterClient().
					WithFailedInitialize("failed to initialize cluster client").
					Build(),
				processor: tu.NewMockDiffProcessor().
					WithSuccessfulInitialize().
					Build(),
				loader: &itu.MockLoader{
					Resources: []*un.Unstructured{},
				},
			},
			setupFiles: func() []string {
				return []string{}
			},
			wantErr:         true,
			wantErrContains: "cannot initialize client",
		},
		"ProcessorInitializeError": {
			fields: fields{
				Namespace: "default",
				Files:     []string{},
			},
			args: args{
				client: tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					Build(),
				processor: tu.NewMockDiffProcessor().
					WithFailedInitialize("failed to initialize processor").
					Build(),
				loader: &itu.MockLoader{
					Resources: []*un.Unstructured{},
				},
			},
			setupFiles: func() []string {
				return []string{}
			},
			wantErr:         true,
			wantErrContains: "cannot initialize diff processor",
		},
		"LoaderError": {
			fields: fields{
				Namespace: "default",
				Files:     []string{},
			},
			args: args{
				client: tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					Build(),
				processor: tu.NewMockDiffProcessor().
					WithSuccessfulInitialize().
					Build(),
				loader: &itu.MockLoader{
					Err: errors.New("failed to load resources"),
				},
			},
			setupFiles: func() []string {
				return []string{}
			},
			wantErr:         true,
			wantErrContains: "cannot load resources",
		},
		"ProcessResourcesError": {
			fields: fields{
				Namespace: "default",
				Files:     []string{},
			},
			args: args{
				client: tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					Build(),
				processor: tu.NewMockDiffProcessor().
					WithSuccessfulInitialize().
					WithFailedPerformDiff("processing error").
					Build(),
				loader: &itu.MockLoader{
					Resources: []*un.Unstructured{
						tu.NewResource("test.org/v1", "TestResource", "test-resource").Build(),
					},
				},
			},
			setupFiles: func() []string {
				// Create a temporary test file
				tempDir, err := os.MkdirTemp("", "diff-test")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

				tempFile := filepath.Join(tempDir, "test-resource.yaml")
				content := `
apiVersion: test.org/v1alpha1
kind: TestResource
metadata:
  name: test-resource
`
				if err := os.WriteFile(tempFile, []byte(content), 0600); err != nil {
					t.Fatalf("Failed to write temp file: %v", err)
				}

				return []string{tempFile}
			},
			wantErr:         true,
			wantErrContains: "unable to process one or more resources",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup test files if needed
			files := tc.setupFiles()

			c := &Cmd{
				Namespace: tc.fields.Namespace,
				Files:     files,
				NoColor:   tc.fields.NoColor,
				Compact:   tc.fields.Compact,
			}

			err := c.Run(
				kongCtx,
				tu.TestLogger(t, false),
				tc.args.client,
				tc.args.processor,
				tc.args.loader,
			)

			if (err != nil) != tc.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if err != nil && tc.wantErrContains != "" {
				if !strings.Contains(err.Error(), tc.wantErrContains) {
					t.Errorf("Run() error = %v, wantErrContains %v", err, tc.wantErrContains)
				}
			}
		})
	}
}

func TestDiffCommand(t *testing.T) {
	// Create common test resources
	testComposition, _ := createTestCompositionWithExtraResources()
	testXRD := createTestXRD()
	testExtraResource := createExtraResource()
	existingResource := createExistingComposedResource()
	matchingResource := createMatchingComposedResource()

	// Convert the test XRD to unstructured for GetXRDs to return
	xrdUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(testXRD)
	if err != nil {
		t.Fatalf("Failed to convert XRD to unstructured: %v", err)
	}

	tests := map[string]struct {
		setupClient    func() cc.ClusterClient
		setupProcessor func() dp.DiffProcessor
		setupLoader    func() *itu.MockLoader
		expectedOutput string   // Text that should be present in output
		notExpected    []string // Text that should NOT be present in output
		expectError    bool
		errorContains  string
	}{
		// ====== Tests for resources with extra resources ======

		"ExtraResources_ResourceWithDifferentValues": {
			setupClient: func() cc.ClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithSuccessfulCompositionMatch(testComposition).
					WithGetAllResourcesByLabels(func(_ context.Context, gvks []schema.GroupVersionKind, selectors []metav1.LabelSelector) ([]*un.Unstructured, error) {
						// Validate the GVK and selector match what we expect
						if len(gvks) != 1 || len(selectors) != 1 {
							return nil, errors.New("unexpected number of GVKs or selectors")
						}

						// Verify the GVK matches our extra resource - using GVK now instead of GVR
						expectedGVK := schema.GroupVersionKind{
							Group:   "example.org",
							Version: "v1",
							Kind:    "ExtraResource",
						}
						if gvks[0] != expectedGVK {
							return nil, errors.Errorf("unexpected GVK: %v", gvks[0])
						}

						// Verify the selector matches our label selector
						expectedSelector := metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						}
						if !cmp.Equal(selectors[0].MatchLabels, expectedSelector.MatchLabels) {
							return nil, errors.New("unexpected selector")
						}

						return []*un.Unstructured{testExtraResource}, nil
					}).
					WithGetFunctionsFromPipeline(func(*xpextv1.Composition) ([]pkgv1.Function, error) {
						// Return functions for the composition pipeline
						return []pkgv1.Function{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "function-extra-resources",
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "function-patch-and-transform",
								},
							},
						}, nil
					}).
					WithGetXRDs(func(context.Context) ([]*un.Unstructured, error) {
						return []*un.Unstructured{
							{Object: xrdUnstructured},
						}, nil
					}).
					WithGetResource(func(_ context.Context, _ schema.GroupVersionKind, _, name string) (*un.Unstructured, error) {
						if name == "test-xr-composed-resource" {
							return existingResource, nil
						}
						return nil, errors.Errorf("resource %q not found", name)
					}).
					WithGetCRD(func(context.Context, schema.GroupVersionKind) (*un.Unstructured, error) {
						// For this test, we can return nil as it doesn't focus on validation
						return nil, errors.New("CRD not found")
					}).
					WithNoResourcesRequiringCRDs().
					WithSuccessfulDryRun().
					Build()
			},
			setupProcessor: func() dp.DiffProcessor {
				return tu.NewMockDiffProcessor().
					WithSuccessfulInitialize().
					WithPerformDiff(func(w io.Writer, _ context.Context, _ []*un.Unstructured) error {
						// Generate a mock diff for our test
						_, _ = fmt.Fprintf(w, `~ ComposedResource/test-xr-composed-resource
{
  "spec": {
    "coolParam": "test-value",
    "extraData": "extra-resource-data",
    "replicas": 3
  }
}`)
						return nil
					}).
					Build()
			},
			setupLoader: func() *itu.MockLoader {
				// Create a test XR content
				xrYAML := []byte(`
apiVersion: example.org/v1
kind: XExampleResource
metadata:
  name: test-xr
spec:
  coolParam: test-value
  replicas: 3
`)
				return &itu.MockLoader{
					Resources: []*un.Unstructured{
						func() *un.Unstructured {
							// Parse the YAML into an unstructured object
							obj := &un.Unstructured{}
							if err := yaml.Unmarshal(xrYAML, &obj.Object); err != nil {
								t.Fatalf("Failed to unmarshal test XR: %v", err)
							}
							return obj
						}(),
					},
				}
			},
			expectedOutput: "ComposedResource", // Should mention resource type
			notExpected:    nil,
			expectError:    false,
		},

		"ExtraResources_GetAllResourcesError": {
			setupClient: func() cc.ClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithSuccessfulCompositionMatch(testComposition).
					WithGetAllResourcesByLabels(func(context.Context, []schema.GroupVersionKind, []metav1.LabelSelector) ([]*un.Unstructured, error) {
						return nil, errors.New("error getting resources")
					}).
					WithGetFunctionsFromPipeline(func(*xpextv1.Composition) ([]pkgv1.Function, error) {
						return []pkgv1.Function{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "function-extra-resources",
								},
							},
						}, nil
					}).
					Build()
			},
			setupProcessor: func() dp.DiffProcessor {
				return tu.NewMockDiffProcessor().
					WithSuccessfulInitialize().
					WithPerformDiff(func(io.Writer, context.Context, []*un.Unstructured) error {
						return errors.New("processing error")
					}).
					Build()
			},
			setupLoader: func() *itu.MockLoader {
				// Create a test XR content
				xrYAML := []byte(`
apiVersion: example.org/v1
kind: XExampleResource
metadata:
  name: test-xr
spec:
  coolParam: test-value
`)
				return &itu.MockLoader{
					Resources: []*un.Unstructured{
						func() *un.Unstructured {
							// Parse the YAML into an unstructured object
							obj := &un.Unstructured{}
							if err := yaml.Unmarshal(xrYAML, &obj.Object); err != nil {
								t.Fatalf("Failed to unmarshal test XR: %v", err)
							}
							return obj
						}(),
					},
				}
			},
			expectedOutput: "",
			notExpected:    nil,
			expectError:    true,
			errorContains:  "processing error",
		},

		// ====== Tests for matching resources ======

		"MatchingResources_NoChanges": {
			setupClient: func() cc.ClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithSuccessfulCompositionMatch(testComposition).
					WithGetAllResourcesByLabels(func(context.Context, []schema.GroupVersionKind, []metav1.LabelSelector) ([]*un.Unstructured, error) {
						return []*un.Unstructured{testExtraResource}, nil
					}).
					WithGetFunctionsFromPipeline(func(*xpextv1.Composition) ([]pkgv1.Function, error) {
						return []pkgv1.Function{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "function-extra-resources",
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "function-patch-and-transform",
								},
							},
						}, nil
					}).
					WithGetXRDs(func(context.Context) ([]*un.Unstructured, error) {
						return []*un.Unstructured{
							{Object: xrdUnstructured},
						}, nil
					}).
					WithGetResource(func(_ context.Context, _ schema.GroupVersionKind, _, name string) (*un.Unstructured, error) {
						if name == "test-xr-composed-resource" {
							return matchingResource, nil
						}
						return nil, errors.Errorf("resource %q not found", name)
					}).
					WithSuccessfulDryRun().
					Build()
			},
			setupProcessor: func() dp.DiffProcessor {
				return tu.NewMockDiffProcessor().
					WithSuccessfulInitialize().
					WithPerformDiff(func(io.Writer, context.Context, []*un.Unstructured) error {
						// For matching resources, we don't produce any output
						return nil
					}).
					Build()
			},
			setupLoader: func() *itu.MockLoader {
				// Create a test XR content
				xrYAML := []byte(`
apiVersion: example.org/v1
kind: XExampleResource
metadata:
  name: test-xr
spec:
  coolParam: test-value
  replicas: 3
`)
				return &itu.MockLoader{
					Resources: []*un.Unstructured{
						func() *un.Unstructured {
							// Parse the YAML into an unstructured object
							obj := &un.Unstructured{}
							if err := yaml.Unmarshal(xrYAML, &obj.Object); err != nil {
								t.Fatalf("Failed to unmarshal test XR: %v", err)
							}
							return obj
						}(),
					},
				}
			},
			expectedOutput: "",
			notExpected:    []string{"ComposedResource", "test-xr-composed-resource"},
			expectError:    false,
		},

		"ResourceNotFound_ShownAsNew": {
			setupClient: func() cc.ClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithSuccessfulCompositionMatch(testComposition).
					WithGetAllResourcesByLabels(func(context.Context, []schema.GroupVersionKind, []metav1.LabelSelector) ([]*un.Unstructured, error) {
						return []*un.Unstructured{testExtraResource}, nil
					}).
					WithGetFunctionsFromPipeline(func(*xpextv1.Composition) ([]pkgv1.Function, error) {
						return []pkgv1.Function{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "function-extra-resources",
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "function-patch-and-transform",
								},
							},
						}, nil
					}).
					WithGetXRDs(func(context.Context) ([]*un.Unstructured, error) {
						return []*un.Unstructured{
							{Object: xrdUnstructured},
						}, nil
					}).
					WithGetResource(func(context.Context, schema.GroupVersionKind, string, string) (*un.Unstructured, error) {
						// Simulate resource not found
						return nil, errors.New("resource not found")
					}).
					WithSuccessfulDryRun().
					Build()
			},
			setupProcessor: func() dp.DiffProcessor {
				return tu.NewMockDiffProcessor().
					WithSuccessfulInitialize().
					WithPerformDiff(func(w io.Writer, _ context.Context, _ []*un.Unstructured) error {
						// Generate output for a new resource
						_, _ = fmt.Fprintf(w, `+++ ComposedResource/test-xr-composed-resource
{
  "spec": {
    "coolParam": "test-value",
    "extraData": "extra-resource-data",
    "replicas": 3
  }
}`)
						return nil
					}).
					Build()
			},
			setupLoader: func() *itu.MockLoader {
				// Create a test XR content
				xrYAML := []byte(`
apiVersion: example.org/v1
kind: XExampleResource
metadata:
  name: test-xr
spec:
  coolParam: test-value
  replicas: 3
`)
				return &itu.MockLoader{
					Resources: []*un.Unstructured{
						func() *un.Unstructured {
							obj := &un.Unstructured{}
							if err := yaml.Unmarshal(xrYAML, &obj.Object); err != nil {
								t.Fatalf("Failed to unmarshal test XR: %v", err)
							}
							return obj
						}(),
					},
				}
			},
			expectedOutput: "+++ ComposedResource/test-xr-composed-resource", // Should show as new resource
			expectError:    false,
		},

		// ====== General error conditions ======

		"ClientInitializationError": {
			setupClient: func() cc.ClusterClient {
				return tu.NewMockClusterClient().
					WithFailedInitialize("client initialization error").
					Build()
			},
			setupProcessor: func() dp.DiffProcessor {
				return tu.NewMockDiffProcessor().
					WithSuccessfulInitialize().
					Build()
			},
			setupLoader: func() *itu.MockLoader {
				return &itu.MockLoader{
					Resources: []*un.Unstructured{
						tu.NewResource("example.org/v1", "XExampleResource", "test-xr").Build(),
					},
				}
			},
			expectError:   true,
			errorContains: "cannot initialize client",
		},

		"ProcessorInitializationError": {
			setupClient: func() cc.ClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					Build()
			},
			setupProcessor: func() dp.DiffProcessor {
				return tu.NewMockDiffProcessor().
					WithFailedInitialize("processor initialization error").
					Build()
			},
			setupLoader: func() *itu.MockLoader {
				return &itu.MockLoader{
					Resources: []*un.Unstructured{
						tu.NewResource("example.org/v1", "XExampleResource", "test-xr").Build(),
					},
				}
			},
			expectError:   true,
			errorContains: "cannot initialize diff processor",
		},

		"LoaderError": {
			setupClient: func() cc.ClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					Build()
			},
			setupProcessor: func() dp.DiffProcessor {
				return tu.NewMockDiffProcessor().
					WithSuccessfulInitialize().
					Build()
			},
			setupLoader: func() *itu.MockLoader {
				return &itu.MockLoader{
					Resources: nil,
					Err:       errors.New("loader error"),
				}
			},
			expectError:   true,
			errorContains: "cannot load resources",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the mocks based on the test case
			mockClient := tt.setupClient()
			mockProcessor := tt.setupProcessor()
			mockLoader := tt.setupLoader()

			// Create a buffer to capture output
			var buf bytes.Buffer

			// Create our command
			cmd := &Cmd{
				Namespace: "default",
				Timeout:   time.Second * 30,
			}

			// Create a Kong context
			parser, err := kong.New(&struct{}{})
			if err != nil {
				t.Fatalf("Failed to create Kong parser: %v", err)
			}
			kongCtx, err := parser.Parse([]string{})
			if err != nil {
				t.Fatalf("Failed to parse Kong context: %v", err)
			}
			kongCtx.Stdout = &buf

			// Create a logger
			logger := tu.TestLogger(t, false)

			// Execute the test
			err = cmd.Run(kongCtx, logger, mockClient, mockProcessor, mockLoader)

			// Check for expected errors
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorContains, err)
				}
				return
			}

			// Check for unexpected errors
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
				return
			}

			// Get the captured output
			capturedOutput := buf.String()

			// Check expected output
			if tt.expectedOutput != "" {
				if !strings.Contains(capturedOutput, tt.expectedOutput) {
					t.Errorf("Expected output to contain '%s', but it didn't\nOutput: %s", tt.expectedOutput, capturedOutput)
				}
			}

			// Check for text that should NOT be present
			if tt.notExpected != nil {
				for _, unexpected := range tt.notExpected {
					if strings.Contains(capturedOutput, unexpected) {
						t.Errorf("Output should not contain '%s', but it did\nOutput: %s", unexpected, capturedOutput)
					}
				}
			}
		})
	}
}
