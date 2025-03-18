package testutils

import (
	"context"
	"encoding/json"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MockBuilder provides a fluent API for building mock objects used in testing.
// This helps reduce duplication in test setup code while making the intent clearer.

// ======================================================================================
// ClusterClient Mock Builder
// ======================================================================================

// ClusterClientBuilder helps build mock ClusterClient instances.
type ClusterClientBuilder struct {
	mock *MockClusterClient
}

// NewMockClusterClient creates a new ClusterClientBuilder.
func NewMockClusterClient() *ClusterClientBuilder {
	return &ClusterClientBuilder{
		mock: &MockClusterClient{},
	}
}

// WithInitialize adds an implementation for the Initialize method.
func (b *ClusterClientBuilder) WithInitialize(fn func(context.Context) error) *ClusterClientBuilder {
	b.mock.InitializeFn = fn
	return b
}

// WithSuccessfulInitialize sets a successful Initialize implementation.
func (b *ClusterClientBuilder) WithSuccessfulInitialize() *ClusterClientBuilder {
	return b.WithInitialize(func(ctx context.Context) error {
		return nil
	})
}

// WithFailedInitialize sets a failing Initialize implementation.
func (b *ClusterClientBuilder) WithFailedInitialize(errMsg string) *ClusterClientBuilder {
	return b.WithInitialize(func(ctx context.Context) error {
		return errors.New(errMsg)
	})
}

// WithFindMatchingComposition adds an implementation for the FindMatchingComposition method.
func (b *ClusterClientBuilder) WithFindMatchingComposition(fn func(*unstructured.Unstructured) (*apiextensionsv1.Composition, error)) *ClusterClientBuilder {
	b.mock.FindMatchingCompositionFn = fn
	return b
}

// WithSuccessfulCompositionMatch sets a successful FindMatchingComposition implementation.
func (b *ClusterClientBuilder) WithSuccessfulCompositionMatch(comp *apiextensionsv1.Composition) *ClusterClientBuilder {
	return b.WithFindMatchingComposition(func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
		return comp, nil
	})
}

// WithNoMatchingComposition sets a FindMatchingComposition implementation that returns "not found".
func (b *ClusterClientBuilder) WithNoMatchingComposition() *ClusterClientBuilder {
	return b.WithFindMatchingComposition(func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
		return nil, errors.New("composition not found")
	})
}

// WithGetFunctionsFromPipeline adds an implementation for the GetFunctionsFromPipeline method.
func (b *ClusterClientBuilder) WithGetFunctionsFromPipeline(fn func(*apiextensionsv1.Composition) ([]pkgv1.Function, error)) *ClusterClientBuilder {
	b.mock.GetFunctionsFromPipelineFn = fn
	return b
}

// WithSuccessfulFunctionsFetch sets a successful GetFunctionsFromPipeline implementation.
func (b *ClusterClientBuilder) WithSuccessfulFunctionsFetch(functions []pkgv1.Function) *ClusterClientBuilder {
	return b.WithGetFunctionsFromPipeline(func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
		return functions, nil
	})
}

// WithFailedFunctionsFetch sets a failing GetFunctionsFromPipeline implementation.
func (b *ClusterClientBuilder) WithFailedFunctionsFetch(errMsg string) *ClusterClientBuilder {
	return b.WithGetFunctionsFromPipeline(func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
		return nil, errors.New(errMsg)
	})
}

// WithGetXRDs adds an implementation for the GetXRDs method.
func (b *ClusterClientBuilder) WithGetXRDs(fn func(context.Context) ([]*unstructured.Unstructured, error)) *ClusterClientBuilder {
	b.mock.GetXRDsFn = fn
	return b
}

// WithSuccessfulXRDsFetch sets a successful GetXRDs implementation.
func (b *ClusterClientBuilder) WithSuccessfulXRDsFetch(xrds []*unstructured.Unstructured) *ClusterClientBuilder {
	return b.WithGetXRDs(func(ctx context.Context) ([]*unstructured.Unstructured, error) {
		return xrds, nil
	})
}

// WithFailedXRDsFetch sets a failing GetXRDs implementation.
func (b *ClusterClientBuilder) WithFailedXRDsFetch(errMsg string) *ClusterClientBuilder {
	return b.WithGetXRDs(func(ctx context.Context) ([]*unstructured.Unstructured, error) {
		return nil, errors.New(errMsg)
	})
}

