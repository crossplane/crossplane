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
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

const (
	runtimeContainerName = "package-runtime"
	// Providers are expected to use port 8080 if they expose Prometheus
	// metrics, which any provider built using controller-runtime will do by
	// default.
	metricsPortName   = "metrics"
	metricsPortNumber = 8080

	webhookTLSCertDirEnvVar = "WEBHOOK_TLS_CERT_DIR"
	webhookPortName         = "webhook"

	// See https://github.com/grpc/grpc/blob/v1.58.0/doc/naming.md
	grpcPortName       = "grpc"
	servicePort        = 9443
	serviceEndpointFmt = "dns:///%s.%s:%d"

	essTLSCertDirEnvVar = "ESS_TLS_CERTS_DIR"

	tlsServerCertDirEnvVar   = "TLS_SERVER_CERTS_DIR"
	tlsServerCertsVolumeName = "tls-server-certs"
	tlsServerCertsDir        = "/tls/server"

	tlsClientCertDirEnvVar   = "TLS_CLIENT_CERTS_DIR"
	tlsClientCertsVolumeName = "tls-client-certs"
	tlsClientCertsDir        = "/tls/client"
)

//nolint:gochecknoglobals // We treat these as constants, but take their addresses.
var (
	runAsUser                = int64(2000)
	runAsGroup               = int64(2000)
	allowPrivilegeEscalation = false
	privileged               = false
	runAsNonRoot             = true
)

// ManifestBuilder builds the runtime manifests for a package revision.
type ManifestBuilder interface {
	// ServiceAccount builds and returns the service account manifest.
	ServiceAccount(overrides ...ServiceAccountOverride) *corev1.ServiceAccount
	// Deployment builds and returns the deployment manifest.
	Deployment(serviceAccount string, overrides ...DeploymentOverride) *appsv1.Deployment
	// Service builds and returns the service manifest.
	Service(overrides ...ServiceOverride) *corev1.Service
	// TLSClientSecret builds and returns the TLS client secret manifest.
	TLSClientSecret() *corev1.Secret
	// TLSServerSecret builds and returns the TLS server secret manifest.
	TLSServerSecret() *corev1.Secret
}

// A RuntimeHooks performs runtime operations before and after a revision
// establishes objects.
type RuntimeHooks interface {
	// Pre performs operations meant to happen before establishing objects.
	Pre(ctx context.Context, obj runtime.Object, pr v1.PackageRevisionWithRuntime, b ManifestBuilder) error

	// Post performs operations meant to happen after establishing objects.
	Post(ctx context.Context, obj runtime.Object, pr v1.PackageRevisionWithRuntime, b ManifestBuilder) error

	// Deactivate performs operations meant to happen before deactivating a revision.
	Deactivate(ctx context.Context, pr v1.PackageRevisionWithRuntime, b ManifestBuilder) error
}

// RuntimeManifestBuilder builds the runtime manifests for a package revision.
type RuntimeManifestBuilder struct {
	revision                  v1.PackageRevisionWithRuntime
	namespace                 string
	serviceAccountPullSecrets []corev1.LocalObjectReference
	runtimeConfig             *v1beta1.DeploymentRuntimeConfig
	controllerConfig          *v1alpha1.ControllerConfig
	pullSecrets               []string
}

// RuntimeManifestBuilderOption is used to configure a RuntimeManifestBuilder.
type RuntimeManifestBuilderOption func(*RuntimeManifestBuilder)

// RuntimeManifestBuilderWithRuntimeConfig sets the deployment runtime config to
// use when building the runtime manifests.
func RuntimeManifestBuilderWithRuntimeConfig(rc *v1beta1.DeploymentRuntimeConfig) RuntimeManifestBuilderOption {
	return func(b *RuntimeManifestBuilder) {
		b.runtimeConfig = rc
	}
}

// RuntimeManifestBuilderWithControllerConfig sets the controller config to use
// when building the runtime manifests.
func RuntimeManifestBuilderWithControllerConfig(cc *v1alpha1.ControllerConfig) RuntimeManifestBuilderOption {
	return func(b *RuntimeManifestBuilder) {
		b.controllerConfig = cc
	}
}

