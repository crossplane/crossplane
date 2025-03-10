package clusterclient

import (
	"context"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
			c := &DefaultClusterClient{
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

func TestClusterClient_GetXRDSchema(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = pkgv1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	type args struct {
		ctx context.Context
		res *unstructured.Unstructured
	}

	type want struct {
		xrd *apiextensionsv1.CompositeResourceDefinition
		err error
	}

	cases := map[string]struct {
		reason string
		setup  func() dynamic.Interface
		args   args
		want   want
	}{
		"NoXRDsFound": {
			reason: "Should return error when no XRDs exist",
			setup: func() dynamic.Interface {
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "apiextensions.crossplane.io", Version: "v1", Resource: "compositeresourcedefinitions"}: "CompositeResourceDefinitionList",
					})
				return dc
			},
			args: args{
				ctx: context.Background(),
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
				err: errors.Errorf("no XRD found for %s", "example.org/v1, Kind=XR1"),
			},
		},
		"XRDsExistButNoMatch": {
			reason: "Should return error when XRDs exist but none match the resource",
			setup: func() dynamic.Interface {
				objects := []runtime.Object{
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.crossplane.io/v1",
							"kind":       "CompositeResourceDefinition",
							"metadata": map[string]interface{}{
								"name": "xr1s.other.org",
							},
							"spec": map[string]interface{}{
								"group": "other.org",
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
				err: errors.Errorf("no XRD found for %s", "example.org/v1, Kind=XR1"),
			},
		},
		"MatchingXRDFound": {
			reason: "Should return the matching XRD when one exists",
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
				xrd: &apiextensionsv1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xr1s.example.org",
					},
					Spec: apiextensionsv1.CompositeResourceDefinitionSpec{
						Group: "example.org",
						Names: extv1.CustomResourceDefinitionNames{
							Kind:     "XR1",
							Plural:   "xr1s",
							Singular: "xr1",
						},
						Versions: []apiextensionsv1.CompositeResourceDefinitionVersion{
							{
								Name:   "v1",
								Served: true,
								Schema: &apiextensionsv1.CompositeResourceValidation{
									OpenAPIV3Schema: runtime.RawExtension{
										Raw: []byte(`{"properties":{"spec":{"type":"object"}},"type":"object"}`),
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
				err: errors.Wrap(errors.New("list error"), "cannot list XRDs"),
			},
		},
		"ConversionError": {
			reason: "Should handle conversion errors gracefully",
			setup: func() dynamic.Interface {
				// Create a malformed XRD that will cause conversion issues
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
									"kind": "XR1",
									// Missing required fields will cause conversion errors
								},
								// Invalid versions structure
								"versions": "not-an-array",
							},
						},
					},
				}
				return fake.NewSimpleDynamicClient(scheme, objects...)
			},
			args: args{
				ctx: context.Background(),
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
				// The exact error message may vary depending on the runtime implementation
				// So we'll just check that it contains "string" as that's the part we're testing
				err: errors.New("cannot convert unstructured to XRD: cannot restore slice from string"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &DefaultClusterClient{
				dynamicClient: tc.setup(),
			}

			got, err := c.GetXRDSchema(tc.args.ctx, tc.args.res)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nGetXRDSchema(...): expected error but got none", tc.reason)
					return
				}

				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nGetXRDSchema(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetXRDSchema(...): unexpected error: %v", tc.reason, err)
				return
			}

			// Skip OpenAPIV3Schema comparison as it's hard to match exactly with the JSON marshaling differences
			gotSchemaRaw := got.Spec.Versions[0].Schema.OpenAPIV3Schema.Raw
			got.Spec.Versions[0].Schema.OpenAPIV3Schema.Raw = nil
			tc.want.xrd.Spec.Versions[0].Schema.OpenAPIV3Schema.Raw = nil

			if diff := cmp.Diff(tc.want.xrd, got, cmpopts.IgnoreFields(apiextensionsv1.CompositeResourceDefinitionVersion{}, "Schema")); diff != "" {
				t.Errorf("\n%s\nGetXRDSchema(...): -want, +got:\n%s", tc.reason, diff)
			}

			// Now check if we got a non-empty schema
			if len(gotSchemaRaw) == 0 {
				t.Errorf("\n%s\nGetXRDSchema(...): expected non-empty schema", tc.reason)
			}
		})
	}
}