// WithGetResource adds an implementation for the GetResource method.
func (b *ClusterClientBuilder) WithGetResource(fn func(context.Context, schema.GroupVersionKind, string, string) (*unstructured.Unstructured, error)) *ClusterClientBuilder {
	b.mock.GetResourceFn = fn
	return b
}

// WithResourcesExist sets a GetResource implementation that returns resources from a map.
func (b *ClusterClientBuilder) WithResourcesExist(resources ...*unstructured.Unstructured) *ClusterClientBuilder {
	resourceMap := make(map[string]*unstructured.Unstructured)

	// Build a map for fast lookup
	for _, res := range resources {
		// Use name + kind as a unique key
		key := res.GetName() + "|" + res.GetKind()
		resourceMap[key] = res
	}

	return b.WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
		// Try to find the resource by name and kind
		key := name + "|" + gvk.Kind
		if res, found := resourceMap[key]; found {
			return res, nil
		}
		return nil, errors.Errorf("resource %q not found", name)
	})
}

// WithResourceNotFound sets a GetResource implementation that always returns "not found".
func (b *ClusterClientBuilder) WithResourceNotFound() *ClusterClientBuilder {
	return b.WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
		return nil, errors.Errorf("resource %q not found", name)
	})
}

// WithDryRunApply adds an implementation for the DryRunApply method.
func (b *ClusterClientBuilder) WithDryRunApply(fn func(context.Context, *unstructured.Unstructured) (*unstructured.Unstructured, error)) *ClusterClientBuilder {
	b.mock.DryRunApplyFn = fn
	return b
}

// WithSuccessfulDryRun sets a DryRunApply implementation that returns the input resource.
func (b *ClusterClientBuilder) WithSuccessfulDryRun() *ClusterClientBuilder {
	return b.WithDryRunApply(func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		return obj, nil
	})
}

// WithFailedDryRun sets a DryRunApply implementation that returns an error.
func (b *ClusterClientBuilder) WithFailedDryRun(errMsg string) *ClusterClientBuilder {
	return b.WithDryRunApply(func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		return nil, errors.New(errMsg)
	})
}

// WithGetResourcesByLabel adds an implementation for the GetResourcesByLabel method.
func (b *ClusterClientBuilder) WithGetResourcesByLabel(fn func(context.Context, string, schema.GroupVersionResource, metav1.LabelSelector) ([]*unstructured.Unstructured, error)) *ClusterClientBuilder {
	b.mock.GetResourcesByLabelFn = fn
	return b
}

// WithResourcesFoundByLabel sets a GetResourcesByLabel implementation that returns resources for a specific label.
func (b *ClusterClientBuilder) WithResourcesFoundByLabel(resources []*unstructured.Unstructured, label string, value string) *ClusterClientBuilder {
	return b.WithGetResourcesByLabel(func(ctx context.Context, ns string, gvr schema.GroupVersionResource, selector metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
		// Check if the selector matches our expected label
		if labelValue, exists := selector.MatchLabels[label]; exists && labelValue == value {
			return resources, nil
		}
		return []*unstructured.Unstructured{}, nil
	})
}

// WithGetAllResourcesByLabels adds an implementation for the GetAllResourcesByLabels method.
func (b *ClusterClientBuilder) WithGetAllResourcesByLabels(fn func(context.Context, []schema.GroupVersionResource, []metav1.LabelSelector) ([]*unstructured.Unstructured, error)) *ClusterClientBuilder {
	b.mock.GetAllResourcesByLabelsFn = fn
	return b
}

// WithEnvironmentConfigs adds an implementation for the GetEnvironmentConfigs method.
func (b *ClusterClientBuilder) WithEnvironmentConfigs(fn func(context.Context) ([]*unstructured.Unstructured, error)) *ClusterClientBuilder {
	b.mock.GetEnvironmentConfigsFn = fn
	return b
}

// WithSuccessfulEnvironmentConfigsFetch sets a successful GetEnvironmentConfigs implementation.
func (b *ClusterClientBuilder) WithSuccessfulEnvironmentConfigsFetch(configs []*unstructured.Unstructured) *ClusterClientBuilder {
	return b.WithEnvironmentConfigs(func(ctx context.Context) ([]*unstructured.Unstructured, error) {
		return configs, nil
	})
}

