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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

type compositionCacheKey struct {
	apiVersion string
	kind       string
}

// ClusterClient defines the interface for interacting with a Kubernetes cluster
// to retrieve Crossplane resources for diffing.
type ClusterClient interface {
	// Initialize sets up any required resources
	Initialize(ctx context.Context) error

	// FindMatchingComposition finds a composition that matches the given XR
	FindMatchingComposition(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error)

	// GetEnvironmentConfigs fetches environment configs from the cluster
	GetEnvironmentConfigs(ctx context.Context) ([]*unstructured.Unstructured, error)

	// GetAllResourcesByLabels retrieves all resources matching the given GVRs and selectors
	GetAllResourcesByLabels(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]*unstructured.Unstructured, error)

	// GetFunctionsFromPipeline retrieves all functions used in the composition's pipeline
	GetFunctionsFromPipeline(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error)

	// GetXRDs retrieves the XRD schemas from the cluster
	GetXRDs(ctx context.Context) ([]*unstructured.Unstructured, error)

	// GetResource retrieves a resource from the cluster based on its GVK, namespace, and name
	GetResource(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error)

	// GetResourceTree retrieves the resource tree from the cluster
	GetResourceTree(ctx context.Context, root *unstructured.Unstructured) (*resource.Resource, error)

	// GetResourcesByLabel retrieves all resources from the cluster based on the provided GVR and selector
	GetResourcesByLabel(ctx context.Context, ns string, gvr schema.GroupVersionResource, sel metav1.LabelSelector) ([]*unstructured.Unstructured, error)

	// DryRunApply performs a server-side apply with dry-run flag for diffing
	DryRunApply(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)

	// IsCRDRequired checks if a resource requires a CRD
	IsCRDRequired(ctx context.Context, gvk schema.GroupVersionKind) bool
}

// DefaultClusterClient handles all interactions with the Kubernetes cluster.
type DefaultClusterClient struct {
	dynamicClient          dynamic.Interface
	xrmClient              *xrm.Client
	discoveryClient        discovery.DiscoveryInterface
	compositions           map[compositionCacheKey]*apiextensionsv1.Composition
	functions              map[string]pkgv1.Function
	logger                 logging.Logger
	resourceMap            map[schema.GroupVersionKind]bool
	resourceMapMutex       sync.RWMutex
	resourceMapInitialized bool
}

