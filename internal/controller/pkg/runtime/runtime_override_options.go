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
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane/internal/initializer"
)

// ServiceAccountOverride is a modifier option that overrides a ServiceAccount.
type ServiceAccountOverride func(sa *corev1.ServiceAccount)

// ServiceAccountWithAdditionalPullSecrets adds additional image pull secrets to
// a ServiceAccount.
func ServiceAccountWithAdditionalPullSecrets(secrets []corev1.LocalObjectReference) ServiceAccountOverride {
	return func(sa *corev1.ServiceAccount) {
		sa.ImagePullSecrets = append(sa.ImagePullSecrets, secrets...)
	}
}

// ServiceAccountWithOptionalName overrides the name of a ServiceAccount if
// empty.
func ServiceAccountWithOptionalName(name string) ServiceAccountOverride {
	return func(sa *corev1.ServiceAccount) {
		if sa.Name == "" {
			sa.Name = name
		}
	}
}

// ServiceAccountWithNamespace overrides the namespace of a ServiceAccount.
func ServiceAccountWithNamespace(namespace string) ServiceAccountOverride {
	return func(sa *corev1.ServiceAccount) {
		sa.Namespace = namespace
	}
}

// ServiceAccountWithOwnerReferences overrides the owner references of a
// ServiceAccount.
func ServiceAccountWithOwnerReferences(owners []metav1.OwnerReference) ServiceAccountOverride {
	return func(sa *corev1.ServiceAccount) {
		sa.OwnerReferences = owners
	}
}

// DeploymentOverride is a modifier option that overrides a Deployment.
type DeploymentOverride func(deployment *appsv1.Deployment)

// DeploymentWithOptionalName overrides the name of a Deployment if empty.
func DeploymentWithOptionalName(name string) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		if d.Name == "" {
			d.Name = name
		}
	}
}

// DeploymentWithNamespace overrides the namespace of a Deployment.
func DeploymentWithNamespace(namespace string) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		d.Namespace = namespace
	}
}

// DeploymentWithOptionalPodScrapeAnnotations adds Prometheus scrape annotations
// to a Deployment pod template if they are not already set.
func DeploymentWithOptionalPodScrapeAnnotations() DeploymentOverride {
	return func(d *appsv1.Deployment) {
		if d.Spec.Template.Annotations == nil {
			d.Spec.Template.Annotations = map[string]string{}
		}

		metricsPort := strconv.Itoa(MetricsPortNumber)

		for _, port := range d.Spec.Template.Spec.Containers[0].Ports {
			if port.Name == MetricsPortName {
				metricsPort = strconv.Itoa(int(port.ContainerPort))
				break
			}
		}

		if _, ok := d.Spec.Template.Annotations["prometheus.io/scrape"]; !ok {
			d.Spec.Template.Annotations["prometheus.io/scrape"] = "true"
			d.Spec.Template.Annotations["prometheus.io/port"] = metricsPort
			d.Spec.Template.Annotations["prometheus.io/path"] = "/metrics"
		}
	}
}

// DeploymentWithOwnerReferences overrides the owner references of a Deployment.
func DeploymentWithOwnerReferences(owners []metav1.OwnerReference) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		d.OwnerReferences = owners
	}
}

// DeploymentWithOptionalReplicas set the replicas if it is unset.
func DeploymentWithOptionalReplicas(replicas int32) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		if d.Spec.Replicas == nil {
			d.Spec.Replicas = &replicas
		}
	}
}

// DeploymentWithSelectors overrides the selectors of a Deployment. It also
// ensures that the pod template labels always contains the deployment selector.
func DeploymentWithSelectors(selectors map[string]string) DeploymentOverride {
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

// DeploymentWithOptionalServiceAccount overrides the service account of a Deployment.
func DeploymentWithOptionalServiceAccount(sa string) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		if d.Spec.Template.Spec.ServiceAccountName == "" {
			d.Spec.Template.Spec.ServiceAccountName = sa
		}
	}
}

// DeploymentWithImagePullSecrets overrides the image pull secrets of a
// Deployment.
func DeploymentWithImagePullSecrets(secrets []corev1.LocalObjectReference) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.ImagePullSecrets = secrets
	}
}

// DeploymentWithAdditionalPullSecret adds additional image pull secret to
// a Deployment.
func DeploymentWithAdditionalPullSecret(secret corev1.LocalObjectReference) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.ImagePullSecrets = append(d.Spec.Template.Spec.ImagePullSecrets, secret)
	}
}

// DeploymentRuntimeWithOptionalImage set the image for the runtime container if
// it is unset, e.g. not specified in the DeploymentRuntimeConfig. Note that if
// the image was already set, we use it exactly as is (i.e., no default registry).
func DeploymentRuntimeWithOptionalImage(image string) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		if d.Spec.Template.Spec.Containers[0].Image == "" {
			d.Spec.Template.Spec.Containers[0].Image = image
		}
	}
}

// DeploymentRuntimeWithOptionalImagePullPolicy set the image pull policy if it
// is unset.
func DeploymentRuntimeWithOptionalImagePullPolicy(policy corev1.PullPolicy) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		if d.Spec.Template.Spec.Containers[0].ImagePullPolicy == "" {
			d.Spec.Template.Spec.Containers[0].ImagePullPolicy = policy
		}
	}
}

// DeploymentRuntimeWithImagePullPolicy overrides the image pull policy of the
// runtime container of a Deployment.
func DeploymentRuntimeWithImagePullPolicy(policy corev1.PullPolicy) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].ImagePullPolicy = policy
	}
}