// Build creates and returns the configured mock ClusterClient.
func (b *ClusterClientBuilder) Build() *MockClusterClient {
	return b.mock
}

// WithResourcesByLabel adds an implementation for the GetResourcesByLabel method.
func (b *ClusterClientBuilder) WithResourcesByLabel(fn func(context.Context, string, schema.GroupVersionResource, metav1.LabelSelector) ([]*unstructured.Unstructured, error)) *ClusterClientBuilder {
	b.mock.GetResourcesByLabelFn = fn
	return b
}

// WithComposedResourcesByOwner sets up a GetResourcesByLabel implementation that returns resources by owner
func (b *ClusterClientBuilder) WithComposedResourcesByOwner(resources ...*unstructured.Unstructured) *ClusterClientBuilder {
	return b.WithResourcesByLabel(func(ctx context.Context, ns string, gvr schema.GroupVersionResource, selector metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
		// Check if this is looking for composed resources with crossplane.io/composite label
		if val, exists := selector.MatchLabels["crossplane.io/composite"]; exists {
			// Filter resources with this composite owner
			var owned []*unstructured.Unstructured
			for _, res := range resources {
				// Check if this resource has the composite owner we're looking for
				if labels := res.GetLabels(); labels != nil {
					if owner, ok := labels["crossplane.io/composite"]; ok && owner == val {
						owned = append(owned, res)
					}
				}
			}
			return owned, nil
		}
		return []*unstructured.Unstructured{}, nil
	})
}

// ======================================================================================
// DiffProcessor Mock Builder
// ======================================================================================

// DiffProcessorBuilder helps build mock DiffProcessor instances.
type DiffProcessorBuilder struct {
	mock *MockDiffProcessor
}

// NewMockDiffProcessor creates a new DiffProcessorBuilder.
func NewMockDiffProcessor() *DiffProcessorBuilder {
	return &DiffProcessorBuilder{
		mock: &MockDiffProcessor{},
	}
}

// WithInitialize adds an implementation for the Initialize method.
func (b *DiffProcessorBuilder) WithInitialize(fn func(io.Writer, context.Context) error) *DiffProcessorBuilder {
	b.mock.InitializeFn = fn
	return b
}

// WithSuccessfulInitialize sets a successful Initialize implementation.
func (b *DiffProcessorBuilder) WithSuccessfulInitialize() *DiffProcessorBuilder {
	return b.WithInitialize(func(writer io.Writer, ctx context.Context) error {
		return nil
	})
}

// WithFailedInitialize sets a failing Initialize implementation.
func (b *DiffProcessorBuilder) WithFailedInitialize(errMsg string) *DiffProcessorBuilder {
	return b.WithInitialize(func(writer io.Writer, ctx context.Context) error {
		return errors.New(errMsg)
	})
}

// WithProcessResource adds an implementation for the ProcessResource method.
func (b *DiffProcessorBuilder) WithProcessResource(fn func(io.Writer, context.Context, *unstructured.Unstructured) error) *DiffProcessorBuilder {
	b.mock.ProcessResourceFn = fn
	return b
}

// WithSuccessfulResourceProcessing sets a successful ProcessResource implementation.
func (b *DiffProcessorBuilder) WithSuccessfulResourceProcessing() *DiffProcessorBuilder {
	return b.WithProcessResource(func(stdout io.Writer, ctx context.Context, res *unstructured.Unstructured) error {
		return nil
	})
}

// WithResourceProcessingOutput sets a ProcessResource implementation that writes a specific output.
func (b *DiffProcessorBuilder) WithResourceProcessingOutput(output string) *DiffProcessorBuilder {
	return b.WithProcessResource(func(stdout io.Writer, ctx context.Context, res *unstructured.Unstructured) error {
		if stdout != nil {
			_, _ = io.WriteString(stdout, output)
		}
		return nil
	})
}

// WithFailedResourceProcessing sets a failing ProcessResource implementation.
func (b *DiffProcessorBuilder) WithFailedResourceProcessing(errMsg string) *DiffProcessorBuilder {
	return b.WithProcessResource(func(stdout io.Writer, ctx context.Context, res *unstructured.Unstructured) error {
		return errors.New(errMsg)
	})
}

