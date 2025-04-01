package testutils

import (
	"context"
	"encoding/json"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	cpd "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	cmp "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal/resource"
	"io"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"strings"
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
func (b *ClusterClientBuilder) WithFindMatchingComposition(fn func(context.Context, *un.Unstructured) (*xpextv1.Composition, error)) *ClusterClientBuilder {
	b.mock.FindMatchingCompositionFn = fn
	return b
}

// WithSuccessfulCompositionMatch sets a successful FindMatchingComposition implementation.
func (b *ClusterClientBuilder) WithSuccessfulCompositionMatch(comp *xpextv1.Composition) *ClusterClientBuilder {
	return b.WithFindMatchingComposition(func(ctx context.Context, res *un.Unstructured) (*xpextv1.Composition, error) {
		return comp, nil
	})
}

// WithNoMatchingComposition sets a FindMatchingComposition implementation that returns "not found".
func (b *ClusterClientBuilder) WithNoMatchingComposition() *ClusterClientBuilder {
	return b.WithFindMatchingComposition(func(ctx context.Context, res *un.Unstructured) (*xpextv1.Composition, error) {
		return nil, errors.New("composition not found")
	})
}

// WithGetFunctionsFromPipeline adds an implementation for the GetFunctionsFromPipeline method.
func (b *ClusterClientBuilder) WithGetFunctionsFromPipeline(fn func(*xpextv1.Composition) ([]pkgv1.Function, error)) *ClusterClientBuilder {
	b.mock.GetFunctionsFromPipelineFn = fn
	return b
}

// WithSuccessfulFunctionsFetch sets a successful GetFunctionsFromPipeline implementation.
func (b *ClusterClientBuilder) WithSuccessfulFunctionsFetch(functions []pkgv1.Function) *ClusterClientBuilder {
	return b.WithGetFunctionsFromPipeline(func(comp *xpextv1.Composition) ([]pkgv1.Function, error) {
		return functions, nil
	})
}

// WithFailedFunctionsFetch sets a failing GetFunctionsFromPipeline implementation.
func (b *ClusterClientBuilder) WithFailedFunctionsFetch(errMsg string) *ClusterClientBuilder {
	return b.WithGetFunctionsFromPipeline(func(comp *xpextv1.Composition) ([]pkgv1.Function, error) {
		return nil, errors.New(errMsg)
	})
}

// WithGetXRDs adds an implementation for the GetXRDs method.
func (b *ClusterClientBuilder) WithGetXRDs(fn func(context.Context) ([]*un.Unstructured, error)) *ClusterClientBuilder {
	b.mock.GetXRDsFn = fn
	return b
}

// WithSuccessfulXRDsFetch sets a successful GetXRDs implementation.
func (b *ClusterClientBuilder) WithSuccessfulXRDsFetch(xrds []*un.Unstructured) *ClusterClientBuilder {
	return b.WithGetXRDs(func(ctx context.Context) ([]*un.Unstructured, error) {
		return xrds, nil
	})
}

// WithFailedXRDsFetch sets a failing GetXRDs implementation.
func (b *ClusterClientBuilder) WithFailedXRDsFetch(errMsg string) *ClusterClientBuilder {
	return b.WithGetXRDs(func(ctx context.Context) ([]*un.Unstructured, error) {
		return nil, errors.New(errMsg)
	})
}

// WithGetResource adds an implementation for the GetResource method.
func (b *ClusterClientBuilder) WithGetResource(fn func(context.Context, schema.GroupVersionKind, string, string) (*un.Unstructured, error)) *ClusterClientBuilder {
	b.mock.GetResourceFn = fn
	return b
}

// WithResourcesExist sets a GetResource implementation that returns resources from a map.
func (b *ClusterClientBuilder) WithResourcesExist(resources ...*un.Unstructured) *ClusterClientBuilder {
	resourceMap := make(map[string]*un.Unstructured)

	// Build a map for fast lookup
	for _, res := range resources {
		// Use name + kind as a unique key
		key := res.GetName() + "|" + res.GetKind()
		resourceMap[key] = res
	}

	return b.WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*un.Unstructured, error) {
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
	return b.WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*un.Unstructured, error) {
		// Create a proper Kubernetes "not found" error
		return nil, apierrors.NewNotFound(
			schema.GroupResource{
				Group:    gvk.Group,
				Resource: strings.ToLower(gvk.Kind) + "s", // Naive pluralization similar to the real code
			},
			name,
		)
	})
}

