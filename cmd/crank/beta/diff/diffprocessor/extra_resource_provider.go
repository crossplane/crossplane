package diffprocessor

import (
	"context"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	ucomposite "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"io"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	"github.com/crossplane/crossplane/cmd/crank/render"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ExtraResourceProvider is an interface for components that can identify and fetch
// additional resources needed for rendering.
type ExtraResourceProvider interface {
	// GetExtraResources identifies and fetches extra resources required for rendering
	// based on the composition and existing resources.
	GetExtraResources(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error)
}

// SelectorExtraResourceProvider fetches extra resources of the Selector type from function-extra-resources.
type SelectorExtraResourceProvider struct {
	client cc.ClusterClient
}

// NewSelectorExtraResourceProvider creates a new SelectorExtraResourceProvider.
func NewSelectorExtraResourceProvider(client cc.ClusterClient) *SelectorExtraResourceProvider {
	return &SelectorExtraResourceProvider{
		client: client,
	}
}

// GetExtraResources implements the ExtraResourceProvider interface for Selector type resources.
func (p *SelectorExtraResourceProvider) GetExtraResources(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	gvrs, selectors, err := p.identifyNeededSelectorResources(comp, xr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot identify needed selector resources")
	}

	if len(gvrs) == 0 {
		return nil, nil
	}

	extraResources, err := p.client.GetAllResourcesByLabels(ctx, gvrs, selectors)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get selector resources")
	}

	return extraResources, nil
}

// identifyNeededSelectorResources analyzes a composition to determine what Selector type extra resources are needed.
func (p *SelectorExtraResourceProvider) identifyNeededSelectorResources(comp *apiextensionsv1.Composition, xr *unstructured.Unstructured) ([]schema.GroupVersionResource, []metav1.LabelSelector, error) {
	// If no pipeline mode or no steps, return empty
	if comp.Spec.Mode == nil || *comp.Spec.Mode != apiextensionsv1.CompositionModePipeline {
		return nil, nil, nil
	}

	var resources []schema.GroupVersionResource
	var selectors []metav1.LabelSelector

	// Look for function-extra-resources steps
	for _, step := range comp.Spec.Pipeline {
		if step.FunctionRef.Name != "function-extra-resources" || step.Input == nil {
			continue
		}

		// Parse the input into an unstructured object
		input := &unstructured.Unstructured{}
		if err := input.UnmarshalJSON(step.Input.Raw); err != nil {
			return nil, nil, errors.Wrap(err, "cannot unmarshal function-extra-resources input")
		}

		// Extract extra resources configuration
		extraResources, found, err := unstructured.NestedSlice(input.Object, "spec", "extraResources")
		if err != nil || !found {
			continue
		}

		// Process each extra resource configuration
		for _, er := range extraResources {
			erMap, ok := er.(map[string]interface{})
			if !ok {
				continue
			}

			// Get common resource details
			apiVersion, _, _ := unstructured.NestedString(erMap, "apiVersion")
			kind, _, _ := unstructured.NestedString(erMap, "kind")

			if apiVersion == "" || kind == "" {
				continue
			}

			// Get the resource type - default is Reference
			resourceType, _, _ := unstructured.NestedString(erMap, "type")
			if resourceType == "" {
				resourceType = "Reference"
			}

			// Only process Selector type resources here
			if resourceType != "Selector" {
				continue
			}

			// Create GVR for Selector type resources
			gvr, labelSelector, err := p.processSelector(erMap, apiVersion, kind, xr)
			if err != nil {
				return nil, nil, err
			}
			resources = append(resources, gvr)
			selectors = append(selectors, labelSelector)
		}
	}

	return resources, selectors, nil
}