// WithProcessAll adds an implementation for the ProcessAll method.
func (b *DiffProcessorBuilder) WithProcessAll(fn func(io.Writer, context.Context, []*unstructured.Unstructured) error) *DiffProcessorBuilder {
	b.mock.ProcessAllFn = fn
	return b
}

// WithSuccessfulAllProcessing sets a successful ProcessAll implementation.
func (b *DiffProcessorBuilder) WithSuccessfulAllProcessing() *DiffProcessorBuilder {
	return b.WithProcessAll(func(stdout io.Writer, ctx context.Context, resources []*unstructured.Unstructured) error {
		return nil
	})
}

// WithFailedAllProcessing sets a failing ProcessAll implementation.
func (b *DiffProcessorBuilder) WithFailedAllProcessing(errMsg string) *DiffProcessorBuilder {
	return b.WithProcessAll(func(stdout io.Writer, ctx context.Context, resources []*unstructured.Unstructured) error {
		return errors.New(errMsg)
	})
}

// Build creates and returns the configured mock DiffProcessor.
func (b *DiffProcessorBuilder) Build() *MockDiffProcessor {
	return b.mock
}

// ======================================================================================
// ExtraResourceProvider Mock Builder
// ======================================================================================

// ExtraResourceProviderBuilder helps build mock ExtraResourceProvider instances.
type ExtraResourceProviderBuilder struct {
	mock *MockExtraResourceProvider
}

// NewMockExtraResourceProvider creates a new ExtraResourceProviderBuilder.
func NewMockExtraResourceProvider() *ExtraResourceProviderBuilder {
	return &ExtraResourceProviderBuilder{
		mock: &MockExtraResourceProvider{},
	}
}

// WithGetExtraResources adds an implementation for the GetExtraResources method.
func (b *ExtraResourceProviderBuilder) WithGetExtraResources(fn func(context.Context, *apiextensionsv1.Composition, *unstructured.Unstructured, []*unstructured.Unstructured) ([]*unstructured.Unstructured, error)) *ExtraResourceProviderBuilder {
	b.mock.GetExtraResourcesFn = fn
	return b
}

// WithSuccessfulExtraResourcesFetch sets a successful GetExtraResources implementation.
func (b *ExtraResourceProviderBuilder) WithSuccessfulExtraResourcesFetch(resources []*unstructured.Unstructured) *ExtraResourceProviderBuilder {
	return b.WithGetExtraResources(func(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, existing []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
		return resources, nil
	})
}

// WithFailedExtraResourcesFetch sets a failing GetExtraResources implementation.
func (b *ExtraResourceProviderBuilder) WithFailedExtraResourcesFetch(errMsg string) *ExtraResourceProviderBuilder {
	return b.WithGetExtraResources(func(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, existing []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
		return nil, errors.New(errMsg)
	})
}

// Build creates and returns the configured mock ExtraResourceProvider.
func (b *ExtraResourceProviderBuilder) Build() *MockExtraResourceProvider {
	return b.mock
}

// ======================================================================================
// Resource Building Helpers
// ======================================================================================

// ResourceBuilder helps construct unstructured resources for testing.
type ResourceBuilder struct {
	resource *unstructured.Unstructured
}

// NewResource creates a new ResourceBuilder.
func NewResource(apiVersion, kind, name string) *ResourceBuilder {
	return &ResourceBuilder{
		resource: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": apiVersion,
				"kind":       kind,
				"metadata": map[string]interface{}{
					"name": name,
				},
			},
		},
	}
}

// InNamespace sets the namespace for the resource.
func (b *ResourceBuilder) InNamespace(namespace string) *ResourceBuilder {
	if namespace != "" {
		b.resource.SetNamespace(namespace)
	}
	return b
}

// WithLabels adds labels to the resource.
func (b *ResourceBuilder) WithLabels(labels map[string]string) *ResourceBuilder {
	if len(labels) > 0 {
		b.resource.SetLabels(labels)
	}
	return b
}

// WithAnnotations adds annotations to the resource.
func (b *ResourceBuilder) WithAnnotations(annotations map[string]string) *ResourceBuilder {
	if len(annotations) > 0 {
		b.resource.SetAnnotations(annotations)
	}
	return b
}

// WithSpec sets the spec field of the resource.
func (b *ResourceBuilder) WithSpec(spec map[string]interface{}) *ResourceBuilder {
	if len(spec) > 0 {
		_ = unstructured.SetNestedMap(b.resource.Object, spec, "spec")
	}
	return b
}