// DeploymentRuntimeWithTLSServerSecret mounts a TLS Server secret as a volume
// and sets the path of the mounted volume as an environment variable of the
// runtime container of a Deployment.
func DeploymentRuntimeWithTLSServerSecret(secret string) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		mountTLSSecret(secret, TLSServerCertsVolumeName, TLSServerCertsDir, TLSServerCertDirEnvVar, d)
	}
}

// DeploymentRuntimeWithTLSClientSecret mounts a TLS Client secret as a volume
// and sets the path of the mounted volume as an environment variable of the
// runtime container of a Deployment.
func DeploymentRuntimeWithTLSClientSecret(secret string) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		mountTLSSecret(secret, TLSClientCertsVolumeName, TLSClientCertsDir, TLSClientCertDirEnvVar, d)
	}
}

// DeploymentRuntimeWithAdditionalEnvironments adds additional environment
// variables to the runtime container of a Deployment.
func DeploymentRuntimeWithAdditionalEnvironments(env []corev1.EnvVar) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, env...)
	}
}

// DeploymentRuntimeWithAdditionalPorts adds additional ports to the runtime
// container of a Deployment. A port will be added only if a port of the same name doesn't already exist.
func DeploymentRuntimeWithAdditionalPorts(ports []corev1.ContainerPort) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		names := make(map[string]bool)
		for _, p := range d.Spec.Template.Spec.Containers[0].Ports {
			names[p.Name] = true
		}

		for _, p := range ports {
			if _, ok := names[p.Name]; ok {
				continue
			}

			d.Spec.Template.Spec.Containers[0].Ports = append(d.Spec.Template.Spec.Containers[0].Ports, p)
		}
	}
}

// DeploymentWithOptionalPodSecurityContext sets the pod security context if it
// is unset.
func DeploymentWithOptionalPodSecurityContext(podSecurityContext *corev1.PodSecurityContext) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		if d.Spec.Template.Spec.SecurityContext == nil {
			d.Spec.Template.Spec.SecurityContext = podSecurityContext
		}
	}
}

// DeploymentRuntimeWithOptionalSecurityContext sets the security context of the
// runtime container if it is unset.
func DeploymentRuntimeWithOptionalSecurityContext(securityContext *corev1.SecurityContext) DeploymentOverride {
	return func(d *appsv1.Deployment) {
		if d.Spec.Template.Spec.Containers[0].SecurityContext == nil {
			d.Spec.Template.Spec.Containers[0].SecurityContext = securityContext
		}
	}
}

// DeploymentWithRuntimeContainer ensures that the runtime container exists and
// is the first container.
func DeploymentWithRuntimeContainer() DeploymentOverride {
	return func(d *appsv1.Deployment) {
		for i := range d.Spec.Template.Spec.Containers {
			if d.Spec.Template.Spec.Containers[i].Name == ContainerName {
				if i == 0 {
					// Already the first container, done.
					return
				}
				// Move the runtime container to the first position
				rc := d.Spec.Template.Spec.Containers[i]
				d.Spec.Template.Spec.Containers = append([]corev1.Container{rc}, append(d.Spec.Template.Spec.Containers[:i], d.Spec.Template.Spec.Containers[i+1:]...)...)

				return
			}
		}

		// The runtime container does not exist, add it to the first position
		d.Spec.Template.Spec.Containers = append([]corev1.Container{
			{
				Name: ContainerName,
			},
		}, d.Spec.Template.Spec.Containers...)
	}
}

// ServiceOverride is a modifier option that overrides a Service.
type ServiceOverride func(service *corev1.Service)

// ServiceWithName overrides the name of a Service.
func ServiceWithName(name string) ServiceOverride {
	return func(s *corev1.Service) {
		s.Name = name
	}
}

// ServiceWithOptionalName overrides the name of a Service if empty.
func ServiceWithOptionalName(name string) ServiceOverride {
	return func(s *corev1.Service) {
		if s.Name == "" {
			s.Name = name
		}
	}
}

// ServiceWithNamespace overrides the namespace of a Service.
func ServiceWithNamespace(namespace string) ServiceOverride {
	return func(s *corev1.Service) {
		s.Namespace = namespace
	}
}

// ServiceWithOwnerReferences overrides the owner references of a Service.
func ServiceWithOwnerReferences(owners []metav1.OwnerReference) ServiceOverride {
	return func(s *corev1.Service) {
		s.OwnerReferences = owners
	}
}

// ServiceWithSelectors overrides the selectors of a Service.
func ServiceWithSelectors(selectors map[string]string) ServiceOverride {
	return func(s *corev1.Service) {
		s.Spec.Selector = selectors
	}
}

// ServiceWithAdditionalPorts adds additional ports to a Service if no port with the same name was found.
func ServiceWithAdditionalPorts(ports []corev1.ServicePort) ServiceOverride {
	return func(s *corev1.Service) {
		names := make(map[string]bool)
		for _, p := range s.Spec.Ports {
			names[p.Name] = true
		}

		for _, p := range ports {
			if _, ok := names[p.Name]; ok {
				continue
			}

			s.Spec.Ports = append(s.Spec.Ports, p)
		}
	}
}

// ServiceWithClusterIP overrides the cluster IP of a Service.
func ServiceWithClusterIP(clusterIP string) ServiceOverride {
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
	d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts, vm)

	envs := []corev1.EnvVar{
		{Name: envName, Value: mountPath},
	}
	d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, envs...)
}
