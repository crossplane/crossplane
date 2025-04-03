package kubernetes

import (
	"context"
	"strings"
	"testing"

	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	kt "k8s.io/client-go/testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
)

var _ ResourceClient = (*tu.MockResourceClient)(nil)

func TestResourceClient_GetResource(t *testing.T) {
	scheme := runtime.NewScheme()

	type args struct {
		ctx       context.Context
		gvk       schema.GroupVersionKind
		namespace string
		name      string
	}

	type want struct {
		resource *un.Unstructured
		err      error
	}

	tests := map[string]struct {
		reason string
		setup  func() (dynamic.Interface, TypeConverter)
		args   args
		want   want
	}{
		"NamespacedResourceFound": {
			reason: "Should return the resource when it exists in a namespace",
			setup: func() (dynamic.Interface, TypeConverter) {
				// Use the resource builder to create test objects
				objects := []runtime.Object{
					tu.NewResource("example.org/v1", "ExampleResource", "test-resource").
						InNamespace("test-namespace").
						WithSpecField("property", "value").
						Build(),
				}

				dynamicClient := fake.NewSimpleDynamicClient(scheme, objects...)

				// Create mock type converter using the builder
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(_ context.Context, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						return schema.GroupVersionResource{
							Group:    gvk.Group,
							Version:  gvk.Version,
							Resource: "exampleresources",
						}, nil
					}).Build()

				return dynamicClient, mockConverter
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "ExampleResource",
				},
				namespace: "test-namespace",
				name:      "test-resource",
			},
			want: want{
				resource: tu.NewResource("example.org/v1", "ExampleResource", "test-resource").
					InNamespace("test-namespace").
					WithSpecField("property", "value").
					Build(),
			},
		},
		"ClusterScopedResourceFound": {
			reason: "Should return the resource when it exists at cluster scope",
			setup: func() (dynamic.Interface, TypeConverter) {
				objects := []runtime.Object{
					tu.NewResource("example.org/v1", "ClusterResource", "test-cluster-resource").
						WithSpecField("property", "value").
						Build(),
				}

				dynamicClient := fake.NewSimpleDynamicClient(scheme, objects...)

				// Create mock type converter using the builder
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(_ context.Context, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						return schema.GroupVersionResource{
							Group:    gvk.Group,
							Version:  gvk.Version,
							Resource: "clusterresources",
						}, nil
					}).Build()

				return dynamicClient, mockConverter
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "ClusterResource",
				},
				namespace: "", // Cluster-scoped
				name:      "test-cluster-resource",
			},
			want: want{
				resource: tu.NewResource("example.org/v1", "ClusterResource", "test-cluster-resource").
					WithSpecField("property", "value").
					Build(),
			},
		},
		"ResourceNotFound": {
			reason: "Should return an error when the resource doesn't exist",
			setup: func() (dynamic.Interface, TypeConverter) {
				dc := fake.NewSimpleDynamicClient(scheme)
				dc.Fake.PrependReactor("get", "*", func(kt.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("resource not found")
				})

				// Create mock type converter using the builder
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(_ context.Context, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						return schema.GroupVersionResource{
							Group:    gvk.Group,
							Version:  gvk.Version,
							Resource: "exampleresources",
						}, nil
					}).Build()

				return dc, mockConverter
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "ExampleResource",
				},
				namespace: "test-namespace",
				name:      "nonexistent-resource",
			},
			want: want{
				resource: nil,
				err:      errors.New("cannot get resource test-namespace/nonexistent-resource of kind ExampleResource"),
			},
		},
		"ConverterError": {
			reason: "Should return an error when GVK to GVR conversion fails",
			setup: func() (dynamic.Interface, TypeConverter) {
				dynamicClient := fake.NewSimpleDynamicClient(scheme)

				// Create mock type converter that returns an error
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(context.Context, schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						return schema.GroupVersionResource{}, errors.New("conversion error")
					}).Build()

				return dynamicClient, mockConverter
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "ExampleResource",
				},
				namespace: "test-namespace",
				name:      "test-resource",
			},
			want: want{
				resource: nil,
				err:      errors.New("cannot get resource test-namespace/test-resource of kind ExampleResource"),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dynamicClient, converter := tc.setup()

			c := &DefaultResourceClient{
				dynamicClient: dynamicClient,
				converter:     converter,
				logger:        tu.TestLogger(t, false),
			}

			got, err := c.GetResource(tc.args.ctx, tc.args.gvk, tc.args.namespace, tc.args.name)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nGetResource(...): expected error but got none", tc.reason)
					return
				}

				if !strings.Contains(err.Error(), tc.want.err.Error()) {
					t.Errorf("\n%s\nGetResource(...): expected error containing %q, got %q",
						tc.reason, tc.want.err.Error(), err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetResource(...): unexpected error: %v", tc.reason, err)
				return
			}

			// Remove resourceVersion from comparison since it's added by the fake client
			gotCopy := got.DeepCopy()
			if gotCopy != nil && gotCopy.Object != nil {
				meta, found, _ := un.NestedMap(gotCopy.Object, "metadata")
				if found && meta != nil {
					delete(meta, "resourceVersion")
					_ = un.SetNestedMap(gotCopy.Object, meta, "metadata")
				}
			}

			if diff := cmp.Diff(tc.want.resource, gotCopy); diff != "" {
				t.Errorf("\n%s\nGetResource(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestResourceClient_GetResourcesByLabel(t *testing.T) {
	scheme := runtime.NewScheme()

	tests := map[string]struct {
		reason string
		setup  func() (dynamic.Interface, TypeConverter)
		args   struct {
			ctx       context.Context
			namespace string
			gvk       schema.GroupVersionKind
			selector  metav1.LabelSelector
		}
		want struct {
			resources []*un.Unstructured
			err       error
		}
	}{
		"NoMatchingResources": {
			reason: "Should return empty list when no resources match selector",
			setup: func() (dynamic.Interface, TypeConverter) {
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "example.org", Version: "v1", Resource: "resources"}: "ResourceList",
					})

				// Create mock type converter using the builder
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(_ context.Context, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						return schema.GroupVersionResource{
							Group:    gvk.Group,
							Version:  gvk.Version,
							Resource: "resources",
						}, nil
					}).Build()

				return dc, mockConverter
			},
			args: struct {
				ctx       context.Context
				namespace string
				gvk       schema.GroupVersionKind
				selector  metav1.LabelSelector
			}{
				ctx:       context.Background(),
				namespace: "test-namespace",
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Resource",
				},
				selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
			},
			want: struct {
				resources []*un.Unstructured
				err       error
			}{
				resources: []*un.Unstructured{},
			},
		},
		"MatchingResources": {
			reason: "Should return resources matching label selector",
			setup: func() (dynamic.Interface, TypeConverter) {
				// Use resource builders for cleaner test objects
				objects := []runtime.Object{
					// Resource that matches our selector
					tu.NewResource("example.org/v1", "Resource", "matched-resource-1").
						InNamespace("test-namespace").
						WithLabels(map[string]string{
							"app": "test",
							"env": "dev",
						}).
						Build(),

					// Resource that matches our selector with different labels
					tu.NewResource("example.org/v1", "Resource", "matched-resource-2").
						InNamespace("test-namespace").
						WithLabels(map[string]string{
							"app": "test",
							"env": "prod",
						}).
						Build(),

					// Resource that doesn't match our selector
					tu.NewResource("example.org/v1", "Resource", "unmatched-resource").
						InNamespace("test-namespace").
						WithLabels(map[string]string{
							"app": "other",
						}).
						Build(),
				}

				dc := fake.NewSimpleDynamicClient(scheme, objects...)

				// Create mock type converter using the builder
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(_ context.Context, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						return schema.GroupVersionResource{
							Group:    gvk.Group,
							Version:  gvk.Version,
							Resource: "resources",
						}, nil
					}).Build()

				return dc, mockConverter
			},
			args: struct {
				ctx       context.Context
				namespace string
				gvk       schema.GroupVersionKind
				selector  metav1.LabelSelector
			}{
				ctx:       context.Background(),
				namespace: "test-namespace",
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Resource",
				},
				selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
			},
			want: struct {
				resources []*un.Unstructured
				err       error
			}{
				resources: []*un.Unstructured{
					// Expected matching resources using builders
					tu.NewResource("example.org/v1", "Resource", "matched-resource-1").
						InNamespace("test-namespace").
						WithLabels(map[string]string{
							"app": "test",
							"env": "dev",
						}).
						Build(),
					tu.NewResource("example.org/v1", "Resource", "matched-resource-2").
						InNamespace("test-namespace").
						WithLabels(map[string]string{
							"app": "test",
							"env": "prod",
						}).
						Build(),
				},
			},
		},
		"ListError": {
			reason: "Should propagate errors from the Kubernetes API",
			setup: func() (dynamic.Interface, TypeConverter) {
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "example.org", Version: "v1", Resource: "resources"}: "ResourceList",
					})

				dc.Fake.PrependReactor("list", "resources", func(kt.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("list error")
				})

				// Create mock type converter using the builder
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(_ context.Context, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						return schema.GroupVersionResource{
							Group:    gvk.Group,
							Version:  gvk.Version,
							Resource: "resources",
						}, nil
					}).Build()

				return dc, mockConverter
			},
			args: struct {
				ctx       context.Context
				namespace string
				gvk       schema.GroupVersionKind
				selector  metav1.LabelSelector
			}{
				ctx:       context.Background(),
				namespace: "test-namespace",
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Resource",
				},
				selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
			},
			want: struct {
				resources []*un.Unstructured
				err       error
			}{
				err: errors.New("cannot list resources for 'example.org/v1, Kind=Resource' matching"),
			},
		},
		"ConverterError": {
			reason: "Should propagate errors from the type converter",
			setup: func() (dynamic.Interface, TypeConverter) {
				dc := fake.NewSimpleDynamicClient(scheme)

				// Create mock type converter that returns an error
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(context.Context, schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						return schema.GroupVersionResource{}, errors.New("conversion error")
					}).Build()

				return dc, mockConverter
			},
			args: struct {
				ctx       context.Context
				namespace string
				gvk       schema.GroupVersionKind
				selector  metav1.LabelSelector
			}{
				ctx:       context.Background(),
				namespace: "test-namespace",
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Resource",
				},
				selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
			},
			want: struct {
				resources []*un.Unstructured
				err       error
			}{
				err: errors.New("cannot list resources for 'example.org/v1, Kind=Resource' matching labels"),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dynamicClient, converter := tc.setup()

			c := &DefaultResourceClient{
				dynamicClient: dynamicClient,
				converter:     converter,
				logger:        tu.TestLogger(t, false),
			}

			got, err := c.GetResourcesByLabel(tc.args.ctx, tc.args.namespace, tc.args.gvk, tc.args.selector)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nGetResourcesByLabel(...): expected error but got none", tc.reason)
					return
				}

				// Check that the error contains the expected message
				if !strings.Contains(err.Error(), tc.want.err.Error()) {
					t.Errorf("\n%s\nGetResourcesByLabel(...): expected error containing %q, got: %v",
						tc.reason, tc.want.err.Error(), err)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetResourcesByLabel(...): unexpected error: %v", tc.reason, err)
				return
			}

			if diff := cmp.Diff(len(tc.want.resources), len(got)); diff != "" {
				t.Errorf("\n%s\nGetResourcesByLabel(...): -want resource count, +got resource count:\n%s", tc.reason, diff)
			}

			// Compare resources by name to handle ordering differences
			wantResources := make(map[string]bool)
			for _, res := range tc.want.resources {
				wantResources[res.GetName()] = true
			}

			for _, gotRes := range got {
				if !wantResources[gotRes.GetName()] {
					t.Errorf("\n%s\nGetResourcesByLabel(...): unexpected resource: %s", tc.reason, gotRes.GetName())
				}
			}

			// Also check if any expected resources are missing
			gotResources := make(map[string]bool)
			for _, res := range got {
				gotResources[res.GetName()] = true
			}

			for _, wantRes := range tc.want.resources {
				if !gotResources[wantRes.GetName()] {
					t.Errorf("\n%s\nGetResourcesByLabel(...): missing expected resource: %s", tc.reason, wantRes.GetName())
				}
			}
		})
	}
}