// WithDryRunApply adds an implementation for the DryRunApply method.
func (b *ClusterClientBuilder) WithDryRunApply(fn func(context.Context, *un.Unstructured) (*un.Unstructured, error)) *ClusterClientBuilder {
	b.mock.DryRunApplyFn = fn
	return b
}

// WithSuccessfulDryRun sets a DryRunApply implementation that returns the input resource.
func (b *ClusterClientBuilder) WithSuccessfulDryRun() *ClusterClientBuilder {
	return b.WithDryRunApply(func(ctx context.Context, obj *un.Unstructured) (*un.Unstructured, error) {
		return obj, nil
	})
}

// WithFailedDryRun sets a DryRunApply implementation that returns an error.
func (b *ClusterClientBuilder) WithFailedDryRun(errMsg string) *ClusterClientBuilder {
	return b.WithDryRunApply(func(ctx context.Context, obj *un.Unstructured) (*un.Unstructured, error) {
		return nil, errors.New(errMsg)
	})
}

// WithGetResourcesByLabel adds an implementation for the GetResourcesByLabel method.
func (b *ClusterClientBuilder) WithGetResourcesByLabel(fn func(context.Context, string, schema.GroupVersionKind, metav1.LabelSelector) ([]*un.Unstructured, error)) *ClusterClientBuilder {
	b.mock.GetResourcesByLabelFn = fn
	return b
}

// WithResourcesFoundByLabel sets a GetResourcesByLabel implementation that returns resources for a specific label.
func (b *ClusterClientBuilder) WithResourcesFoundByLabel(resources []*un.Unstructured, label string, value string) *ClusterClientBuilder {
	return b.WithGetResourcesByLabel(func(ctx context.Context, ns string, gvk schema.GroupVersionKind, selector metav1.LabelSelector) ([]*un.Unstructured, error) {
		// Check if the selector matches our expected label
		if labelValue, exists := selector.MatchLabels[label]; exists && labelValue == value {
			return resources, nil
		}
		return []*un.Unstructured{}, nil
	})
}

// WithGetAllResourcesByLabels adds an implementation for the GetAllResourcesByLabels method.
func (b *ClusterClientBuilder) WithGetAllResourcesByLabels(fn func(context.Context, []schema.GroupVersionKind, []metav1.LabelSelector) ([]*un.Unstructured, error)) *ClusterClientBuilder {
	b.mock.GetAllResourcesByLabelsFn = fn
	return b
}

// WithEnvironmentConfigs adds an implementation for the GetEnvironmentConfigs method.
func (b *ClusterClientBuilder) WithEnvironmentConfigs(fn func(context.Context) ([]*un.Unstructured, error)) *ClusterClientBuilder {
	b.mock.GetEnvironmentConfigsFn = fn
	return b
}

// WithSuccessfulEnvironmentConfigsFetch sets a successful GetEnvironmentConfigs implementation.
func (b *ClusterClientBuilder) WithSuccessfulEnvironmentConfigsFetch(configs []*un.Unstructured) *ClusterClientBuilder {
	return b.WithEnvironmentConfigs(func(ctx context.Context) ([]*un.Unstructured, error) {
		return configs, nil
	})
}

// WithGetResourceTree adds an implementation for the GetResourceTree method
func (b *ClusterClientBuilder) WithGetResourceTree(fn func(context.Context, *un.Unstructured) (*resource.Resource, error)) *ClusterClientBuilder {
	b.mock.GetResourceTreeFn = fn
	return b
}

// WithSuccessfulResourceTreeFetch sets a successful GetResourceTree implementation
func (b *ClusterClientBuilder) WithSuccessfulResourceTreeFetch(resourceTree *resource.Resource) *ClusterClientBuilder {
	return b.WithGetResourceTree(func(ctx context.Context, root *un.Unstructured) (*resource.Resource, error) {
		return resourceTree, nil
	})
}

