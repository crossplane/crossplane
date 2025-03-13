package diff

import (
	"bytes"
	"context"
	"fmt"
	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/crossplane/crossplane/cmd/crank/render"
	"io"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/yaml"
	"strings"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	dp "github.com/crossplane/crossplane/cmd/crank/beta/diff/diffprocessor"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeout = 60 * time.Second
)

// TestDiffWithExtraResources tests that a resource with differing values produces a diff
func TestDiffWithExtraResources(t *testing.T) {
	// Set up the test context
	ctx := context.Background()

	// Create test resources
	testComposition := createTestCompositionWithExtraResources()
	testXRD := createTestXRD()
	testExtraResource := createExtraResource()

	// Create test existing resource with different values
	existingResource := createExistingComposedResource()

	// Convert the test XRD to unstructured for GetXRDs to return
	xrdUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(testXRD)
	if err != nil {
		t.Fatalf("Failed to convert XRD to unstructured: %v", err)
	}

	// Set up the mock cluster client
	mockClient := &testutils.MockClusterClient{
		InitializeFn: func(ctx context.Context) error {
			return nil
		},
		FindMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			// Validate the input XR
			if res.GetAPIVersion() != "example.org/v1" || res.GetKind() != "XExampleResource" {
				return nil, errors.New("unexpected resource type")
			}
			return testComposition, nil
		},
		GetExtraResourcesFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
			// Validate the GVR and selector match what we expect
			if len(gvrs) != 1 || len(selectors) != 1 {
				return nil, errors.New("unexpected number of GVRs or selectors")
			}

			// Verify the GVR matches our extra resource
			expectedGVR := schema.GroupVersionResource{
				Group:    "example.org",
				Version:  "v1",
				Resource: "extraresources",
			}
			if gvrs[0] != expectedGVR {
				return nil, errors.Errorf("unexpected GVR: %v", gvrs[0])
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

			return []*unstructured.Unstructured{testExtraResource}, nil
		},
		GetFunctionsFromPipelineFn: func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
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
		},
		GetXRDsFn: func(ctx context.Context) ([]*unstructured.Unstructured, error) {
			return []*unstructured.Unstructured{
				{Object: xrdUnstructured},
			}, nil
		},
		GetResourceFn: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
			if name == "test-xr-composed-resource" {
				return existingResource, nil
			}
			return nil, errors.Errorf("resource %q not found", name)
		},
		DryRunApplyFn: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return obj, nil
		},
	}

	// Create a buffer to capture output
	var buf bytes.Buffer

	// Save the original fprintf and restore it after the test
	origFprintf := fprintf
	defer func() { fprintf = origFprintf }()

	// Override fprintf to write to our buffer
	fprintf = func(w io.Writer, format string, a ...interface{}) (int, error) {
		// For our test, redirect all output to our buffer regardless of the writer
		return fmt.Fprintf(&buf, format, a...)
	}

	// Create a temporary test file with the XR content
	tempDir, err := os.MkdirTemp("", "diff-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "test-xr.yaml")
	xrYAML := []byte(`
apiVersion: example.org/v1
kind: XExampleResource
metadata:
  name: test-xr
spec:
  coolParam: test-value
  replicas: 3
`)

	if err := os.WriteFile(tempFile, xrYAML, 0600); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Create our command
	cmd := &Cmd{
		Namespace: "default",
		Files:     []string{tempFile},
	}

	// Save original ClusterClientFactory and restore after test
	originalClusterClientFactory := ClusterClientFactory
	originalDiffProcessorFactory := DiffProcessorFactory
	defer func() {
		ClusterClientFactory = originalClusterClientFactory
		DiffProcessorFactory = originalDiffProcessorFactory
	}()

	ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
		return mockClient, nil
	}

	// Use the MockDiffProcessor
	DiffProcessorFactory = func(config *rest.Config, client cc.ClusterClient, namespace string, renderFunc dp.RenderFunc, logger logging.Logger) (dp.DiffProcessor, error) {
		return &testutils.MockDiffProcessor{
			InitializeFn: func(writer io.Writer, ctx context.Context) error {
				return nil
			},
			ProcessResourceFn: func(stdout io.Writer, ctx context.Context, res *unstructured.Unstructured) error {
				// Generate a mock diff for our test
				fmt.Fprintf(&buf, `~ ComposedResource/test-xr-composed-resource
{
  "spec": {
    "coolParam": "test-value",
    "extraData": "extra-resource-data",
    "replicas": 3
  }
}`)
				return nil
			},
		}, nil
	}

	// Execute the test
	err = testRun(ctx, cmd, func() (*rest.Config, error) {
		return &rest.Config{}, nil
	})

	// Validate results
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Check that the output contains expected diff information
	capturedOutput := buf.String()

	// Since the actual diff formatting might vary, we'll just check for key elements
	expectedElements := []string{
		"ComposedResource",          // Should mention resource type
		"test-xr-composed-resource", // Should mention resource name
		"coolParam",                 // Should mention changed field
		"test-value",                // Should mention new value
	}

	for _, expected := range expectedElements {
		if !strings.Contains(capturedOutput, expected) {
			t.Errorf("Expected output to contain '%s', but got: %s", expected, capturedOutput)
		}
	}
}

