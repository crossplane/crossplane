package claim

import (
	"context"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// NOPConnectionUnpublisher is a ConnectionUnpublisher that does nothing.
type NOPConnectionUnpublisher struct{}

// NewNOPConnectionUnpublisher returns a new NOPConnectionUnpublisher
func NewNOPConnectionUnpublisher() *NOPConnectionUnpublisher {
	return &NOPConnectionUnpublisher{}
}

// UnpublishConnection does nothing and returns no error with
// UnpublishConnection. Expected to be used where deletion of connection
// secret is already handled by K8s garbage collection and there is actually
// nothing to do to unpublish connection details.
func (n *NOPConnectionUnpublisher) UnpublishConnection(_ context.Context, _ resource.LocalConnectionSecretOwner, _ managed.ConnectionDetails) error {
	return nil
}

// SecretStoreConnectionUnpublisher unpublishes secret store connection secrets.
type SecretStoreConnectionUnpublisher struct {
	publisher managed.ConnectionPublisher
}

// NewSecretStoreConnectionUnpublisher returns a new SecretStoreConnectionUnpublisher.
func NewSecretStoreConnectionUnpublisher(p managed.ConnectionPublisher) *SecretStoreConnectionUnpublisher {
	return &SecretStoreConnectionUnpublisher{
		publisher: p,
	}
}

// UnpublishConnection details for the supplied Managed resource.
func (u *SecretStoreConnectionUnpublisher) UnpublishConnection(ctx context.Context, so resource.LocalConnectionSecretOwner, c managed.ConnectionDetails) error {
	return u.publisher.UnpublishConnection(ctx, newClaimAsSecretOwner(so), c)
}

// soClaim is a type that enables using claim type with Secret Store
// UnpublishConnection method by satisfyng resource.ConnectionSecretOwner
// interface.
type soClaim struct {
	resource.LocalConnectionSecretOwner
}

func newClaimAsSecretOwner(lo resource.LocalConnectionSecretOwner) *soClaim {
	return &soClaim{
		LocalConnectionSecretOwner: lo,
	}
}

func (s soClaim) SetWriteConnectionSecretToReference(_ *xpv1.SecretReference) {}

func (s soClaim) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	// SecretStoreConnectionUnpublisher does not use
	// WriteConnectionSecretToReference interface, so, we are implementing
	// just to satisfy resource.ConnectionSecretOwner interface.
	return nil
}
