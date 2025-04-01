// Package clusterclient contains the base level client(s) responsible for interacting with the kubernetes cluster.
package clusterclient

import (
	"context"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/pkg"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal/resource"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal/resource/xrm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"sync"
)

// ClusterClient defines the interface for interacting with a Kubernetes cluster
// to retrieve Crossplane resources for diffing.
type ClusterClient interface {
	// Initialize sets up any required resources
	Initialize(ctx context.Context) error

	// FindMatchingComposition finds a composition that matches the given XR
	FindMatchingComposition(ctx context.Context, res *un.Unstructured) (*apiextensionsv1.Composition, error)

	// GetEnvironmentConfigs fetches environment configs from the cluster
	GetEnvironmentConfigs(ctx context.Context) ([]*un.Unstructured, error)

	// GetAllResourcesByLabels gets all resources matching the given GVK/selector pairs
	GetAllResourcesByLabels(ctx context.Context, gvks []schema.GroupVersionKind, selectors []metav1.LabelSelector) ([]*un.Unstructured, error)

	// GetFunctionsFromPipeline retrieves all functions used in the composition's pipeline
	GetFunctionsFromPipeline(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error)

	// GetXRDs retrieves the XRD schemas from the cluster
	GetXRDs(ctx context.Context) ([]*un.Unstructured, error)

	// GetResource retrieves a resource from the cluster based on its GVK, namespace, and name
	GetResource(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*un.Unstructured, error)

	// GetResourceTree retrieves the resource tree from the cluster
	GetResourceTree(ctx context.Context, root *un.Unstructured) (*resource.Resource, error)

	// GetResourcesByLabel looks up resources matching the given GVK and label selector
	GetResourcesByLabel(ctx context.Context, ns string, gvk schema.GroupVersionKind, sel metav1.LabelSelector) ([]*un.Unstructured, error)

	// DryRunApply performs a server-side apply with dry-run flag for diffing
	DryRunApply(ctx context.Context, obj *un.Unstructured) (*un.Unstructured, error)

	// GetCRD gets the CRD for a given GVK
	GetCRD(ctx context.Context, gvk schema.GroupVersionKind) (*un.Unstructured, error)

	// IsCRDRequired checks if a resource requires a CRD
	IsCRDRequired(ctx context.Context, gvk schema.GroupVersionKind) bool
}

// DefaultClusterClient handles all interactions with the Kubernetes cluster.
type DefaultClusterClient struct {
	dynamicClient   dynamic.Interface
	xrmClient       *xrm.Client
	discoveryClient discovery.DiscoveryInterface
	compositions    map[string]*apiextensionsv1.Composition
	functions       map[string]pkgv1.Function
	logger          logging.Logger

	// Resource caching
	resourceMap map[schema.GroupVersionKind]bool

	// GVK caching
	gvkToGVRMap   map[schema.GroupVersionKind]schema.GroupVersionResource
	gvkToGVRMutex sync.RWMutex

	// XRD caching
	xrds       []*un.Unstructured
	xrdsMutex  sync.RWMutex
	xrdsLoaded bool
}

// NewClusterClient creates a new DefaultClusterClient instance.
func NewClusterClient(config *rest.Config, opts ...Option) (ClusterClient, error) {
	// Set up default configuration
	options := &Options{
		Logger: logging.NewNopLogger(),
	}

	// Apply all provided options
	for _, opt := range opts {
		opt(options)
	}

	// Set default QPS and Burst if they are not set in the config
	// or override with values from options if provided
	originalQPS := config.QPS
	originalBurst := config.Burst

	if options.QPS > 0 {
		config.QPS = options.QPS
	} else if config.QPS == 0 {
		config.QPS = 20
	}

	if options.Burst > 0 {
		config.Burst = options.Burst
	} else if config.Burst == 0 {
		config.Burst = 30
	}

	options.Logger.Debug("Configured REST client rate limits",
		"original_qps", originalQPS,
		"original_burst", originalBurst,
		"options_qps", options.QPS,
		"options_burst", options.Burst,
		"final_qps", config.QPS,
		"final_burst", config.Burst)

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create dynamic client")
	}

	c, err := client.New(config, client.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		return nil, errors.Wrap(err, "cannot create static client")
	}

	_ = pkg.AddToScheme(c.Scheme())

	// Create an XRM client to get the resource tree
	xrmClient, err := xrm.NewClient(c,
		xrm.WithConnectionSecrets(false),
		xrm.WithConcurrency(5))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create resource tree client")
	}

	// Create discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create discovery client")
	}

	cc := DefaultClusterClient{
		dynamicClient:   dynamicClient,
		xrmClient:       xrmClient,
		logger:          options.Logger,
		discoveryClient: discoveryClient,
		resourceMap:     make(map[schema.GroupVersionKind]bool),
		gvkToGVRMap:     make(map[schema.GroupVersionKind]schema.GroupVersionResource),
	}

	return &cc, nil
}

