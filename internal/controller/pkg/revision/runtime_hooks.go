package revision

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/initializer"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errNotProvider                            = "not a provider package"
	errNotProviderRevision                    = "not a provider revision"
	errGetControllerConfig                    = "cannot get referenced controller config"
	errGetServiceAccount                      = "cannot get Crossplane service account"
	errDeleteProviderDeployment               = "cannot delete provider package deployment"
	errDeleteProviderSA                       = "cannot delete provider package service account"
	errDeleteProviderService                  = "cannot delete provider package service"
	errApplyProviderDeployment                = "cannot apply provider package deployment"
	errApplyProviderSecret                    = "cannot apply provider package secret"
	errApplyProviderSA                        = "cannot apply provider package service account"
	errApplyProviderService                   = "cannot apply provider package service"
	errFmtUnavailableProviderDeployment       = "provider package deployment is unavailable with message: %s"
	errNoAvailableConditionProviderDeployment = "provider package deployment has no condition of type \"Available\" yet"

	errNotFunction                            = "not a function package"
	errDeleteFunctionDeployment               = "cannot delete function package deployment"
	errDeleteFunctionSA                       = "cannot delete function package service account"
	errApplyFunctionDeployment                = "cannot apply function package deployment"
	errApplyFunctionSecret                    = "cannot apply function package secret"
	errApplyFunctionSA                        = "cannot apply function package service account"
	errApplyFunctionService                   = "cannot apply function package service"
	errFmtUnavailableFunctionDeployment       = "function package deployment is unavailable with message: %s"
	errNoAvailableConditionFunctionDeployment = "function package deployment has no condition of type \"Available\" yet"
)

type ManifestBuilder interface {
	ServiceAccount(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount
	Deployment(serviceAccount string, overrides ...DeploymentOverrides) *appsv1.Deployment
	Service(overrides ...ServiceOverrides) *corev1.Service
	TLSClientSecret() *corev1.Secret
	TLSServerSecret() *corev1.Secret
}

// A RuntimeHooks performs runtime operations before and after a revision
// establishes objects.
type RuntimeHooks interface {
	// Pre performs operations meant to happen before establishing objects.
	Pre(context.Context, runtime.Object, v1.PackageWithRuntimeRevision, ManifestBuilder) error

	// Post performs operations meant to happen after establishing objects.
	Post(context.Context, runtime.Object, v1.PackageWithRuntimeRevision, ManifestBuilder) error

	// Deactivate performs operations meant to happen before deactivating a revision.
	Deactivate(context.Context, v1.PackageWithRuntimeRevision, ManifestBuilder) error
}

type ProviderHooks struct {
	client resource.ClientApplicator
}

func NewProviderHooks(client resource.ClientApplicator) *ProviderHooks {
	return &ProviderHooks{
		client: client,
	}
}

func (h *ProviderHooks) Pre(ctx context.Context, pkg runtime.Object, pr v1.PackageWithRuntimeRevision, manifests ManifestBuilder) error {
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
	svc := manifests.Service()
	if err := h.client.Apply(ctx, svc); err != nil {
		return errors.Wrap(err, errApplyProviderService)
	}

	secClient := manifests.TLSClientSecret()
	secServer := manifests.TLSServerSecret()

	if secClient == nil || secServer == nil {
		// We should wait for the provider revision reconciler to set the secret
		// names before proceeding creating the TLS secrets
		return nil
	}

	if err := h.client.Apply(ctx, secClient); err != nil {
		return errors.Wrap(err, errApplyProviderSecret)
	}
	if err := h.client.Apply(ctx, secServer); err != nil {
		return errors.Wrap(err, errApplyProviderSecret)
	}

	if err := initializer.NewTLSCertificateGenerator(secClient.Namespace, initializer.RootCACertSecretName,
		initializer.TLSCertificateGeneratorWithOwner(pr.GetOwnerReferences()),
		initializer.TLSCertificateGeneratorWithServerSecretName(secServer.GetName(), initializer.DNSNamesForService(svc.Name, svc.Namespace)),
		initializer.TLSCertificateGeneratorWithClientSecretName(secClient.GetName(), []string{pr.GetName()})).Run(ctx, h.client); err != nil {
		return errors.Wrapf(err, "cannot generate TLS certificates for %q", pr.GetLabels()[v1.LabelParentPackage])
	}

	return nil
}

func (h *ProviderHooks) Post(ctx context.Context, pkg runtime.Object, pr v1.PackageWithRuntimeRevision, manifests ManifestBuilder) error {
	po, _ := xpkg.TryConvert(pkg, &pkgmetav1.Provider{})
	providerMeta, ok := po.(*pkgmetav1.Provider)
	if !ok {
		return errors.New("not a provider package")
	}
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	sa := manifests.ServiceAccount()
	if err := h.client.Apply(ctx, sa); err != nil {
		return errors.Wrap(err, errApplyProviderSA)
	}

	d := manifests.Deployment(sa.Name, providerDeploymentOverrides(providerMeta, pr)...)
	if err := h.client.Apply(ctx, d); err != nil {
		return errors.Wrap(err, errApplyProviderDeployment)
	}

	// TODO(phisco): check who is actually using this
	pr.SetControllerReference(v1.ControllerReference{Name: d.GetName()})

	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable {
			if c.Status == corev1.ConditionTrue {
				return nil
			}
			return errors.Errorf(errFmtUnavailableProviderDeployment, c.Message)
		}
	}
	return errors.New(errNoAvailableConditionProviderDeployment)
}

