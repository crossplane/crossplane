package revision

import (
	"context"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/initializer"
	"github.com/crossplane/crossplane/internal/xpkg"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ManifestBuilder interface {
	ServiceAccount(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount
	Deployment(serviceAccount string, overrides ...DeploymentOverrides) *appsv1.Deployment
	Service(overrides ...ServiceOverrides) *corev1.Service
	TLSClientSecret() *corev1.Secret
	TLSServerSecret() *corev1.Secret
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

func (h *ProviderHooksNew) Deactivate(ctx context.Context, pr v1.PackageWithRuntimeRevision, manifests ManifestBuilder) error {
	sa := manifests.ServiceAccount()
	// Delete the service account if it exists.
	if err := h.client.Delete(ctx, sa); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderSA)
	}

	// Delete the deployment if it exists.
	// Different from the Post hook, we don't need to pass the
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

type FunctionHooksNew struct {
	client resource.ClientApplicator
}

func NewFunctionHooksNew(client resource.ClientApplicator) *FunctionHooksNew {
	return &FunctionHooksNew{
		client: client,
	}
}

func (h *FunctionHooksNew) Pre(ctx context.Context, pkg runtime.Object, pr v1.PackageWithRuntimeRevision, manifests ManifestBuilder) error {
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

func (h *FunctionHooksNew) Deactivate(ctx context.Context, pr v1.PackageWithRuntimeRevision, manifests ManifestBuilder) error {
	sa := manifests.ServiceAccount()
	// Delete the service account if it exists.
	if err := h.client.Delete(ctx, sa); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderSA)
	}

	// Delete the deployment if it exists.
	// Different from the Post hook, we don't need to pass the
	// "functionDeploymentOverrides()" here, because we're only interested
	// in the name and namespace of the deployment to delete it.
	if err := h.client.Delete(ctx, manifests.Deployment(sa.Name)); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderDeployment)
	}

	// NOTE(ezgidemirel): Service and secret are created per package. Therefore,
	// we're not deleting them here.
	return nil
}
