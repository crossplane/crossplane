package kubernetes

import (
	"context"
	"strings"
	"testing"

	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	kt "k8s.io/client-go/testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
)

var _ SchemaClient = (*tu.MockSchemaClient)(nil)

func TestSchemaClient_IsCRDRequired(t *testing.T) {
	// Set up context for tests
	ctx := context.Background()

	tests := map[string]struct {
		reason         string
		setupConverter func() TypeConverter
		gvk            schema.GroupVersionKind
		want           bool
	}{
		"CoreResource": {
			reason: "Core API resources (group='') should not require a CRD",
			setupConverter: func() TypeConverter {
				// Just need a mock converter as it shouldn't be called for core resources
				return tu.NewMockTypeConverter().Build()
			},
			gvk: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			want: false, // Core API resource should not require a CRD
		},
		"KubernetesExtensionResource": {
			reason: "Kubernetes extension resources (like apps/v1) should not require a CRD",
			setupConverter: func() TypeConverter {
				// Just need a mock converter as it shouldn't be called for k8s resources
				return tu.NewMockTypeConverter().Build()
			},
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			want: false, // Kubernetes extension should not require a CRD
		},
		"CustomResource": {
			reason: "Custom resources (non-standard domain) should require a CRD",
			setupConverter: func() TypeConverter {
				// For custom resources, our converter should return a successful resource name
				return tu.NewMockTypeConverter().
					WithGetResourceNameForGVK(func(_ context.Context, gvk schema.GroupVersionKind) (string, error) {
						if gvk.Group == "example.org" && gvk.Version == "v1" && gvk.Kind == "XResource" {
							return "xresources", nil
						}
						return "", errors.New("unexpected GVK in test")
					}).Build()
			},
			gvk: schema.GroupVersionKind{
				Group:   "example.org",
				Version: "v1",
				Kind:    "XResource",
			},
			want: true, // Custom resource should require a CRD
		},
		"APIExtensionResource": {
			reason: "API Extensions resources like CRDs themselves should require special handling",
			setupConverter: func() TypeConverter {
				// For apiextensions resources, our converter should return a successful resource name
				return tu.NewMockTypeConverter().
					WithGetResourceNameForGVK(func(_ context.Context, gvk schema.GroupVersionKind) (string, error) {
						if gvk.Group == "apiextensions.k8s.io" && gvk.Version == "v1" && gvk.Kind == "CustomResourceDefinition" {
							return "customresourcedefinitions", nil
						}
						return "", errors.New("unexpected GVK in test")
					}).Build()
			},
			gvk: schema.GroupVersionKind{
				Group:   "apiextensions.k8s.io",
				Version: "v1",
				Kind:    "CustomResourceDefinition",
			},
			want: true, // APIExtensions resources are handled specially and require CRDs
		},
		"OtherK8sIOButNotAPIExtensions": {
			reason: "Other k8s.io resources that are not from apiextensions should not require a CRD",
			setupConverter: func() TypeConverter {
				// For networking.k8s.io resources, our converter should return a successful resource name
				return tu.NewMockTypeConverter().
					WithGetResourceNameForGVK(func(_ context.Context, gvk schema.GroupVersionKind) (string, error) {
						if gvk.Group == "networking.k8s.io" && gvk.Version == "v1" && gvk.Kind == "NetworkPolicy" {
							return "networkpolicies", nil
						}
						return "", errors.New("unexpected GVK in test")
					}).Build()
			},
			gvk: schema.GroupVersionKind{
				Group:   "networking.k8s.io",
				Version: "v1",
				Kind:    "NetworkPolicy",
			},
			want: false, // Other k8s.io resources should not require a CRD
		},
		"ConverterError": {
			reason: "If type conversion fails, should default to requiring a CRD",
			setupConverter: func() TypeConverter {
				// Create mock type converter that returns an error
				return tu.NewMockTypeConverter().
					WithGetResourceNameForGVK(func(context.Context, schema.GroupVersionKind) (string, error) {
						return "", errors.New("conversion error")
					}).Build()
			},
			gvk: schema.GroupVersionKind{
				Group:   "example.org",
				Version: "v1",
				Kind:    "XResource",
			},
			want: true, // Default to requiring CRD on conversion failure
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create a schema client with the test converter
			c := &DefaultSchemaClient{
				typeConverter:   tc.setupConverter(),
				logger:          tu.TestLogger(t, false),
				resourceTypeMap: make(map[schema.GroupVersionKind]bool),
			}

			// Call the method under test
			got := c.IsCRDRequired(ctx, tc.gvk)

			// Verify result
			if got != tc.want {
				t.Errorf("\n%s\nIsCRDRequired() = %v, want %v", tc.reason, got, tc.want)
			}
		})
	}
}

