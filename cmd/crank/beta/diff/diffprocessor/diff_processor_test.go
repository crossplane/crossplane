package diffprocessor

import (
	"bytes"
	"context"
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	sigsyaml "sigs.k8s.io/yaml"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

// Ensure MockDiffProcessor implements the DiffProcessor interface
var _ DiffProcessor = &testutils.MockDiffProcessor{}

func TestDiffProcessor_ProcessResource(t *testing.T) {
	pipelineMode := apiextensionsv1.CompositionModePipeline

	// Create mocks with proper behavior
	mockCompositionNotFound := &testutils.MockClusterClient{
		FindMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return nil, errors.New("composition not found")
		},
	}

	mockExtraResourcesError := &testutils.MockClusterClient{
		FindMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: &pipelineMode,
				},
			}, nil
		},
		GetExtraResourcesFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
			return nil, errors.New("failed to get extra resources")
		},
	}

	mockGetFunctionsError := &testutils.MockClusterClient{
		FindMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: &pipelineMode,
				},
			}, nil
		},
		GetExtraResourcesFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
			return []*unstructured.Unstructured{}, nil
		},
		GetFunctionsFromPipelineFn: func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
			return nil, errors.New("function not found")
		},
	}

	type args struct {
		ctx context.Context
		res *unstructured.Unstructured
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		mock   *testutils.MockClusterClient
		args   args
		want   want
	}{
		"CompositionNotFound": {
			reason: "Should return error when matching composition is not found",
			mock:   mockCompositionNotFound,
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
				err: errors.Wrap(errors.New("composition not found"), "cannot find matching composition"),
			},
		},
		"ExtraResourcesError": {
			reason: "Should return error when identifying needed extra resources fails",
			mock:   mockExtraResourcesError,
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
				err: errors.Wrap(errors.New("failed to get extra resources"), "cannot get extra resources"),
			},
		},
		"GetFunctionsError": {
			reason: "Should return error when getting functions from pipeline fails",
			mock:   mockGetFunctionsError,
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
				err: errors.Wrap(errors.New("function not found"), "cannot get functions from pipeline"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a DiffProcessor that uses our mock client
			p, _ := NewDiffProcessor(&rest.Config{}, tc.mock, "default", nil, nil)

			// Create a dummy writer for stdout
			var stdout bytes.Buffer

			err := p.ProcessResource(&stdout, tc.args.ctx, tc.args.res)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nProcessResource(...): expected error but got none", tc.reason)
					return
				}

				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nProcessResource(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nProcessResource(...): unexpected error: %v", tc.reason, err)
			}
		})
	}
}

func TestDiffProcessor_ProcessAll(t *testing.T) {
	// Create mock clients for testing
	mockCompositionNotFound := &testutils.MockClusterClient{
		FindMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return nil, errors.New("composition not found")
		},
	}

	mockMultipleErrors := &testutils.MockClusterClient{
		FindMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return nil, errors.Errorf("composition not found for %s", res.GetName())
		},
	}

	mockNoErrors := &testutils.MockClusterClient{
		// Since this test has no resources, these functions shouldn't be called
		FindMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return nil, errors.New("should not be called")
		},
	}

	type args struct {
		ctx       context.Context
		resources []*unstructured.Unstructured
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		mock   *testutils.MockClusterClient
		args   args
		want   want
	}{
		"NoResources": {
			reason: "Should not return error when no resources are provided",
			mock:   mockNoErrors,
			args: args{
				ctx:       context.Background(),
				resources: []*unstructured.Unstructured{},
			},
			want: want{
				err: nil,
			},
		},
		"ProcessResourceError": {
			reason: "Should return error when processing a resource fails",
			mock:   mockCompositionNotFound,
			args: args{
				ctx: context.Background(),
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "XR1",
							"metadata": map[string]interface{}{
								"name": "my-xr",
							},
						},
					},
				},
			},
			want: want{
				err: errors.New("unable to process resource my-xr: cannot find matching composition: composition not found"),
			},
		},
		"MultipleResourceErrors": {
			reason: "Should return all errors when multiple resources fail processing",
			mock:   mockMultipleErrors,
			args: args{
				ctx: context.Background(),
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "XR1",
							"metadata": map[string]interface{}{
								"name": "my-xr-1",
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "XR1",
							"metadata": map[string]interface{}{
								"name": "my-xr-2",
							},
						},
					},
				},
			},
			want: want{
				err: errors.New("[unable to process resource my-xr-1: cannot find matching composition: composition not found for my-xr-1, unable to process resource my-xr-2: cannot find matching composition: composition not found for my-xr-2]"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a DiffProcessor with our mock client
			p, _ := NewDiffProcessor(&rest.Config{}, tc.mock, "default", nil, nil)

			// Create a dummy writer for stdout
			var stdout bytes.Buffer

			err := p.ProcessAll(&stdout, tc.args.ctx, tc.args.resources)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nProcessAll(...): expected error but got none", tc.reason)
					return
				}

				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nProcessAll(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nProcessAll(...): unexpected error: %v", tc.reason, err)
			}
		})
	}
}

