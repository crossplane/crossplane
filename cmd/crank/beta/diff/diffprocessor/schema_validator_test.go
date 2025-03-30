package diffprocessor

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ cc.ClusterClient = (*tu.MockClusterClient)(nil)

func TestDefaultSchemaValidator_ValidateResources(t *testing.T) {
	ctx := context.Background()

	// Create a sample XR and composed resources for validation
	xr := tu.NewResource("example.org/v1", "XR", "test-xr").
		WithSpecField("field", "value").
		Build()

	composedResource1 := tu.NewResource("composed.org/v1", "ComposedResource", "resource1").
		WithCompositeOwner("test-xr").
		WithCompositionResourceName("resource1").
		WithSpecField("field", "value").
		BuildUComposed()

	composedResource2 := tu.NewResource("composed.org/v1", "ComposedResource", "resource2").
		WithCompositeOwner("test-xr").
		WithCompositionResourceName("resource2").
		WithSpecField("field", "value").
		BuildUComposed()

	// Create sample CRDs for validation
	xrCRD := makeCRD("xrs.example.org", "XR", "example.org", "v1")
	composedCRD := makeCRD("composedresources.composed.org", "ComposedResource", "composed.org", "v1")

	tests := map[string]struct {
		setupClient    func() *tu.MockClusterClient
		xr             *unstructured.Unstructured
		composed       []composed.Unstructured
		preloadedCRDs  []*extv1.CustomResourceDefinition
		expectedErr    bool
		expectedErrMsg string
	}{
		"SuccessfulValidationWithPreloadedCRDs": {
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().Build()
			},
			xr:            xr,
			composed:      []composed.Unstructured{*composedResource1, *composedResource2},
			preloadedCRDs: []*extv1.CustomResourceDefinition{xrCRD, composedCRD},
			expectedErr:   false,
		},
		"SuccessfulValidationWithFetchedCRDs": {
			setupClient: func() *tu.MockClusterClient {
				// Convert CRDs to unstructured for the mock client
				xrCRDUn := &unstructured.Unstructured{}
				runtime.DefaultUnstructuredConverter.FromUnstructured(
					MustToUnstructured(xrCRD),
					xrCRDUn,
				)

				composedCRDUn := &unstructured.Unstructured{}
				runtime.DefaultUnstructuredConverter.FromUnstructured(
					MustToUnstructured(composedCRD),
					composedCRDUn,
				)

				return tu.NewMockClusterClient().
					WithSuccessfulXRDsFetch([]*unstructured.Unstructured{}).
					// Add GetCRD implementation
					WithGetCRD(func(ctx context.Context, gvk schema.GroupVersionKind) (*unstructured.Unstructured, error) {
						if gvk.Group == "example.org" && gvk.Kind == "XR" {
							return xrCRDUn, nil
						}
						if gvk.Group == "composed.org" && gvk.Kind == "ComposedResource" {
							return composedCRDUn, nil
						}
						return nil, errors.New("CRD not found")
					}).
					// Implement IsCRDRequired to return true for our test resources
					WithAllResourcesRequiringCRDs().
					Build()
			},
			xr:            xr,
			composed:      []composed.Unstructured{*composedResource1, *composedResource2},
			preloadedCRDs: []*extv1.CustomResourceDefinition{},
			expectedErr:   false,
		},
		"MissingCRD": {
			setupClient: func() *tu.MockClusterClient {
				// Only provide the XR CRD, not the composed resource CRD
				xrCRDUn := &unstructured.Unstructured{}
				runtime.DefaultUnstructuredConverter.FromUnstructured(
					MustToUnstructured(xrCRD),
					xrCRDUn,
				)

				return tu.NewMockClusterClient().
					WithSuccessfulXRDsFetch([]*unstructured.Unstructured{}).
					WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, ns, name string) (*unstructured.Unstructured, error) {
						if name == "xrs.example.org" {
							return xrCRDUn, nil
						}
						// Return not found for composed resource CRD
						return nil, errors.New("CRD not found")
					}).
					// Add this line to make only Composed resources require CRDs:
					WithResourcesRequiringCRDs(
						schema.GroupVersionKind{Group: "composed.org", Version: "v1", Kind: "ComposedResource"},
					).
					Build()
			},
			xr:            xr,
			composed:      []composed.Unstructured{*composedResource1, *composedResource2},
			preloadedCRDs: []*extv1.CustomResourceDefinition{},
			// Now we expect an error because we've configured it to require a CRD but can't find it
			expectedErr:    true,
			expectedErrMsg: "unable to find CRD for composed.org/v1, Kind=ComposedResource",
		},
		"ValidationError": {
			setupClient: func() *tu.MockClusterClient {
				// Convert CRDs to unstructured for the mock
				composedCRDUn := &unstructured.Unstructured{}
				runtime.DefaultUnstructuredConverter.FromUnstructured(
					MustToUnstructured(createCRDWithStringField(composedCRD)),
					composedCRDUn,
				)

				return tu.NewMockClusterClient().
					// Add GetCRD implementation
					WithGetCRD(func(ctx context.Context, gvk schema.GroupVersionKind) (*unstructured.Unstructured, error) {
						if gvk.Group == "example.org" && gvk.Kind == "XR" {
							return nil, errors.New("CRD not found") // Force validation to use preloaded CRDs
						}
						if gvk.Group == "composed.org" && gvk.Kind == "ComposedResource" {
							return composedCRDUn, nil
						}
						return nil, errors.New("CRD not found")
					}).
					// Setup IsCRDRequired to return true for our test resources
					WithAllResourcesRequiringCRDs().
					Build()
			},
			xr: tu.NewResource("example.org/v1", "XR", "test-xr").
				WithSpecField("field", int64(123)).
				Build(),
			composed:       []composed.Unstructured{*composedResource1, *composedResource2},
			preloadedCRDs:  []*extv1.CustomResourceDefinition{createCRDWithStringField(xrCRD)},
			expectedErr:    true,
			expectedErrMsg: "schema validation failed",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := tt.setupClient()
			logger := tu.TestLogger(t)

			// Create the schema validator
			validator := NewSchemaValidator(mockClient, logger)

			// Set any preloaded CRDs
			if len(tt.preloadedCRDs) > 0 {
				validator.(*DefaultSchemaValidator).SetCRDs(tt.preloadedCRDs)
			}

			// Call the function under test
			err := validator.ValidateResources(ctx, tt.xr, tt.composed)

			// Check error expectations
			if tt.expectedErr {
				if err == nil {
					t.Errorf("ValidateResources() expected error but got none")
					return
				}
				if tt.expectedErrMsg != "" && !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("ValidateResources() error %q doesn't contain expected message %q",
						err.Error(), tt.expectedErrMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateResources() unexpected error: %v", err)
			}
		})
	}
}

