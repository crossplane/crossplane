package diffprocessor

import (
	"context"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	ucomposite "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"k8s.io/apimachinery/pkg/runtime"
	"regexp"
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

// BaseResourceProvider contains common functionality for all providers
type BaseResourceProvider struct {
	// Common functionality and helper methods
	logger logging.Logger
}

// IsPipelineMode checks if the composition uses the pipeline mode
func (p *BaseResourceProvider) IsPipelineMode(comp *apiextensionsv1.Composition) bool {
	isPipeline := comp.Spec.Mode != nil && *comp.Spec.Mode == apiextensionsv1.CompositionModePipeline

	p.logger.Debug("Checking if composition is in pipeline mode",
		"composition_name", comp.GetName(),
		"is_pipeline", isPipeline)

	return isPipeline
}

// FindStepsWithFunction finds all steps with a specific function name and returns them
func (p *BaseResourceProvider) FindStepsWithFunction(comp *apiextensionsv1.Composition, functionName string) []apiextensionsv1.PipelineStep {
	if !p.IsPipelineMode(comp) {
		p.logger.Debug("Composition is not in pipeline mode, no steps to find",
			"composition_name", comp.GetName())
		return nil
	}

	p.logger.Debug("Searching for steps with function",
		"composition_name", comp.GetName(),
		"function_name", functionName,
		"pipeline_steps", len(comp.Spec.Pipeline))

	var steps []apiextensionsv1.PipelineStep
	for i, step := range comp.Spec.Pipeline {
		if step.FunctionRef.Name == functionName && step.Input != nil {
			p.logger.Debug("Found matching step",
				"index", i,
				"step_name", step.Step,
				"function_name", functionName)
			steps = append(steps, step)
		}
	}

	p.logger.Debug("Found steps with function",
		"function_name", functionName,
		"matches", len(steps))

	return steps
}

// ParseStepInput parses a step's input into an unstructured object
func (p *BaseResourceProvider) ParseStepInput(step apiextensionsv1.PipelineStep) (*unstructured.Unstructured, error) {
	p.logger.Debug("Parsing step input",
		"step_name", step.Step,
		"function_name", step.FunctionRef.Name,
		"input_size", len(step.Input.Raw))

	input := &unstructured.Unstructured{}
	if err := input.UnmarshalJSON(step.Input.Raw); err != nil {
		p.logger.Debug("Failed to unmarshal function input",
			"step_name", step.Step,
			"error", err)
		return nil, errors.Wrap(err, "cannot unmarshal function input")
	}

	p.logger.Debug("Successfully parsed step input",
		"step_name", step.Step,
		"api_version", input.GetAPIVersion(),
		"kind", input.GetKind())

	return input, nil
}

// GetExtraResourcesConfig extracts the extraResources configuration from an input object
func (p *BaseResourceProvider) GetExtraResourcesConfig(input *unstructured.Unstructured) ([]interface{}, error) {
	p.logger.Debug("Extracting extraResources config",
		"input_kind", input.GetKind(),
		"input_api_version", input.GetAPIVersion())

	resources, found, err := unstructured.NestedSlice(input.Object, "spec", "extraResources")
	if err != nil {
		p.logger.Debug("Error extracting extraResources",
			"error", err)
		return nil, errors.Wrap(err, "cannot get extraResources")
	}

	if !found {
		p.logger.Debug("No extraResources found in input")
		return nil, nil
	}

	p.logger.Debug("Found extraResources in input",
		"count", len(resources))

	return resources, nil
}

// ResolveFieldPath resolves a field path in the XR
func (p *BaseResourceProvider) ResolveFieldPath(xr *unstructured.Unstructured, path string) (string, error) {
	p.logger.Debug("Resolving field path",
		"path", path,
		"xr_name", xr.GetName())

	// Try the path as is
	value, err := fieldpath.Pave(xr.UnstructuredContent()).GetString(path)
	if err == nil && value != "" {
		p.logger.Debug("Resolved field path directly",
			"path", path,
			"value", value)
		return value, nil
	}

	// Try with spec. prefix if no prefix was provided
	if !strings.HasPrefix(path, "spec.") && !strings.HasPrefix(path, "status.") {
		augmentedPath := "spec." + path
		p.logger.Debug("Trying augmented path with spec. prefix",
			"original_path", path,
			"augmented_path", augmentedPath)

		value, err = fieldpath.Pave(xr.UnstructuredContent()).GetString(augmentedPath)
		if err == nil && value != "" {
			p.logger.Debug("Resolved field path with spec. prefix",
				"augmented_path", augmentedPath,
				"value", value)
			return value, nil
		}
	}

	p.logger.Debug("Failed to resolve field path",
		"path", path,
		"error", err)

	return "", errors.New("field path not found in XR")
}

// SelectorExtraResourceProvider fetches extra resources of the Selector type
type SelectorExtraResourceProvider struct {
	BaseResourceProvider
	client cc.ClusterClient
}

// NewSelectorExtraResourceProvider creates a new SelectorExtraResourceProvider
func NewSelectorExtraResourceProvider(client cc.ClusterClient, logger logging.Logger) *SelectorExtraResourceProvider {
	return &SelectorExtraResourceProvider{
		BaseResourceProvider: BaseResourceProvider{
			logger: logger,
		},
		client: client,
	}
}

// GetExtraResources implements the ExtraResourceProvider interface
func (p *SelectorExtraResourceProvider) GetExtraResources(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	// If not pipeline mode, return nil
	if !p.IsPipelineMode(comp) {
		p.logger.Debug("Composition is not in pipeline mode, skipping selector resources")
		return nil, nil
	}

	p.logger.Debug("Identifying needed selector resources",
		"composition", comp.GetName(),
		"xr", xr.GetName())

	gvrs, selectors, err := p.identifyNeededSelectorResources(comp, xr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot identify needed selector resources")
	}

	if len(gvrs) == 0 {
		p.logger.Debug("No selector resources needed")
		return nil, nil
	}

	p.logger.Debug("Getting resources from cluster",
		"resource_selectors_count", len(gvrs))

	// Get resources from the cluster using selectors
	extraResources, err := p.client.GetAllResourcesByLabels(ctx, gvrs, selectors)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get selector resources")
	}

	p.logger.Debug("Retrieved selector resources",
		"count", len(extraResources))

	return extraResources, nil
}

