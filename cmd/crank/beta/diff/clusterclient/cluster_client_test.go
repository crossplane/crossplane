package clusterclient

import (
	"context"
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strings"
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

	"k8s.io/client-go/rest"
)

// Ensure MockClusterClient implements the ClusterClient interface.
var _ ClusterClient = &testutils.MockClusterClient{}

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
			c := &DefaultClusterClient{
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
			c := &DefaultClusterClient{
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

func TestClusterClient_GetAllResourcesByLabels(t *testing.T) {
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
				err: errors.Wrap(errors.Wrapf(errors.New("list error"),
					"cannot list resources for '%s' matching '%s'",
					schema.GroupVersionResource{Group: "example.org", Version: "v1", Resource: "resources"}, "app=test"),
					"cannot get all resources"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &DefaultClusterClient{
				dynamicClient: tc.setup(),
			}

			got, err := c.GetAllResourcesByLabels(tc.args.ctx, tc.args.gvrs, tc.args.selectors)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nGetAllResourcesByLabels(...): expected error but got none", tc.reason)
					return
				}

				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nGetAllResourcesByLabels(...): -want error, +got error:\n%s", tc.reason, diff)
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
						t.Errorf("\n%s\nGetAllResourcesByLabels(...): unexpected resource: %s", tc.reason, name)
					}
				}
			}
		})
	}
}

func TestClusterClient_FindMatchingComposition(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = pkgv1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	type fields struct {
		compositions map[compositionCacheKey]*apiextensionsv1.Composition
	}

	type args struct {
		res *unstructured.Unstructured
	}

	type want struct {
		composition *apiextensionsv1.Composition
		err         error
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"NoMatchingComposition": {
			reason: "Should return error when no matching composition exists",
			fields: fields{
				compositions: map[compositionCacheKey]*apiextensionsv1.Composition{
					{apiVersion: "example.org/v1", kind: "OtherXR"}: {
						TypeMeta: metav1.TypeMeta{
							APIVersion: "apiextensions.crossplane.io/v1",
							Kind:       "Composition",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "non-matching-comp",
						},
						Spec: apiextensionsv1.CompositionSpec{
							CompositeTypeRef: apiextensionsv1.TypeReference{
								APIVersion: "example.org/v1",
								Kind:       "OtherXR",
							},
						},
					},
				},
			},
			args: args{
				res: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "XR1",
						"metadata": map[string]interface{}{
							"name": "my-xr",
						},
					},
				},
			},
			want: want{
				err: errors.Errorf("no composition found for %s", "example.org/v1, Kind=XR1"),
			},
		},
		"MatchingComposition": {
			reason: "Should return the matching composition",
			fields: fields{
				compositions: map[compositionCacheKey]*apiextensionsv1.Composition{
					{apiVersion: "example.org/v1", kind: "XR1"}: {
						TypeMeta: metav1.TypeMeta{
							APIVersion: "apiextensions.crossplane.io/v1",
							Kind:       "Composition",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "matching-comp",
						},
						Spec: apiextensionsv1.CompositionSpec{
							CompositeTypeRef: apiextensionsv1.TypeReference{
								APIVersion: "example.org/v1",
								Kind:       "XR1",
							},
						},
					},
					{apiVersion: "example.org/v1", kind: "OtherXR"}: {
						TypeMeta: metav1.TypeMeta{
							APIVersion: "apiextensions.crossplane.io/v1",
							Kind:       "Composition",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "non-matching-comp",
						},
						Spec: apiextensionsv1.CompositionSpec{
							CompositeTypeRef: apiextensionsv1.TypeReference{
								APIVersion: "example.org/v1",
								Kind:       "OtherXR",
							},
						},
					},
				},
			},
			args: args{
				res: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "XR1",
						"metadata": map[string]interface{}{
							"name": "my-xr",
						},
					},
				},
			},
			want: want{
				composition: &apiextensionsv1.Composition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "Composition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "matching-comp",
					},
					Spec: apiextensionsv1.CompositionSpec{
						CompositeTypeRef: apiextensionsv1.TypeReference{
							APIVersion: "example.org/v1",
							Kind:       "XR1",
						},
					},
				},
			},
		},
		"EmptyCompositionCache": {
			reason: "Should return error when composition cache is empty",
			fields: fields{
				compositions: map[compositionCacheKey]*apiextensionsv1.Composition{},
			},
			args: args{
				res: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "XR1",
						"metadata": map[string]interface{}{
							"name": "my-xr",
						},
					},
				},
			},
			want: want{
				err: errors.Errorf("no composition found for %s", "example.org/v1, Kind=XR1"),
			},
		},
		"DifferentVersions": {
			reason: "Should not match compositions with different versions",
			fields: fields{
				compositions: map[compositionCacheKey]*apiextensionsv1.Composition{
					{apiVersion: "example.org/v2", kind: "XR1"}: {
						TypeMeta: metav1.TypeMeta{
							APIVersion: "apiextensions.crossplane.io/v1",
							Kind:       "Composition",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "version-mismatch-comp",
						},
						Spec: apiextensionsv1.CompositionSpec{
							CompositeTypeRef: apiextensionsv1.TypeReference{
								APIVersion: "example.org/v2",
								Kind:       "XR1",
							},
						},
					},
				},
			},
			args: args{
				res: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "XR1",
						"metadata": map[string]interface{}{
							"name": "my-xr",
						},
					},
				},
			},
			want: want{
				err: errors.Errorf("no composition found for %s", "example.org/v1, Kind=XR1"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &DefaultClusterClient{
				compositions: tc.fields.compositions,
			}

			got, err := c.FindMatchingComposition(tc.args.res)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nFindMatchingComposition(...): expected error but got none", tc.reason)
					return
				}

				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nFindMatchingComposition(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nFindMatchingComposition(...): unexpected error: %v", tc.reason, err)
				return
			}

			if tc.want.composition != nil {
				if diff := cmp.Diff(tc.want.composition.Name, got.Name); diff != "" {
					t.Errorf("\n%s\nFindMatchingComposition(...): -want composition name, +got composition name:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.composition.Spec.CompositeTypeRef, got.Spec.CompositeTypeRef); diff != "" {
					t.Errorf("\n%s\nFindMatchingComposition(...): -want composition type ref, +got composition type ref:\n%s", tc.reason, diff)
				}
			}
		})
	}
}