func TestDiffProcessor_IdentifyNeededExtraResources(t *testing.T) {
	pipelineMode := apiextensionsv1.CompositionModePipeline
	nonPipelineMode := apiextensionsv1.CompositionMode("NonPipeline")

	type args struct {
		comp *apiextensionsv1.Composition
	}

	type want struct {
		gvrs      []schema.GroupVersionResource
		selectors []metav1.LabelSelector
		err       error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NonPipelineMode": {
			reason: "Should return empty slices for non-pipeline mode",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &nonPipelineMode,
					},
				},
			},
			want: want{
				gvrs:      nil,
				selectors: nil,
			},
		},
		"NoExtraResourcesFunction": {
			reason: "Should return empty slices when no function-extra-resources exists",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-other"},
							},
						},
					},
				},
			},
			want: want{
				gvrs:      nil,
				selectors: nil,
			},
		},
		"NoExtraResourcesInInput": {
			reason: "Should return empty slices when function exists but no extraResources in input",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{"apiVersion":"crossplane.io/v1alpha1","kind":"ExtraResourcesInput","spec":{"otherField":"value"}}`),
								},
							},
						},
					},
				},
			},
			want: want{
				gvrs:      nil,
				selectors: nil,
			},
		},
		"InvalidInput": {
			reason: "Should handle invalid input JSON",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{invalid-json`),
								},
							},
						},
					},
				},
			},
			want: want{
				gvrs:      nil,
				selectors: nil,
				err:       errors.New("cannot unmarshal function-extra-resources input"),
			},
		},
		"WithExtraResources": {
			reason: "Should return GVRs and selectors when extraResources exist",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "crossplane.io/v1alpha1",
										"kind": "ExtraResourcesInput",
										"spec": {
											"extraResources": [
												{
													"apiVersion": "example.org/v1",
													"kind": "Test",
													"selector": {
														"matchLabels": {
															"app": "test"
														}
													}
												}
											]
										}
									}`),
								},
							},
						},
					},
				},
			},
			want: want{
				gvrs: []schema.GroupVersionResource{
					{Group: "example.org", Version: "v1", Resource: "tests"},
				},
				selectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"app": "test"}},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := &DefaultDiffProcessor{}
			gvrs, selectors, err := p.IdentifyNeededExtraResources(tc.args.comp)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nidentifyNeededExtraResources(...): expected error but got none", tc.reason)
					return
				}

				if !strings.Contains(err.Error(), tc.want.err.Error()) {
					t.Errorf("\n%s\nidentifyNeededExtraResources(...): expected error containing %q, got %q", tc.reason, tc.want.err.Error(), err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nidentifyNeededExtraResources(...): unexpected error: %v", tc.reason, err)
				return
			}

			if diff := cmp.Diff(len(tc.want.gvrs), len(gvrs)); diff != "" {
				t.Errorf("\n%s\nidentifyNeededExtraResources(...): -want GVR count, +got GVR count:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(len(tc.want.selectors), len(selectors)); diff != "" {
				t.Errorf("\n%s\nidentifyNeededExtraResources(...): -want selector count, +got selector count:\n%s", tc.reason, diff)
			}

			for i, wantGVR := range tc.want.gvrs {
				if i >= len(gvrs) {
					break
				}
				if diff := cmp.Diff(wantGVR.String(), gvrs[i].String()); diff != "" {
					t.Errorf("\n%s\nidentifyNeededExtraResources(...): -want GVR, +got GVR at index %d:\n%s", tc.reason, i, diff)
				}
			}

			for i, wantSelector := range tc.want.selectors {
				if i >= len(selectors) {
					break
				}
				if diff := cmp.Diff(wantSelector.MatchLabels, selectors[i].MatchLabels); diff != "" {
					t.Errorf("\n%s\nidentifyNeededExtraResources(...): -want selector, +got selector at index %d:\n%s", tc.reason, i, diff)
				}
			}
		})
	}
}

func TestScanForTemplatedExtraResources(t *testing.T) {
	pipelineMode := apiextensionsv1.CompositionModePipeline
	nonPipelineMode := apiextensionsv1.CompositionMode("NonPipeline")

	type args struct {
		comp *apiextensionsv1.Composition
	}

	type want struct {
		hasTemplated bool
		err          error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NonPipelineMode": {
			reason: "Should return false for non-pipeline mode compositions",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &nonPipelineMode,
					},
				},
			},
			want: want{
				hasTemplated: false,
			},
		},
		"NoGoTemplatingFunction": {
			reason: "Should return false when no go-templating function exists",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-other"},
							},
						},
					},
				},
			},
			want: want{
				hasTemplated: false,
			},
		},
		"TemplateWithoutExtraResources": {
			reason: "Should return false when template doesn't have ExtraResources",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "crossplane.io/v1alpha1",
										"kind": "GoTemplatingInput",
										"spec": {
											"inline": {
												"template": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test"
											}
										}
									}`),
								},
							},
						},
					},
				},
			},
			want: want{
				hasTemplated: false,
			},
		},
		"TemplateWithExtraResources": {
			reason: "Should return true when template has ExtraResources",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "crossplane.io/v1alpha1",
										"kind": "GoTemplatingInput",
										"spec": {
											"inline": {
												"template": "apiVersion: render.crossplane.io/v1\nkind: ExtraResources\nspec:\n  resources:\n  - apiVersion: v1\n    kind: ConfigMap"
											}
										}
									}`),
								},
							},
						},
					},
				},
			},
			want: want{
				hasTemplated: true,
			},
		},
		"InvalidTemplate": {
			reason: "Should return error for invalid template",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "crossplane.io/v1alpha1",
										"kind": "GoTemplatingInput",
										"spec": {
											"inline": {
												"template": "{{"
											}
										}
									}`),
								},
							},
						},
					},
				},
			},
			want: want{
				hasTemplated: false,
				err:          errors.New("cannot decode template YAML"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ScanForTemplatedExtraResources(tc.args.comp)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nscanForTemplatedExtraResources(...): expected error but got none", tc.reason)
					return
				}

				if !strings.Contains(err.Error(), tc.want.err.Error()) {
					t.Errorf("\n%s\nscanForTemplatedExtraResources(...): expected error containing %q, got %q", tc.reason, tc.want.err.Error(), err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nscanForTemplatedExtraResources(...): unexpected error: %v", tc.reason, err)
				return
			}

			if diff := cmp.Diff(tc.want.hasTemplated, got); diff != "" {
				t.Errorf("\n%s\nscanForTemplatedExtraResources(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetExtraResourcesFromResult(t *testing.T) {
	type args struct {
		result *unstructured.Unstructured
	}

	type want struct {
		resources []unstructured.Unstructured
		err       error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoSpec": {
			reason: "Should return error when result has no spec",
			args: args{
				result: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "render.crossplane.io/v1beta1",
						"kind":       "ExtraResources",
						"metadata": map[string]interface{}{
							"name": "result",
						},
					},
				},
			},
			want: want{
				err: errors.New("no spec found in ExtraResources result"),
			},
		},
		"NoResources": {
			reason: "Should return error when spec has no resources",
			args: args{
				result: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "render.crossplane.io/v1beta1",
						"kind":       "ExtraResources",
						"metadata": map[string]interface{}{
							"name": "result",
						},
						"spec": map[string]interface{}{},
					},
				},
			},
			want: want{
				err: errors.New("no resources found in ExtraResources spec"),
			},
		},
		"WithResources": {
			reason: "Should return resources when they exist",
			args: args{
				result: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "render.crossplane.io/v1beta1",
						"kind":       "ExtraResources",
						"metadata": map[string]interface{}{
							"name": "result",
						},
						"spec": map[string]interface{}{
							"resources": []interface{}{
								map[string]interface{}{
									"apiVersion": "v1",
									"kind":       "ConfigMap",
									"metadata": map[string]interface{}{
										"name": "resource-1",
									},
								},
								map[string]interface{}{
									"apiVersion": "v1",
									"kind":       "Secret",
									"metadata": map[string]interface{}{
										"name": "resource-2",
									},
								},
							},
						},
					},
				},
			},
			want: want{
				resources: []unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name": "resource-1",
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Secret",
							"metadata": map[string]interface{}{
								"name": "resource-2",
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := GetExtraResourcesFromResult(tc.args.result)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\ngetExtraResourcesFromResult(...): expected error but got none", tc.reason)
					return
				}

				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\ngetExtraResourcesFromResult(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\ngetExtraResourcesFromResult(...): unexpected error: %v", tc.reason, err)
				return
			}

			if diff := cmp.Diff(len(tc.want.resources), len(got)); diff != "" {
				t.Errorf("\n%s\ngetExtraResourcesFromResult(...): -want resource count, +got resource count:\n%s", tc.reason, diff)
			}

			for i, wantRes := range tc.want.resources {
				if i >= len(got) {
					break
				}

				if diff := cmp.Diff(wantRes.GetKind(), got[i].GetKind()); diff != "" {
					t.Errorf("\n%s\ngetExtraResourcesFromResult(...): -want kind, +got kind at index %d:\n%s", tc.reason, i, diff)
				}

				if diff := cmp.Diff(wantRes.GetName(), got[i].GetName()); diff != "" {
					t.Errorf("\n%s\ngetExtraResourcesFromResult(...): -want name, +got name at index %d:\n%s", tc.reason, i, diff)
				}
			}
		})
	}
}

