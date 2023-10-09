package revision

import (
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
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

const (
	errGetControllerConfig = "cannot get referenced controller config"
	errGetServiceAccount   = "cannot get Crossplane service account"
)

// ManifestBuilder builds the runtime manifests for a package revision.
type ManifestBuilder interface {
	// ServiceAccount builds and returns the service account manifest.
	ServiceAccount(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount
	// Deployment builds and returns the deployment manifest.
	Deployment(serviceAccount string, overrides ...DeploymentOverrides) *appsv1.Deployment
	// Service builds and returns the service manifest.
	Service(overrides ...ServiceOverrides) *corev1.Service
	// TLSClientSecret builds and returns the TLS client secret manifest.
	TLSClientSecret() *corev1.Secret
	// TLSServerSecret builds and returns the TLS server secret manifest.
	TLSServerSecret() *corev1.Secret
}

// A RuntimeHooks performs runtime operations before and after a revision
// establishes objects.
type RuntimeHooks interface {
	// Pre performs operations meant to happen before establishing objects.
	Pre(context.Context, runtime.Object, v1.PackageRevisionWithRuntime, ManifestBuilder) error

	// Post performs operations meant to happen after establishing objects.
	Post(context.Context, runtime.Object, v1.PackageRevisionWithRuntime, ManifestBuilder) error

	// Deactivate performs operations meant to happen before deactivating a revision.
	Deactivate(context.Context, v1.PackageRevisionWithRuntime, ManifestBuilder) error
}

// RuntimeManifestBuilder builds the runtime manifests for a package revision.
type RuntimeManifestBuilder struct {
	revision                  v1.PackageRevisionWithRuntime
	namespace                 string
	serviceAccountPullSecrets []corev1.LocalObjectReference
	controllerConfig          *v1alpha1.ControllerConfig
}

// NewRuntimeManifestBuilder returns a new RuntimeManifestBuilder.
func NewRuntimeManifestBuilder(ctx context.Context, client client.Client, namespace string, serviceAccount string, pwr v1.PackageRevisionWithRuntime) (*RuntimeManifestBuilder, error) {
	b := &RuntimeManifestBuilder{
		namespace: namespace,
		revision:  pwr,
	}

	if ccRef := pwr.GetControllerConfigRef(); ccRef != nil {
		cc := &v1alpha1.ControllerConfig{}
		if err := client.Get(ctx, types.NamespacedName{Name: ccRef.Name}, cc); err != nil {
			return nil, errors.Wrap(err, errGetControllerConfig)
		}
		b.controllerConfig = cc
	}

	sa := &corev1.ServiceAccount{}
	// Fetch XP ServiceAccount to get the ImagePullSecrets defined there.
	// We will append them to the list of ImagePullSecrets for the runtime
	// ServiceAccount.
	if err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: serviceAccount}, sa); err != nil {
		return nil, errors.Wrap(err, errGetServiceAccount)
	}
	b.serviceAccountPullSecrets = sa.ImagePullSecrets

	return b, nil
}

// ServiceAccount builds and returns the ServiceAccount manifest.
func (b *RuntimeManifestBuilder) ServiceAccount(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount {
	sa := defaultServiceAccount(b.revision.GetName())

	overrides = append(overrides,
		// Currently it is not possible to override the namespace,
		// ownerReferences or pullSecrets of the service account, and we could
		// define them as defaults. However, we will leave them as overrides
		// to indicate that we are opinionated about them currently and follow
		// a consistent pattern.
		ServiceAccountWithNamespace(b.namespace),
		ServiceAccountWithOwnerReferences([]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(b.revision, b.revision.GetObjectKind().GroupVersionKind()))}),
		ServiceAccountWithAdditionalPullSecrets(append(b.revision.GetPackagePullSecrets(), b.serviceAccountPullSecrets...)),
	)

	if cc := b.controllerConfig; cc != nil {
		overrides = append(overrides, ServiceAccountWithControllerConfig(cc))
	}

	for _, o := range overrides {
		o(sa)
	}

	return sa
}

// Deployment builds and returns the Deployment manifest.
func (b *RuntimeManifestBuilder) Deployment(serviceAccount string, overrides ...DeploymentOverrides) *appsv1.Deployment {
	d := defaultDeployment(b.revision.GetName())

	overrides = append(overrides,
		DeploymentWithNamespace(b.namespace),
		DeploymentWithOwnerReferences([]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(b.revision, b.revision.GetObjectKind().GroupVersionKind()))}),
		DeploymentWithSelectors(b.podSelectors()),
		DeploymentWithServiceAccount(serviceAccount),
		DeploymentWithImagePullSecrets(b.revision.GetPackagePullSecrets()),
		DeploymentRuntimeWithImage(b.revision.GetSource()),
	)

	if b.revision.GetPackagePullPolicy() != nil {
		// If the package pull policy is set, it will override the default
		// or whatever is set in the runtime config.
		overrides = append(overrides, DeploymentRuntimeWithImagePullPolicy(*b.revision.GetPackagePullPolicy()))
	}

	if b.revision.GetTLSClientSecretName() != nil {
		overrides = append(overrides, DeploymentRuntimeWithTLSClientSecret(*b.revision.GetTLSClientSecretName()))
	}

	if b.revision.GetTLSServerSecretName() != nil {
		overrides = append(overrides, DeploymentRuntimeWithTLSServerSecret(*b.revision.GetTLSServerSecretName()))
	}

	if b.controllerConfig != nil {
		overrides = append(overrides, DeploymentForControllerConfig(b.controllerConfig))
	}

	for _, o := range overrides {
		o(d)
	}

	return d
}

// Service builds and returns the Service manifest.
func (b *RuntimeManifestBuilder) Service(overrides ...ServiceOverrides) *corev1.Service {
	svc := defaultService(b.packageName())

	overrides = append(overrides,
		// Currently it is not possible to override the namespace,
		// ownerReferences, selectors or ports of the service, and we could
		// define them as defaults. However, we will leave them as overrides
		// to indicate that we are opinionated about them currently and follow
		// a consistent pattern.
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

	for _, o := range overrides {
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
	if _, ok := b.revision.(*v1beta1.FunctionRevision); ok {
		return "function"
	}
	return "provider"
}
