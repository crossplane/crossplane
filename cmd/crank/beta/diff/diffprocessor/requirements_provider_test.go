package diffprocessor

import (
	"context"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestUnifiedExtraResourceProvider_ProcessRequirements(t *testing.T) {
	ctx := t.Context()

	// Create resources for testing
	configMap := tu.NewResource("v1", "ConfigMap", "config1").Build()
	secret := tu.NewResource("v1", "Secret", "secret1").Build()

	// Mock client that returns appropriate resources
	mockClient := tu.NewMockClusterClient().
		WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, ns, name string) (*unstructured.Unstructured, error) {
			if gvk.Kind == "ConfigMap" && name == "config1" {
				return configMap, nil
			}
			if gvk.Kind == "Secret" && name == "secret1" {
				return secret, nil
			}
			return nil, errors.New("resource not found")
		}).
		WithGetResourcesByLabel(func(ctx context.Context, ns string, gvr schema.GroupVersionKind, sel metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
			// Return resources for label-based selectors
			if sel.MatchLabels["app"] == "test-app" {
				return []*unstructured.Unstructured{configMap}, nil
			}
			return []*unstructured.Unstructured{}, nil
		}).
		Build()

	// Create the provider
	provider := NewRequirementsProvider(
		mockClient,
		nil, // renderFn not needed for this test
		tu.VerboseTestLogger(t),
	)

	// Test cases
	tests := map[string]struct {
		requirements map[string]v1.Requirements
		wantCount    int
		wantNames    []string
		wantErr      bool
	}{
		"EmptyRequirements": {
			requirements: map[string]v1.Requirements{},
			wantCount:    0,
			wantErr:      false,
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
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
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