// Initialize loads compositions and functions from the cluster.
func (c *DefaultClusterClient) Initialize(ctx context.Context) error {
	c.logger.Debug("Initializing cluster client")

	// Fetch compositions
	compositions, err := c.listCompositions(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot list compositions")
	}

	// Fetch functions
	functions, err := c.listFunctions(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot list functions")
	}

	// Initialize and populate maps
	c.compositions = make(map[string]*apiextensionsv1.Composition, len(compositions))
	c.functions = make(map[string]pkgv1.Function, len(functions))

	// Process compositions
	for i := range compositions {
		comp := compositions[i]
		c.compositions[comp.GetName()] = &comp
	}

	// Process functions
	for i := range functions {
		c.functions[functions[i].GetName()] = functions[i]
	}

	// Preload XRDs to populate the cache
	_, err = c.GetXRDs(ctx)
	if err != nil {
		c.logger.Debug("Failed to preload XRDs",
			"error", err)
		return errors.Wrap(err, "Failed to preload XRDs")
	}

	c.logger.Debug("Cluster client initialization complete",
		"compositions_count", len(c.compositions),
		"functions_count", len(c.functions))
	return nil
}

// GetAllResourcesByLabels fetches all resources from the cluster based on the provided GVKs and selectors
func (c *DefaultClusterClient) GetAllResourcesByLabels(ctx context.Context, gvks []schema.GroupVersionKind, selectors []metav1.LabelSelector) ([]*un.Unstructured, error) {
	if len(gvks) != len(selectors) {
		c.logger.Debug("GVKs and selectors count mismatch",
			"gvks_count", len(gvks),
			"selectors_count", len(selectors))
		return nil, errors.New("number of GVKs must match number of selectors")
	}

	c.logger.Debug("Fetching resources by labels",
		"gvks_count", len(gvks))

	var resources []*un.Unstructured

	for i, gvk := range gvks {
		// List resources matching the selector
		sel := selectors[i]
		c.logger.Debug("Getting resources for GVK with selector",
			"gvk", gvk.String(),
			"selector", sel.MatchLabels)

		res, err := c.GetResourcesByLabel(ctx, "", gvk, sel)
		if err != nil {
			c.logger.Debug("Failed to get resources by label",
				"gvk", gvk.String(),
				"error", err)
			return nil, errors.Wrapf(err, "cannot get all resources")
		}

		c.logger.Debug("Found resources for GVK",
			"gvk", gvk.String(),
			"count", len(res))
		resources = append(resources, res...)
	}

	c.logger.Debug("Completed fetching resources by labels",
		"total_resources", len(resources))
	return resources, nil
}

// GetResourcesByLabel retrieves all resources from the cluster based on the provided GVK and selector
func (c *DefaultClusterClient) GetResourcesByLabel(ctx context.Context, ns string, gvk schema.GroupVersionKind, sel metav1.LabelSelector) ([]*un.Unstructured, error) {
	c.logger.Debug("Getting resources by label",
		"namespace", ns,
		"gvk", gvk.String(),
		"selector", sel.MatchLabels)

	// Convert GVK to GVR - now with proper error handling
	gvr, err := c.gvkToGVR(ctx, gvk)
	if err != nil {
		c.logger.Debug("Failed to convert GVK to GVR",
			"gvk", gvk.String(),
			"error", err)
		return nil, errors.Wrapf(err, "cannot list resources for '%s' matching '%s'",
			gvk.String(), labels.Set(sel.MatchLabels).String())
	}

	opts := metav1.ListOptions{}
	if len(sel.MatchLabels) > 0 {
		opts.LabelSelector = labels.Set(sel.MatchLabels).String()
	}

	c.logger.Debug("Listing resources",
		"labelSelector", opts.LabelSelector)
	list, err := c.dynamicClient.Resource(gvr).Namespace(ns).List(ctx, opts)
	if err != nil {
		c.logger.Debug("Failed to list resources",
			"gvk", gvk.String(),
			"labelSelector", opts.LabelSelector,
			"error", err)
		return nil, errors.Wrapf(err, "cannot list resources for '%s' matching '%s'",
			gvk.String(), opts.LabelSelector)
	}

	resources := make([]*un.Unstructured, 0, len(list.Items))
	for _, item := range list.Items {
		// Create a pointer to each item
		resources = append(resources, &item)
	}

	c.logger.Debug("Resources found by label",
		"count", len(resources),
		"gvk", gvk.String())
	return resources, nil
}

