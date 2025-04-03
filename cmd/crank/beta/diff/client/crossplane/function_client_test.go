package crossplane

import (
	"context"
	"strings"
	"testing"

	"strings"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
)

var _ FunctionClient = (*tu.MockFunctionClient)(nil)

// Fix for the FunctionClient tests.
func TestDefaultFunctionClient_GetFunctionsFromPipeline(t *testing.T) {
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
		reason       string
		fields       fields
		mockResource *tu.MockResourceClient
		args         args
		want         want
	}{
		"NonPipelineMode": {
			reason: "Should throw an error when composition is not in pipeline mode",
			fields: fields{
				functions: map[string]pkgv1.Function{},
			},
			mockResource: tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				Build(),
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &nonPipelineMode,
					},
				},
			},
			want: want{
				err: errors.New("unsupported composition Mode 'NonPipeline'; supported types are [Pipeline]"),
			},
		},
		"NoModeSpecified": {
			reason: "Should throw an error when composition mode is not specified",
			fields: fields{
				functions: map[string]pkgv1.Function{},
			},
			mockResource: tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				Build(),
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: nil,
					},
				},
			},
			want: want{
				err: errors.New("unsupported Composition; no Mode found"),
			},
		},
		"EmptyPipeline": {
			reason: "Should return empty slice for empty pipeline",
			fields: fields{
				functions: map[string]pkgv1.Function{},
			},
			mockResource: tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				Build(),
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
			mockResource: tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				Build(),
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
			mockResource: tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				Build(),
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
			mockResource: tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				Build(),
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

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultFunctionClient{
				resourceClient: tt.mockResource,
				functions:      tt.fields.functions,
				logger:         tu.TestLogger(t, false),
			}

			got, err := c.GetFunctionsFromPipeline(tt.args.comp)

			if tt.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nGetFunctionsFromPipeline(...): expected error but got none", tt.reason)
					return
				}

				if diff := cmp.Diff(tt.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nGetFunctionsFromPipeline(...): -want error, +got error:\n%s", tt.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetFunctionsFromPipeline(...): unexpected error: %v", tt.reason, err)
				return
			}

			if tt.want.functions == nil {
				if got != nil {
					t.Errorf("\n%s\nGetFunctionsFromPipeline(...): expected nil functions, got %v", tt.reason, got)
				}
				return
			}

			if diff := cmp.Diff(len(tt.want.functions), len(got)); diff != "" {
				t.Errorf("\n%s\nGetFunctionsFromPipeline(...): -want function count, +got function count:\n%s", tt.reason, diff)
			}

			// Check each function matches what we expect
			for i, wantFn := range tt.want.functions {
				if i >= len(got) {
					break
				}
				if diff := cmp.Diff(wantFn.GetName(), got[i].GetName()); diff != "" {
					t.Errorf("\n%s\nGetFunctionsFromPipeline(...): -want function name, +got function name at index %d:\n%s", tt.reason, i, diff)
				}
			}
		})
	}
}

func TestDefaultFunctionClient_ListFunctions(t *testing.T) {
	ctx := context.Background()

	// Create test functions
	fn1 := pkgv1.Function{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "pkg.crossplane.io/v1",
			Kind:       "Function",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "function-1",
		},
		Spec: pkgv1.FunctionSpec{
			PackageSpec: pkgv1.PackageSpec{
				Package: "registry.upbound.io/crossplane/function-patch:v0.1.0",
			},
		},
	}

	fn2 := pkgv1.Function{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "pkg.crossplane.io/v1",
			Kind:       "Function",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "function-2",
		},
		Spec: pkgv1.FunctionSpec{
			PackageSpec: pkgv1.PackageSpec{
				Package: "registry.upbound.io/crossplane/function-go-templating:v0.1.0",
			},
		},
	}

	// Convert to unstructured for testing
	u1 := &un.Unstructured{}
	obj1, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&fn1)
	u1.SetUnstructuredContent(obj1)
	u1.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "pkg.crossplane.io",
		Version: "v1",
		Kind:    "Function",
	})

	u2 := &un.Unstructured{}
	obj2, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&fn2)
	u2.SetUnstructuredContent(obj2)
	u2.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "pkg.crossplane.io",
		Version: "v1",
		Kind:    "Function",
	})

	tests := map[string]struct {
		reason       string
		mockResource tu.MockResourceClient
		want         []pkgv1.Function
		wantErr      bool
		errSubstring string
	}{
		"SuccessfulList": {
			reason: "Should return functions when list succeeds",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResources(func(_ context.Context, gvk schema.GroupVersionKind, _ string) ([]*un.Unstructured, error) {
					if gvk.Group == "pkg.crossplane.io" && gvk.Kind == "Function" {
						return []*un.Unstructured{u1, u2}, nil
					}
					return nil, errors.New("unexpected GVK")
				}).
				Build(),
			want:    []pkgv1.Function{fn1, fn2},
			wantErr: false,
		},
		"EmptyList": {
			reason: "Should return empty list when no functions exist",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			want:    []pkgv1.Function{},
			wantErr: false,
		},
		"ListError": {
			reason: "Should return error when list fails",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResourcesFailure("list error").
				Build(),
			want:         nil,
			wantErr:      true,
			errSubstring: "cannot list functions from cluster",
		},
		"ConversionError": {
			reason: "Should return error when conversion fails",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResources(func(_ context.Context, gvk schema.GroupVersionKind, _ string) ([]*un.Unstructured, error) {
					// Create an invalid unstructured that will fail conversion
					invalid := &un.Unstructured{}
					invalid.SetGroupVersionKind(gvk)
					invalid.SetName("invalid")
					// Put invalid data to force conversion failure
					invalid.Object["spec"] = 123 // This will cause conversion to fail since spec should be a map
					return []*un.Unstructured{invalid}, nil
				}).
				Build(),
			want:         nil,
			wantErr:      true,
			errSubstring: "cannot convert unstructured to Function",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultFunctionClient{
				resourceClient: &tt.mockResource,
				logger:         tu.TestLogger(t, false),
				functions:      make(map[string]pkgv1.Function),
			}

			got, err := c.ListFunctions(ctx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("\n%s\nListFunctions(): expected error but got none", tt.reason)
					return
				}
				if tt.errSubstring != "" && !strings.Contains(err.Error(), tt.errSubstring) {
					t.Errorf("\n%s\nListFunctions(): expected error containing %q, got %q", tt.reason, tt.errSubstring, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nListFunctions(): unexpected error: %v", tt.reason, err)
				return
			}

			if diff := cmp.Diff(len(tt.want), len(got)); diff != "" {
				t.Errorf("\n%s\nListFunctions(): -want count, +got count:\n%s", tt.reason, diff)
			}

			// Verify functions are returned correctly
			if len(got) > 0 {
				// Create maps of function names for comparison
				wantFns := make(map[string]bool)
				gotFns := make(map[string]bool)

				for _, fn := range tt.want {
					wantFns[fn.GetName()] = true
				}

				for _, fn := range got {
					gotFns[fn.GetName()] = true
				}

				// Check for missing functions
				for name := range wantFns {
					if !gotFns[name] {
						t.Errorf("\n%s\nListFunctions(): missing expected function with name %s", tt.reason, name)
					}
				}

				// Check for unexpected functions
				for name := range gotFns {
					if !wantFns[name] {
						t.Errorf("\n%s\nListFunctions(): unexpected function with name %s", tt.reason, name)
					}
				}
			}
		})
	}
}