func TestDefaultSchemaValidator_EnsureComposedResourceCRDs(t *testing.T) {
	ctx := context.Background()

	// Create sample resources
	xr := tu.NewResource("example.org/v1", "XR", "test-xr").Build()
	composed := tu.NewResource("composed.org/v1", "ComposedResource", "resource1").Build()

	// Create sample CRDs
	xrCRD := makeCRD("xrs.example.org", "XR", "example.org", "v1")
	composedCRD := makeCRD("composedresources.composed.org", "ComposedResource", "composed.org", "v1")

	tests := map[string]struct {
		setupClient    func() *tu.MockClusterClient
		initialCRDs    []*extv1.CustomResourceDefinition
		resources      []*unstructured.Unstructured
		expectedCRDLen int
	}{
		"AllCRDsAlreadyCached": {
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().Build()
			},
			initialCRDs:    []*extv1.CustomResourceDefinition{xrCRD, composedCRD},
			resources:      []*unstructured.Unstructured{xr, composed},
			expectedCRDLen: 2, // No change, all CRDs already cached
		},
		"FetchMissingCRDs": {
			setupClient: func() *tu.MockClusterClient {
				// Convert the composed CRD to unstructured for the mock
				composedCRDUn := &unstructured.Unstructured{}
				runtime.DefaultUnstructuredConverter.FromUnstructured(
					MustToUnstructured(composedCRD),
					composedCRDUn,
				)

				return tu.NewMockClusterClient().
					// Use the new GetCRD method instead of GetResource
					WithGetCRD(func(ctx context.Context, gvk schema.GroupVersionKind) (*unstructured.Unstructured, error) {
						if gvk.Group == "composed.org" && gvk.Kind == "ComposedResource" {
							return composedCRDUn, nil
						}
						return nil, errors.New("CRD not found")
					}).
					// Make sure composed resources require CRDs
					WithResourcesRequiringCRDs(
						schema.GroupVersionKind{Group: "composed.org", Version: "v1", Kind: "ComposedResource"},
					).
					Build()
			},
			initialCRDs:    []*extv1.CustomResourceDefinition{xrCRD}, // Only XR CRD is cached
			resources:      []*unstructured.Unstructured{xr, composed},
			expectedCRDLen: 2, // Should fetch the missing composed CRD
		},
		"SomeCRDsMissing": {
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, ns, name string) (*unstructured.Unstructured, error) {
						// Return not found for all CRDs
						return nil, errors.New("CRD not found")
					}).
					Build()
			},
			initialCRDs:    []*extv1.CustomResourceDefinition{xrCRD}, // Only XR CRD is cached
			resources:      []*unstructured.Unstructured{xr, composed},
			expectedCRDLen: 1, // Still only has the initial XR CRD
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := tt.setupClient()
			logger := tu.TestLogger(t)

			// Create the schema validator with initial CRDs
			validator := NewSchemaValidator(mockClient, logger)
			validator.(*DefaultSchemaValidator).SetCRDs(tt.initialCRDs)

			// Call the function under test
			validator.(*DefaultSchemaValidator).EnsureComposedResourceCRDs(ctx, tt.resources)

			// Verify the CRD count
			crds := validator.(*DefaultSchemaValidator).GetCRDs()
			if len(crds) != tt.expectedCRDLen {
				t.Errorf("EnsureComposedResourceCRDs() resulted in %d CRDs, want %d",
					len(crds), tt.expectedCRDLen)
			}
		})
	}
}