// GetEnvironmentConfigs fetches environment configs from the cluster.
func (c *DefaultClusterClient) GetEnvironmentConfigs(ctx context.Context) ([]*un.Unstructured, error) {
	c.logger.Debug("Getting environment configs")

	envConfigsGVR := schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1alpha1",
		Resource: "environmentconfigs",
	}

	// we have the EnvironmentConfig type in the same package, so we could use it here, but
	// that might be troublesome for adding it to the un ExtraResources list
	envConfigsClient := c.dynamicClient.Resource(envConfigsGVR)

	c.logger.Debug("Listing environment configs")
	list, err := envConfigsClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Debug("Failed to list environment configs", "error", err)
		return nil, errors.Wrap(err, "cannot list environment configs")
	}

	envConfigs := make([]*un.Unstructured, len(list.Items))
	for i := range list.Items {
		envConfigs[i] = &list.Items[i]
	}

	c.logger.Debug("Environment configs retrieved", "count", len(envConfigs))
	return envConfigs, nil
}

// FindMatchingComposition finds a composition matching the given resource.
// It handles both XRs and Claims, finding the appropriate composition in each case.
func (c *DefaultClusterClient) FindMatchingComposition(ctx context.Context, res *un.Unstructured) (*apiextensionsv1.Composition, error) {
	// Determine if we're dealing with a claim or an XR
	gvk := res.GroupVersionKind()
	resourceID := fmt.Sprintf("%s/%s", gvk.String(), res.GetName())

	c.logger.Debug("Finding matching composition",
		"resource_name", res.GetName(),
		"gvk", gvk.String())

	// First, check if this is a claim by looking for an XRD that defines this as a claim
	xrdForClaim, err := c.findClaimXRD(ctx, gvk)
	if err != nil {
		c.logger.Debug("Error checking if resource is claim type",
			"resource", resourceID,
			"error", err)
		// Continue as if not a claim - we'll try normal composition matching
	}

	// If it's a claim, we need to find compositions for the corresponding XR type
	var targetGVK schema.GroupVersionKind
	if xrdForClaim != nil {
		targetGVK, err = c.getXRTypeFromXRD(xrdForClaim, resourceID)
		if err != nil {
			return nil, errors.Wrapf(err, "claim %s requires its XR type to find a composition", resourceID)
		}
	} else {
		// Not a claim or couldn't determine XRD - use the actual resource GVK
		targetGVK = gvk
	}

	// Case 1: Check for direct composition reference in spec.compositionRef.name
	comp, err := c.findByDirectReference(res, targetGVK, resourceID)
	if err != nil || comp != nil {
		return comp, err
	}

	// Case 2: Check for selector-based composition reference in spec.compositionSelector.matchLabels
	comp, err = c.findByLabelSelector(res, targetGVK, resourceID)
	if err != nil || comp != nil {
		return comp, err
	}

	// Case 3: Look up by composite type reference (original behavior)
	return c.findByTypeReference(targetGVK, resourceID)
}

// getXRTypeFromXRD extracts the XR GroupVersionKind from an XRD
func (c *DefaultClusterClient) getXRTypeFromXRD(xrdForClaim *un.Unstructured, resourceID string) (schema.GroupVersionKind, error) {
	// Get the XR type from the XRD
	xrGroup, found, _ := un.NestedString(xrdForClaim.Object, "spec", "group")
	xrKind, kindFound, _ := un.NestedString(xrdForClaim.Object, "spec", "names", "kind")

	if !found || !kindFound {
		return schema.GroupVersionKind{}, errors.New("could not determine group or kind from XRD")
	}

	// Find the referenceable version - there should be exactly one
	xrVersion := ""
	versions, versionsFound, _ := un.NestedSlice(xrdForClaim.Object, "spec", "versions")
	if versionsFound && len(versions) > 0 {
		// Look for the one version that is marked referenceable
		for _, versionObj := range versions {
			if version, ok := versionObj.(map[string]interface{}); ok {
				ref, refFound, _ := un.NestedBool(version, "referenceable")
				if refFound && ref {
					name, nameFound, _ := un.NestedString(version, "name")
					if nameFound {
						xrVersion = name
						break
					}
				}
			}
		}
	}

	// If no referenceable version found, we shouldn't guess
	if xrVersion == "" {
		return schema.GroupVersionKind{}, errors.New("no referenceable version found in XRD")
	}

	targetGVK := schema.GroupVersionKind{
		Group:   xrGroup,
		Version: xrVersion,
		Kind:    xrKind,
	}

	c.logger.Debug("Claim resource detected - targeting XR type for composition matching",
		"claim", resourceID,
		"targetXR", targetGVK.String())

	return targetGVK, nil
}