// TODO:  now that we are getting `Requirements` from the output of `render`, does this become unnecessary?
// maybe we converge on that approach and always do a prelim render.
// identifyNeededSelectorResources analyzes a composition to determine what Selector type resources are needed
func (p *SelectorExtraResourceProvider) identifyNeededSelectorResources(comp *apiextensionsv1.Composition, xr *unstructured.Unstructured) ([]schema.GroupVersionResource, []metav1.LabelSelector, error) {
	// Find steps with the extra-resources function
	steps := p.FindStepsWithFunction(comp, "function-extra-resources")
	if len(steps) == 0 {
		p.logger.Debug("No function-extra-resources steps found in composition")
		return nil, nil, nil
	}

	p.logger.Debug("Found function-extra-resources steps",
		"count", len(steps))

	var resources []schema.GroupVersionResource
	var selectors []metav1.LabelSelector

	// Process each step
	for i, step := range steps {
		p.logger.Debug("Processing step",
			"index", i,
			"step_name", step.Step)

		input, err := p.ParseStepInput(step)
		if err != nil {
			return nil, nil, errors.Wrap(err, "cannot parse function input")
		}

		// Extract extra resources config
		extraResources, err := p.GetExtraResourcesConfig(input)
		if err != nil || extraResources == nil {
			p.logger.Debug("No extra resources config found in step",
				"step_name", step.Step)
			continue
		}

		p.logger.Debug("Found extra resources in step",
			"step_name", step.Step,
			"count", len(extraResources))

		// Process each extra resource config
		for j, er := range extraResources {
			erMap, ok := er.(map[string]interface{})
			if !ok {
				p.logger.Debug("Extra resource is not a map",
					"index", j)
				continue
			}

			// Get common resource details
			apiVersion, _, _ := unstructured.NestedString(erMap, "apiVersion")
			kind, _, _ := unstructured.NestedString(erMap, "kind")

			if apiVersion == "" || kind == "" {
				p.logger.Debug("Missing apiVersion or kind in extra resource",
					"index", j,
					"apiVersion", apiVersion,
					"kind", kind)
				continue
			}

			// Get the resource type - default is Reference
			resourceType, _, _ := unstructured.NestedString(erMap, "type")
			if resourceType == "" {
				resourceType = "Reference"
			}

			// Only process Selector type resources here
			if resourceType != "Selector" {
				p.logger.Debug("Skipping non-Selector resource",
					"index", j,
					"type", resourceType)
				continue
			}

			p.logger.Debug("Processing Selector resource",
				"index", j,
				"apiVersion", apiVersion,
				"kind", kind)

			// Process selector
			gvr, labelSelector, err := p.processSelector(erMap, apiVersion, kind, xr)
			if err != nil {
				return nil, nil, err
			}

			p.logger.Debug("Created GVR and selector for resource",
				"gvr", gvr.String(),
				"selector", labelSelector.MatchLabels)

			resources = append(resources, gvr)
			selectors = append(selectors, labelSelector)
		}
	}

	p.logger.Debug("Identified selector resources",
		"count", len(resources))

	return resources, selectors, nil
}