func (h *ProviderHooks) Deactivate(ctx context.Context, pr v1.PackageWithRuntimeRevision, manifests ManifestBuilder) error {
	sa := manifests.ServiceAccount()
	// Delete the service account if it exists.
	if err := h.client.Delete(ctx, sa); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderSA)
	}

	// Delete the deployment if it exists.
	// Different from the Post runtimeHook, we don't need to pass the
	// "providerDeploymentOverrides()" here, because we're only interested
	// in the name and namespace of the deployment to delete it.
	if err := h.client.Delete(ctx, manifests.Deployment(sa.Name)); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderDeployment)
	}

	// TODO(phisco): only added to cleanup the service we were previously
	// 	deploying for each provider revision, remove in a future release.
	svc := manifests.Service(ServiceWithName(pr.GetName()))
	if err := h.client.Delete(ctx, svc); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderService)
	}

	// NOTE(phisco): Service and TLS secrets are created per package. Therefore,
	// we're not deleting them here.
	return nil
}

type FunctionHooks struct {
	client resource.ClientApplicator
}

func NewFunctionHooks(client resource.ClientApplicator) *FunctionHooks {
	return &FunctionHooks{
		client: client,
	}
}

func (h *FunctionHooks) Pre(ctx context.Context, _ runtime.Object, pr v1.PackageWithRuntimeRevision, manifests ManifestBuilder) error {
	// TODO(ezgidemirel): update any status fields relevant to package revisions.

	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	// Ensure Prerequisites
	svc := manifests.Service(functionServiceOverrides()...)
	if err := h.client.Apply(ctx, svc); err != nil {
		return errors.Wrap(err, errApplyFunctionService)
	}

	// N.B.: We expect the revision to be applied by the caller
	fRev := pr.(*v1beta1.FunctionRevision)
	fRev.Status.Endpoint = fmt.Sprintf(serviceEndpointFmt, svc.Name, svc.Namespace, servicePort)

	secServer := manifests.TLSServerSecret()
	if err := h.client.Apply(ctx, secServer); err != nil {
		return errors.Wrap(err, errApplyFunctionSecret)
	}

	if err := initializer.NewTLSCertificateGenerator(secServer.Namespace, initializer.RootCACertSecretName,
		initializer.TLSCertificateGeneratorWithServerSecretName(secServer.GetName(), initializer.DNSNamesForService(svc.Name, svc.Namespace)),
		initializer.TLSCertificateGeneratorWithOwner([]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(pr, pr.GetObjectKind().GroupVersionKind()))})).Run(ctx, h.client); err != nil {
		return errors.Wrapf(err, "cannot generate TLS certificates for %q", pr.GetLabels()[v1.LabelParentPackage])
	}

	return nil
}

func (h *FunctionHooks) Post(ctx context.Context, pkg runtime.Object, pr v1.PackageWithRuntimeRevision, manifests ManifestBuilder) error {
	po, _ := xpkg.TryConvert(pkg, &pkgmetav1beta1.Function{})
	functionMeta, ok := po.(*pkgmetav1beta1.Function)
	if !ok {
		return errors.New(errNotFunction)
	}
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	sa := manifests.ServiceAccount()
	if err := h.client.Apply(ctx, sa); err != nil {
		return errors.Wrap(err, errApplyFunctionSA)
	}

	d := manifests.Deployment(sa.Name, functionDeploymentOverrides(functionMeta, pr)...)
	if err := h.client.Apply(ctx, d); err != nil {
		return errors.Wrap(err, errApplyFunctionDeployment)
	}

	// TODO(phisco): check who is actually using this
	pr.SetControllerReference(v1.ControllerReference{Name: d.GetName()})

	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable {
			if c.Status == corev1.ConditionTrue {
				return nil
			}
			return errors.Errorf(errFmtUnavailableFunctionDeployment, c.Message)
		}
	}
	return errors.New(errNoAvailableConditionFunctionDeployment)
}

func (h *FunctionHooks) Deactivate(ctx context.Context, _ v1.PackageWithRuntimeRevision, manifests ManifestBuilder) error {
	sa := manifests.ServiceAccount()
	// Delete the service account if it exists.
	if err := h.client.Delete(ctx, sa); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderSA)
	}

	// Delete the deployment if it exists.
	// Different from the Post runtimeHook, we don't need to pass the
	// "functionDeploymentOverrides()" here, because we're only interested
	// in the name and namespace of the deployment to delete it.
	if err := h.client.Delete(ctx, manifests.Deployment(sa.Name)); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderDeployment)
	}

	// NOTE(ezgidemirel): Service and secret are created per package. Therefore,
	// we're not deleting them here.
	return nil
}