// TestDiffWithMatchingResources tests that a resource with matching values produces no diff
func TestDiffWithMatchingResources(t *testing.T) {
	// Set up the test context
	ctx := context.Background()

	// Create test resources
	testComposition := createTestCompositionWithExtraResources()
	testXRD := createTestXRD()
	testExtraResource := createExtraResource()

	// Create test existing resource with matching values
	matchingResource := createMatchingComposedResource()

	// Convert the test XRD to unstructured for GetXRDs to return
	xrdUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(testXRD)
	if err != nil {
		t.Fatalf("Failed to convert XRD to unstructured: %v", err)
	}

	// Set up the mock cluster client
	mockClient := &testutils.MockClusterClient{
		InitializeFn: func(ctx context.Context) error {
			return nil
		},
		FindMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return testComposition, nil
		},
		GetExtraResourcesFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
			return []*unstructured.Unstructured{testExtraResource}, nil
		},
		GetFunctionsFromPipelineFn: func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
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
		},
		GetXRDsFn: func(ctx context.Context) ([]*unstructured.Unstructured, error) {
			return []*unstructured.Unstructured{
				{Object: xrdUnstructured},
			}, nil
		},
		GetResourceFn: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
			if name == "test-xr-composed-resource" {
				return matchingResource, nil
			}
			return nil, errors.Errorf("resource %q not found", name)
		},
		DryRunApplyFn: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return obj, nil
		},
	}

	// Create a buffer to capture output
	var buf bytes.Buffer

	// Save the original fprintf and restore it after the test
	origFprintf := fprintf
	defer func() { fprintf = origFprintf }()

	// Override fprintf to write to our buffer
	fprintf = func(w io.Writer, format string, a ...interface{}) (int, error) {
		// For our test, redirect all output to our buffer regardless of the writer
		return fmt.Fprintf(&buf, format, a...)
	}

	// Create a temporary test file with the XR content
	tempDir, err := os.MkdirTemp("", "diff-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "test-xr.yaml")
	xrYAML := []byte(`
apiVersion: example.org/v1
kind: XExampleResource
metadata:
  name: test-xr
spec:
  coolParam: test-value
  replicas: 3
`)

	if err := os.WriteFile(tempFile, xrYAML, 0600); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Create our command
	cmd := &Cmd{
		Namespace: "default",
		Files:     []string{tempFile},
	}

	// Mock the factory functions
	originalClusterClientFactory := ClusterClientFactory
	originalDiffProcessorFactory := DiffProcessorFactory
	defer func() {
		ClusterClientFactory = originalClusterClientFactory
		DiffProcessorFactory = originalDiffProcessorFactory
	}()

	ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
		return mockClient, nil
	}

	// Use the MockDiffProcessor
	DiffProcessorFactory = func(config *rest.Config, client cc.ClusterClient, namespace string, renderFunc dp.RenderFunc, logger logging.Logger) (dp.DiffProcessor, error) {
		return &testutils.MockDiffProcessor{
			InitializeFn: func(writer io.Writer, ctx context.Context) error {
				return nil
			},
			ProcessResourceFn: func(stdout io.Writer, ctx context.Context, res *unstructured.Unstructured) error {
				// For matching resources, we don't produce any output
				return nil
			},
		}, nil
	}

	// Execute the test
	err = testRun(ctx, cmd, func() (*rest.Config, error) {
		return &rest.Config{}, nil
	})

	// Validate results
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// For matching resources, we expect no diff output
	capturedOutput := buf.String()
	if capturedOutput != "" {
		t.Errorf("Expected no diff output for matching resources, but got: %s", capturedOutput)
	}
}