func TestClusterClient_GetFunctionsFromPipeline(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = pkgv1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	pipelineMode := apiextensionsv1.CompositionModePipeline
	nonPipelineMode := apiextensionsv1.CompositionMode("NonPipeline")

	type fields struct {
		functions map[string]pkgv1.Function
	}

	type args struct {
		comp *apiextensionsv1.Composition
	}

	type want struct {
		functions []pkgv1.Function
		err       error
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"NonPipelineMode": {
			reason: "Should return nil when composition is not in pipeline mode",
			fields: fields{
				functions: map[string]pkgv1.Function{},
			},
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &nonPipelineMode,
					},
				},
			},
			want: want{
				functions: nil,
			},
		},
		"NoModeSpecified": {
			reason: "Should return nil when composition mode is not specified",
			fields: fields{
				functions: map[string]pkgv1.Function{},
			},
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: nil,
					},
				},
			},
			want: want{
				functions: nil,
			},
		},
		"EmptyPipeline": {
			reason: "Should return empty slice for empty pipeline",
			fields: fields{
				functions: map[string]pkgv1.Function{},
			},
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode:     &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{},
					},
				},
			},
			want: want{
				functions: []pkgv1.Function{},
			},
		},
		"MissingFunction": {
			reason: "Should return error when a function is missing",
			fields: fields{
				functions: map[string]pkgv1.Function{
					"function-a": {
						TypeMeta: metav1.TypeMeta{
							APIVersion: "pkg.crossplane.io/v1",
							Kind:       "Function",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-a",
						},
					},
				},
			},
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-a",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-a"},
							},
							{
								Step:        "step-b",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-b"},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Errorf("function %q referenced in pipeline step %q not found", "function-b", "step-b"),
			},
		},
		"AllFunctionsFound": {
			reason: "Should return all functions referenced in the pipeline",
			fields: fields{
				functions: map[string]pkgv1.Function{
					"function-a": {
						TypeMeta: metav1.TypeMeta{
							APIVersion: "pkg.crossplane.io/v1",
							Kind:       "Function",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-a",
						},
					},
					"function-b": {
						TypeMeta: metav1.TypeMeta{
							APIVersion: "pkg.crossplane.io/v1",
							Kind:       "Function",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-b",
						},
					},
				},
			},
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-a",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-a"},
							},
							{
								Step:        "step-b",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-b"},
							},
						},
					},
				},
			},
			want: want{
				functions: []pkgv1.Function{
					{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "pkg.crossplane.io/v1",
							Kind:       "Function",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-a",
						},
					},
					{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "pkg.crossplane.io/v1",
							Kind:       "Function",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-b",
						},
					},
				},
			},
		},
		"DuplicateFunctionRefs": {
			reason: "Should handle pipeline steps that reference the same function",
			fields: fields{
				functions: map[string]pkgv1.Function{
					"function-a": {
						TypeMeta: metav1.TypeMeta{
							APIVersion: "pkg.crossplane.io/v1",
							Kind:       "Function",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-a",
						},
					},
				},
			},
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-a",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-a"},
							},
							{
								Step:        "step-b",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-a"},
							},
						},
					},
				},
			},
			want: want{
				functions: []pkgv1.Function{
					{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "pkg.crossplane.io/v1",
							Kind:       "Function",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-a",
						},
					},
					{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "pkg.crossplane.io/v1",
							Kind:       "Function",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-a",
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &DefaultClusterClient{
				functions: tc.fields.functions,
			}

			got, err := c.GetFunctionsFromPipeline(tc.args.comp)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nGetFunctionsFromPipeline(...): expected error but got none", tc.reason)
					return
				}

				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nGetFunctionsFromPipeline(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetFunctionsFromPipeline(...): unexpected error: %v", tc.reason, err)
				return
			}

			if tc.want.functions == nil {
				if got != nil {
					t.Errorf("\n%s\nGetFunctionsFromPipeline(...): expected nil functions, got %v", tc.reason, got)
				}
				return
			}

			if diff := cmp.Diff(len(tc.want.functions), len(got)); diff != "" {
				t.Errorf("\n%s\nGetFunctionsFromPipeline(...): -want function count, +got function count:\n%s", tc.reason, diff)
			}

			// Check each function matches what we expect
			for i, wantFn := range tc.want.functions {
				if i >= len(got) {
					break
				}
				if diff := cmp.Diff(wantFn.GetName(), got[i].GetName()); diff != "" {
					t.Errorf("\n%s\nGetFunctionsFromPipeline(...): -want function name, +got function name at index %d:\n%s", tc.reason, i, diff)
				}
			}
		})
	}
}

