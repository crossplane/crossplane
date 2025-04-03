package kubernetes

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	kt "k8s.io/client-go/testing"
	"strings"
	"testing"
)

func TestSchemaClient_IsCRDRequired(t *testing.T) {
	// Set up context for tests
	ctx := context.Background()

	tests := map[string]struct {
		reason string
		setup  func() discovery.DiscoveryInterface
		gvk    schema.GroupVersionKind
		want   bool
	}{
		"CoreResource": {
			reason: "Core API resources (group='') should not require a CRD",
			setup: func() discovery.DiscoveryInterface {
				// Create a fake discovery client that returns core API resources
				fakeDiscovery := &fakediscovery.FakeDiscovery{
					Fake: &kt.Fake{},
				}
				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: "v1",
						APIResources: []metav1.APIResource{
							{
								Name: "pods",
								Kind: "Pod",
							},
						},
					},
				}
				return fakeDiscovery
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
			setup: func() discovery.DiscoveryInterface {
				// Create a fake discovery client that returns Kubernetes extension resources
				fakeDiscovery := &fakediscovery.FakeDiscovery{
					Fake: &kt.Fake{},
				}
				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: "apps/v1",
						APIResources: []metav1.APIResource{
							{
								Name: "deployments",
								Kind: "Deployment",
							},
						},
					},
				}
				return fakeDiscovery
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
			setup: func() discovery.DiscoveryInterface {
				// Create a fake discovery client with knowledge of the custom resource
				fakeDiscovery := &fakediscovery.FakeDiscovery{
					Fake: &kt.Fake{},
				}
				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: "example.org/v1",
						APIResources: []metav1.APIResource{
							{
								Name: "xresources",
								Kind: "XResource",
							},
						},
					},
				}
				return fakeDiscovery
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
			setup: func() discovery.DiscoveryInterface {
				// Create a fake discovery client that returns apiextensions resources
				fakeDiscovery := &fakediscovery.FakeDiscovery{
					Fake: &kt.Fake{},
				}
				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: "apiextensions.k8s.io/v1",
						APIResources: []metav1.APIResource{
							{
								Name: "customresourcedefinitions",
								Kind: "CustomResourceDefinition",
							},
						},
					},
				}
				return fakeDiscovery
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
			setup: func() discovery.DiscoveryInterface {
				// Create a fake discovery client that returns other k8s.io resources
				fakeDiscovery := &fakediscovery.FakeDiscovery{
					Fake: &kt.Fake{},
				}
				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: "networking.k8s.io/v1",
						APIResources: []metav1.APIResource{
							{
								Name: "networkpolicies",
								Kind: "NetworkPolicy",
							},
						},
					},
				}
				return fakeDiscovery
			},
			gvk: schema.GroupVersionKind{
				Group:   "networking.k8s.io",
				Version: "v1",
				Kind:    "NetworkPolicy",
			},
			want: false, // Other k8s.io resources should not require a CRD
		},
		"DiscoveryFailure": {
			reason: "If discovery fails, should default to requiring a CRD",
			setup: func() discovery.DiscoveryInterface {
				// Create a fake discovery client that returns an error for any request
				fakeDiscovery := &fakediscovery.FakeDiscovery{
					Fake: &kt.Fake{},
				}
				// Empty resources list will cause "not found" error
				fakeDiscovery.Resources = []*metav1.APIResourceList{}
				return fakeDiscovery
			},
			gvk: schema.GroupVersionKind{
				Group:   "example.org",
				Version: "v1",
				Kind:    "XResource",
			},
			want: true, // Default to requiring CRD on discovery failure
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create a schema client with the test discovery client
			c := &DefaultSchemaClient{
				discoveryClient: tc.setup(),
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
		setup  func() (dynamic.Interface, discovery.DiscoveryInterface)
		args   args
		want   want
	}{
		"SuccessfulCRDRetrieval": {
			reason: "Should retrieve CRD when it exists",
			setup: func() (dynamic.Interface, discovery.DiscoveryInterface) {
				// Set up the dynamic client to return our test CRD
				dynamicClient := fake.NewSimpleDynamicClient(scheme)
				dynamicClient.Fake.PrependReactor("get", "customresourcedefinitions", func(action kt.Action) (bool, runtime.Object, error) {
					getAction := action.(kt.GetAction)
					if getAction.GetName() == "xresources.example.org" {
						return true, testCRD, nil
					}
					return false, nil, nil
				})

				// Create fake discovery client with resource information
				fakeDiscovery := &fakediscovery.FakeDiscovery{
					Fake: &kt.Fake{},
				}
				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: "example.org/v1",
						APIResources: []metav1.APIResource{
							{
								Name: "xresources",
								Kind: "XResource",
							},
						},
					},
				}

				return dynamicClient, fakeDiscovery
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
			setup: func() (dynamic.Interface, discovery.DiscoveryInterface) {
				dynamicClient := fake.NewSimpleDynamicClient(scheme)
				dynamicClient.Fake.PrependReactor("get", "customresourcedefinitions", func(kt.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("CRD not found")
				})

				// Create fake discovery client with resource information
				fakeDiscovery := &fakediscovery.FakeDiscovery{
					Fake: &kt.Fake{},
				}
				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: "example.org/v1",
						APIResources: []metav1.APIResource{
							{
								Name: "nonexistentresources",
								Kind: "NonexistentResource",
							},
						},
					},
				}

				return dynamicClient, fakeDiscovery
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
		"DiscoveryFailure": {
			reason: "Should return error when discovery fails",
			setup: func() (dynamic.Interface, discovery.DiscoveryInterface) {
				dynamicClient := fake.NewSimpleDynamicClient(scheme)

				// Create fake discovery client that returns an error
				fakeDiscovery := &fakediscovery.FakeDiscovery{
					Fake: &kt.Fake{},
				}
				// Empty resources list will cause "not found" error
				fakeDiscovery.Resources = []*metav1.APIResourceList{}

				return dynamicClient, fakeDiscovery
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
				err: errors.New("failed to discover resources for example.org/v1"),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dynamicClient, discoveryClient := tc.setup()

			c := &DefaultSchemaClient{
				dynamicClient:   dynamicClient,
				discoveryClient: discoveryClient,
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