// findByDirectReference attempts to find a composition directly referenced by name
func (c *DefaultClusterClient) findByDirectReference(res *un.Unstructured, targetGVK schema.GroupVersionKind, resourceID string) (*apiextensionsv1.Composition, error) {
	compositionRefName, compositionRefFound, err := un.NestedString(res.Object, "spec", "compositionRef", "name")
	if err == nil && compositionRefFound && compositionRefName != "" {
		c.logger.Debug("Found direct composition reference",
			"resource", resourceID,
			"compositionName", compositionRefName)

		// Look up composition by name
		if comp, ok := c.compositions[compositionRefName]; ok {
			// Validate that the composition's compositeTypeRef matches the target GVK
			if !isCompositionCompatible(comp, targetGVK) {
				return nil, errors.Errorf("composition %s is not compatible with %s",
					compositionRefName, targetGVK.String())
			}

			c.logger.Debug("Found composition by direct reference",
				"resource", resourceID,
				"composition", comp.GetName())
			return comp, nil
		}

		// If we got here, the named composition wasn't found
		return nil, errors.Errorf("composition %s referenced in %s not found",
			compositionRefName, resourceID)
	}

	return nil, nil // No direct reference found
}

// findByLabelSelector attempts to find compositions that match label selectors
func (c *DefaultClusterClient) findByLabelSelector(res *un.Unstructured, targetGVK schema.GroupVersionKind, resourceID string) (*apiextensionsv1.Composition, error) {
	matchLabels, selectorFound, err := un.NestedMap(res.Object, "spec", "compositionSelector", "matchLabels")
	if err == nil && selectorFound && len(matchLabels) > 0 {
		c.logger.Debug("Found composition selector",
			"resource", resourceID,
			"matchLabels", matchLabels)

		// Convert matchLabels to string map for comparison
		stringLabels := make(map[string]string)
		for k, v := range matchLabels {
			if strVal, ok := v.(string); ok {
				stringLabels[k] = strVal
			}
		}

		// Find compositions with matching labels
		var matchingCompositions []*apiextensionsv1.Composition

		// Search through all compositions looking for compatible ones with matching labels
		for _, comp := range c.compositions {
			// Check if this composition is for the right XR type
			if isCompositionCompatible(comp, targetGVK) {
				// Check if labels match
				if labelsMatch(comp.GetLabels(), stringLabels) {
					matchingCompositions = append(matchingCompositions, comp)
				}
			}
		}

		// Handle matching results
		switch len(matchingCompositions) {
		case 0:
			return nil, errors.Errorf("no compatible composition found matching labels %v for %s",
				stringLabels, resourceID)
		case 1:
			c.logger.Debug("Found composition by label selector",
				"resource", resourceID,
				"composition", matchingCompositions[0].GetName())
			return matchingCompositions[0], nil
		default:
			// Multiple matches - this is ambiguous and should fail
			names := make([]string, len(matchingCompositions))
			for i, comp := range matchingCompositions {
				names[i] = comp.GetName()
			}
			return nil, errors.New("ambiguous composition selection: multiple compositions match")
		}
	}

	return nil, nil // No label selector found or no matches
}

// findByTypeReference attempts to find a composition by matching the type reference
func (c *DefaultClusterClient) findByTypeReference(targetGVK schema.GroupVersionKind, resourceID string) (*apiextensionsv1.Composition, error) {
	// We need to get all compositions that match this target type
	var compatibleCompositions []*apiextensionsv1.Composition

	for _, comp := range c.compositions {
		if comp.Spec.CompositeTypeRef.APIVersion == targetGVK.GroupVersion().String() &&
			comp.Spec.CompositeTypeRef.Kind == targetGVK.Kind {
			compatibleCompositions = append(compatibleCompositions, comp)
		}
	}

	if len(compatibleCompositions) == 0 {
		c.logger.Debug("No matching composition found",
			"targetGVK", targetGVK.String())
		return nil, errors.Errorf("no composition found for %s", targetGVK.String())
	}

	if len(compatibleCompositions) > 1 {
		// Multiple compositions match, but no selection criteria was provided
		// This is an ambiguous situation
		names := make([]string, len(compatibleCompositions))
		for i, comp := range compatibleCompositions {
			names[i] = comp.GetName()
		}
		return nil, errors.Errorf("ambiguous composition selection: multiple compositions exist for %s", targetGVK.String())
	}

	// We have exactly one matching composition
	c.logger.Debug("Found matching composition by type reference",
		"resource_name", resourceID,
		"composition_name", compatibleCompositions[0].GetName())
	return compatibleCompositions[0], nil
}

