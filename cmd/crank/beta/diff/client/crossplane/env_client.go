// package crossplane/environment_client.go

package crossplane

import (
	"context"
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/client/core"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/client/kubernetes"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// EnvironmentClient handles environment configurations
type EnvironmentClient interface {
	core.Initializable

	// GetEnvironmentConfigs gets all environment configurations
	GetEnvironmentConfigs(ctx context.Context) ([]*un.Unstructured, error)

	// GetEnvironmentConfig gets a specific environment config by name
	GetEnvironmentConfig(ctx context.Context, name string) (*un.Unstructured, error)
}

// DefaultEnvironmentClient implements EnvironmentClient
type DefaultEnvironmentClient struct {
	resourceClient kubernetes.ResourceClient
	logger         logging.Logger

	// Cache of environment configs
	envConfigs map[string]*un.Unstructured
}

// NewEnvironmentClient creates a new DefaultEnvironmentClient
func NewEnvironmentClient(resourceClient kubernetes.ResourceClient, logger logging.Logger) EnvironmentClient {
	return &DefaultEnvironmentClient{
		resourceClient: resourceClient,
		logger:         logger,
		envConfigs:     make(map[string]*un.Unstructured),
	}
}

// Initialize loads environment configs into the cache
func (c *DefaultEnvironmentClient) Initialize(ctx context.Context) error {
	c.logger.Debug("Initializing environment client")

	// List environment configs to populate the cache
	configs, err := c.GetEnvironmentConfigs(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot list environment configs")
	}

	// Store in cache
	for _, config := range configs {
		c.envConfigs[config.GetName()] = config
	}

	c.logger.Debug("Environment client initialized", "envConfigsCount", len(c.envConfigs))
	return nil
}

// GetEnvironmentConfigs gets all environment configurations
func (c *DefaultEnvironmentClient) GetEnvironmentConfigs(ctx context.Context) ([]*un.Unstructured, error) {
	c.logger.Debug("Getting environment configs")

	// Define the EnvironmentConfig GVK
	gvk := schema.GroupVersionKind{
		Group:   "apiextensions.crossplane.io",
		Version: "v1alpha1",
		Kind:    "EnvironmentConfig",
	}

	// List all EnvironmentConfigs
	envConfigs, err := c.resourceClient.ListResources(ctx, gvk, "")
	if err != nil {
		c.logger.Debug("Failed to list environment configs", "error", err)
		return nil, errors.Wrap(err, "cannot list environment configs")
	}

	c.logger.Debug("Environment configs retrieved", "count", len(envConfigs))
	return envConfigs, nil
}

// GetEnvironmentConfig gets a specific environment config by name
func (c *DefaultEnvironmentClient) GetEnvironmentConfig(ctx context.Context, name string) (*un.Unstructured, error) {
	// Check cache first
	if config, ok := c.envConfigs[name]; ok {
		return config, nil
	}

	// Not in cache, fetch from cluster
	gvk := schema.GroupVersionKind{
		Group:   "apiextensions.crossplane.io",
		Version: "v1alpha1",
		Kind:    "EnvironmentConfig",
	}

	config, err := c.resourceClient.GetResource(ctx, gvk, "", name)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get environment config %s", name)
	}

	// Update cache
	c.envConfigs[name] = config

	return config, nil
}
