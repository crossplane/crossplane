package clusterclient

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	kt "k8s.io/client-go/testing"
)

func TestClusterClient_GetEnvironmentConfigs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = pkgv1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	type args struct {
		ctx context.Context
	}

	type want struct {
		envConfigs []unstructured.Unstructured
		err        error
	}

	cases := map[string]struct {
		reason string
		setup  func() dynamic.Interface
		args   args
		want   want
	}{
		"NoConfigs": {
			reason: "Should return empty list when no configs exist",
			setup: func() dynamic.Interface {
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "apiextensions.crossplane.io", Version: "v1alpha1", Resource: "environmentconfigs"}: "EnvironmentConfigList",
					})
				return dc
			},
			args: args{
				ctx: context.Background(),
			},
			want: want{
				envConfigs: []unstructured.Unstructured{},
			},
		},
		"AllConfigs": {
			reason: "Should return all configs when they exist",
			setup: func() dynamic.Interface {
				objects := []runtime.Object{
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.crossplane.io/v1alpha1",
							"kind":       "EnvironmentConfig",
							"metadata": map[string]interface{}{
								"name": "config1",
							},
							"data": map[string]interface{}{
								"key": "value1",
							},
						},
					},
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.crossplane.io/v1alpha1",
							"kind":       "EnvironmentConfig",
							"metadata": map[string]interface{}{
								"name": "config2",
							},
							"data": map[string]interface{}{
								"key": "value2",
							},
						},
					},
				}
				return fake.NewSimpleDynamicClient(scheme, objects...)
			},
			args: args{
				ctx: context.Background(),
			},
			want: want{
				envConfigs: []unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.crossplane.io/v1alpha1",
							"kind":       "EnvironmentConfig",
							"metadata": map[string]interface{}{
								"name": "config1",
							},
							"data": map[string]interface{}{
								"key": "value1",
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.crossplane.io/v1alpha1",
							"kind":       "EnvironmentConfig",
							"metadata": map[string]interface{}{
								"name": "config2",
							},
							"data": map[string]interface{}{
								"key": "value2",
							},
						},
					},
				},
			},
		},
		"APIError": {
			reason: "Should propagate errors from the Kubernetes API",
			setup: func() dynamic.Interface {
				// Create a client with the exact GVR that will be used
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "apiextensions.crossplane.io", Version: "v1alpha1", Resource: "environmentconfigs"}: "EnvironmentConfigList",
					})

				// Add reactor that will respond to this exact GVR
				dc.Fake.PrependReactor("list", "environmentconfigs", func(action kt.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("api server down")
				})

				return dc
			},
			args: args{
				ctx: context.Background(),
			},
			want: want{
				envConfigs: nil,
				err:        errors.Wrap(errors.New("api server down"), "cannot list environment configs"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &ClusterClient{
				dynamicClient: tc.setup(),
			}

			got, err := c.GetEnvironmentConfigs(tc.args.ctx)

			if tc.want.err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nInitialize(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nInitialize(...): unexpected error: %v", tc.reason, err)
				return
			}

			if diff := cmp.Diff(tc.want.envConfigs, got); diff != "" {
				t.Errorf("\n%s\nGetEnvironmentConfigs(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestClusterClient_Initialize(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = pkgv1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	type args struct {
		ctx context.Context
	}

	type want struct {
		compositions map[compositionCacheKey]*apiextensionsv1.Composition
		functions    map[string]pkgv1.Function
		err          error
	}

	cases := map[string]struct {
		reason string
		setup  func() dynamic.Interface
		args   args
		want   want
	}{
		"NoCompositionsOrFunctions": {
			reason: "Should initialize with empty maps when no resources exist",
			setup: func() dynamic.Interface {
				return fake.NewSimpleDynamicClient(scheme)
			},
			args: args{
				ctx: context.Background(),
			},
			want: want{
				compositions: map[compositionCacheKey]*apiextensionsv1.Composition{},
				functions:    map[string]pkgv1.Function{},
			},
		},
		"WithCompositionsAndFunctions": {
			reason: "Should initialize with compositions and functions when they exist",
			setup: func() dynamic.Interface {
				objects := []runtime.Object{
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.crossplane.io/v1",
							"kind":       "Composition",
							"metadata": map[string]interface{}{
								"name": "comp1",
							},
							"spec": map[string]interface{}{
								"compositeTypeRef": map[string]interface{}{
									"apiVersion": "example.org/v1",
									"kind":       "XR1",
								},
							},
						},
					},
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "pkg.crossplane.io/v1",
							"kind":       "Function",
							"metadata": map[string]interface{}{
								"name": "func1",
							},
						},
					},
				}
				return fake.NewSimpleDynamicClient(scheme, objects...)
			},
			args: args{
				ctx: context.Background(),
			},
			want: want{
				compositions: map[compositionCacheKey]*apiextensionsv1.Composition{
					{apiVersion: "example.org/v1", kind: "XR1"}: {
						TypeMeta: metav1.TypeMeta{
							APIVersion: "apiextensions.crossplane.io/v1",
							Kind:       "Composition",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "comp1",
						},
						Spec: apiextensionsv1.CompositionSpec{
							CompositeTypeRef: apiextensionsv1.TypeReference{
								APIVersion: "example.org/v1",
								Kind:       "XR1",
							},
						},
					},
				},
				functions: map[string]pkgv1.Function{
					"func1": {
						TypeMeta: metav1.TypeMeta{
							APIVersion: "pkg.crossplane.io/v1",
							Kind:       "Function",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "func1",
						},
					},
				},
			},
		},
		"CompositionListError": {
			reason: "Should propagate errors from composition listing",
			setup: func() dynamic.Interface {
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "apiextensions.crossplane.io", Version: "v1", Resource: "compositions"}: "CompositionList",
						{Group: "pkg.crossplane.io", Version: "v1", Resource: "functions"}:              "FunctionList",
					})

				dc.Fake.PrependReactor("list", "compositions", func(action kt.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("composition list error")
				})

				return dc
			},
			args: args{
				ctx: context.Background(),
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errors.New("composition list error"), "cannot list compositions from cluster"), "cannot list compositions"),
			},
		},
		"FunctionListError": {
			reason: "Should propagate errors from function listing",
			setup: func() dynamic.Interface {
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "apiextensions.crossplane.io", Version: "v1", Resource: "compositions"}: "CompositionList",
						{Group: "pkg.crossplane.io", Version: "v1", Resource: "functions"}:              "FunctionList",
					})

				dc.Fake.PrependReactor("list", "functions", func(action kt.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("function list error")
				})

				return dc
			},
			args: args{
				ctx: context.Background(),
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errors.New("function list error"), "cannot list functions from cluster"), "cannot list functions"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &ClusterClient{
				dynamicClient: tc.setup(),
			}

			err := c.Initialize(tc.args.ctx)

			if tc.want.err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nInitialize(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nInitialize(...): unexpected error: %v", tc.reason, err)
				return
			}

			if diff := cmp.Diff(len(tc.want.compositions), len(c.compositions)); diff != "" {
				t.Errorf("\n%s\nInitialize(...): -want composition count, +got composition count:\n%s", tc.reason, diff)
			}

			for k, wantComp := range tc.want.compositions {
				gotComp, ok := c.compositions[k]
				if !ok {
					t.Errorf("\n%s\nInitialize(...): missing composition for key %v", tc.reason, k)
					continue
				}

				if diff := cmp.Diff(wantComp.Spec.CompositeTypeRef, gotComp.Spec.CompositeTypeRef); diff != "" {
					t.Errorf("\n%s\nInitialize(...): -want composition, +got composition:\n%s", tc.reason, diff)
				}
			}

			if diff := cmp.Diff(len(tc.want.functions), len(c.functions)); diff != "" {
				t.Errorf("\n%s\nInitialize(...): -want function count, +got function count:\n%s", tc.reason, diff)
			}

			for name, wantFunc := range tc.want.functions {
				gotFunc, ok := c.functions[name]
				if !ok {
					t.Errorf("\n%s\nInitialize(...): missing function with name %s", tc.reason, name)
					continue
				}

				if diff := cmp.Diff(wantFunc.GetName(), gotFunc.GetName()); diff != "" {
					t.Errorf("\n%s\nInitialize(...): -want function, +got function:\n%s", tc.reason, diff)
				}
			}
		})
	}
}