// processSelector handles the creation of GVR and label selector for Selector type resources.
func (p *SelectorExtraResourceProvider) processSelector(erMap map[string]interface{}, apiVersion, kind string, xr *unstructured.Unstructured) (schema.GroupVersionResource, metav1.LabelSelector, error) {
	// Create GVR for this resource type
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, metav1.LabelSelector{}, errors.Wrapf(err, "cannot parse group version %q", apiVersion)
	}

	gvr := schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: fmt.Sprintf("%ss", strings.ToLower(kind)), // naive pluralization
	}

	// Create label selector
	labelSelector := metav1.LabelSelector{
		MatchLabels: make(map[string]string),
	}

	// Check if selector exists and has matchLabels
	selector, selectorFound, _ := unstructured.NestedMap(erMap, "selector")
	if selectorFound {
		matchLabelsSlice, matchLabelsFound, _ := unstructured.NestedSlice(selector, "matchLabels")
		if matchLabelsFound {
			for _, label := range matchLabelsSlice {
				labelMap, isMap := label.(map[string]interface{})
				if !isMap {
					continue
				}

				// Get the key of the label
				key, keyExists, _ := unstructured.NestedString(labelMap, "key")
				if !keyExists || key == "" {
					continue
				}

				// Get the type of the label (defaults to FromCompositeFieldPath)
				labelType, labelTypeExists, _ := unstructured.NestedString(labelMap, "type")
				if !labelTypeExists {
					labelType = "FromCompositeFieldPath"
				}

				var labelValue string

				switch labelType {
				case "Value":
					// Static value
					value, valueExists, _ := unstructured.NestedString(labelMap, "value")
					if !valueExists || value == "" {
						continue
					}
					labelValue = value

				case "FromCompositeFieldPath":
					// Value from XR field path
					if xr == nil {
						// If no XR is provided, we can't resolve the field path
						// In this case, we skip this label
						continue
					}

					fieldPath, fieldPathExists, _ := unstructured.NestedString(labelMap, "valueFromFieldPath")
					if !fieldPathExists || fieldPath == "" {
						continue
					}

					// Get the value from the XR using the field path
					// First try with the path as is
					fieldValue, err := fieldpath.Pave(xr.UnstructuredContent()).GetString(fieldPath)
					if err == nil && fieldValue != "" {
						labelValue = fieldValue
					} else {
						// If that fails, check if we need to add "spec." prefix for compatibility
						if !strings.HasPrefix(fieldPath, "spec.") && !strings.HasPrefix(fieldPath, "status.") {
							augmentedPath := "spec." + fieldPath
							fieldValue, err = fieldpath.Pave(xr.UnstructuredContent()).GetString(augmentedPath)
							if err == nil && fieldValue != "" {
								labelValue = fieldValue
							} else {
								// Still couldn't find a value, skip this label
								continue
							}
						} else {
							// Path was already in spec/status but value wasn't found, skip
							continue
						}
					}

				default:
					// Unknown label type, skip
					continue
				}

				// Add the label to the selector
				labelSelector.MatchLabels[key] = labelValue
			}
		}
	}

	return gvr, labelSelector, nil
}

// ReferenceExtraResourceProvider fetches extra resources of the Reference type from function-extra-resources.
type ReferenceExtraResourceProvider struct {
	client cc.ClusterClient
}

// NewReferenceExtraResourceProvider creates a new ReferenceExtraResourceProvider.
func NewReferenceExtraResourceProvider(client cc.ClusterClient) *ReferenceExtraResourceProvider {
	return &ReferenceExtraResourceProvider{
		client: client,
	}
}

// GetExtraResources implements the ExtraResourceProvider interface for Reference type resources.
func (p *ReferenceExtraResourceProvider) GetExtraResources(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	references, err := p.identifyNeededReferenceResources(comp, xr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot identify needed reference resources")
	}

	if len(references) == 0 {
		return nil, nil
	}

	var extraResources []*unstructured.Unstructured

	// Fetch each reference resource
	for _, ref := range references {
		resource, err := p.client.GetResource(ctx, ref.gvk, ref.namespace, ref.name)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot get reference resource %s/%s", ref.namespace, ref.name)
		}
		extraResources = append(extraResources, resource)
	}

	return extraResources, nil
}

// referenceResource holds information about a Reference type resource.
type referenceResource struct {
	gvk       schema.GroupVersionKind
	namespace string
	name      string
}

