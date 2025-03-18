package clusterclient

import (
	"context"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"strings"
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

	// GetResourcesByLabel retrieves all resources from the cluster based on the provided GVR and selector
	GetResourcesByLabel(ctx context.Context, ns string, gvr schema.GroupVersionResource, sel metav1.LabelSelector) ([]*unstructured.Unstructured, error)

	// DryRunApply performs a server-side apply with dry-run flag for diffing
	DryRunApply(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
}

// DefaultClusterClient handles all interactions with the Kubernetes cluster.
type DefaultClusterClient struct {
	dynamicClient dynamic.Interface
	compositions  map[compositionCacheKey]*apiextensionsv1.Composition
	functions     map[string]pkgv1.Function
}

// NewClusterClient creates a new DefaultClusterClient instance.
func NewClusterClient(config *rest.Config) (*DefaultClusterClient, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create dynamic client")
	}

	return &DefaultClusterClient{
		dynamicClient: dynamicClient,
	}, nil
}

// Initialize loads compositions and functions from the cluster.
func (c *DefaultClusterClient) Initialize(ctx context.Context) error {
	compositions, err := c.listCompositions(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot list compositions")
	}

	c.compositions = make(map[compositionCacheKey]*apiextensionsv1.Composition, len(compositions))
	for i := range compositions {
		key := compositionCacheKey{
			apiVersion: compositions[i].Spec.CompositeTypeRef.APIVersion,
			kind:       compositions[i].Spec.CompositeTypeRef.Kind,
		}
		c.compositions[key] = &compositions[i]
	}

	functions, err := c.listFunctions(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot list functions")
	}

	c.functions = make(map[string]pkgv1.Function, len(functions))
	for i := range functions {
		c.functions[functions[i].GetName()] = functions[i]
	}

	return nil
}

// GetAllResourcesByLabels fetches all resources from the cluster based on the provided GVRs and selectors
func (c *DefaultClusterClient) GetAllResourcesByLabels(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
	if len(gvrs) != len(selectors) {
		return nil, errors.New("number of GVRs must match number of selectors")
	}

	var resources []*unstructured.Unstructured

	for i, gvr := range gvrs {
		// List resources matching the selector
		sel := selectors[i]

		res, err := c.GetResourcesByLabel(ctx, "", gvr, sel)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot get all resources")
		}

		resources = append(resources, res...)
	}

	return resources, nil
}

func (c *DefaultClusterClient) GetResourcesByLabel(ctx context.Context, ns string, gvr schema.GroupVersionResource, sel metav1.LabelSelector) ([]*unstructured.Unstructured, error) {

	var resources []*unstructured.Unstructured

	opts := metav1.ListOptions{}
	if len(sel.MatchLabels) > 0 {
		opts.LabelSelector = labels.Set(sel.MatchLabels).String()
	}

	list, err := c.dynamicClient.Resource(gvr).Namespace(ns).List(ctx, opts)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot list resources for '%s' matching '%s'", gvr, opts.LabelSelector)
	}

	for _, item := range list.Items {

		// Create a pointer to each item
		resources = append(resources, &item)
	}
	return resources, nil
}

// GetEnvironmentConfigs fetches environment configs from the cluster.
func (c *DefaultClusterClient) GetEnvironmentConfigs(ctx context.Context) ([]*unstructured.Unstructured, error) {
	envConfigsGVR := schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1alpha1",
		Resource: "environmentconfigs",
	}

	// TODO make sure namespacing works everywhere
	// TODO fix naive pluralization
	// TODO handle composition lookup with selectors if we find more than one matching the ref type
	// TODO:  nested external resources

	// we have the EnvironmentConfig type in the same package, so we could use it here, but
	// that might be troublesome for adding it to the unstructured ExtraResources list
	envConfigsClient := c.dynamicClient.Resource(envConfigsGVR)

	list, err := envConfigsClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list environment configs")
	}

	envConfigs := make([]*unstructured.Unstructured, len(list.Items))
	for i := range list.Items {
		envConfigs[i] = &list.Items[i]
	}

	return envConfigs, nil
}

// FindMatchingComposition finds a composition matching the given resource.
func (c *DefaultClusterClient) FindMatchingComposition(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
	xrGVK := res.GroupVersionKind()
	key := compositionCacheKey{
		apiVersion: xrGVK.GroupVersion().String(),
		kind:       xrGVK.Kind,
	}

	comp, ok := c.compositions[key]
	if !ok {
		return nil, errors.Errorf("no composition found for %s", xrGVK.String())
	}

	return comp, nil
}

