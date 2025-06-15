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

package runtime

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/controller/pkg/revision"
	"github.com/crossplane/crossplane/internal/initializer"
)

const (
	errDeleteProviderDeployment               = "cannot delete provider package deployment"
	errDeleteProviderService                  = "cannot delete provider package service"
	errApplyProviderDeployment                = "cannot apply provider package deployment"
	errApplyProviderSecret                    = "cannot apply provider package secret"
	errApplyProviderSA                        = "cannot apply provider package service account"
	errApplyProviderService                   = "cannot apply provider package service"
	errFmtUnavailableProviderDeployment       = "provider package deployment is unavailable with message: %s"
	errNoAvailableConditionProviderDeployment = "provider package deployment has no condition of type \"Available\" yet"
	errParseProviderImage                     = "cannot parse provider package image"
)

// ProviderHooks performs runtime operations for provider packages.
type ProviderHooks struct {
	client          resource.ClientApplicator
	defaultRegistry string
}

// NewProviderHooks returns a new ProviderHooks.
func NewProviderHooks(client client.Client, defaultRegistry string) *ProviderHooks {
	return &ProviderHooks{
		client: resource.ClientApplicator{
			Client:     client,
			Applicator: resource.NewAPIPatchingApplicator(client),
		},
		defaultRegistry: defaultRegistry,
	}
}

// Pre performs operations meant to happen before establishing objects.
func (h *ProviderHooks) Pre(ctx context.Context, pr v1.PackageRevisionWithRuntime, build ManifestBuilder) error {
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	// Ensure Prerequisites
	// Note(turkenh): We need certificates have generated when we get to the
	// establish step, i.e., we want to inject the CA to CRDs (webhook caBundle).
	// Therefore, we need to generate the certificates pre-establish and
	// generating certificates requires the service to be defined. This is why
	// we're creating the service here but service account and deployment in the
	// post-establish.
	svc := build.Service(ServiceWithAdditionalPorts([]corev1.ServicePort{
		{
			Name:       WebhookPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       revision.ServicePort,
			TargetPort: intstr.FromString(WebhookPortName),
		},
	}))
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
func (h *ProviderHooks) Post(ctx context.Context, pr v1.PackageRevisionWithRuntime, build ManifestBuilder) error {
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	sa := build.ServiceAccount()

	// Determine the function's image, taking into account the default registry.
	image, err := name.ParseReference(pr.GetResolvedSource(), name.WithDefaultRegistry(h.defaultRegistry))
	if err != nil {
		return errors.Wrap(err, errParseProviderImage)
	}
	d := build.Deployment(sa.Name, providerDeploymentOverrides(pr, image.Name())...)
	// Create/Apply the SA only if the deployment references it.
	// This is to avoid creating a SA that is not used by the deployment when
	// the SA is managed externally by the user and configured by setting
	// `deploymentTemplate.spec.template.spec.serviceAccountName` in the
	// DeploymentRuntimeConfig.
	if sa.Name == d.Spec.Template.Spec.ServiceAccountName {
		if err := applySA(ctx, h.client, sa); err != nil {
			return errors.Wrap(err, errApplyProviderSA)
		}
	}
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

	// NOTE(turkenh): We don't delete the service account here because it might
	// be used by other package revisions, e.g. user might have specified a
	// service account name in the runtime config. This should not be a problem
	// because we add the owner reference to the service account when we create
	// them, and they will be garbage collected when the package revision is
	// deleted if they are not used by any other package revisions.

	// NOTE(phisco): Service and TLS secrets are created per package. Therefore,
	// we're not deleting them here.
	return nil
}

func providerDeploymentOverrides(pr v1.PackageRevisionWithRuntime, image string) []DeploymentOverride {
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

		// Add optional scrape annotations to the deployment. It is possible to
		// disable the scraping by setting the annotation "prometheus.io/scrape"
		// as "false" in the DeploymentRuntimeConfig.
		DeploymentWithOptionalPodScrapeAnnotations(),
	}

	do = append(do, DeploymentRuntimeWithOptionalImage(image))

	if pr.GetTLSClientSecretName() != nil {
		do = append(do, DeploymentRuntimeWithAdditionalEnvironments([]corev1.EnvVar{
			// for backward compatibility with existing providers, we set the
			// environment variable ESS_TLS_CERTS_DIR to the same value as
			// TLS_CLIENT_CERTS_DIR to ease the transition to the new certificates.
			{
				Name:  ESSTLSCertDirEnvVar,
				Value: fmt.Sprintf("$(%s)", TLSClientCertDirEnvVar),
			},
		}))
	}

	if pr.GetTLSServerSecretName() != nil {
		do = append(do, DeploymentRuntimeWithAdditionalPorts([]corev1.ContainerPort{
			{
				Name:          WebhookPortName,
				ContainerPort: revision.ServicePort,
			},
		}), DeploymentRuntimeWithAdditionalEnvironments([]corev1.EnvVar{
			// for backward compatibility with existing providers, we set the
			// environment variable WEBHOOK_TLS_CERT_DIR to the same value as
			// TLS_SERVER_CERTS_DIR to ease the transition to the new certificates.
			{
				Name:  WebhookTLSCertDirEnvVar,
				Value: fmt.Sprintf("$(%s)", TLSServerCertDirEnvVar),
			},
		}))
	}

	return do
}

// applySA creates/updates a ServiceAccount and includes any image pull secrets
// that have been added by external controllers.
func applySA(ctx context.Context, cl resource.ClientApplicator, sa *corev1.ServiceAccount) error {
	oldSa := &corev1.ServiceAccount{}
	if err := cl.Get(ctx, types.NamespacedName{Name: sa.Name, Namespace: sa.Namespace}, oldSa); err == nil {
		// Add pull secrets created by other controllers
		existingSecrets := make(map[string]bool)
		for _, secret := range sa.ImagePullSecrets {
			existingSecrets[secret.Name] = true
		}

		for _, secret := range oldSa.ImagePullSecrets {
			if !existingSecrets[secret.Name] {
				sa.ImagePullSecrets = append(sa.ImagePullSecrets, secret)
			}
		}
	}
	return cl.Apply(ctx, sa)
}
