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

	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/initializer"
)

const (
	errDeleteFunctionDeployment               = "cannot delete function package deployment"
	errApplyFunctionDeployment                = "cannot apply function package deployment"
	errApplyFunctionSecret                    = "cannot apply function package secret"
	errApplyFunctionSA                        = "cannot apply function package service account"
	errApplyFunctionService                   = "cannot apply function package service"
	errFmtUnavailableFunctionDeployment       = "function package deployment is unavailable with message: %s"
	errNoAvailableConditionFunctionDeployment = "function package deployment has no condition of type \"Available\" yet"
	errParseFunctionImage                     = "cannot parse function package image"
)

// FunctionHooks performs runtime operations for function packages.
type FunctionHooks struct {
	client resource.ClientApplicator

	// namespace is the namespace in which runtime objects are created.
	namespace string
	// coreServiceAccount is the name of the core Crossplane ServiceAccount. We
	// propagate its image pull secrets to the runtime ServiceAccount.
	coreServiceAccount string

	conditions conditions.Manager
}

// NewFunctionHooks returns a new FunctionHooks.
func NewFunctionHooks(c client.Client, namespace, coreServiceAccount string) *FunctionHooks {
	return &FunctionHooks{
		client: resource.ClientApplicator{
			Client:     c,
			Applicator: resource.NewAPIPatchingApplicator(c),
		},
		namespace:          namespace,
		coreServiceAccount: coreServiceAccount,
		conditions:         conditions.ObservedGenerationPropagationManager{},
	}
}

// Pre performs operations meant to happen before establishing objects.
func (h *FunctionHooks) Pre(ctx context.Context, pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error {
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
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
		return errors.Wrap(err, errApplyFunctionService)
	}

	// N.B.: We expect the revision to be applied by the caller
	fRev, ok := pr.(*v1.FunctionRevision)
	if !ok {
		return errors.Errorf("cannot apply function package hooks to %T", pr)
	}

	fRev.Status.Endpoint = fmt.Sprintf(ServiceEndpointFmt, svc.Name, svc.Namespace, GRPCPort)

	secServer := h.tlsServerSecret(pr)

	if secServer == nil {
		// We should wait for the package manager to set the secret name on the
		// revision before proceeding creating the TLS secret. This mirrors the
		// provider hooks, which wait on the same field.
		return nil
	}

	if err := applySharedRuntimeObject(ctx, h.client.Client, pr, secServer); err != nil {
		return errors.Wrap(err, errApplyFunctionSecret)
	}

	if err := initializer.NewTLSCertificateGenerator(secServer.Namespace, initializer.RootCACertSecretName,
		initializer.TLSCertificateGeneratorWithServerSecretName(secServer.GetName(), initializer.DNSNamesForService(svc.Name, svc.Namespace)),
		initializer.TLSCertificateGeneratorWithOwner([]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(pr, pr.GetObjectKind().GroupVersionKind()))})).Run(ctx, h.client.Client); err != nil {
		return errors.Wrapf(err, "cannot generate TLS certificates for %q", pr.GetLabels()[v1.LabelParentPackage])
	}

	return nil
}

// Post performs operations meant to happen after establishing objects.
func (h *FunctionHooks) Post(ctx context.Context, pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error {
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

	sa := h.serviceAccount(pr, rc, saPullSecrets)

	// Determine the function's image.
	image, err := name.ParseReference(pr.GetResolvedSource(), name.StrictValidation)
	if err != nil {
		return errors.Wrap(err, errParseFunctionImage)
	}

	d := h.deployment(pr, rc, sa.Name, image.Name(), pullSecrets)
	// Create/Apply the SA only if the deployment references it.
	// This is to avoid creating a SA that is NOT used by the deployment when
	// the SA is managed externally by the user and configured by setting
	// `deploymentTemplate.spec.template.spec.serviceAccountName` in the
	// DeploymentRuntimeConfig.
	if sa.Name == d.Spec.Template.Spec.ServiceAccountName {
		if err := applySA(ctx, h.client, pr, sa); err != nil {
			return errors.Wrap(err, errApplyFunctionSA)
		}
	}

	if err := applyRuntimeObject(ctx, h.client.Client, d); err != nil {
		return errors.Wrap(err, errApplyFunctionDeployment)
	}

	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable {
			if c.Status != corev1.ConditionTrue {
				return errors.Errorf(errFmtUnavailableFunctionDeployment, c.Message)
			}

			h.conditions.For(pr).MarkConditions(v1.RuntimeHealthy(), v1.RuntimeActive())

			return nil
		}
	}

	return errors.New(errNoAvailableConditionFunctionDeployment)
}

