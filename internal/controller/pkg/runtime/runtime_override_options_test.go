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
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestDeploymentWithRuntimeContainer(t *testing.T) {
	type args struct {
		deployment *appsv1.Deployment
	}
	type want struct {
		deployment *appsv1.Deployment
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoContainers": {
			reason: "Should not add the runtime container if there are no containers",
			args: args{
				deployment: &appsv1.Deployment{},
			},
			want: want{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: ContainerName,
									},
								},
							},
						},
					},
				},
			},
		},
		"AlreadyFirstAndOnlyContainer": {
			reason: "Should do nothing if the runtime container is already the first and only container",
			args: args{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: ContainerName,
									},
								},
							},
						},
					},
				},
			},
			want: want{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: ContainerName,
									},
								},
							},
						},
					},
				},
			},
		},
		"AddedToExistingContainers": {
			reason: "Should not add the container to the existing containers as the first container",
			args: args{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "some-other-container",
									},
								},
							},
						},
					},
				},
			},
			want: want{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: ContainerName,
									},
									{
										Name: "some-other-container",
									},
								},
							},
						},
					},
				},
			},
		},
		"ExistButInWrongPlace": {
			reason: "Should move the runtime container to the first container position if it exists but is not the first container",
			args: args{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "some-other-container",
									},
									{
										Name: ContainerName,
									},
									{
										Name: "another-one",
									},
								},
							},
						},
					},
				},
			},
			want: want{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: ContainerName,
									},
									{
										Name: "some-other-container",
									},
									{
										Name: "another-one",
									},
								},
							},
						},
					},
				},
			},
		},
		"ExistAtTheEnd": {
			reason: "Should move the runtime container to the first container position if it exists as the last container",
			args: args{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "some-other-container",
									},
									{
										Name: "another-one",
									},
									{
										Name: ContainerName,
									},
								},
							},
						},
					},
				},
			},
			want: want{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: ContainerName,
									},
									{
										Name: "some-other-container",
									},
									{
										Name: "another-one",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			DeploymentWithRuntimeContainer()(tc.args.deployment)
			if diff := cmp.Diff(tc.want.deployment, tc.args.deployment); diff != "" {
				t.Errorf("\n%s\nDeploymentWithRuntimeContainer(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDeploymentRuntimeWithAdditionalPorts(t *testing.T) {
	type args struct {
		deployment *appsv1.Deployment
		ports      []corev1.ContainerPort
	}
	type want struct {
		deployment *appsv1.Deployment
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoPorts": {
			reason: "Should add the given ports if no ports are set",
			args: args{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{},
								},
							},
						},
					},
				},
				ports: []corev1.ContainerPort{
					{ContainerPort: 80, Name: "http"},
					{ContainerPort: 443, Name: "https"},
				},
			},
			want: want{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Ports: []corev1.ContainerPort{
											{ContainerPort: 80, Name: "http"},
											{ContainerPort: 443, Name: "https"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"DontOverridePorts": {
			reason: "Should add only new ports and not override existing ports",
			args: args{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Ports: []corev1.ContainerPort{
											{ContainerPort: 8080, Name: "http"},
										},
									},
								},
							},
						},
					},
				},
				ports: []corev1.ContainerPort{
					{ContainerPort: 80, Name: "http"},
					{ContainerPort: 443, Name: "https"},
				},
			},
			want: want{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Ports: []corev1.ContainerPort{
											{ContainerPort: 8080, Name: "http"},
											{ContainerPort: 443, Name: "https"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			DeploymentRuntimeWithAdditionalPorts(tc.args.ports)(tc.args.deployment)
			if diff := cmp.Diff(tc.want.deployment, tc.args.deployment); diff != "" {
				t.Errorf("\n%s\nDeploymentRuntimeWithAdditionalPorts(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
