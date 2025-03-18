package diffprocessor

import (
	"context"
	"dario.cat/mergo"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	ucomposite "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal"
	"github.com/crossplane/crossplane/cmd/crank/beta/validate"
	"github.com/crossplane/crossplane/cmd/crank/render"
	"io"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/uuid"
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
	client                cc.ClusterClient
	config                ProcessorConfig
	crds                  []*extv1.CustomResourceDefinition
	extraResourceProvider ExtraResourceProvider
}

// NewDiffProcessor creates a new DefaultDiffProcessor with the provided options
func NewDiffProcessor(client cc.ClusterClient, options ...DiffProcessorOption) (DiffProcessor, error) {
	if client == nil {
		return nil, errors.New("client cannot be nil")
	}

	// Create default configuration
	config := ProcessorConfig{
		Namespace:  "default",
		Colorize:   true,
		Compact:    false,
		Logger:     logging.NewNopLogger(),
		RenderFunc: render.Render,
	}

	// Apply all provided options
	for _, option := range options {
		option(&config)
	}

	// Validate required fields
	if config.RestConfig == nil {
		return nil, errors.New("REST config cannot be nil")
	}

	processor := &DefaultDiffProcessor{
		client: client,
		config: config,
		crds:   []*extv1.CustomResourceDefinition{},
	}

	// Create environment config provider with empty configs (will be populated in Initialize)
	envConfigProvider := NewEnvironmentConfigProvider([]*unstructured.Unstructured{})

	// Create the composite provider with all our extra resource providers
	processor.extraResourceProvider = NewCompositeExtraResourceProvider(
		envConfigProvider,
		NewSelectorExtraResourceProvider(client),
		NewReferenceExtraResourceProvider(client),
		NewTemplatedExtraResourceProvider(client, config.RenderFunc, config.Logger),
	)

	return processor, nil
}

// Initialize loads required resources like CRDs and environment configs
func (p *DefaultDiffProcessor) Initialize(writer io.Writer, ctx context.Context) error {
	xrds, err := p.client.GetXRDs(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot get XRDs")
	}

	// Use the helper function to convert XRDs to CRDs
	crds, err := internal.ConvertToCRDs(xrds)
	if err != nil {
		return errors.Wrap(err, "cannot convert XRDs to CRDs")
	}

	p.crds = crds

	// Get and cache environment configs
	environmentConfigs, err := p.client.GetEnvironmentConfigs(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot get environment configs")
	}

	// Update the EnvironmentConfigProvider with the fetched configs
	// Find the EnvironmentConfigProvider in our composite provider
	if compositeProvider, ok := p.extraResourceProvider.(*CompositeExtraResourceProvider); ok {
		for _, provider := range compositeProvider.providers {
			if envProvider, ok := provider.(*EnvironmentConfigProvider); ok {
				envProvider.configs = environmentConfigs
				break
			}
		}
	}

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
	// Convert the unstructured resource to a composite unstructured for rendering
	xr := ucomposite.New()
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(res.UnstructuredContent(), xr); err != nil {
		return errors.Wrap(err, "cannot convert XR to composite unstructured")
	}

	// Find the matching composition
	comp, err := p.client.FindMatchingComposition(res)
	if err != nil {
		return errors.Wrap(err, "cannot find matching composition")
	}

	// Get functions for rendering
	fns, err := p.client.GetFunctionsFromPipeline(comp)
	if err != nil {
		return errors.Wrap(err, "cannot get functions from pipeline")
	}

	// Get all extra resources using our extraResourceProvider
	extraResources, err := p.extraResourceProvider.GetExtraResources(ctx, comp, res, []*unstructured.Unstructured{})
	if err != nil {
		return errors.Wrap(err, "cannot get extra resources")
	}

	// Convert the extra resources to the format expected by render.Inputs
	extraResourcesForRender := make([]unstructured.Unstructured, 0, len(extraResources))
	for _, er := range extraResources {
		extraResourcesForRender = append(extraResourcesForRender, *er)
	}

	// Render the resources
	desired, err := p.config.RenderFunc(ctx, p.config.Logger, render.Inputs{
		CompositeResource: xr,
		Composition:       comp,
		Functions:         fns,
		ExtraResources:    extraResourcesForRender,
	})
	if err != nil {
		return errors.Wrap(err, "cannot render resources")
	}

	// Helper function to print diffs
	printDiff := func(composite string, res runtime.Object) error {
		diff, err := p.CalculateDiff(ctx, composite, res)
		if err != nil {
			return errors.Wrap(err, "cannot calculate diff")
		}
		if diff != "" {
			_, _ = fmt.Fprintf(stdout, "%s\n---\n", diff)
		}

		return nil
	}

	compositeUnstructured := &unstructured.Unstructured{Object: desired.CompositeResource.UnstructuredContent()}

	// Merge the input XR with the rendered XR to get the full XR
	xrUnstructured, err := mergeUnstructured(compositeUnstructured, res)
	if err != nil {
		return errors.Wrap(err, "cannot merge input XR with result of rendered XR")
	}

	// Validate the resources
	if err := p.ValidateResources(stdout, xrUnstructured, desired.ComposedResources); err != nil {
		return errors.Wrap(err, "cannot validate resources")
	}

	// Print the diffs
	var errs []error
	errs = append(errs, printDiff("", xrUnstructured))

	// Diff the composed resources
	for _, d := range desired.ComposedResources {
		un := &unstructured.Unstructured{Object: d.UnstructuredContent()}
		errs = append(errs, printDiff(xr.GetName(), un))
	}
	return errors.Wrap(errors.Join(errs...), "cannot print diff")
}