// RuntimeManifestBuilderWithServiceAccountPullSecrets sets the service account
// pull secrets to use when building the runtime manifests.
func RuntimeManifestBuilderWithServiceAccountPullSecrets(secrets []corev1.LocalObjectReference) RuntimeManifestBuilderOption {
	return func(b *RuntimeManifestBuilder) {
		b.serviceAccountPullSecrets = secrets
	}
}

// RuntimeManifestBuilderWithPullSecrets sets the pull secrets to use when
// building the runtime manifests.
func RuntimeManifestBuilderWithPullSecrets(secrets ...string) RuntimeManifestBuilderOption {
	return func(b *RuntimeManifestBuilder) {
		b.pullSecrets = secrets
	}
}

// NewRuntimeManifestBuilder returns a new RuntimeManifestBuilder.
func NewRuntimeManifestBuilder(pwr v1.PackageRevisionWithRuntime, namespace string, opts ...RuntimeManifestBuilderOption) *RuntimeManifestBuilder {
	b := &RuntimeManifestBuilder{
		namespace: namespace,
		revision:  pwr,
	}

	for _, o := range opts {
		o(b)
	}

	return b
}

// ServiceAccount builds and returns the ServiceAccount manifest.
func (b *RuntimeManifestBuilder) ServiceAccount(overrides ...ServiceAccountOverride) *corev1.ServiceAccount {
	sa := &corev1.ServiceAccount{}
	if b.runtimeConfig != nil {
		sa = serviceAccountFromRuntimeConfig(b.runtimeConfig.Spec.ServiceAccountTemplate)
	}

	var allOverrides []ServiceAccountOverride
	allOverrides = append(allOverrides,
		// Optional defaults, will be used only if the runtime config does not
		// specify them.
		ServiceAccountWithOptionalName(b.revision.GetName()),

		// Overrides that we are opinionated about.
		ServiceAccountWithNamespace(b.namespace),
		ServiceAccountWithOwnerReferences([]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(b.revision, b.revision.GetObjectKind().GroupVersionKind()))}),
		ServiceAccountWithAdditionalPullSecrets(append(b.revision.GetPackagePullSecrets(), b.serviceAccountPullSecrets...)),
	)

	if cc := b.controllerConfig; cc != nil {
		allOverrides = append(allOverrides, ServiceAccountWithControllerConfig(cc))
	}

	// We append the overrides passed to the function last so that they can
	// override the above ones.
	allOverrides = append(allOverrides, overrides...)

	for _, o := range allOverrides {
		o(sa)
	}

	return sa
}

// Deployment builds and returns the Deployment manifest.
func (b *RuntimeManifestBuilder) Deployment(serviceAccount string, overrides ...DeploymentOverride) *appsv1.Deployment {
	d := &appsv1.Deployment{}
	if b.runtimeConfig != nil {
		d = deploymentFromRuntimeConfig(b.runtimeConfig.Spec.DeploymentTemplate)
	}

	allOverrides := make([]DeploymentOverride, 0, len(overrides)+20) // 20 is just a reasonable guess at the number of overrides we'll add.
	allOverrides = append(allOverrides,
		// This will ensure that the runtime container exists and always the
		// first one.
		DeploymentWithRuntimeContainer(),

		// Optional defaults, will be used only if the runtime config does not
		// specify them.
		DeploymentWithOptionalName(b.revision.GetName()),
		DeploymentWithOptionalReplicas(1),
		DeploymentWithOptionalPodSecurityContext(&corev1.PodSecurityContext{
			RunAsNonRoot: &runAsNonRoot,
			RunAsUser:    &runAsUser,
			RunAsGroup:   &runAsGroup,
		}),
		DeploymentRuntimeWithOptionalImagePullPolicy(corev1.PullIfNotPresent),
		DeploymentRuntimeWithOptionalSecurityContext(&corev1.SecurityContext{
			RunAsUser:                &runAsUser,
			RunAsGroup:               &runAsGroup,
			AllowPrivilegeEscalation: &allowPrivilegeEscalation,
			Privileged:               &privileged,
			RunAsNonRoot:             &runAsNonRoot,
		}),
		DeploymentWithOptionalServiceAccount(serviceAccount),

		// Overrides that we are opinionated about.
		DeploymentWithNamespace(b.namespace),
		DeploymentWithOwnerReferences([]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(b.revision, b.revision.GetObjectKind().GroupVersionKind()))}),
		DeploymentWithSelectors(b.podSelectors()),
		DeploymentWithImagePullSecrets(b.revision.GetPackagePullSecrets()),
		DeploymentRuntimeWithAdditionalPorts([]corev1.ContainerPort{
			{
				Name:          metricsPortName,
				ContainerPort: metricsPortNumber,
			},
		}),
	)

	for _, s := range b.pullSecrets {
		allOverrides = append(allOverrides, DeploymentWithAdditionalPullSecret(corev1.LocalObjectReference{Name: s}))
	}

	if b.revision.GetPackagePullPolicy() != nil {
		// If the package pull policy is set, it will override the default
		// or whatever is set in the runtime config.
		allOverrides = append(allOverrides, DeploymentRuntimeWithImagePullPolicy(*b.revision.GetPackagePullPolicy()))
	}

	if b.revision.GetTLSClientSecretName() != nil {
		allOverrides = append(allOverrides, DeploymentRuntimeWithTLSClientSecret(*b.revision.GetTLSClientSecretName()))
	}

	if b.revision.GetTLSServerSecretName() != nil {
		allOverrides = append(allOverrides, DeploymentRuntimeWithTLSServerSecret(*b.revision.GetTLSServerSecretName()))
	}

	// We append the overrides passed to the function last so that they can
	// override the above ones.
	allOverrides = append(allOverrides, overrides...)

	// ControllerConfig overrides should be applied last so that they can
	// override any other overrides compatible with the existing behavior.
	if b.controllerConfig != nil {
		allOverrides = append(allOverrides, DeploymentForControllerConfig(b.controllerConfig))
	}

	for _, o := range allOverrides {
		o(d)
	}

	return d
}

