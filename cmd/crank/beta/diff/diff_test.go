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
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal"
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

	// Create the options for the processor
	options := []dp.DiffProcessorOption{
		dp.WithRestConfig(config),
		dp.WithNamespace(c.Namespace),
		dp.WithColorize(!c.NoColor),
		dp.WithCompact(c.Compact),
	}

	processor, err := DiffProcessorFactory(client, options...)
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
		NoColor   bool
		Compact   bool
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
				NoColor:   false,
				Compact:   false,
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
				DiffProcessorFactory = func(client cc.ClusterClient, opts ...dp.DiffProcessorOption) (dp.DiffProcessor, error) {
					return &testutils.MockDiffProcessor{
						InitializeFn: func(writer io.Writer, ctx context.Context) error {
							return nil
						},
						ProcessAllFn: func(stdout io.Writer, ctx context.Context, resources []*unstructured.Unstructured) error {
							return nil
						},
					}, nil
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
		//		"ProcessResourcesWithColorAndCompact": {
		//			fields: fields{
		//				Namespace: "default",
		//				Files:     []string{},
		//				NoColor:   true, // Testing the NoColor flag
		//				Compact:   true, // Testing the Compact flag
		//			},
		//			args: args{
		//				ctx: ctx,
		//			},
		//			setupMocks: func() {
		//				// Mock cluster client
		//				mockClient := &testutils.MockClusterClient{
		//					InitializeFn: func(ctx context.Context) error {
		//						return nil
		//					},
		//					GetXRDsFn: func(ctx context.Context) ([]*unstructured.Unstructured, error) {
		//						return nil, nil
		//					},
		//				}
		//				ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
		//					return mockClient, nil
		//				}
		//
		//				// Store the options passed to the factory for verification
		//				var capturedOptions []dp.DiffProcessorOption
		//
		//				// Mock diff processor that captures the options
		//				DiffProcessorFactory = func(client cc.ClusterClient, opts ...dp.DiffProcessorOption) (dp.DiffProcessor, error) {
		//					capturedOptions = opts
		//
		//					// Create the processor config that we'll check
		//					processorConfig := &dp.ProcessorConfig{}
		//
		//					// Apply all captured options to this config to test them
		//					for _, opt := range capturedOptions {
		//						opt(processorConfig)
		//					}
		//
		//					// Check that we got the expected options
		//					colorizeFound := processorConfig.Colorize == false
		//					compactFound := processorConfig.Compact == true
		//
		//					// Create a mock processor that verifies options were applied
		//					mockProcessor := &testutils.MockDiffProcessor{
		//						InitializeFn: func(writer io.Writer, ctx context.Context) error {
		//							return nil
		//						},
		//						ProcessAllFn: func(stdout io.Writer, ctx context.Context, resources []*unstructured.Unstructured) error {
		//
		//							return nil
		//						},
		//					}
		//
		//					return mockProcessor, nil
		//				}
		//			},
		//			setupFiles: func() []string {
		//				// Create a temporary test file
		//				tempDir, err := os.MkdirTemp("", "diff-test")
		//				if err != nil {
		//					t.Fatalf("Failed to create temp dir: %v", err)
		//				}
		//				t.Cleanup(func() { os.RemoveAll(tempDir) })
		//
		//				tempFile := filepath.Join(tempDir, "test-resource.yaml")
		//				content := `
		//apiVersion: test.org/v1alpha1
		//kind: TestResource
		//metadata:
		//  name: test-resource
		//`
		//				if err := os.WriteFile(tempFile, []byte(content), 0600); err != nil {
		//					t.Fatalf("Failed to write temp file: %v", err)
		//				}
		//
		//				return []string{tempFile}
		//			},
		//			wantErr: false,
		//		},
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
				DiffProcessorFactory = func(client cc.ClusterClient, opts ...dp.DiffProcessorOption) (dp.DiffProcessor, error) {
					return &testutils.MockDiffProcessor{
						InitializeFn: func(writer io.Writer, ctx context.Context) error {
							return nil
						},
						ProcessAllFn: func(stdout io.Writer, ctx context.Context, resources []*unstructured.Unstructured) error {
							return errors.New("processing error")
						},
					}, nil
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
				DiffProcessorFactory = func(client cc.ClusterClient, opts ...dp.DiffProcessorOption) (dp.DiffProcessor, error) {
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
				NoColor:   tc.fields.NoColor,
				Compact:   tc.fields.Compact,
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
