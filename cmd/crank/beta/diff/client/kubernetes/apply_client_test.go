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

var _ ApplyClient = (*tu.MockApplyClient)(nil)

func TestApplyClient_DryRunApply(t *testing.T) {
	scheme := runtime.NewScheme()

	type args struct {
		ctx context.Context
		obj *un.Unstructured
	}

	type want struct {
		result *un.Unstructured
		err    error
	}

	tests := map[string]struct {
		reason string
		setup  func() (dynamic.Interface, TypeConverter)
		args   args
		want   want
	}{
		"NamespacedResourceApplied": {
			reason: "Should successfully apply a namespaced resource",
			setup: func() (dynamic.Interface, TypeConverter) {
				obj := tu.NewResource("example.org/v1", "ExampleResource", "test-resource").
					InNamespace("test-namespace").
					WithSpecField("property", "new-value").
					Build()

				// Create dynamic client that returns the object with a resource version
				dynamicClient := fake.NewSimpleDynamicClient(scheme)
				// Add reactor to handle apply operation
				dynamicClient.Fake.PrependReactor("patch", "exampleresources", func(kt.Action) (bool, runtime.Object, error) {
					// For apply, we'd return the "server-modified" version
					result := obj.DeepCopy()
					result.SetResourceVersion("1000") // Server would set this
					return true, result, nil
				})

				// Create type converter
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
				obj: tu.NewResource("example.org/v1", "ExampleResource", "test-resource").
					InNamespace("test-namespace").
					WithSpecField("property", "new-value").
					Build(),
			},
			want: want{
				result: tu.NewResource("example.org/v1", "ExampleResource", "test-resource").
					InNamespace("test-namespace").
					WithSpecField("property", "new-value").
					Build(),
			},
		},
		"ClusterScopedResourceApplied": {
			reason: "Should successfully apply a cluster-scoped resource",
			setup: func() (dynamic.Interface, TypeConverter) {
				obj := tu.NewResource("example.org/v1", "ClusterResource", "test-cluster-resource").
					WithSpecField("property", "new-value").
					Build()

				// Create dynamic client that returns the object with a resource version
				dynamicClient := fake.NewSimpleDynamicClient(scheme)
				// Add reactor to handle apply operation
				dynamicClient.Fake.PrependReactor("patch", "clusterresources", func(kt.Action) (bool, runtime.Object, error) {
					// For apply, we'd return the "server-modified" version
					result := obj.DeepCopy()
					result.SetResourceVersion("1000") // Server would set this
					return true, result, nil
				})

				// Create type converter
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
				obj: tu.NewResource("example.org/v1", "ClusterResource", "test-cluster-resource").
					WithSpecField("property", "new-value").
					Build(),
			},
			want: want{
				result: tu.NewResource("example.org/v1", "ClusterResource", "test-cluster-resource").
					WithSpecField("property", "new-value").
					Build(),
			},
		},
		"ConverterError": {
			reason: "Should return error when GVK to GVR conversion fails",
			setup: func() (dynamic.Interface, TypeConverter) {
				dynamicClient := fake.NewSimpleDynamicClient(scheme)

				// Create type converter that returns an error
				mockConverter := tu.NewMockTypeConverter().
					WithGVKToGVR(func(context.Context, schema.GroupVersionKind) (schema.GroupVersionResource, error) {
						return schema.GroupVersionResource{}, errors.New("conversion error")
					}).Build()

				return dynamicClient, mockConverter
			},
			args: args{
				ctx: context.Background(),
				obj: tu.NewResource("example.org/v1", "ExampleResource", "test-resource").
					InNamespace("test-namespace").
					WithSpecField("property", "new-value").
					Build(),
			},
			want: want{
				err: errors.New("cannot perform dry-run apply for ExampleResource/test-resource"),
			},
		},
		"ApplyError": {
			reason: "Should return error when apply fails",
			setup: func() (dynamic.Interface, TypeConverter) {
				dynamicClient := fake.NewSimpleDynamicClient(scheme)
				// Add reactor to make apply fail
				dynamicClient.Fake.PrependReactor("patch", "exampleresources", func(kt.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("apply failed")
				})

				// Create type converter
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
				obj: tu.NewResource("example.org/v1", "ExampleResource", "test-resource").
					InNamespace("test-namespace").
					WithSpecField("property", "new-value").
					Build(),
			},
			want: want{
				err: errors.New("failed to apply resource"),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dynamicClient, converter := tc.setup()

			c := &DefaultApplyClient{
				dynamicClient: dynamicClient,
				typeConverter: converter,
				logger:        tu.TestLogger(t, false),
			}

			got, err := c.DryRunApply(tc.args.ctx, tc.args.obj)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nDryRunApply(...): expected error but got none", tc.reason)
					return
				}

				if !strings.Contains(err.Error(), tc.want.err.Error()) {
					t.Errorf("\n%s\nDryRunApply(...): expected error containing %q, got %q",
						tc.reason, tc.want.err.Error(), err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nDryRunApply(...): unexpected error: %v", tc.reason, err)
				return
			}

			// For successful cases, compare the original parts of results
			// We remove the resourceVersion before comparing since we set it in our test
			gotCopy := got.DeepCopy()
			if _, exists, _ := un.NestedString(gotCopy.Object, "metadata", "resourceVersion"); exists {
				un.RemoveNestedField(gotCopy.Object, "metadata", "resourceVersion")
			}

			wantCopy := tc.want.result.DeepCopy()
			if _, exists, _ := un.NestedString(wantCopy.Object, "metadata", "resourceVersion"); exists {
				un.RemoveNestedField(wantCopy.Object, "metadata", "resourceVersion")
			}

			if diff := cmp.Diff(wantCopy, gotCopy); diff != "" {
				t.Errorf("\n%s\nDryRunApply(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
