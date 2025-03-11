package diff

import (
	"encoding/json"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	dp "github.com/crossplane/crossplane/cmd/crank/beta/diff/diffprocessor"
	"io"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"context"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

// Testing data for integration tests

// createTestXR creates a test XR for validation
func createTestXR() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.org/v1",
			"kind":       "XExampleResource",
			"metadata": map[string]interface{}{
				"name": "test-xr",
			},
			"spec": map[string]interface{}{
				"coolParam": "test-value",
				"replicas":  3,
			},
		},
	}
}

// createTestCompositionWithExtraResources creates a test Composition with a function-extra-resources step
func createTestCompositionWithExtraResources() *apiextensionsv1.Composition {
	pipelineMode := apiextensionsv1.CompositionModePipeline

	// Create the extra resources function input
	extraResourcesInput := map[string]interface{}{
		"apiVersion": "function.crossplane.io/v1beta1",
		"kind":       "ExtraResources",
		"spec": map[string]interface{}{
			"extraResources": []interface{}{
				map[string]interface{}{
					"apiVersion": "example.org/v1",
					"kind":       "ExtraResource",
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "test-app",
						},
					},
				},
			},
		},
	}

	extraResourcesRaw, _ := json.Marshal(extraResourcesInput)

	// Create template function input to create composed resources
	templateInput := map[string]interface{}{
		"apiVersion": "apiextensions.crossplane.io/v1",
		"kind":       "Composition",
		"spec": map[string]interface{}{
			"resources": []interface{}{
				map[string]interface{}{
					"name": "composed-resource",
					"base": map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ComposedResource",
						"metadata": map[string]interface{}{
							"name": "test-composed-resource",
							"labels": map[string]interface{}{
								"app": "crossplane",
							},
						},
						"spec": map[string]interface{}{
							"coolParam": "{{ .observed.composite.spec.coolParam }}",
							"replicas":  "{{ .observed.composite.spec.replicas }}",
							"extraData": "{{ index .observed.resources \"extra-resource-0\" \"spec\" \"data\" }}",
						},
					},
				},
			},
		},
	}

	templateRaw, _ := json.Marshal(templateInput)

	return &apiextensionsv1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-composition",
		},
		Spec: apiextensionsv1.CompositionSpec{
			CompositeTypeRef: apiextensionsv1.TypeReference{
				APIVersion: "example.org/v1",
				Kind:       "XExampleResource",
			},
			Mode: &pipelineMode,
			Pipeline: []apiextensionsv1.PipelineStep{
				{
					Step:        "extra-resources",
					FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
					Input:       &runtime.RawExtension{Raw: extraResourcesRaw},
				},
				{
					Step:        "templating",
					FunctionRef: apiextensionsv1.FunctionReference{Name: "function-patch-and-transform"},
					Input:       &runtime.RawExtension{Raw: templateRaw},
				},
			},
		},
	}
}

// createTestXRD creates a test XRD for the XR
func createTestXRD() *apiextensionsv1.CompositeResourceDefinition {
	return &apiextensionsv1.CompositeResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "xexampleresources.example.org",
		},
		Spec: apiextensionsv1.CompositeResourceDefinitionSpec{
			Group: "example.org",
			Names: extv1.CustomResourceDefinitionNames{
				Kind:     "XExampleResource",
				Plural:   "xexampleresources",
				Singular: "xexampleresource",
			},
			Versions: []apiextensionsv1.CompositeResourceDefinitionVersion{
				{
					Name:          "v1",
					Served:        true,
					Referenceable: true,
					Schema: &apiextensionsv1.CompositeResourceValidation{
						OpenAPIV3Schema: runtime.RawExtension{
							Raw: []byte(`{
								"type": "object",
								"properties": {
									"spec": {
										"type": "object",
										"properties": {
											"coolParam": {
												"type": "string"
											},
											"replicas": {
												"type": "integer"
											}
										}
									},
									"status": {
										"type": "object",
										"properties": {
											"coolStatus": {
												"type": "string"
											}
										}
									}
								}
							}`),
						},
					},
				},
			},
		},
	}
}

// createExtraResource creates a test extra resource
func createExtraResource() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.org/v1",
			"kind":       "ExtraResource",
			"metadata": map[string]interface{}{
				"name": "test-extra-resource",
				"labels": map[string]interface{}{
					"app": "test-app",
				},
			},
			"spec": map[string]interface{}{
				"data": "extra-resource-data",
			},
		},
	}
}

// createExistingComposedResource creates an existing composed resource with different values
func createExistingComposedResource() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.org/v1",
			"kind":       "ComposedResource",
			"metadata": map[string]interface{}{
				"name": "test-xr-composed-resource",
				"labels": map[string]interface{}{
					"app":                     "crossplane",
					"crossplane.io/composite": "test-xr",
				},
				"annotations": map[string]interface{}{
					"crossplane.io/composition-resource-name": "composed-resource",
				},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "example.org/v1",
						"kind":               "XExampleResource",
						"name":               "test-xr",
						"controller":         true,
						"blockOwnerDeletion": true,
					},
				},
			},
			"spec": map[string]interface{}{
				"coolParam": "old-value", // Different from what will be rendered
				"replicas":  2,           // Different from what will be rendered
				"extraData": "old-data",  // Different from what will be rendered
			},
		},
	}
}