// processSelector processes a selector type resource
func (p *SelectorExtraResourceProvider) processSelector(erMap map[string]interface{}, apiVersion, kind string, xr *unstructured.Unstructured) (schema.GroupVersionResource, metav1.LabelSelector, error) {
	// Create GVR
	p.logger.Debug("Creating GVR from apiVersion and kind",
		"apiVersion", apiVersion,
		"kind", kind)

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
	if !selectorFound {
		p.logger.Debug("No selector found in erMap")
		return gvr, labelSelector, nil
	}

	matchLabelsSlice, matchLabelsFound, _ := unstructured.NestedSlice(selector, "matchLabels")
	if !matchLabelsFound {
		p.logger.Debug("No matchLabels found in selector")
		return gvr, labelSelector, nil
	}

	p.logger.Debug("Processing matchLabels",
		"count", len(matchLabelsSlice))

	for i, label := range matchLabelsSlice {
		labelMap, isMap := label.(map[string]interface{})
		if !isMap {
			p.logger.Debug("Label is not a map", "index", i)
			continue
		}

		// Get the key of the label
		key, keyExists, _ := unstructured.NestedString(labelMap, "key")
		if !keyExists || key == "" {
			p.logger.Debug("Missing key in label", "index", i)
			continue
		}

		// Get the type of the label (defaults to FromCompositeFieldPath)
		labelType, labelTypeExists, _ := unstructured.NestedString(labelMap, "type")
		if !labelTypeExists {
			labelType = "FromCompositeFieldPath"
		}

		p.logger.Debug("Processing label",
			"index", i,
			"key", key,
			"type", labelType)

		var labelValue string

		switch labelType {
		case "Value":
			// Static value
			value, valueExists, _ := unstructured.NestedString(labelMap, "value")
			if !valueExists || value == "" {
				p.logger.Debug("Missing value in Value type label", "key", key)
				continue
			}
			labelValue = value
			p.logger.Debug("Using static value for label",
				"key", key,
				"value", labelValue)

		case "FromCompositeFieldPath":
			// Value from XR field path
			if xr == nil {
				p.logger.Debug("XR is nil, cannot resolve field path", "key", key)
				continue
			}

			fieldPath, fieldPathExists, _ := unstructured.NestedString(labelMap, "valueFromFieldPath")
			if !fieldPathExists || fieldPath == "" {
				p.logger.Debug("Missing fieldPath in FromCompositeFieldPath type label", "key", key)
				continue
			}

			// Use base provider's field path resolution
			fieldValue, err := p.ResolveFieldPath(xr, fieldPath)
			if err != nil {
				p.logger.Debug("Failed to resolve field path",
					"key", key,
					"path", fieldPath,
					"error", err)
				continue
			}

			labelValue = fieldValue
			p.logger.Debug("Resolved field path for label",
				"key", key,
				"path", fieldPath,
				"value", labelValue)

		default:
			// Unknown label type, skip
			p.logger.Debug("Unknown label type",
				"key", key,
				"type", labelType)
			continue
		}

		// Add the label to the selector
		labelSelector.MatchLabels[key] = labelValue
	}

	p.logger.Debug("Created label selector",
		"gvr", gvr.String(),
		"labels", labelSelector.MatchLabels)

	return gvr, labelSelector, nil
}

