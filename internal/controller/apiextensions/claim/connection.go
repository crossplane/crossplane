package claim

import (
	"context"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// NopConnectionUnpublisher is a ConnectionUnpublisher that does nothing.
type NopConnectionUnpublisher struct{}

// NewNopConnectionUnpublisher returns a new NopConnectionUnpublisher
func NewNopConnectionUnpublisher() *NopConnectionUnpublisher {
	return &NopConnectionUnpublisher{}
}

// UnpublishConnection does nothing and returns no error with
// UnpublishConnection. Expected to be used where deletion of connection
// secret is already handled by K8s garbage collection and there is actually
// nothing to do to unpublish connection details.
func (n *NopConnectionUnpublisher) UnpublishConnection(_ context.Context, _ resource.LocalConnectionSecretOwner, _ managed.ConnectionDetails) error {
	return nil
}

// SecretStoreConnectionUnpublisher unpublishes secret store connection secrets.
type SecretStoreConnectionUnpublisher struct {
	// TODO(turkenh): Use a narrower interface, i.e. we don't need Publish
	//  method here. Please note we cannot use ConnectionUnpublisher interface
	//  defined in this package as it expects a LocalConnectionSecretOwner
	//  which is exactly what this struct is providing. Ideally, we should
	//  split managed.ConnectionPublisher as Publisher and Unpublisher, but I
	//  would like to leave this to a further PR after we graduate Secret Store
	//  and decide to clean up the old API.
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
