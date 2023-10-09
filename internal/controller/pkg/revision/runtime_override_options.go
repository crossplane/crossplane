package revision

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/internal/initializer"
)

// ServiceAccountOverrides is a modifier option that overrides a ServiceAccount.
type ServiceAccountOverrides func(sa *corev1.ServiceAccount)

// ServiceAccountWithAdditionalPullSecrets adds additional image pull secrets to
// a ServiceAccount.
func ServiceAccountWithAdditionalPullSecrets(secrets []corev1.LocalObjectReference) ServiceAccountOverrides {
	return func(sa *corev1.ServiceAccount) {
		sa.ImagePullSecrets = append(sa.ImagePullSecrets, secrets...)
	}
}

// ServiceAccountWithNamespace overrides the namespace of a ServiceAccount.
func ServiceAccountWithNamespace(namespace string) ServiceAccountOverrides {
	return func(sa *corev1.ServiceAccount) {
		sa.Namespace = namespace
	}
}

// ServiceAccountWithOwnerReferences overrides the owner references of a
// ServiceAccount.
func ServiceAccountWithOwnerReferences(owners []metav1.OwnerReference) ServiceAccountOverrides {
	return func(sa *corev1.ServiceAccount) {
		sa.OwnerReferences = owners
	}
}

// ServiceAccountWithControllerConfig overrides the labels, annotations and
// name of a ServiceAccount with the values defined in the ControllerConfig.
func ServiceAccountWithControllerConfig(cc *v1alpha1.ControllerConfig) ServiceAccountOverrides {
	return func(sa *corev1.ServiceAccount) {
		sa.Labels = cc.Labels
		sa.Annotations = cc.Annotations
		if cc.Spec.ServiceAccountName != nil {
			sa.Name = *cc.Spec.ServiceAccountName
		}
	}
}

// DeploymentOverrides is a modifier option that overrides a Deployment.
type DeploymentOverrides func(deployment *appsv1.Deployment)

// DeploymentWithName overrides the name of a Deployment.
func DeploymentWithName(name string) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Name = name
	}
}

// DeploymentWithNamespace overrides the namespace of a Deployment.
func DeploymentWithNamespace(namespace string) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Namespace = namespace
	}
}

// DeploymentWithOwnerReferences overrides the owner references of a Deployment.
func DeploymentWithOwnerReferences(owners []metav1.OwnerReference) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.OwnerReferences = owners
	}
}

// DeploymentWithSelectors overrides the selectors of a Deployment. It also
// ensures that the pod template labels always contains the deployment selector.
func DeploymentWithSelectors(selectors map[string]string) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		if d.Spec.Selector == nil {
			d.Spec.Selector = &metav1.LabelSelector{}
		}
		d.Spec.Selector.MatchLabels = selectors
		// Ensure that the pod template labels always contains the deployment
		// selector.
		if d.Spec.Template.Labels == nil {
			d.Spec.Template.Labels = map[string]string{}
		}
		for k, v := range selectors {
			d.Spec.Template.Labels[k] = v
		}
	}
}

// DeploymentWithServiceAccount overrides the service account of a Deployment.
func DeploymentWithServiceAccount(sa string) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.ServiceAccountName = sa
	}
}

// DeploymentWithImagePullSecrets overrides the image pull secrets of a
// Deployment.
func DeploymentWithImagePullSecrets(secrets []corev1.LocalObjectReference) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.ImagePullSecrets = secrets
	}
}

// DeploymentRuntimeWithImage overrides the image of the runtime container of a
// Deployment.
func DeploymentRuntimeWithImage(image string) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		// We will always ensure that the runtime container is the first.
		d.Spec.Template.Spec.Containers[0].Image = image
	}
}

// DeploymentRuntimeWithImagePullPolicy overrides the image pull policy of the
// runtime container of a Deployment.
func DeploymentRuntimeWithImagePullPolicy(policy corev1.PullPolicy) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].ImagePullPolicy = policy
	}
}

