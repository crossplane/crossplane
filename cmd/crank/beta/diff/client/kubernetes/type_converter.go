package kubernetes

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/cmd/crank/beta/diff/client/core"
)

// TypeConverter provides conversion between Kubernetes types.
type TypeConverter interface {
	// GVKToGVR converts a GroupVersionKind to a GroupVersionResource
	GVKToGVR(ctx context.Context, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error)

	// GetResourceNameForGVK returns the resource name for a given GVK
	GetResourceNameForGVK(ctx context.Context, gvk schema.GroupVersionKind) (string, error)
}

// DefaultTypeConverter implements TypeConverter.
type DefaultTypeConverter struct {
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
	logger          logging.Logger

	// GVK caching
	gvkToGVRMap   map[schema.GroupVersionKind]schema.GroupVersionResource
	gvkToGVRMutex sync.RWMutex
}

// NewTypeConverter creates a new DefaultTypeConverter.
func NewTypeConverter(clients *core.Clients, logger logging.Logger) TypeConverter {
	return &DefaultTypeConverter{
		dynamicClient:   clients.Dynamic,
		discoveryClient: clients.Discovery,
		logger:          logger,
		gvkToGVRMap:     make(map[schema.GroupVersionKind]schema.GroupVersionResource),
	}
}

// GVKToGVR converts a GroupVersionKind to a GroupVersionResource.
func (c *DefaultTypeConverter) GVKToGVR(ctx context.Context, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	// Use the cached mapping if we have it
	c.gvkToGVRMutex.RLock()
	if gvr, ok := c.gvkToGVRMap[gvk]; ok {
		c.gvkToGVRMutex.RUnlock()
		return gvr, nil
	}
	c.gvkToGVRMutex.RUnlock()

	// Get the resource name
	resourceName, err := c.GetResourceNameForGVK(ctx, gvk)
	if err != nil {
		c.logger.Debug("Failed to get resource name for GVK", "gvk", gvk.String(), "error", err)
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
	c.gvkToGVRMap[gvk] = gvr
	c.gvkToGVRMutex.Unlock()

	return gvr, nil
}

// GetResourceNameForGVK returns the resource name for a given GVK.
func (c *DefaultTypeConverter) GetResourceNameForGVK(_ context.Context, gvk schema.GroupVersionKind) (string, error) {
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
	return "", errors.Errorf("no resource found for kind %s in group version %s", gvk.Kind, gvk.GroupVersion().String())
}
