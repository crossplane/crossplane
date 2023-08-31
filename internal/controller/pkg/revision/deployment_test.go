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

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
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

func serviceFunction(function *pkgmetav1beta1.Function, rev v1.PackageRevision) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      function.GetName(),
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			// We use whatever is on the deployment so that ControllerConfig
			// overrides are accounted for.
			Selector: map[string]string{
				"pkg.crossplane.io/revision": rev.GetName(),
				"pkg.crossplane.io/function": function.GetName(),
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

func secretServer(rev v1.PackageRevision) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      *rev.GetTLSServerSecretName(),
			Namespace: namespace,
		},
	}
}

func secretClient(rev v1.PackageRevision) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      *rev.GetTLSClientSecretName(),
			Namespace: namespace,
		},
	}
}

func deploymentProvider(provider *pkgmetav1.Provider, revision string, img string, modifiers ...deploymentModifier) *appsv1.Deployment {
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
								{
									Name:  "TLS_SERVER_CERTS_DIR",
									Value: "/tls/server",
								},
								{
									Name:  "TLS_CLIENT_CERTS_DIR",
									Value: "/tls/client",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "tls-server-certs",
									ReadOnly:  true,
									MountPath: "/tls/server",
								},
								{
									Name:      "tls-client-certs",
									ReadOnly:  true,
									MountPath: "/tls/client",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "tls-server-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "server-secret-name",
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
							Name: "tls-client-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "client-secret-name",
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

	for _, modifier := range modifiers {
		modifier(d)
	}

	return d
}

func deploymentFunction(function *pkgmetav1beta1.Function, revision string, img string, modifiers ...deploymentModifier) *appsv1.Deployment {
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
					"pkg.crossplane.io/function": function.GetName(),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      function.GetName(),
					Namespace: namespace,
					Labels: map[string]string{
						"pkg.crossplane.io/revision": revision,
						"pkg.crossplane.io/function": function.GetName(),
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: revision,
					Containers: []corev1.Container{
						{
							Name:            function.GetName(),
							Image:           img,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          promPortName,
									ContainerPort: promPortNumber,
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
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "tls-server-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "server-secret-name",
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
		ss  *corev1.Secret
		cs  *corev1.Secret
	}

	img := "img:tag"
	pkgImg := "pkg-img:tag"
	ccImg := "cc-img:tag"
	webhookTLSSecretName := "secret-name"
	tlsServerSecretName := "server-secret-name"
	tlsClientSecretName := "client-secret-name"

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
			TLSServerSecretName:       &tlsServerSecretName,
			TLSClientSecretName:       &tlsClientSecretName,
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
			TLSServerSecretName:       &tlsServerSecretName,
			TLSClientSecretName:       &tlsClientSecretName,
		},
	}

	revisionWithCC := &v1.ProviderRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rev-123",
		},
		Spec: v1.PackageRevisionSpec{
			ControllerConfigReference: &v1.ControllerConfigReference{Name: "cc"},
			Package:                   pkgImg,
			Revision:                  3,
			TLSServerSecretName:       &tlsServerSecretName,
			TLSClientSecretName:       &tlsClientSecretName,
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

	ccWithVolumes := &v1alpha1.ControllerConfig{
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
			Volumes: []corev1.Volume{
				{Name: "vol-a"},
				{Name: "vol-b"},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "vm-a"},
				{Name: "vm-b"},
			},
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
				d:   deploymentProvider(providerWithoutImage, revisionWithCC.GetName(), pkgImg),
				svc: service(providerWithoutImage, revisionWithoutCC),
				ss:  secretServer(revisionWithoutCC),
				cs:  secretClient(revisionWithoutCC),
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
				d: deploymentProvider(providerWithImage, revisionWithoutCCWithWebhook.GetName(), img,
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
					withAdditionalPort(corev1.ContainerPort{Name: webhookPortName, ContainerPort: servicePort}),
				),
				svc: service(providerWithImage, revisionWithoutCCWithWebhook),
				ss:  secretServer(revisionWithoutCC),
				cs:  secretClient(revisionWithoutCC),
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
				d:   deploymentProvider(providerWithoutImage, revisionWithoutCC.GetName(), img),
				svc: service(providerWithoutImage, revisionWithoutCC),
				ss:  secretServer(revisionWithoutCC),
				cs:  secretClient(revisionWithoutCC),
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
				d: deploymentProvider(providerWithImage, revisionWithCC.GetName(), ccImg, withPodTemplateLabels(map[string]string{
					"pkg.crossplane.io/revision": revisionWithCC.GetName(),
					"pkg.crossplane.io/provider": providerWithImage.GetName(),
					"k":                          "v",
				})),
				svc: service(providerWithImage, revisionWithCC),
				ss:  secretServer(revisionWithoutCC),
				cs:  secretClient(revisionWithoutCC),
			},
		},
		"WithVolumes": {
			reason: "If a ControllerConfig is referenced and it contains volumes and volumeMounts.",
			fields: args{
				provider: providerWithImage,
				revision: revisionWithCC,
				cc:       ccWithVolumes,
			},
			want: want{
				sa: serviceaccount(revisionWithCC),
				d: deploymentProvider(providerWithImage, revisionWithCC.GetName(), ccImg, withPodTemplateLabels(map[string]string{
					"pkg.crossplane.io/revision": revisionWithCC.GetName(),
					"pkg.crossplane.io/provider": providerWithImage.GetName(),
					"k":                          "v"}),
					withAdditionalVolume(corev1.Volume{Name: "vol-a"}),
					withAdditionalVolume(corev1.Volume{Name: "vol-b"}),
					withAdditionalVolumeMount(corev1.VolumeMount{Name: "vm-a"}),
					withAdditionalVolumeMount(corev1.VolumeMount{Name: "vm-b"}),
				),
				svc: service(providerWithImage, revisionWithCC),
				ss:  secretServer(revisionWithoutCC),
				cs:  secretClient(revisionWithoutCC),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sa, d, svc, ss, cs := buildProviderDeployment(tc.fields.provider, tc.fields.revision, tc.fields.cc, namespace, nil)

			if diff := cmp.Diff(tc.want.sa, sa, cmpopts.IgnoreTypes([]metav1.OwnerReference{})); diff != "" {
				t.Errorf("-want, +got:\n%s\n", diff)
			}
			if diff := cmp.Diff(tc.want.d, d, cmpopts.IgnoreTypes(&corev1.SecurityContext{}, &corev1.PodSecurityContext{}, []metav1.OwnerReference{})); diff != "" {
				t.Errorf("-want, +got:\n%s\n", diff)
			}
			if diff := cmp.Diff(tc.want.svc, svc, cmpopts.IgnoreTypes([]metav1.OwnerReference{})); diff != "" {
				t.Errorf("-want, +got:\n%s\n", diff)
			}
			if diff := cmp.Diff(tc.want.ss, ss, cmpopts.IgnoreTypes([]metav1.OwnerReference{})); diff != "" {
				t.Errorf("-want, +got:\n%s\n", diff)
			}
			if diff := cmp.Diff(tc.want.cs, cs, cmpopts.IgnoreTypes([]metav1.OwnerReference{})); diff != "" {
				t.Errorf("-want, +got:\n%s\n", diff)
			}
		})
	}

}