// ReferenceExtraResourceProvider fetches extra resources of the Reference type
type ReferenceExtraResourceProvider struct {
	BaseResourceProvider
	client cc.ClusterClient
}

// NewReferenceExtraResourceProvider creates a new ReferenceExtraResourceProvider
func NewReferenceExtraResourceProvider(client cc.ClusterClient, logger logging.Logger) *ReferenceExtraResourceProvider {
	return &ReferenceExtraResourceProvider{
		BaseResourceProvider: BaseResourceProvider{
			logger: logger,
		},
		client: client,
	}
}

// GetExtraResources implements the ExtraResourceProvider interface
func (p *ReferenceExtraResourceProvider) GetExtraResources(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	// If not pipeline mode, return nil
	if !p.IsPipelineMode(comp) {
		p.logger.Debug("Composition is not in pipeline mode, skipping reference resources")
		return nil, nil
	}

	p.logger.Debug("Identifying needed reference resources",
		"composition", comp.GetName(),
		"xr", xr.GetName())

	references, err := p.identifyNeededReferenceResources(comp, xr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot identify needed reference resources")
	}

	if len(references) == 0 {
		p.logger.Debug("No reference resources needed")
		return nil, nil
	}

	p.logger.Debug("Getting reference resources from cluster",
		"count", len(references))

	var extraResources []*unstructured.Unstructured

	// Fetch each reference resource
	for i, ref := range references {
		p.logger.Debug("Fetching reference resource",
			"index", i,
			"gvk", ref.gvk.String(),
			"namespace", ref.namespace,
			"name", ref.name)

		resource, err := p.client.GetResource(ctx, ref.gvk, ref.namespace, ref.name)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot get reference resource %s/%s", ref.namespace, ref.name)
		}

		p.logger.Debug("Retrieved reference resource",
			"index", i,
			"name", resource.GetName(),
			"kind", resource.GetKind())

		extraResources = append(extraResources, resource)
	}

	p.logger.Debug("Retrieved all reference resources",
		"count", len(extraResources))

	return extraResources, nil
}

// referenceResource holds information about a Reference type resource
type referenceResource struct {
	gvk       schema.GroupVersionKind
	namespace string
	name      string
}