func TestClusterClient_GetExtraResources(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = pkgv1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	type args struct {
		ctx       context.Context
		gvrs      []schema.GroupVersionResource
		selectors []metav1.LabelSelector
	}

	type want struct {
		resources []unstructured.Unstructured
		err       error
	}

	cases := map[string]struct {
		reason string
		setup  func() dynamic.Interface
		args   args
		want   want
	}{
		"MismatchedGVRsAndSelectors": {
			reason: "Should return error when GVRs and selectors count mismatch",
			setup: func() dynamic.Interface {
				return fake.NewSimpleDynamicClient(scheme)
			},
			args: args{
				ctx: context.Background(),
				gvrs: []schema.GroupVersionResource{
					{Group: "example.org", Version: "v1", Resource: "resources"},
				},
				selectors: []metav1.LabelSelector{},
			},
			want: want{
				err: errors.New("number of GVRs must match number of selectors"),
			},
		},
		"NoMatchingResources": {
			reason: "Should return empty list when no resources match selector",
			setup: func() dynamic.Interface {
				c := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "example.org", Version: "v1", Resource: "resources"}: "ResourceList",
					})
				return c
			},
			args: args{
				ctx: context.Background(),
				gvrs: []schema.GroupVersionResource{
					{Group: "example.org", Version: "v1", Resource: "resources"},
				},
				selectors: []metav1.LabelSelector{
					{
						MatchLabels: map[string]string{"app": "test"},
					},
				},
			},
			want: want{
				resources: []unstructured.Unstructured{},
			},
		},
		"MatchingResources": {
			reason: "Should return resources matching selector",
			setup: func() dynamic.Interface {
				objects := []runtime.Object{
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name":   "res1",
								"labels": map[string]interface{}{"app": "test"},
							},
						},
					},
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name":   "res2",
								"labels": map[string]interface{}{"app": "other"},
							},
						},
					},
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v2",
							"kind":       "OtherResource",
							"metadata": map[string]interface{}{
								"name":   "other1",
								"labels": map[string]interface{}{"type": "test"},
							},
						},
					},
				}
				return fake.NewSimpleDynamicClient(scheme, objects...)
			},
			args: args{
				ctx: context.Background(),
				gvrs: []schema.GroupVersionResource{
					{Group: "example.org", Version: "v1", Resource: "resources"},
					{Group: "example.org", Version: "v2", Resource: "otherresources"},
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
				resources: []unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name":   "res1",
								"labels": map[string]interface{}{"app": "test"},
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v2",
							"kind":       "OtherResource",
							"metadata": map[string]interface{}{
								"name":   "other1",
								"labels": map[string]interface{}{"type": "test"},
							},
						},
					},
				},
			},
		},
		"ListError": {
			reason: "Should propagate errors from the Kubernetes API",
			setup: func() dynamic.Interface {
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "example.org", Version: "v1", Resource: "resources"}: "ResourceList",
					})
				dc.Fake.PrependReactor("list", "resources", func(action kt.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("list error")
				})
				return dc
			},
			args: args{
				ctx: context.Background(),
				gvrs: []schema.GroupVersionResource{
					{Group: "example.org", Version: "v1", Resource: "resources"},
				},
				selectors: []metav1.LabelSelector{
					{
						MatchLabels: map[string]string{"app": "test"},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errors.New("list error"), "cannot list resources for %s",
					schema.GroupVersionResource{Group: "example.org", Version: "v1", Resource: "resources"}),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &ClusterClient{
				dynamicClient: tc.setup(),
			}

			got, err := c.GetExtraResources(tc.args.ctx, tc.args.gvrs, tc.args.selectors)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nGetExtraResources(...): expected error but got none", tc.reason)
					return
				}

				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nGetExtraResources(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetExtraResources(...): unexpected error: %v", tc.reason, err)
				return
			}

			if diff := cmp.Diff(len(tc.want.resources), len(got)); diff != "" {
				t.Errorf("\n%s\nGetExtraResources(...): -want resource count, +got resource count:\n%s", tc.reason, diff)
			}

			// Just comparing lengths isn't enough since we want to make sure the right resources were returned
			if len(tc.want.resources) > 0 {
				// Create a map of resource names for easier lookup
				wantResources := make(map[string]bool)
				for _, res := range tc.want.resources {
					wantResources[res.GetName()] = true
				}

				for _, gotRes := range got {
					name := gotRes.GetName()
					if !wantResources[name] {
						t.Errorf("\n%s\nGetExtraResources(...): unexpected resource: %s", tc.reason, name)
					}
				}
			}
		})
	}
}

// TODO:  tests for FindMatchingComposition, GetFunctionsFromPipeline, GetXRDSchema