// GetFunctionsFromPipeline returns functions referenced in the composition pipeline.
func (c *DefaultClusterClient) GetFunctionsFromPipeline(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
	if comp.Spec.Mode == nil || *comp.Spec.Mode != apiextensionsv1.CompositionModePipeline {
		return nil, nil
	}

	functions := make([]pkgv1.Function, 0, len(comp.Spec.Pipeline))
	for _, step := range comp.Spec.Pipeline {
		fn, ok := c.functions[step.FunctionRef.Name]
		if !ok {
			return nil, errors.Errorf("function %q referenced in pipeline step %q not found", step.FunctionRef.Name, step.Step)
		}
		functions = append(functions, fn)
	}

	return functions, nil
}

func (c *DefaultClusterClient) listCompositions(ctx context.Context) ([]apiextensionsv1.Composition, error) {
	compositionsGVR := schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1",
		Resource: "compositions",
	}
	compositionsClient := c.dynamicClient.Resource(compositionsGVR)

	list, err := compositionsClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list compositions from cluster")
	}

	compositions := make([]apiextensionsv1.Composition, 0, len(list.Items))
	for _, obj := range list.Items {
		comp := &apiextensionsv1.Composition{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, comp); err != nil {
			return nil, errors.Wrap(err, "cannot convert unstructured to Composition")
		}
		compositions = append(compositions, *comp)
	}

	return compositions, nil
}

func (c *DefaultClusterClient) findMatchingComposition(res *unstructured.Unstructured, compositionMap map[compositionCacheKey]*apiextensionsv1.Composition) (*apiextensionsv1.Composition, error) {
	xrGVK := res.GroupVersionKind()
	key := compositionCacheKey{
		apiVersion: xrGVK.GroupVersion().String(),
		kind:       xrGVK.Kind,
	}

	comp, ok := compositionMap[key]
	if !ok {
		return nil, errors.Errorf("no composition found for %s", xrGVK.String())
	}

	return comp, nil
}

func (c *DefaultClusterClient) listFunctions(ctx context.Context) ([]pkgv1.Function, error) {
	functionsGVR := schema.GroupVersionResource{
		Group:    "pkg.crossplane.io",
		Version:  "v1",
		Resource: "functions",
	}
	functionsClient := c.dynamicClient.Resource(functionsGVR)

	list, err := functionsClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list functions from cluster")
	}

	functions := make([]pkgv1.Function, 0, len(list.Items))
	for _, obj := range list.Items {
		fn := &pkgv1.Function{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, fn); err != nil {
			return nil, errors.Wrap(err, "cannot convert unstructured to Function")
		}
		functions = append(functions, *fn)
	}

	return functions, nil
}

func (c *DefaultClusterClient) GetXRDs(ctx context.Context) ([]*unstructured.Unstructured, error) {
	// Create a dynamic resource interface for XRDs
	xrdsGVR := schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1",
		Resource: "compositeresourcedefinitions",
	}
	xrdsClient := c.dynamicClient.Resource(xrdsGVR)

	// List all XRDs since we need to find one matching the XR's group/kind
	list, err := xrdsClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list XRDs")
	}

	items := list.Items
	result := make([]*unstructured.Unstructured, len(items))

	for i := range items {
		// Create a pointer to each item
		result[i] = &items[i]
	}

	return result, nil
}

// GetResource retrieves a resource from the cluster using the dynamic client
func (c *DefaultClusterClient) GetResource(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
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
	var resource *unstructured.Unstructured
	var err error

	// If namespace is empty string, it will be ignored
	resource, err = c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})

	if err != nil {
		return nil, errors.Wrapf(err, "cannot get resource %s/%s of kind %s", namespace, name, gvk.Kind)
	}

	return resource, nil
}

// DryRunApply performs a server-side apply with dry-run flag for diffing
func (c *DefaultClusterClient) DryRunApply(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// Create GVR from the object
	gvk := obj.GroupVersionKind()
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: fmt.Sprintf("%ss", strings.ToLower(gvk.Kind)), // naive pluralization
	}

	// Get the resource client for the namespace
	// if obj.namespace is empty, calling Namespace() is a no-op.
	resourceClient := c.dynamicClient.Resource(gvr).Namespace(obj.GetNamespace())

	// Set a field manager for server-side apply
	fieldManager := "crossplane-diff"

	// Create apply options for a dry run
	applyOptions := metav1.ApplyOptions{
		FieldManager: fieldManager,
		Force:        true,
		DryRun:       []string{metav1.DryRunAll},
	}

	// Perform a dry-run server-side apply
	result, err := resourceClient.Apply(ctx, obj.GetName(), obj, applyOptions)
	if err != nil {
		// Log the error details for debugging
		return nil, errors.Wrapf(err, "failed to apply resource %s/%s: %v",
			obj.GetNamespace(), obj.GetName(), err)
	}
	return result, nil
}
