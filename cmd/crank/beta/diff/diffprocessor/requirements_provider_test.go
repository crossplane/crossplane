package diffprocessor

import (
	"context"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestRequirementsProvider_ProvideRequirements(t *testing.T) {
	ctx := context.Background()

	// Create resources for testing
	configMap := tu.NewResource("v1", "ConfigMap", "config1").Build()
	secret := tu.NewResource("v1", "Secret", "secret1").Build()

	tests := map[string]struct {
		requirements           map[string]v1.Requirements
		setupResourceClient    func() *tu.MockResourceClient
		setupEnvironmentClient func() *tu.MockEnvironmentClient
		wantCount              int
		wantNames              []string
		wantErr                bool
	}{
		"EmptyRequirements": {
			requirements: map[string]v1.Requirements{},
			setupResourceClient: func() *tu.MockResourceClient {
				return tu.NewMockResourceClient().Build()
			},
			setupEnvironmentClient: func() *tu.MockEnvironmentClient {
				return tu.NewMockEnvironmentClient().
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			wantCount: 0,
			wantErr:   false,
		},
		"NameSelector": {
			requirements: map[string]v1.Requirements{
				"step1": {
					ExtraResources: map[string]*v1.ResourceSelector{
						"config": {
							ApiVersion: "v1",
							Kind:       "ConfigMap",
							Match: &v1.ResourceSelector_MatchName{
								MatchName: "config1",
							},
						},
					},
				},
			},
			setupResourceClient: func() *tu.MockResourceClient {
				return tu.NewMockResourceClient().
					WithGetResource(func(_ context.Context, gvk schema.GroupVersionKind, _, name string) (*un.Unstructured, error) {
						if gvk.Kind == "ConfigMap" && name == "config1" {
							return configMap, nil
						}
						return nil, errors.New("resource not found")
					}).
					Build()
			},
			setupEnvironmentClient: func() *tu.MockEnvironmentClient {
				return tu.NewMockEnvironmentClient().
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			wantCount: 1,
			wantNames: []string{"config1"},
			wantErr:   false,
		},
		"LabelSelector": {
			requirements: map[string]v1.Requirements{
				"step1": {
					ExtraResources: map[string]*v1.ResourceSelector{
						"config": {
							ApiVersion: "v1",
							Kind:       "ConfigMap",
							Match: &v1.ResourceSelector_MatchLabels{
								MatchLabels: &v1.MatchLabels{
									Labels: map[string]string{
										"app": "test-app",
									},
								},
							},
						},
					},
				},
			},
			setupResourceClient: func() *tu.MockResourceClient {
				return tu.NewMockResourceClient().
					WithGetResourcesByLabel(func(_ context.Context, _ string, _ schema.GroupVersionKind, sel metav1.LabelSelector) ([]*un.Unstructured, error) {
						// Return resources for label-based selectors
						if sel.MatchLabels["app"] == "test-app" {
							return []*un.Unstructured{configMap}, nil
						}
						return []*un.Unstructured{}, nil
					}).
					Build()
			},
			setupEnvironmentClient: func() *tu.MockEnvironmentClient {
				return tu.NewMockEnvironmentClient().
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			wantCount: 1,
			wantNames: []string{"config1"},
			wantErr:   false,
		},
		"MultipleSelectors": {
			requirements: map[string]v1.Requirements{
				"step1": {
					ExtraResources: map[string]*v1.ResourceSelector{
						"config": {
							ApiVersion: "v1",
							Kind:       "ConfigMap",
							Match: &v1.ResourceSelector_MatchName{
								MatchName: "config1",
							},
						},
						"secret": {
							ApiVersion: "v1",
							Kind:       "Secret",
							Match: &v1.ResourceSelector_MatchName{
								MatchName: "secret1",
							},
						},
					},
				},
			},
			setupResourceClient: func() *tu.MockResourceClient {
				return tu.NewMockResourceClient().
					WithGetResource(func(_ context.Context, gvk schema.GroupVersionKind, _, name string) (*un.Unstructured, error) {
						if gvk.Kind == "ConfigMap" && name == "config1" {
							return configMap, nil
						}
						if gvk.Kind == "Secret" && name == "secret1" {
							return secret, nil
						}
						return nil, errors.New("resource not found")
					}).
					Build()
			},
			setupEnvironmentClient: func() *tu.MockEnvironmentClient {
				return tu.NewMockEnvironmentClient().
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			wantCount: 2,
			wantNames: []string{"config1", "secret1"},
			wantErr:   false,
		},
		"ResourceNotFound": {
			requirements: map[string]v1.Requirements{
				"step1": {
					ExtraResources: map[string]*v1.ResourceSelector{
						"missing": {
							ApiVersion: "v1",
							Kind:       "ConfigMap",
							Match: &v1.ResourceSelector_MatchName{
								MatchName: "missing-resource",
							},
						},
					},
				},
			},
			setupResourceClient: func() *tu.MockResourceClient {
				return tu.NewMockResourceClient().
					WithResourceNotFound().
					Build()
			},
			setupEnvironmentClient: func() *tu.MockEnvironmentClient {
				return tu.NewMockEnvironmentClient().
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			wantErr: true,
		},
		"EnvironmentConfigsAvailable": {
			requirements: map[string]v1.Requirements{
				"step1": {
					ExtraResources: map[string]*v1.ResourceSelector{
						"config": {
							ApiVersion: "v1",
							Kind:       "ConfigMap",
							Match: &v1.ResourceSelector_MatchName{
								MatchName: "config1",
							},
						},
					},
				},
			},
			setupResourceClient: func() *tu.MockResourceClient {
				// This resource client should not be called because the resource is in the env configs
				return tu.NewMockResourceClient().
					WithGetResource(func(_ context.Context, _ schema.GroupVersionKind, _, _ string) (*un.Unstructured, error) {
						return nil, errors.New("should not be called")
					}).
					Build()
			},
			setupEnvironmentClient: func() *tu.MockEnvironmentClient {
				return tu.NewMockEnvironmentClient().
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{configMap}).
					Build()
			},
			wantCount: 1,
			wantNames: []string{"config1"},
			wantErr:   false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up clients
			resourceClient := tt.setupResourceClient()
			environmentClient := tt.setupEnvironmentClient()

			// Create the requirements provider
			provider := NewRequirementsProvider(
				resourceClient,
				environmentClient,
				nil, // renderFn not needed for this test
				tu.TestLogger(t, false),
			)

			// Initialize the provider to cache any environment configs
			if err := provider.Initialize(ctx); err != nil {
				t.Fatalf("Failed to initialize provider: %v", err)
			}

			// Call the method being tested
			resources, err := provider.ProvideRequirements(ctx, tt.requirements)

			// Check error cases
			if tt.wantErr {
				if err == nil {
					t.Errorf("ProvideRequirements() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("ProvideRequirements() unexpected error: %v", err)
			}

			// Check resource count
			if len(resources) != tt.wantCount {
				t.Errorf("ProvideRequirements() returned %d resources, want %d",
					len(resources), tt.wantCount)
			}

			// Verify expected resource names if specified
			if tt.wantNames != nil {
				foundNames := make(map[string]bool)
				for _, res := range resources {
					foundNames[res.GetName()] = true
				}

				for _, name := range tt.wantNames {
					if !foundNames[name] {
						t.Errorf("Expected resource %q not found in result", name)
					}
				}
			}
		})
	}
}
