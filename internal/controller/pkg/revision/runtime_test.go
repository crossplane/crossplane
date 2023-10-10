package revision

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

const (
	namespace = "crossplane-system"

	providerImage        = "crossplane/provider-foo:v1.2.3"
	providerName         = "provider-foo"
	providerRevisionName = "provider-foo-1234"

	functionImage        = "crossplane/function-foo:v1.2.3"
	functionName         = "function-foo"
	functionRevisionName = "function-foo-1234"

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
		},
		Spec: v1.ProviderRevisionSpec{
			PackageRevisionSpec: v1.PackageRevisionSpec{
				Package: providerImage,
			},
			PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
				TLSServerSecretName: pointer.String(tlsServerSecretName),
				TLSClientSecretName: pointer.String(tlsClientSecretName),
			},
		},
	}

	functionRevision = &v1beta1.FunctionRevision{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "pkg.crossplane.io/v1beta1",
			Kind:       "FunctionRevision",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: functionRevisionName,
			Labels: map[string]string{
				v1.LabelParentPackage: functionName,
			},
		},
		Spec: v1beta1.FunctionRevisionSpec{
			PackageRevisionSpec: v1.PackageRevisionSpec{
				Package: functionImage,
			},
			PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
				TLSServerSecretName: pointer.String(tlsServerSecretName),
			},
		},
	}
)

