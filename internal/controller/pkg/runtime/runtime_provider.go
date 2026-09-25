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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	extv1alpha1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	pkgmetav1 "github.com/crossplane/crossplane/apis/v2/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/controller/pkg/revision"
	"github.com/crossplane/crossplane/v2/internal/initializer"
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
	errMigrateProviderDeployment              = "cannot migrate provider package deployment selector"
	errListMRDs                               = "cannot list ManagedResourceDefinitions to determine whether the provider runtime can start"

	msgAwaitingActivation = "Package runtime is scaled to zero; awaiting the first ManagedResourceDefinition to be activated"
)

// ProviderHooks performs runtime operations for provider packages.
type ProviderHooks struct {
	client resource.ClientApplicator

	// namespace is the namespace in which runtime objects are created.
	namespace string
	// coreServiceAccount is the name of the core Crossplane ServiceAccount. We
	// propagate its image pull secrets to the runtime ServiceAccount.
	coreServiceAccount string

	migrator DeploymentSelectorMigrator

	conditions conditions.Manager
}

// NewProviderHooks returns a new ProviderHooks.
func NewProviderHooks(c client.Client, namespace, coreServiceAccount string, m DeploymentSelectorMigrator) *ProviderHooks {
	if m == nil {
		m = NewNopDeploymentSelectorMigrator()
	}

	return &ProviderHooks{
		client: resource.ClientApplicator{
			Client:     c,
			Applicator: resource.NewAPIPatchingApplicator(c),
		},
		namespace:          namespace,
		coreServiceAccount: coreServiceAccount,
		migrator:           m,
		conditions:         conditions.ObservedGenerationPropagationManager{},
	}
}