// Service builds and returns the Service manifest.
func (b *RuntimeManifestBuilder) Service(overrides ...ServiceOverride) *corev1.Service {
	svc := &corev1.Service{}
	if b.runtimeConfig != nil {
		svc = serviceFromRuntimeConfig(b.runtimeConfig.Spec.ServiceTemplate)
	}

	var allOverrides []ServiceOverride
	allOverrides = append(allOverrides,
		// Optional defaults, will be used only if the runtime config does not
		// specify them.
		ServiceWithOptionalName(b.packageName()),

		// Overrides that we are opinionated about.
		ServiceWithNamespace(b.namespace),
		ServiceWithOwnerReferences([]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(b.revision, b.revision.GetObjectKind().GroupVersionKind()))}),
		ServiceWithSelectors(b.podSelectors()),
		ServiceWithAdditionalPorts([]corev1.ServicePort{
			{
				Protocol:   corev1.ProtocolTCP,
				Port:       servicePort,
				TargetPort: intstr.FromInt32(servicePort),
			},
		}))

	// We append the overrides passed to the function last so that they can
	// override the above ones.
	allOverrides = append(allOverrides, overrides...)

	for _, o := range allOverrides {
		o(svc)
	}

	return svc
}

// TLSClientSecret builds and returns the Secret manifest for the TLS client certificate.
func (b *RuntimeManifestBuilder) TLSClientSecret() *corev1.Secret {
	if b.revision.GetTLSClientSecretName() == nil {
		return nil
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            *b.revision.GetTLSClientSecretName(),
			Namespace:       b.namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(b.revision, b.revision.GetObjectKind().GroupVersionKind()))},
		},
	}
}

// TLSServerSecret builds and returns the Secret manifest for the TLS server certificate.
func (b *RuntimeManifestBuilder) TLSServerSecret() *corev1.Secret {
	if b.revision.GetTLSServerSecretName() == nil {
		return nil
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            *b.revision.GetTLSServerSecretName(),
			Namespace:       b.namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(b.revision, b.revision.GetObjectKind().GroupVersionKind()))},
		},
	}
}

func (b *RuntimeManifestBuilder) podSelectors() map[string]string {
	return map[string]string{
		"pkg.crossplane.io/revision":           b.revision.GetName(),
		"pkg.crossplane.io/" + b.packageType(): b.packageName(),
	}
}

func (b *RuntimeManifestBuilder) packageName() string {
	return b.revision.GetLabels()[v1.LabelParentPackage]
}

func (b *RuntimeManifestBuilder) packageType() string {
	if _, ok := b.revision.(*v1.FunctionRevision); ok {
		return "function"
	}
	return "provider"
}