// findClaimXRD checks if the given GVK is a claim type and returns the corresponding XRD if found
func (c *DefaultClusterClient) findClaimXRD(ctx context.Context, gvk schema.GroupVersionKind) (*un.Unstructured, error) {
	c.logger.Debug("Checking if resource is a claim type",
		"gvk", gvk.String())

	// List all XRDs
	xrds, err := c.GetXRDs(ctx)
	if err != nil {
		c.logger.Debug("Error getting XRDs",
			"error", err)
		return nil, errors.Wrap(err, "cannot get XRDs")
	}

	// Loop through XRDs to find one that defines this GVK as a claim
	for _, xrd := range xrds {
		claimGroup, found, _ := un.NestedString(xrd.Object, "spec", "group")

		// Skip if group doesn't match
		if !found || claimGroup != gvk.Group {
			continue
		}

		// Check claim kind
		claimNames, found, _ := un.NestedMap(xrd.Object, "spec", "claimNames")
		if !found || claimNames == nil {
			continue
		}

		claimKind, found, _ := un.NestedString(claimNames, "kind")
		if !found || claimKind != gvk.Kind {
			continue
		}

		c.logger.Debug("Found matching XRD for claim type",
			"gvk", gvk.String(),
			"xrd", xrd.GetName())

		return xrd, nil
	}

	// No matching XRD found - not a claim type
	return nil, nil
}

// Helper function to check if a composition is compatible with an XR's GVK
func isCompositionCompatible(comp *apiextensionsv1.Composition, xrGVK schema.GroupVersionKind) bool {
	return comp.Spec.CompositeTypeRef.APIVersion == xrGVK.GroupVersion().String() &&
		comp.Spec.CompositeTypeRef.Kind == xrGVK.Kind
}

// Helper function to check if labels match the selector
func labelsMatch(labels, selector map[string]string) bool {
	// A resource matches a selector if all the selector's labels exist in the resource's labels
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// GetFunctionsFromPipeline returns functions referenced in the composition pipeline.
func (c *DefaultClusterClient) GetFunctionsFromPipeline(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
	c.logger.Debug("Getting functions from pipeline", "composition_name", comp.GetName())

	if comp.Spec.Mode == nil || *comp.Spec.Mode != apiextensionsv1.CompositionModePipeline {
		c.logger.Debug("Composition is not in pipeline mode",
			"composition_name", comp.GetName(),
			"mode", func() string {
				if comp.Spec.Mode == nil {
					return "nil"
				}
				return string(*comp.Spec.Mode)
			}())
		if comp.Spec.Mode != nil {
			return nil, fmt.Errorf("unsupported composition Mode '%s'; supported types are [%s]", *comp.Spec.Mode, apiextensionsv1.CompositionModePipeline)
		}
		return nil, errors.New("unsupported Composition; no Mode found")
	}

	functions := make([]pkgv1.Function, 0, len(comp.Spec.Pipeline))
	c.logger.Debug("Processing pipeline steps", "steps_count", len(comp.Spec.Pipeline))

	for _, step := range comp.Spec.Pipeline {
		fn, ok := c.functions[step.FunctionRef.Name]
		if !ok {
			c.logger.Debug("Function not found",
				"step", step.Step,
				"function_name", step.FunctionRef.Name)
			return nil, errors.Errorf("function %q referenced in pipeline step %q not found", step.FunctionRef.Name, step.Step)
		}
		c.logger.Debug("Found function for step",
			"step", step.Step,
			"function_name", fn.GetName())
		functions = append(functions, fn)
	}

	c.logger.Debug("Retrieved functions from pipeline",
		"functions_count", len(functions),
		"composition_name", comp.GetName())
	return functions, nil
}

func (c *DefaultClusterClient) listCompositions(ctx context.Context) ([]apiextensionsv1.Composition, error) {
	c.logger.Debug("Listing compositions from cluster")

	compositionsGVR := schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1",
		Resource: "compositions",
	}
	compositionsClient := c.dynamicClient.Resource(compositionsGVR)

	c.logger.Debug("Fetching compositions using dynamic client")
	list, err := compositionsClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Debug("Failed to list compositions", "error", err)
		return nil, errors.Wrap(err, "cannot list compositions from cluster")
	}

	compositions := make([]apiextensionsv1.Composition, 0, len(list.Items))
	c.logger.Debug("Converting compositions from un", "count", len(list.Items))

	for _, obj := range list.Items {
		comp := &apiextensionsv1.Composition{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, comp); err != nil {
			c.logger.Debug("Failed to convert composition from un",
				"name", obj.GetName(),
				"error", err)
			return nil, errors.Wrap(err, "cannot convert unstructured to Composition")
		}
		compositions = append(compositions, *comp)
	}

	c.logger.Debug("Successfully retrieved compositions", "count", len(compositions))
	return compositions, nil
}

