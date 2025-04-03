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
	resourceName, err := convertGVKToCRDName(gvk)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot determine CRD name for %s", gvk.String())
	}

	c.logger.Debug("Looking up CRD", "gvk", gvk.String(), "crdName", resourceName)

	// Define the CRD GVR directly to avoid recursion
	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	// Fetch the CRD
	crd, err := c.dynamicClient.Resource(crdGVR).Get(ctx, resourceName, metav1.GetOptions{})
	if err != nil {
		c.logger.Debug("Failed to get CRD", "gvk", gvk.String(), "crdName", resourceName, "error", err)
		return nil, errors.Wrapf(err, "cannot get CRD %s for %s", resourceName, gvk.String())
	}

	c.logger.Debug("Successfully retrieved CRD", "gvk", gvk.String(), "crdName", resourceName)
	return crd, nil
}

// IsCRDRequired checks if a GVK requires a CRD
func (c *DefaultSchemaClient) IsCRDRequired(ctx context.Context, gvk schema.GroupVersionKind) bool {
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
	resources, err := c.discoveryClient.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		// If we can't find it through discovery, assume it requires a CRD
		c.logger.Debug("Resource not found in discovery, assuming CRD is required",
			"gvk", gvk.String(),
			"error", err)
		c.cacheResourceType(gvk, true)
		return true
	}

	// Check if this kind exists in the discovered resources
	for _, r := range resources.APIResources {
		if r.Kind == gvk.Kind {
			// It's discoverable, so it's a CRD
			c.cacheResourceType(gvk, true)
			return true
		}
	}

	// Default to requiring a CRD
	c.cacheResourceType(gvk, true)
	return true
}

// ValidateResource validates a resource against its schema
func (c *DefaultSchemaClient) ValidateResource(ctx context.Context, resource *un.Unstructured) error {
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

// Helper to convert GVK to CRD name
func convertGVKToCRDName(gvk schema.GroupVersionKind) (string, error) {
	// Format is: plural.group
	// We'll make a simple pluralization for now
	plural := strings.ToLower(gvk.Kind) + "s"
	return fmt.Sprintf("%s.%s", plural, gvk.Group), nil
}
