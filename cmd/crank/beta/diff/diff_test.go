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
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal"
	"github.com/crossplane/crossplane/cmd/crank/render"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	dp "github.com/crossplane/crossplane/cmd/crank/beta/diff/diffprocessor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

// Custom Run function for testing - this avoids calling the real Run()
func testRun(ctx context.Context, c *Cmd, setupConfig func() (*rest.Config, error)) error {
	config, err := setupConfig()
	if err != nil {
		return errors.Wrap(err, "failed to get kubeconfig")
	}

	client, err := ClusterClientFactory(config)
	if err != nil {
		return errors.Wrap(err, "cannot initialize cluster client")
	}

	if err := client.Initialize(ctx); err != nil {
		return errors.Wrap(err, "cannot initialize diff processor")
	}

	// Create a temporary test file if needed
	tempFiles := make([]string, 0, len(c.Files))
	stdinUsed := false

	for _, f := range c.Files {
		if f == "test-xr.yaml" {
			// Create a temp file with test content
			tempDir, err := os.MkdirTemp("", "diff-test")
			if err != nil {
				return errors.Wrap(err, "failed to create temp dir")
			}
			defer os.RemoveAll(tempDir)

			tempFile := filepath.Join(tempDir, "test-xr.yaml")
			content := `
apiVersion: example.org/v1
kind: XExampleResource
metadata:
  name: test-xr
spec:
  coolParam: test-value
  replicas: 3
`
			if err := os.WriteFile(tempFile, []byte(content), 0600); err != nil {
				return errors.Wrap(err, "failed to write temp file")
			}

			tempFiles = append(tempFiles, tempFile)
		} else if f == "-" {
			if !stdinUsed {
				tempFiles = append(tempFiles, "-")
				stdinUsed = true
			}
			// Skip duplicate stdin markers
		} else {
			tempFiles = append(tempFiles, f)
		}
	}

	// Create a composite loader for the resources (inlined from loadResources)
	loader, err := internal.NewCompositeLoader(tempFiles)
	if err != nil {
		return errors.Wrap(err, "cannot create resource loader")
	}

	resources, err := loader.Load()
	if err != nil {
		return errors.Wrap(err, "cannot load resources")
	}

	renderFunc := func(ctx context.Context, logger logging.Logger, in render.Inputs) (render.Outputs, error) {
		// This is a placeholder - in tests, this will typically be overridden
		return render.Outputs{}, nil
	}

	processor, err := DiffProcessorFactory(config, client, c.Namespace, renderFunc, nil)
	if err != nil {
		return errors.Wrap(err, "cannot create diff processor")
	}

	// Initialize the diff processor with a dummy writer
	var dummyWriter bytes.Buffer
	if err := processor.Initialize(&dummyWriter, ctx); err != nil {
		return errors.Wrap(err, "cannot initialize diff processor")
	}

	if err := processor.ProcessAll(nil, ctx, resources); err != nil {
		return errors.Wrap(err, "unable to process one or more resources")
	}

	return nil
}

