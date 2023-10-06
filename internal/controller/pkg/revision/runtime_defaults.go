package revision

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	replicas                 = int32(1)
	runAsUser                = int64(2000)
	runAsGroup               = int64(2000)
	allowPrivilegeEscalation = false
	privileged               = false
	runAsNonRoot             = true
)

func defaultServiceAccount(name string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			// It is possible to override the name of the service account in the
			// RuntimeConfig. So, we define it as a default here.
			Name: name,
		},
	}
}

func defaultDeployment(name string) *appsv1.Deployment {
	// TODO(turkenh): Implement configurable defaults.
	// See https://github.com/crossplane/crossplane/issues/4699#issuecomment-1748403479
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			// It is possible to override the name of the deployment in the
			// RuntimeConfig. So, we define it as a default here.
			Name: name,
		},
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

func defaultService(name string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			// It is possible to override the name of the service in the
			// RuntimeConfig. So, we define it as a default here.
			Name: name,
		},
	}
}