// identifyNeededReferenceResources analyzes a composition to determine what Reference type resources are needed
func (p *ReferenceExtraResourceProvider) identifyNeededReferenceResources(comp *apiextensionsv1.Composition, xr *unstructured.Unstructured) ([]referenceResource, error) {
	// Find steps with the extra-resources function
	steps := p.FindStepsWithFunction(comp, "function-extra-resources")
	if len(steps) == 0 {
		p.logger.Debug("No function-extra-resources steps found in composition")
		return nil, nil
	}

	p.logger.Debug("Found function-extra-resources steps",
		"count", len(steps))

	var references []referenceResource

	// Process each step
	for i, step := range steps {
		p.logger.Debug("Processing step",
			"index", i,
			"step_name", step.Step)

		input, err := p.ParseStepInput(step)
		if err != nil {
			return nil, errors.Wrap(err, "cannot parse function input")
		}

		// Extract extra resources config
		extraResources, err := p.GetExtraResourcesConfig(input)
		if err != nil || extraResources == nil {
			p.logger.Debug("No extra resources config found in step",
				"step_name", step.Step)
			continue
		}

		p.logger.Debug("Found extra resources in step",
			"step_name", step.Step,
			"count", len(extraResources))

		// Process each extra resource config
		for j, er := range extraResources {
			erMap, ok := er.(map[string]interface{})
			if !ok {
				p.logger.Debug("Extra resource is not a map",
					"index", j)
				continue
			}

			// Get common resource details
			apiVersion, _, _ := unstructured.NestedString(erMap, "apiVersion")
			kind, _, _ := unstructured.NestedString(erMap, "kind")

			if apiVersion == "" || kind == "" {
				p.logger.Debug("Missing apiVersion or kind in extra resource",
					"index", j,
					"apiVersion", apiVersion,
					"kind", kind)
				continue
			}

			// Get the resource type - default is Reference
			resourceType, _, _ := unstructured.NestedString(erMap, "type")
			if resourceType == "" {
				resourceType = "Reference"
			}

			// Only process Reference type resources here
			if resourceType != "Reference" {
				p.logger.Debug("Skipping non-Reference resource",
					"index", j,
					"type", resourceType)
				continue
			}

			p.logger.Debug("Processing Reference resource",
				"index", j,
				"apiVersion", apiVersion,
				"kind", kind)

			// Extract reference details
			ref, refFound, _ := unstructured.NestedMap(erMap, "ref")
			if !refFound {
				p.logger.Debug("No ref field found in Reference resource")
				continue
			}

			name, nameFound, _ := unstructured.NestedString(ref, "name")
			if !nameFound || name == "" {
				p.logger.Debug("Missing name in ref field")
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

			p.logger.Debug("Adding reference resource",
				"gvk", gvk.String(),
				"namespace", namespace,
				"name", name)

			references = append(references, referenceResource{
				gvk:       gvk,
				namespace: namespace,
				name:      name,
			})
		}
	}

	p.logger.Debug("Identified reference resources",
		"count", len(references))

	return references, nil
}

// TemplatedExtraResourceProvider fetches resources from go-templating
type TemplatedExtraResourceProvider struct {
	BaseResourceProvider
	client   cc.ClusterClient
	renderFn RenderFunc
	logger   logging.Logger
}

// NewTemplatedExtraResourceProvider creates a new TemplatedExtraResourceProvider
func NewTemplatedExtraResourceProvider(client cc.ClusterClient, renderFn RenderFunc, logger logging.Logger) *TemplatedExtraResourceProvider {
	return &TemplatedExtraResourceProvider{
		BaseResourceProvider: BaseResourceProvider{
			logger: logger,
		},
		client:   client,
		renderFn: renderFn,
		logger:   logger,
	}
}

// GetExtraResources implements the ExtraResourceProvider interface
func (p *TemplatedExtraResourceProvider) GetExtraResources(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	// If not pipeline mode, return nil
	if !p.IsPipelineMode(comp) {
		p.logger.Debug("Composition is not in pipeline mode, skipping templated resources")
		return nil, nil
	}

	p.logger.Debug("Scanning for templated resources in composition",
		"composition", comp.GetName())

	matchingSteps, err := p.ScanForTemplatedResources(comp)
	if err != nil {
		return nil, errors.Wrap(err, "cannot scan for templated extra resources")
	}

	if len(matchingSteps) == 0 {
		p.logger.Debug("No templated extra resources found in composition")
		return nil, nil
	}

	p.logger.Debug("Found templated resource steps",
		"count", len(matchingSteps),
		"steps", matchingSteps)

	// Convert XR unstructured to composite.Unstructured
	p.logger.Debug("Converting XR to composite.Unstructured",
		"xr", xr.GetName())

	xrComposite, err := UnstructuredToComposite(xr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert XR to composite unstructured")
	}

	// Get functions for preliminary render
	p.logger.Debug("Getting functions for preliminary render")

	fns, err := p.client.GetFunctionsFromPipeline(comp)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get functions from pipeline")
	}

	p.logger.Debug("Creating render resources from existing resources",
		"resources_count", len(resources))

	// Create a slice of unstructured.Unstructured for the render input
	renderResources := make([]unstructured.Unstructured, 0, len(resources))
	for _, r := range resources {
		renderResources = append(renderResources, *r)
	}

	// Perform a preliminary render to identify required ExtraResources
	p.logger.Debug("Performing preliminary render to identify ExtraResources requirements")

	preliminary, err := p.renderFn(ctx, p.logger, render.Inputs{
		CompositeResource: xrComposite,
		Composition:       comp,
		Functions:         fns,
		ExtraResources:    renderResources,
	})
	if err != nil {
		return nil, errors.Wrap(err, "cannot perform preliminary render")
	}

	// Process requirements from the preliminary render to get required resources
	p.logger.Debug("Processing requirements from preliminary render",
		"requirements_count", len(preliminary.Requirements))

	extraFromRequirements, err := p.ProcessRequirements(ctx, matchingSteps, preliminary)
	if err != nil {
		return nil, errors.Wrap(err, "cannot process requirements from preliminary render")
	}

	p.logger.Debug("Retrieved templated extra resources",
		"count", len(extraFromRequirements))

	return extraFromRequirements, nil
}

