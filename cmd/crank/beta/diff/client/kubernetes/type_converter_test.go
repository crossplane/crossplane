package kubernetes

import (
	"context"
	"strings"
	"testing"

	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	testdiscovery "k8s.io/client-go/discovery/fake"
	kt "k8s.io/client-go/testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
)

var _ TypeConverter = (*tu.MockTypeConverter)(nil)

func TestTypeConverter_GVKToGVR(t *testing.T) {
	type args struct {
		ctx context.Context
		gvk schema.GroupVersionKind
	}

	type want struct {
		gvr schema.GroupVersionResource
		err error
	}

	tests := map[string]struct {
		reason    string
		resources []*metav1.APIResourceList
		args      args
		want      want
	}{
		"StandardResourceMapping": {
			reason: "Should correctly map a standard resource GVK to GVR",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "example.org/v1",
					APIResources: []metav1.APIResource{
						{
							Name:       "resources",
							Kind:       "Resource",
							Namespaced: true,
						},
					},
				},
			},

			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Resource",
				},
			},
			want: want{
				gvr: schema.GroupVersionResource{
					Group:    "example.org",
					Version:  "v1",
					Resource: "resources",
				},
			},
		},
		"NonStandardResourceMapping": {
			reason: "Should correctly map a non-standard pluralized resource",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "example.org/v1",
					APIResources: []metav1.APIResource{
						{
							Name:       "indices", // Non-standard pluralization
							Kind:       "Index",
							Namespaced: true,
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Index",
				},
			},
			want: want{
				gvr: schema.GroupVersionResource{
					Group:    "example.org",
					Version:  "v1",
					Resource: "indices",
				},
			},
		},
		"DiscoveryFailure": {
			reason:    "Should return error when discovery fails",
			resources: []*metav1.APIResourceList{},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Resource",
				},
			},
			want: want{
				err: errors.New("failed to discover resources for example.org/v1"),
			},
		},
		"ResourceNotFound": {
			reason: "Should return error when resource kind is not found",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "example.org/v1",
					APIResources: []metav1.APIResource{
						{
							Name:       "other-resources",
							Kind:       "OtherResource",
							Namespaced: true,
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Resource", // Not in discovery
				},
			},
			want: want{
				err: errors.New("no resource found for kind Resource in group version example.org/v1"),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			discoveryClient := &testdiscovery.FakeDiscovery{
				Fake: &kt.Fake{},
			}
			discoveryClient.Resources = tc.resources

			c := &DefaultTypeConverter{
				discoveryClient: discoveryClient,
				logger:          tu.TestLogger(t, false),
				gvkToGVRMap:     make(map[schema.GroupVersionKind]schema.GroupVersionResource),
			}

			gvr, err := c.GVKToGVR(tc.args.ctx, tc.args.gvk)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nGVKToGVR(...): expected error but got none", tc.reason)
					return
				}

				if !strings.Contains(err.Error(), tc.want.err.Error()) {
					t.Errorf("\n%s\nGVKToGVR(...): expected error containing %q, got %q",
						tc.reason, tc.want.err.Error(), err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGVKToGVR(...): unexpected error: %v", tc.reason, err)
				return
			}

			if gvr != tc.want.gvr {
				t.Errorf("\n%s\nGVKToGVR(...): -want GVR, +got GVR:\n%v vs %v",
					tc.reason, tc.want.gvr, gvr)
			}
		})
	}
}

func TestTypeConverter_GetResourceNameForGVK(t *testing.T) {
	type args struct {
		ctx context.Context
		gvk schema.GroupVersionKind
	}

	type want struct {
		resourceName string
		err          error
	}

	tests := map[string]struct {
		reason    string
		resources []*metav1.APIResourceList
		args      args
		want      want
	}{
		"StandardResource": {
			reason: "Should correctly get resource name for a standard resource",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "example.org/v1",
					APIResources: []metav1.APIResource{
						{
							Name:       "resources",
							Kind:       "Resource",
							Namespaced: true,
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Resource",
				},
			},
			want: want{
				resourceName: "resources",
			},
		},
		"NonStandardPluralization": {
			reason: "Should correctly get resource name for a non-standard pluralized resource",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "example.org/v1",
					APIResources: []metav1.APIResource{
						{
							Name:       "indices", // Non-standard pluralization
							Kind:       "Index",
							Namespaced: true,
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Index",
				},
			},
			want: want{
				resourceName: "indices",
			},
		},
		"MultipleResourcesForKind": {
			reason: "Should get the first resource name when multiple resources exist for the same kind",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "example.org/v1",
					APIResources: []metav1.APIResource{
						{
							Name:       "resources",
							Kind:       "Resource",
							Namespaced: true,
						},
						{
							Name:       "resources/status",
							Kind:       "Resource", // Same kind, subresource
							Namespaced: true,
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Resource",
				},
			},
			want: want{
				resourceName: "resources", // Gets the first one
			},
		},
		"DiscoveryFailure": {
			reason:    "Should return error when discovery fails",
			resources: []*metav1.APIResourceList{},

			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Resource",
				},
			},
			want: want{
				err: errors.New("failed to discover resources for example.org/v1"),
			},
		},
		"ResourceNotFound": {
			reason: "Should return error when resource kind is not found",
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "example.org/v1",
					APIResources: []metav1.APIResource{
						{
							Name:       "other-resources",
							Kind:       "OtherResource",
							Namespaced: true,
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Resource", // Not in discovery
				},
			},
			want: want{
				err: errors.New("no resource found for kind Resource in group version example.org/v1"),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			discoveryClient := &testdiscovery.FakeDiscovery{
				Fake: &kt.Fake{},
			}
			discoveryClient.Resources = tc.resources

			c := &DefaultTypeConverter{
				discoveryClient: discoveryClient,
				logger:          tu.TestLogger(t, false),
				gvkToGVRMap:     make(map[schema.GroupVersionKind]schema.GroupVersionResource),
			}

			resourceName, err := c.GetResourceNameForGVK(tc.args.ctx, tc.args.gvk)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nGetResourceNameForGVK(...): expected error but got none", tc.reason)
					return
				}

				if !strings.Contains(err.Error(), tc.want.err.Error()) {
					t.Errorf("\n%s\nGetResourceNameForGVK(...): expected error containing %q, got %q",
						tc.reason, tc.want.err.Error(), err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetResourceNameForGVK(...): unexpected error: %v", tc.reason, err)
				return
			}

			if resourceName != tc.want.resourceName {
				t.Errorf("\n%s\nGetResourceNameForGVK(...): want %q, got %q",
					tc.reason, tc.want.resourceName, resourceName)
			}
		})
	}
}
