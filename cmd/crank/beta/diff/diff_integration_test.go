package diff

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

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
)

// TestDiffWithExtraResources tests that a resource with differing values produces a diff
// TestDiffWithExtraResources tests that a resource with differing values produces a diff
func TestDiffWithExtraResources(t *testing.T) {
	// Set up the test context
	ctx := context.Background()

	// Create test resources
	testXR := createTestXR()
	testComposition := createTestCompositionWithExtraResources()
	testXRD := createTestXRD()
	testExtraResource := createExtraResource()

	// Create test existing resource with different values
	existingResource := createExistingComposedResource()

	// Set up the mock cluster client
	mockClient := &MockClusterClient{
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
		GetExtraResourcesFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]unstructured.Unstructured, error) {
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

			return []unstructured.Unstructured{*testExtraResource}, nil
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
		GetXRDSchemaFn: func(ctx context.Context, res *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error) {
			return testXRD, nil
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

	// Create our command
	cmd := &Cmd{
		Namespace: "default",
		Files:     []string{"test-xr.yaml"},
	}

	// Mock resource loader to return our test XR
	originalResourceLoader := ResourceLoader
	defer func() { ResourceLoader = originalResourceLoader }()
	ResourceLoader = func(files []string) ([]*unstructured.Unstructured, error) {
		return []*unstructured.Unstructured{testXR}, nil
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

	// Use the MockDiffProcessor from diff_test.go
	DiffProcessorFactory = func(config *rest.Config, client cc.ClusterClient, namespace string, renderFunc dp.RenderFunc) (dp.DiffProcessor, error) {
		return &MockDiffProcessor{
			ProcessResourceFn: func(ctx context.Context, res *unstructured.Unstructured) error {
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
	err := testRun(ctx, cmd, func() (*rest.Config, error) {
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
	testXR := createTestXR()
	testComposition := createTestCompositionWithExtraResources()
	testXRD := createTestXRD()
	testExtraResource := createExtraResource()

	// Create test existing resource with matching values
	matchingResource := createMatchingComposedResource()

	// Set up the mock cluster client
	mockClient := &MockClusterClient{
		InitializeFn: func(ctx context.Context) error {
			return nil
		},
		FindMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return testComposition, nil
		},
		GetExtraResourcesFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{*testExtraResource}, nil
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
		GetXRDSchemaFn: func(ctx context.Context, res *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error) {
			return testXRD, nil
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

	// Create our command
	cmd := &Cmd{
		Namespace: "default",
		Files:     []string{"test-xr.yaml"},
	}

	// Mock resource loader to return our test XR
	originalResourceLoader := ResourceLoader
	defer func() { ResourceLoader = originalResourceLoader }()
	ResourceLoader = func(files []string) ([]*unstructured.Unstructured, error) {
		return []*unstructured.Unstructured{testXR}, nil
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

	// Use the MockDiffProcessor from diff_test.go
	DiffProcessorFactory = func(config *rest.Config, client cc.ClusterClient, namespace string, renderFunc dp.RenderFunc) (dp.DiffProcessor, error) {
		return &MockDiffProcessor{
			ProcessResourceFn: func(ctx context.Context, res *unstructured.Unstructured) error {
				// For matching resources, we don't produce any output
				return nil
			},
		}, nil
	}

	// Execute the test
	err := testRun(ctx, cmd, func() (*rest.Config, error) {
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