// Deactivate performs operations meant to happen before deactivating a revision.
func (h *FunctionHooks) Deactivate(ctx context.Context, pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error {
	// We're only interested in the name and namespace of the deployment in
	// order to delete it, so we don't bother resolving the image or the pull
	// secrets here.
	sa := h.serviceAccount(pr, rc, nil)
	if err := deleteRuntimeObjectControlledBy(ctx, h.client.Client, pr, h.deployment(pr, rc, sa.Name, "", nil)); err != nil {
		return errors.Wrap(err, errDeleteFunctionDeployment)
	}

	// NOTE(turkenh): We don't delete the service account here because it might
	// be used by other package revisions, e.g. user might have specified a
	// service account name in the runtime config. This should not be a problem
	// because we add the owner reference to the service account when we create
	// them, and they will be garbage collected when the package revision is
	// deleted if they are not used by any other package revisions.

	// NOTE(ezgidemirel): Service and secret are created per package. Therefore,
	// we're not deleting them here.

	// NOTE(jbw976): We leave our owner references on those shared objects alone, controlling flag
	// included. The revision taking over demotes us as part of claiming them, which keeps the
	// handover to a single writer.
	return nil
}

// serviceAccount builds the ServiceAccount of a function revision's runtime.
// The supplied pull secrets are appended to the revision's own.
func (h *FunctionHooks) serviceAccount(pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig, pullSecrets []corev1.LocalObjectReference) *corev1.ServiceAccount {
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

// deployment builds the Deployment of a function revision's runtime.
func (h *FunctionHooks) deployment(pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig, serviceAccount, image string, pullSecrets []string) *appsv1.Deployment {
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

	for _, s := range pullSecrets {
		overrides = append(overrides, DeploymentWithAdditionalPullSecret(corev1.LocalObjectReference{Name: s}))
	}

	if pr.GetPackagePullPolicy() != nil {
		// If the package pull policy is set, it will override the default
		// or whatever is set in the runtime config.
		overrides = append(overrides, DeploymentRuntimeWithImagePullPolicy(*pr.GetPackagePullPolicy()))
	}

	// NOTE(negz): We never build a TLS client secret for a function, but the
	// package manager still names one on the revision, and functions have
	// always mounted it.
	if pr.GetObservedTLSClientSecretName() != nil {
		overrides = append(overrides, DeploymentRuntimeWithTLSClientSecret(*pr.GetObservedTLSClientSecretName()))
	}

	if pr.GetObservedTLSServerSecretName() != nil {
		overrides = append(overrides, DeploymentRuntimeWithTLSServerSecret(*pr.GetObservedTLSServerSecretName()))
	}

	// Function specific overrides. They go last so that they win.
	overrides = append(overrides,
		DeploymentRuntimeWithAdditionalPorts([]corev1.ContainerPort{
			{
				Name:          GRPCPortName,
				ContainerPort: GRPCPort,
			},
		}),
		DeploymentRuntimeWithAdditionalEnvironments([]corev1.EnvVar{
			{
				Name: "FUNCTION_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: fmt.Sprintf("metadata.labels['%s']", v1.LabelFunction),
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
		DeploymentRuntimeWithOptionalImage(image),
	)

	for _, o := range overrides {
		o(d)
	}

	d.TypeMeta = metav1.TypeMeta{
		APIVersion: appsv1.SchemeGroupVersion.String(),
		Kind:       "Deployment",
	}

	return d
}

// service builds the Service of a function revision's runtime. It is shared by
// all revisions of a function.
func (h *FunctionHooks) service(pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) *corev1.Service {
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

		// Function specific overrides. They go last so that they win.

		// We want a headless service so that our gRPC client (i.e. the Crossplane
		// FunctionComposer) can load balance across the endpoints.
		// https://kubernetes.io/docs/concepts/services-networking/service/#headless-services
		ServiceWithClusterIP(corev1.ClusterIPNone),
		ServiceWithAdditionalPorts([]corev1.ServicePort{
			{
				Name:        GRPCPortName,
				Protocol:    corev1.ProtocolTCP,
				Port:        GRPCPort,
				TargetPort:  intstr.FromString(GRPCPortName),
				AppProtocol: &AppProtocolTLS,
			},
		}),
	} {
		o(svc)
	}

	return svc
}

// tlsServerSecret builds the Secret holding a function revision's TLS server
// certificate. It returns nil until the package manager has named the secret.
func (h *FunctionHooks) tlsServerSecret(pr v1.PackageRevisionWithRuntime) *corev1.Secret {
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

func (h *FunctionHooks) owner(pr v1.PackageRevisionWithRuntime) metav1.OwnerReference {
	return meta.AsController(meta.TypedReferenceTo(pr, pr.GetObjectKind().GroupVersionKind()))
}

func (h *FunctionHooks) podSelectors(pr v1.PackageRevisionWithRuntime) map[string]string {
	return map[string]string{
		v1.LabelRevision: pr.GetName(),
		v1.LabelFunction: h.packageName(pr),
	}
}

func (h *FunctionHooks) packageName(pr v1.PackageRevisionWithRuntime) string {
	return pr.GetLabels()[v1.LabelParentPackage]
}