// ProcessRequirements extracts and fetches resources from Requirements in render output
func (p *TemplatedExtraResourceProvider) ProcessRequirements(ctx context.Context, steps []string, prelimOutput render.Outputs) ([]*unstructured.Unstructured, error) {
	var extraResources []*unstructured.Unstructured

	// If no requirements, return empty slice
	if len(prelimOutput.Requirements) == 0 {
		p.logger.Debug("No requirements in preliminary output")
		return extraResources, nil
	}

	p.logger.Debug("Processing requirements from preliminary output",
		"steps_count", len(steps),
		"requirements_count", len(prelimOutput.Requirements))

	// Iterate over each step's requirements
	for _, stepName := range steps {
		reqs, found := prelimOutput.Requirements[stepName]
		if !found {
			p.logger.Debug("No requirements found for step", "step", stepName)
			continue
		}

		p.logger.Debug("Processing requirements for step",
			"step", stepName,
			"extra_resources_count", len(reqs.ExtraResources))

		// Process each resource selector in ExtraResources
		for resourceKey, selector := range reqs.ExtraResources {
			// Convert to a standard ResourceSelector (from fnv1.ResourceSelector)
			if selector == nil {
				p.logger.Debug("Nil selector for resource key", "key", resourceKey)
				continue
			}

			p.logger.Debug("Processing resource selector",
				"key", resourceKey,
				"api_version", selector.ApiVersion,
				"kind", selector.Kind)

			// split apiVersion into group and version
			group, version := "", ""
			if parts := strings.SplitN(selector.ApiVersion, "/", 2); len(parts) == 2 {
				// Normal case: group/version (e.g., "apps/v1")
				group, version = parts[0], parts[1]
			} else {
				// Core case: version only (e.g., "v1")
				version = selector.ApiVersion
			}

			p.logger.Debug("Parsed API version",
				"group", group,
				"version", version)

			// Determine the type of selector and fetch accordingly
			switch {
			case selector.GetMatchName() != "":
				// Reference type (matchName)
				gvk := schema.GroupVersionKind{
					Group:   group,
					Version: version,
					Kind:    selector.Kind,
				}

				name := selector.GetMatchName()
				// TODO namespacing?
				ns := ""

				p.logger.Debug("Fetching reference resource by name",
					"gvk", gvk.String(),
					"namespace", ns,
					"name", name)

				resource, err := p.client.GetResource(ctx, gvk, ns, name)
				if err != nil {
					return nil, errors.Wrapf(err, "cannot get reference resource %s/%s from requirement in step %s", ns, name, stepName)
				}

				p.logger.Debug("Retrieved reference resource",
					"kind", resource.GetKind(),
					"name", resource.GetName())

				extraResources = append(extraResources, resource)

			case selector.GetMatchLabels() != nil:
				// Selector type (matchLabels)
				gvr := schema.GroupVersionResource{
					Group:    group,
					Version:  version,
					Resource: fmt.Sprintf("%ss", strings.ToLower(selector.Kind)), // naive pluralization
				}

				// Convert MatchLabels to LabelSelector
				labelSelector := metav1.LabelSelector{
					MatchLabels: selector.GetMatchLabels().GetLabels(),
				}

				p.logger.Debug("Fetching resources by labels",
					"gvr", gvr.String(),
					"labels", labelSelector.MatchLabels)

				resources, err := p.client.GetResourcesByLabel(ctx, "", gvr, labelSelector)
				if err != nil {
					return nil, errors.Wrapf(err, "cannot get resources matching labels for requirement in step %s", stepName)
				}

				p.logger.Debug("Retrieved resources by label",
					"count", len(resources))

				extraResources = append(extraResources, resources...)

			default:
				p.logger.Debug("Unsupported selector type, neither matchName nor matchLabels specified")
			}
		}
	}

	p.logger.Debug("Completed processing all requirements",
		"resources_count", len(extraResources))

	return extraResources, nil
}