// WithSpecField sets a specific field in the spec.
func (b *ResourceBuilder) WithSpecField(name string, value interface{}) *ResourceBuilder {
	spec, _, _ := unstructured.NestedMap(b.resource.Object, "spec")
	if spec == nil {
		spec = map[string]interface{}{}
	}
	spec[name] = value
	_ = unstructured.SetNestedMap(b.resource.Object, spec, "spec")
	return b
}

// WithStatus sets the status field of the resource.
func (b *ResourceBuilder) WithStatus(status map[string]interface{}) *ResourceBuilder {
	if len(status) > 0 {
		_ = unstructured.SetNestedMap(b.resource.Object, status, "status")
	}
	return b
}

// WithStatusField sets a specific field in the status.
func (b *ResourceBuilder) WithStatusField(name string, value interface{}) *ResourceBuilder {
	status, _, _ := unstructured.NestedMap(b.resource.Object, "status")
	if status == nil {
		status = map[string]interface{}{}
	}
	status[name] = value
	_ = unstructured.SetNestedMap(b.resource.Object, status, "status")
	return b
}

// WithCompositeOwner sets up the resource as a composed resource with the given composite owner.
func (b *ResourceBuilder) WithCompositeOwner(owner string) *ResourceBuilder {
	// Add standard Crossplane labels and annotations for a composed resource
	labels := b.resource.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels["crossplane.io/composite"] = owner
	b.resource.SetLabels(labels)

	return b
}

// WithCompositionResourceName sets the composition resource name annotation.
func (b *ResourceBuilder) WithCompositionResourceName(name string) *ResourceBuilder {
	annotations := b.resource.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations["crossplane.io/composition-resource-name"] = name
	b.resource.SetAnnotations(annotations)

	return b
}

// Build returns the built unstructured resource.
func (b *ResourceBuilder) Build() *unstructured.Unstructured {
	return b.resource.DeepCopy()
}

// ======================================================================================
// Composition Building Helpers
// ======================================================================================

// CompositionBuilder helps construct Composition objects for testing.
type CompositionBuilder struct {
	composition *apiextensionsv1.Composition
}

// NewComposition creates a new CompositionBuilder.
func NewComposition(name string) *CompositionBuilder {
	return &CompositionBuilder{
		composition: &apiextensionsv1.Composition{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apiextensions.crossplane.io/v1",
				Kind:       "Composition",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: apiextensionsv1.CompositionSpec{},
		},
	}
}

// WithCompositeTypeRef sets the composite type reference.
func (b *CompositionBuilder) WithCompositeTypeRef(apiVersion, kind string) *CompositionBuilder {
	b.composition.Spec.CompositeTypeRef = apiextensionsv1.TypeReference{
		APIVersion: apiVersion,
		Kind:       kind,
	}
	return b
}

// WithPipelineMode sets the composition mode to pipeline.
func (b *CompositionBuilder) WithPipelineMode() *CompositionBuilder {
	mode := apiextensionsv1.CompositionModePipeline
	b.composition.Spec.Mode = &mode
	return b
}

// WithPipelineStep adds a pipeline step to the composition.
func (b *CompositionBuilder) WithPipelineStep(step, functionName string, input map[string]interface{}) *CompositionBuilder {
	var rawInput *runtime.RawExtension
	if input != nil {
		// Properly serialize the map to JSON bytes
		jsonBytes, err := json.Marshal(input)
		if err == nil {
			rawInput = &runtime.RawExtension{
				Raw: jsonBytes,
			}
		}
	}

	b.composition.Spec.Pipeline = append(b.composition.Spec.Pipeline, apiextensionsv1.PipelineStep{
		Step:        step,
		FunctionRef: apiextensionsv1.FunctionReference{Name: functionName},
		Input:       rawInput,
	})
	return b
}

// Build returns the built Composition.
func (b *CompositionBuilder) Build() *apiextensionsv1.Composition {
	return b.composition.DeepCopy()
}