// createMatchingComposedResource creates a composed resource that matches what would be rendered
func createMatchingComposedResource() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.org/v1",
			"kind":       "ComposedResource",
			"metadata": map[string]interface{}{
				"name": "test-xr-composed-resource",
				"labels": map[string]interface{}{
					"app":                     "crossplane",
					"crossplane.io/composite": "test-xr",
				},
				"annotations": map[string]interface{}{
					"crossplane.io/composition-resource-name": "composed-resource",
				},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "example.org/v1",
						"kind":               "XExampleResource",
						"name":               "test-xr",
						"controller":         true,
						"blockOwnerDeletion": true,
					},
				},
			},
			"spec": map[string]interface{}{
				"coolParam": "test-value",          // Matches what would be rendered
				"replicas":  3,                     // Matches what would be rendered
				"extraData": "extra-resource-data", // Matches what would be rendered
			},
		},
	}
}

// Define a var for fprintf to allow test overriding
var fprintf = fmt.Fprintf

// Define a var for getting dynamic client to allow test overriding
var getDynamicClient = func(config *rest.Config) (dynamic.Interface, error) {
	return dynamic.NewForConfig(config)
}

// MockDynamicClient mocks the dynamic.Interface
type MockDynamicClient struct {
	ResourceFn func(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface
}

// Resource implements the dynamic.Interface method
func (m *MockDynamicClient) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return m.ResourceFn(gvr)
}

// MockNamespaceableResourceInterface implements dynamic.NamespaceableResourceInterface
type MockNamespaceableResourceInterface struct {
	NamespaceFn func(namespace string) dynamic.ResourceInterface
	GetFn       func(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error)
	ListFn      func(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error)
	CreateFn    func(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error)
	UpdateFn    func(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error)
	DeleteFn    func(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error
	PatchFn     func(ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error)
}

// Namespace implements dynamic.NamespaceableResourceInterface
func (m *MockNamespaceableResourceInterface) Namespace(namespace string) dynamic.ResourceInterface {
	if m.NamespaceFn != nil {
		return m.NamespaceFn(namespace)
	}
	return &MockResourceInterface{
		GetFn:    m.GetFn,
		ListFn:   m.ListFn,
		CreateFn: m.CreateFn,
		UpdateFn: m.UpdateFn,
		DeleteFn: m.DeleteFn,
		PatchFn:  m.PatchFn,
	}
}

// Create implements dynamic.ResourceInterface
func (m *MockNamespaceableResourceInterface) Create(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, obj, options, subresources...)
	}
	return nil, nil
}

// Update implements dynamic.ResourceInterface
func (m *MockNamespaceableResourceInterface) Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, obj, options, subresources...)
	}
	return nil, nil
}

// UpdateStatus implements dynamic.ResourceInterface
func (m *MockNamespaceableResourceInterface) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return nil, nil
}

// Delete implements dynamic.ResourceInterface
func (m *MockNamespaceableResourceInterface) Delete(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, name, options, subresources...)
	}
	return nil
}

// DeleteCollection implements dynamic.ResourceInterface
func (m *MockNamespaceableResourceInterface) DeleteCollection(ctx context.Context, options metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return nil
}

// Get implements dynamic.ResourceInterface
func (m *MockNamespaceableResourceInterface) Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, name, options, subresources...)
	}
	return nil, nil
}

// List implements dynamic.ResourceInterface
func (m *MockNamespaceableResourceInterface) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, opts)
	}
	return nil, nil
}

// Watch implements dynamic.ResourceInterface
func (m *MockNamespaceableResourceInterface) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}

// Patch implements dynamic.ResourceInterface
func (m *MockNamespaceableResourceInterface) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if m.PatchFn != nil {
		return m.PatchFn(ctx, name, pt, data, options, subresources...)
	}
	return nil, nil
}

// Apply implements dynamic.ResourceInterface
func (m *MockNamespaceableResourceInterface) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, nil
}

// ApplyStatus implements dynamic.ResourceInterface
func (m *MockNamespaceableResourceInterface) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return nil, nil
}

// MockResourceInterface mocks dynamic.ResourceInterface for namespaced resources
type MockResourceInterface struct {
	GetFn    func(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error)
	ListFn   func(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error)
	CreateFn func(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error)
	UpdateFn func(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error)
	DeleteFn func(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error
	PatchFn  func(ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error)
}

// Create implements dynamic.ResourceInterface
func (m *MockResourceInterface) Create(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, obj, options, subresources...)
	}
	return nil, nil
}

// Update implements dynamic.ResourceInterface
func (m *MockResourceInterface) Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, obj, options, subresources...)
	}
	return nil, nil
}

// UpdateStatus implements dynamic.ResourceInterface
func (m *MockResourceInterface) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return nil, nil
}

// Delete implements dynamic.ResourceInterface
func (m *MockResourceInterface) Delete(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, name, options, subresources...)
	}
	return nil
}