// NewClusterClient creates a new DefaultClusterClient instance.
func NewClusterClient(config *rest.Config, opts ...ClusterClientOption) (*DefaultClusterClient, error) {
	// Set up default configuration
	options := &ClusterClientOptions{
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

	return &DefaultClusterClient{
		dynamicClient:   dynamicClient,
		xrmClient:       xrmClient,
		logger:          options.Logger,
		discoveryClient: discoveryClient,
		resourceMap:     make(map[schema.GroupVersionKind]bool),
	}, nil
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
	c.compositions = make(map[compositionCacheKey]*apiextensionsv1.Composition, len(compositions))
	c.functions = make(map[string]pkgv1.Function, len(functions))

	// Process compositions
	for i := range compositions {
		key := compositionCacheKey{
			apiVersion: compositions[i].Spec.CompositeTypeRef.APIVersion,
			kind:       compositions[i].Spec.CompositeTypeRef.Kind,
		}
		c.compositions[key] = &compositions[i]
	}

	// Process functions
	for i := range functions {
		c.functions[functions[i].GetName()] = functions[i]
	}

	c.logger.Debug("Cluster client initialization complete",
		"compositions_count", len(c.compositions),
		"functions_count", len(c.functions))
	return nil
}

// GetAllResourcesByLabels fetches all resources from the cluster based on the provided GVRs and selectors
func (c *DefaultClusterClient) GetAllResourcesByLabels(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
	if len(gvrs) != len(selectors) {
		c.logger.Debug("GVRs and selectors count mismatch",
			"gvrs_count", len(gvrs),
			"selectors_count", len(selectors))
		return nil, errors.New("number of GVRs must match number of selectors")
	}

	c.logger.Debug("Fetching resources by labels",
		"gvrs_count", len(gvrs))

	var resources []*unstructured.Unstructured

	for i, gvr := range gvrs {
		// List resources matching the selector
		sel := selectors[i]
		c.logger.Debug("Getting resources for GVR with selector",
			"gvr", gvr.String(),
			"selector", sel.MatchLabels)

		res, err := c.GetResourcesByLabel(ctx, "", gvr, sel)
		if err != nil {
			c.logger.Debug("Failed to get resources by label",
				"gvr", gvr.String(),
				"error", err)
			return nil, errors.Wrapf(err, "cannot get all resources")
		}

		c.logger.Debug("Found resources for GVR",
			"gvr", gvr.String(),
			"count", len(res))
		resources = append(resources, res...)
	}

	c.logger.Debug("Completed fetching resources by labels",
		"total_resources", len(resources))
	return resources, nil
}

func (c *DefaultClusterClient) GetResourcesByLabel(ctx context.Context, ns string, gvr schema.GroupVersionResource, sel metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
	c.logger.Debug("Getting resources by label",
		"namespace", ns,
		"gvr", gvr.String(),
		"selector", sel.MatchLabels)

	var resources []*unstructured.Unstructured

	opts := metav1.ListOptions{}
	if len(sel.MatchLabels) > 0 {
		opts.LabelSelector = labels.Set(sel.MatchLabels).String()
	}

	c.logger.Debug("Listing resources",
		"labelSelector", opts.LabelSelector)
	list, err := c.dynamicClient.Resource(gvr).Namespace(ns).List(ctx, opts)
	if err != nil {
		c.logger.Debug("Failed to list resources",
			"gvr", gvr.String(),
			"labelSelector", opts.LabelSelector,
			"error", err)
		return nil, errors.Wrapf(err, "cannot list resources for '%s' matching '%s'", gvr, opts.LabelSelector)
	}

	for _, item := range list.Items {
		// Create a pointer to each item
		resources = append(resources, &item)
	}

	c.logger.Debug("Resources found by label",
		"count", len(resources),
		"gvr", gvr.String())
	return resources, nil
}

// GetEnvironmentConfigs fetches environment configs from the cluster.
func (c *DefaultClusterClient) GetEnvironmentConfigs(ctx context.Context) ([]*unstructured.Unstructured, error) {
	c.logger.Debug("Getting environment configs")

	envConfigsGVR := schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1alpha1",
		Resource: "environmentconfigs",
	}

	// we have the EnvironmentConfig type in the same package, so we could use it here, but
	// that might be troublesome for adding it to the unstructured ExtraResources list
	envConfigsClient := c.dynamicClient.Resource(envConfigsGVR)

	c.logger.Debug("Listing environment configs")
	list, err := envConfigsClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Debug("Failed to list environment configs", "error", err)
		return nil, errors.Wrap(err, "cannot list environment configs")
	}

	envConfigs := make([]*unstructured.Unstructured, len(list.Items))
	for i := range list.Items {
		envConfigs[i] = &list.Items[i]
	}

	c.logger.Debug("Environment configs retrieved", "count", len(envConfigs))
	return envConfigs, nil
}

