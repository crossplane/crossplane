package clusterclient

import (
	"context"
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
)

type compositionCacheKey struct {
	apiVersion string
	kind       string
}

// ClusterClient handles all interactions with the Kubernetes cluster.
type ClusterClient struct {
	dynamicClient dynamic.Interface
	compositions  map[compositionCacheKey]*apiextensionsv1.Composition
	functions     map[string]pkgv1.Function
}

// NewClusterClient creates a new ClusterClient instance.
func NewClusterClient(config *rest.Config) (*ClusterClient, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create dynamic client")
	}

	return &ClusterClient{
		dynamicClient: dynamicClient,
	}, nil
}

// Initialize loads compositions and functions from the cluster.
func (c *ClusterClient) Initialize(ctx context.Context) error {
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

// GetExtraResources fetches extra resources from the cluster.
// GetExtraResources fetches extra resources from the cluster based on the provided GVRs and selectors
func (c *ClusterClient) GetExtraResources(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]unstructured.Unstructured, error) {
	if len(gvrs) != len(selectors) {
		return nil, errors.New("number of GVRs must match number of selectors")
	}

	var resources []unstructured.Unstructured

	for i, gvr := range gvrs {
		// List resources matching the selector
		opts := metav1.ListOptions{}
		if len(selectors[i].MatchLabels) > 0 {
			opts.LabelSelector = labels.Set(selectors[i].MatchLabels).String()
		}

		list, err := c.dynamicClient.Resource(gvr).List(ctx, opts)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot list resources for %s", gvr)
		}

		resources = append(resources, list.Items...)
	}

	return resources, nil
}

// GetEnvironmentConfigs fetches environment configs from the cluster.
func (c *ClusterClient) GetEnvironmentConfigs(ctx context.Context) ([]unstructured.Unstructured, error) {
	envConfigsGVR := schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1alpha1",
		Resource: "environmentconfigs",
	}

	// TODO:  we have the EnvironmentConfig type in the same package, so we can use it here, but
	// this might be troublesome for adding it to the unstructured ExtraResources list
	envConfigsClient := c.dynamicClient.Resource(envConfigsGVR)

	list, err := envConfigsClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list environment configs")
	}

	envConfigs := make([]unstructured.Unstructured, len(list.Items))
	for i := range list.Items {
		envConfigs[i] = list.Items[i]
	}

	return envConfigs, nil
}

// FindMatchingComposition finds a composition matching the given resource.
func (c *ClusterClient) FindMatchingComposition(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
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
func (c *ClusterClient) GetFunctionsFromPipeline(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
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

func (c *ClusterClient) listCompositions(ctx context.Context) ([]apiextensionsv1.Composition, error) {
	compositionsGVR := schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1",
		Resource: "compositions",
	}
	compositionsClient := c.dynamicClient.Resource(compositionsGVR)

	list, err := compositionsClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list compositions")
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

func (c *ClusterClient) findMatchingComposition(res *unstructured.Unstructured, compositionMap map[compositionCacheKey]*apiextensionsv1.Composition) (*apiextensionsv1.Composition, error) {
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

func (c *ClusterClient) listFunctions(ctx context.Context) ([]pkgv1.Function, error) {
	functionsGVR := schema.GroupVersionResource{
		Group:    "pkg.crossplane.io",
		Version:  "v1",
		Resource: "functions",
	}
	functionsClient := c.dynamicClient.Resource(functionsGVR)

	list, err := functionsClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list functions")
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

func (c *ClusterClient) GetXRDSchema(ctx context.Context, res *unstructured.Unstructured) (*apiextensionsv1.CompositeResourceDefinition, error) {
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

	// Find the XRD that defines this XR's type
	xrGVK := res.GroupVersionKind()
	for _, obj := range list.Items {
		xrd := &apiextensionsv1.CompositeResourceDefinition{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, xrd); err != nil {
			return nil, errors.Wrap(err, "cannot convert unstructured to XRD")
		}

		// The XRD's group and kind must match the XR's
		if xrd.Spec.Group == xrGVK.Group && xrd.Spec.Names.Kind == xrGVK.Kind {
			return xrd, nil
		}
	}

	return nil, errors.Errorf("no XRD found for %s", xrGVK.String())
}
