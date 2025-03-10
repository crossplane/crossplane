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
	"context"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	dp "github.com/crossplane/crossplane/cmd/crank/beta/diff/diffprocessor"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

// MockClusterClient implements the ClusterClient interface for testing
type MockClusterClient struct {
	InitializeFn               func(ctx context.Context) error
	FindMatchingCompositionFn  func(*unstructured.Unstructured) (*apiextensionsv1.Composition, error)
	GetExtraResourcesFn        func(context.Context, []schema.GroupVersionResource, []metav1.LabelSelector) ([]unstructured.Unstructured, error)
	GetFunctionsFromPipelineFn func(*apiextensionsv1.Composition) ([]pkgv1.Function, error)
	GetXRDSchemaFn             func(context.Context, *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error)
}

// Initialize implements the ClusterClient interface
func (m *MockClusterClient) Initialize(ctx context.Context) error {
	if m.InitializeFn != nil {
		return m.InitializeFn(ctx)
	}
	return nil
}

// FindMatchingComposition implements the ClusterClient interface
func (m *MockClusterClient) FindMatchingComposition(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
	if m.FindMatchingCompositionFn != nil {
		return m.FindMatchingCompositionFn(res)
	}
	return nil, nil
}

// GetExtraResources implements the ClusterClient interface
func (m *MockClusterClient) GetExtraResources(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]unstructured.Unstructured, error) {
	if m.GetExtraResourcesFn != nil {
		return m.GetExtraResourcesFn(ctx, gvrs, selectors)
	}
	return nil, nil
}

// GetFunctionsFromPipeline implements the ClusterClient interface
func (m *MockClusterClient) GetFunctionsFromPipeline(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
	if m.GetFunctionsFromPipelineFn != nil {
		return m.GetFunctionsFromPipelineFn(comp)
	}
	return nil, nil
}

// GetXRDSchema implements the ClusterClient interface
func (m *MockClusterClient) GetXRDSchema(ctx context.Context, res *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error) {
	if m.GetXRDSchemaFn != nil {
		return m.GetXRDSchemaFn(ctx, res)
	}
	return nil, nil
}

// Ensure MockClusterClient implements the ClusterClient interface
var _ cc.ClusterClient = &MockClusterClient{}

// MockDiffProcessor implements the DiffProcessor interface for testing
type MockDiffProcessor struct {
	ProcessAllFn func(ctx context.Context, resources []*unstructured.Unstructured) error
}

// ProcessAll implements the DiffProcessor.ProcessAll method
func (m *MockDiffProcessor) ProcessAll(ctx context.Context, resources []*unstructured.Unstructured) error {
	if m.ProcessAllFn != nil {
		return m.ProcessAllFn(ctx, resources)
	}
	return nil
}

// Ensure MockDiffProcessor implements the DiffProcessor interface
var _ dp.DiffProcessor = &MockDiffProcessor{}

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

	resources, err := ResourceLoader(c.Files)
	if err != nil {
		return errors.Wrap(err, "failed to load resources")
	}

	processor, err := DiffProcessorFactory(config, client, c.Namespace)
	if err != nil {
		return errors.Wrap(err, "cannot create diff processor")
	}

	if err := processor.ProcessAll(ctx, resources); err != nil {
		return errors.Wrap(err, "unable to process one or more resources")
	}

	return nil
}

