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
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	extv1alpha1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	pkgmetav1 "github.com/crossplane/crossplane/apis/v2/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/controller/pkg/revision"
)

const (
	namespace = "crossplane-system"

	providerImage        = "xpkg.crossplane.io/crossplane/provider-foo:v1.2.3"
	providerName         = "crossplane-provider-foo"
	providerRevisionName = "provider-foo-1234"
	providerRevisionUID  = "12345678-1234-1234-1234-123456789012"

	functionImage        = "xpkg.crossplane.io/crossplane/function-foo:v1.2.3"
	functionName         = "function-foo"
	functionRevisionName = "function-foo-1234"
	functionRevisionUID  = "98765432-1234-1234-1234-210987654321"

	tlsServerSecretName = "tls-server-secret"
	tlsClientSecretName = "tls-client-secret"
)

var (
	providerRevision = &v1.ProviderRevision{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "pkg.crossplane.io/v1",
			Kind:       "ProviderRevision",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: providerRevisionName,
			Labels: map[string]string{
				v1.LabelParentPackage: providerName,
			},
			UID: types.UID(providerRevisionUID),
		},
		Spec: v1.ProviderRevisionSpec{
			PackageRevisionSpec: v1.PackageRevisionSpec{
				Package: providerImage,
			},
		},
		Status: v1.ProviderRevisionStatus{
			PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
				TLSServerSecretName: new(tlsServerSecretName),
				TLSClientSecretName: new(tlsClientSecretName),
			},
		},
	}

	functionRevision = &v1.FunctionRevision{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "pkg.crossplane.io/v1beta1",
			Kind:       "FunctionRevision",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: functionRevisionName,
			Labels: map[string]string{
				v1.LabelParentPackage: functionName,
			},
			UID: types.UID(functionRevisionUID),
		},
		Spec: v1.FunctionRevisionSpec{
			PackageRevisionSpec: v1.PackageRevisionSpec{
				Package: functionImage,
			},
		},
		Status: v1.FunctionRevisionStatus{
			PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
				TLSServerSecretName: new(tlsServerSecretName),
			},
		},
	}

	// An incoming (active) revision that will claim ownership of the objects shared by other revisions of the package.
	incoming = metav1.OwnerReference{Name: "incoming", UID: "incoming-uid", Controller: new(true), BlockOwnerDeletion: new(true)}
	// an outgoing (inactive) revision that will be demoted/removed from ownership of shared objects.
	outgoing = metav1.OwnerReference{Name: "outgoing", UID: "outgoing-uid", Controller: new(true), BlockOwnerDeletion: new(true)}
	// the same outgoing (inactive) revision that has now been demoted from ownership of shared objects.
	demoted = metav1.OwnerReference{Name: "outgoing", UID: "outgoing-uid", Controller: new(false), BlockOwnerDeletion: new(true)}
)

