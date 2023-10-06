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
	Service(overrides ...ServiceOverrides) *corev1.Service
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

func (h *ProviderHooksNew) Pre(ctx context.Context, pkg runtime.Object, pr v1.PackageWithRuntimeRevision, manifests ManifestBuilder) error {
	po, _ := xpkg.TryConvert(pkg, &pkgmetav1.Provider{})
	providerMeta, ok := po.(*pkgmetav1.Provider)
	if !ok {
		return errors.New(errNotProvider)
	}

	provRev, ok := pr.(*v1.ProviderRevision)
	if !ok {
		return errors.New(errNotProviderRevision)
	}

	provRev.Status.PermissionRequests = providerMeta.Spec.Controller.PermissionRequests

	// TODO(hasheddan): update any status fields relevant to package revisions.

	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	// Ensure Prerequisites
	_ = manifests.Service()
	// TODO(turkenh): Create/Apply Service

	return nil
}

func (h *ProviderHooksNew) Post(ctx context.Context, pkg runtime.Object, pr v1.PackageWithRuntimeRevision, manifests ManifestBuilder) error {
	po, _ := xpkg.TryConvert(pkg, &pkgmetav1.Provider{})
	providerMeta, ok := po.(*pkgmetav1.Provider)
	if !ok {
		return errors.New("not a provider package")
	}
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	sa := manifests.ServiceAccount()
	// TODO(turkenh): Create/Apply SA

	_ = manifests.Deployment(sa.Name, providerDeploymentOverrides(providerMeta, pr)...)
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

func (h *FunctionHooksNew) Pre(ctx context.Context, pkg runtime.Object, pr v1.PackageWithRuntimeRevision, manifests ManifestBuilder) error {
	fo, _ := xpkg.TryConvert(pkg, &pkgmetav1beta1.Function{})
	_, ok := fo.(*pkgmetav1beta1.Function)
	if !ok {
		return errors.New(errNotFunction)
	}

	// TODO(ezgidemirel): update any status fields relevant to package revisions.

	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	// Ensure Prerequisites
	_ = manifests.Service(functionServiceOverrides()...)
	// TODO(turkenh): Create/Apply Service

	return nil
}

func (h *FunctionHooksNew) Post(ctx context.Context, pkg runtime.Object, pr v1.PackageWithRuntimeRevision, manifests ManifestBuilder) error {
	po, _ := xpkg.TryConvert(pkg, &pkgmetav1beta1.Function{})
	functionMeta, ok := po.(*pkgmetav1beta1.Function)
	if !ok {
		return errors.New(errNotFunction)
	}
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	sa := manifests.ServiceAccount()
	// TODO(turkenh): Create/Apply SA

	_ = manifests.Deployment(sa.Name, functionDeploymentOverrides(functionMeta, pr)...)
	// TODO(turkenh): Create/Apply Deployment

	return nil
}

func (h *FunctionHooksNew) Deactivate(ctx context.Context, pr v1.PackageWithRuntimeRevision) error {
	//TODO implement me
	panic("implement me")
}