// DeploymentRuntimeWithTLSServerSecret mounts a TLS Server secret as a volume
// and sets the path of the mounted volume as an environment variable of the
// runtime container of a Deployment.
func DeploymentRuntimeWithTLSServerSecret(secret string) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		mountTLSSecret(secret, tlsServerCertsVolumeName, tlsServerCertsDir, tlsServerCertDirEnvVar, d)
	}
}

// DeploymentRuntimeWithTLSClientSecret mounts a TLS Client secret as a volume
// and sets the path of the mounted volume as an environment variable of the
// runtime container of a Deployment.
func DeploymentRuntimeWithTLSClientSecret(secret string) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		mountTLSSecret(secret, tlsClientCertsVolumeName, tlsClientCertsDir, tlsClientCertDirEnvVar, d)
	}
}

// DeploymentRuntimeWithAdditionalEnvironments adds additional environment
// variables to the runtime container of a Deployment.
func DeploymentRuntimeWithAdditionalEnvironments(env []corev1.EnvVar) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, env...)
	}
}

// DeploymentRuntimeWithAdditionalPorts adds additional ports to the runtime
// container of a Deployment.
func DeploymentRuntimeWithAdditionalPorts(ports []corev1.ContainerPort) DeploymentOverrides {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Ports = append(d.Spec.Template.Spec.Containers[0].Ports, ports...)
	}
}

// DeploymentForControllerConfig overrides the deployment with the values
// defined in the ControllerConfig.
func DeploymentForControllerConfig(cc *v1alpha1.ControllerConfig) DeploymentOverrides { //nolint:gocyclo // Simple if statements for setting values if they are not nil/empty.
	return func(d *appsv1.Deployment) {
		d.Labels = cc.Labels
		d.Annotations = cc.Annotations
		if cc.Spec.Metadata != nil {
			d.Spec.Template.Annotations = cc.Spec.Metadata.Annotations
		}

		if cc.Spec.Metadata != nil {
			if d.Spec.Template.Labels == nil {
				d.Spec.Template.Labels = map[string]string{}
			}
			for k, v := range cc.Spec.Metadata.Labels {
				d.Spec.Template.Labels[k] = v
			}
		}

		if cc.Spec.Replicas != nil {
			d.Spec.Replicas = cc.Spec.Replicas
		}
		if cc.Spec.Image != nil {
			d.Spec.Template.Spec.Containers[0].Image = *cc.Spec.Image
		}
		if cc.Spec.ImagePullPolicy != nil {
			d.Spec.Template.Spec.Containers[0].ImagePullPolicy = *cc.Spec.ImagePullPolicy
		}
		if len(cc.Spec.Ports) > 0 {
			d.Spec.Template.Spec.Containers[0].Ports = cc.Spec.Ports
		}
		if cc.Spec.NodeSelector != nil {
			d.Spec.Template.Spec.NodeSelector = cc.Spec.NodeSelector
		}
		if cc.Spec.ServiceAccountName != nil {
			d.Spec.Template.Spec.ServiceAccountName = *cc.Spec.ServiceAccountName
		}
		if cc.Spec.NodeName != nil {
			d.Spec.Template.Spec.NodeName = *cc.Spec.NodeName
		}
		if cc.Spec.PodSecurityContext != nil {
			d.Spec.Template.Spec.SecurityContext = cc.Spec.PodSecurityContext
		}
		if cc.Spec.SecurityContext != nil {
			d.Spec.Template.Spec.Containers[0].SecurityContext = cc.Spec.SecurityContext
		}
		if len(cc.Spec.ImagePullSecrets) > 0 {
			d.Spec.Template.Spec.ImagePullSecrets = cc.Spec.ImagePullSecrets
		}
		if cc.Spec.Affinity != nil {
			d.Spec.Template.Spec.Affinity = cc.Spec.Affinity
		}
		if len(cc.Spec.Tolerations) > 0 {
			d.Spec.Template.Spec.Tolerations = cc.Spec.Tolerations
		}
		if cc.Spec.PriorityClassName != nil {
			d.Spec.Template.Spec.PriorityClassName = *cc.Spec.PriorityClassName
		}
		if cc.Spec.RuntimeClassName != nil {
			d.Spec.Template.Spec.RuntimeClassName = cc.Spec.RuntimeClassName
		}
		if cc.Spec.ResourceRequirements != nil {
			d.Spec.Template.Spec.Containers[0].Resources = *cc.Spec.ResourceRequirements
		}
		if len(cc.Spec.Args) > 0 {
			d.Spec.Template.Spec.Containers[0].Args = cc.Spec.Args
		}
		if len(cc.Spec.EnvFrom) > 0 {
			d.Spec.Template.Spec.Containers[0].EnvFrom = cc.Spec.EnvFrom
		}
		if len(cc.Spec.Env) > 0 {
			// We already have some environment variables that we will always
			// want to set (e.g. POD_NAMESPACE), so we just append the new ones
			// that user provided if there are any.
			d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, cc.Spec.Env...)
		}
		if len(cc.Spec.Volumes) > 0 {
			d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, cc.Spec.Volumes...)
		}
		if len(cc.Spec.VolumeMounts) > 0 {
			d.Spec.Template.Spec.Containers[0].VolumeMounts =
				append(d.Spec.Template.Spec.Containers[0].VolumeMounts, cc.Spec.VolumeMounts...)
		}
	}
}

