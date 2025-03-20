package diff

import (
	"bytes"
	"context"
	"fmt"
	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/go-logr/logr/testr"
	"io"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"strconv"
	"strings"
	"testing"
	"time"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeout = 60 * time.Second
)

type ownerRelationship struct {
	ownerFile  string
	ownedFiles []string
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

	// Test cases
	tests := []struct {
		name                    string
		setupFiles              []string
		setupFilesWithOwnerRefs []ownerRelationship
		inputFile               string
		expectedOutput          string
		expectedError           bool
		noColor                 bool
	}{
		{
			name:      "New resource shows color diff",
			inputFile: "testdata/diff/new-xr.yaml",
			setupFiles: []string{
				"testdata/diff/resources/xrd.yaml",
				"testdata/diff/resources/composition.yaml",
				"testdata/diff/resources/functions.yaml",
			},
			expectedOutput: strings.Join([]string{
				`+++ XDownstreamResource/test-resource
`, tu.Green(`+ apiVersion: nop.example.org/v1alpha1
+ kind: XDownstreamResource
+ metadata:
+   annotations:
+     crossplane.io/composition-resource-name: nop-resource
+   generateName: test-resource-
+   labels:
+     crossplane.io/composite: test-resource
+   name: test-resource
+ spec:
+   forProvider:
+     configData: new-value
`), `
---
+++ XNopResource/test-resource
`, tu.Green(`+ apiVersion: diff.example.org/v1alpha1
+ kind: XNopResource
+ metadata:
+   name: test-resource
+ spec:
+   coolField: new-value
`), `
---
`,
			}, ""),
			expectedError: false,
		},
		{
			name: "Modified resource shows color diff",
			setupFiles: []string{
				"testdata/diff/resources/xrd.yaml",
				"testdata/diff/resources/composition.yaml",
				"testdata/diff/resources/functions.yaml",
				// put an existing XR in the cluster to diff against
				"testdata/diff/resources/existing-downstream-resource.yaml",
				"testdata/diff/resources/existing-xr.yaml",
			},
			inputFile: "testdata/diff/modified-xr.yaml",
			expectedOutput: `
~~~ XDownstreamResource/test-resource
  apiVersion: nop.example.org/v1alpha1
  kind: XDownstreamResource
  metadata:
    annotations:
      crossplane.io/composition-resource-name: nop-resource
    generateName: test-resource-
    labels:
      crossplane.io/composite: test-resource
    name: test-resource
  spec:
    forProvider:
` + tu.Red("-     configData: existing-value") + `
` + tu.Green("+     configData: modified-value") + `

---
~~~ XNopResource/test-resource
  apiVersion: diff.example.org/v1alpha1
  kind: XNopResource
  metadata:
    name: test-resource
  spec:
` + tu.Red("-   coolField: existing-value") + `
` + tu.Green("+   coolField: modified-value") + `

---
`,
			expectedError: false,
		},
		{
			name: "Modified XR that creates new downstream resource shows color diff",
			setupFiles: []string{
				"testdata/diff/resources/xrd.yaml",
				"testdata/diff/resources/composition.yaml",
				"testdata/diff/resources/functions.yaml",
				"testdata/diff/resources/existing-xr.yaml",
			},
			inputFile: "testdata/diff/modified-xr.yaml",
			expectedOutput: `
+++ XDownstreamResource/test-resource
` + tu.Green(`+ apiVersion: nop.example.org/v1alpha1
+ kind: XDownstreamResource
+ metadata:
+   annotations:
+     crossplane.io/composition-resource-name: nop-resource
+   generateName: test-resource-
+   labels:
+     crossplane.io/composite: test-resource
+   name: test-resource
+ spec:
+   forProvider:
+     configData: modified-value
`) + `
---
~~~ XNopResource/test-resource
  apiVersion: diff.example.org/v1alpha1
  kind: XNopResource
  metadata:
    name: test-resource
  spec:
` + tu.Red("-   coolField: existing-value") + `
` + tu.Green("+   coolField: modified-value") + `

---
`,
			expectedError: false,
		},
		{
			name: "EnvironmentConfig incorporation in diff",
			setupFiles: []string{
				"testdata/diff/resources/xdownstreamenvresource-xrd.yaml",
				"testdata/diff/resources/env-xrd.yaml",
				"testdata/diff/resources/env-composition.yaml",
				"testdata/diff/resources/functions.yaml",
				"testdata/diff/resources/environment-config.yaml",
				"testdata/diff/resources/existing-env-downstream-resource.yaml",
				"testdata/diff/resources/existing-env-xr.yaml",
			},
			inputFile: "testdata/diff/modified-env-xr.yaml",
			expectedOutput: `
~~~ XDownstreamEnvResource/test-env-resource
  apiVersion: nop.example.org/v1alpha1
  kind: XDownstreamEnvResource
  metadata:
    annotations:
      crossplane.io/composition-resource-name: env-resource
    generateName: test-env-resource-
    labels:
      crossplane.io/composite: test-env-resource
    name: test-env-resource
  spec:
    forProvider:
-     configData: existing-config-value
+     configData: modified-config-value
      environment: staging
      region: us-west-2
      serviceLevel: premium

---
~~~ XEnvResource/test-env-resource
  apiVersion: diff.example.org/v1alpha1
  kind: XEnvResource
  metadata:
    name: test-env-resource
  spec:
-   configKey: existing-config-value
+   configKey: modified-config-value

---
`,
			expectedError: false,
			noColor:       true,
		},
		{
			// this test does a weird thing where it changes the XR but all the downstream changes come from external
			// resources, including a field path from the XR itself.
			name: "Diff with external resource dependencies via fn-external-resources",
			setupFiles: []string{
				"testdata/diff/resources/xrd.yaml",
				"testdata/diff/resources/functions.yaml",
				"testdata/diff/resources/external-resource-configmap.yaml",
				"testdata/diff/resources/external-res-fn-composition.yaml",
				"testdata/diff/resources/existing-xr-with-external-dep.yaml",
				"testdata/diff/resources/existing-downstream-with-external-dep.yaml",
				"testdata/diff/resources/external-named-clusterrole.yaml",
			},
			inputFile: "testdata/diff/modified-xr-with-external-dep.yaml",
			expectedOutput: `
~~~ XDownstreamResource/test-resource
  apiVersion: nop.example.org/v1alpha1
  kind: XDownstreamResource
  metadata:
    annotations:
      crossplane.io/composition-resource-name: nop-resource
    generateName: test-resource-
    labels:
      crossplane.io/composite: test-resource
    name: test-resource
  spec:
    forProvider:
-     configData: existing-value
-     roleName: old-role-name
+     configData: testing-external-resource-data
+     roleName: external-named-clusterrole

---
~~~ XNopResource/test-resource
  apiVersion: diff.example.org/v1alpha1
  kind: XNopResource
  metadata:
    name: test-resource
  spec:
-   coolField: existing-value
-   environment: staging
+   coolField: modified-with-external-dep
+   environment: testing

---
`,
			expectedError: false,
			noColor:       true,
		},
		{
			// this one is ironically more complicated since it has to invoke render first to find the resources it needs
			// to pull in, then pull them in, then render again with them.
			name: "Diff with templated ExtraResources embedded in go-templating function",
			setupFiles: []string{
				"testdata/diff/resources/xrd.yaml",
				"testdata/diff/resources/functions.yaml",
				"testdata/diff/resources/external-resource-configmap.yaml",
				"testdata/diff/resources/external-res-gotpl-composition.yaml",
				"testdata/diff/resources/existing-xr-with-external-dep.yaml",
				"testdata/diff/resources/existing-downstream-with-external-dep.yaml",
			},
			inputFile: "testdata/diff/modified-xr-with-external-dep.yaml",
			expectedOutput: `
~~~ XDownstreamResource/test-resource
  apiVersion: nop.example.org/v1alpha1
  kind: XDownstreamResource
  metadata:
    annotations:
      crossplane.io/composition-resource-name: nop-resource
    generateName: test-resource-
    labels:
      crossplane.io/composite: test-resource
    name: test-resource
  spec:
    forProvider:
-     configData: existing-value
-     roleName: old-role-name
+     configData: modified-with-external-dep
+     roleName: templated-external-resource-testing

---
~~~ XNopResource/test-resource
  apiVersion: diff.example.org/v1alpha1
  kind: XNopResource
  metadata:
    name: test-resource
  spec:
-   coolField: existing-value
-   environment: staging
+   coolField: modified-with-external-dep
+   environment: testing

---
`,
			expectedError: false,
			noColor:       true,
		},
		{
			name: "Resource removal detection",
			setupFilesWithOwnerRefs: []ownerRelationship{
				{
					ownerFile: "testdata/diff/resources/existing-xr.yaml",
					ownedFiles: []string{
						"testdata/diff/resources/removal-test-downstream-resource1.yaml", // Will be kept
						"testdata/diff/resources/removal-test-downstream-resource2.yaml", // Will be removed
					},
				},
			},
			setupFiles: []string{
				"testdata/diff/resources/xrd.yaml",
				"testdata/diff/resources/removal-test-composition.yaml",
				"testdata/diff/resources/functions.yaml",
			},
			inputFile: "testdata/diff/modified-xr.yaml",
			expectedOutput: `
~~~ XDownstreamResource/resource-to-be-kept
  apiVersion: nop.example.org/v1alpha1
  kind: XDownstreamResource
  metadata:
    annotations:
      crossplane.io/composition-resource-name: resource1
    generateName: test-resource-
    labels:
      crossplane.io/composite: test-resource
    name: resource-to-be-kept
  spec:
    forProvider:
-     configData: existing-value
+     configData: modified-value

---
--- XDownstreamResource/resource-to-be-removed
- apiVersion: nop.example.org/v1alpha1
- kind: XDownstreamResource
- metadata:
-   annotations:
-     crossplane.io/composition-resource-name: resource2
-   generateName: test-resource-
-   labels:
-     crossplane.io/composite: test-resource
-   name: resource-to-be-removed
- spec:
-   forProvider:
-     configData: existing-value

---
~~~ XNopResource/test-resource
  apiVersion: diff.example.org/v1alpha1
  kind: XNopResource
  metadata:
    name: test-resource
  spec:
-   coolField: existing-value
+   coolField: modified-value

---
`,
			expectedError: false,
			noColor:       true,
		},
	}

	tu.SetupKubeTestLogger(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup a brand new test environment for each test case
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

			// Ensure we clean up at the end of the test
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

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// Apply the setup resources
			if err := applyResourcesFromFiles(ctx, k8sClient, tt.setupFiles); err != nil {
				t.Fatalf("failed to setup resources: %v", err)
			}

			// Apply resources with owner references
			if len(tt.setupFilesWithOwnerRefs) > 0 {
				if err := applyResourcesWithOwnership(ctx, k8sClient, tt.setupFilesWithOwnerRefs); err != nil {
					t.Fatalf("failed to setup owner references: %v", err)
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
				NoColor:   tt.noColor,
			}

			// Use real implementations
			origClusterClientFactory := ClusterClientFactory
			origDiffProcessorFactory := DiffProcessorFactory
			defer func() {
				ClusterClientFactory = origClusterClientFactory
				DiffProcessorFactory = origDiffProcessorFactory
			}()

			// TODO: This seems a bit redundant with the Kong binding?
			// Use the real implementation but with our test config
			ClusterClientFactory = func(config *rest.Config, opts ...cc.ClusterClientOption) (cc.ClusterClient, error) {
				return cc.NewClusterClient(cfg, opts...)
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
			err = cmd.Run(kongCtx, logging.NewLogrLogger(testr.New(t)), cfg)

			if tt.expectedError && err == nil {
				t.Fatal("expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Fatalf("expected no error but got: %v", err)
			}

			// Check the output
			outputStr := stdout.String()
			// Using TrimSpace because the output might have trailing newlines
			if !strings.Contains(strings.TrimSpace(outputStr), strings.TrimSpace(tt.expectedOutput)) {
				// Strings aren't equal, *including* ansi.  but we can compare ignoring ansi to determine what output to
				// show for the failure.  if the difference is only in color codes, we'll show escaped ansi codes.
				out := outputStr
				expect := tt.expectedOutput
				if tu.CompareIgnoringAnsi(strings.TrimSpace(outputStr), strings.TrimSpace(tt.expectedOutput)) {
					out = strconv.QuoteToASCII(outputStr)
					expect = strconv.QuoteToASCII(tt.expectedOutput)
				}
				t.Fatalf("expected output to contain:\n%s\n\nbut got:\n%s", expect, out)
			}
		})
	}
}

// applyResourcesFromFiles loads and applies resources from YAML files
func applyResourcesFromFiles(ctx context.Context, c client.Client, paths []string) error {
	for _, path := range paths {
		if err := applyAllResourcesFromFile(ctx, c, path, nil); err != nil {
			return fmt.Errorf("failed to apply resources from %s: %w", path, err)
		}
	}
	return nil
}

// applyResourcesWithOwnership loads and applies resources, handling owner relationships
func applyResourcesWithOwnership(ctx context.Context, c client.Client, relationships []ownerRelationship) error {
	// Process each owner-owned relationship
	for _, rel := range relationships {
		// First, apply the owner file and get the owner resource
		// Note: We assume owner file has only a single document as specified in requirements
		owner, err := applySingleResourceFromFile(ctx, c, rel.ownerFile)
		if err != nil {
			return fmt.Errorf("failed to apply owner resource %s: %w", rel.ownerFile, err)
		}

		// Now apply all owned resources with the owner reference
		for _, ownedFile := range rel.ownedFiles {
			if err := applyAllResourcesFromFile(ctx, c, ownedFile, owner); err != nil {
				return fmt.Errorf("failed to apply owned resource %s: %w", ownedFile, err)
			}
		}
	}
	return nil
}

// applySingleResourceFromFile loads and applies a single resource from a file
// This function assumes the file contains only a single document
func applySingleResourceFromFile(ctx context.Context, c client.Client, path string) (*unstructured.Unstructured, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	// Parse the resource (assuming single document)
	resource := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(data, resource); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML from %s: %w", path, err)
	}

	// Skip empty documents
	if len(resource.Object) == 0 {
		return nil, fmt.Errorf("file %s contains empty document", path)
	}

	// Create or update the resource
	if err := createOrUpdateResource(ctx, c, resource); err != nil {
		return nil, err
	}

	return resource, nil
}