// FindMatchingComposition finds a composition matching the given resource.
func (c *DefaultClusterClient) FindMatchingComposition(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
	xrGVK := res.GroupVersionKind()
	key := compositionCacheKey{
		apiVersion: xrGVK.GroupVersion().String(),
		kind:       xrGVK.Kind,
	}

	c.logger.Debug("Finding matching composition",
		"resource_name", res.GetName(),
		"gvk", xrGVK.String(),
		"key", fmt.Sprintf("%s/%s", key.apiVersion, key.kind))

	comp, ok := c.compositions[key]
	if !ok {
		c.logger.Debug("No matching composition found",
			"gvk", xrGVK.String(),
			"compositions_count", len(c.compositions))
		return nil, errors.Errorf("no composition found for %s", xrGVK.String())
	}

	c.logger.Debug("Found matching composition",
		"resource_name", res.GetName(),
		"composition_name", comp.GetName())
	return comp, nil
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
			return nil, errors.New(fmt.Sprintf("Unsupported composition Mode '%s'; supported types are [%s]", *comp.Spec.Mode, apiextensionsv1.CompositionModePipeline))
		}
		return nil, errors.New("Unsupported Composition; no Mode found.")
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
	c.logger.Debug("Converting compositions from unstructured", "count", len(list.Items))

	for _, obj := range list.Items {
		comp := &apiextensionsv1.Composition{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, comp); err != nil {
			c.logger.Debug("Failed to convert composition from unstructured",
				"name", obj.GetName(),
				"error", err)
			return nil, errors.Wrap(err, "cannot convert unstructured to Composition")
		}
		compositions = append(compositions, *comp)
	}

	c.logger.Debug("Successfully retrieved compositions", "count", len(compositions))
	return compositions, nil
}

func (c *DefaultClusterClient) findMatchingComposition(res *unstructured.Unstructured, compositionMap map[compositionCacheKey]*apiextensionsv1.Composition) (*apiextensionsv1.Composition, error) {
	xrGVK := res.GroupVersionKind()
	key := compositionCacheKey{
		apiVersion: xrGVK.GroupVersion().String(),
		kind:       xrGVK.Kind,
	}

	c.logger.Debug("Finding matching composition in provided map",
		"resource_name", res.GetName(),
		"gvk", xrGVK.String(),
		"key", fmt.Sprintf("%s/%s", key.apiVersion, key.kind),
		"available_compositions", len(compositionMap))

	comp, ok := compositionMap[key]
	if !ok {
		c.logger.Debug("No matching composition found in map", "gvk", xrGVK.String())
		return nil, errors.Errorf("no composition found for %s", xrGVK.String())
	}

	c.logger.Debug("Found matching composition in map",
		"resource_name", res.GetName(),
		"composition_name", comp.GetName())
	return comp, nil
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
	c.logger.Debug("Converting functions from unstructured", "count", len(list.Items))

	for _, obj := range list.Items {
		fn := &pkgv1.Function{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, fn); err != nil {
			c.logger.Debug("Failed to convert function from unstructured",
				"name", obj.GetName(),
				"error", err)
			return nil, errors.Wrap(err, "cannot convert unstructured to Function")
		}
		functions = append(functions, *fn)
	}

	c.logger.Debug("Successfully retrieved functions", "count", len(functions))
	return functions, nil
}

func (c *DefaultClusterClient) GetXRDs(ctx context.Context) ([]*unstructured.Unstructured, error) {
	c.logger.Debug("Getting XRDs from cluster")

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
	result := make([]*unstructured.Unstructured, len(items))

	c.logger.Debug("Processing XRDs", "count", len(items))
	for i := range items {
		// Create a pointer to each item
		result[i] = &items[i]
	}

	c.logger.Debug("Successfully retrieved XRDs", "count", len(result))
	return result, nil
}

// GetResource retrieves a resource from the cluster using the dynamic client
func (c *DefaultClusterClient) GetResource(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
	resourceID := fmt.Sprintf("%s/%s/%s", gvk.String(), namespace, name)
	c.logger.Debug("Getting resource from cluster", "resource", resourceID)

	// Create a GroupVersionResource from the GroupVersionKind
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: strings.ToLower(gvk.Kind) + "s", // Naive pluralization
	}

	// Handle special cases for some well-known types
	switch gvk.Kind {
	case "Ingress":
		gvr.Resource = "ingresses"
	case "Endpoints":
		gvr.Resource = "endpoints"
	case "ConfigMap":
		gvr.Resource = "configmaps"
		// Add other special cases as needed
	}

	// Get the resource
	var res *unstructured.Unstructured
	var err error

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