// ServiceOverrides is a modifier option that overrides a Service.
type ServiceOverrides func(service *corev1.Service)

// ServiceWithName overrides the name of a Service.
func ServiceWithName(name string) ServiceOverrides {
	return func(s *corev1.Service) {
		s.Name = name
	}
}

// ServiceWithNamespace overrides the namespace of a Service.
func ServiceWithNamespace(namespace string) ServiceOverrides {
	return func(s *corev1.Service) {
		s.Namespace = namespace
	}
}

// ServiceWithOwnerReferences overrides the owner references of a Service.
func ServiceWithOwnerReferences(owners []metav1.OwnerReference) ServiceOverrides {
	return func(s *corev1.Service) {
		s.OwnerReferences = owners
	}
}

// ServiceWithSelectors overrides the selectors of a Service.
func ServiceWithSelectors(selectors map[string]string) ServiceOverrides {
	return func(s *corev1.Service) {
		s.Spec.Selector = selectors
	}
}

// ServiceWithAdditionalPorts adds additional ports to a Service.
func ServiceWithAdditionalPorts(ports []corev1.ServicePort) ServiceOverrides {
	return func(s *corev1.Service) {
		s.Spec.Ports = append(s.Spec.Ports, ports...)
	}
}

// ServiceWithClusterIP overrides the cluster IP of a Service.
func ServiceWithClusterIP(clusterIP string) ServiceOverrides {
	return func(s *corev1.Service) {
		s.Spec.ClusterIP = clusterIP
	}
}

func mountTLSSecret(secret, volName, mountPath, envName string, d *appsv1.Deployment) {
	v := corev1.Volume{
		Name: volName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secret,
				Items: []corev1.KeyToPath{
					// These are known and validated keys in TLS secrets.
					{Key: corev1.TLSCertKey, Path: corev1.TLSCertKey},
					{Key: corev1.TLSPrivateKeyKey, Path: corev1.TLSPrivateKeyKey},
					{Key: initializer.SecretKeyCACert, Path: initializer.SecretKeyCACert},
				},
			},
		},
	}
	d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, v)

	vm := corev1.VolumeMount{
		Name:      volName,
		ReadOnly:  true,
		MountPath: mountPath,
	}
	d.Spec.Template.Spec.Containers[0].VolumeMounts =
		append(d.Spec.Template.Spec.Containers[0].VolumeMounts, vm)

	envs := []corev1.EnvVar{
		{Name: envName, Value: mountPath},
	}
	d.Spec.Template.Spec.Containers[0].Env =
		append(d.Spec.Template.Spec.Containers[0].Env, envs...)
}