// identifyNeededReferenceResources analyzes a composition to determine what Reference type extra resources are needed.
func (p *ReferenceExtraResourceProvider) identifyNeededReferenceResources(comp *apiextensionsv1.Composition, xr *unstructured.Unstructured) ([]referenceResource, error) {
	// If no pipeline mode or no steps, return empty
	if comp.Spec.Mode == nil || *comp.Spec.Mode != apiextensionsv1.CompositionModePipeline {
		return nil, nil
	}

	var references []referenceResource

	// Look for function-extra-resources steps
	for _, step := range comp.Spec.Pipeline {
		if step.FunctionRef.Name != "function-extra-resources" || step.Input == nil {
			continue
		}

		// Parse the input into an unstructured object
		input := &unstructured.Unstructured{}
		if err := input.UnmarshalJSON(step.Input.Raw); err != nil {
			return nil, errors.Wrap(err, "cannot unmarshal function-extra-resources input")
		}

		// Extract extra resources configuration
		extraResources, found, err := unstructured.NestedSlice(input.Object, "spec", "extraResources")
		if err != nil || !found {
			continue
		}

		// Process each extra resource configuration
		for _, er := range extraResources {
			erMap, ok := er.(map[string]interface{})
			if !ok {
				continue
			}

			// Get common resource details
			apiVersion, _, _ := unstructured.NestedString(erMap, "apiVersion")
			kind, _, _ := unstructured.NestedString(erMap, "kind")

			if apiVersion == "" || kind == "" {
				continue
			}

			// Get the resource type - default is Reference
			resourceType, _, _ := unstructured.NestedString(erMap, "type")
			if resourceType == "" {
				resourceType = "Reference"
			}

			// Only process Reference type resources here
			if resourceType != "Reference" {
				continue
			}

			// Extract reference details
			ref, refFound, _ := unstructured.NestedMap(erMap, "ref")
			if !refFound {
				continue
			}

			name, nameFound, _ := unstructured.NestedString(ref, "name")
			if !nameFound || name == "" {
				continue
			}

			// Namespace is optional for cluster-scoped resources
			namespace, _, _ := unstructured.NestedString(ref, "namespace")

			// Create the GVK
			gvk := schema.GroupVersionKind{
				Group:   strings.Split(apiVersion, "/")[0],
				Version: strings.Split(apiVersion, "/")[1],
				Kind:    kind,
			}

			references = append(references, referenceResource{
				gvk:       gvk,
				namespace: namespace,
				name:      name,
			})
		}
	}

	return references, nil
}

// TemplatedExtraResourceProvider fetches resources emerging from go-templating in the composition.
type TemplatedExtraResourceProvider struct {
	client   cc.ClusterClient
	renderFn RenderFunc
	logger   logging.Logger
}

// NewTemplatedExtraResourceProvider creates a new TemplatedExtraResourceProvider.
func NewTemplatedExtraResourceProvider(client cc.ClusterClient, renderFn RenderFunc, logger logging.Logger) *TemplatedExtraResourceProvider {
	return &TemplatedExtraResourceProvider{
		client:   client,
		renderFn: renderFn,
		logger:   logger,
	}
}

// GetExtraResources implements the ExtraResourceProvider interface for templated extra resources.
func (p *TemplatedExtraResourceProvider) GetExtraResources(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	hasTemplatedExtra, err := ScanForTemplatedExtraResources(comp)
	if err != nil {
		return nil, errors.Wrap(err, "cannot scan for templated extra resources")
	}

	if !hasTemplatedExtra {
		return nil, nil
	}

	// Convert XR unstructured to composite.Unstructured
	xrComposite, err := UnstructuredToComposite(xr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert XR to composite unstructured")
	}

	// Get functions for preliminary render
	fns, err := p.client.GetFunctionsFromPipeline(comp)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get functions from pipeline")
	}

	// Create a slice of unstructured.Unstructured for the render input
	renderResources := make([]unstructured.Unstructured, 0, len(resources))
	for _, r := range resources {
		renderResources = append(renderResources, *r)
	}

	// Perform a preliminary render to generate ExtraResources
	preliminary, err := p.renderFn(ctx, p.logger, render.Inputs{
		CompositeResource: xrComposite,
		Composition:       comp,
		Functions:         fns,
		ExtraResources:    renderResources,
	})
	if err != nil {
		return nil, errors.Wrap(err, "cannot perform preliminary render")
	}

	var extraResources []*unstructured.Unstructured

	// Process the rendered results looking for ExtraResources objects
	for _, result := range preliminary.Results {
		if result.GetKind() == "ExtraResources" {
			additional, err := GetExtraResourcesFromResult(&result)
			if err != nil {
				return nil, errors.Wrap(err, "cannot get extra resources from result")
			}

			extraResources = append(extraResources, additional...)
		}
	}

	return extraResources, nil
}

