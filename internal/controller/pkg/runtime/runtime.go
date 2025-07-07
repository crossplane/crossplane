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
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

const (
	// ContainerName is the name of the package runtime container.
	ContainerName = "package-runtime"
	// Providers are expected to use port 8080 if they expose Prometheus
	// metrics, which any provider built using controller-runtime will do by
	// default.

	// MetricsPortName is the name of the metrics port.
	MetricsPortName = "metrics"
	// MetricsPortNumber is the port number for metrics.
	MetricsPortNumber = 8080

	// WebhookTLSCertDirEnvVar is the environment variable for webhook TLS certificate directory.
	WebhookTLSCertDirEnvVar = "WEBHOOK_TLS_CERT_DIR"
	// WebhookPortName is the name of the webhook port.
	WebhookPortName = "webhook"

	// See https://github.com/grpc/grpc/blob/v1.58.0/doc/naming.md

	// GRPCPortName is the name of the gRPC port.
	GRPCPortName = "grpc"
	// GRPCPort is the port number for gRPC.
	GRPCPort = 9443
	// ServiceEndpointFmt is the format string for service endpoints.
	ServiceEndpointFmt = "dns:///%s.%s:%d"

	// ESSTLSCertDirEnvVar is the environment variable for ESS TLS certificate directory.
	ESSTLSCertDirEnvVar = "ESS_TLS_CERTS_DIR"

	// TLSServerCertDirEnvVar is the environment variable for TLS server certificate directory.
	TLSServerCertDirEnvVar = "TLS_SERVER_CERTS_DIR"
	// TLSServerCertsVolumeName is the name of the TLS server certificates volume.
	TLSServerCertsVolumeName = "tls-server-certs"
	// TLSServerCertsDir is the directory path for TLS server certificates.
	TLSServerCertsDir = "/tls/server"

	// TLSClientCertDirEnvVar is the environment variable for TLS client certificate directory.
	TLSClientCertDirEnvVar = "TLS_CLIENT_CERTS_DIR"
	// TLSClientCertsVolumeName is the name of the TLS client certificates volume.
	TLSClientCertsVolumeName = "tls-client-certs"
	// TLSClientCertsDir is the directory path for TLS client certificates.
	TLSClientCertsDir = "/tls/client"
)