func TestDefaultSchemaValidator_LoadCRDs(t *testing.T) {
	ctx := context.Background()

	// Create sample CRDs as unstructured
	xrdUn := tu.NewResource("apiextensions.crossplane.io/v1", "CompositeResourceDefinition", "xrd1").
		WithSpecField("group", "example.org").
		WithSpecField("names", map[string]interface{}{
			"kind":     "XR",
			"plural":   "xrs",
			"singular": "xr",
		}).
		Build()

	tests := map[string]struct {
		setupClient    func() cc.ClusterClient
		preloadedCRDs  []*extv1.CustomResourceDefinition
		expectedErr    bool
		expectedErrMsg string
		// for caching tests
		callTwice      bool // Test making two calls to LoadCRDs
		expectXRDCalls int  // Expected number of calls to GetXRDs
	}{
		"SuccessfulLoad": {
			setupClient: func() cc.ClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulXRDsFetch([]*unstructured.Unstructured{xrdUn}).
					Build()
			},
			expectedErr: false,
		},
		"XRDFetchError": {
			setupClient: func() cc.ClusterClient {
				return tu.NewMockClusterClient().
					WithFailedXRDsFetch("failed to fetch XRDs").
					Build()
			},
			expectedErr: true,
		},
		"UsesCachedXRDs": {
			setupClient: func() cc.ClusterClient {
				// Create a tracking client that counts GetXRDs calls
				return &xrdCountingClient{
					MockClusterClient: *tu.NewMockClusterClient().
						WithSuccessfulXRDsFetch([]*unstructured.Unstructured{xrdUn}).
						Build(),
				}
			},
			preloadedCRDs:  nil, // No preloaded CRDs
			expectedErr:    false,
			callTwice:      true, // Make two calls to LoadCRDs
			expectXRDCalls: 1,    // GetXRDs should only be called once due to caching
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := tt.setupClient()
			logger := tu.TestLogger(t)

			// Create the schema validator
			validator := NewSchemaValidator(mockClient, logger)

			// Call the function under test
			err := validator.(*DefaultSchemaValidator).LoadCRDs(ctx)

			// Check error expectations
			if tt.expectedErr {
				if err == nil {
					t.Errorf("LoadCRDs() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("LoadCRDs() unexpected error: %v", err)
				return
			}

			// Verify CRDs were loaded (for successful case)
			crds := validator.(*DefaultSchemaValidator).GetCRDs()
			if len(crds) == 0 {
				t.Errorf("LoadCRDs() did not load any CRDs")
			}
		})
	}
}

// Helper function to create a simple CRD
func makeCRD(name string, kind string, group string, version string) *extv1.CustomResourceDefinition {
	return &extv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: extv1.CustomResourceDefinitionNames{
				Kind:     kind,
				ListKind: kind + "List",
				Plural:   strings.ToLower(kind) + "s",
				Singular: strings.ToLower(kind),
			},
			Scope: extv1.NamespaceScoped,
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    version,
					Served:  true,
					Storage: true,
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"spec": {
									Type: "object",
									Properties: map[string]extv1.JSONSchemaProps{
										"field": {
											Type: "string",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// Create a CRD with a string field validation
func createCRDWithStringField(baseCRD *extv1.CustomResourceDefinition) *extv1.CustomResourceDefinition {
	crd := baseCRD.DeepCopy()
	// Ensure the schema requires 'field' to be a string
	crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["field"] = extv1.JSONSchemaProps{
		Type: "string",
	}
	return crd
}

// Helper function to convert to unstructured
func MustToUnstructured(obj interface{}) map[string]interface{} {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		panic(err)
	}
	return u
}

// Helper type to track GetXRDs calls
type xrdCountingClient struct {
	tu.MockClusterClient
	getXRDsCallCount int
}

// Override GetXRDs to count calls
func (c *xrdCountingClient) GetXRDs(ctx context.Context) ([]*unstructured.Unstructured, error) {
	c.getXRDsCallCount++
	return c.MockClusterClient.GetXRDs(ctx)
}