// UnstructuredToComposite converts an unstructured.Unstructured to a composite.Unstructured.
func UnstructuredToComposite(u *unstructured.Unstructured) (*ucomposite.Unstructured, error) {
	xr := ucomposite.New()
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), xr); err != nil {
		return nil, errors.Wrap(err, "cannot convert unstructured to composite")
	}
	return xr, nil
}

// EnvironmentConfigProvider provides environment configs from the cluster.
type EnvironmentConfigProvider struct {
	configs []*unstructured.Unstructured
}

// NewEnvironmentConfigProvider creates a new EnvironmentConfigProvider.
func NewEnvironmentConfigProvider(configs []*unstructured.Unstructured) *EnvironmentConfigProvider {
	return &EnvironmentConfigProvider{
		configs: configs,
	}
}

// GetExtraResources implements the ExtraResourceProvider interface for environment configs.
func (p *EnvironmentConfigProvider) GetExtraResources(_ context.Context, _ *apiextensionsv1.Composition, _ *unstructured.Unstructured, _ []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	// Just return the cached environment configs
	return p.configs, nil
}

// CompositeExtraResourceProvider combines multiple ExtraResourceProviders.
type CompositeExtraResourceProvider struct {
	providers []ExtraResourceProvider
}

// NewCompositeExtraResourceProvider creates a new CompositeExtraResourceProvider.
func NewCompositeExtraResourceProvider(providers ...ExtraResourceProvider) *CompositeExtraResourceProvider {
	return &CompositeExtraResourceProvider{
		providers: providers,
	}
}

// GetExtraResources implements the ExtraResourceProvider interface.
// It calls each provider in sequence, accumulating resources as it goes.
func (p *CompositeExtraResourceProvider) GetExtraResources(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	allResources := make([]*unstructured.Unstructured, len(resources))
	copy(allResources, resources)

	for _, provider := range p.providers {
		extraResources, err := provider.GetExtraResources(ctx, comp, xr, allResources)
		if err != nil {
			return nil, err
		}

		if extraResources != nil {
			allResources = append(allResources, extraResources...)
		}
	}

	// Return just the newly added resources (excluding the initial resources)
	return allResources[len(resources):], nil
}

func ScanForTemplatedExtraResources(comp *apiextensionsv1.Composition) (bool, error) {
	if comp.Spec.Mode == nil || *comp.Spec.Mode != apiextensionsv1.CompositionModePipeline {
		return false, nil
	}

	for _, step := range comp.Spec.Pipeline {
		if step.FunctionRef.Name != "function-go-templating" || step.Input == nil {
			continue
		}

		// Parse the input into an unstructured object
		input := &unstructured.Unstructured{}
		if err := input.UnmarshalJSON(step.Input.Raw); err != nil {
			return false, errors.Wrap(err, "cannot unmarshal function-go-templating input")
		}

		// Look for template string
		template, found, err := unstructured.NestedString(input.Object, "spec", "inline", "template")
		if err != nil || !found {
			continue
		}

		// Parse the template string as YAML and look for ExtraResources documents
		decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(template), 4096)
		for {
			obj := make(map[string]interface{})
			if err := decoder.Decode(&obj); err != nil {
				if err == io.EOF {
					break
				}
				return false, errors.Wrap(err, "cannot decode template YAML")
			}

			u := &unstructured.Unstructured{Object: obj}
			if u.GetKind() == "ExtraResources" {
				return true, nil
			}
		}
	}

	return false, nil
}

func GetExtraResourcesFromResult(result *unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	spec, found, err := unstructured.NestedMap(result.Object, "spec")
	if err != nil || !found {
		return nil, errors.New("no spec found in ExtraResources result")
	}

	extraResources, found, err := unstructured.NestedSlice(spec, "resources")
	if err != nil || !found {
		return nil, errors.New("no resources found in ExtraResources spec")
	}

	var resources []*unstructured.Unstructured
	for _, er := range extraResources {
		erMap, ok := er.(map[string]interface{})
		if !ok {
			continue
		}

		u := unstructured.Unstructured{Object: erMap}
		resources = append(resources, &u)
	}

	return resources, nil
}