func TestDefaultFunctionClient_Initialize(t *testing.T) {
	ctx := context.Background()

	// Create test functions
	fn1 := pkgv1.Function{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "pkg.crossplane.io/v1",
			Kind:       "Function",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "function-1",
		},
	}

	fn2 := pkgv1.Function{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "pkg.crossplane.io/v1",
			Kind:       "Function",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "function-2",
		},
	}

	// Convert to unstructured for testing
	u1 := &un.Unstructured{}
	obj1, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&fn1)
	u1.SetUnstructuredContent(obj1)
	u1.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "pkg.crossplane.io",
		Version: "v1",
		Kind:    "Function",
	})

	u2 := &un.Unstructured{}
	obj2, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&fn2)
	u2.SetUnstructuredContent(obj2)
	u2.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "pkg.crossplane.io",
		Version: "v1",
		Kind:    "Function",
	})

	tests := map[string]struct {
		reason       string
		mockResource tu.MockResourceClient
		wantErr      bool
		wantCached   map[string]bool
	}{
		"SuccessfulInitialization": {
			reason: "Should successfully initialize and cache functions",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResources(func(_ context.Context, gvk schema.GroupVersionKind, _ string) ([]*un.Unstructured, error) {
					if gvk.Group == "pkg.crossplane.io" && gvk.Kind == "Function" {
						return []*un.Unstructured{u1, u2}, nil
					}
					return nil, errors.New("unexpected GVK")
				}).
				Build(),
			wantErr: false,
			wantCached: map[string]bool{
				"function-1": true,
				"function-2": true,
			},
		},
		"NoFunctions": {
			reason: "Should successfully initialize with empty cache when no functions exist",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			wantErr:    false,
			wantCached: map[string]bool{},
		},
		"ResourceClientInitFailed": {
			reason: "Should return error when resource client initialization fails",
			mockResource: *tu.NewMockResourceClient().
				WithInitialize(func(_ context.Context) error {
					return errors.New("init error")
				}).
				Build(),
			wantErr: true,
		},
		"ListFunctionsFailed": {
			reason: "Should return error when listing functions fails",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResourcesFailure("list error").
				Build(),
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultFunctionClient{
				resourceClient: &tt.mockResource,
				logger:         tu.TestLogger(t, false),
				functions:      make(map[string]pkgv1.Function),
			}

			err := c.Initialize(ctx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("\n%s\nInitialize(): expected error but got none", tt.reason)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nInitialize(): unexpected error: %v", tt.reason, err)
				return
			}

			// If successful, check the cache state
			for name := range tt.wantCached {
				if _, ok := c.functions[name]; !ok {
					t.Errorf("\n%s\nInitialize(): expected function %s to be cached, but it's not", tt.reason, name)
				}
			}

			// Check we don't have extra functions
			if len(c.functions) != len(tt.wantCached) {
				t.Errorf("\n%s\nInitialize(): expected %d cached functions, got %d", tt.reason, len(tt.wantCached), len(c.functions))
			}
		})
	}
}