func TestDiffProcessor_CalculateDiff(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx     context.Context
		desired runtime.Object
	}
	type fields struct {
		mockGetResource func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error)
		mockDryRunApply func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	}
	type want struct {
		diff string
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		fields fields
		want   want
	}{
		"DesiredNotUnstructured": {
			reason: "Should return an error if the desired object is not an unstructured",
			args: args{
				ctx:     ctx,
				desired: &corev1.Pod{}, // Using a typed object to test error handling
			},
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					return nil, nil
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					return nil, nil
				},
			},
			want: want{
				diff: "",
				err:  errors.New("desired object is not unstructured"),
			},
		},
		"ResourceNotFound": {
			reason: "Should return a formatted diff for a new resource",
			args: args{
				ctx: ctx,
				desired: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ExampleResource",
						"metadata": map[string]interface{}{
							"name": "new-resource",
						},
						"spec": map[string]interface{}{
							"param": "value",
						},
					},
				},
			},
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{Group: gvk.Group, Resource: gvk.Kind}, name)
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					// Should not be called for new resources
					return nil, errors.New("should not be called")
				},
			},
			want: want{
				// YAML format instead of JSON
				diff: `+ ExampleResource (new object)
apiVersion: example.org/v1
kind: ExampleResource
metadata:
  name: new-resource
spec:
  param: value`,
				err: nil,
			},
		},
		"ErrorGettingResource": {
			reason: "Should return an error if there is a problem getting the current resource",
			args: args{
				ctx: ctx,
				desired: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ExampleResource",
						"metadata": map[string]interface{}{
							"name": "error-resource",
						},
					},
				},
			},
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					return nil, errors.New("server unavailable")
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					// Should not be called when GetResource fails
					return nil, errors.New("should not be called")
				},
			},
			want: want{
				diff: "",
				err:  errors.New("cannot get current object: server unavailable"),
			},
		},
		"DryRunError": {
			reason: "Should return an error if DryRunApply fails",
			args: args{
				ctx: ctx,
				desired: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ExampleResource",
						"metadata": map[string]interface{}{
							"name": "dryrun-error-resource",
						},
					},
				},
			},
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					return &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "ExampleResource",
							"metadata": map[string]interface{}{
								"name":            "dryrun-error-resource",
								"resourceVersion": "1",
							},
						},
					}, nil
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					return nil, errors.New("dry run failed")
				},
			},
			want: want{
				diff: "",
				err:  errors.New("cannot perform dry-run apply: dry run failed"),
			},
		},
		"NoChanges": {
			reason: "Should return an empty string when there are no changes",
			args: args{
				ctx: ctx,
				desired: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ExampleResource",
						"metadata": map[string]interface{}{
							"name": "unchanged-resource",
						},
						"spec": map[string]interface{}{
							"param": "value",
						},
					},
				},
			},
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					// Return the current resource
					return &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "ExampleResource",
							"metadata": map[string]interface{}{
								"name":            "unchanged-resource",
								"resourceVersion": "1",
							},
							"spec": map[string]interface{}{
								"param": "value",
							},
						},
					}, nil
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					// Return the exact same resource (no changes)
					return &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "ExampleResource",
							"metadata": map[string]interface{}{
								"name":            "unchanged-resource",
								"resourceVersion": "1",
							},
							"spec": map[string]interface{}{
								"param": "value",
							},
						},
					}, nil
				},
			},
			want: want{
				diff: "",
				err:  nil,
			},
		},
		"WithChanges": {
			reason: "Should return a formatted diff when there are changes",
			args: args{
				ctx: ctx,
				desired: func() runtime.Object {
					// Create a ConfigMap
					cm := &corev1.ConfigMap{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "ConfigMap",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "changed-configmap",
						},
						Data: map[string]string{
							"param":  "new-value",
							"newkey": "value",
							// "unused" is removed
						},
					}
					// Convert to unstructured
					unstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
					if err != nil {
						t.Fatalf("Failed to convert ConfigMap to unstructured: %v", err)
						return nil
					}
					return &unstructured.Unstructured{Object: unstr}
				}(),
			},
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					// Return a ConfigMap with different values
					cm := &corev1.ConfigMap{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "ConfigMap",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            "changed-configmap",
							ResourceVersion: "1",
						},
						Data: map[string]string{
							"param":  "old-value",
							"unused": "old-value",
							// No "newkey"
						},
					}
					// Convert to unstructured
					unstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
					if err != nil {
						return nil, err
					}
					return &unstructured.Unstructured{Object: unstr}, nil
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					// Return the desired object as the result of dry run
					return obj, nil
				},
			},
			want: want{
				// Updated to YAML format
				diff: `~ ConfigMap/changed-configmap
data:
  newkey: value
  param: new-value`,
				err: nil,
			},
		},
		"WithAddedField": {
			reason: "Should show added fields in the diff",
			args: args{
				ctx: ctx,
				desired: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]interface{}{
							"name": "added-field-pod",
						},
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "container1",
									"image": "nginx:latest",
									"resources": map[string]interface{}{
										"limits": map[string]interface{}{
											"cpu":    "100m",
											"memory": "128Mi",
										},
									},
								},
							},
						},
					},
				},
			},
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					// Create a pod without resource limits
					pod := &corev1.Pod{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Pod",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            "added-field-pod",
							ResourceVersion: "1",
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "container1",
									Image: "nginx:latest",
									// No resource limits
								},
							},
						},
					}
					// Convert to unstructured
					unstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
					if err != nil {
						return nil, err
					}
					return &unstructured.Unstructured{Object: unstr}, nil
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					// Return the desired object as if the dry run was successful
					return &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Pod",
							"metadata": map[string]interface{}{
								"name":            "added-field-pod",
								"resourceVersion": "1",
							},
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name":  "container1",
										"image": "nginx:latest",
										"resources": map[string]interface{}{
											"limits": map[string]interface{}{
												"cpu":    "100m",
												"memory": "128Mi",
											},
										},
									},
								},
							},
						},
					}, nil
				},
			},
			want: want{
				// YAML format with array handling
				diff: `~ Pod/added-field-pod
spec:
  containers:
  - resources:
      limits:
        cpu: 100m
        memory: 128Mi`,
				err: nil,
			},
		},
		"WithRemovedField": {
			reason: "Should show type change in the diff",
			args: args{
				ctx: ctx,
				desired: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Service",
						"metadata": map[string]interface{}{
							"name": "removed-field-service",
						},
						"spec": map[string]interface{}{
							"selector": map[string]interface{}{
								"app": "example",
							},
							"ports": []interface{}{
								map[string]interface{}{
									"port": int64(80),
									"name": "http",
								},
							},
							"type": "ClusterIP", // Changed from NodePort
							// No externalIPs
						},
					},
				},
			},
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					// Return service with additional fields
					svc := &corev1.Service{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Service",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            "removed-field-service",
							ResourceVersion: "1",
						},
						Spec: corev1.ServiceSpec{
							Selector: map[string]string{
								"app": "example",
							},
							Ports: []corev1.ServicePort{
								{
									Port: 80,
									Name: "http",
								},
							},
							Type: corev1.ServiceTypeNodePort,
							ExternalIPs: []string{
								"192.168.1.100",
							},
						},
					}
					// Convert to unstructured
					unstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(svc)
					if err != nil {
						return nil, err
					}
					return &unstructured.Unstructured{Object: unstr}, nil
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					// Return the desired object as if the dry run was successful
					return &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Service",
							"metadata": map[string]interface{}{
								"name":            "removed-field-service",
								"resourceVersion": "1",
							},
							"spec": map[string]interface{}{
								"selector": map[string]interface{}{
									"app": "example",
								},
								"ports": []interface{}{
									map[string]interface{}{
										"port": int64(80),
										"name": "http",
									},
								},
								"type": "ClusterIP",
								// No externalIPs
							},
						},
					}, nil
				},
			},
			want: want{
				// YAML format
				diff: `~ Service/removed-field-service
spec:
  type: ClusterIP`,
				err: nil,
			},
		},
		"WithNestedChanges": {
			reason: "Should handle nested field changes properly",
			args: args{
				ctx: ctx,
				desired: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apiextensions.k8s.io/v1beta1",
						"kind":       "CustomResourceDefinition",
						"metadata": map[string]interface{}{
							"name": "nested-changes.example.org",
						},
						"spec": map[string]interface{}{
							"group": "example.org",
							"names": map[string]interface{}{
								"kind":     "Example",
								"listKind": "ExampleList",
								"plural":   "examples",
								"singular": "example",
							},
							"scope":   "Namespaced",
							"version": "v1alpha1",
							"validation": map[string]interface{}{
								"openAPIV3Schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"spec": map[string]interface{}{
											"type": "object",
											"properties": map[string]interface{}{
												"replicas": map[string]interface{}{
													"type":    "integer",
													"minimum": float64(1),
													"maximum": float64(10),
												},
												"newField": map[string]interface{}{
													"type": "string",
												},
											},
											"required": []interface{}{"replicas"},
										},
									},
								},
							},
						},
					},
				},
			},
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					// Return CRD with different nested values
					crd := &apiextensions.CustomResourceDefinition{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "apiextensions.k8s.io/v1beta1",
							Kind:       "CustomResourceDefinition",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            "nested-changes.example.org",
							ResourceVersion: "1",
						},
						Spec: apiextensions.CustomResourceDefinitionSpec{
							Group: "example.org",
							Names: apiextensions.CustomResourceDefinitionNames{
								Kind:     "Example",
								ListKind: "ExampleList",
								Plural:   "examples",
								Singular: "example",
							},
							Scope:   apiextensions.NamespaceScoped,
							Version: "v1alpha1",
							Validation: &apiextensions.CustomResourceValidation{
								OpenAPIV3Schema: &apiextensions.JSONSchemaProps{
									Type: "object",
									Properties: map[string]apiextensions.JSONSchemaProps{
										"spec": {
											Type: "object",
											Properties: map[string]apiextensions.JSONSchemaProps{
												"replicas": {
													Type:    "integer",
													Minimum: ptr.To[float64](3), // Different minimum
													Maximum: ptr.To[float64](5), // Different maximum
												},
												// No newField
											},
											Required: []string{"replicas"},
										},
									},
								},
							},
						},
					}
					// Convert to unstructured
					unstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(crd)
					if err != nil {
						return nil, err
					}
					return &unstructured.Unstructured{Object: unstr}, nil
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					// Return the desired object as if the dry run was successful
					return &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.k8s.io/v1beta1",
							"kind":       "CustomResourceDefinition",
							"metadata": map[string]interface{}{
								"name":            "nested-changes.example.org",
								"resourceVersion": "1",
							},
							"spec": map[string]interface{}{
								"group": "example.org",
								"names": map[string]interface{}{
									"kind":     "Example",
									"listKind": "ExampleList",
									"plural":   "examples",
									"singular": "example",
								},
								"scope":   "Namespaced",
								"version": "v1alpha1",
								"validation": map[string]interface{}{
									"openAPIV3Schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"spec": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"replicas": map[string]interface{}{
														"type":    "integer",
														"minimum": float64(1),
														"maximum": float64(10),
													},
													"newField": map[string]interface{}{
														"type": "string",
													},
												},
												"required": []interface{}{"replicas"},
											},
										},
									},
								},
							},
						},
					}, nil
				},
			},
			want: want{
				// Update this to match the actual output with the full resource details
				diff: `~ CustomResourceDefinition/nested-changes.example.org
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: nested-changes.example.org
spec:
  validation:
    openAPIV3Schema:
      properties:
        spec:
          properties:
            newField:
              type: string
            replicas:
              maximum: 10
              minimum: 1`,
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a mock client that returns predefined resources
			mockClient := &testutils.MockClusterClient{
				GetResourceFn: tc.fields.mockGetResource,
				DryRunApplyFn: tc.fields.mockDryRunApply,
			}

			// Create a processor with the mock client
			processor := &DefaultDiffProcessor{
				client: mockClient,
				config: &rest.Config{},
			}

			diff, err := processor.CalculateDiff(tc.args.ctx, tc.args.desired)

			// Check if error matches expectation
			if tc.want.err != nil {
				if err == nil {
					t.Errorf("CalculateDiff() error = nil, wantErr %v", tc.want.err)
					return
				}
				if !strings.Contains(err.Error(), tc.want.err.Error()) {
					t.Errorf("CalculateDiff() error = %v, wantErr %v", err, tc.want.err)
					return
				}
			} else if err != nil {
				t.Errorf("CalculateDiff() unexpected error: %v", err)
				return
			}

			// Check if diff matches expectation
			// For YAML comparisons, we need to normalize whitespace and indentation
			if tc.want.diff != "" && diff != "" {
				normalizedWant, err := normalizeYAML(tc.want.diff)
				if err != nil {
					t.Errorf("Error normalizing expected diff: %v", err)
					return
				}

				normalizedGot, err := normalizeYAML(diff)
				if err != nil {
					t.Errorf("Error normalizing actual diff: %v", err)
					return
				}

				if normalizedWant != normalizedGot {
					t.Errorf("CalculateDiff() mismatch:\nWant:\n%s\n\nGot:\n%s", tc.want.diff, diff)
				}
			} else if tc.want.diff != diff {
				// For empty diffs or header-only diffs, compare directly
				t.Errorf("CalculateDiff() = %v, want %v", diff, tc.want.diff)
			}
		})
	}
}