func TestResourceClient_GetAllResourcesByLabels(t *testing.T) {
	scheme := runtime.NewScheme()

	type args struct {
		ctx       context.Context
		gvks      []schema.GroupVersionKind
		selectors []metav1.LabelSelector
	}

	type want struct {
		resources []*un.Unstructured
		err       error
	}

	cases := map[string]struct {
		reason string
		setup  func() (dynamic.Interface, TypeConverter)
		args   args
		want   want
	}{
		"MismatchedGVKsAndSelectors": {
			reason: "Should return error when GVKs and selectors count mismatch",
			setup: func() (dynamic.Interface, TypeConverter) {
				return fake.NewSimpleDynamicClient(scheme), tu.NewMockTypeConverter().Build()
			},
			args: args{
				ctx: context.Background(),
				gvks: []schema.GroupVersionKind{
					{Group: "example.org", Version: "v1", Kind: "Resource"},
				},
				selectors: []metav1.LabelSelector{},
			},
			want: want{
				err: errors.New("number of GVKs must match number of selectors"),
			},
		},
		"NoMatchingResources": {
			reason: "Should return empty list when no resources match selector",
			setup: func() (dynamic.Interface, TypeConverter) {
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "example.org", Version: "v1", Resource: "resources"}: "ResourceList",
					})

				// Create mock type converter
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(_ context.Context, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						return schema.GroupVersionResource{
							Group:    gvk.Group,
							Version:  gvk.Version,
							Resource: strings.ToLower(gvk.Kind) + "s",
						}, nil
					}).Build()

				return dc, mockConverter
			},
			args: args{
				ctx: context.Background(),
				gvks: []schema.GroupVersionKind{
					{Group: "example.org", Version: "v1", Kind: "Resource"},
				},
				selectors: []metav1.LabelSelector{
					{
						MatchLabels: map[string]string{"app": "test"},
					},
				},
			},
			want: want{
				resources: []*un.Unstructured{},
			},
		},
		"MatchingResources": {
			reason: "Should return resources matching selector",
			setup: func() (dynamic.Interface, TypeConverter) {
				objects := []runtime.Object{
					// Use resource builders for the test objects
					tu.NewResource("example.org/v1", "Resource", "res1").
						WithLabels(map[string]string{
							"app": "test",
							"env": "dev",
						}).
						Build(),
					tu.NewResource("example.org/v1", "Resource", "res2").
						WithLabels(map[string]string{
							"app": "other",
						}).
						Build(),
					tu.NewResource("example.org/v2", "OtherResource", "other1").
						WithLabels(map[string]string{
							"type": "test",
						}).
						Build(),
				}

				dc := fake.NewSimpleDynamicClient(scheme, objects...)

				// Create mock type converter
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(_ context.Context, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						resource := ""
						switch gvk.Kind {
						case "Resource":
							resource = "resources"
						case "OtherResource":
							resource = "otherresources"
						}
						return schema.GroupVersionResource{
							Group:    gvk.Group,
							Version:  gvk.Version,
							Resource: resource,
						}, nil
					}).Build()

				return dc, mockConverter
			},
			args: args{
				ctx: context.Background(),
				gvks: []schema.GroupVersionKind{
					{Group: "example.org", Version: "v1", Kind: "Resource"},
					{Group: "example.org", Version: "v2", Kind: "OtherResource"},
				},
				selectors: []metav1.LabelSelector{
					{
						MatchLabels: map[string]string{"app": "test"},
					},
					{
						MatchLabels: map[string]string{"type": "test"},
					},
				},
			},
			want: want{
				resources: []*un.Unstructured{
					tu.NewResource("example.org/v1", "Resource", "res1").
						WithLabels(map[string]string{
							"app": "test",
							"env": "dev",
						}).
						Build(),
					tu.NewResource("example.org/v2", "OtherResource", "other1").
						WithLabels(map[string]string{
							"type": "test",
						}).
						Build(),
				},
			},
		},
		"ListError": {
			reason: "Should propagate errors from the Kubernetes API",
			setup: func() (dynamic.Interface, TypeConverter) {
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "example.org", Version: "v1", Resource: "resources"}: "ResourceList",
					})
				dc.Fake.PrependReactor("list", "resources", func(kt.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("list error")
				})

				// Create mock type converter
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(_ context.Context, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						return schema.GroupVersionResource{
							Group:    gvk.Group,
							Version:  gvk.Version,
							Resource: "resources",
						}, nil
					}).Build()

				return dc, mockConverter
			},
			args: args{
				ctx: context.Background(),
				gvks: []schema.GroupVersionKind{
					{Group: "example.org", Version: "v1", Kind: "Resource"},
				},
				selectors: []metav1.LabelSelector{
					{
						MatchLabels: map[string]string{"app": "test"},
					},
				},
			},
			want: want{
				err: errors.New("cannot get all resources"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dynamicClient, converter := tc.setup()

			c := &DefaultResourceClient{
				dynamicClient: dynamicClient,
				converter:     converter,
				logger:        tu.TestLogger(t, false),
			}

			got, err := c.GetAllResourcesByLabels(tc.args.ctx, tc.args.gvks, tc.args.selectors)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nGetAllResourcesByLabels(...): expected error but got none", tc.reason)
					return
				}

				if !strings.Contains(err.Error(), tc.want.err.Error()) {
					t.Errorf("\n%s\nGetAllResourcesByLabels(...): expected error containing %q, got %q",
						tc.reason, tc.want.err.Error(), err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetAllResourcesByLabels(...): unexpected error: %v", tc.reason, err)
				return
			}

			if diff := cmp.Diff(len(tc.want.resources), len(got)); diff != "" {
				t.Errorf("\n%s\nGetAllResourcesByLabels(...): -want resource count, +got resource count:\n%s", tc.reason, diff)
			}

			// For successful cases, compare results
			// Create maps of resource names for easier comparison
			wantResources := make(map[string]bool)
			gotResources := make(map[string]bool)

			for _, res := range tc.want.resources {
				wantResources[res.GetName()] = true
			}

			for _, res := range got {
				gotResources[res.GetName()] = true
			}

			// Check for missing resources
			for name := range wantResources {
				if !gotResources[name] {
					t.Errorf("\n%s\nGetAllResourcesByLabels(...): missing expected resource: %s", tc.reason, name)
				}
			}

			// Check for unexpected resources
			for name := range gotResources {
				if !wantResources[name] {
					t.Errorf("\n%s\nGetAllResourcesByLabels(...): unexpected resource: %s", tc.reason, name)
				}
			}
		})
	}
}

