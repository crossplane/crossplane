// package kubernetes/schema_client.go

package kubernetes

import (
	"context"
	"fmt"
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/client/core"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"sync"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

// SchemaClient handles operations related to Kubernetes schemas and CRDs
type SchemaClient interface {

	// GetCRD gets the CustomResourceDefinition for a given GVK
	GetCRD(ctx context.Context, gvk schema.GroupVersionKind) (*un.Unstructured, error)

	// IsCRDRequired checks if a GVK requires a CRD
	IsCRDRequired(ctx context.Context, gvk schema.GroupVersionKind) bool

	// ValidateResource validates a resource against its schema
	ValidateResource(ctx context.Context, resource *un.Unstructured) error
}

// DefaultSchemaClient implements SchemaClient
type DefaultSchemaClient struct {
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
	logger          logging.Logger

	// Resource type caching
	resourceTypeMap map[schema.GroupVersionKind]bool
	resourceMapMu   sync.RWMutex
}

// NewSchemaClient creates a new DefaultSchemaClient
func NewSchemaClient(clients *core.Clients, logger logging.Logger) SchemaClient {
	return &DefaultSchemaClient{
		dynamicClient:   clients.Dynamic,
		discoveryClient: clients.Discovery,
		logger:          logger,
		resourceTypeMap: make(map[schema.GroupVersionKind]bool),
	}
}

// GetCRD gets the CustomResourceDefinition for a given GVK
func (c *DefaultSchemaClient) GetCRD(ctx context.Context, gvk schema.GroupVersionKind) (*un.Unstructured, error) {
	// Get the pluralized resource name
	resourceName, err := c.convertGVKToCRDName(gvk)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot determine CRD name for %s", gvk.String())
	}

	c.logger.Debug("Looking up CRD", "gvk", gvk.String(), "crdName", resourceName)

	// Construct the CRD name using the resource name and group
	crdName := fmt.Sprintf("%s.%s", resourceName, gvk.Group)

	// Define the CRD GVR directly to avoid recursion
	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	// Fetch the CRD
	crd, err := c.dynamicClient.Resource(crdGVR).Get(ctx, crdName, metav1.GetOptions{})
	if err != nil {
		c.logger.Debug("Failed to get CRD", "gvk", gvk.String(), "crdName", crdName, "error", err)
		return nil, errors.Wrapf(err, "cannot get CRD %s for %s", crdName, gvk.String())
	}

	c.logger.Debug("Successfully retrieved CRD", "gvk", gvk.String(), "crdName", resourceName)
	return crd, nil
}

// IsCRDRequired checks if a GVK requires a CRD
func (c *DefaultSchemaClient) IsCRDRequired(_ context.Context, gvk schema.GroupVersionKind) bool {
	// Check cache first
	c.resourceMapMu.RLock()
	if val, ok := c.resourceTypeMap[gvk]; ok {
		c.resourceMapMu.RUnlock()
		return val
	}
	c.resourceMapMu.RUnlock()

	// Core API resources never need CRDs
	if gvk.Group == "" {
		c.cacheResourceType(gvk, false)
		return false
	}

	// Standard Kubernetes API groups
	builtInGroups := []string{
		"apps", "batch", "extensions", "policy", "autoscaling",
	}
	for _, group := range builtInGroups {
		if gvk.Group == group {
			c.cacheResourceType(gvk, false)
			return false
		}
	}

	// k8s.io domain suffix groups are typically built-in
	// (except apiextensions.k8s.io which defines CRDs themselves)
	if strings.HasSuffix(gvk.Group, ".k8s.io") && gvk.Group != "apiextensions.k8s.io" {
		c.cacheResourceType(gvk, false)
		return false
	}

	// Try to query the discovery API to see if this resource exists
	// TODO:  should this use the TypeConverter?
	_, err := c.convertGVKToCRDName(gvk)
	if err != nil {
		// If we can't find it through discovery, assume it requires a CRD
		c.logger.Debug("Resource not found in discovery, assuming CRD is required",
			"gvk", gvk.String(),
			"error", err)
		c.cacheResourceType(gvk, true)
		return true
	}

	// Default to requiring a CRD
	c.cacheResourceType(gvk, true)
	return true
}

// ValidateResource validates a resource against its schema
func (c *DefaultSchemaClient) ValidateResource(_ context.Context, resource *un.Unstructured) error {
	// This would use OpenAPI validation - simplified for now
	c.logger.Debug("Validating resource", "kind", resource.GetKind(), "name", resource.GetName())
	return nil
}

// Helper to cache resource type requirements
func (c *DefaultSchemaClient) cacheResourceType(gvk schema.GroupVersionKind, requiresCRD bool) {
	c.resourceMapMu.Lock()
	defer c.resourceMapMu.Unlock()
	c.resourceTypeMap[gvk] = requiresCRD
}

// TODO this is just type converter GVKToGVR?
func (c *DefaultSchemaClient) convertGVKToCRDName(gvk schema.GroupVersionKind) (string, error) {
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