//nolint:gochecknoglobals // We treat these as constants, but take their addresses.
var (
	// RunAsUser is the user ID to run containers as.
	RunAsUser = int64(2000)
	// RunAsGroup is the group ID to run containers as.
	RunAsGroup = int64(2000)
	// AllowPrivilegeEscalation indicates whether privilege escalation is allowed.
	AllowPrivilegeEscalation = false
	// Privileged indicates whether containers run in privileged mode.
	Privileged = false
	// RunAsNonRoot indicates whether containers must run as non-root user.
	RunAsNonRoot = true
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

// A Hooks performs runtime operations before and after a revision
// establishes objects.
type Hooks interface {
	// Pre performs operations meant to happen before establishing objects.
	Pre(ctx context.Context, pr v1.PackageRevisionWithRuntime, b ManifestBuilder) error

	// Post performs operations meant to happen after establishing objects.
	Post(ctx context.Context, pr v1.PackageRevisionWithRuntime, b ManifestBuilder) error

	// Deactivate performs operations meant to happen before deactivating a revision.
	Deactivate(ctx context.Context, pr v1.PackageRevisionWithRuntime, b ManifestBuilder) error
}

// DeploymentRuntimeBuilder builds the Deployment runtime manifests for
// a package revision.
type DeploymentRuntimeBuilder struct {
	revision                  v1.PackageRevisionWithRuntime
	namespace                 string
	serviceAccountPullSecrets []corev1.LocalObjectReference
	runtimeConfig             *v1beta1.DeploymentRuntimeConfig
	pullSecrets               []string
}

// BuilderOption is used to configure a DeploymentRuntimeBuilder.
type BuilderOption func(*DeploymentRuntimeBuilder)

// BuilderWithRuntimeConfig sets the deployment runtime config to
// use when building the runtime manifests.
func BuilderWithRuntimeConfig(rc *v1beta1.DeploymentRuntimeConfig) BuilderOption {
	return func(b *DeploymentRuntimeBuilder) {
		b.runtimeConfig = rc
	}
}

// BuilderWithServiceAccountPullSecrets sets the service account
// pull secrets to use when building the runtime manifests.
func BuilderWithServiceAccountPullSecrets(secrets []corev1.LocalObjectReference) BuilderOption {
	return func(b *DeploymentRuntimeBuilder) {
		b.serviceAccountPullSecrets = secrets
	}
}

// BuilderWithPullSecrets sets the pull secrets to use when
// building the runtime manifests.
func BuilderWithPullSecrets(secrets ...string) BuilderOption {
	return func(b *DeploymentRuntimeBuilder) {
		b.pullSecrets = secrets
	}
}

// NewDeploymentRuntimeBuilder returns a new DeploymentRuntimeBuilder.
func NewDeploymentRuntimeBuilder(pwr v1.PackageRevisionWithRuntime, namespace string, opts ...BuilderOption) *DeploymentRuntimeBuilder {
	b := &DeploymentRuntimeBuilder{
		namespace: namespace,
		revision:  pwr,
	}

	for _, o := range opts {
		o(b)
	}

	return b
}

// ServiceAccount builds and returns the ServiceAccount manifest.
func (b *DeploymentRuntimeBuilder) ServiceAccount(overrides ...ServiceAccountOverride) *corev1.ServiceAccount {
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

	// We append the overrides passed to the function last so that they can
	// override the above ones.
	allOverrides = append(allOverrides, overrides...)

	for _, o := range allOverrides {
		o(sa)
	}

	return sa
}

// Deployment builds and returns the Deployment manifest.
func (b *DeploymentRuntimeBuilder) Deployment(serviceAccount string, overrides ...DeploymentOverride) *appsv1.Deployment {
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
		DeploymentWithNamespace(b.namespace),
		DeploymentWithOwnerReferences([]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(b.revision, b.revision.GetObjectKind().GroupVersionKind()))}),
		DeploymentWithSelectors(b.podSelectors()),
		DeploymentWithImagePullSecrets(b.revision.GetPackagePullSecrets()),
		DeploymentRuntimeWithAdditionalPorts([]corev1.ContainerPort{
			{
				Name:          MetricsPortName,
				ContainerPort: MetricsPortNumber,
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

	if b.revision.GetObservedTLSClientSecretName() != nil {
		allOverrides = append(allOverrides, DeploymentRuntimeWithTLSClientSecret(*b.revision.GetObservedTLSClientSecretName()))
	}

	if b.revision.GetObservedTLSServerSecretName() != nil {
		allOverrides = append(allOverrides, DeploymentRuntimeWithTLSServerSecret(*b.revision.GetObservedTLSServerSecretName()))
	}

	// We append the overrides passed to the function last so that they can
	// override the above ones.
	allOverrides = append(allOverrides, overrides...)

	for _, o := range allOverrides {
		o(d)
	}

	return d
}

// Service builds and returns the Service manifest.
func (b *DeploymentRuntimeBuilder) Service(overrides ...ServiceOverride) *corev1.Service {
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
		ServiceWithSelectors(b.podSelectors()))

	// We append the overrides passed to the function last so that they can
	// override the above ones.
	allOverrides = append(allOverrides, overrides...)

	for _, o := range allOverrides {
		o(svc)
	}

	return svc
}

// TLSClientSecret builds and returns the Secret manifest for the TLS client certificate.
func (b *DeploymentRuntimeBuilder) TLSClientSecret() *corev1.Secret {
	if b.revision.GetObservedTLSClientSecretName() == nil {
		return nil
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            *b.revision.GetObservedTLSClientSecretName(),
			Namespace:       b.namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(b.revision, b.revision.GetObjectKind().GroupVersionKind()))},
		},
	}
}

// TLSServerSecret builds and returns the Secret manifest for the TLS server certificate.
func (b *DeploymentRuntimeBuilder) TLSServerSecret() *corev1.Secret {
	if b.revision.GetObservedTLSServerSecretName() == nil {
		return nil
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            *b.revision.GetObservedTLSServerSecretName(),
			Namespace:       b.namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(b.revision, b.revision.GetObjectKind().GroupVersionKind()))},
		},
	}
}

func (b *DeploymentRuntimeBuilder) podSelectors() map[string]string {
	return map[string]string{
		"pkg.crossplane.io/revision":           b.revision.GetName(),
		"pkg.crossplane.io/" + b.packageType(): b.packageName(),
	}
}

func (b *DeploymentRuntimeBuilder) packageName() string {
	return b.revision.GetLabels()[v1.LabelParentPackage]
}

func (b *DeploymentRuntimeBuilder) packageType() string {
	if _, ok := b.revision.(*v1.FunctionRevision); ok {
		return "function"
	}

	return "provider"
}