func (c *DefaultClusterClient) listFunctions(ctx context.Context) ([]pkgv1.Function, error) {
	c.logger.Debug("Listing functions from cluster")

	functionsGVR := schema.GroupVersionResource{
		Group:    "pkg.crossplane.io",
		Version:  "v1",
		Resource: "functions",
	}
	functionsClient := c.dynamicClient.Resource(functionsGVR)

	c.logger.Debug("Fetching functions using dynamic client")
	list, err := functionsClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Debug("Failed to list functions", "error", err)
		return nil, errors.Wrap(err, "cannot list functions from cluster")
	}

	functions := make([]pkgv1.Function, 0, len(list.Items))
	c.logger.Debug("Converting functions from un", "count", len(list.Items))

	for _, obj := range list.Items {
		fn := &pkgv1.Function{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, fn); err != nil {
			c.logger.Debug("Failed to convert function from un",
				"name", obj.GetName(),
				"error", err)
			return nil, errors.Wrap(err, "cannot convert unstructured to Function")
		}
		functions = append(functions, *fn)
	}

	c.logger.Debug("Successfully retrieved functions", "count", len(functions))
	return functions, nil
}

// GetXRDs returns the set of all XRDs in the cluster and caches them.
func (c *DefaultClusterClient) GetXRDs(ctx context.Context) ([]*un.Unstructured, error) {
	// Check if XRDs are already loaded
	c.xrdsMutex.RLock()
	if c.xrdsLoaded {
		xrds := c.xrds
		c.xrdsMutex.RUnlock()
		c.logger.Debug("Using cached XRDs", "count", len(xrds))
		return xrds, nil
	}
	c.xrdsMutex.RUnlock()

	// Need to load XRDs
	c.xrdsMutex.Lock()
	defer c.xrdsMutex.Unlock()

	// Double-check now that we have the write lock
	if c.xrdsLoaded {
		c.logger.Debug("Using cached XRDs (after recheck)", "count", len(c.xrds))
		return c.xrds, nil
	}

	c.logger.Debug("Fetching XRDs from cluster")

	// Create a dynamic resource interface for XRDs
	xrdsGVR := schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1",
		Resource: "compositeresourcedefinitions",
	}
	xrdsClient := c.dynamicClient.Resource(xrdsGVR)

	// List all XRDs since we need to find one matching the XR's group/kind
	c.logger.Debug("Listing XRDs using dynamic client")
	list, err := xrdsClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Debug("Failed to list XRDs", "error", err)
		return nil, errors.Wrap(err, "cannot list XRDs")
	}

	items := list.Items
	result := make([]*un.Unstructured, len(items))

	c.logger.Debug("Processing XRDs", "count", len(items))
	for i := range items {
		// Create a pointer to each item
		result[i] = &items[i]
	}

	// Cache the result
	c.xrds = result
	c.xrdsLoaded = true

	c.logger.Debug("Successfully retrieved and cached XRDs", "count", len(result))
	return result, nil
}

