package revision

import (
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	runtimeContainerName = "package-runtime"
)

type RuntimeManifestBuilder struct {
	controllerConfig *v1alpha1.ControllerConfig
	runtimeConfig    *v1beta1.DeploymentRuntimeConfig
	namespace        string
	revision         v1.PackageWithRuntimeRevision
}

func NewRuntimeManifestBuilder(ctx context.Context, client client.Client, namespace string, pwr v1.PackageWithRuntimeRevision) (*RuntimeManifestBuilder, error) {
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

	return b, nil
}

type DeploymentOverrides func(deployment *appsv1.Deployment)

func DeploymentWithName(name string) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Name = name
	}
}

func DeploymentWithNamespace(namespace string) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Namespace = namespace
	}
}

func DeploymentWithOwnerReferences(owners []metav1.OwnerReference) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.OwnerReferences = owners
	}
}

func DeploymentWithSelectors(selectors map[string]string) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Spec.Selector.MatchLabels = selectors
		// Ensure that the pod template labels always contains the deployment
		// selector.
		for k, v := range selectors {
			d.Spec.Template.Labels[k] = v
		}
	}
}

func DeploymentWithServiceAccount(sa string) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.ServiceAccountName = sa
	}
}

func DeploymentWithImagePullSecrets(secrets []corev1.LocalObjectReference) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.ImagePullSecrets = secrets
	}
}

func DeploymentRuntimeWithImage(image string) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Image = image
	}
}

func DeploymentRuntimeWithImagePullPolicy(policy corev1.PullPolicy) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].ImagePullPolicy = policy
	}
}

func DeploymentRuntimeWithTLSServerSecret(secret string) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		mountTLSSecret(secret, tlsServerCertsVolumeName, tlsServerCertsDir, tlsServerCertDirEnvVar, d)
	}
}

func DeploymentRuntimeWithTLSClientSecret(secret string) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		mountTLSSecret(secret, tlsClientCertsVolumeName, tlsClientCertsDir, tlsClientCertDirEnvVar, d)
	}
}

func DeploymentRuntimeWithAdditionalEnvironments(env []corev1.EnvVar) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, env...)
	}
}

func DeploymentRuntimeWithAdditionalPorts(ports []corev1.ContainerPort) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Ports = append(d.Spec.Template.Spec.Containers[0].Ports, ports...)
	}
}

func (b *RuntimeManifestBuilder) Deployment(overrides ...DeploymentOverrides) *appsv1.Deployment {
	d := b.defaultDeployment()

	if b.controllerConfig != nil {
		// Do something with the controller config
	}

	if b.runtimeConfig != nil {
		// Do something with the runtime config
	}

	overrides = append(overrides,
		DeploymentWithName(b.revision.GetName()),
		DeploymentWithNamespace(b.namespace),
		DeploymentWithOwnerReferences([]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(b.revision, b.revision.GetObjectKind().GroupVersionKind()))}),
		DeploymentWithSelectors(b.podSelectors()),
		DeploymentWithServiceAccount(b.revision.GetName()),
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

func (b *RuntimeManifestBuilder) ServiceAccount() *corev1.ServiceAccount {
	//TODO implement me
	panic("implement me")
}

func (b *RuntimeManifestBuilder) Service() *corev1.Service {
	//TODO implement me
	panic("implement me")
}

func (b *RuntimeManifestBuilder) TLSServerSecret() *corev1.Secret {
	//TODO implement me
	panic("implement me")
}

func (b *RuntimeManifestBuilder) TLSClientSecret() *corev1.Secret {
	//TODO implement me
	panic("implement me")
}

func (b *RuntimeManifestBuilder) podSelectors() map[string]string {
	return map[string]string{
		"pkg.crossplane.io/revision":           b.revision.GetName(),
		"pkg.crossplane.io/" + b.packageType(): b.packageName(),
	}
}

func (b *RuntimeManifestBuilder) defaultDeployment() *appsv1.Deployment {
	// TODO(turkenh): Implement configurable defaults.
	// See https://github.com/crossplane/crossplane/issues/4699#issuecomment-1748403479
	return &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
						RunAsUser:    &runAsUser,
						RunAsGroup:   &runAsGroup,
					},
					Containers: []corev1.Container{
						{
							Name:            runtimeContainerName,
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:                &runAsUser,
								RunAsGroup:               &runAsGroup,
								AllowPrivilegeEscalation: &allowPrivilegeEscalation,
								Privileged:               &privileged,
								RunAsNonRoot:             &runAsNonRoot,
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          metricsPortName,
									ContainerPort: metricsPortNumber,
								},
							},
						},
					},
				},
			},
		},
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