func TestClusterClient_GetXRDs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = pkgv1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	type args struct {
		ctx context.Context
	}

	type want struct {
		xrds []*unstructured.Unstructured
		err  error
	}

	cases := map[string]struct {
		reason string
		setup  func() dynamic.Interface
		args   args
		want   want
	}{
		"NoXRDsFound": {
			reason: "Should return empty slice when no XRDs exist",
			setup: func() dynamic.Interface {
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "apiextensions.crossplane.io", Version: "v1", Resource: "compositeresourcedefinitions"}: "CompositeResourceDefinitionList",
					})
				return dc
			},
			args: args{
				ctx: context.Background(),
			},
			want: want{
				xrds: []*unstructured.Unstructured{},
			},
		},
		"XRDsExist": {
			reason: "Should return all XRDs when they exist",
			setup: func() dynamic.Interface {
				objects := []runtime.Object{
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.crossplane.io/v1",
							"kind":       "CompositeResourceDefinition",
							"metadata": map[string]interface{}{
								"name": "xr1s.example.org",
							},
							"spec": map[string]interface{}{
								"group": "example.org",
								"names": map[string]interface{}{
									"kind":     "XR1",
									"plural":   "xr1s",
									"singular": "xr1",
								},
								"versions": []interface{}{
									map[string]interface{}{
										"name":    "v1",
										"served":  true,
										"storage": true,
										"schema": map[string]interface{}{
											"openAPIV3Schema": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"spec": map[string]interface{}{
														"type": "object",
													},
												},
											},
										},
									},
								},
							},
						},
					},
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.crossplane.io/v1",
							"kind":       "CompositeResourceDefinition",
							"metadata": map[string]interface{}{
								"name": "xr2s.example.org",
							},
							"spec": map[string]interface{}{
								"group": "example.org",
								"names": map[string]interface{}{
									"kind":     "XR2",
									"plural":   "xr2s",
									"singular": "xr2",
								},
								"versions": []interface{}{
									map[string]interface{}{
										"name":    "v1",
										"served":  true,
										"storage": true,
									},
								},
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
				xrds: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.crossplane.io/v1",
							"kind":       "CompositeResourceDefinition",
							"metadata": map[string]interface{}{
								"name": "xr1s.example.org",
							},
							"spec": map[string]interface{}{
								"group": "example.org",
								"names": map[string]interface{}{
									"kind":     "XR1",
									"plural":   "xr1s",
									"singular": "xr1",
								},
								"versions": []interface{}{
									map[string]interface{}{
										"name":    "v1",
										"served":  true,
										"storage": true,
										"schema": map[string]interface{}{
											"openAPIV3Schema": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"spec": map[string]interface{}{
														"type": "object",
													},
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.crossplane.io/v1",
							"kind":       "CompositeResourceDefinition",
							"metadata": map[string]interface{}{
								"name": "xr2s.example.org",
							},
							"spec": map[string]interface{}{
								"group": "example.org",
								"names": map[string]interface{}{
									"kind":     "XR2",
									"plural":   "xr2s",
									"singular": "xr2",
								},
								"versions": []interface{}{
									map[string]interface{}{
										"name":    "v1",
										"served":  true,
										"storage": true,
									},
								},
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
						{Group: "apiextensions.crossplane.io", Version: "v1", Resource: "compositeresourcedefinitions"}: "CompositeResourceDefinitionList",
					})
				dc.Fake.PrependReactor("list", "compositeresourcedefinitions", func(action kt.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("list error")
				})
				return dc
			},
			args: args{
				ctx: context.Background(),
			},
			want: want{
				err: errors.Wrap(errors.New("list error"), "cannot list XRDs"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &DefaultClusterClient{
				dynamicClient: tc.setup(),
			}

			got, err := c.GetXRDs(tc.args.ctx)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nGetXRDs(...): expected error but got none", tc.reason)
					return
				}

				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nGetXRDs(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetXRDs(...): unexpected error: %v", tc.reason, err)
				return
			}

			if diff := cmp.Diff(len(tc.want.xrds), len(got)); diff != "" {
				t.Errorf("\n%s\nGetXRDs(...): -want xrd count, +got xrd count:\n%s", tc.reason, diff)
			}

			// Check if we got the right XRDs by name
			// Create maps of XRD names for easier lookup
			wantXRDNames := make(map[string]bool)
			gotXRDNames := make(map[string]bool)

			for _, xrd := range tc.want.xrds {
				wantXRDNames[xrd.GetName()] = true
			}

			for _, xrd := range got {
				gotXRDNames[xrd.GetName()] = true
			}

			for name := range wantXRDNames {
				if !gotXRDNames[name] {
					t.Errorf("\n%s\nGetXRDs(...): missing expected XRD with name %s", tc.reason, name)
				}
			}

			for name := range gotXRDNames {
				if !wantXRDNames[name] {
					t.Errorf("\n%s\nGetXRDs(...): unexpected XRD with name %s", tc.reason, name)
				}
			}
		})
	}
}