func TestSchemaClient_GetCRD(t *testing.T) {
	scheme := runtime.NewScheme()

	type args struct {
		ctx context.Context
		gvk schema.GroupVersionKind
	}

	type want struct {
		crd *un.Unstructured
		err error
	}

	// Create a test CRD as unstructured
	testCRD := &un.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "xresources.example.org",
			},
			"spec": map[string]interface{}{
				"group": "example.org",
				"names": map[string]interface{}{
					"kind":     "XResource",
					"plural":   "xresources",
					"singular": "xresource",
				},
				"scope": "Namespaced",
				"versions": []interface{}{
					map[string]interface{}{
						"name":    "v1",
						"served":  true,
						"storage": true,
					},
				},
			},
		},
	}

	tests := map[string]struct {
		reason string
		setup  func() (dynamic.Interface, TypeConverter)
		args   args
		want   want
	}{
		"SuccessfulCRDRetrieval": {
			reason: "Should retrieve CRD when it exists",
			setup: func() (dynamic.Interface, TypeConverter) {
				// Set up the dynamic client to return our test CRD
				dynamicClient := fake.NewSimpleDynamicClient(scheme)
				dynamicClient.Fake.PrependReactor("get", "customresourcedefinitions", func(action kt.Action) (bool, runtime.Object, error) {
					getAction := action.(kt.GetAction)
					if getAction.GetName() == "xresources.example.org" {
						return true, testCRD, nil
					}
					return false, nil, nil
				})

				// Create mock type converter that returns "xresources" for the given GVK
				mockConverter := tu.NewMockTypeConverter().
					WithGetResourceNameForGVK(func(_ context.Context, gvk schema.GroupVersionKind) (string, error) {
						if gvk.Group == "example.org" && gvk.Version == "v1" && gvk.Kind == "XResource" {
							return "xresources", nil
						}
						return "", errors.New("unexpected GVK in test")
					}).Build()

				return dynamicClient, mockConverter
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "XResource",
				},
			},
			want: want{
				crd: testCRD,
				err: nil,
			},
		},
		"CRDNotFound": {
			reason: "Should return error when CRD doesn't exist",
			setup: func() (dynamic.Interface, TypeConverter) {
				dynamicClient := fake.NewSimpleDynamicClient(scheme)
				dynamicClient.Fake.PrependReactor("get", "customresourcedefinitions", func(kt.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("CRD not found")
				})

				// Create mock type converter that returns "nonexistentresources"
				mockConverter := tu.NewMockTypeConverter().
					WithGetResourceNameForGVK(func(_ context.Context, gvk schema.GroupVersionKind) (string, error) {
						if gvk.Group == "example.org" && gvk.Version == "v1" && gvk.Kind == "NonexistentResource" {
							return "nonexistentresources", nil
						}
						return "", errors.New("unexpected GVK in test")
					}).Build()

				return dynamicClient, mockConverter
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "NonexistentResource",
				},
			},
			want: want{
				crd: nil,
				err: errors.New("cannot get CRD"),
			},
		},
		"TypeConverterError": {
			reason: "Should return error when type conversion fails",
			setup: func() (dynamic.Interface, TypeConverter) {
				dynamicClient := fake.NewSimpleDynamicClient(scheme)

				// Create mock type converter that returns an error
				mockConverter := tu.NewMockTypeConverter().
					WithGetResourceNameForGVK(func(context.Context, schema.GroupVersionKind) (string, error) {
						return "", errors.New("conversion error")
					}).Build()

				return dynamicClient, mockConverter
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "XResource",
				},
			},
			want: want{
				crd: nil,
				err: errors.New("cannot determine CRD name for"),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dynamicClient, converter := tc.setup()

			c := &DefaultSchemaClient{
				dynamicClient:   dynamicClient,
				typeConverter:   converter,
				logger:          tu.TestLogger(t, false),
				resourceTypeMap: make(map[schema.GroupVersionKind]bool),
			}

			crd, err := c.GetCRD(tc.args.ctx, tc.args.gvk)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nGetCRD(...): expected error but got none", tc.reason)
					return
				}

				if !strings.Contains(err.Error(), tc.want.err.Error()) {
					t.Errorf("\n%s\nGetCRD(...): expected error containing %q, got %q",
						tc.reason, tc.want.err.Error(), err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetCRD(...): unexpected error: %v", tc.reason, err)
				return
			}

			if diff := cmp.Diff(tc.want.crd, crd); diff != "" {
				t.Errorf("\n%s\nGetCRD(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSchemaClient_ValidateResource(t *testing.T) {
	ctx := context.Background()

	testCases := map[string]struct {
		resource *un.Unstructured
		wantErr  bool
	}{
		"SimpleValidResource": {
			resource: tu.NewResource("example.org/v1", "XResource", "test-resource").
				WithSpecField("field1", "value1").
				Build(),
			wantErr: false,
		},
		// You could add more tests here if the ValidateResource method had more logic,
		// but in the current implementation it's a no-op that always succeeds
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			c := &DefaultSchemaClient{
				logger:          tu.TestLogger(t, false),
				resourceTypeMap: make(map[schema.GroupVersionKind]bool),
			}

			err := c.ValidateResource(ctx, tc.resource)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateResource() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