// GetResource retrieves a resource from the cluster based on its GVK, namespace, and name
func (c *DefaultClusterClient) GetResource(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*un.Unstructured, error) {
	resourceID := fmt.Sprintf("%s/%s/%s", gvk.String(), namespace, name)
	c.logger.Debug("Getting resource from cluster", "resource", resourceID)

	// Convert GVK to GVR with error handling
	gvr, err := c.gvkToGVR(ctx, gvk)
	if err != nil {
		c.logger.Debug("Failed to convert GVK to GVR",
			"gvk", gvk.String(),
			"error", err)
		return nil, errors.Wrapf(err, "cannot get resource %s/%s of kind %s", namespace, name, gvk.Kind)
	}

	// Get the resource
	var res *un.Unstructured

	// If namespace is empty string, it will be ignored
	res, err = c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})

	if err != nil {
		c.logger.Debug("Failed to get resource",
			"resource", resourceID,
			"gvr", gvr.String(),
			"error", err)
		return nil, errors.Wrapf(err, "cannot get resource %s/%s of kind %s", namespace, name, gvk.Kind)
	}

	c.logger.Debug("Retrieved resource",
		"resource", resourceID,
		"uid", res.GetUID(),
		"resourceVersion", res.GetResourceVersion())
	return res, nil
}

// GetResourceTree returns the tree of children beneath a given root.
func (c *DefaultClusterClient) GetResourceTree(ctx context.Context, root *un.Unstructured) (*resource.Resource, error) {
	c.logger.Debug("Getting resource tree",
		"resource_kind", root.GetKind(),
		"resource_name", root.GetName(),
		"resource_uid", root.GetUID())

	// Convert to resource.Resource for the XRM client
	res := &resource.Resource{
		Unstructured: *root,
	}

	tree, err := c.xrmClient.GetResourceTree(ctx, res)
	if err != nil {
		c.logger.Debug("Failed to get resource tree",
			"resource_kind", root.GetKind(),
			"resource_name", root.GetName(),
			"error", err)
		return nil, errors.Wrap(err, "failed to get resource tree")
	}

	// Count children for logging
	childCount := len(tree.Children)
	c.logger.Debug("Retrieved resource tree",
		"resource_kind", root.GetKind(),
		"resource_name", root.GetName(),
		"child_count", childCount)

	return tree, nil
}

// DryRunApply performs a server-side apply with dry-run flag for diffing
func (c *DefaultClusterClient) DryRunApply(ctx context.Context, obj *un.Unstructured) (*un.Unstructured, error) {
	resourceID := fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName())
	c.logger.Debug("Performing dry-run apply", "resource", resourceID)

	// Get the GVK from the object
	gvk := obj.GroupVersionKind()

	// Convert GVK to GVR with error handling
	gvr, err := c.gvkToGVR(ctx, gvk)
	if err != nil {
		c.logger.Debug("Failed to convert GVK to GVR",
			"gvk", gvk.String(),
			"error", err)
		return nil, errors.Wrapf(err, "cannot perform dry-run apply for %s", resourceID)
	}

	// Get the resource client for the namespace
	resourceClient := c.dynamicClient.Resource(gvr).Namespace(obj.GetNamespace())

	// Create apply options for a dry run
	applyOptions := metav1.ApplyOptions{
		FieldManager: "crossplane-diff",
		Force:        true,
		DryRun:       []string{metav1.DryRunAll},
	}

	// Perform a dry-run server-side apply
	result, err := resourceClient.Apply(ctx, obj.GetName(), obj, applyOptions)
	if err != nil {
		c.logger.Debug("Dry-run apply failed",
			"resource", resourceID,
			"error", err)
		return nil, errors.Wrapf(err, "failed to apply resource %s/%s: %v",
			obj.GetNamespace(), obj.GetName(), err)
	}

	c.logger.Debug("Dry-run apply successful",
		"resource", resourceID,
		"resourceVersion", result.GetResourceVersion())
	return result, nil
}

// IsCRDRequired checks if a resource requires a CRD
func (c *DefaultClusterClient) IsCRDRequired(ctx context.Context, gvk schema.GroupVersionKind) bool {
	// Core API resources never need CRDs
	if gvk.Group == "" {
		return false
	}

	// Standard Kubernetes API groups
	builtInGroups := []string{
		"apps", "batch", "extensions", "policy", "autoscaling",
	}
	for _, group := range builtInGroups {
		if gvk.Group == group {
			return false
		}
	}

	// k8s.io domain suffix groups are typically built-in
	// (except apiextensions.k8s.io which defines CRDs themselves)
	if strings.HasSuffix(gvk.Group, ".k8s.io") &&
		gvk.Group != "apiextensions.k8s.io" {
		return false
	}

	// If we get here, assume it requires a CRD unless we can prove otherwise
	// Try to query the discovery API to see if this resource exists
	_, err := c.getResourceForGVK(ctx, gvk)
	if err != nil {
		// If we couldn't find it through discovery, it likely requires a CRD
		c.logger.Debug("Resource not found in discovery, assuming CRD is required",
			"gvk", gvk.String(),
			"error", err)
		return true
	}

	// We found the resource, but it's not in the built-in list we checked above
	// This is most likely a CRD
	return true
}