// ValidateResources validates the resources using schema validation
func (p *DefaultDiffProcessor) ValidateResources(writer io.Writer, xr *unstructured.Unstructured, composed []composed.Unstructured) error {
	// Make sure we have CRDs before validation
	if len(p.crds) == 0 {
		return errors.New("no CRDs available for validation")
	}

	// Convert XR and composed resources to unstructured
	resources := make([]*unstructured.Unstructured, 0, len(composed)+1)

	// Add the XR to the validation list; we've already taken care of merging it together with desired
	resources = append(resources, xr)

	// Add composed resources to validation list
	for i := range composed {
		// Use the correct index (i) for accessing composed resources
		composedUnstr := &unstructured.Unstructured{Object: composed[i].UnstructuredContent()}
		resources = append(resources, composedUnstr)
	}

	loggerWriter := internal.NewLoggerWriter(p.config.Logger)

	// Validate using the converted CRD schema
	// Do not actually write to stdout (as it can interfere with diffs) -- just use the logger
	if err := validate.SchemaValidation(resources, p.crds, true, loggerWriter); err != nil {
		return errors.Wrap(err, "schema validation failed")
	}

	return nil
}

// CalculateDiff calculates the difference between desired state and current state
func (p *DefaultDiffProcessor) CalculateDiff(ctx context.Context, composite string, desired runtime.Object) (string, error) {
	// Convert desired to unstructured
	desiredUnstr, ok := desired.(*unstructured.Unstructured)
	if !ok {
		return "", errors.New("desired object is not unstructured")
	}

	// Fetch current object from cluster
	current, _, err := p.fetchCurrentObject(ctx, composite, desiredUnstr)
	if err != nil {
		return "", err
	}

	p.makeObjectValid(desiredUnstr)

	wouldBeResult := desiredUnstr
	if current != nil {
		// Perform a dry-run apply to get the result after we'd apply
		wouldBeResult, err = p.client.DryRunApply(ctx, desiredUnstr)
		if err != nil {
			return "", errors.Wrap(err, "cannot dry-run apply desired object")
		}
	}

	// Get diff options from the processor configuration
	diffOpts := p.config.GetDiffOptions()

	// Generate diff with the configured options
	diff, err := GenerateDiffWithOptions(current, wouldBeResult, desiredUnstr.GetKind(), desiredUnstr.GetName(), diffOpts)
	if err != nil {
		return "", errors.Wrap(err, "cannot generate diff")
	}

	return diff, nil
}

// makeObjectValid makes sure all OwnerReferences have a valid UID
func (p *DefaultDiffProcessor) makeObjectValid(obj *unstructured.Unstructured) {
	// Get the current owner references
	refs := obj.GetOwnerReferences()

	// Create new slice to hold the updated references
	updatedRefs := make([]metav1.OwnerReference, 0, len(refs))

	// Set a valid UID for each reference
	for _, ref := range refs {
		if ref.UID == "" {
			ref.UID = uuid.NewUUID()
		}
		updatedRefs = append(updatedRefs, ref)
	}

	// Update the object with the modified owner references
	obj.SetOwnerReferences(updatedRefs)
}

// fetchCurrentObject retrieves the current state of the object from the cluster
// It returns the current object, a boolean indicating if it's a new object, and any error
func (p *DefaultDiffProcessor) fetchCurrentObject(ctx context.Context, composite string, desired *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	// Get the GroupVersionKind and name/namespace for lookup
	gvk := desired.GroupVersionKind()
	name := desired.GetName()
	namespace := desired.GetNamespace()

	var current *unstructured.Unstructured
	var err error
	isNewObject := false

	if composite != "" {
		// For composed resources, use the label selector approach
		sel := metav1.LabelSelector{
			MatchLabels: map[string]string{
				"crossplane.io/composite": composite,
			},
		}
		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: fmt.Sprintf("%ss", strings.ToLower(gvk.Kind)), // naive pluralization
		}

		// Get the current object from the cluster using ClusterClient
		currents, err := p.client.GetResourcesByLabel(ctx, namespace, gvr, sel)
		if err != nil {
			return nil, false, errors.Wrap(err, "cannot get current object")
		}
		if len(currents) > 1 {
			return nil, false, errors.New(fmt.Sprintf("more than one matching resource found for %s/%s", gvk.Kind, name))
		}

		if len(currents) == 1 {
			current = currents[0]
		} else {
			isNewObject = true
		}
	} else {
		// For XRs, use direct lookup by name
		current, err = p.client.GetResource(ctx, gvk, namespace, name)
		if apierrors.IsNotFound(err) {
			isNewObject = true
		} else if err != nil {
			return nil, false, errors.Wrap(err, "cannot get current object")
		}
	}

	return current, isNewObject, nil
}

// mergeUnstructured merges two unstructured objects
func mergeUnstructured(dest *unstructured.Unstructured, src *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// Start with a deep copy of the rendered resource
	result := dest.DeepCopy()
	if err := mergo.Merge(&result.Object, src.Object, mergo.WithOverride); err != nil {
		return nil, errors.Wrap(err, "cannot merge unstructured objects")
	}

	return result, nil
}