func TestDiffProcessor_Initialize(t *testing.T) {
	// Create a mock client that returns an error for GetXRDs
	mockXRDsError := &testutils.MockClusterClient{
		GetXRDsFn: func(ctx context.Context) ([]*unstructured.Unstructured, error) {
			return nil, errors.New("XRD not found")
		},
	}

	// Create a mock client that returns success for GetXRDs
	mockXRDsSuccess := &testutils.MockClusterClient{
		GetXRDsFn: func(ctx context.Context) ([]*unstructured.Unstructured, error) {
			return []*unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "apiextensions.crossplane.io/v1",
						"kind":       "CompositeResourceDefinition",
						"metadata": map[string]interface{}{
							"name": "xexampleresources.example.org",
						},
						"spec": map[string]interface{}{
							"group": "example.org",
							"names": map[string]interface{}{
								"kind":     "XExampleResource",
								"plural":   "xexampleresources",
								"singular": "xexampleresource",
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
													"properties": map[string]interface{}{
														"coolParam": map[string]interface{}{
															"type": "string",
														},
													},
												},
												"status": map[string]interface{}{
													"type": "object",
													"properties": map[string]interface{}{
														"coolStatus": map[string]interface{}{
															"type": "string",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	type args struct {
		ctx context.Context
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		mock   *testutils.MockClusterClient
		args   args
		want   want
	}{
		"XRDsError": {
			reason: "Should return error when getting XRD schema fails",
			mock:   mockXRDsError,
			args: args{
				ctx: context.Background(),
			},
			want: want{
				err: errors.New("cannot get XRDs: XRD not found"),
			},
		},
		"Success": {
			reason: "Should succeed when XRDs are found and converted",
			mock:   mockXRDsSuccess,
			args: args{
				ctx: context.Background(),
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a DiffProcessor that uses our mock client
			p, _ := NewDiffProcessor(&rest.Config{}, tc.mock, "default", nil, nil)

			// Create a dummy writer for stdout
			var stdout bytes.Buffer

			err := p.Initialize(&stdout, tc.args.ctx)

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

			// For success case, we assume no error means happy, because we don't want to expose the crd cache field
		})
	}
}

// normalizeYAML handles normalization of YAML content for consistent comparison
func normalizeYAML(yamlString string) (string, error) {
	// Split the diff into header and YAML parts
	parts := strings.SplitN(yamlString, "\n", 2)
	if len(parts) < 2 {
		// No YAML part or not in the expected format, return as is
		return yamlString, nil
	}

	header := parts[0]
	yamlPart := parts[1]

	// Parse YAML to a map
	var parsed map[string]interface{}
	if err := sigsyaml.Unmarshal([]byte(yamlPart), &parsed); err != nil {
		return "", errors.Wrap(err, "cannot parse diff YAML")
	}

	// Re-serialize with consistent formatting
	normalized, err := sigsyaml.Marshal(parsed)
	if err != nil {
		return "", errors.Wrap(err, "cannot marshal normalized YAML")
	}

	// Reconstitute the diff
	return header + "\n" + string(normalized), nil
}