func (c *DefaultClusterClient) GetResourceTree(ctx context.Context, root *unstructured.Unstructured) (*resource.Resource, error) {
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
func (c *DefaultClusterClient) DryRunApply(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	resourceID := fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName())
	c.logger.Debug("Performing dry-run apply", "resource", resourceID)

	// Create GVR from the object
	gvk := obj.GroupVersionKind()
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: fmt.Sprintf("%ss", strings.ToLower(gvk.Kind)), // naive pluralization
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

	// All other groups likely require CRDs
	return true
}

// Private helper methods
func (c *DefaultClusterClient) ensureResourceMapLoaded(ctx context.Context) {
	c.resourceMapMutex.RLock()
	if c.resourceMapInitialized {
		c.resourceMapMutex.RUnlock()
		return
	}
	c.resourceMapMutex.RUnlock()

	// Need to load resources, acquire write lock
	c.resourceMapMutex.Lock()
	defer c.resourceMapMutex.Unlock()

	// Check again in case another goroutine loaded while we were waiting
	if c.resourceMapInitialized {
		return
	}

	c.logger.Debug("Loading API resources from server")

	// Initialize if needed
	if c.resourceMap == nil {
		c.resourceMap = make(map[schema.GroupVersionKind]bool)
	}

	// Get API resources from the server
	_, apiResourceLists, err := c.discoveryClient.ServerGroupsAndResources()
	if err != nil {
		// This can return a partial error, so we'll still process what we got
		if !discovery.IsGroupDiscoveryFailedError(err) {
			c.logger.Debug("Failed to get server resources (continuing)", "error", err)
			// Mark as initialized anyway so we don't repeatedly try on failure
			c.resourceMapInitialized = true
			return
		}
		// Log the partial error but continue
		c.logger.Debug("Partial error getting API resources", "error", err)
	}

	// Process discovered resources
	for _, apiResourceList := range apiResourceLists {
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			c.logger.Debug("Failed to parse group version",
				"groupVersion", apiResourceList.GroupVersion,
				"error", err)
			continue
		}

		for _, apiResource := range apiResourceList.APIResources {
			gvk := schema.GroupVersionKind{
				Group:   gv.Group,
				Version: gv.Version,
				Kind:    apiResource.Kind,
			}
			c.resourceMap[gvk] = true
		}
	}

	c.logger.Debug("Loaded API resources", "count", len(c.resourceMap))
	c.resourceMapInitialized = true
}

func (c *DefaultClusterClient) isKnownResource(gvk schema.GroupVersionKind) bool {
	c.resourceMapMutex.RLock()
	defer c.resourceMapMutex.RUnlock()

	return c.resourceMap[gvk]
}

// ClusterClientOptions holds configuration options for the cluster client
type ClusterClientOptions struct {
	// Logger is the logger to use
	Logger logging.Logger

	// QPS indicates the maximum queries per second to the API server
	QPS float32

	// Burst indicates the maximum burst size for throttle
	Burst int
}

// ClusterClientOption defines a function that can modify ClusterClientOptions
type ClusterClientOption func(*ClusterClientOptions)

// WithLogger sets the logger for the cluster client
func WithLogger(logger logging.Logger) ClusterClientOption {
	return func(o *ClusterClientOptions) {
		o.Logger = logger
	}
}

// WithQPS sets the QPS for the client
func WithQPS(qps float32) ClusterClientOption {
	return func(o *ClusterClientOptions) {
		o.QPS = qps
	}
}

// WithBurst sets the Burst for the client
func WithBurst(burst int) ClusterClientOption {
	return func(o *ClusterClientOptions) {
		o.Burst = burst
	}
}