// Pre performs operations meant to happen before establishing objects.
func (h *ProviderHooks) Pre(ctx context.Context, pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error {
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	// Migrate the deployment selector if needed. This has to happen before we
	// apply anything, so that a deployment with an outdated selector is deleted
	// and recreated by the post-establish step. Only the deployment's name and
	// namespace matter here.
	sa := h.serviceAccount(pr, rc, nil)
	if err := h.migrator.MigrateDeploymentSelector(ctx, pr, h.deployment(pr, rc, sa.Name, "", nil, false)); err != nil {
		return errors.Wrap(err, errMigrateProviderDeployment)
	}

	pr.SetObservedTLSServerSecretName(pr.GetTLSServerSecretName())
	pr.SetObservedTLSClientSecretName(pr.GetTLSClientSecretName())

	// Ensure Prerequisites
	// Note(turkenh): We need certificates have generated when we get to the
	// establish step, i.e., we want to inject the CA to CRDs (webhook caBundle).
	// Therefore, we need to generate the certificates pre-establish and
	// generating certificates requires the service to be defined. This is why
	// we're creating the service here but service account and deployment in the
	// post-establish.
	svc := h.service(pr, rc)
	if err := applySharedRuntimeObject(ctx, h.client.Client, pr, svc); err != nil {
		return errors.Wrap(err, errApplyProviderService)
	}

	secClient := h.tlsClientSecret(pr)
	secServer := h.tlsServerSecret(pr)

	if secClient == nil || secServer == nil {
		// We should wait for the provider revision reconciler to set the secret
		// names before proceeding creating the TLS secrets
		return nil
	}

	if err := applySharedRuntimeObject(ctx, h.client.Client, pr, secClient); err != nil {
		return errors.Wrap(err, errApplyProviderSecret)
	}

	if err := applySharedRuntimeObject(ctx, h.client.Client, pr, secServer); err != nil {
		return errors.Wrap(err, errApplyProviderSecret)
	}

	owner := meta.AsController(meta.TypedReferenceTo(pr, pr.GetObjectKind().GroupVersionKind()))
	if err := initializer.NewTLSCertificateGenerator(secClient.Namespace, initializer.RootCACertSecretName,
		initializer.TLSCertificateGeneratorWithOwner([]metav1.OwnerReference{owner}),
		initializer.TLSCertificateGeneratorWithServerSecretName(secServer.GetName(), initializer.DNSNamesForService(svc.Name, svc.Namespace)),
		initializer.TLSCertificateGeneratorWithClientSecretName(secClient.GetName(), []string{pr.GetName()})).Run(ctx, h.client.Client); err != nil {
		return errors.Wrapf(err, "cannot generate TLS certificates for %q", pr.GetLabels()[v1.LabelParentPackage])
	}

	return nil
}

// Post performs operations meant to happen after establishing objects.
func (h *ProviderHooks) Post(ctx context.Context, pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error {
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	saPullSecrets, err := corePullSecrets(ctx, h.client.Client, h.namespace, h.coreServiceAccount)
	if err != nil {
		return err
	}

	pullSecrets, err := imageConfigPullSecrets(ctx, h.client.Client, pr)
	if err != nil {
		return err
	}

	mrds, err := h.ownedMRDs(ctx, pr)
	if err != nil {
		return err
	}

	awaiting := awaitingActivation(pr, mrds)

	sa := h.serviceAccount(pr, rc, saPullSecrets)

	// Determine the provider's image.
	image, err := name.ParseReference(pr.GetResolvedSource(), name.StrictValidation)
	if err != nil {
		return errors.Wrap(err, errParseProviderImage)
	}

	d := h.deployment(pr, rc, sa.Name, image.Name(), pullSecrets, awaiting)
	// Create/Apply the SA only if the deployment references it.
	// This is to avoid creating a SA that is not used by the deployment when
	// the SA is managed externally by the user and configured by setting
	// `deploymentTemplate.spec.template.spec.serviceAccountName` in the
	// DeploymentRuntimeConfig.
	if sa.Name == d.Spec.Template.Spec.ServiceAccountName {
		if err := applySA(ctx, h.client, pr, sa); err != nil {
			return errors.Wrap(err, errApplyProviderSA)
		}
	}

	if err := applyRuntimeObject(ctx, h.client.Client, d); err != nil {
		return errors.Wrap(err, errApplyProviderDeployment)
	}

	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable {
			if c.Status != corev1.ConditionTrue {
				return errors.Errorf(errFmtUnavailableProviderDeployment, c.Message)
			}

			if awaiting {
				h.conditions.For(pr).MarkConditions(v1.RuntimeHealthy(), v1.RuntimeAwaitingActivation().WithMessage(msgAwaitingActivation))
			} else {
				h.conditions.For(pr).MarkConditions(v1.RuntimeHealthy(), v1.RuntimeActive())
			}

			return nil
		}
	}

	return errors.New(errNoAvailableConditionProviderDeployment)
}

// Deactivate performs operations meant to happen before deactivating a revision.
func (h *ProviderHooks) Deactivate(ctx context.Context, pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error {
	// We're only interested in the name and namespace of the deployment in
	// order to delete it, so we don't bother resolving the image, the pull
	// secrets or the replica count here.
	sa := h.serviceAccount(pr, rc, nil)
	if err := deleteRuntimeObjectControlledBy(ctx, h.client.Client, pr, h.deployment(pr, rc, sa.Name, "", nil, false)); err != nil {
		return errors.Wrap(err, errDeleteProviderDeployment)
	}

	// TODO(phisco): only added to cleanup the service we were previously
	// 	deploying for each provider revision, remove in a future release.
	// 	Only the name and namespace matter in order to delete it.
	svc := h.service(pr, rc)
	svc.SetName(pr.GetName())

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

	// NOTE(jbw976): We leave our owner references on those shared objects alone, controlling flag
	// included. The revision taking over demotes us as part of claiming them, which keeps the
	// handover to a single writer.
	return nil
}

// serviceAccount builds the ServiceAccount of a provider revision's runtime.
// The supplied pull secrets are appended to the revision's own.
func (h *ProviderHooks) serviceAccount(pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig, pullSecrets []corev1.LocalObjectReference) *corev1.ServiceAccount {
	sa := &corev1.ServiceAccount{}
	if rc != nil {
		sa = serviceAccountFromRuntimeConfig(rc.Spec.ServiceAccountTemplate)
	}

	sa.TypeMeta = metav1.TypeMeta{
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind:       "ServiceAccount",
	}

	for _, o := range []ServiceAccountOverride{
		// Optional defaults, will be used only if the runtime config does not
		// specify them.
		ServiceAccountWithOptionalName(pr.GetName()),

		// Overrides that we are opinionated about.
		ServiceAccountWithNamespace(h.namespace),
		ServiceAccountWithOwnerReferences([]metav1.OwnerReference{h.owner(pr)}),
		ServiceAccountWithAdditionalPullSecrets(append(pr.GetPackagePullSecrets(), pullSecrets...)),
	} {
		o(sa)
	}

	return sa
}

// deployment builds the Deployment of a provider revision's runtime.
func (h *ProviderHooks) deployment(pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig, serviceAccount, image string, pullSecrets []string, awaitingActivation bool) *appsv1.Deployment {
	d := &appsv1.Deployment{}
	if rc != nil {
		d = deploymentFromRuntimeConfig(rc.Spec.DeploymentTemplate)
	}

	overrides := []DeploymentOverride{
		// This will ensure that the runtime container exists and always the
		// first one.
		DeploymentWithRuntimeContainer(),

		// Optional defaults, will be used only if the runtime config does not
		// specify them.
		DeploymentWithOptionalName(pr.GetName()),
		DeploymentWithOptionalReplicas(1),
		DeploymentWithOptionalPodSecurityContext(&corev1.PodSecurityContext{
			RunAsNonRoot: &RunAsNonRoot,
			RunAsUser:    &RunAsUser,
			RunAsGroup:   &RunAsGroup,
		}),
		DeploymentRuntimeWithOptionalImagePullPolicy(corev1.PullIfNotPresent),
		DeploymentRuntimeWithOptionalSecurityContext(&corev1.SecurityContext{
			RunAsUser:                &RunAsUser,
			RunAsGroup:               &RunAsGroup,
			AllowPrivilegeEscalation: &AllowPrivilegeEscalation,
			Privileged:               &Privileged,
			RunAsNonRoot:             &RunAsNonRoot,
		}),
		DeploymentWithOptionalServiceAccount(serviceAccount),

		// Overrides that we are opinionated about.
		DeploymentWithNamespace(h.namespace),
		DeploymentWithOwnerReferences([]metav1.OwnerReference{h.owner(pr)}),
		DeploymentWithSelectors(h.podSelectors(pr)),
		DeploymentWithImagePullSecrets(pr.GetPackagePullSecrets()),
		DeploymentRuntimeWithAdditionalPorts([]corev1.ContainerPort{
			{
				Name:          MetricsPortName,
				ContainerPort: MetricsPortNumber,
			},
		}),
	}

	if awaitingActivation {
		// Scale the runtime to zero while awaiting activation, overriding any
		// replica count from the deployment runtime config. A provider only
		// asks for multiple replicas for leader-election standby or webhook
		// redundancy, neither of which matters while none of its managed
		// resources are being reconciled.
		overrides = append(overrides, DeploymentWithReplicas(0))
	}

	for _, s := range pullSecrets {
		overrides = append(overrides, DeploymentWithAdditionalPullSecret(corev1.LocalObjectReference{Name: s}))
	}

	if pr.GetPackagePullPolicy() != nil {
		// If the package pull policy is set, it will override the default
		// or whatever is set in the runtime config.
		overrides = append(overrides, DeploymentRuntimeWithImagePullPolicy(*pr.GetPackagePullPolicy()))
	}

	if pr.GetObservedTLSClientSecretName() != nil {
		overrides = append(overrides, DeploymentRuntimeWithTLSClientSecret(*pr.GetObservedTLSClientSecretName()))
	}

	if pr.GetObservedTLSServerSecretName() != nil {
		overrides = append(overrides, DeploymentRuntimeWithTLSServerSecret(*pr.GetObservedTLSServerSecretName()))
	}

	// Provider specific overrides. They go last so that they win.
	overrides = append(overrides,
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
			{
				Name: "PROVIDER_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: fmt.Sprintf("metadata.labels['%s']", v1.LabelProvider),
					},
				},
			},
			{
				Name: "REVISION_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: fmt.Sprintf("metadata.labels['%s']", v1.LabelRevision),
					},
				},
			},
			{
				Name:  "REVISION_UID",
				Value: string(pr.GetUID()),
			},
		}),

		// Add optional scrape annotations to the deployment. It is possible to
		// disable the scraping by setting the annotation "prometheus.io/scrape"
		// as "false" in the DeploymentRuntimeConfig.
		DeploymentWithOptionalPodScrapeAnnotations(),

		DeploymentRuntimeWithOptionalImage(image),
	)

	if pr.GetObservedTLSServerSecretName() != nil {
		overrides = append(overrides, DeploymentRuntimeWithAdditionalPorts([]corev1.ContainerPort{
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

	for _, o := range overrides {
		o(d)
	}

	d.TypeMeta = metav1.TypeMeta{
		APIVersion: appsv1.SchemeGroupVersion.String(),
		Kind:       "Deployment",
	}

	return d
}

// service builds the Service of a provider revision's runtime. It is shared by
// all revisions of a provider.
func (h *ProviderHooks) service(pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) *corev1.Service {
	svc := &corev1.Service{}
	if rc != nil {
		svc = serviceFromRuntimeConfig(rc.Spec.ServiceTemplate)
	}

	svc.TypeMeta = metav1.TypeMeta{
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind:       "Service",
	}

	for _, o := range []ServiceOverride{
		// Optional defaults, will be used only if the runtime config does not
		// specify them.
		ServiceWithOptionalName(h.packageName(pr)),

		// Overrides that we are opinionated about.
		ServiceWithNamespace(h.namespace),
		ServiceWithOwnerReferences([]metav1.OwnerReference{h.owner(pr)}),
		ServiceWithSelectors(h.podSelectors(pr)),

		// Provider specific overrides. They go last so that they win.
		ServiceWithAdditionalPorts([]corev1.ServicePort{
			{
				Name:       WebhookPortName,
				Protocol:   corev1.ProtocolTCP,
				Port:       revision.ServicePort,
				TargetPort: intstr.FromString(WebhookPortName),
			},
		}),
	} {
		o(svc)
	}

	return svc
}

// tlsClientSecret builds the Secret holding a provider revision's TLS client
// certificate. It returns nil until the package manager has named the secret.
func (h *ProviderHooks) tlsClientSecret(pr v1.PackageRevisionWithRuntime) *corev1.Secret {
	if pr.GetObservedTLSClientSecretName() == nil {
		return nil
	}

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            *pr.GetObservedTLSClientSecretName(),
			Namespace:       h.namespace,
			OwnerReferences: []metav1.OwnerReference{h.owner(pr)},
		},
	}
}

// tlsServerSecret builds the Secret holding a provider revision's TLS server
// certificate. It returns nil until the package manager has named the secret.
func (h *ProviderHooks) tlsServerSecret(pr v1.PackageRevisionWithRuntime) *corev1.Secret {
	if pr.GetObservedTLSServerSecretName() == nil {
		return nil
	}

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            *pr.GetObservedTLSServerSecretName(),
			Namespace:       h.namespace,
			OwnerReferences: []metav1.OwnerReference{h.owner(pr)},
		},
	}
}