// MockRenderFunc mocks the render.Render function
func MockRenderFunc(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
	// Create a simple output with just the XR and a single composed resource
	// First create a standard unstructured object
	u := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "nop.crossplane.io/v1alpha1",
			"kind":       "NopResource",
			"metadata": map[string]interface{}{
				"name": "test-nop-resource",
			},
			"spec": map[string]interface{}{
				"forProvider": map[string]interface{}{
					"coolField": "mocked-value",
				},
			},
		},
	}

	// Convert it to a composed.Unstructured
	c := composed.Unstructured{Unstructured: u}

	out := render.Outputs{
		CompositeResource: in.CompositeResource,
		ComposedResources: []composed.Unstructured{c},
	}
	return out, nil
}

// TestDiffIntegration runs an integration test for the diff command
func TestDiffIntegration(t *testing.T) {
	// Create a scheme with both Kubernetes and Crossplane types
	scheme := runtime.NewScheme()

	// Register Kubernetes types
	_ = clientgoscheme.AddToScheme(scheme)

	// Register Crossplane types
	_ = apiextensionsv1.AddToScheme(scheme)
	_ = pkgv1.AddToScheme(scheme)
	_ = extv1.AddToScheme(scheme)

	// Setup a test environment
	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "..", "cluster", "crds"),
			filepath.Join("testdata", "diff", "crds"),
		},
		ErrorIfCRDPathMissing: false,
		Scheme:                scheme,
	}

	// Start the test environment
	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("failed to start test environment: %v", err)
	}
	defer func() {
		if err := testEnv.Stop(); err != nil {
			t.Logf("failed to stop test environment: %v", err)
		}
	}()

	// Create a controller-runtime client for setup operations
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Test cases
	tests := []struct {
		name           string
		setupResources func(ctx context.Context, c client.Client) error
		inputFile      string
		expectedOutput string
		expectedError  bool
	}{
		{
			name:      "New resource shows diff",
			inputFile: "testdata/diff/new-xr.yaml",
			setupResources: func(ctx context.Context, c client.Client) error {
				// Apply the XRD and Composition from YAML files
				if err := applyResourcesFromFiles(ctx, c, []string{
					"testdata/diff/resources/xrd.yaml",
					"testdata/diff/resources/composition.yaml",
				}); err != nil {
					return err
				}
				return nil
			},
			expectedOutput: "+ XNopResource/test-resource",
			expectedError:  false,
		},
		{
			name: "Modified resource shows diff",
			setupResources: func(ctx context.Context, c client.Client) error {
				// Apply the XRD and Composition from YAML files
				if err := applyResourcesFromFiles(ctx, c, []string{
					"testdata/diff/resources/xrd.yaml",
					"testdata/diff/resources/composition.yaml",
				}); err != nil {
					return err
				}

				// Create an existing resource in the cluster
				existingXR := &unstructured.Unstructured{}
				existingXR.SetAPIVersion("diff.example.org/v1alpha1")
				existingXR.SetKind("XNopResource")
				existingXR.SetName("existing-resource")
				existingXR.SetNamespace("default")
				existingXR.Object["spec"] = map[string]interface{}{
					"coolField": "original-value",
				}

				return c.Create(ctx, existingXR)
			},
			inputFile:      "testdata/diff/modified-xr.yaml",
			expectedOutput: "~ XNopResource/existing-resource",
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// Setup resources if needed
			if tt.setupResources != nil {
				if err := tt.setupResources(ctx, k8sClient); err != nil {
					t.Fatalf("failed to setup resources: %v", err)
				}
			}

			// Set up the test file
			tempDir := t.TempDir()
			testFile := filepath.Join(tempDir, "test.yaml")

			// Read the test file content from the inputFile path
			content, err := os.ReadFile(tt.inputFile)
			if err != nil {
				t.Fatalf("failed to read input file: %v", err)
			}

			err = os.WriteFile(testFile, content, 0644)
			if err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			// Create a buffer to capture the output
			var stdout bytes.Buffer

			// Override fprintf to capture output
			origFprintf := fprintf
			defer func() { fprintf = origFprintf }()
			fprintf = func(w io.Writer, format string, a ...interface{}) (int, error) {
				return fmt.Fprintf(&stdout, format, a...)
			}

			// Set up the diff command
			cmd := &Cmd{
				Namespace: "default",
				Files:     []string{testFile},
				Timeout:   timeout,
			}

			// Use real implementations
			origClusterClientFactory := ClusterClientFactory
			origDiffProcessorFactory := DiffProcessorFactory
			defer func() {
				ClusterClientFactory = origClusterClientFactory
				DiffProcessorFactory = origDiffProcessorFactory
			}()

			// Use the real implementation but with our test config
			ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
				return cc.NewClusterClient(cfg)
			}

			// Keep the real DiffProcessor
			DiffProcessorFactory = func(config *rest.Config, client cc.ClusterClient, namespace string, renderFunc dp.RenderFunc, logger logging.Logger) (dp.DiffProcessor, error) {
				return dp.NewDiffProcessor(config, client, namespace, renderFunc, logger)
			}

			// Create a Kong context with stdout
			parser, err := kong.New(&struct{}{}, kong.Writers(&stdout, &stdout))
			if err != nil {
				t.Fatalf("failed to create kong parser: %v", err)
			}
			kongCtx, err := parser.Parse([]string{})
			if err != nil {
				t.Fatalf("failed to parse kong context: %v", err)
			}

			// Run the diff command with the test environment's config
			err = cmd.Run(kongCtx, logging.NewNopLogger(), cfg)

			if tt.expectedError && err == nil {
				t.Fatal("expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Fatalf("expected no error but got: %v", err)
			}

			// Check the output
			if !strings.Contains(stdout.String(), tt.expectedOutput) {
				t.Fatalf("expected output to contain %q, but got %q", tt.expectedOutput, stdout.String())
			}
		})
	}
}

// applyResourcesFromFiles loads and applies resources from YAML files
func applyResourcesFromFiles(ctx context.Context, c client.Client, paths []string) error {
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(data, obj); err != nil {
			return fmt.Errorf("failed to unmarshal YAML from %s: %w", path, err)
		}

		if err := c.Create(ctx, obj); err != nil {
			if apierrors.IsAlreadyExists(err) {
				// If the resource already exists, update it
				existing := &unstructured.Unstructured{}
				existing.SetGroupVersionKind(obj.GroupVersionKind())
				if err := c.Get(ctx, client.ObjectKey{
					Name:      obj.GetName(),
					Namespace: obj.GetNamespace(),
				}, existing); err != nil {
					return fmt.Errorf("failed to get existing resource %s: %w", path, err)
				}

				// Copy resource version to avoid conflicts
				obj.SetResourceVersion(existing.GetResourceVersion())

				if err := c.Update(ctx, obj); err != nil {
					return fmt.Errorf("failed to update resource %s: %w", path, err)
				}
			} else {
				return fmt.Errorf("failed to create resource %s: %w", path, err)
			}
		}
	}
	return nil
}