// WithEmptyResourceTree sets a GetResourceTree implementation that returns just the root with no children
func (b *ClusterClientBuilder) WithEmptyResourceTree() *ClusterClientBuilder {
	return b.WithGetResourceTree(func(ctx context.Context, root *un.Unstructured) (*resource.Resource, error) {
		return &resource.Resource{
			Unstructured: *root.DeepCopy(),
			Children:     []*resource.Resource{},
		}, nil
	})
}

// WithFailedResourceTreeFetch sets a failing GetResourceTree implementation
func (b *ClusterClientBuilder) WithFailedResourceTreeFetch(errMsg string) *ClusterClientBuilder {
	return b.WithGetResourceTree(func(ctx context.Context, root *un.Unstructured) (*resource.Resource, error) {
		return nil, errors.New(errMsg)
	})
}

// WithResourceTreeFromXRAndComposed creates a basic resource tree from an XR and cpd resources
func (b *ClusterClientBuilder) WithResourceTreeFromXRAndComposed(xr *un.Unstructured, composed []*un.Unstructured) *ClusterClientBuilder {
	return b.WithGetResourceTree(func(ctx context.Context, root *un.Unstructured) (*resource.Resource, error) {
		// Make sure we're looking for the right XR
		if root.GetName() != xr.GetName() || root.GetKind() != xr.GetKind() {
			return nil, errors.Errorf("unexpected resource %s/%s", root.GetKind(), root.GetName())
		}

		// Create the resource tree with the XR as root
		resourceTree := &resource.Resource{
			Unstructured: *xr.DeepCopy(),
			Children:     make([]*resource.Resource, 0, len(composed)),
		}

		// Add cpd resources as children
		for _, comp := range composed {
			resourceTree.Children = append(resourceTree.Children, &resource.Resource{
				Unstructured: *comp.DeepCopy(),
				Children:     []*resource.Resource{},
			})
		}

		return resourceTree, nil
	})
}

// WithResourcesByLabel adds an implementation for the GetResourcesByLabel method.
func (b *ClusterClientBuilder) WithResourcesByLabel(fn func(context.Context, string, schema.GroupVersionKind, metav1.LabelSelector) ([]*un.Unstructured, error)) *ClusterClientBuilder {
	b.mock.GetResourcesByLabelFn = fn
	return b
}