func TestCmd_Run(t *testing.T) {
	ctx := context.Background()

	// Save original factory functions
	originalClusterClientFactory := ClusterClientFactory
	originalDiffProcessorFactory := DiffProcessorFactory
	originalResourceLoader := ResourceLoader

	// Restore original functions at the end of the test
	defer func() {
		ClusterClientFactory = originalClusterClientFactory
		DiffProcessorFactory = originalDiffProcessorFactory
		ResourceLoader = originalResourceLoader
	}()

	// Sample resources for testing
	sampleResources := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "test.org/v1alpha1",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"name": "test-resource",
				},
			},
		},
	}

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
		wantErr         bool
		wantErrContains string
	}{
		"SuccessfulRun": {
			fields: fields{
				Namespace: "default",
				Files:     []string{"test-file.yaml"},
			},
			args: args{
				ctx: ctx,
			},
			setupMocks: func() {
				// Mock resource loading
				ResourceLoader = func(files []string) ([]*unstructured.Unstructured, error) {
					return sampleResources, nil
				}

				// Mock cluster client
				mockClient := &MockClusterClient{}
				ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
					return mockClient, nil
				}

				// Mock diff processor
				mockProcessor := &MockDiffProcessor{}
				DiffProcessorFactory = func(config *rest.Config, client cc.ClusterClient, namespace string) (dp.DiffProcessor, error) {
					return mockProcessor, nil
				}
			},
			wantErr: false,
		},
		"LoadResourcesError": {
			fields: fields{
				Namespace: "default",
				Files:     []string{"non-existent-file.yaml"},
			},
			args: args{
				ctx: ctx,
			},
			setupMocks: func() {
				// Mock resource loading error
				ResourceLoader = func(files []string) ([]*unstructured.Unstructured, error) {
					return nil, errors.New("failed to load resources")
				}
			},
			wantErr:         true,
			wantErrContains: "failed to load resources",
		},
		"ClusterClientInitializeError": {
			fields: fields{
				Namespace: "default",
				Files:     []string{"test-file.yaml"},
			},
			args: args{
				ctx: ctx,
			},
			setupMocks: func() {
				// Mock resource loading
				ResourceLoader = func(files []string) ([]*unstructured.Unstructured, error) {
					return sampleResources, nil
				}

				// Mock cluster client initialization error
				mockClient := &MockClusterClient{
					InitializeFn: func(ctx context.Context) error {
						return errors.New("failed to initialize cluster client")
					},
				}
				ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
					return mockClient, nil
				}
			},
			wantErr:         true,
			wantErrContains: "initialize diff processor",
		},
		"ProcessResourcesError": {
			fields: fields{
				Namespace: "default",
				Files:     []string{"test-file.yaml"},
			},
			args: args{
				ctx: ctx,
			},
			setupMocks: func() {
				// Mock resource loading
				ResourceLoader = func(files []string) ([]*unstructured.Unstructured, error) {
					return sampleResources, nil
				}

				// Mock cluster client
				mockClient := &MockClusterClient{}
				ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
					return mockClient, nil
				}

				// Mock diff processor with processing error
				mockProcessor := &MockDiffProcessor{
					ProcessAllFn: func(ctx context.Context, resources []*unstructured.Unstructured) error {
						return errors.New("processing error")
					},
				}
				DiffProcessorFactory = func(config *rest.Config, client cc.ClusterClient, namespace string) (dp.DiffProcessor, error) {
					return mockProcessor, nil
				}
			},
			wantErr:         true,
			wantErrContains: "process one or more resources",
		},
		"ClusterClientFactoryError": {
			fields: fields{
				Namespace: "default",
				Files:     []string{"test-file.yaml"},
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
			wantErr:         true,
			wantErrContains: "cannot initialize cluster client",
		},
		"DiffProcessorFactoryError": {
			fields: fields{
				Namespace: "default",
				Files:     []string{"test-file.yaml"},
			},
			args: args{
				ctx: ctx,
			},
			setupMocks: func() {
				// Mock resource loading
				ResourceLoader = func(files []string) ([]*unstructured.Unstructured, error) {
					return sampleResources, nil
				}

				// Mock cluster client
				mockClient := &MockClusterClient{}
				ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
					return mockClient, nil
				}

				// Mock diff processor factory error
				DiffProcessorFactory = func(config *rest.Config, client cc.ClusterClient, namespace string) (dp.DiffProcessor, error) {
					return nil, errors.New("failed to create diff processor")
				}
			},
			wantErr:         true,
			wantErrContains: "cannot create diff processor",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup mocks for this test case
			tc.setupMocks()

			c := &Cmd{
				Namespace: tc.fields.Namespace,
				Files:     tc.fields.Files,
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
