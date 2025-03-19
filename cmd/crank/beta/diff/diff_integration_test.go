package diff

import (
	"bytes"
	"context"
	"fmt"
	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/go-logr/stdr"
	"io"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	stdlog "log"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
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
		name           string
		setupFiles     []string
		inputFile      string
		expectedOutput string
		expectedError  bool
		noColor        bool
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
				`+++ XNopResource/test-resource
`, tu.Green(`+ apiVersion: diff.example.org/v1alpha1
+ kind: XNopResource
+ metadata:
+   name: test-resource
+ spec:
+   coolField: new-value
`), `
---
+++ XDownstreamResource/test-resource
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
~~~ XNopResource/test-resource
  apiVersion: diff.example.org/v1alpha1
  kind: XNopResource
  metadata:
    name: test-resource
  spec:
` + tu.Red("-   coolField: existing-value") + `
` + tu.Green("+   coolField: modified-value") + `

---
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
~~~ XNopResource/test-resource
  apiVersion: diff.example.org/v1alpha1
  kind: XNopResource
  metadata:
    name: test-resource
  spec:
` + tu.Red("-   coolField: existing-value") + `
` + tu.Green("+   coolField: modified-value") + `

---
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
~~~ XEnvResource/test-env-resource
  apiVersion: diff.example.org/v1alpha1
  kind: XEnvResource
  metadata:
    name: test-env-resource
  spec:
-   configKey: existing-config-value
+   configKey: modified-config-value

---
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
`,
			expectedError: false,
			noColor:       true,
		},
		{
			// this test does a weird thing where it changes the XR but all the downstream changes come from external
			// resources, including a field path from the XR itself.
			name: "Diff with external resource dependencies via fn-external-resources with field paths",
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
`,
			expectedError: false,
			noColor:       true,
		},
	}

	SetupTestLogger(t)

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
			ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
				return cc.NewClusterClient(cfg)
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
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		// Split the file into individual YAML documents
		docs := bytes.Split(data, []byte("---"))

		for _, doc := range docs {
			// Skip empty documents
			if len(bytes.TrimSpace(doc)) == 0 {
				continue
			}

			obj := &unstructured.Unstructured{}
			if err := yaml.Unmarshal(doc, obj); err != nil {
				return fmt.Errorf("failed to unmarshal YAML document from %s: %w", path, err)
			}

			// Skip empty objects
			if len(obj.Object) == 0 {
				continue
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
	}
	return nil
}

func SetupTestLogger(t *testing.T) {
	// Create a logr.Logger that writes to testing.T.Log
	testLogger := stdr.NewWithOptions(stdlog.New(testWriter{t}, "", 0), stdr.Options{LogCaller: stdr.All})

	// Set the logger for controller-runtime
	log.SetLogger(testLogger)
}

// testWriter adapts testing.T.Log to io.Writer
type testWriter struct {
	t *testing.T
}

func (tw testWriter) Write(p []byte) (int, error) {
	tw.t.Log(string(p))
	return len(p), nil
}
