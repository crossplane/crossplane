package managed

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

const errFmtUnexpectedObjectType = "unexpected object type %T"

// typedExternalConnectDisconnectorWrapper wraps a TypedExternalConnector to a
// common ExternalConnector.
type typedExternalConnectDisconnectorWrapper[managed resource.Managed] struct {
	c TypedExternalConnectDisconnector[managed]
}

func (c *typedExternalConnectDisconnectorWrapper[managed]) Connect(ctx context.Context, mg resource.Managed) (ExternalClient, error) {
	cr, ok := mg.(managed)
	if !ok {
		return nil, errors.Errorf(errFmtUnexpectedObjectType, mg)
	}

	external, err := c.c.Connect(ctx, cr)
	if err != nil {
		return nil, err
	}

	return &typedExternalClientWrapper[managed]{c: external}, nil
}

func (c *typedExternalConnectDisconnectorWrapper[managed]) Disconnect(ctx context.Context) error {
	return c.c.Disconnect(ctx)
}

// typedExternalClientWrapper wraps a TypedExternalClient to a common
// ExternalClient.
type typedExternalClientWrapper[managed resource.Managed] struct {
	c TypedExternalClient[managed]
}

func (c *typedExternalClientWrapper[managed]) Observe(ctx context.Context, mg resource.Managed) (ExternalObservation, error) {
	cr, ok := mg.(managed)
	if !ok {
		return ExternalObservation{}, errors.Errorf(errFmtUnexpectedObjectType, mg)
	}

	return c.c.Observe(ctx, cr)
}

func (c *typedExternalClientWrapper[managed]) Create(ctx context.Context, mg resource.Managed) (ExternalCreation, error) {
	cr, ok := mg.(managed)
	if !ok {
		return ExternalCreation{}, errors.Errorf(errFmtUnexpectedObjectType, mg)
	}

	return c.c.Create(ctx, cr)
}

func (c *typedExternalClientWrapper[managed]) Update(ctx context.Context, mg resource.Managed) (ExternalUpdate, error) {
	cr, ok := mg.(managed)
	if !ok {
		return ExternalUpdate{}, errors.Errorf(errFmtUnexpectedObjectType, mg)
	}

	return c.c.Update(ctx, cr)
}

func (c *typedExternalClientWrapper[managed]) Delete(ctx context.Context, mg resource.Managed) (ExternalDelete, error) {
	cr, ok := mg.(managed)
	if !ok {
		return ExternalDelete{}, errors.Errorf(errFmtUnexpectedObjectType, mg)
	}

	return c.c.Delete(ctx, cr)
}

func (c *typedExternalClientWrapper[managed]) Disconnect(ctx context.Context) error {
	return c.c.Disconnect(ctx)
}
