package diffprocessor

import (
	"context"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	ucomposite "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal"
	"github.com/crossplane/crossplane/cmd/crank/beta/validate"
	"github.com/crossplane/crossplane/cmd/crank/render"
	"io"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml" // For NewYAMLOrJSONDecoder
	"k8s.io/client-go/rest"
	"reflect"
	sigsyaml "sigs.k8s.io/yaml" // For Marshal/Unmarshal functionality (aliased to avoid conflicts)

	"strings"
)

// RenderFunc defines the signature of a function that can render resources
type RenderFunc func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error)

// DiffProcessor interface for processing resources
type DiffProcessor interface {
	ProcessAll(stdout io.Writer, ctx context.Context, resources []*unstructured.Unstructured) error
	ProcessResource(stdout io.Writer, ctx context.Context, res *unstructured.Unstructured) error
	Initialize(writer io.Writer, ctx context.Context) error
}

// DefaultDiffProcessor handles the processing of resources for diffing.
type DefaultDiffProcessor struct {
	client    cc.ClusterClient
	config    *rest.Config
	namespace string
	renderFn  RenderFunc
	log       logging.Logger
	manager   *validate.Manager
}

// NewDiffProcessor creates a new DefaultDiffProcessor
// If renderFn is nil, it defaults to render.Render
func NewDiffProcessor(config *rest.Config, client cc.ClusterClient, namespace string, renderFn RenderFunc, logger logging.Logger) (DiffProcessor, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}
	if client == nil {
		return nil, errors.New("client cannot be nil")
	}

	// Default to the standard Render function if none provided
	if renderFn == nil {
		renderFn = render.Render
	}
	if logger == nil {
		logger = logging.NewNopLogger()
	}

	return &DefaultDiffProcessor{
		client:    client,
		config:    config,
		namespace: namespace,
		renderFn:  renderFn,
		log:       logger,
	}, nil
}

func (p *DefaultDiffProcessor) Initialize(writer io.Writer, ctx context.Context) error {
	xrds, err := p.client.GetXRDs(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot get XRDs")
	}

	// TODO:  we are initializing this with constants; probably make sure we don't need the downstream cache, etc
	// since we pull direct from cluster
	m := validate.NewManager("~/.crossplane/cache", nil, writer)

	// Convert XRDs/CRDs to CRDs and add package dependencies
	if err := m.PrepExtensions(xrds); err != nil {
		return errors.Wrapf(err, "cannot prepare extensions")
	}

	p.manager = m

	return nil
}

// ProcessAll handles all resources stored in the processor.
func (p *DefaultDiffProcessor) ProcessAll(stdout io.Writer, ctx context.Context, resources []*unstructured.Unstructured) error {
	var errs []error
	for _, res := range resources {
		if err := p.ProcessResource(stdout, ctx, res); err != nil {
			errs = append(errs, errors.Wrapf(err, "unable to process resource %s", res.GetName()))
		}
	}

	return errors.Join(errs...)
}

// ProcessResource handles one resource at a time.
func (p *DefaultDiffProcessor) ProcessResource(stdout io.Writer, ctx context.Context, res *unstructured.Unstructured) error {
	comp, err := p.client.FindMatchingComposition(res)
	if err != nil {
		return errors.Wrap(err, "cannot find matching composition")
	}

	gvrs, selectors, err := p.IdentifyNeededExtraResources(comp)
	if err != nil {
		return errors.Wrap(err, "cannot identify needed extra resources")
	}

	extraResources, err := p.client.GetExtraResources(ctx, gvrs, selectors)
	if err != nil {
		return errors.Wrap(err, "cannot get extra resources")
	}

	fns, err := p.client.GetFunctionsFromPipeline(comp)
	if err != nil {
		return errors.Wrap(err, "cannot get functions from pipeline")
	}

	xr := ucomposite.New()
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(res.UnstructuredContent(), xr); err != nil {
		return errors.Wrap(err, "cannot convert XR to composite unstructured")
	}

	hasTemplatedExtra, err := ScanForTemplatedExtraResources(comp)
	if err != nil {
		return errors.Wrap(err, "cannot scan for templated extra resources")
	}

	if hasTemplatedExtra {
		extraResources, err = p.HandleTemplatedExtraResources(ctx, comp, xr, fns, extraResources)
		if err != nil {
			return err
		}
	}

	desired, err := p.renderFn(ctx, nil, render.Inputs{
		CompositeResource: xr,
		Composition:       comp,
		Functions:         fns,
		// don't dereference the slice until the last minute
		ExtraResources: internal.DereferenceSlice(extraResources),
	})
	if err != nil {
		return errors.Wrap(err, "cannot render resources")
	}

	if err := p.ValidateResources(stdout, desired); err != nil {
		return errors.Wrap(err, "cannot validate resources")
	}

	for _, d := range desired.ComposedResources {
		diff, err := p.CalculateDiff(ctx, &d)
		if err != nil {
			return errors.Wrap(err, "cannot calculate diff")
		}
		if diff != "" {
			_, _ = fmt.Fprintf(stdout, "%s\n---\n", diff)
		}
	}
	return nil
}

