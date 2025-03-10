package diffprocessor

import (
	"context"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

// stubClusterClient is a stub implementation of the cc.ClusterClient interface
type stubClusterClient struct {
	findMatchingCompositionFn  func(*unstructured.Unstructured) (*apiextensionsv1.Composition, error)
	getExtraResourcesFn        func(context.Context, []schema.GroupVersionResource, []metav1.LabelSelector) ([]unstructured.Unstructured, error)
	getFunctionsFromPipelineFn func(*apiextensionsv1.Composition) ([]pkgv1.Function, error)
	getXRDSchemaFn             func(context.Context, *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error)
}

func (s *stubClusterClient) FindMatchingComposition(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
	if s.findMatchingCompositionFn != nil {
		return s.findMatchingCompositionFn(res)
	}
	return nil, errors.New("FindMatchingComposition not implemented")
}

func (s *stubClusterClient) GetExtraResources(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]unstructured.Unstructured, error) {
	if s.getExtraResourcesFn != nil {
		return s.getExtraResourcesFn(ctx, gvrs, selectors)
	}
	return nil, errors.New("GetExtraResources not implemented")
}

func (s *stubClusterClient) GetFunctionsFromPipeline(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
	if s.getFunctionsFromPipelineFn != nil {
		return s.getFunctionsFromPipelineFn(comp)
	}
	return nil, errors.New("GetFunctionsFromPipeline not implemented")
}

func (s *stubClusterClient) GetXRDSchema(ctx context.Context, res *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error) {
	if s.getXRDSchemaFn != nil {
		return s.getXRDSchemaFn(ctx, res)
	}
	return nil, errors.New("GetXRDSchema not implemented")
}

func (s *stubClusterClient) Initialize(ctx context.Context) error {
	return nil
}

// MockDiffProcessor creates a DiffProcessor with mocked behavior for testing
func MockDiffProcessor(client cc.ClusterClient, config *rest.Config, namespace string) *DiffProcessor {
	return &DiffProcessor{
		client:    client,
		config:    config,
		namespace: namespace,
	}
}

func TestDiffProcessor_ProcessResource(t *testing.T) {
	pipelineMode := apiextensionsv1.CompositionModePipeline

	// Create mocks with proper behavior
	mockCompositionNotFound := &stubClusterClient{
		findMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return nil, errors.New("composition not found")
		},
		// Define empty implementations for other methods to avoid nil pointer errors
		getExtraResourcesFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]unstructured.Unstructured, error) {
			return nil, errors.New("should not be called")
		},
		getFunctionsFromPipelineFn: func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
			return nil, errors.New("should not be called")
		},
		getXRDSchemaFn: func(ctx context.Context, res *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error) {
			return nil, errors.New("should not be called")
		},
	}

	mockExtraResourcesError := &stubClusterClient{
		findMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: &pipelineMode,
				},
			}, nil
		},
		getExtraResourcesFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]unstructured.Unstructured, error) {
			return nil, errors.New("failed to get extra resources")
		},
		getFunctionsFromPipelineFn: func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
			return nil, errors.New("should not be called")
		},
		getXRDSchemaFn: func(ctx context.Context, res *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error) {
			return nil, errors.New("should not be called")
		},
	}

	mockGetFunctionsError := &stubClusterClient{
		findMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: &pipelineMode,
				},
			}, nil
		},
		getExtraResourcesFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{}, nil
		},
		getFunctionsFromPipelineFn: func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
			return nil, errors.New("function not found")
		},
		getXRDSchemaFn: func(ctx context.Context, res *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error) {
			return nil, errors.New("should not be called")
		},
	}

	mockXRDSchemaError := &stubClusterClient{
		findMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: &pipelineMode,
				},
			}, nil
		},
		getExtraResourcesFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{}, nil
		},
		getFunctionsFromPipelineFn: func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
			return []pkgv1.Function{}, nil
		},
		getXRDSchemaFn: func(ctx context.Context, res *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error) {
			return nil, errors.New("XRD not found")
		},
	}

	type args struct {
		ctx context.Context
		res *unstructured.Unstructured
	}

	type want struct {
		err error
	}

	// For testing, we'll use the stub directly instead of trying to convert it
	// to a real cc.ClusterClient

	cases := map[string]struct {
		reason string
		stub   *stubClusterClient
		args   args
		want   want
	}{
		"CompositionNotFound": {
			reason: "Should return error when matching composition is not found",
			stub:   mockCompositionNotFound,
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
			stub:   mockExtraResourcesError,
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
			stub:   mockGetFunctionsError,
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
		"XRDSchemaError": {
			reason: "Should return error when getting XRD schema fails",
			stub:   mockXRDSchemaError,
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
				err: errors.Wrap(errors.New("XRD not found"), "cannot get XRD xrdSchema"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a DiffProcessor that uses our stub client directly
			p := &DiffProcessor{
				client:    tc.stub, // Use the stub directly as the client
				config:    &rest.Config{},
				namespace: "default",
			}

			err := p.ProcessResource(tc.args.ctx, tc.args.res)

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
	mockCompositionNotFound := &stubClusterClient{
		findMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return nil, errors.New("composition not found")
		},
		// Define empty implementations for other methods
		getExtraResourcesFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]unstructured.Unstructured, error) {
			return nil, errors.New("should not be called")
		},
		getFunctionsFromPipelineFn: func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
			return nil, errors.New("should not be called")
		},
		getXRDSchemaFn: func(ctx context.Context, res *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error) {
			return nil, errors.New("should not be called")
		},
	}

	mockMultipleErrors := &stubClusterClient{
		findMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return nil, errors.Errorf("composition not found for %s", res.GetName())
		},
		// Define empty implementations for other methods
		getExtraResourcesFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]unstructured.Unstructured, error) {
			return nil, errors.New("should not be called")
		},
		getFunctionsFromPipelineFn: func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
			return nil, errors.New("should not be called")
		},
		getXRDSchemaFn: func(ctx context.Context, res *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error) {
			return nil, errors.New("should not be called")
		},
	}

	mockNoErrors := &stubClusterClient{
		// Since this test has no resources, these functions shouldn't be called
		findMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return nil, errors.New("should not be called")
		},
		getExtraResourcesFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]unstructured.Unstructured, error) {
			return nil, errors.New("should not be called")
		},
		getFunctionsFromPipelineFn: func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
			return nil, errors.New("should not be called")
		},
		getXRDSchemaFn: func(ctx context.Context, res *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error) {
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

	// For testing, we'll use the stub directly

	cases := map[string]struct {
		reason string
		stub   *stubClusterClient
		args   args
		want   want
	}{
		"NoResources": {
			reason: "Should not return error when no resources are provided",
			stub:   mockNoErrors,
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
			stub:   mockCompositionNotFound,
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
			stub:   mockMultipleErrors,
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
			p := &DiffProcessor{
				client:    tc.stub, // Use the stub directly as the client
				config:    &rest.Config{},
				namespace: "default",
			}

			err := p.ProcessAll(tc.args.ctx, tc.args.resources)

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
			p := &DiffProcessor{}
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
