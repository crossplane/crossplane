package kubernetes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/cmd/crank/beta/diff/client/core"
)

// ApplyClient handles server-side apply operations.
type ApplyClient interface {
	// DryRunApply performs a dry-run server-side apply
	DryRunApply(ctx context.Context, obj *un.Unstructured) (*un.Unstructured, error)
}

// DefaultApplyClient implements ApplyClient.
type DefaultApplyClient struct {
	dynamicClient dynamic.Interface
	typeConverter TypeConverter
	logger        logging.Logger
}

// NewApplyClient creates a new DefaultApplyClient.
func NewApplyClient(clients *core.Clients, converter TypeConverter, logger logging.Logger) ApplyClient {
	return &DefaultApplyClient{
		dynamicClient: clients.Dynamic,
		typeConverter: converter,
		logger:        logger,
	}
}

// DryRunApply performs a dry-run server-side apply.
func (c *DefaultApplyClient) DryRunApply(ctx context.Context, obj *un.Unstructured) (*un.Unstructured, error) {
	resourceID := fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName())
	c.logger.Debug("Performing dry-run apply", "resource", resourceID)

	// Get the GVK from the object
	gvk := obj.GroupVersionKind()

	// Convert GVK to GVR
	gvr, err := c.typeConverter.GVKToGVR(ctx, gvk)
	if err != nil {
		c.logger.Debug("Failed to convert GVK to GVR", "gvk", gvk.String(), "error", err)
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
		c.logger.Debug("Dry-run apply failed", "resource", resourceID, "error", err)
		return nil, errors.Wrapf(err, "failed to apply resource %s/%s",
			obj.GetNamespace(), obj.GetName())
	}

	c.logger.Debug("Dry-run apply successful", "resource", resourceID, "resourceVersion", result.GetResourceVersion())
	return result, nil
}
