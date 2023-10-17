/*
Copyright 2023 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package revision

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/initializer"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errNotProvider                            = "not a provider package"
	errNotProviderRevision                    = "not a provider revision"
	errDeleteProviderDeployment               = "cannot delete provider package deployment"
	errDeleteProviderSA                       = "cannot delete provider package service account"
	errDeleteProviderService                  = "cannot delete provider package service"
	errApplyProviderDeployment                = "cannot apply provider package deployment"
	errApplyProviderSecret                    = "cannot apply provider package secret"
	errApplyProviderSA                        = "cannot apply provider package service account"
	errApplyProviderService                   = "cannot apply provider package service"
	errFmtUnavailableProviderDeployment       = "provider package deployment is unavailable with message: %s"
	errNoAvailableConditionProviderDeployment = "provider package deployment has no condition of type \"Available\" yet"
)

// ProviderHooks performs runtime operations for provider packages.
type ProviderHooks struct {
	client resource.ClientApplicator
}

// NewProviderHooks returns a new ProviderHooks.
func NewProviderHooks(client client.Client) *ProviderHooks {
	return &ProviderHooks{
		client: resource.ClientApplicator{
			Client:     client,
			Applicator: resource.NewAPIPatchingApplicator(client),
		},
	}
}

// Pre performs operations meant to happen before establishing objects.
func (h *ProviderHooks) Pre(ctx context.Context, pkg runtime.Object, pr v1.PackageRevisionWithRuntime, build ManifestBuilder) error {
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
	// Note(turkenh): We need certificates have generated when we get to the
	// establish step, i.e. we want to inject the CA to CRDs (webhook caBundle).
	// Therefore, we need to generate the certificates pre establish and
	// generating certificates requires the service to be defined. This is why
	// we're creating the service here but service account and deployment in the
	// post establish.
	// As a rule of thumb, we create objects named after the package in the
	// pre hook and objects named after the package revision in the post hook.
	svc := build.Service(ServiceWithSelectors(providerSelectors(providerMeta, pr)))
	if err := h.client.Apply(ctx, svc); err != nil {
		return errors.Wrap(err, errApplyProviderService)
	}

	secClient := build.TLSClientSecret()
	secServer := build.TLSServerSecret()

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

// Post performs operations meant to happen after establishing objects.
func (h *ProviderHooks) Post(ctx context.Context, pkg runtime.Object, pr v1.PackageRevisionWithRuntime, build ManifestBuilder) error {
	po, _ := xpkg.TryConvert(pkg, &pkgmetav1.Provider{})
	providerMeta, ok := po.(*pkgmetav1.Provider)
	if !ok {
		return errors.New("not a provider package")
	}
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	sa := build.ServiceAccount()
	if err := h.client.Apply(ctx, sa); err != nil {
		return errors.Wrap(err, errApplyProviderSA)
	}

	d := build.Deployment(sa.Name, providerDeploymentOverrides(providerMeta, pr)...)
	if err := h.client.Apply(ctx, d); err != nil {
		return errors.Wrap(err, errApplyProviderDeployment)
	}

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

// Deactivate performs operations meant to happen before deactivating a revision.
func (h *ProviderHooks) Deactivate(ctx context.Context, pr v1.PackageRevisionWithRuntime, build ManifestBuilder) error {
	sa := build.ServiceAccount()
	// Delete the service account if it exists.
	if err := h.client.Delete(ctx, sa); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderSA)
	}

	// Delete the deployment if it exists.
	// Different from the Post runtimeHook, we don't need to pass the
	// "providerDeploymentOverrides()" here, because we're only interested
	// in the name and namespace of the deployment to delete it.
	if err := h.client.Delete(ctx, build.Deployment(sa.Name)); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderDeployment)
	}

	// TODO(phisco): only added to cleanup the service we were previously
	// 	deploying for each provider revision, remove in a future release.
	svc := build.Service(ServiceWithName(pr.GetName()))
	if err := h.client.Delete(ctx, svc); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errDeleteProviderService)
	}

	// NOTE(phisco): Service and TLS secrets are created per package. Therefore,
	// we're not deleting them here.
	return nil
}

func providerDeploymentOverrides(providerMeta *pkgmetav1.Provider, pr v1.PackageRevisionWithRuntime) []DeploymentOverride {
	do := []DeploymentOverride{
		DeploymentRuntimeWithAdditionalEnvironments([]corev1.EnvVar{
			{
				// NOTE(turkenh): POD_NAMESPACE is needed to
				// set a default scope/namespace of the
				// default StoreConfig, similar to init
				// container of Core Crossplane.
				Name: "POD_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
		}),

		// Note(turkenh): By default, in manifest builder, we're setting the
		// provider name in the selector using the package name, derived from
		// "v1.LabelParentPackage". However, we used to set the provider name in
		// the selector using the provider "meta" name. Since we cannot change
		// the selector in-place, this would require a migration. We're keeping
		// the old selector for backward compatibility with existing providers
		// and plan to remove this after implementing a migration in a future
		// release.
		DeploymentWithSelectors(providerSelectors(providerMeta, pr)),
	}

	image := pr.GetSource()
	if providerMeta.Spec.Controller.Image != nil {
		image = *providerMeta.Spec.Controller.Image
	}
	do = append(do, DeploymentRuntimeWithOptionalImage(image))

	if pr.GetTLSClientSecretName() != nil {
		do = append(do, DeploymentRuntimeWithAdditionalEnvironments([]corev1.EnvVar{
			// for backward compatibility with existing providers, we set the
			// environment variable ESS_TLS_CERTS_DIR to the same value as
			// TLS_CLIENT_CERTS_DIR to ease the transition to the new certificates.
			{
				Name:  essTLSCertDirEnvVar,
				Value: fmt.Sprintf("$(%s)", tlsClientCertDirEnvVar),
			},
		}))
	}

	if pr.GetTLSServerSecretName() != nil {
		do = append(do, DeploymentRuntimeWithAdditionalPorts([]corev1.ContainerPort{
			{
				Name:          webhookPortName,
				ContainerPort: servicePort,
			},
		}), DeploymentRuntimeWithAdditionalEnvironments([]corev1.EnvVar{
			// for backward compatibility with existing providers, we set the
			// environment variable WEBHOOK_TLS_CERT_DIR to the same value as
			// TLS_SERVER_CERTS_DIR to ease the transition to the new certificates.
			{
				Name:  webhookTLSCertDirEnvVar,
				Value: fmt.Sprintf("$(%s)", tlsServerCertDirEnvVar),
			},
		}))
	}

	return do
}

func providerSelectors(providerMeta *pkgmetav1.Provider, pr v1.PackageRevisionWithRuntime) map[string]string {
	return map[string]string{
		"pkg.crossplane.io/revision": pr.GetName(),
		"pkg.crossplane.io/provider": providerMeta.GetName(),
	}
}