func TestCmd_Run(t *testing.T) {
	ctx := context.Background()

	// Save original factory functions
	originalClusterClientFactory := ClusterClientFactory
	originalDiffProcessorFactory := DiffProcessorFactory

	// Restore original functions at the end of the test
	defer func() {
		ClusterClientFactory = originalClusterClientFactory
		DiffProcessorFactory = originalDiffProcessorFactory
	}()

	type fields struct {
		Namespace string
		Files     []string
	}

	type args struct {
		ctx context.Context
	}

	tests := map[string]struct {
		fields          fields
		args            args
		setupMocks      func()
		setupFiles      func() []string
		wantErr         bool
		wantErrContains string
	}{
		"SuccessfulRun": {
			fields: fields{
				Namespace: "default",
				Files:     []string{},
			},
			args: args{
				ctx: ctx,
			},
			setupMocks: func() {
				// Mock cluster client
				mockClient := &testutils.MockClusterClient{
					InitializeFn: func(ctx context.Context) error {
						return nil
					},
					GetXRDsFn: func(ctx context.Context) ([]*unstructured.Unstructured, error) {
						return nil, nil
					},
				}
				ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
					return mockClient, nil
				}

				// Mock diff processor
				mockProcessor := &testutils.MockDiffProcessor{
					InitializeFn: func(writer io.Writer, ctx context.Context) error {
						return nil
					},
					// Mock loadResources
					ProcessAllFn: func(stdout io.Writer, ctx context.Context, resources []*unstructured.Unstructured) error {
						return nil
					},
				}
				DiffProcessorFactory = func(config *rest.Config, client cc.ClusterClient, namespace string, renderFunc dp.RenderFunc, logger logging.Logger) (dp.DiffProcessor, error) {
					return mockProcessor, nil
				}
			},
			setupFiles: func() []string {
				// Create a temporary test file
				tempDir, err := os.MkdirTemp("", "diff-test")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				t.Cleanup(func() { os.RemoveAll(tempDir) })

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
		"ClusterClientInitializeError": {
			fields: fields{
				Namespace: "default",
				Files:     []string{},
			},
			args: args{
				ctx: ctx,
			},
			setupMocks: func() {
				// Mock cluster client initialization error
				mockClient := &testutils.MockClusterClient{
					InitializeFn: func(ctx context.Context) error {
						return errors.New("failed to initialize cluster client")
					},
				}
				ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
					return mockClient, nil
				}
			},
			setupFiles: func() []string {
				return []string{}
			},
			wantErr:         true,
			wantErrContains: "initialize diff processor",
		},
		"ProcessResourcesError": {
			fields: fields{
				Namespace: "default",
				Files:     []string{},
			},
			args: args{
				ctx: ctx,
			},
			setupMocks: func() {
				// Mock cluster client
				mockClient := &testutils.MockClusterClient{
					InitializeFn: func(ctx context.Context) error {
						return nil
					},
					GetXRDsFn: func(ctx context.Context) ([]*unstructured.Unstructured, error) {
						return nil, nil
					},
				}
				ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
					return mockClient, nil
				}

				// Mock diff processor with processing error
				mockProcessor := &testutils.MockDiffProcessor{
					InitializeFn: func(writer io.Writer, ctx context.Context) error {
						return nil
					},
					ProcessAllFn: func(stdout io.Writer, ctx context.Context, resources []*unstructured.Unstructured) error {
						return errors.New("processing error")
					},
				}
				DiffProcessorFactory = func(config *rest.Config, client cc.ClusterClient, namespace string, renderFunc dp.RenderFunc, logger logging.Logger) (dp.DiffProcessor, error) {
					return mockProcessor, nil
				}
			},
			setupFiles: func() []string {
				// Create a temporary test file
				tempDir, err := os.MkdirTemp("", "diff-test")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				t.Cleanup(func() { os.RemoveAll(tempDir) })

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
			wantErrContains: "process one or more resources",
		},
		"ClusterClientFactoryError": {
			fields: fields{
				Namespace: "default",
				Files:     []string{},
			},
			args: args{
				ctx: ctx,
			},
			setupMocks: func() {
				// Mock cluster client factory error
				ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
					return nil, errors.New("failed to create cluster client")
				}
			},
			setupFiles: func() []string {
				return []string{}
			},
			wantErr:         true,
			wantErrContains: "cannot initialize cluster client",
		},
		"DiffProcessorFactoryError": {
			fields: fields{
				Namespace: "default",
				Files:     []string{},
			},
			args: args{
				ctx: ctx,
			},
			setupMocks: func() {
				// Mock cluster client
				mockClient := &testutils.MockClusterClient{
					InitializeFn: func(ctx context.Context) error {
						return nil
					},
				}
				ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
					return mockClient, nil
				}

				// Mock diff processor factory error
				DiffProcessorFactory = func(config *rest.Config, client cc.ClusterClient, namespace string, renderFunc dp.RenderFunc, logger logging.Logger) (dp.DiffProcessor, error) {
					return nil, errors.New("failed to create diff processor")
				}
			},
			setupFiles: func() []string {
				// Create a temporary test file
				tempDir, err := os.MkdirTemp("", "diff-test")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				t.Cleanup(func() { os.RemoveAll(tempDir) })

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
			wantErrContains: "cannot create diff processor",
		},
		"DiffProcessorInitializeError": {
			fields: fields{
				Namespace: "default",
				Files:     []string{},
			},
			args: args{
				ctx: ctx,
			},
			setupMocks: func() {
				// Mock cluster client
				mockClient := &testutils.MockClusterClient{
					InitializeFn: func(ctx context.Context) error {
						return nil
					},
				}
				ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
					return mockClient, nil
				}

				// Mock diff processor with initialize error
				mockProcessor := &testutils.MockDiffProcessor{
					InitializeFn: func(writer io.Writer, ctx context.Context) error {
						return errors.New("initialization error")
					},
				}
				DiffProcessorFactory = func(config *rest.Config, client cc.ClusterClient, namespace string, renderFunc dp.RenderFunc, logger logging.Logger) (dp.DiffProcessor, error) {
					return mockProcessor, nil
				}
			},
			setupFiles: func() []string {
				// Create a temporary test file
				tempDir, err := os.MkdirTemp("", "diff-test")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				t.Cleanup(func() { os.RemoveAll(tempDir) })

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
			wantErrContains: "cannot initialize diff processor",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup mocks for this test case
			tc.setupMocks()

			// Setup test files if needed
			files := tc.setupFiles()

			c := &Cmd{
				Namespace: tc.fields.Namespace,
				Files:     files,
			}

			// Use our test version of Run() that doesn't call clientcmd.BuildConfigFromFlags
			err := testRun(tc.args.ctx, c, func() (*rest.Config, error) {
				return &rest.Config{}, nil // Return a dummy config
			})

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

// Test direct usage of CompositeLoader
func TestResourceLoading(t *testing.T) {
	// Create a temporary test file
	tempDir, err := os.MkdirTemp("", "diff-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "test-resource.yaml")
	content := `
apiVersion: example.org/v1
kind: TestResource
metadata:
  name: test-resource
spec:
  testField: testValue
`
	if err := os.WriteFile(tempFile, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Test loading from file
	loader, err := internal.NewCompositeLoader([]string{tempFile})
	if err != nil {
		t.Errorf("NewCompositeLoader() error = %v", err)
		return
	}

	resources, err := loader.Load()
	if err != nil {
		t.Errorf("loader.Load() error = %v", err)
		return
	}

	if len(resources) != 1 {
		t.Errorf("loader.Load() expected 1 resource, got %d", len(resources))
		return
	}

	if resources[0].GetKind() != "TestResource" {
		t.Errorf("loader.Load() expected kind TestResource, got %s", resources[0].GetKind())
	}

	if resources[0].GetName() != "test-resource" {
		t.Errorf("loader.Load() expected name test-resource, got %s", resources[0].GetName())
	}

	// Test with multiple files
	tempFile2 := filepath.Join(tempDir, "test-resource2.yaml")
	content2 := `
apiVersion: example.org/v1
kind: TestResource2
metadata:
  name: test-resource2
spec:
  testField: testValue2
`
	if err := os.WriteFile(tempFile2, []byte(content2), 0600); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	loader, err = internal.NewCompositeLoader([]string{tempFile, tempFile2})
	if err != nil {
		t.Errorf("NewCompositeLoader() error = %v", err)
		return
	}

	resources, err = loader.Load()
	if err != nil {
		t.Errorf("loader.Load() error = %v", err)
		return
	}

	if len(resources) != 2 {
		t.Errorf("loader.Load() expected 2 resources, got %d", len(resources))
		return
	}

	// Test with directory
	loader, err = internal.NewCompositeLoader([]string{tempDir})
	if err != nil {
		t.Errorf("NewCompositeLoader() error = %v", err)
		return
	}

	resources, err = loader.Load()
	if err != nil {
		t.Errorf("loader.Load() error = %v", err)
		return
	}

	if len(resources) != 2 {
		t.Errorf("loader.Load() expected 2 resources from directory, got %d", len(resources))
	}
}

// Test loading resource from stdin
func TestLoadResourcesFromStdin(t *testing.T) {
	// Skip this test in automated CI environments where stdin might not be available
	if os.Getenv("CI") != "" {
		t.Skip("Skipping stdin test in CI environment")
	}

	// Save original stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Create a pipe to simulate stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	// Set os.Stdin to our read pipe
	os.Stdin = r

	// Write content to the pipe (simulating stdin input)
	stdinContent := `
apiVersion: example.org/v1
kind: StdinResource
metadata:
  name: stdin-resource
spec:
  field: stdin-value
`
	go func() {
		defer w.Close()
		_, err := io.WriteString(w, stdinContent)
		if err != nil {
			t.Errorf("Failed to write to stdin pipe: %v", err)
		}
	}()

	// Test loading from stdin using the CompositeLoader directly
	loader, err := internal.NewCompositeLoader([]string{"-"})
	if err != nil {
		t.Errorf("NewCompositeLoader() with stdin error = %v", err)
		return
	}

	resources, err := loader.Load()
	if err != nil {
		t.Errorf("loader.Load() from stdin error = %v", err)
		return
	}

	if len(resources) != 1 {
		t.Errorf("loader.Load() from stdin expected 1 resource, got %d", len(resources))
		return
	}

	if resources[0].GetKind() != "StdinResource" {
		t.Errorf("loader.Load() from stdin expected kind StdinResource, got %s", resources[0].GetKind())
	}

	if resources[0].GetName() != "stdin-resource" {
		t.Errorf("loader.Load() from stdin expected name stdin-resource, got %s", resources[0].GetName())
	}
}