// GetCRD retrieves the CustomResourceDefinition for a given GVK
func (c *DefaultClusterClient) GetCRD(ctx context.Context, gvk schema.GroupVersionKind) (*un.Unstructured, error) {
	// Get the resource name using the central method
	resourceName, err := c.getResourceForGVK(ctx, gvk)
	if err != nil {
		c.logger.Debug("Failed to get resource name for GVK",
			"gvk", gvk.String(),
			"error", err)
		return nil, errors.Wrapf(err, "cannot determine resource name for %s", gvk.String())
	}

	// Construct the CRD name using the resource name and group
	crdName := fmt.Sprintf("%s.%s", resourceName, gvk.Group)

	c.logger.Debug("Looking up CRD",
		"gvk", gvk.String(),
		"crdName", crdName)

	// Define the CRD GVR directly to avoid recursion
	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	// Fetch the CRD
	crd, err := c.dynamicClient.Resource(crdGVR).Get(ctx, crdName, metav1.GetOptions{})
	if err != nil {
		c.logger.Debug("Failed to get CRD",
			"gvk", gvk.String(),
			"crdName", crdName,
			"error", err)
		return nil, errors.Wrapf(err, "cannot get CRD %s for %s", crdName, gvk.String())
	}

	c.logger.Debug("Successfully retrieved CRD",
		"gvk", gvk.String(),
		"crdName", crdName)
	return crd, nil
}

// gvkToGVR converts a GroupVersionKind to a GroupVersionResource
// using the discovery client, returning an error if the mapping cannot be determined.
func (c *DefaultClusterClient) gvkToGVR(ctx context.Context, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	// Use the cached mapping if we have it
	c.gvkToGVRMutex.RLock()
	if gvr, ok := c.gvkToGVRMap[gvk]; ok {
		c.gvkToGVRMutex.RUnlock()
		return gvr, nil
	}
	c.gvkToGVRMutex.RUnlock()

	// Get the resource name using the central method
	resourceName, err := c.getResourceForGVK(ctx, gvk)
	if err != nil {
		c.logger.Debug("Failed to get resource for GVK",
			"gvk", gvk.String(),
			"error", err)
		return schema.GroupVersionResource{}, err
	}

	// Create the GVR
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: resourceName,
	}

	// Cache this mapping for future use
	c.gvkToGVRMutex.Lock()
	if c.gvkToGVRMap == nil {
		c.gvkToGVRMap = make(map[schema.GroupVersionKind]schema.GroupVersionResource)
	}
	c.gvkToGVRMap[gvk] = gvr
	c.gvkToGVRMutex.Unlock()

	return gvr, nil
}

// getResourceForGVK returns the resource name for a given GroupVersionKind using the discovery client.
// It returns an error if the resource cannot be determined or if the discovery client fails.
func (c *DefaultClusterClient) getResourceForGVK(_ context.Context, gvk schema.GroupVersionKind) (string, error) {

	// Get resources for the specified group version
	resources, err := c.discoveryClient.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		return "", errors.Wrapf(err, "failed to discover resources for %s", gvk.GroupVersion().String())
	}

	if resources == nil || len(resources.APIResources) == 0 {
		return "", errors.Errorf("no resources found for group version %s", gvk.GroupVersion().String())
	}

	// Find the API resource that matches our kind
	for _, r := range resources.APIResources {
		if r.Kind == gvk.Kind {
			return r.Name, nil
		}
	}

	// If we get here, we couldn't find a matching resource kind
	return "", errors.Errorf("no resource found for kind %s in group version %s",
		gvk.Kind, gvk.GroupVersion().String())
}

// Options holds configuration options for the cluster client
type Options struct {
	// Logger is the logger to use
	Logger logging.Logger

	// QPS indicates the maximum queries per second to the API server
	QPS float32

	// Burst indicates the maximum burst size for throttle
	Burst int
}

// Option defines a function that can modify Options
type Option func(*Options)

// WithLogger sets the logger for the cluster client
func WithLogger(logger logging.Logger) Option {
	return func(o *Options) {
		o.Logger = logger
	}
}

// WithQPS sets the QPS for the client
func WithQPS(qps float32) Option {
	return func(o *Options) {
		o.QPS = qps
	}
}

// WithBurst sets the Burst for the client
func WithBurst(burst int) Option {
	return func(o *Options) {
		o.Burst = burst
	}
}