// WithComposedResourcesByOwner sets up a GetResourcesByLabel implementation that returns resources by owner
func (b *ClusterClientBuilder) WithComposedResourcesByOwner(resources ...*un.Unstructured) *ClusterClientBuilder {
	return b.WithResourcesByLabel(func(ctx context.Context, ns string, gvk schema.GroupVersionKind, selector metav1.LabelSelector) ([]*un.Unstructured, error) {
		// Check if this is looking for cpd resources with crossplane.io/composite label
		if val, exists := selector.MatchLabels["crossplane.io/composite"]; exists {
			// Filter resources with this composite owner
			var owned []*un.Unstructured
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
		return []*un.Unstructured{}, nil
	})
}

// WithIsCRDRequired adds an implementation for the IsCRDRequired method.
func (b *ClusterClientBuilder) WithIsCRDRequired(fn func(context.Context, schema.GroupVersionKind) bool) *ClusterClientBuilder {
	b.mock.IsCRDRequiredFn = fn
	return b
}

// WithResourcesRequiringCRDs sets only the specified GVKs to require CRDs.
// All other resources will be considered as not requiring CRDs.
func (b *ClusterClientBuilder) WithResourcesRequiringCRDs(crdsRequiredGVKs ...schema.GroupVersionKind) *ClusterClientBuilder {
	requiresCRD := make(map[schema.GroupVersionKind]bool)
	for _, gvk := range crdsRequiredGVKs {
		requiresCRD[gvk] = true
	}

	return b.WithIsCRDRequired(func(ctx context.Context, gvk schema.GroupVersionKind) bool {
		// Only require CRDs for specified GVKs
		return requiresCRD[gvk]
	})
}

// WithAllResourcesRequiringCRDs sets all resources to require CRDs.
func (b *ClusterClientBuilder) WithAllResourcesRequiringCRDs() *ClusterClientBuilder {
	return b.WithIsCRDRequired(func(ctx context.Context, gvk schema.GroupVersionKind) bool {
		return true
	})
}

// WithNoResourcesRequiringCRDs sets all resources to not require CRDs.
func (b *ClusterClientBuilder) WithNoResourcesRequiringCRDs() *ClusterClientBuilder {
	return b.WithIsCRDRequired(func(ctx context.Context, gvk schema.GroupVersionKind) bool {
		return false
	})
}

// WithGetCRD adds an implementation for the GetCRD method.
func (b *ClusterClientBuilder) WithGetCRD(fn func(ctx context.Context, gvk schema.GroupVersionKind) (*un.Unstructured, error)) *ClusterClientBuilder {
	b.mock.GetCRDFn = fn
	return b
}

// WithSuccessfulCRDFetch sets a GetCRD implementation that returns a specific CRD.
func (b *ClusterClientBuilder) WithSuccessfulCRDFetch(crd *un.Unstructured) *ClusterClientBuilder {
	return b.WithGetCRD(func(ctx context.Context, gvk schema.GroupVersionKind) (*un.Unstructured, error) {
		if crd.GetKind() != "CustomResourceDefinition" {
			return nil, errors.Errorf("setup error:  desired return from GetCRD isn't a CRD but a %s", crd.GetKind())
		}
		return crd, nil
	})
}

// Build creates and returns the configured mock ClusterClient.
func (b *ClusterClientBuilder) Build() *MockClusterClient {
	return b.mock
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
func (b *DiffProcessorBuilder) WithInitialize(fn func(context.Context) error) *DiffProcessorBuilder {
	b.mock.InitializeFn = fn
	return b
}

// WithSuccessfulInitialize sets a successful Initialize implementation.
func (b *DiffProcessorBuilder) WithSuccessfulInitialize() *DiffProcessorBuilder {
	return b.WithInitialize(func(ctx context.Context) error {
		return nil
	})
}

// WithFailedInitialize sets a failing Initialize implementation.
func (b *DiffProcessorBuilder) WithFailedInitialize(errMsg string) *DiffProcessorBuilder {
	return b.WithInitialize(func(ctx context.Context) error {
		return errors.New(errMsg)
	})
}

// WithPerformDiff adds an implementation for the PerformDiff method.
func (b *DiffProcessorBuilder) WithPerformDiff(fn func(io.Writer, context.Context, []*un.Unstructured) error) *DiffProcessorBuilder {
	b.mock.PerformDiffFn = fn
	return b
}

// WithSuccessfulPerformDiff sets a successful PerformDiff implementation.
func (b *DiffProcessorBuilder) WithSuccessfulPerformDiff() *DiffProcessorBuilder {
	return b.WithPerformDiff(func(stdout io.Writer, ctx context.Context, resources []*un.Unstructured) error {
		return nil
	})
}

// WithDiffOutput sets a PerformDiff implementation that writes a specific output.
func (b *DiffProcessorBuilder) WithDiffOutput(output string) *DiffProcessorBuilder {
	return b.WithPerformDiff(func(stdout io.Writer, ctx context.Context, resources []*un.Unstructured) error {
		if stdout != nil {
			_, _ = io.WriteString(stdout, output)
		}
		return nil
	})
}

// WithFailedPerformDiff sets a failing PerformDiff implementation.
func (b *DiffProcessorBuilder) WithFailedPerformDiff(errMsg string) *DiffProcessorBuilder {
	return b.WithPerformDiff(func(stdout io.Writer, ctx context.Context, resources []*un.Unstructured) error {
		return errors.New(errMsg)
	})
}

// Build creates and returns the configured mock DiffProcessor.
func (b *DiffProcessorBuilder) Build() *MockDiffProcessor {
	return b.mock
}

// ======================================================================================
// Resource Building Helpers
// ======================================================================================

// ResourceBuilder helps construct un resources for testing.
type ResourceBuilder struct {
	resource *un.Unstructured
}

// NewResource creates a new ResourceBuilder.
func NewResource(apiVersion, kind, name string) *ResourceBuilder {
	return &ResourceBuilder{
		resource: &un.Unstructured{
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

// WithGenerateName sets the namespace for the resource.
func (b *ResourceBuilder) WithGenerateName(generateName string) *ResourceBuilder {
	if generateName != "" {
		b.resource.SetGenerateName(generateName)
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
		_ = un.SetNestedMap(b.resource.Object, spec, "spec")
	}
	return b
}

// WithSpecField sets a specific field in the spec.
func (b *ResourceBuilder) WithSpecField(name string, value interface{}) *ResourceBuilder {
	spec, _, _ := un.NestedMap(b.resource.Object, "spec")
	if spec == nil {
		spec = map[string]interface{}{}
	}
	spec[name] = value
	_ = un.SetNestedMap(b.resource.Object, spec, "spec")
	return b
}

// WithStatus sets the status field of the resource.
func (b *ResourceBuilder) WithStatus(status map[string]interface{}) *ResourceBuilder {
	if len(status) > 0 {
		_ = un.SetNestedMap(b.resource.Object, status, "status")
	}
	return b
}

// WithStatusField sets a specific field in the status.
func (b *ResourceBuilder) WithStatusField(name string, value interface{}) *ResourceBuilder {
	status, _, _ := un.NestedMap(b.resource.Object, "status")
	if status == nil {
		status = map[string]interface{}{}
	}
	status[name] = value
	_ = un.SetNestedMap(b.resource.Object, status, "status")
	return b
}

// WithOwnerReference appends an owner ref to a resource.
func (b *ResourceBuilder) WithOwnerReference(kind, name, apiVersion, uid string) *ResourceBuilder {
	// Get existing owner references, or create an empty slice if none exist
	ownerRefs := b.resource.GetOwnerReferences()

	// Create the new owner reference
	newOwnerRef := metav1.OwnerReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		UID:        types.UID(uid),
	}

	// Append the new owner reference
	ownerRefs = append(ownerRefs, newOwnerRef)

	// Set the updated owner references on the resource
	b.resource.SetOwnerReferences(ownerRefs)

	return b
}

// WithCompositeOwner sets up the resource as a cpd resource with the given composite owner.
func (b *ResourceBuilder) WithCompositeOwner(owner string) *ResourceBuilder {
	// Add standard Crossplane labels and annotations for a cpd resource
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

// Build returns the built un resource.
func (b *ResourceBuilder) Build() *un.Unstructured {
	return b.resource.DeepCopy()
}

// BuildUComposite returns the built un resource as a *cmp.Unstructured.
func (b *ResourceBuilder) BuildUComposite() *cmp.Unstructured {
	built := &cmp.Unstructured{}
	built.SetUnstructuredContent(b.Build().UnstructuredContent())
	return built
}

func (b *ResourceBuilder) BuildUComposed() *cpd.Unstructured {
	built := &cpd.Unstructured{}
	built.SetUnstructuredContent(b.Build().UnstructuredContent())
	return built
}

// ======================================================================================
// Composition Building Helpers
// ======================================================================================

// CompositionBuilder helps construct Composition objects for testing.
type CompositionBuilder struct {
	composition *xpextv1.Composition
}

// NewComposition creates a new CompositionBuilder.
func NewComposition(name string) *CompositionBuilder {
	return &CompositionBuilder{
		composition: &xpextv1.Composition{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apiextensions.crossplane.io/v1",
				Kind:       "Composition",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: xpextv1.CompositionSpec{},
		},
	}
}

// WithCompositeTypeRef sets the composite type reference.
func (b *CompositionBuilder) WithCompositeTypeRef(apiVersion, kind string) *CompositionBuilder {
	b.composition.Spec.CompositeTypeRef = xpextv1.TypeReference{
		APIVersion: apiVersion,
		Kind:       kind,
	}
	return b
}

// WithPipelineMode sets the composition mode to pipeline.
func (b *CompositionBuilder) WithPipelineMode() *CompositionBuilder {
	mode := xpextv1.CompositionModePipeline
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

	b.composition.Spec.Pipeline = append(b.composition.Spec.Pipeline, xpextv1.PipelineStep{
		Step:        step,
		FunctionRef: xpextv1.FunctionReference{Name: functionName},
		Input:       rawInput,
	})
	return b
}

// Build returns the built Composition.
func (b *CompositionBuilder) Build() *xpextv1.Composition {
	return b.composition.DeepCopy()
}