// applyAllResourcesFromFile loads and applies resources from a file
// If owner is provided, adds owner reference to all resources
func applyAllResourcesFromFile(ctx context.Context, c client.Client, path string, owner *unstructured.Unstructured) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	// Use a YAML decoder to handle multiple documents
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var resources []*unstructured.Unstructured

	for {
		resource := &unstructured.Unstructured{}
		err := decoder.Decode(resource)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode YAML document from %s: %w", path, err)
		}

		// Skip empty documents
		if len(resource.Object) == 0 {
			continue
		}

		// Add owner reference if owner is provided
		if owner != nil {
			// Set the child's owner reference pointing to the parent
			setOwnerReference(resource, owner)
		}

		// Add to our list of resources
		resources = append(resources, resource)
	}

	// Create/update all resources first
	for _, resource := range resources {
		if err := createOrUpdateResource(ctx, c, resource); err != nil {
			return err
		}
	}

	// Update the owner with references to all resources
	if owner != nil && len(resources) > 0 {
		// Make sure we have the latest version of the owner from the server
		existingOwner := &unstructured.Unstructured{}
		existingOwner.SetGroupVersionKind(owner.GroupVersionKind())

		err := c.Get(ctx, client.ObjectKey{
			Name:      owner.GetName(),
			Namespace: owner.GetNamespace(),
		}, existingOwner)

		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to get existing owner: %w", err)
			}

			// Owner doesn't exist yet, create it first
			if err := c.Create(ctx, owner); err != nil {
				return fmt.Errorf("failed to create owner: %w", err)
			}
		} else {
			// Owner exists, use its resourceVersion
			owner.SetResourceVersion(existingOwner.GetResourceVersion())
			owner.SetUID(existingOwner.GetUID())
		}

		// Add references to all resources
		for _, resource := range resources {
			if err := addResourceRef(owner, resource); err != nil {
				return fmt.Errorf("unable to add resource ref: %w", err)
			}
		}

		// Update the owner
		if err := c.Update(ctx, owner); err != nil {
			return fmt.Errorf("failed to update owner with resource references: %w", err)
		}
	}

	return nil
}