func TestRuntimeManifestBuilder_Deployment(t *testing.T) {
	type args struct {
		builder            ManifestBuilder
		overrides          []DeploymentOverrides
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
		"ProviderDeploymentNoControllerConfig": {
			reason: "No overrides should result in a deployment with default values",
			args: args{
				builder: &RuntimeManifestBuilder{
					revision:  providerRevision,
					namespace: namespace,
				},
				serviceAccountName: providerRevisionName,
				overrides:          providerDeploymentOverrides(&pkgmetav1.Provider{}, providerRevision),
			},
			want: want{
				want: deploymentProvider(providerName, providerRevisionName, providerImage),
			},
		},
		"ProviderDeploymentWithControllerConfig": {
			reason: "Overrides from the controller config should be applied to the deployment",
			args: args{
				builder: &RuntimeManifestBuilder{
					revision:  providerRevision,
					namespace: namespace,
					controllerConfig: &v1alpha1.ControllerConfig{
						Spec: v1alpha1.ControllerConfigSpec{
							Replicas: pointer.Int32(3),
							Metadata: &v1alpha1.PodObjectMeta{
								Labels: map[string]string{
									"k": "v",
								},
							},
							Image: pointer.String("crossplane/provider-foo:v1.2.4"),
							Volumes: []corev1.Volume{
								{Name: "vol-a"},
								{Name: "vol-b"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "vm-a"},
								{Name: "vm-b"},
							},
						},
					},
				},
				serviceAccountName: providerRevisionName,
				overrides:          providerDeploymentOverrides(&pkgmetav1.Provider{}, providerRevision),
			},
			want: want{
				want: deploymentProvider(providerName, providerRevisionName, providerImage, func(deployment *appsv1.Deployment) {
					deployment.Spec.Replicas = pointer.Int32(3)
					deployment.Spec.Template.Labels["k"] = "v"
					deployment.Spec.Template.Spec.Containers[0].Image = "crossplane/provider-foo:v1.2.4"
					deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{Name: "vol-a"}, corev1.Volume{Name: "vol-b"})
					deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{Name: "vm-a"}, corev1.VolumeMount{Name: "vm-b"})
				}),
			},
		},
		"FunctionDeploymentNoControllerConfig": {
			reason: "No overrides should result in a deployment with default values",
			args: args{
				builder: &RuntimeManifestBuilder{
					revision:  functionRevision,
					namespace: namespace,
				},
				serviceAccountName: functionRevisionName,
				overrides:          functionDeploymentOverrides(&pkgmetav1beta1.Function{}, functionRevision),
			},
			want: want{
				want: deploymentFunction(functionName, functionRevisionName, functionImage),
			},
		},
		"FunctionDeploymentWithControllerConfig": {
			reason: "Overrides from the controller config should be applied to the deployment",
			args: args{
				builder: &RuntimeManifestBuilder{
					revision:  functionRevision,
					namespace: namespace,
					controllerConfig: &v1alpha1.ControllerConfig{
						Spec: v1alpha1.ControllerConfigSpec{
							Replicas: pointer.Int32(3),
						},
					},
				},
				serviceAccountName: functionRevisionName,
				overrides:          functionDeploymentOverrides(&pkgmetav1beta1.Function{}, functionRevision),
			},
			want: want{
				want: deploymentFunction(functionName, functionRevisionName, functionImage, func(deployment *appsv1.Deployment) {
					deployment.Spec.Replicas = pointer.Int32(3)
				}),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.args.builder.Deployment(tc.args.serviceAccountName, tc.args.overrides...)
			if diff := cmp.Diff(tc.want.want, got); diff != "" {
				t.Errorf("\n%s\nDeployment(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func deploymentProvider(provider string, revision string, image string, overrides ...DeploymentOverrides) *appsv1.Deployment {
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      revision,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "pkg.crossplane.io/v1",
					Kind:               "ProviderRevision",
					Name:               revision,
					Controller:         pointer.Bool(true),
					BlockOwnerDeletion: pointer.Bool(true),
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"pkg.crossplane.io/revision": revision,
					"pkg.crossplane.io/provider": provider,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"pkg.crossplane.io/revision": revision,
						"pkg.crossplane.io/provider": provider,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: revision,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
						RunAsUser:    &runAsUser,
						RunAsGroup:   &runAsGroup,
					},
					Containers: []corev1.Container{
						{
							Name:            runtimeContainerName,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          metricsPortName,
									ContainerPort: metricsPortNumber,
								},
								{
									Name:          webhookPortName,
									ContainerPort: servicePort,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name:  "ESS_TLS_CERTS_DIR",
									Value: "$(TLS_CLIENT_CERTS_DIR)",
								},
								{
									Name:  "WEBHOOK_TLS_CERT_DIR",
									Value: "$(TLS_SERVER_CERTS_DIR)",
								},
								{
									Name:  "TLS_CLIENT_CERTS_DIR",
									Value: "/tls/client",
								},
								{
									Name:  "TLS_SERVER_CERTS_DIR",
									Value: "/tls/server",
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
								RunAsUser:                &runAsUser,
								RunAsGroup:               &runAsGroup,
								AllowPrivilegeEscalation: &allowPrivilegeEscalation,
								Privileged:               &privileged,
								RunAsNonRoot:             &runAsNonRoot,
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

func deploymentFunction(function string, revision string, image string, overrides ...DeploymentOverrides) *appsv1.Deployment {
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      revision,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "pkg.crossplane.io/v1beta1",
					Kind:               "FunctionRevision",
					Name:               revision,
					Controller:         pointer.Bool(true),
					BlockOwnerDeletion: pointer.Bool(true),
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"pkg.crossplane.io/revision": revision,
					"pkg.crossplane.io/function": function,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"pkg.crossplane.io/revision": revision,
						"pkg.crossplane.io/function": function,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: revision,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
						RunAsUser:    &runAsUser,
						RunAsGroup:   &runAsGroup,
					},
					Containers: []corev1.Container{
						{
							Name:            runtimeContainerName,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          metricsPortName,
									ContainerPort: metricsPortNumber,
								},
								{
									Name:          grpcPortName,
									ContainerPort: servicePort,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "TLS_SERVER_CERTS_DIR",
									Value: "/tls/server",
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
								RunAsUser:                &runAsUser,
								RunAsGroup:               &runAsGroup,
								AllowPrivilegeEscalation: &allowPrivilegeEscalation,
								Privileged:               &privileged,
								RunAsNonRoot:             &runAsNonRoot,
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

// MockManifestBuilder is a mock implementation of ManifestBuilder.
type MockManifestBuilder struct {
	ServiceAccountFn  func(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount
	DeploymentFn      func(serviceAccount string, overrides ...DeploymentOverrides) *appsv1.Deployment
	ServiceFn         func(overrides ...ServiceOverrides) *corev1.Service
	TLSClientSecretFn func() *corev1.Secret
	TLSServerSecretFn func() *corev1.Secret
}

// ServiceAccount returns the result of calling ServiceAccountFn.
func (b *MockManifestBuilder) ServiceAccount(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount {
	return b.ServiceAccountFn(overrides...)
}

// Deployment returns the result of calling DeploymentFn.
func (b *MockManifestBuilder) Deployment(serviceAccount string, overrides ...DeploymentOverrides) *appsv1.Deployment {
	return b.DeploymentFn(serviceAccount, overrides...)
}

// Service returns the result of calling ServiceFn.
func (b *MockManifestBuilder) Service(overrides ...ServiceOverrides) *corev1.Service {
	return b.ServiceFn(overrides...)
}

// TLSClientSecret returns the result of calling TLSClientSecretFn.
func (b *MockManifestBuilder) TLSClientSecret() *corev1.Secret {
	return b.TLSClientSecretFn()
}

// TLSServerSecret returns the result of calling TLSServerSecretFn.
func (b *MockManifestBuilder) TLSServerSecret() *corev1.Secret {
	return b.TLSServerSecretFn()
}
