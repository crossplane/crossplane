/*
Copyright 2020 The Crossplane Authors.

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

package containerized

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/crossplane/crossplane-runtime/pkg/resource"

	oamv1alpha2 "github.com/crossplane/crossplane/apis/oam/v1alpha2"
)

// Reconcile error strings.
const (
	errNotContainerizedWorkload = "object is not a containerized workload"
)

const defaultNamespace = "default"

const labelKey = "containerizedworkload.oam.crossplane.io"

var (
	deploymentKind       = reflect.TypeOf(appsv1.Deployment{}).Name()
	deploymentAPIVersion = appsv1.SchemeGroupVersion.String()
)

// Translator translates a ContainerizedWorkload into a Deployment.
// nolint:gocyclo
func Translator(ctx context.Context, w resource.Workload) ([]resource.Object, error) {
	cw, ok := w.(*oamv1alpha2.ContainerizedWorkload)
	if !ok {
		return nil, errors.New(errNotContainerizedWorkload)
	}

	d := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       deploymentKind,
			APIVersion: deploymentAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: cw.GetName(),
			// NOTE(hasheddan): we always create the Deployment in the default
			// namespace because there is not currently a namespace scheduling
			// mechanism in the Crossplane OAM implementation. It is likely that
			// this will be addressed in the future by adding a Scope.
			Namespace: defaultNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					labelKey: string(cw.GetUID()),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						labelKey: string(cw.GetUID()),
					},
				},
			},
		},
	}
	if cw.Spec.OperatingSystem != nil {
		if d.Spec.Template.Spec.NodeSelector == nil {
			d.Spec.Template.Spec.NodeSelector = map[string]string{}
		}
		d.Spec.Template.Spec.NodeSelector["beta.kubernetes.io/os"] = string(*cw.Spec.OperatingSystem)
	}

	if cw.Spec.CPUArchitecture != nil {
		if d.Spec.Template.Spec.NodeSelector == nil {
			d.Spec.Template.Spec.NodeSelector = map[string]string{}
		}
		d.Spec.Template.Spec.NodeSelector["kubernetes.io/arch"] = string(*cw.Spec.CPUArchitecture)
	}

	for _, container := range cw.Spec.Containers {
		if container.ImagePullSecret != nil {
			d.Spec.Template.Spec.ImagePullSecrets = append(d.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{
				Name: *container.ImagePullSecret,
			})
		}
		kubernetesContainer := corev1.Container{
			Name:    container.Name,
			Image:   container.Image,
			Command: container.Command,
			Args:    container.Arguments,
		}

		if container.Resources != nil {
			kubernetesContainer.Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    container.Resources.CPU.Required,
					corev1.ResourceMemory: container.Resources.Memory.Required,
				},
			}
			for _, v := range container.Resources.Volumes {
				mount := corev1.VolumeMount{
					Name:      v.Name,
					MountPath: v.MouthPath,
				}
				if v.AccessMode != nil && *v.AccessMode == oamv1alpha2.VolumeAccessModeRO {
					mount.ReadOnly = true
				}
				kubernetesContainer.VolumeMounts = append(kubernetesContainer.VolumeMounts, mount)

			}
		}

		for _, p := range container.Ports {
			port := corev1.ContainerPort{
				Name:          p.Name,
				ContainerPort: p.Port,
			}
			if p.Protocol != nil {
				port.Protocol = corev1.Protocol(*p.Protocol)
			}
			kubernetesContainer.Ports = append(kubernetesContainer.Ports, port)
		}

		for _, e := range container.Environment {
			if e.Value != nil {
				kubernetesContainer.Env = append(kubernetesContainer.Env, corev1.EnvVar{
					Name:  e.Name,
					Value: *e.Value,
				})
				continue
			}
			if e.FromSecret != nil {
				kubernetesContainer.Env = append(kubernetesContainer.Env, corev1.EnvVar{
					Name: e.Name,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: e.FromSecret.Key,
							LocalObjectReference: corev1.LocalObjectReference{
								Name: e.FromSecret.Name,
							},
						},
					},
				})
			}
		}

		if container.LivenessProbe != nil {
			kubernetesContainer.LivenessProbe = &corev1.Probe{}
			if container.LivenessProbe.InitialDelaySeconds != nil {
				kubernetesContainer.LivenessProbe.InitialDelaySeconds = *container.LivenessProbe.InitialDelaySeconds
			}
			if container.LivenessProbe.TimeoutSeconds != nil {
				kubernetesContainer.LivenessProbe.TimeoutSeconds = *container.LivenessProbe.TimeoutSeconds
			}
			if container.LivenessProbe.PeriodSeconds != nil {
				kubernetesContainer.LivenessProbe.PeriodSeconds = *container.LivenessProbe.PeriodSeconds
			}
			if container.LivenessProbe.SuccessThreshold != nil {
				kubernetesContainer.LivenessProbe.SuccessThreshold = *container.LivenessProbe.SuccessThreshold
			}
			if container.LivenessProbe.FailureThreshold != nil {
				kubernetesContainer.LivenessProbe.FailureThreshold = *container.LivenessProbe.FailureThreshold
			}

			// NOTE(hasheddan): Kubernetes specifies that only one type of
			// handler should be provided. OAM does not impose that same
			// restriction. We optimistically check all and set whatever is
			// provided.
			if container.LivenessProbe.HTTPGet != nil {
				kubernetesContainer.LivenessProbe.Handler.HTTPGet = &corev1.HTTPGetAction{
					Path: container.LivenessProbe.HTTPGet.Path,
					Port: intstr.IntOrString{IntVal: container.LivenessProbe.HTTPGet.Port},
				}

				for _, h := range container.LivenessProbe.HTTPGet.HTTPHeaders {
					kubernetesContainer.LivenessProbe.Handler.HTTPGet.HTTPHeaders = append(kubernetesContainer.LivenessProbe.Handler.HTTPGet.HTTPHeaders, corev1.HTTPHeader{
						Name:  h.Name,
						Value: h.Value,
					})
				}
			}
			if container.LivenessProbe.Exec != nil {
				kubernetesContainer.LivenessProbe.Exec = &corev1.ExecAction{
					Command: container.LivenessProbe.Exec.Command,
				}
			}
			if container.LivenessProbe.TCPSocket != nil {
				kubernetesContainer.LivenessProbe.TCPSocket = &corev1.TCPSocketAction{
					Port: intstr.IntOrString{IntVal: container.LivenessProbe.TCPSocket.Port},
				}
			}
		}

		if container.ReadinessProbe != nil {
			kubernetesContainer.ReadinessProbe = &corev1.Probe{}
			if container.ReadinessProbe.InitialDelaySeconds != nil {
				kubernetesContainer.ReadinessProbe.InitialDelaySeconds = *container.ReadinessProbe.InitialDelaySeconds
			}
			if container.ReadinessProbe.TimeoutSeconds != nil {
				kubernetesContainer.ReadinessProbe.TimeoutSeconds = *container.ReadinessProbe.TimeoutSeconds
			}
			if container.ReadinessProbe.PeriodSeconds != nil {
				kubernetesContainer.ReadinessProbe.PeriodSeconds = *container.ReadinessProbe.PeriodSeconds
			}
			if container.ReadinessProbe.SuccessThreshold != nil {
				kubernetesContainer.ReadinessProbe.SuccessThreshold = *container.ReadinessProbe.SuccessThreshold
			}
			if container.ReadinessProbe.FailureThreshold != nil {
				kubernetesContainer.ReadinessProbe.FailureThreshold = *container.ReadinessProbe.FailureThreshold
			}

			// NOTE(hasheddan): Kubernetes specifies that only one type of
			// handler should be provided. OAM does not impose that same
			// restriction. We optimistically check all and set whatever is
			// provided.
			if container.ReadinessProbe.HTTPGet != nil {
				kubernetesContainer.ReadinessProbe.Handler.HTTPGet = &corev1.HTTPGetAction{
					Path: container.ReadinessProbe.HTTPGet.Path,
					Port: intstr.IntOrString{IntVal: container.ReadinessProbe.HTTPGet.Port},
				}

				for _, h := range container.ReadinessProbe.HTTPGet.HTTPHeaders {
					kubernetesContainer.ReadinessProbe.Handler.HTTPGet.HTTPHeaders = append(kubernetesContainer.ReadinessProbe.Handler.HTTPGet.HTTPHeaders, corev1.HTTPHeader{
						Name:  h.Name,
						Value: h.Value,
					})
				}
			}
			if container.ReadinessProbe.Exec != nil {
				kubernetesContainer.ReadinessProbe.Exec = &corev1.ExecAction{
					Command: container.ReadinessProbe.Exec.Command,
				}
			}
			if container.ReadinessProbe.TCPSocket != nil {
				kubernetesContainer.ReadinessProbe.TCPSocket = &corev1.TCPSocketAction{
					Port: intstr.IntOrString{IntVal: container.ReadinessProbe.TCPSocket.Port},
				}
			}
		}

		d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, kubernetesContainer)
	}

	return []resource.Object{d}, nil
}