func TestProviderDeployment(t *testing.T) {
	type args struct {
		revision           v1.PackageRevisionWithRuntime
		runtimeConfig      *v1beta1.DeploymentRuntimeConfig
		serviceAccountName string
		awaitingActivation bool
	}

	type want struct {
		want *appsv1.Deployment
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ProviderDeploymentNoScrapeAnnotation": {
			reason: "It should be possible to disable default scrape annotations",
			args: args{
				revision: providerRevision,
				runtimeConfig: &v1beta1.DeploymentRuntimeConfig{
					Spec: v1beta1.DeploymentRuntimeConfigSpec{
						DeploymentTemplate: &v1beta1.DeploymentTemplate{
							Spec: &appsv1.DeploymentSpec{
								Template: corev1.PodTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Annotations: map[string]string{
											"prometheus.io/scrape": "false",
										},
									},
									Spec: corev1.PodSpec{},
								},
							},
						},
					},
				},
				serviceAccountName: providerRevisionName,
			},
			want: want{
				want: deploymentProvider(providerName, providerRevisionName, providerImage, DeploymentWithSelectors(map[string]string{
					v1.LabelProvider: providerName,
					v1.LabelRevision: providerRevisionName,
				}), func(deployment *appsv1.Deployment) {
					deployment.Spec.Template.Annotations = map[string]string{
						"prometheus.io/scrape": "false",
					}
				}),
			},
		},
		"ProviderDeploymentScaleToZero": {
			reason: "Awaiting activation should scale the deployment to zero replicas",
			args: args{
				revision:           providerRevision,
				awaitingActivation: true,
				serviceAccountName: providerRevisionName,
			},
			want: want{
				want: deploymentProvider(providerName, providerRevisionName, providerImage, DeploymentWithSelectors(map[string]string{
					v1.LabelProvider: providerName,
					v1.LabelRevision: providerRevisionName,
				}), func(deployment *appsv1.Deployment) {
					deployment.Spec.Replicas = ptr.To[int32](0)
				}),
			},
		},
		"ProviderDeploymentScaleToZeroWithRuntimeConfigReplicas": {
			reason: "Awaiting activation should scale to zero even when the runtime config sets an explicit replica count",
			args: args{
				revision:           providerRevision,
				awaitingActivation: true,
				runtimeConfig: &v1beta1.DeploymentRuntimeConfig{
					Spec: v1beta1.DeploymentRuntimeConfigSpec{
						DeploymentTemplate: &v1beta1.DeploymentTemplate{
							Spec: &appsv1.DeploymentSpec{
								Replicas: ptr.To[int32](3),
							},
						},
					},
				},
				serviceAccountName: providerRevisionName,
			},
			want: want{
				want: deploymentProvider(providerName, providerRevisionName, providerImage, DeploymentWithSelectors(map[string]string{
					v1.LabelProvider: providerName,
					v1.LabelRevision: providerRevisionName,
				}), func(deployment *appsv1.Deployment) {
					deployment.Spec.Replicas = ptr.To[int32](0)
				}),
			},
		},
		"ProviderDeploymentWithAdvancedRuntimeConfig": {
			reason: "Baseline provided by the runtime config should be applied to the deployment for advanced use cases",
			args: args{
				revision: providerRevision,
				runtimeConfig: &v1beta1.DeploymentRuntimeConfig{
					Spec: v1beta1.DeploymentRuntimeConfigSpec{
						DeploymentTemplate: &v1beta1.DeploymentTemplate{
							Metadata: &v1beta1.ObjectMeta{
								Name: new("my-provider-foo"),
								Labels: map[string]string{
									"x": "y",
								},
								Annotations: map[string]string{
									"foo": "bar",
								},
							},
							Spec: &appsv1.DeploymentSpec{
								Replicas: ptr.To[int32](3),
								Template: corev1.PodTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											"k": "v",
										},
									},
									Spec: corev1.PodSpec{
										Volumes: []corev1.Volume{
											{Name: "vol-a"},
											{Name: "vol-b"},
										},
										Containers: []corev1.Container{
											{
												Name:  "sidecar",
												Image: "sidecar/sidecar:v1.0.0",
											},
											{
												Name:  ContainerName,
												Image: "crossplane/provider-foo:v1.2.4",
												VolumeMounts: []corev1.VolumeMount{
													{Name: "vm-a"},
													{Name: "vm-b"},
												},
												Resources: corev1.ResourceRequirements{
													Requests: corev1.ResourceList{
														"cpu":    resource.MustParse("1"),
														"memory": resource.MustParse("1Gi"),
													},
													Limits: corev1.ResourceList{
														"cpu":    resource.MustParse("2"),
														"memory": resource.MustParse("2Gi"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				serviceAccountName: providerRevisionName,
			},
			want: want{
				want: deploymentProvider(providerName, providerRevisionName, providerImage, DeploymentWithSelectors(map[string]string{
					v1.LabelProvider: providerName,
					v1.LabelRevision: providerRevisionName,
				}), func(deployment *appsv1.Deployment) {
					deployment.Name = "my-provider-foo"
					deployment.Labels = map[string]string{
						"x": "y",
					}
					deployment.Annotations = map[string]string{
						"foo": "bar",
					}
					deployment.Spec.Replicas = ptr.To[int32](3)
					deployment.Spec.Template.Labels["k"] = "v"
					deployment.Spec.Template.Spec.Containers[0].Image = "crossplane/provider-foo:v1.2.4"
					deployment.Spec.Template.Spec.Volumes = append([]corev1.Volume{{Name: "vol-a"}, {Name: "vol-b"}}, deployment.Spec.Template.Spec.Volumes...)
					deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append([]corev1.VolumeMount{{Name: "vm-a"}, {Name: "vm-b"}}, deployment.Spec.Template.Spec.Containers[0].VolumeMounts...)
					deployment.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu":    resource.MustParse("1"),
							"memory": resource.MustParse("1Gi"),
						},
						Limits: corev1.ResourceList{
							"cpu":    resource.MustParse("2"),
							"memory": resource.MustParse("2Gi"),
						},
					}
					deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, corev1.Container{
						Name:  "sidecar",
						Image: "sidecar/sidecar:v1.0.0",
					})
				}),
			},
		},
		"ProviderDeploymentWithRuntimeConfig": {
			reason: "Baseline provided by the runtime config should be applied to the deployment",
			args: args{
				revision: providerRevision,
				runtimeConfig: &v1beta1.DeploymentRuntimeConfig{
					Spec: v1beta1.DeploymentRuntimeConfigSpec{
						DeploymentTemplate: &v1beta1.DeploymentTemplate{
							Spec: &appsv1.DeploymentSpec{
								Replicas: ptr.To[int32](3),
								Template: corev1.PodTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											"k": "v",
										},
									},
									Spec: corev1.PodSpec{
										Volumes: []corev1.Volume{
											{Name: "vol-a"},
											{Name: "vol-b"},
										},
										Containers: []corev1.Container{
											{
												Name:  ContainerName,
												Image: "crossplane/provider-foo:v1.2.4",
												VolumeMounts: []corev1.VolumeMount{
													{Name: "vm-a"},
													{Name: "vm-b"},
												},
												Ports: []corev1.ContainerPort{
													{ContainerPort: 7070, Name: MetricsPortName},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				serviceAccountName: providerRevisionName,
			},
			want: want{
				want: deploymentProvider(providerName, providerRevisionName, providerImage, DeploymentWithSelectors(map[string]string{
					v1.LabelProvider: providerName,
					v1.LabelRevision: providerRevisionName,
				}), func(deployment *appsv1.Deployment) {
					deployment.Spec.Replicas = ptr.To[int32](3)
					deployment.Spec.Template.Labels["k"] = "v"
					deployment.Spec.Template.Spec.Containers[0].Image = "crossplane/provider-foo:v1.2.4"
					deployment.Spec.Template.Spec.Volumes = append([]corev1.Volume{{Name: "vol-a"}, {Name: "vol-b"}}, deployment.Spec.Template.Spec.Volumes...)
					deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append([]corev1.VolumeMount{{Name: "vm-a"}, {Name: "vm-b"}}, deployment.Spec.Template.Spec.Containers[0].VolumeMounts...)
					deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = 7070
					deployment.Spec.Template.Annotations["prometheus.io/port"] = "7070"
				}),
			},
		},
		"ProviderDeploymentWithoutRuntimeConfig": {
			reason: "No overrides should result in a deployment with default values",
			args: args{
				revision:           providerRevision,
				serviceAccountName: providerRevisionName,
			},
			want: want{
				want: deploymentProvider(providerName, providerRevisionName, providerImage, DeploymentWithSelectors(map[string]string{
					v1.LabelProvider: providerName,
					v1.LabelRevision: providerRevisionName,
				})),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := NewProviderHooks(nil, namespace, crossplaneName, nil).deployment(tc.args.revision, tc.args.runtimeConfig, tc.args.serviceAccountName, providerImage, nil, tc.args.awaitingActivation)
			if diff := cmp.Diff(tc.want.want, got); diff != "" {
				t.Errorf("\n%s\ndeployment(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestFunctionDeployment(t *testing.T) {
	type args struct {
		revision           v1.PackageRevisionWithRuntime
		runtimeConfig      *v1beta1.DeploymentRuntimeConfig
		serviceAccountName string
	}

	type want struct {
		want *appsv1.Deployment
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"FunctionDeploymentWithoutRuntimeConfig": {
			reason: "No overrides should result in a deployment with default values",
			args: args{
				revision:           functionRevision,
				serviceAccountName: functionRevisionName,
			},
			want: want{
				want: deploymentFunction(functionName, functionRevisionName, functionImage),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := NewFunctionHooks(nil, namespace, crossplaneName).deployment(tc.args.revision, tc.args.runtimeConfig, tc.args.serviceAccountName, functionImage, nil)
			if diff := cmp.Diff(tc.want.want, got); diff != "" {
				t.Errorf("\n%s\ndeployment(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestProviderService(t *testing.T) {
	type args struct {
		revision      v1.PackageRevisionWithRuntime
		runtimeConfig *v1beta1.DeploymentRuntimeConfig
	}

	type want struct {
		want *corev1.Service
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ProviderServiceNoRuntimeConfig": {
			reason: "No runtime config should result in a service with default values",
			args: args{
				revision: providerRevision,
			},
			want: want{
				want: &corev1.Service{
					TypeMeta: metav1.TypeMeta{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "Service",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      providerName,
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "pkg.crossplane.io/v1",
								Kind:               "ProviderRevision",
								Name:               providerRevisionName,
								UID:                types.UID(providerRevisionUID),
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
							},
						},
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							v1.LabelProvider: providerName,
							v1.LabelRevision: providerRevisionName,
						},
						Ports: []corev1.ServicePort{
							{
								Name:       WebhookPortName,
								Port:       int32(revision.ServicePort),
								TargetPort: intstr.FromString(WebhookPortName),
								Protocol:   corev1.ProtocolTCP,
							},
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := NewProviderHooks(nil, namespace, crossplaneName, nil).service(tc.args.revision, tc.args.runtimeConfig)
			if diff := cmp.Diff(tc.want.want, got); diff != "" {
				t.Errorf("\n%s\nservice(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func deploymentProvider(provider string, rev string, image string, overrides ...DeploymentOverride) *appsv1.Deployment {
	d := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      rev,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "pkg.crossplane.io/v1",
					Kind:               "ProviderRevision",
					Name:               rev,
					UID:                types.UID(providerRevisionUID),
					Controller:         new(true),
					BlockOwnerDeletion: new(true),
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1.LabelRevision: rev,
					v1.LabelProvider: provider,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   "8080",
						"prometheus.io/path":   "/metrics",
					},
					Labels: map[string]string{
						v1.LabelRevision: rev,
						v1.LabelProvider: provider,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: rev,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &RunAsNonRoot,
						RunAsUser:    &RunAsUser,
						RunAsGroup:   &RunAsGroup,
					},
					Containers: []corev1.Container{
						{
							Name:            ContainerName,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          MetricsPortName,
									ContainerPort: MetricsPortNumber,
								},
								{
									Name:          WebhookPortName,
									ContainerPort: revision.ServicePort,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "TLS_CLIENT_CERTS_DIR",
									Value: "/tls/client",
								},
								{
									Name:  "TLS_SERVER_CERTS_DIR",
									Value: "/tls/server",
								},
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name: "PROVIDER_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: fmt.Sprintf("metadata.labels['%s']", v1.LabelProvider),
										},
									},
								},
								{
									Name: "REVISION_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: fmt.Sprintf("metadata.labels['%s']", v1.LabelRevision),
										},
									},
								},
								{
									Name:  "REVISION_UID",
									Value: providerRevisionUID,
								},
								{
									Name:  "WEBHOOK_TLS_CERT_DIR",
									Value: "$(TLS_SERVER_CERTS_DIR)",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "tls-client-certs",
									ReadOnly:  true,
									MountPath: "/tls/client",
								},
								{
									Name:      "tls-server-certs",
									ReadOnly:  true,
									MountPath: "/tls/server",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:                &RunAsUser,
								RunAsGroup:               &RunAsGroup,
								AllowPrivilegeEscalation: &AllowPrivilegeEscalation,
								Privileged:               &Privileged,
								RunAsNonRoot:             &RunAsNonRoot,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "tls-client-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: tlsClientSecretName,
									Items: []corev1.KeyToPath{
										{
											Key:  "tls.crt",
											Path: "tls.crt",
										},
										{
											Key:  "tls.key",
											Path: "tls.key",
										},
										{
											Key:  "ca.crt",
											Path: "ca.crt",
										},
									},
								},
							},
						},
						{
							Name: "tls-server-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: tlsServerSecretName,
									Items: []corev1.KeyToPath{
										{
											Key:  "tls.crt",
											Path: "tls.crt",
										},
										{
											Key:  "tls.key",
											Path: "tls.key",
										},
										{
											Key:  "ca.crt",
											Path: "ca.crt",
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

	for _, o := range overrides {
		o(d)
	}

	return d
}

func deploymentFunction(function string, rev string, image string, overrides ...DeploymentOverride) *appsv1.Deployment {
	d := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      rev,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "pkg.crossplane.io/v1beta1",
					Kind:               "FunctionRevision",
					Name:               rev,
					UID:                types.UID(functionRevisionUID),
					Controller:         new(true),
					BlockOwnerDeletion: new(true),
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1.LabelRevision: rev,
					v1.LabelFunction: function,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.LabelRevision: rev,
						v1.LabelFunction: function,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: rev,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &RunAsNonRoot,
						RunAsUser:    &RunAsUser,
						RunAsGroup:   &RunAsGroup,
					},
					Containers: []corev1.Container{
						{
							Name:            ContainerName,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          MetricsPortName,
									ContainerPort: MetricsPortNumber,
								},
								{
									Name:          GRPCPortName,
									ContainerPort: revision.ServicePort,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "TLS_SERVER_CERTS_DIR",
									Value: "/tls/server",
								},
								{
									Name: "FUNCTION_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: fmt.Sprintf("metadata.labels['%s']", v1.LabelFunction),
										},
									},
								},
								{
									Name: "REVISION_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: fmt.Sprintf("metadata.labels['%s']", v1.LabelRevision),
										},
									},
								},
								{
									Name:  "REVISION_UID",
									Value: functionRevisionUID,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "tls-server-certs",
									ReadOnly:  true,
									MountPath: "/tls/server",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:                &RunAsUser,
								RunAsGroup:               &RunAsGroup,
								AllowPrivilegeEscalation: &AllowPrivilegeEscalation,
								Privileged:               &Privileged,
								RunAsNonRoot:             &RunAsNonRoot,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "tls-server-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: tlsServerSecretName,
									Items: []corev1.KeyToPath{
										{
											Key:  "tls.crt",
											Path: "tls.crt",
										},
										{
											Key:  "tls.key",
											Path: "tls.key",
										},
										{
											Key:  "ca.crt",
											Path: "ca.crt",
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

	for _, o := range overrides {
		o(d)
	}

	return d
}

func TestDemotedControllers(t *testing.T) {
	owner := &metav1.ObjectMeta{UID: incoming.UID}

	cases := map[string]struct {
		reason string
		refs   []metav1.OwnerReference
		want   []metav1.OwnerReference
	}{
		"NewController": {
			reason: "Should return empty when the only controller is the new owner.",
			refs:   []metav1.OwnerReference{incoming},
		},
		"NewDemotesOld": {
			reason: "Should demote the old controller when a new revision takes ownership.",
			refs:   []metav1.OwnerReference{outgoing},
			want:   []metav1.OwnerReference{demoted},
		},
		"MixedReferences": {
			reason: "Should demote only old controllers; skip incoming and non-controlling refs.",
			refs:   []metav1.OwnerReference{incoming, outgoing, demoted},
			want:   []metav1.OwnerReference{demoted},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			obj := &corev1.Service{ObjectMeta: metav1.ObjectMeta{OwnerReferences: tc.refs}}
			got := demotedControllers(obj, owner)
			if len(got) == 0 {
				got = nil
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\ndemotedControllers(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAwaitingActivation(t *testing.T) {
	inactiveMRD := extv1alpha1.ManagedResourceDefinition{
		Spec: extv1alpha1.ManagedResourceDefinitionSpec{
			State: extv1alpha1.ManagedResourceDefinitionInactive,
		},
	}

	safeStartRevision := func() *v1.ProviderRevision {
		pr := &v1.ProviderRevision{}
		pr.SetCapabilities([]string{pkgmetav1.ProviderCapabilitySafeStart})
		return pr
	}

	safeStartRevisionActive := func() *v1.ProviderRevision {
		pr := safeStartRevision()
		pr.SetConditions(v1.RuntimeActive())
		return pr
	}

	cases := map[string]struct {
		revision        v1.PackageRevisionWithRuntime
		mrds            []extv1alpha1.ManagedResourceDefinition
		runtimeConfig   *v1beta1.DeploymentRuntimeConfig
		wantScaleToZero bool
	}{
		"NoSafeStartCapability": {
			revision:        &v1.ProviderRevision{},
			mrds:            []extv1alpha1.ManagedResourceDefinition{inactiveMRD},
			wantScaleToZero: false,
		},
		"NoMRDs": {
			revision:        safeStartRevision(),
			mrds:            nil,
			wantScaleToZero: false,
		},
		"ActiveMRD": {
			revision: safeStartRevision(),
			mrds: []extv1alpha1.ManagedResourceDefinition{{
				Spec: extv1alpha1.ManagedResourceDefinitionSpec{State: extv1alpha1.ManagedResourceDefinitionActive},
			}},
			wantScaleToZero: false,
		},
		"AllInactiveMRDs": {
			revision:        safeStartRevision(),
			mrds:            []extv1alpha1.ManagedResourceDefinition{inactiveMRD},
			wantScaleToZero: true,
		},
		"AlreadyActivatedRuntimeWithInactiveMRDs": {
			// Once TypeRuntimeActive is True the runtime must not be scaled
			// back to zero, even if MRDs later appear inactive (e.g. via a
			// manual edit). This guards the one-way activation latch.
			revision:        safeStartRevisionActive(),
			mrds:            []extv1alpha1.ManagedResourceDefinition{inactiveMRD},
			wantScaleToZero: false,
		},
		"RuntimeConfigWithExplicitReplicas": {
			// Awaiting activation scales the runtime to zero regardless of an
			// explicit replica count in the DeploymentRuntimeConfig. The
			// configured count only takes effect once the runtime is activated.
			revision: safeStartRevision(),
			mrds:     []extv1alpha1.ManagedResourceDefinition{inactiveMRD},
			runtimeConfig: &v1beta1.DeploymentRuntimeConfig{
				Spec: v1beta1.DeploymentRuntimeConfigSpec{
					DeploymentTemplate: &v1beta1.DeploymentTemplate{
						Spec: &appsv1.DeploymentSpec{
							Replicas: ptr.To[int32](2),
						},
					},
				},
			},
			wantScaleToZero: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := awaitingActivation(tc.revision, tc.mrds)
			if diff := cmp.Diff(tc.wantScaleToZero, got); diff != "" {
				t.Errorf("awaitingActivation(...): -want, +got:\n%s", diff)
			}

			// The replica count in a runtime config must not stop the
			// runtime being scaled to zero while awaiting activation.
			d := NewProviderHooks(nil, namespace, crossplaneName, nil).deployment(tc.revision, tc.runtimeConfig, "", "", nil, got)
			if got && ptr.Deref(d.Spec.Replicas, -1) != 0 {
				t.Errorf("deployment(...): want 0 replicas while awaiting activation, got %v", d.Spec.Replicas)
			}
		})
	}
}

func TestFunctionService(t *testing.T) {
	type args struct {
		revision      v1.PackageRevisionWithRuntime
		runtimeConfig *v1beta1.DeploymentRuntimeConfig
	}

	type want struct {
		want *corev1.Service
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"FunctionServiceNoRuntimeConfig": {
			reason: "A function service should be headless and serve gRPC.",
			args: args{
				revision: functionRevision,
			},
			want: want{
				want: &corev1.Service{
					TypeMeta: metav1.TypeMeta{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "Service",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      functionName,
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "pkg.crossplane.io/v1beta1",
								Kind:               "FunctionRevision",
								Name:               functionRevisionName,
								UID:                types.UID(functionRevisionUID),
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
							},
						},
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: corev1.ClusterIPNone,
						Selector: map[string]string{
							v1.LabelFunction: functionName,
							v1.LabelRevision: functionRevisionName,
						},
						Ports: []corev1.ServicePort{
							{
								Name:        GRPCPortName,
								Protocol:    corev1.ProtocolTCP,
								Port:        GRPCPort,
								TargetPort:  intstr.FromString(GRPCPortName),
								AppProtocol: &AppProtocolTLS,
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := NewFunctionHooks(nil, namespace, crossplaneName).service(tc.args.revision, tc.args.runtimeConfig)
			if diff := cmp.Diff(tc.want.want, got); diff != "" {
				t.Errorf("\n%s\nservice(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCorePullSecrets(t *testing.T) {
	errBoom := errors.New("boom")

	type want struct {
		secrets []corev1.LocalObjectReference
		err     error
	}

	cases := map[string]struct {
		reason string
		client client.Client
		want   want
	}{
		"ErrGetServiceAccount": {
			reason: "We should return an error if we can't get the core Crossplane service account.",
			client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
			want:   want{err: errors.Wrap(errBoom, errGetServiceAccount)},
		},
		"NoPullSecrets": {
			reason: "We should return nothing if the core Crossplane service account has no pull secrets.",
			client: &test.MockClient{MockGet: test.NewMockGetFn(nil)},
			want:   want{},
		},
		"PullSecrets": {
			reason: "We should return the core Crossplane service account's pull secrets.",
			client: &test.MockClient{MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
				o.(*corev1.ServiceAccount).ImagePullSecrets = []corev1.LocalObjectReference{{Name: "core-secret"}}
				return nil
			})},
			want: want{secrets: []corev1.LocalObjectReference{{Name: "core-secret"}}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := corePullSecrets(context.Background(), tc.client, namespace, "crossplane")
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ncorePullSecrets(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.secrets, got); diff != "" {
				t.Errorf("\n%s\ncorePullSecrets(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestImageConfigPullSecrets(t *testing.T) {
	errBoom := errors.New("boom")

	withRefs := func(refs ...v1.ImageConfigRef) *v1.ProviderRevision {
		pr := &v1.ProviderRevision{}
		pr.SetAppliedImageConfigRefs(refs...)
		return pr
	}

	type want struct {
		secrets []string
		err     error
	}

	cases := map[string]struct {
		reason   string
		client   client.Client
		revision v1.PackageRevisionWithRuntime
		want     want
	}{
		"NoAppliedConfigs": {
			reason:   "We should return nothing if no image config set a pull secret.",
			client:   &test.MockClient{},
			revision: withRefs(v1.ImageConfigRef{Name: "some-config", Reason: v1.ImageConfigReasonRuntime}),
			want:     want{},
		},
		"ErrGetImageConfig": {
			reason:   "We should return an error if we can't get the applied image config.",
			client:   &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
			revision: withRefs(v1.ImageConfigRef{Name: "some-config", Reason: v1.ImageConfigReasonSetPullSecret}),
			want:     want{err: errors.Wrap(errBoom, errGetPullConfig)},
		},
		"PullSecret": {
			reason: "We should return the pull secret named by the applied image config.",
			client: &test.MockClient{MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
				o.(*v1beta1.ImageConfig).Spec.Registry = &v1beta1.RegistryConfig{
					Authentication: &v1beta1.RegistryAuthentication{
						PullSecretRef: corev1.LocalObjectReference{Name: "pull-secret"},
					},
				}
				return nil
			})},
			revision: withRefs(v1.ImageConfigRef{Name: "some-config", Reason: v1.ImageConfigReasonSetPullSecret}),
			want:     want{secrets: []string{"pull-secret"}},
		},
		"EmptyPullSecretName": {
			reason: "We should return nothing if the applied image config names an empty pull secret.",
			client: &test.MockClient{MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
				o.(*v1beta1.ImageConfig).Spec.Registry = &v1beta1.RegistryConfig{
					Authentication: &v1beta1.RegistryAuthentication{
						PullSecretRef: corev1.LocalObjectReference{Name: ""},
					},
				}
				return nil
			})},
			revision: withRefs(v1.ImageConfigRef{Name: "some-config", Reason: v1.ImageConfigReasonSetPullSecret}),
			want:     want{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := imageConfigPullSecrets(context.Background(), tc.client, tc.revision)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nimageConfigPullSecrets(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.secrets, got); diff != "" {
				t.Errorf("\n%s\nimageConfigPullSecrets(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