// setOwnerReference adds an owner reference to the resource
func setOwnerReference(resource, owner *unstructured.Unstructured) {
	// Create owner reference
	ownerRef := metav1.OwnerReference{
		APIVersion:         owner.GetAPIVersion(),
		Kind:               owner.GetKind(),
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}

	// Set the owner reference
	resource.SetOwnerReferences([]metav1.OwnerReference{ownerRef})
}

// addResourceRef adds a reference to the child resource in the parent's resourceRefs array
func addResourceRef(parent, child *unstructured.Unstructured) error {
	// Create the resource reference
	ref := map[string]interface{}{
		"apiVersion": child.GetAPIVersion(),
		"kind":       child.GetKind(),
		"name":       child.GetName(),
	}

	// If the child has a namespace, include it
	if ns := child.GetNamespace(); ns != "" {
		ref["namespace"] = ns
	}

	// Get current resourceRefs or initialize if not present
	resourceRefs, found, err := unstructured.NestedSlice(parent.Object, "spec", "resourceRefs")
	if err != nil {
		return errors.Wrap(err, "cannot get resourceRefs from parent")
	}

	if !found || resourceRefs == nil {
		resourceRefs = []interface{}{}
	}

	// Add the new reference and update the parent
	resourceRefs = append(resourceRefs, ref)
	return unstructured.SetNestedSlice(parent.Object, resourceRefs, "spec", "resourceRefs")
}

// createOrUpdateResource creates or updates a resource in the cluster
func createOrUpdateResource(ctx context.Context, c client.Client, obj *unstructured.Unstructured) error {
	err := c.Create(ctx, obj)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// If the resource already exists, update it
			existing := &unstructured.Unstructured{}
			existing.SetGroupVersionKind(obj.GroupVersionKind())
			if err := c.Get(ctx, client.ObjectKey{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			}, existing); err != nil {
				return fmt.Errorf("failed to get existing resource: %w", err)
			}

			// Copy resource version to avoid conflicts
			obj.SetResourceVersion(existing.GetResourceVersion())
			// Copy UID to ensure we're updating the same object
			obj.SetUID(existing.GetUID())

			if err := c.Update(ctx, obj); err != nil {
				return fmt.Errorf("failed to update resource: %w", err)
			}
		} else {
			return fmt.Errorf("failed to create resource: %w", err)
		}
	}
	return nil
}
