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
