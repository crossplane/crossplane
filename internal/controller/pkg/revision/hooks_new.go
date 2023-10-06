package revision

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/xpkg"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ManifestBuilder interface {
	ServiceAccount(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount
	Deployment(serviceAccount string, overrides ...DeploymentOverrides) *appsv1.Deployment
	Service() *corev1.Service
	TLSServerSecret() *corev1.Secret
	TLSClientSecret() *corev1.Secret
}

type ProviderHooksNew struct {
	client resource.ClientApplicator
}

func NewProviderHooksNew(client resource.ClientApplicator) *ProviderHooksNew {
	return &ProviderHooksNew{
		client: client,
	}
}

func (h *ProviderHooksNew) Pre(ctx context.Context, pkg runtime.Object, pr v1.PackageWithRuntimeRevision) error {
	//TODO implement me
	panic("implement me")
}

func (h *ProviderHooksNew) Post(ctx context.Context, pkg runtime.Object, pr v1.PackageWithRuntimeRevision, manifests ManifestBuilder) error {
	po, _ := xpkg.TryConvert(pkg, &pkgmetav1.Provider{})
	pkgProvider, ok := po.(*pkgmetav1.Provider)
	if !ok {
		return errors.New("not a provider package")
	}
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	sa := manifests.ServiceAccount()
	// TODO(turkenh): Create/Apply SA

	_ = manifests.Deployment(sa.Name, providerDeploymentOverrides(pkgProvider, pr)...)
	// TODO(turkenh): Create/Apply Deployment

	return nil
}

func (h *ProviderHooksNew) Deactivate(ctx context.Context, pr v1.PackageWithRuntimeRevision) error {
	//TODO implement me
	panic("implement me")
}

type FunctionHooksNew struct {
	client resource.ClientApplicator
}

func NewFunctionHooksNew(client resource.ClientApplicator) *FunctionHooksNew {
	return &FunctionHooksNew{
		client: client,
	}
}

func (h *FunctionHooksNew) Pre(ctx context.Context, pkg runtime.Object, pr v1.PackageWithRuntimeRevision) error {
	//TODO implement me
	panic("implement me")
}

func (h *FunctionHooksNew) Post(ctx context.Context, pkg runtime.Object, pr v1.PackageWithRuntimeRevision, manifests ManifestBuilder) error {
	po, _ := xpkg.TryConvert(pkg, &pkgmetav1beta1.Function{})
	pkgFunction, ok := po.(*pkgmetav1beta1.Function)
	if !ok {
		return errors.New(errNotFunction)
	}
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	sa := manifests.ServiceAccount()
	// TODO(turkenh): Create/Apply SA

	_ = manifests.Deployment(sa.Name, functionDeploymentOverrides(pkgFunction, pr)...)
	// TODO(turkenh): Create/Apply Deployment

	return nil
}

func (h *FunctionHooksNew) Deactivate(ctx context.Context, pr v1.PackageWithRuntimeRevision) error {
	//TODO implement me
	panic("implement me")
}