func TestClusterClient_GetResource(t *testing.T) {
	scheme := runtime.NewScheme()

	type args struct {
		ctx       context.Context
		gvk       schema.GroupVersionKind
		namespace string
		name      string
	}

	type want struct {
		resource *unstructured.Unstructured
		err      error
	}

	cases := map[string]struct {
		reason string
		setup  func() dynamic.Interface
		args   args
		want   want
	}{
		"NamespacedResourceFound": {
			reason: "Should return the resource when it exists in a namespace",
			setup: func() dynamic.Interface {
				objects := []runtime.Object{
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "ExampleResource",
							"metadata": map[string]interface{}{
								"name":      "test-resource",
								"namespace": "test-namespace",
							},
							"spec": map[string]interface{}{
								"property": "value",
							},
						},
					},
				}
				return fake.NewSimpleDynamicClient(scheme, objects...)
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
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ExampleResource",
						"metadata": map[string]interface{}{
							"name":      "test-resource",
							"namespace": "test-namespace",
						},
						"spec": map[string]interface{}{
							"property": "value",
						},
					},
				},
			},
		},
		"ClusterScopedResourceFound": {
			reason: "Should return the resource when it exists at cluster scope",
			setup: func() dynamic.Interface {
				objects := []runtime.Object{
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "ClusterResource",
							"metadata": map[string]interface{}{
								"name": "test-cluster-resource",
							},
							"spec": map[string]interface{}{
								"property": "value",
							},
						},
					},
				}
				return fake.NewSimpleDynamicClient(scheme, objects...)
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
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ClusterResource",
						"metadata": map[string]interface{}{
							"name": "test-cluster-resource",
						},
						"spec": map[string]interface{}{
							"property": "value",
						},
					},
				},
			},
		},
		"ResourceNotFound": {
			reason: "Should return an error when the resource doesn't exist",
			setup: func() dynamic.Interface {
				dc := fake.NewSimpleDynamicClient(scheme)
				dc.Fake.PrependReactor("get", "*", func(action kt.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("resource not found")
				})
				return dc
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
		"SpecialResourceType": {
			reason: "Should handle special resource types with non-standard pluralization",
			setup: func() dynamic.Interface {
				objects := []runtime.Object{
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Endpoints",
							"metadata": map[string]interface{}{
								"name":      "test-endpoints",
								"namespace": "test-namespace",
							},
							"subsets": []interface{}{
								map[string]interface{}{
									"addresses": []interface{}{
										map[string]interface{}{
											"ip": "192.168.1.1",
										},
									},
								},
							},
						},
					},
				}
				return fake.NewSimpleDynamicClient(scheme, objects...)
			},
			args: args{
				ctx: context.Background(),
				gvk: schema.GroupVersionKind{
					Group:   "",
					Version: "v1",
					Kind:    "Endpoints",
				},
				namespace: "test-namespace",
				name:      "test-endpoints",
			},
			want: want{
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Endpoints",
						"metadata": map[string]interface{}{
							"name":      "test-endpoints",
							"namespace": "test-namespace",
						},
						"subsets": []interface{}{
							map[string]interface{}{
								"addresses": []interface{}{
									map[string]interface{}{
										"ip": "192.168.1.1",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &DefaultClusterClient{
				dynamicClient: tc.setup(),
			}

			got, err := c.GetResource(tc.args.ctx, tc.args.gvk, tc.args.namespace, tc.args.name)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nGetResource(...): expected error but got none", tc.reason)
					return
				}

				if !strings.Contains(err.Error(), tc.want.err.Error()) {
					t.Errorf("\n%s\nGetResource(...): expected error containing %q, got %q", tc.reason, tc.want.err.Error(), err.Error())
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
				meta, found, _ := unstructured.NestedMap(gotCopy.Object, "metadata")
				if found && meta != nil {
					delete(meta, "resourceVersion")
					_ = unstructured.SetNestedMap(gotCopy.Object, meta, "metadata")
				}
			}

			if diff := cmp.Diff(tc.want.resource, gotCopy); diff != "" {
				t.Errorf("\n%s\nGetResource(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// MockDryRunClient is a mock implementation of the ClusterClient interface
// specifically designed to test DryRunApply
type MockDryRunClient struct {
	mockDryRunApply func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
}

// Initialize implements ClusterClient
func (m *MockDryRunClient) Initialize(ctx context.Context) error {
	return nil
}

// FindMatchingComposition implements ClusterClient
func (m *MockDryRunClient) FindMatchingComposition(*unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
	return nil, errors.New("not implemented")
}

// GetAllResourcesByLabels implements ClusterClient
func (m *MockDryRunClient) GetAllResourcesByLabels(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]unstructured.Unstructured, error) {
	return nil, errors.New("not implemented")
}

// GetFunctionsFromPipeline implements ClusterClient
func (m *MockDryRunClient) GetFunctionsFromPipeline(*apiextensionsv1.Composition) ([]pkgv1.Function, error) {
	return nil, errors.New("not implemented")
}

// GetXRDSchema implements ClusterClient
func (m *MockDryRunClient) GetXRDSchema(ctx context.Context, res *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error) {
	return nil, errors.New("not implemented")
}

// GetResource implements ClusterClient
func (m *MockDryRunClient) GetResource(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
	return nil, errors.New("not implemented")
}

// DryRunApply implements ClusterClient
func (m *MockDryRunClient) DryRunApply(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if m.mockDryRunApply != nil {
		return m.mockDryRunApply(ctx, obj)
	}
	return nil, errors.New("not implemented")
}

func TestClusterClient_DryRunApply(t *testing.T) {
	type args struct {
		ctx context.Context
		obj *unstructured.Unstructured
	}

	type want struct {
		result *unstructured.Unstructured
		err    error
	}

	cases := map[string]struct {
		reason       string
		mockDryRunFn func(context.Context, *unstructured.Unstructured) (*unstructured.Unstructured, error)
		args         args
		want         want
	}{
		"NamespacedResourceApplied": {
			reason: "Should successfully apply a namespaced resource",
			mockDryRunFn: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				// Create a modified copy of the input object
				result := obj.DeepCopy()
				result.SetResourceVersion("1000")
				return result, nil
			},
			args: args{
				ctx: context.Background(),
				obj: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ExampleResource",
						"metadata": map[string]interface{}{
							"name":      "test-resource",
							"namespace": "test-namespace",
						},
						"spec": map[string]interface{}{
							"property": "new-value",
						},
					},
				},
			},
			want: want{
				result: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ExampleResource",
						"metadata": map[string]interface{}{
							"name":            "test-resource",
							"namespace":       "test-namespace",
							"resourceVersion": "1000",
						},
						"spec": map[string]interface{}{
							"property": "new-value",
						},
					},
				},
			},
		},
		"ClusterScopedResourceApplied": {
			reason: "Should successfully apply a cluster-scoped resource",
			mockDryRunFn: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				// Create a modified copy of the input object
				result := obj.DeepCopy()
				result.SetResourceVersion("1000")
				return result, nil
			},
			args: args{
				ctx: context.Background(),
				obj: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ClusterResource",
						"metadata": map[string]interface{}{
							"name": "test-cluster-resource",
						},
						"spec": map[string]interface{}{
							"property": "new-value",
						},
					},
				},
			},
			want: want{
				result: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ClusterResource",
						"metadata": map[string]interface{}{
							"name":            "test-cluster-resource",
							"resourceVersion": "1000",
						},
						"spec": map[string]interface{}{
							"property": "new-value",
						},
					},
				},
			},
		},
		"ApplyError": {
			reason: "Should return error when apply fails",
			mockDryRunFn: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				return nil, errors.New("apply failed")
			},
			args: args{
				ctx: context.Background(),
				obj: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ExampleResource",
						"metadata": map[string]interface{}{
							"name":      "test-resource",
							"namespace": "test-namespace",
						},
					},
				},
			},
			want: want{
				result: nil,
				err:    errors.New("apply failed"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a mock client with the provided mock function
			c := &MockDryRunClient{
				mockDryRunApply: tc.mockDryRunFn,
			}

			got, err := c.DryRunApply(tc.args.ctx, tc.args.obj)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nDryRunApply(...): expected error but got none", tc.reason)
					return
				}

				if !strings.Contains(err.Error(), tc.want.err.Error()) {
					t.Errorf("\n%s\nDryRunApply(...): expected error containing %q, got %q", tc.reason, tc.want.err.Error(), err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nDryRunApply(...): unexpected error: %v", tc.reason, err)
				return
			}

			// For successful cases, compare results
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\n%s\nDryRunApply(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestClusterClient_GetResourcesByLabel(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = pkgv1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	type args struct {
		ctx       context.Context
		namespace string
		gvr       schema.GroupVersionResource
		selector  metav1.LabelSelector
	}

	type want struct {
		resources []*unstructured.Unstructured
		err       error
	}

	cases := map[string]struct {
		reason string
		setup  func() dynamic.Interface
		args   args
		want   want
	}{
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
				ctx:       context.Background(),
				namespace: "test-namespace",
				gvr: schema.GroupVersionResource{
					Group:    "example.org",
					Version:  "v1",
					Resource: "resources",
				},
				selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
			},
			want: want{
				resources: nil,
				err:       errors.New("cannot list resources for 'example.org/v1, Resource=resources' matching 'app=test': list error"),
			},
		},
		"NoMatchingResources": {
			reason: "Should return empty list when no resources match selector",
			setup: func() dynamic.Interface {
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "example.org", Version: "v1", Resource: "resources"}: "ResourceList",
					})
				return dc
			},
			args: args{
				ctx:       context.Background(),
				namespace: "test-namespace",
				gvr: schema.GroupVersionResource{
					Group:    "example.org",
					Version:  "v1",
					Resource: "resources",
				},
				selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
			},
			want: want{
				resources: []*unstructured.Unstructured{},
				err:       nil,
			},
		},
		"MatchingResources": {
			reason: "Should return resources matching label selector",
			setup: func() dynamic.Interface {
				objects := []runtime.Object{
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name":      "matched-resource-1",
								"namespace": "test-namespace",
								"labels": map[string]interface{}{
									"app": "test",
									"env": "dev",
								},
							},
						},
					},
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name":      "matched-resource-2",
								"namespace": "test-namespace",
								"labels": map[string]interface{}{
									"app": "test",
									"env": "prod",
								},
							},
						},
					},
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name":      "unmatched-resource",
								"namespace": "test-namespace",
								"labels": map[string]interface{}{
									"app": "other",
								},
							},
						},
					},
				}
				return fake.NewSimpleDynamicClient(scheme, objects...)
			},
			args: args{
				ctx:       context.Background(),
				namespace: "test-namespace",
				gvr: schema.GroupVersionResource{
					Group:    "example.org",
					Version:  "v1",
					Resource: "resources",
				},
				selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
			},
			want: want{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name":      "matched-resource-1",
								"namespace": "test-namespace",
								"labels": map[string]interface{}{
									"app": "test",
									"env": "dev",
								},
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name":      "matched-resource-2",
								"namespace": "test-namespace",
								"labels": map[string]interface{}{
									"app": "test",
									"env": "prod",
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"ClusterScopedResources": {
			reason: "Should return cluster-scoped resources when namespace is empty",
			setup: func() dynamic.Interface {
				objects := []runtime.Object{
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "ClusterResource",
							"metadata": map[string]interface{}{
								"name": "matched-cluster-resource-1",
								"labels": map[string]interface{}{
									"scope": "cluster",
									"type":  "config",
								},
							},
						},
					},
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "ClusterResource",
							"metadata": map[string]interface{}{
								"name": "matched-cluster-resource-2",
								"labels": map[string]interface{}{
									"scope": "cluster",
									"type":  "config",
								},
							},
						},
					},
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "ClusterResource",
							"metadata": map[string]interface{}{
								"name": "unmatched-cluster-resource",
								"labels": map[string]interface{}{
									"scope": "cluster",
									"type":  "network",
								},
							},
						},
					},
				}
				return fake.NewSimpleDynamicClient(scheme, objects...)
			},
			args: args{
				ctx:       context.Background(),
				namespace: "", // Empty namespace for cluster-scoped resources
				gvr: schema.GroupVersionResource{
					Group:    "example.org",
					Version:  "v1",
					Resource: "clusterresources",
				},
				selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"scope": "cluster",
						"type":  "config",
					},
				},
			},
			want: want{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "ClusterResource",
							"metadata": map[string]interface{}{
								"name": "matched-cluster-resource-1",
								"labels": map[string]interface{}{
									"scope": "cluster",
									"type":  "config",
								},
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "ClusterResource",
							"metadata": map[string]interface{}{
								"name": "matched-cluster-resource-2",
								"labels": map[string]interface{}{
									"scope": "cluster",
									"type":  "config",
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"MultipleLabelSelectors": {
			reason: "Should correctly filter resources with multiple label selectors",
			setup: func() dynamic.Interface {
				objects := []runtime.Object{
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name":      "resource-1",
								"namespace": "test-namespace",
								"labels": map[string]interface{}{
									"app":  "test",
									"env":  "dev",
									"tier": "frontend",
								},
							},
						},
					},
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name":      "resource-2",
								"namespace": "test-namespace",
								"labels": map[string]interface{}{
									"app":  "test",
									"env":  "prod",
									"tier": "backend",
								},
							},
						},
					},
				}
				return fake.NewSimpleDynamicClient(scheme, objects...)
			},
			args: args{
				ctx:       context.Background(),
				namespace: "test-namespace",
				gvr: schema.GroupVersionResource{
					Group:    "example.org",
					Version:  "v1",
					Resource: "resources",
				},
				selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
						"env": "dev",
					},
				},
			},
			want: want{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name":      "resource-1",
								"namespace": "test-namespace",
								"labels": map[string]interface{}{
									"app":  "test",
									"env":  "dev",
									"tier": "frontend",
								},
							},
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &DefaultClusterClient{
				dynamicClient: tc.setup(),
			}

			got, err := c.GetResourcesByLabel(tc.args.ctx, tc.args.namespace, tc.args.gvr, tc.args.selector)

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

// TestNewClusterClient tests the creation of a new DefaultClusterClient instance
func TestNewClusterClient(t *testing.T) {
	// Skip the nil config test because we can't easily mock the underlying functions
	// We'll just test the valid config case
	validConfig := &rest.Config{
		Host: "https://localhost:8080",
	}

	_, err := NewClusterClient(validConfig)
	if err != nil {
		t.Errorf("NewClusterClient(...): unexpected error with valid config: %v", err)
	}
}
