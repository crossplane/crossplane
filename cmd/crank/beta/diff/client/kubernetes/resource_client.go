package kubernetes

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/cmd/crank/beta/diff/client/core"
)

// ResourceClient handles basic CRUD operations for Kubernetes resources.
type ResourceClient interface {
	// GetResource retrieves a resource by its GVK, namespace, and name
	GetResource(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*un.Unstructured, error)

	// ListResources lists resources matching the given GVK and namespace
	ListResources(ctx context.Context, gvk schema.GroupVersionKind, namespace string) ([]*un.Unstructured, error)

	// GetResourcesByLabel returns resources matching labels in the given namespace
	GetResourcesByLabel(ctx context.Context, namespace string, gvk schema.GroupVersionKind, sel metav1.LabelSelector) ([]*un.Unstructured, error)

	// GetAllResourcesByLabels gets resources by labels across multiple GVKs
	GetAllResourcesByLabels(ctx context.Context, gvks []schema.GroupVersionKind, selectors []metav1.LabelSelector) ([]*un.Unstructured, error)
}

// DefaultResourceClient implements the ResourceClient interface.
type DefaultResourceClient struct {
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
	converter       TypeConverter
	logger          logging.Logger
}

// NewResourceClient creates a new DefaultResourceClient instance.
func NewResourceClient(clients *core.Clients, converter TypeConverter, logger logging.Logger) ResourceClient {
	return &DefaultResourceClient{
		dynamicClient:   clients.Dynamic,
		discoveryClient: clients.Discovery,
		converter:       converter,
		logger:          logger,
	}
}

// GetResource retrieves a resource from the cluster based on its GVK, namespace, and name.
func (c *DefaultResourceClient) GetResource(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*un.Unstructured, error) {
	resourceID := fmt.Sprintf("%s/%s/%s", gvk.String(), namespace, name)
	c.logger.Debug("Getting resource from cluster", "resource", resourceID)

	// Convert GVK to GVR
	gvr, err := c.converter.GVKToGVR(ctx, gvk)
	if err != nil {
		c.logger.Debug("Failed to convert GVK to GVR", "gvk", gvk.String(), "error", err)
		return nil, errors.Wrapf(err, "cannot get resource %s/%s of kind %s", namespace, name, gvk.Kind)
	}

	// Get the resource
	res, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		c.logger.Debug("Failed to get resource", "resource", resourceID, "error", err)
		return nil, errors.Wrapf(err, "cannot get resource %s/%s of kind %s", namespace, name, gvk.Kind)
	}

	c.logger.Debug("Retrieved resource",
		"resource", resourceID,
		"uid", res.GetUID(),
		"resourceVersion", res.GetResourceVersion())
	return res, nil
}

// GetResourcesByLabel returns resources matching labels in the given namespace.
func (c *DefaultResourceClient) GetResourcesByLabel(ctx context.Context, namespace string, gvk schema.GroupVersionKind, sel metav1.LabelSelector) ([]*un.Unstructured, error) {
	c.logger.Debug("Getting resources by label",
		"namespace", namespace,
		"gvk", gvk.String(),
		"selector", sel.MatchLabels)

	// Convert GVK to GVR
	gvr, err := c.converter.GVKToGVR(ctx, gvk)
	if err != nil {
		c.logger.Debug("Failed to convert GVK to GVR", "gvk", gvk.String(), "error", err)
		return nil, errors.Wrapf(err, "cannot list resources for '%s' matching labels", gvk.String())
	}

	// Create list options with label selector
	opts := metav1.ListOptions{}
	if len(sel.MatchLabels) > 0 {
		opts.LabelSelector = metav1.FormatLabelSelector(&sel)
	}

	// Perform the list operation
	list, err := c.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, opts)
	if err != nil {
		c.logger.Debug("Failed to list resources", "gvk", gvk.String(), "labelSelector", opts.LabelSelector, "error", err)
		return nil, errors.Wrapf(err, "cannot list resources for '%s' matching '%s'", gvk.String(), opts.LabelSelector)
	}

	// Convert the list items to a slice of pointers
	resources := make([]*un.Unstructured, 0, len(list.Items))
	for i := range list.Items {
		resources = append(resources, &list.Items[i])
	}

	c.logger.Debug("Resources found by label", "count", len(resources), "gvk", gvk.String())
	return resources, nil
}

// ListResources lists resources matching the given GVK and namespace.
func (c *DefaultResourceClient) ListResources(ctx context.Context, gvk schema.GroupVersionKind, namespace string) ([]*un.Unstructured, error) {
	c.logger.Debug("Listing resources", "gvk", gvk.String(), "namespace", namespace)

	// Convert GVK to GVR
	gvr, err := c.converter.GVKToGVR(ctx, gvk)
	if err != nil {
		c.logger.Debug("Failed to convert GVK to GVR", "gvk", gvk.String(), "error", err)
		return nil, errors.Wrapf(err, "cannot list resources for '%s'", gvk.String())
	}

	// Perform the list operation
	list, err := c.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Debug("Failed to list resources", "gvk", gvk.String(), "namespace", namespace, "error", err)
		return nil, errors.Wrapf(err, "cannot list resources for '%s'", gvk.String())
	}

	// Convert from items to slice of pointers
	resources := make([]*un.Unstructured, 0, len(list.Items))
	for i := range list.Items {
		resources = append(resources, &list.Items[i])
	}

	c.logger.Debug("Listed resources", "gvk", gvk.String(), "namespace", namespace, "count", len(resources))
	return resources, nil
}

// GetAllResourcesByLabels gets resources by labels across multiple GVKs.
func (c *DefaultResourceClient) GetAllResourcesByLabels(ctx context.Context, gvks []schema.GroupVersionKind, selectors []metav1.LabelSelector) ([]*un.Unstructured, error) {
	if len(gvks) != len(selectors) {
		c.logger.Debug("GVKs and selectors count mismatch", "gvks_count", len(gvks), "selectors_count", len(selectors))
		return nil, errors.New("number of GVKs must match number of selectors")
	}

	c.logger.Debug("Fetching resources by labels", "gvks_count", len(gvks))

	var resources []*un.Unstructured

	for i, gvk := range gvks {
		// List resources matching the selector
		sel := selectors[i]
		c.logger.Debug("Getting resources for GVK with selector", "gvk", gvk.String(), "selector", sel.MatchLabels)

		res, err := c.GetResourcesByLabel(ctx, "", gvk, sel)
		if err != nil {
			c.logger.Debug("Failed to get resources by label", "gvk", gvk.String(), "error", err)
			return nil, errors.Wrapf(err, "cannot get all resources")
		}

		c.logger.Debug("Found resources for GVK", "gvk", gvk.String(), "count", len(res))
		resources = append(resources, res...)
	}

	c.logger.Debug("Completed fetching resources by labels", "total_resources", len(resources))
	return resources, nil
}