// IdentifyNeededExtraResources analyzes a composition to determine what extra resources are needed
func (p *DefaultDiffProcessor) IdentifyNeededExtraResources(comp *apiextensionsv1.Composition) ([]schema.GroupVersionResource, []metav1.LabelSelector, error) {
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

			// Get the resource selector details
			apiVersion, _, _ := unstructured.NestedString(erMap, "apiVersion")
			kind, _, _ := unstructured.NestedString(erMap, "kind")
			selector, _, _ := unstructured.NestedMap(erMap, "selector", "matchLabels")

			if apiVersion == "" || kind == "" {
				continue
			}

			// Create GVR for this resource type
			gv, err := schema.ParseGroupVersion(apiVersion)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "cannot parse group version %q", apiVersion)
			}

			gvr := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: fmt.Sprintf("%ss", strings.ToLower(kind)), // naive pluralization
			}
			resources = append(resources, gvr)

			// Create label selector
			labelSelector := metav1.LabelSelector{
				MatchLabels: make(map[string]string),
			}
			for k, v := range selector {
				if s, ok := v.(string); ok {
					labelSelector.MatchLabels[k] = s
				}
			}
			selectors = append(selectors, labelSelector)
		}
	}

	return resources, selectors, nil
}

// HandleTemplatedExtraResources processes templated extra resources.
func (p *DefaultDiffProcessor) HandleTemplatedExtraResources(ctx context.Context, comp *apiextensionsv1.Composition, xr *ucomposite.Unstructured, fns []pkgv1.Function, extraResources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	preliminary, err := render.Render(ctx, nil, render.Inputs{
		CompositeResource: xr,
		Composition:       comp,
		Functions:         fns,
		ExtraResources:    internal.DereferenceSlice(extraResources),
	})
	if err != nil {
		return nil, errors.Wrap(err, "cannot perform preliminary render")
	}

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

func (p *DefaultDiffProcessor) ValidateResources(writer io.Writer, desired render.Outputs) error {

	// Convert XR and composed resources to unstructured
	resources := make([]*unstructured.Unstructured, 0, len(desired.ComposedResources)+1)

	// Convert XR from composite.Unstructured to regular Unstructured
	xr := &unstructured.Unstructured{Object: desired.CompositeResource.UnstructuredContent()}
	resources = append(resources, xr)

	// Add composed resources to validation list
	for i := range desired.ComposedResources {
		resources = append(resources, &unstructured.Unstructured{Object: desired.ComposedResources[i].UnstructuredContent()})
	}

	// Validate using the converted CRD schema
	if err := validate.SchemaValidation(resources, p.manager.GetCRDs(), true, writer); err != nil {
		return errors.Wrap(err, "schema validation failed")
	}

	return nil
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

// CalculateDiff calculates the difference between desired state and current state
// using the ClusterClient's DryRunApply method
func (p *DefaultDiffProcessor) CalculateDiff(ctx context.Context, desired runtime.Object) (string, error) {
	// Convert desired to unstructured
	desiredUnstr, ok := desired.(*unstructured.Unstructured)
	if !ok {
		return "", errors.New("desired object is not unstructured")
	}

	// Get the GroupVersionKind and name/namespace for lookup
	gvk := desiredUnstr.GroupVersionKind()
	name := desiredUnstr.GetName()
	namespace := desiredUnstr.GetNamespace()

	// Get the current object from the cluster using ClusterClient
	current, err := p.client.GetResource(ctx, gvk, namespace, name)

	// If not found, show the entire object as new
	if apierrors.IsNotFound(err) {
		yamlBytes, err := sigsyaml.Marshal(desiredUnstr.Object)
		if err != nil {
			return "", errors.Wrap(err, "cannot marshal desired object to YAML")
		}
		return fmt.Sprintf("+ %s (new object)\n%s", gvk.Kind, string(yamlBytes)), nil
	} else if err != nil {
		return "", errors.Wrap(err, "cannot get current object")
	}

	// Use server-side apply dry run to calculate changes
	// Create a copy of the desired object to use for the dry run
	dryRunObj := desiredUnstr.DeepCopy()

	// Set the resource version from the current object for conflict detection
	dryRunObj.SetResourceVersion(current.GetResourceVersion())

	// Perform a dry-run server-side apply using ClusterClient
	dryRunResult, err := p.client.DryRunApply(ctx, dryRunObj)
	if err != nil {
		return "", errors.Wrap(err, "cannot perform dry-run apply")
	}

	// Calculate the structural differences between current and dry run result
	// using our new YAML-based diff approach
	diff, err := calculateStructuralDiff(current, dryRunResult)
	if err != nil {
		return "", errors.Wrap(err, "cannot calculate structural diff")
	}

	// If there are no changes, return empty string
	if diff == "" {
		return "", nil
	}

	// Format the diff
	return fmt.Sprintf("~ %s/%s\n%s", gvk.Kind, name, diff), nil
}

// calculateStructuralDiff compares current and desired objects and returns a formatted diff
func calculateStructuralDiff(current, desired *unstructured.Unstructured) (string, error) {
	// Remove fields we don't want to include in the diff
	current = cleanupForDiff(current.DeepCopy())
	desired = cleanupForDiff(desired.DeepCopy())

	// If they're identical, there's no diff
	if reflect.DeepEqual(current.Object, desired.Object) {
		return "", nil
	}

	// Calculate the difference between the objects directly
	diffMap := buildDiffMap(current.Object, desired.Object)
	if len(diffMap) == 0 {
		return "", nil
	}

	// Convert the diff to YAML
	yamlBytes, err := sigsyaml.Marshal(diffMap)
	if err != nil {
		return "", errors.Wrap(err, "cannot marshal diff to YAML")
	}

	return string(yamlBytes), nil
}

// buildDiffMap creates a map of differences between current and desired state
// It recursively compares objects and builds a map containing only changed fields
func buildDiffMap(current, desired map[string]interface{}) map[string]interface{} {
	diffMap := make(map[string]interface{})

	// Process all keys in desired
	for key, desiredValue := range desired {
		// Skip metadata.resourceVersion and other server-side fields
		if key == "status" {
			continue
		}

		currentValue, exists := current[key]

		// If the key doesn't exist in current, or values are different
		if !exists || !reflect.DeepEqual(currentValue, desiredValue) {
			// Recursively handle nested maps
			if desiredMap, isDesiredMap := desiredValue.(map[string]interface{}); isDesiredMap {
				if currentMap, isCurrentMap := currentValue.(map[string]interface{}); isCurrentMap {
					// Both are maps, recursively compute diff
					nestedDiff := buildDiffMap(currentMap, desiredMap)
					if len(nestedDiff) > 0 {
						diffMap[key] = nestedDiff
					}
					continue
				}
			}

			// Handle arrays specially to preserve YAML list notation
			if desiredArray, isDesiredArray := desiredValue.([]interface{}); isDesiredArray {
				if currentArray, isCurrentArray := currentValue.([]interface{}); isCurrentArray {
					arrayDiff := buildArrayDiff(currentArray, desiredArray)
					if arrayDiff != nil {
						diffMap[key] = arrayDiff
					}
					continue
				}
			}

			// For any other type, include the value if it's different
			diffMap[key] = desiredValue
		}
	}

	return diffMap
}

// buildArrayDiff compares arrays and returns either nil (if no differences)
// or a slice containing the desired array elements that differ from current
func buildArrayDiff(current, desired []interface{}) []interface{} {
	hasDifferences := false
	result := make([]interface{}, len(desired))

	// Compare elements up to the shorter length
	minLen := len(current)
	if len(desired) < minLen {
		minLen = len(desired)
	}

	for i := 0; i < minLen; i++ {
		currentItem := current[i]
		desiredItem := desired[i]

		if !reflect.DeepEqual(currentItem, desiredItem) {
			// Recursively handle maps in arrays
			if desiredMap, isDesiredMap := desiredItem.(map[string]interface{}); isDesiredMap {
				if currentMap, isCurrentMap := currentItem.(map[string]interface{}); isCurrentMap {
					// Both are maps, recursively compute diff
					nestedDiff := buildDiffMap(currentMap, desiredMap)
					if len(nestedDiff) > 0 {
						result[i] = nestedDiff
						hasDifferences = true
					} else {
						result[i] = desiredItem // No changes, use original
					}
					continue
				}
			}

			// Handle nested arrays
			if desiredArray, isDesiredArray := desiredItem.([]interface{}); isDesiredArray {
				if currentArray, isCurrentArray := currentItem.([]interface{}); isCurrentArray {
					nestedArrayDiff := buildArrayDiff(currentArray, desiredArray)
					if nestedArrayDiff != nil {
						result[i] = nestedArrayDiff
						hasDifferences = true
					} else {
						result[i] = desiredItem // No changes, use original
					}
					continue
				}
			}

			// For other types, include the item if different
			result[i] = desiredItem
			hasDifferences = true
		} else {
			// No difference, copy the item
			result[i] = desiredItem
		}
	}

	// If desired array is longer, include additional elements
	for i := minLen; i < len(desired); i++ {
		result[i] = desired[i]
		hasDifferences = true
	}

	// If current array is longer, we consider this a difference
	if len(current) > len(desired) {
		hasDifferences = true
	}

	if hasDifferences {
		return result
	}
	return nil
}

// cleanupForDiff removes fields that shouldn't be included in the diff
func cleanupForDiff(obj *unstructured.Unstructured) *unstructured.Unstructured {
	// Remove server-side fields and metadata that we don't want to diff
	metadata, found, _ := unstructured.NestedMap(obj.Object, "metadata")
	if found {
		// Remove fields that change automatically or are server-side
		fieldsToRemove := []string{
			"resourceVersion",
			"uid",
			"generation",
			"creationTimestamp",
			"managedFields",
			"selfLink",
		}

		for _, field := range fieldsToRemove {
			delete(metadata, field)
		}

		unstructured.SetNestedMap(obj.Object, metadata, "metadata")
	}

	// Remove status field as we're focused on spec changes
	delete(obj.Object, "status")

	return obj
}
