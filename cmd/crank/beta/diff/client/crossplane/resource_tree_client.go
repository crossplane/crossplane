package crossplane

import (
	"context"

	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/cmd/crank/beta/diff/client/core"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal/resource"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal/resource/xrm"
)

// ResourceTreeClient handles resource tree operations.
type ResourceTreeClient interface {
	core.Initializable

	// GetResourceTree gets the resource tree for a root resource
	GetResourceTree(ctx context.Context, root *un.Unstructured) (*resource.Resource, error)
}

// DefaultResourceTreeClient implements ResourceTreeClient.
type DefaultResourceTreeClient struct {
	treeClient *xrm.Client
	logger     logging.Logger
}

// NewResourceTreeClient creates a new DefaultResourceTreeClient.
func NewResourceTreeClient(treeClient *xrm.Client, logger logging.Logger) ResourceTreeClient {
	return &DefaultResourceTreeClient{
		treeClient: treeClient,
		logger:     logger,
	}
}

// Initialize initializes the resource tree client.
func (c *DefaultResourceTreeClient) Initialize(_ context.Context) error {
	c.logger.Debug("Initializing resource tree client")
	// No initialization needed currently
	return nil
}

// GetResourceTree gets the resource tree for a root resource.
func (c *DefaultResourceTreeClient) GetResourceTree(ctx context.Context, root *un.Unstructured) (*resource.Resource, error) {
	c.logger.Debug("Getting resource tree",
		"resource_kind", root.GetKind(),
		"resource_name", root.GetName(),
		"resource_uid", root.GetUID())

	// Convert to resource.Resource for the XRM client
	res := &resource.Resource{
		Unstructured: *root,
	}

	tree, err := c.treeClient.GetResourceTree(ctx, res)
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
