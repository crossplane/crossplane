package revision

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	runtimeContainerName = "package-runtime"
)

type RuntimeManifestBuilder struct {
	revision                  v1.PackageWithRuntimeRevision
	namespace                 string
	serviceAccountPullSecrets []corev1.LocalObjectReference
	controllerConfig          *v1alpha1.ControllerConfig
	runtimeConfig             *v1beta1.DeploymentRuntimeConfig
}

func NewRuntimeManifestBuilder(ctx context.Context, client client.Client, namespace string, serviceAccount string, pwr v1.PackageWithRuntimeRevision) (*RuntimeManifestBuilder, error) {
	b := &RuntimeManifestBuilder{
		namespace: namespace,
		revision:  pwr,
	}

	if ccRef := pwr.GetControllerConfigRef(); ccRef != nil {
		cc := &v1alpha1.ControllerConfig{}
		if err := client.Get(ctx, types.NamespacedName{Name: ccRef.Name}, cc); err != nil {
			return nil, err
		}
		b.controllerConfig = cc
	}

	if rcRef := pwr.GetRuntimeConfigRef(); rcRef != nil {
		rc := &v1beta1.DeploymentRuntimeConfig{}
		if err := client.Get(ctx, types.NamespacedName{Name: rcRef.Name}, rc); err != nil {
			return nil, err
		}
		b.runtimeConfig = rc
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

func (b *RuntimeManifestBuilder) ServiceAccount(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount {
	sa := defaultServiceAccount(b.revision.GetName())

	if b.controllerConfig != nil {
		// Do something with the controller config
	}

	if b.runtimeConfig != nil {
		// Do something with the runtime config
	}

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

	for _, o := range overrides {
		o(sa)
	}

	return sa
}

func (b *RuntimeManifestBuilder) Deployment(serviceAccount string, overrides ...DeploymentOverrides) *appsv1.Deployment {
	d := defaultDeployment(b.revision.GetName())

	if b.controllerConfig != nil {
		// Do something with the controller config
	}

	if b.runtimeConfig != nil {
		// Do something with the runtime config
	}

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

	for _, o := range overrides {
		o(d)
	}

	return d
}

func (b *RuntimeManifestBuilder) Service(overrides ...ServiceOverrides) *corev1.Service {
	svc := defaultService(b.packageName())

	if b.controllerConfig != nil {
		// Do something with the controller config
	}

	if b.runtimeConfig != nil {
		// Do something with the runtime config
	}

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