// ScanForTemplatedResources checks if the composition has templated extra resources
func (p *TemplatedExtraResourceProvider) ScanForTemplatedResources(comp *apiextensionsv1.Composition) ([]string, error) {
	var matchingSteps []string

	// Find steps with go-templating function
	steps := p.FindStepsWithFunction(comp, "function-go-templating")
	if len(steps) == 0 {
		p.logger.Debug("No function-go-templating steps found in composition")
		return matchingSteps, nil
	}

	p.logger.Debug("Found go-templating steps to scan",
		"count", len(steps))

	for i, step := range steps {
		p.logger.Debug("Scanning step for templated resources",
			"index", i,
			"step_name", step.Step)

		input, err := p.ParseStepInput(step)
		if err != nil {
			return matchingSteps, errors.Wrap(err, "cannot parse function input")
		}

		// Look for template string
		template, found, err := unstructured.NestedString(input.Object, "inline", "template")
		if err != nil || !found {
			p.logger.Debug("No inline template found in step",
				"step_name", step.Step)
			continue
		}

		// Simple string search for the key pattern with whitespace variations
		// This is much more reliable than trying to parse the template
		patternRegex := regexp.MustCompile(`(?i)kind\s*:\s*['"]?ExtraResources['"]?`)
		if patternRegex.MatchString(template) {
			p.logger.Debug("Found ExtraResources reference in template",
				"step_name", step.Step)
			matchingSteps = append(matchingSteps, step.Step)
		} else {
			p.logger.Debug("No ExtraResources reference found in template",
				"step_name", step.Step)
		}
	}

	p.logger.Debug("Completed scanning for templated resources",
		"matching_steps_count", len(matchingSteps))

	return matchingSteps, nil
}

// GetExtraResourcesFromResult extracts resources from an ExtraResources result
func (p *TemplatedExtraResourceProvider) GetExtraResourcesFromResult(result *unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
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
	BaseResourceProvider
	configs []*unstructured.Unstructured
}

// NewEnvironmentConfigProvider creates a new EnvironmentConfigProvider.
func NewEnvironmentConfigProvider(configs []*unstructured.Unstructured, logger logging.Logger) *EnvironmentConfigProvider {
	if logger == nil {
		logger = logging.NewNopLogger()
	}
	return &EnvironmentConfigProvider{
		BaseResourceProvider: BaseResourceProvider{
			logger: logger,
		},
		configs: configs,
	}
}

