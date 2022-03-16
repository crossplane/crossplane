/*
Copyright 2021 The Crossplane Authors.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

type deploymentModifier func(*appsv1.Deployment)

func withPodTemplateLabels(labels map[string]string) deploymentModifier {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Labels = labels
	}
}

func withAdditionalVolume(v corev1.Volume) deploymentModifier {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, v)
	}
}

func withAdditionalVolumeMount(vm corev1.VolumeMount) deploymentModifier {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts, vm)
	}
}

func withAdditionalEnvVar(env corev1.EnvVar) deploymentModifier {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, env)
	}
}

func withAdditionalPort(port corev1.ContainerPort) deploymentModifier {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Ports = append(d.Spec.Template.Spec.Containers[0].Ports, port)
	}
}

const (
	namespace = "ns"
)

func serviceaccount(rev v1.PackageRevision) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rev.GetName(),
			Namespace: namespace,
		},
	}
}

func service(provider *pkgmetav1.Provider, rev v1.PackageRevision) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rev.GetName(),
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			// We use whatever is on the deployment so that ControllerConfig
			// overrides are accounted for.
			Selector: map[string]string{
				"pkg.crossplane.io/revision": rev.GetName(),
				"pkg.crossplane.io/provider": provider.GetName(),
			},
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       9443,
					TargetPort: intstr.FromInt(9443),
				},
			},
		},
	}
}

func deployment(provider *pkgmetav1.Provider, revision string, img string, modifiers ...deploymentModifier) *appsv1.Deployment {
	var (
		replicas = int32(1)
	)

	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      revision,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"pkg.crossplane.io/revision": revision,
					"pkg.crossplane.io/provider": provider.GetName(),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      provider.GetName(),
					Namespace: namespace,
					Labels: map[string]string{
						"pkg.crossplane.io/revision": revision,
						"pkg.crossplane.io/provider": provider.GetName(),
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: revision,
					Containers: []corev1.Container{
						{
							Name:            provider.GetName(),
							Image:           img,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          promPortName,
									ContainerPort: promPortNumber,
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
							},
						},
					},
				},
			},
		},
	}

	for _, modifier := range modifiers {
		modifier(d)
	}

	return d
}

func TestBuildProviderDeployment(t *testing.T) {
	type args struct {
		provider *pkgmetav1.Provider
		revision *v1.ProviderRevision
		cc       *v1alpha1.ControllerConfig
	}
	type want struct {
		sa  *corev1.ServiceAccount
		d   *appsv1.Deployment
		svc *corev1.Service
	}

	img := "img:tag"
	pkgImg := "pkg-img:tag"
	ccImg := "cc-img:tag"
	webhookTLSSecretName := "secret-name"

	providerWithoutImage := &pkgmetav1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pkg",
		},
		Spec: pkgmetav1.ProviderSpec{
			Controller: pkgmetav1.ControllerSpec{},
		},
	}

	providerWithImage := &pkgmetav1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pkg",
		},
		Spec: pkgmetav1.ProviderSpec{
			Controller: pkgmetav1.ControllerSpec{
				Image: &img,
			},
		},
	}

	revisionWithoutCC := &v1.ProviderRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rev-123",
		},
		Spec: v1.PackageRevisionSpec{
			ControllerConfigReference: nil,
			Package:                   pkgImg,
			Revision:                  3,
		},
	}

	revisionWithoutCCWithWebhook := &v1.ProviderRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rev-123",
		},
		Spec: v1.PackageRevisionSpec{
			ControllerConfigReference: nil,
			Package:                   pkgImg,
			Revision:                  3,
			WebhookTLSSecretName:      &webhookTLSSecretName,
		},
	}

	revisionWithCC := &v1.ProviderRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rev-123",
		},
		Spec: v1.PackageRevisionSpec{
			ControllerConfigReference: &xpv1.Reference{Name: "cc"},
			Package:                   pkgImg,
			Revision:                  3,
		},
	}

	cc := &v1alpha1.ControllerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: revisionWithCC.Name,
		},
		Spec: v1alpha1.ControllerConfigSpec{
			Metadata: &v1alpha1.PodObjectMeta{
				Labels: map[string]string{
					"k": "v",
				},
			},
			Image: &ccImg,
		},
	}

	cases := map[string]struct {
		reason string
		fields args
		want   want
	}{
		"NoImgNoCC": {
			reason: "If the meta provider does not specify a controller image and no ControllerConfig is referenced, the package image itself should be used.",
			fields: args{
				provider: providerWithoutImage,
				revision: revisionWithoutCC,
				cc:       nil,
			},
			want: want{
				sa:  serviceaccount(revisionWithoutCC),
				d:   deployment(providerWithoutImage, revisionWithCC.GetName(), pkgImg),
				svc: service(providerWithoutImage, revisionWithoutCC),
			},
		},
		"ImgNoCCWithWebhookTLS": {
			reason: "If the webhook tls secret name is given, then the deployment should be configured to serve behind the given service.",
			fields: args{
				provider: providerWithImage,
				revision: revisionWithoutCCWithWebhook,
				cc:       nil,
			},
			want: want{
				sa: serviceaccount(revisionWithoutCCWithWebhook),
				d: deployment(providerWithImage, revisionWithoutCCWithWebhook.GetName(), img,
					withAdditionalVolume(corev1.Volume{
						Name: webhookVolumeName,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: webhookTLSSecretName,
								Items: []corev1.KeyToPath{
									{Key: "tls.crt", Path: "tls.crt"},
									{Key: "tls.key", Path: "tls.key"},
								},
							},
						},
					}),
					withAdditionalVolumeMount(corev1.VolumeMount{
						Name:      webhookVolumeName,
						ReadOnly:  true,
						MountPath: webhookTLSCertDir,
					}),
					withAdditionalEnvVar(corev1.EnvVar{Name: webhookTLSCertDirEnvVar, Value: webhookTLSCertDir}),
					withAdditionalPort(corev1.ContainerPort{Name: webhookPortName, ContainerPort: webhookPort}),
				),
				svc: service(providerWithImage, revisionWithoutCCWithWebhook),
			},
		},
		"ImgNoCC": {
			reason: "If the meta provider specifies a controller image and no ControllerConfig is reference, the specified image should be used.",
			fields: args{
				provider: providerWithImage,
				revision: revisionWithoutCC,
				cc:       nil,
			},
			want: want{
				sa:  serviceaccount(revisionWithoutCC),
				d:   deployment(providerWithoutImage, revisionWithoutCC.GetName(), img),
				svc: service(providerWithoutImage, revisionWithoutCC),
			},
		},
		"ImgCC": {
			reason: "If a ControllerConfig is referenced and it species a controller image it should always be used.",
			fields: args{
				provider: providerWithImage,
				revision: revisionWithCC,
				cc:       cc,
			},
			want: want{
				sa: serviceaccount(revisionWithCC),
				d: deployment(providerWithImage, revisionWithCC.GetName(), ccImg, withPodTemplateLabels(map[string]string{
					"pkg.crossplane.io/revision": revisionWithCC.GetName(),
					"pkg.crossplane.io/provider": providerWithImage.GetName(),
					"k":                          "v",
				})),
				svc: service(providerWithImage, revisionWithCC),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sa, d, svc := buildProviderDeployment(tc.fields.provider, tc.fields.revision, tc.fields.cc, namespace)

			if diff := cmp.Diff(tc.want.sa, sa, cmpopts.IgnoreTypes([]metav1.OwnerReference{})); diff != "" {
				t.Errorf("-want, +got:\n%s\n", diff)
			}
			if diff := cmp.Diff(tc.want.d, d, cmpopts.IgnoreTypes(&corev1.SecurityContext{}, &corev1.PodSecurityContext{}, []metav1.OwnerReference{})); diff != "" {
				t.Errorf("-want, +got:\n%s\n", diff)
			}
			if diff := cmp.Diff(tc.want.svc, svc, cmpopts.IgnoreTypes([]metav1.OwnerReference{})); diff != "" {
				t.Errorf("-want, +got:\n%s\n", diff)
			}
		})
	}

}