func TestBuildFunctionDeployment(t *testing.T) {
	type args struct {
		function *pkgmetav1beta1.Function
		revision *v1beta1.FunctionRevision
		cc       *v1alpha1.ControllerConfig
	}
	type want struct {
		sa  *corev1.ServiceAccount
		d   *appsv1.Deployment
		svc *corev1.Service
		sec *corev1.Secret
	}

	img := "img:tag"
	pkgImg := "pkg-img:tag"
	ccImg := "cc-img:tag"
	tlsServerSecretName := "server-secret-name"
	tlsClientSecretName := "client-secret-name"

	functionWithoutImage := &pkgmetav1beta1.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pkg",
		},
		Spec: pkgmetav1beta1.FunctionSpec{
			Image: nil,
		},
	}

	functionWithImage := &pkgmetav1beta1.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pkg",
		},
		Spec: pkgmetav1beta1.FunctionSpec{
			Image: &img,
		},
	}

	revisionWithoutCC := &v1beta1.FunctionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rev-123",
			Labels: map[string]string{
				"pkg.crossplane.io/package": "pkg",
			},
		},
		Spec: v1.PackageRevisionSpec{
			ControllerConfigReference: nil,
			Package:                   pkgImg,
			Revision:                  3,
			TLSServerSecretName:       &tlsServerSecretName,
			TLSClientSecretName:       &tlsClientSecretName,
		},
	}

	revisionWithCC := &v1beta1.FunctionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rev-123",
			Labels: map[string]string{
				"pkg.crossplane.io/package": "pkg",
			},
		},
		Spec: v1.PackageRevisionSpec{
			ControllerConfigReference: &v1.ControllerConfigReference{Name: "cc"},
			Package:                   pkgImg,
			Revision:                  3,
			TLSServerSecretName:       &tlsServerSecretName,
			TLSClientSecretName:       &tlsClientSecretName,
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

	ccWithVolumes := &v1alpha1.ControllerConfig{
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
			Volumes: []corev1.Volume{
				{Name: "vol-a"},
				{Name: "vol-b"},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "vm-a"},
				{Name: "vm-b"},
			},
		},
	}

	cases := map[string]struct {
		reason string
		fields args
		want   want
	}{
		"NoImgNoCC": {
			reason: "If the meta function does not specify a controller image and no ControllerConfig is referenced, the package image itself should be used.",
			fields: args{
				function: functionWithoutImage,
				revision: revisionWithoutCC,
				cc:       nil,
			},
			want: want{
				sa:  serviceaccount(revisionWithoutCC),
				d:   deploymentFunction(functionWithoutImage, revisionWithoutCC.GetName(), pkgImg),
				svc: serviceFunction(functionWithoutImage, revisionWithoutCC),
				sec: secretServer(revisionWithoutCC),
			},
		},
		"ImgNoCC": {
			reason: "If the meta function specifies a controller image and no ControllerConfig is reference, the specified image should be used.",
			fields: args{
				function: functionWithImage,
				revision: revisionWithoutCC,
				cc:       nil,
			},
			want: want{
				sa:  serviceaccount(revisionWithoutCC),
				d:   deploymentFunction(functionWithoutImage, revisionWithoutCC.GetName(), img),
				svc: serviceFunction(functionWithoutImage, revisionWithoutCC),
				sec: secretServer(revisionWithoutCC),
			},
		},
		"ImgCC": {
			reason: "If a ControllerConfig is referenced and it species a controller image it should always be used.",
			fields: args{
				function: functionWithImage,
				revision: revisionWithCC,
				cc:       cc,
			},
			want: want{
				sa: serviceaccount(revisionWithCC),
				d: deploymentFunction(functionWithImage, revisionWithCC.GetName(), ccImg, withPodTemplateLabels(map[string]string{
					"pkg.crossplane.io/revision": revisionWithCC.GetName(),
					"pkg.crossplane.io/function": functionWithImage.GetName(),
					"k":                          "v",
				})),
				svc: serviceFunction(functionWithImage, revisionWithCC),
				sec: secretServer(revisionWithoutCC),
			},
		},
		"WithVolumes": {
			reason: "If a ControllerConfig is referenced and it contains volumes and volumeMounts.",
			fields: args{
				function: functionWithImage,
				revision: revisionWithCC,
				cc:       ccWithVolumes,
			},
			want: want{
				sa: serviceaccount(revisionWithCC),
				d: deploymentFunction(functionWithImage, revisionWithCC.GetName(), ccImg, withPodTemplateLabels(map[string]string{
					"pkg.crossplane.io/revision": revisionWithCC.GetName(),
					"pkg.crossplane.io/function": functionWithImage.GetName(),
					"k":                          "v"}),
					withAdditionalVolume(corev1.Volume{Name: "vol-a"}),
					withAdditionalVolume(corev1.Volume{Name: "vol-b"}),
					withAdditionalVolumeMount(corev1.VolumeMount{Name: "vm-a"}),
					withAdditionalVolumeMount(corev1.VolumeMount{Name: "vm-b"}),
				),
				svc: serviceFunction(functionWithImage, revisionWithCC),
				sec: secretServer(revisionWithoutCC),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sa, d, svc, sec := buildFunctionDeployment(tc.fields.function, tc.fields.revision, tc.fields.cc, namespace, nil)

			if diff := cmp.Diff(tc.want.sa, sa, cmpopts.IgnoreTypes([]metav1.OwnerReference{})); diff != "" {
				t.Errorf("-want, +got:\n%s\n", diff)
			}
			if diff := cmp.Diff(tc.want.d, d, cmpopts.IgnoreTypes(&corev1.SecurityContext{}, &corev1.PodSecurityContext{}, []metav1.OwnerReference{})); diff != "" {
				t.Errorf("-want, +got:\n%s\n", diff)
			}
			if diff := cmp.Diff(tc.want.svc, svc, cmpopts.IgnoreTypes([]metav1.OwnerReference{})); diff != "" {
				t.Errorf("-want, +got:\n%s\n", diff)
			}
			if diff := cmp.Diff(tc.want.sec, sec, cmpopts.IgnoreTypes([]metav1.OwnerReference{})); diff != "" {
				t.Errorf("-want, +got:\n%s\n", diff)
			}
		})
	}

}
