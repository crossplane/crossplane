package clusterclient

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal/resource"
	"github.com/go-logr/logr/testr"
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
var _ ClusterClient = &tu.MockClusterClient{}

func TestClusterClient_GetEnvironmentConfigs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = pkgv1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	type args struct {
		ctx context.Context
	}

	type want struct {
		envConfigs []*unstructured.Unstructured
		err        error
	}

	tests := map[string]struct {
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
				envConfigs: []*unstructured.Unstructured{},
			},
		},
		"AllConfigs": {
			reason: "Should return all configs when they exist",
			setup: func() dynamic.Interface {
				// Use resource builders here
				objects := []runtime.Object{
					tu.NewResource("apiextensions.crossplane.io/v1alpha1", "EnvironmentConfig", "config1").
						WithSpecField("data", map[string]interface{}{
							"key": "value1",
						}).
						Build(),
					tu.NewResource("apiextensions.crossplane.io/v1alpha1", "EnvironmentConfig", "config2").
						WithSpecField("data", map[string]interface{}{
							"key": "value2",
						}).
						Build(),
				}
				return fake.NewSimpleDynamicClient(scheme, objects...)
			},
			args: args{
				ctx: context.Background(),
			},
			want: want{
				envConfigs: []*unstructured.Unstructured{
					tu.NewResource("apiextensions.crossplane.io/v1alpha1", "EnvironmentConfig", "config1").
						WithSpecField("data", map[string]interface{}{
							"key": "value1",
						}).
						Build(),
					tu.NewResource("apiextensions.crossplane.io/v1alpha1", "EnvironmentConfig", "config2").
						WithSpecField("data", map[string]interface{}{
							"key": "value2",
						}).
						Build(),
				},
			},
		},
		"APIError": {
			reason: "Should propagate errors from the Kubernetes API",
			setup: func() dynamic.Interface {
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "apiextensions.crossplane.io", Version: "v1alpha1", Resource: "environmentconfigs"}: "EnvironmentConfigList",
					})

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

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultClusterClient{
				dynamicClient: tc.setup(),
				logger:        logging.NewLogrLogger(testr.New(t)),
			}

			got, err := c.GetEnvironmentConfigs(tc.args.ctx)

			if tc.want.err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nGetEnvironmentConfigs(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetEnvironmentConfigs(...): unexpected error: %v", tc.reason, err)
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
					// Use resource builders for composition and function
					tu.NewResource("apiextensions.crossplane.io/v1", "Composition", "comp1").
						WithSpecField("compositeTypeRef", map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "XR1",
						}).
						Build(),
					tu.NewResource("pkg.crossplane.io/v1", "Function", "func1").Build(),
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

				// Setup compositions to respond normally
				objects := []runtime.Object{
					tu.NewResource("apiextensions.crossplane.io/v1", "Composition", "comp1").
						WithSpecField("compositeTypeRef", map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "XR1",
						}).
						Build(),
				}

				dc = fake.NewSimpleDynamicClient(scheme, objects...)

				// But make functions fail
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
				logger:        logging.NewLogrLogger(testr.New(t)),
			}

			err := c.Initialize(tc.args.ctx)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nInitialize(...): expected error but got none", tc.reason)
					return
				}

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
		resources []*unstructured.Unstructured
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
				resources: []*unstructured.Unstructured{},
			},
		},
		"MatchingResources": {
			reason: "Should return resources matching selector",
			setup: func() dynamic.Interface {
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
				resources: []*unstructured.Unstructured{
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
				logger:        logging.NewLogrLogger(testr.New(t)),
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

			// For successful cases, compare results
			for i, wantResource := range tc.want.resources {
				if i >= len(got) {
					break
				}

				if diff := cmp.Diff(wantResource.GetName(), got[i].GetName()); diff != "" {
					t.Errorf("\n%s\nGetAllResourcesByLabels(...): -want resource name, +got resource name at index %d:\n%s", tc.reason, i, diff)
				}

				if diff := cmp.Diff(wantResource.GetLabels(), got[i].GetLabels()); diff != "" {
					t.Errorf("\n%s\nGetAllResourcesByLabels(...): -want resource labels, +got resource labels at index %d:\n%s", tc.reason, i, diff)
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

	tests := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"NoMatchingComposition": {
			reason: "Should return error when no matching composition exists",
			fields: fields{
				compositions: map[compositionCacheKey]*apiextensionsv1.Composition{
					{apiVersion: "example.org/v1", kind: "OtherXR"}: tu.NewComposition("non-matching-comp").
						WithCompositeTypeRef("example.org/v1", "OtherXR").
						Build(),
				},
			},
			args: args{
				res: tu.NewResource("example.org/v1", "XR1", "my-xr").Build(),
			},
			want: want{
				err: errors.Errorf("no composition found for %s", "example.org/v1, Kind=XR1"),
			},
		},
		"MatchingComposition": {
			reason: "Should return the matching composition",
			fields: fields{
				compositions: map[compositionCacheKey]*apiextensionsv1.Composition{
					{apiVersion: "example.org/v1", kind: "XR1"}: tu.NewComposition("matching-comp").
						WithCompositeTypeRef("example.org/v1", "XR1").
						Build(),
					{apiVersion: "example.org/v1", kind: "OtherXR"}: tu.NewComposition("non-matching-comp").
						WithCompositeTypeRef("example.org/v1", "OtherXR").
						Build(),
				},
			},
			args: args{
				res: tu.NewResource("example.org/v1", "XR1", "my-xr").Build(),
			},
			want: want{
				composition: tu.NewComposition("matching-comp").
					WithCompositeTypeRef("example.org/v1", "XR1").
					Build(),
			},
		},
		"EmptyCompositionCache": {
			reason: "Should return error when composition cache is empty",
			fields: fields{
				compositions: map[compositionCacheKey]*apiextensionsv1.Composition{},
			},
			args: args{
				res: tu.NewResource("example.org/v1", "XR1", "my-xr").Build(),
			},
			want: want{
				err: errors.Errorf("no composition found for %s", "example.org/v1, Kind=XR1"),
			},
		},
		"DifferentVersions": {
			reason: "Should not match compositions with different versions",
			fields: fields{
				compositions: map[compositionCacheKey]*apiextensionsv1.Composition{
					{apiVersion: "example.org/v2", kind: "XR1"}: tu.NewComposition("version-mismatch-comp").
						WithCompositeTypeRef("example.org/v2", "XR1").
						Build(),
				},
			},
			args: args{
				res: tu.NewResource("example.org/v1", "XR1", "my-xr").Build(),
			},
			want: want{
				err: errors.Errorf("no composition found for %s", "example.org/v1, Kind=XR1"),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultClusterClient{
				compositions: tc.fields.compositions,
				logger:       logging.NewLogrLogger(testr.New(t)),
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

	tests := map[string]struct {
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
				// Use Composition builder to create a composition with pipeline steps
				comp: tu.NewComposition("test-comp").
					WithPipelineMode().
					WithPipelineStep("step-a", "function-a", nil).
					WithPipelineStep("step-b", "function-b", nil).
					Build(),
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
				// Use Composition builder for a composition with duplicate function references
				comp: tu.NewComposition("test-comp").
					WithPipelineMode().
					WithPipelineStep("step-a", "function-a", nil).
					WithPipelineStep("step-b", "function-a", nil).
					Build(),
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

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultClusterClient{
				functions: tc.fields.functions,
				logger:    logging.NewLogrLogger(testr.New(t)),
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

	tests := map[string]struct {
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
					// Use resource builders for XRDs
					tu.NewResource("apiextensions.crossplane.io/v1", "CompositeResourceDefinition", "xr1s.example.org").
						WithSpecField("group", "example.org").
						WithSpecField("names", map[string]interface{}{
							"kind":     "XR1",
							"plural":   "xr1s",
							"singular": "xr1",
						}).
						WithSpecField("versions", []interface{}{
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
						}).
						Build(),
					tu.NewResource("apiextensions.crossplane.io/v1", "CompositeResourceDefinition", "xr2s.example.org").
						WithSpecField("group", "example.org").
						WithSpecField("names", map[string]interface{}{
							"kind":     "XR2",
							"plural":   "xr2s",
							"singular": "xr2",
						}).
						WithSpecField("versions", []interface{}{
							map[string]interface{}{
								"name":    "v1",
								"served":  true,
								"storage": true,
							},
						}).
						Build(),
				}
				return fake.NewSimpleDynamicClient(scheme, objects...)
			},
			args: args{
				ctx: context.Background(),
			},
			want: want{
				xrds: []*unstructured.Unstructured{
					tu.NewResource("apiextensions.crossplane.io/v1", "CompositeResourceDefinition", "xr1s.example.org").
						WithSpecField("group", "example.org").
						WithSpecField("names", map[string]interface{}{
							"kind":     "XR1",
							"plural":   "xr1s",
							"singular": "xr1",
						}).
						WithSpecField("versions", []interface{}{
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
						}).
						Build(),
					tu.NewResource("apiextensions.crossplane.io/v1", "CompositeResourceDefinition", "xr2s.example.org").
						WithSpecField("group", "example.org").
						WithSpecField("names", map[string]interface{}{
							"kind":     "XR2",
							"plural":   "xr2s",
							"singular": "xr2",
						}).
						WithSpecField("versions", []interface{}{
							map[string]interface{}{
								"name":    "v1",
								"served":  true,
								"storage": true,
							},
						}).
						Build(),
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

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultClusterClient{
				dynamicClient: tc.setup(),
				logger:        logging.NewLogrLogger(testr.New(t)),
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

	tests := map[string]struct {
		reason string
		setup  func() dynamic.Interface
		args   args
		want   want
	}{
		"NamespacedResourceFound": {
			reason: "Should return the resource when it exists in a namespace",
			setup: func() dynamic.Interface {
				// Use the resource builder to create test objects
				objects := []runtime.Object{
					tu.NewResource("example.org/v1", "ExampleResource", "test-resource").
						InNamespace("test-namespace").
						WithSpecField("property", "value").
						Build(),
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
				resource: tu.NewResource("example.org/v1", "ExampleResource", "test-resource").
					InNamespace("test-namespace").
					WithSpecField("property", "value").
					Build(),
			},
		},
		"ClusterScopedResourceFound": {
			reason: "Should return the resource when it exists at cluster scope",
			setup: func() dynamic.Interface {
				objects := []runtime.Object{
					tu.NewResource("example.org/v1", "ClusterResource", "test-cluster-resource").
						WithSpecField("property", "value").
						Build(),
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
				resource: tu.NewResource("example.org/v1", "ClusterResource", "test-cluster-resource").
					WithSpecField("property", "value").
					Build(),
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
					tu.NewResource("v1", "Endpoints", "test-endpoints").
						InNamespace("test-namespace").
						WithSpecField("subsets", []interface{}{
							map[string]interface{}{
								"addresses": []interface{}{
									map[string]interface{}{
										"ip": "192.168.1.1",
									},
								},
							},
						}).
						Build(),
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
				resource: tu.NewResource("v1", "Endpoints", "test-endpoints").
					InNamespace("test-namespace").
					WithSpecField("subsets", []interface{}{
						map[string]interface{}{
							"addresses": []interface{}{
								map[string]interface{}{
									"ip": "192.168.1.1",
								},
							},
						},
					}).
					Build(),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultClusterClient{
				dynamicClient: tc.setup(),
				logger:        logging.NewLogrLogger(testr.New(t)),
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

func TestClusterClient_DryRunApply(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = pkgv1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	type args struct {
		ctx context.Context
		obj *unstructured.Unstructured
	}

	type want struct {
		result *unstructured.Unstructured
		err    error
	}

	tests := map[string]struct {
		reason string
		setup  func() *tu.MockClusterClient
		args   args
		want   want
	}{
		"NamespacedResourceApplied": {
			reason: "Should successfully apply a namespaced resource",
			setup: func() *tu.MockClusterClient {
				return &tu.MockClusterClient{
					DryRunApplyFn: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
						// Create a modified copy of the input object
						result := obj.DeepCopy()
						result.SetResourceVersion("1000")
						return result, nil
					},
				}
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
			setup: func() *tu.MockClusterClient {
				return &tu.MockClusterClient{
					DryRunApplyFn: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
						// Create a modified copy of the input object
						result := obj.DeepCopy()
						result.SetResourceVersion("1000")
						return result, nil
					},
				}
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
		"ApplyError": {
			reason: "Should return error when apply fails",
			setup: func() *tu.MockClusterClient {
				return &tu.MockClusterClient{
					DryRunApplyFn: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
						return nil, errors.New("apply failed")
					},
				}
			},
			args: args{
				ctx: context.Background(),
				obj: tu.NewResource("example.org/v1", "ExampleResource", "test-resource").
					InNamespace("test-namespace").
					Build(),
			},
			want: want{
				result: nil,
				err:    errors.New("apply failed"),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create the mock client using the setup function
			c := tc.setup()

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

			// For successful cases, compare the original parts of results
			// We remove the resourceVersion before comparing since we set it in our test
			gotCopy := got.DeepCopy()
			if _, exists, _ := unstructured.NestedString(gotCopy.Object, "metadata", "resourceVersion"); exists {
				unstructured.RemoveNestedField(gotCopy.Object, "metadata", "resourceVersion")
			}

			wantCopy := tc.want.result.DeepCopy()
			if _, exists, _ := unstructured.NestedString(wantCopy.Object, "metadata", "resourceVersion"); exists {
				unstructured.RemoveNestedField(wantCopy.Object, "metadata", "resourceVersion")
			}

			if diff := cmp.Diff(wantCopy, gotCopy); diff != "" {
				t.Errorf("\n%s\nDryRunApply(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestClusterClient_GetResourcesByLabel(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = pkgv1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	tests := map[string]struct {
		reason string
		setup  func() dynamic.Interface
		args   struct {
			ctx       context.Context
			namespace string
			gvr       schema.GroupVersionResource
			selector  metav1.LabelSelector
		}
		want struct {
			resources []*unstructured.Unstructured
			err       error
		}
	}{
		"NoMatchingResources": {
			reason: "Should return empty list when no resources match selector",
			setup: func() dynamic.Interface {
				dc := fake.NewSimpleDynamicClientWithCustomListKinds(scheme,
					map[schema.GroupVersionResource]string{
						{Group: "example.org", Version: "v1", Resource: "resources"}: "ResourceList",
					})
				return dc
			},
			args: struct {
				ctx       context.Context
				namespace string
				gvr       schema.GroupVersionResource
				selector  metav1.LabelSelector
			}{
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
			want: struct {
				resources []*unstructured.Unstructured
				err       error
			}{
				resources: []*unstructured.Unstructured{},
			},
		},
		"MatchingResources": {
			reason: "Should return resources matching label selector",
			setup: func() dynamic.Interface {
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
				return fake.NewSimpleDynamicClient(scheme, objects...)
			},
			args: struct {
				ctx       context.Context
				namespace string
				gvr       schema.GroupVersionResource
				selector  metav1.LabelSelector
			}{
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
			want: struct {
				resources []*unstructured.Unstructured
				err       error
			}{
				resources: []*unstructured.Unstructured{
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
			args: struct {
				ctx       context.Context
				namespace string
				gvr       schema.GroupVersionResource
				selector  metav1.LabelSelector
			}{
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
			want: struct {
				resources []*unstructured.Unstructured
				err       error
			}{
				err: errors.New("cannot list resources for 'example.org/v1, Resource=resources' matching 'app=test': list error"),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultClusterClient{
				dynamicClient: tc.setup(),
				logger:        logging.NewLogrLogger(testr.New(t)),
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

func TestClusterClient_GetResourceTree(t *testing.T) {
	// Setup test context
	ctx := context.Background()

	// Create test XR
	xr := tu.NewResource("example.org/v1", "XExampleResource", "test-xr").
		WithSpecField("coolParam", "cool-value").
		Build()

	// Create composed resources that would be children of the XR
	composedResource1 := tu.NewResource("composed.org/v1", "ComposedResource", "child-1").
		WithCompositeOwner("test-xr").
		WithCompositionResourceName("resource-1").
		WithSpecField("param", "value-1").
		Build()

	composedResource2 := tu.NewResource("composed.org/v1", "ComposedResource", "child-2").
		WithCompositeOwner("test-xr").
		WithCompositionResourceName("resource-2").
		WithSpecField("param", "value-2").
		Build()

	// Create a test resource tree
	testResourceTree := &resource.Resource{
		Unstructured: *xr.DeepCopy(),
		Children: []*resource.Resource{
			{
				Unstructured: *composedResource1.DeepCopy(),
				Children:     []*resource.Resource{},
			},
			{
				Unstructured: *composedResource2.DeepCopy(),
				Children:     []*resource.Resource{},
			},
		},
	}

	tests := []struct {
		name         string
		clientSetup  func() *tu.MockClusterClient
		input        *unstructured.Unstructured
		expectOutput *resource.Resource
		expectError  bool
		errorPattern string
	}{
		{
			name: "SuccessfulResourceTreeFetch",
			clientSetup: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithGetResourceTree(func(ctx context.Context, root *unstructured.Unstructured) (*resource.Resource, error) {
						// Verify the input is our XR
						if root.GetName() != "test-xr" || root.GetKind() != "XExampleResource" {
							return nil, errors.New("unexpected input resource")
						}
						return testResourceTree, nil
					}).
					Build()
			},
			input:        xr,
			expectOutput: testResourceTree,
			expectError:  false,
		},
		{
			name: "ResourceTreeNotFound",
			clientSetup: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithGetResourceTree(func(ctx context.Context, root *unstructured.Unstructured) (*resource.Resource, error) {
						return nil, errors.New("resource tree not found")
					}).
					Build()
			},
			input:        xr,
			expectOutput: nil,
			expectError:  true,
			errorPattern: "resource tree not found",
		},
		{
			name: "EmptyResourceTree",
			clientSetup: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithGetResourceTree(func(ctx context.Context, root *unstructured.Unstructured) (*resource.Resource, error) {
						// Return an empty resource tree (just the root, no children)
						return &resource.Resource{
							Unstructured: *root.DeepCopy(),
							Children:     []*resource.Resource{},
						}, nil
					}).
					Build()
			},
			input: xr,
			expectOutput: &resource.Resource{
				Unstructured: *xr.DeepCopy(),
				Children:     []*resource.Resource{},
			},
			expectError: false,
		},
		{
			name: "NilInputResource",
			clientSetup: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithGetResourceTree(func(ctx context.Context, root *unstructured.Unstructured) (*resource.Resource, error) {
						if root == nil {
							return nil, errors.New("nil resource provided")
						}
						return nil, nil
					}).
					Build()
			},
			input:        nil,
			expectOutput: nil,
			expectError:  true,
			errorPattern: "nil resource provided",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create the client with our mock implementation
			client := tc.clientSetup()

			// Call the method we're testing
			got, err := client.GetResourceTree(ctx, tc.input)

			// Check for expected errors
			if tc.expectError {
				if err == nil {
					t.Errorf("GetResourceTree() expected error but got none")
					return
				}
				if tc.errorPattern != "" && !strings.Contains(err.Error(), tc.errorPattern) {
					t.Errorf("GetResourceTree() expected error containing %q, got: %v", tc.errorPattern, err)
				}
				return
			}

			// Check for unexpected errors
			if err != nil {
				t.Errorf("GetResourceTree() unexpected error: %v", err)
				return
			}

			// Verify the output matches expectations
			if diff := cmp.Diff(tc.expectOutput, got); diff != "" {
				t.Errorf("GetResourceTree() -want, +got:\n%s", diff)
			}

			// Verify that the tree structure is correct when expected
			if got != nil && tc.expectOutput != nil {
				// Verify root properties
				if diff := cmp.Diff(tc.expectOutput.Unstructured.GetName(), got.Unstructured.GetName()); diff != "" {
					t.Errorf("GetResourceTree() root resource name mismatch -want, +got:\n%s", diff)
				}

				// Verify child count
				if diff := cmp.Diff(len(tc.expectOutput.Children), len(got.Children)); diff != "" {
					t.Errorf("GetResourceTree() child count mismatch -want, +got:\n%s", diff)
				}

				// Verify children names if there are any
				if len(got.Children) > 0 {
					// Create maps of child names for easier comparison
					expectedNames := make(map[string]bool)
					actualNames := make(map[string]bool)

					for _, child := range tc.expectOutput.Children {
						expectedNames[child.Unstructured.GetName()] = true
					}

					for _, child := range got.Children {
						actualNames[child.Unstructured.GetName()] = true
					}

					// Check if any expected children are missing
					for name := range expectedNames {
						if !actualNames[name] {
							t.Errorf("GetResourceTree() missing expected child with name %s", name)
						}
					}

					// Check if there are any unexpected children
					for name := range actualNames {
						if !expectedNames[name] {
							t.Errorf("GetResourceTree() unexpected child with name %s", name)
						}
					}
				}
			}
		})
	}
}

// TestNewClusterClient tests the creation of a new DefaultClusterClient instance
func TestNewClusterClient(t *testing.T) {
	// Set up a test logger
	testLogger := logging.NewNopLogger()

	// Skip the nil config test because we can't easily mock the underlying functions
	// We'll just test the valid config case
	validConfig := &rest.Config{
		Host: "https://localhost:8080",
	}

	// Test without logger option
	_, err := NewClusterClient(validConfig)
	if err != nil {
		t.Errorf("NewClusterClient(...): unexpected error with valid config: %v", err)
	}

	// Test with logger option
	_, err = NewClusterClient(validConfig, WithLogger(testLogger))
	if err != nil {
		t.Errorf("NewClusterClient(...): unexpected error with valid config and logger: %v", err)
	}
}