func (h *ProviderHooks) owner(pr v1.PackageRevisionWithRuntime) metav1.OwnerReference {
	return meta.AsController(meta.TypedReferenceTo(pr, pr.GetObjectKind().GroupVersionKind()))
}

func (h *ProviderHooks) podSelectors(pr v1.PackageRevisionWithRuntime) map[string]string {
	return map[string]string{
		v1.LabelRevision: pr.GetName(),
		v1.LabelProvider: h.packageName(pr),
	}
}

func (h *ProviderHooks) packageName(pr v1.PackageRevisionWithRuntime) string {
	return pr.GetLabels()[v1.LabelParentPackage]
}

// ownedMRDs returns the ManagedResourceDefinitions controlled by pr. It returns
// nil without listing if pr does not have the safe-start capability.
func (h *ProviderHooks) ownedMRDs(ctx context.Context, pr v1.PackageRevisionWithRuntime) ([]extv1alpha1.ManagedResourceDefinition, error) {
	if !pkgmetav1.CapabilitiesContainFuzzyMatch(pr.GetCapabilities(), pkgmetav1.ProviderCapabilitySafeStart) {
		return nil, nil
	}

	mrds := &extv1alpha1.ManagedResourceDefinitionList{}
	if err := h.client.List(ctx, mrds); err != nil {
		return nil, errors.Wrap(err, errListMRDs)
	}

	var owned []extv1alpha1.ManagedResourceDefinition

	for i := range mrds.Items {
		if metav1.IsControlledBy(&mrds.Items[i], pr) {
			owned = append(owned, mrds.Items[i])
		}
	}

	return owned, nil
}

// awaitingActivation returns true if the revision has the safe-start
// capability, has never been activated, and owns at least one
// ManagedResourceDefinition, none of which is active. Its runtime is scaled to
// zero until the first one is activated.
func awaitingActivation(pr v1.PackageRevisionWithRuntime, mrds []extv1alpha1.ManagedResourceDefinition) bool {
	if !pkgmetav1.CapabilitiesContainFuzzyMatch(pr.GetCapabilities(), pkgmetav1.ProviderCapabilitySafeStart) {
		return false
	}

	// One-way latch: once the runtime has been activated, never scale it
	// back to zero even if MRDs later appear inactive (deactivation is not
	// yet supported, but guard against manual edits or future changes).
	if pr.GetCondition(v1.TypeRuntimeActive).Reason == v1.ReasonActiveRuntime {
		return false
	}

	if len(mrds) == 0 {
		return false
	}

	for _, mrd := range mrds {
		if mrd.Spec.State.IsActive() {
			return false
		}
	}

	return true
}