// GetExtraResources implements the ExtraResourceProvider interface for environment configs.
func (p *EnvironmentConfigProvider) GetExtraResources(_ context.Context, comp *apiextensionsv1.Composition, _ *unstructured.Unstructured, _ []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	p.logger.Debug("Providing environment configs",
		"config_count", len(p.configs),
		"composition_name", comp.GetName())

	if len(p.configs) == 0 {
		p.logger.Debug("No environment configs to provide")
	} else {
		for i, cfg := range p.configs {
			p.logger.Debug("Environment config",
				"index", i,
				"name", cfg.GetName())
		}
	}

	// Just return the cached environment configs
	return p.configs, nil
}

// NewCompositeExtraResourceProvider creates a new CompositeExtraResourceProvider.
func NewCompositeExtraResourceProvider(logger logging.Logger, providers ...ExtraResourceProvider) *CompositeExtraResourceProvider {
	if logger == nil {
		logger = logging.NewNopLogger()
	}
	return &CompositeExtraResourceProvider{
		providers: providers,
		logger:    logger,
	}
}

type CompositeExtraResourceProvider struct {
	providers []ExtraResourceProvider
	logger    logging.Logger
}

// GetExtraResources implements the ExtraResourceProvider interface.
// It calls each provider in sequence, accumulating resources as it goes.
func (p *CompositeExtraResourceProvider) GetExtraResources(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	compName := comp.GetName()
	xrName := xr.GetName()

	p.logger.Debug("Getting extra resources",
		"composition", compName,
		"xr", xrName,
		"providersCount", len(p.providers),
		"existingResourcesCount", len(resources))

	allResources := make([]*unstructured.Unstructured, len(resources))
	copy(allResources, resources)

	for i, provider := range p.providers {
		providerType := fmt.Sprintf("%T", provider)

		p.logger.Debug("Querying provider",
			"index", i,
			"type", providerType)

		extraResources, err := provider.GetExtraResources(ctx, comp, xr, allResources)
		if err != nil {
			p.logger.Debug("Provider error",
				"type", providerType,
				"error", err)
			return nil, err
		}

		if len(extraResources) > 0 {
			p.logger.Debug("Provider returned resources",
				"type", providerType,
				"count", len(extraResources))
			allResources = append(allResources, extraResources...)
		}
	}

	// Return just the newly added resources (excluding the initial resources)
	newlyAddedResources := allResources[len(resources):]

	p.logger.Debug("Extra resources collected",
		"composition", compName,
		"xr", xrName,
		"count", len(newlyAddedResources))

	return newlyAddedResources, nil
}

func GetExtraResourcesFromResult(result *unstructured.Unstructured, logger logging.Logger) ([]*unstructured.Unstructured, error) {
	logger.Debug("Extracting resources from ExtraResources result",
		"result_name", result.GetName())

	spec, found, err := unstructured.NestedMap(result.Object, "spec")
	if err != nil || !found {
		logger.Debug("No spec found in ExtraResources result",
			"error", err)
		return nil, errors.New("no spec found in ExtraResources result")
	}

	extraResources, found, err := unstructured.NestedSlice(spec, "resources")
	if err != nil || !found {
		logger.Debug("No resources found in ExtraResources spec",
			"error", err)
		return nil, errors.New("no resources found in ExtraResources spec")
	}

	logger.Debug("Found resources in ExtraResources result",
		"count", len(extraResources))

	var resources []*unstructured.Unstructured
	for i, er := range extraResources {
		erMap, ok := er.(map[string]interface{})
		if !ok {
			logger.Debug("Resource is not a map", "index", i)
			continue
		}

		u := unstructured.Unstructured{Object: erMap}
		logger.Debug("Extracted resource",
			"index", i,
			"kind", u.GetKind(),
			"name", u.GetName())
		resources = append(resources, &u)
	}

	logger.Debug("Extracted resources from result",
		"count", len(resources))

	return resources, nil
}