// DeleteCollection implements dynamic.ResourceInterface
func (m *MockResourceInterface) DeleteCollection(ctx context.Context, options metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return nil
}

// Get implements dynamic.ResourceInterface
func (m *MockResourceInterface) Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, name, options, subresources...)
	}
	return nil, nil
}

// List implements dynamic.ResourceInterface
func (m *MockResourceInterface) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, opts)
	}
	return nil, nil
}

// Watch implements dynamic.ResourceInterface
func (m *MockResourceInterface) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}

// Patch implements dynamic.ResourceInterface
func (m *MockResourceInterface) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if m.PatchFn != nil {
		return m.PatchFn(ctx, name, pt, data, options, subresources...)
	}
	return nil, nil
}

// Apply implements dynamic.ResourceInterface
func (m *MockResourceInterface) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, nil
}

// ApplyStatus implements dynamic.ResourceInterface
func (m *MockResourceInterface) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return nil, nil
}

// MockClusterClient implements the ClusterClient interface for testing
type MockClusterClient struct {
	InitializeFn               func(ctx context.Context) error
	FindMatchingCompositionFn  func(*unstructured.Unstructured) (*apiextensionsv1.Composition, error)
	GetExtraResourcesFn        func(context.Context, []schema.GroupVersionResource, []metav1.LabelSelector) ([]unstructured.Unstructured, error)
	GetFunctionsFromPipelineFn func(*apiextensionsv1.Composition) ([]pkgv1.Function, error)
	GetXRDSchemaFn             func(context.Context, *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error)
	GetResourceFn              func(context.Context, schema.GroupVersionKind, string, string) (*unstructured.Unstructured, error)
	DryRunApplyFn              func(context.Context, *unstructured.Unstructured) (*unstructured.Unstructured, error)
}

// Initialize implements the ClusterClient interface
func (m *MockClusterClient) Initialize(ctx context.Context) error {
	if m.InitializeFn != nil {
		return m.InitializeFn(ctx)
	}
	return nil
}

// FindMatchingComposition implements the ClusterClient interface
func (m *MockClusterClient) FindMatchingComposition(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
	if m.FindMatchingCompositionFn != nil {
		return m.FindMatchingCompositionFn(res)
	}
	return nil, nil
}

// GetExtraResources implements the ClusterClient interface
func (m *MockClusterClient) GetExtraResources(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]unstructured.Unstructured, error) {
	if m.GetExtraResourcesFn != nil {
		return m.GetExtraResourcesFn(ctx, gvrs, selectors)
	}
	return nil, nil
}

// GetFunctionsFromPipeline implements the ClusterClient interface
func (m *MockClusterClient) GetFunctionsFromPipeline(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
	if m.GetFunctionsFromPipelineFn != nil {
		return m.GetFunctionsFromPipelineFn(comp)
	}
	return nil, nil
}

// GetXRDSchema implements the ClusterClient interface
func (m *MockClusterClient) GetXRDSchema(ctx context.Context, res *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error) {
	if m.GetXRDSchemaFn != nil {
		return m.GetXRDSchemaFn(ctx, res)
	}
	return nil, nil
}

// GetResource implements the ClusterClient interface
func (m *MockClusterClient) GetResource(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
	if m.GetResourceFn != nil {
		return m.GetResourceFn(ctx, gvk, namespace, name)
	}
	return nil, errors.New("GetResource not implemented")
}

// DryRunApply implements the ClusterClient interface
func (m *MockClusterClient) DryRunApply(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if m.DryRunApplyFn != nil {
		return m.DryRunApplyFn(ctx, obj)
	}
	return nil, errors.New("DryRunApply not implemented")
}

// Ensure MockClusterClient implements the ClusterClient interface
var _ cc.ClusterClient = &MockClusterClient{}

// MockDiffProcessor implements the DiffProcessor interface for testing
type MockDiffProcessor struct {
	ProcessAllFn      func(ctx context.Context, resources []*unstructured.Unstructured) error
	ProcessResourceFn func(ctx context.Context, res *unstructured.Unstructured) error
}

// ProcessAll implements the DiffProcessor.ProcessAll method
func (m *MockDiffProcessor) ProcessAll(stdout io.Writer, ctx context.Context, resources []*unstructured.Unstructured) error {
	if m.ProcessAllFn != nil {
		return m.ProcessAllFn(ctx, resources)
	}
	// Default implementation processes each resource
	for _, res := range resources {
		if err := m.ProcessResource(nil, ctx, res); err != nil {
			return errors.Wrapf(err, "unable to process resource %s", res.GetName())
		}
	}
	return nil
}

// ProcessResource implements the DiffProcessor.ProcessResource method
func (m *MockDiffProcessor) ProcessResource(stdout io.Writer, ctx context.Context, res *unstructured.Unstructured) error {
	if m.ProcessResourceFn != nil {
		return m.ProcessResourceFn(ctx, res)
	}
	return nil
}

// Ensure MockDiffProcessor implements the DiffProcessor interface
var _ dp.DiffProcessor = &MockDiffProcessor{}