func TestResourceClient_ListResources(t *testing.T) {
	scheme := runtime.NewScheme()

	type args struct {
		ctx       context.Context
		gvk       schema.GroupVersionKind
		namespace string
	}

	type want struct {
		resources []*un.Unstructured
		err       error
	}

	tests := map[string]struct {
		reason string
		setup  func() (dynamic.Interface, TypeConverter)
		args   args
		want   want
	}{
		"NoResources": {
			reason: "Should return empty list when no resources exist",
			setup: func() (dynamic.Interface, TypeConverter) {
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "example.org", Version: "v1", Resource: "resources"}: "ResourceList",
					})

				// Create mock type converter
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(_ context.Context, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						return schema.GroupVersionResource{
							Group:    gvk.Group,
							Version:  gvk.Version,
							Resource: "resources",
						}, nil
					}).Build()

				return dc, mockConverter
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Resource",
				},
				namespace: "",
			},
			want: want{
				resources: []*un.Unstructured{},
			},
		},
		"ResourcesExist": {
			reason: "Should return all resources when they exist",
			setup: func() (dynamic.Interface, TypeConverter) {
				objects := []runtime.Object{
					tu.NewResource("example.org/v1", "Resource", "res1").
						InNamespace("test-namespace").
						WithSpecField("field1", "value1").
						Build(),
					tu.NewResource("example.org/v1", "Resource", "res2").
						InNamespace("test-namespace").
						WithSpecField("field2", "value2").
						Build(),
				}

				dc := fake.NewSimpleDynamicClient(scheme, objects...)

				// Create mock type converter
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(_ context.Context, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						return schema.GroupVersionResource{
							Group:    gvk.Group,
							Version:  gvk.Version,
							Resource: "resources",
						}, nil
					}).Build()

				return dc, mockConverter
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Resource",
				},
				namespace: "test-namespace",
			},
			want: want{
				resources: []*un.Unstructured{
					tu.NewResource("example.org/v1", "Resource", "res1").
						InNamespace("test-namespace").
						WithSpecField("field1", "value1").
						Build(),
					tu.NewResource("example.org/v1", "Resource", "res2").
						InNamespace("test-namespace").
						WithSpecField("field2", "value2").
						Build(),
				},
			},
		},
		"ListError": {
			reason: "Should propagate errors from API server",
			setup: func() (dynamic.Interface, TypeConverter) {
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "example.org", Version: "v1", Resource: "resources"}: "ResourceList",
					})

				dc.Fake.PrependReactor("list", "resources", func(kt.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("list error")
				})

				// Create mock type converter
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(_ context.Context, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						return schema.GroupVersionResource{
							Group:    gvk.Group,
							Version:  gvk.Version,
							Resource: "resources",
						}, nil
					}).Build()

				return dc, mockConverter
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Resource",
				},
				namespace: "test-namespace",
			},
			want: want{
				err: errors.New("cannot list resources for 'example.org/v1, Kind=Resource'"),
			},
		},
		"ConverterError": {
			reason: "Should propagate errors from type converter",
			setup: func() (dynamic.Interface, TypeConverter) {
				dc := fake.NewSimpleDynamicClient(scheme)

				// Create mock type converter that returns an error
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(context.Context, schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						return schema.GroupVersionResource{}, errors.New("conversion error")
					}).Build()

				return dc, mockConverter
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Resource",
				},
				namespace: "test-namespace",
			},
			want: want{
				err: errors.New("cannot list resources for 'example.org/v1, Kind=Resource'"),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dynamicClient, converter := tc.setup()

			c := &DefaultResourceClient{
				dynamicClient: dynamicClient,
				converter:     converter,
				logger:        tu.TestLogger(t, false),
			}

			got, err := c.ListResources(tc.args.ctx, tc.args.gvk, tc.args.namespace)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nListResources(...): expected error but got none", tc.reason)
					return
				}

				if !strings.Contains(err.Error(), tc.want.err.Error()) {
					t.Errorf("\n%s\nListResources(...): expected error containing %q, got %q",
						tc.reason, tc.want.err.Error(), err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nListResources(...): unexpected error: %v", tc.reason, err)
				return
			}

			if diff := cmp.Diff(len(tc.want.resources), len(got)); diff != "" {
				t.Errorf("\n%s\nListResources(...): -want resource count, +got resource count:\n%s", tc.reason, diff)
			}

			// Create maps of resource names for easier comparison
			wantResources := make(map[string]bool)
			gotResources := make(map[string]bool)

			for _, res := range tc.want.resources {
				wantResources[res.GetName()] = true
			}

			for _, res := range got {
				gotResources[res.GetName()] = true
			}

			// Check for missing resources
			for name := range wantResources {
				if !gotResources[name] {
					t.Errorf("\n%s\nListResources(...): missing expected resource: %s", tc.reason, name)
				}
			}

			// Check for unexpected resources
			for name := range gotResources {
				if !wantResources[name] {
					t.Errorf("\n%s\nListResources(...): unexpected resource: %s", tc.reason, name)
				}
			}
		})
	}
}