//// CRDBuilder helps construct CustomResourceDefinition resources for testing.
//type CRDBuilder struct {
//	crd *extv1.CustomResourceDefinition
//}
//
//// NewCRD creates a new CRDBuilder.
//func NewCRD(group, kind string) *CRDBuilder {
//	plural := strings.ToLower(kind) + "s" // Simple pluralization
//	name := plural + "." + group
//
//	return &CRDBuilder{
//		crd: &extv1.CustomResourceDefinition{
//			TypeMeta: metav1.TypeMeta{
//				APIVersion: "apiextensions.k8s.io/v1",
//				Kind:       "CustomResourceDefinition",
//			},
//			ObjectMeta: metav1.ObjectMeta{
//				Name: name,
//			},
//			Spec: extv1.CustomResourceDefinitionSpec{
//				Group: group,
//				Names: extv1.CustomResourceDefinitionNames{
//					Kind:     kind,
//					Plural:   plural,
//					Singular: strings.ToLower(kind),
//				},
//				Scope: extv1.ClusterScoped,
//				Versions: []extv1.CustomResourceDefinitionVersion{
//					{
//						Name:    "v1",
//						Served:  true,
//						Storage: true,
//						Schema: &extv1.CustomResourceValidation{
//							OpenAPIV3Schema: &extv1.JSONSchemaProps{
//								Type: "object",
//								Properties: map[string]extv1.JSONSchemaProps{
//									"spec": {
//										Type:       "object",
//										Properties: map[string]extv1.JSONSchemaProps{},
//									},
//								},
//							},
//						},
//					},
//				},
//			},
//		},
//	}
//}
//
//// WithVersion sets the version for the CRD.
//func (b *CRDBuilder) WithVersion(version string) *CRDBuilder {
//	if len(b.crd.Spec.Versions) > 0 {
//		b.crd.Spec.Versions[0].Name = version
//	}
//	return b
//}
//
//// WithSchemaProperty adds a property to the schema.
//func (b *CRDBuilder) WithSchemaProperty(name, propType string) *CRDBuilder {
//	if len(b.crd.Spec.Versions) > 0 && b.crd.Spec.Versions[0].Schema != nil &&
//		b.crd.Spec.Versions[0].Schema.OpenAPIV3Schema != nil {
//		specProps := b.crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
//		specProps.Properties[name] = extv1.JSONSchemaProps{
//			Type: propType,
//		}
//		b.crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"] = specProps
//	}
//	return b
//}
//
//// WithNestedSchemaProperty adds a nested property structure to the schema.
//func (b *CRDBuilder) WithNestedSchemaProperty(parent, name, propType string) *CRDBuilder {
//	if len(b.crd.Spec.Versions) > 0 && b.crd.Spec.Versions[0].Schema != nil &&
//		b.crd.Spec.Versions[0].Schema.OpenAPIV3Schema != nil {
//		specProps := b.crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
//
//		// Create or get the parent property
//		parentProp, exists := specProps.Properties[parent]
//		if !exists {
//			parentProp = extv1.JSONSchemaProps{
//				Type:       "object",
//				Properties: map[string]extv1.JSONSchemaProps{},
//			}
//		}
//
//		// Add the child property
//		parentProp.Properties[name] = extv1.JSONSchemaProps{
//			Type: propType,
//		}
//
//		// Update the parent property in the spec
//		specProps.Properties[parent] = parentProp
//		b.crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"] = specProps
//	}
//	return b
//}
//
//// WithScope sets the scope for the CRD.
//func (b *CRDBuilder) WithScope(scope extv1.ResourceScope) *CRDBuilder {
//	b.crd.Spec.Scope = scope
//	return b
//}
//
//// WithCategories adds categories to the CRD.
//func (b *CRDBuilder) WithCategories(categories ...string) *CRDBuilder {
//	b.crd.Spec.Names.Categories = categories
//	return b
//}
//
//// Build returns the built CustomResourceDefinition.
//func (b *CRDBuilder) Build() *extv1.CustomResourceDefinition {
//	return b.crd.DeepCopy()
//}
//
//// AsUnstructured converts the CRD to an unstructured.Unstructured.
//func (b *CRDBuilder) AsUnstructured() (*unstructured.Unstructured, error) {
//	crdObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(b.crd)
//	if err != nil {
//		return nil, err
//	}
//	return &unstructured.Unstructured{Object: crdObject}, nil
//}

// Helper function to create a pointer to a CompositionMode
func ComposePtr(mode apiextensionsv1.CompositionMode) *apiextensionsv1.CompositionMode {
	return &mode
}
