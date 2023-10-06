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

package revision

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

var (
	replicas                 = int32(1)
	runAsUser                = int64(2000)
	runAsGroup               = int64(2000)
	allowPrivilegeEscalation = false
	privileged               = false
	runAsNonRoot             = true
)

const (
	// Providers are expected to use port 8080 if they expose Prometheus
	// metrics, which any provider built using controller-runtime will do by
	// default.
	metricsPortName   = "metrics"
	metricsPortNumber = 8080

	webhookTLSCertDirEnvVar = "WEBHOOK_TLS_CERT_DIR"
	webhookPortName         = "webhook"

	// See https://github.com/grpc/grpc/blob/v1.58.0/doc/naming.md
	grpcPortName       = "grpc"
	servicePort        = 9443
	serviceEndpointFmt = "dns:///%s.%s:%d"

	essTLSCertDirEnvVar = "ESS_TLS_CERTS_DIR"

	tlsServerCertDirEnvVar   = "TLS_SERVER_CERTS_DIR"
	tlsServerCertsVolumeName = "tls-server-certs"
	tlsServerCertsDir        = "/tls/server"

	tlsClientCertDirEnvVar   = "TLS_CLIENT_CERTS_DIR"
	tlsClientCertsVolumeName = "tls-client-certs"
	tlsClientCertsDir        = "/tls/client"
)

func buildProviderSecrets(revision v1.PackageWithRuntimeRevision, namespace string) (serverSec *corev1.Secret, clientSec *corev1.Secret) {
	if tlsServerSecretName := revision.GetTLSServerSecretName(); tlsServerSecretName != nil {
		serverSec = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            *tlsServerSecretName,
				Namespace:       namespace,
				OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, revision.GetObjectKind().GroupVersionKind()))},
			},
		}
	}

	if tlsClientSecretName := revision.GetTLSClientSecretName(); tlsClientSecretName != nil {
		clientSec = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            *tlsClientSecretName,
				Namespace:       namespace,
				OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, revision.GetObjectKind().GroupVersionKind()))},
			},
		}
	}
	return serverSec, clientSec
}

func buildProviderService(provider *pkgmetav1.Provider, revision v1.PackageRevision, namespace string) *corev1.Service {
	return getService(
		revision.GetLabels()[v1.LabelParentPackage],
		namespace,
		[]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, revision.GetObjectKind().GroupVersionKind()))},
		buildProviderServiceLabelSelector(provider, revision),
	)
}

func buildProviderServiceLabelSelector(provider *pkgmetav1.Provider, revision v1.PackageRevision) map[string]string {
	return map[string]string{
		"pkg.crossplane.io/revision": revision.GetName(),
		"pkg.crossplane.io/provider": provider.GetName(),
	}
}

// Returns the service account, deployment, service, server and client TLS secrets of the provider.
func buildProviderDeployment(provider *pkgmetav1.Provider, revision v1.PackageWithRuntimeRevision, cc *v1alpha1.ControllerConfig, namespace string, pullSecrets []corev1.LocalObjectReference) (*corev1.ServiceAccount, *appsv1.Deployment) {
	s := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            revision.GetName(),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, revision.GetObjectKind().GroupVersionKind()))},
		},
		ImagePullSecrets: pullSecrets,
	}

	pullPolicy := corev1.PullIfNotPresent
	if revision.GetPackagePullPolicy() != nil {
		pullPolicy = *revision.GetPackagePullPolicy()
	}
	image := revision.GetSource()
	if provider.Spec.Controller.Image != nil {
		image = *provider.Spec.Controller.Image
	}

	svcSelector := buildProviderServiceLabelSelector(provider, revision)

	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            revision.GetName(),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, revision.GetObjectKind().GroupVersionKind()))},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: svcSelector,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      provider.GetName(),
					Namespace: namespace,
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
						RunAsUser:    &runAsUser,
						RunAsGroup:   &runAsGroup,
					},
					ServiceAccountName: s.GetName(),
					ImagePullSecrets:   revision.GetPackagePullSecrets(),
					Containers: []corev1.Container{
						{
							Name:            provider.GetName(),
							Image:           image,
							ImagePullPolicy: pullPolicy,
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
							Env: []corev1.EnvVar{
								{
									// NOTE(turkenh): POD_NAMESPACE is needed to
									// set a default scope/namespace of the
									// default StoreConfig, similar to init
									// container of Core Crossplane.
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
	if revision.GetTLSServerSecretName() != nil {
		mountTLSSecret(*revision.GetTLSServerSecretName(), tlsServerCertsVolumeName, tlsServerCertsDir, tlsServerCertDirEnvVar, d)
		d.Spec.Template.Spec.Containers[0].Ports = append(d.Spec.Template.Spec.Containers[0].Ports,
			corev1.ContainerPort{
				Name:          webhookPortName,
				ContainerPort: servicePort,
			})
		// for backward compatibility with existing providers, we set the
		// environment variable WEBHOOK_TLS_CERT_DIR to the same value as
		// TLS_SERVER_CERTS_DIR to ease the transition to the new certificates.
		d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name:  webhookTLSCertDirEnvVar,
				Value: fmt.Sprintf("$(%s)", tlsServerCertDirEnvVar),
			})
	}

	if revision.GetTLSClientSecretName() != nil {
		mountTLSSecret(*revision.GetTLSClientSecretName(), tlsClientCertsVolumeName, tlsClientCertsDir, tlsClientCertDirEnvVar, d)
		// for backward compatibility with existing providers, we set the
		// environment variable ESS_TLS_CERTS_DIR to the same value as
		// TLS_CLIENT_CERTS_DIR to ease the transition to the new certificates.
		d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name:  essTLSCertDirEnvVar,
				Value: fmt.Sprintf("$(%s)", tlsClientCertDirEnvVar),
			})
	}

	templateLabels := make(map[string]string)
	if cc != nil {
		setControllerConfigConfigurations(s, cc, d, templateLabels)
	}
	for k, v := range d.Spec.Selector.MatchLabels { // ensure the template matches the selector
		templateLabels[k] = v
	}
	d.Spec.Template.Labels = templateLabels

	return s, d
}

func buildFunctionDeployment(function *pkgmetav1beta1.Function, revision v1.PackageWithRuntimeRevision, cc *v1alpha1.ControllerConfig, namespace string, pullSecrets []corev1.LocalObjectReference) (*corev1.ServiceAccount, *appsv1.Deployment) {
	s := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            revision.GetName(),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, revision.GetObjectKind().GroupVersionKind()))},
		},
		ImagePullSecrets: pullSecrets,
	}

	pullPolicy := corev1.PullIfNotPresent
	if revision.GetPackagePullPolicy() != nil {
		pullPolicy = *revision.GetPackagePullPolicy()
	}

	image := revision.GetSource()
	if function.Spec.Image != nil {
		image = *function.Spec.Image
	}

	svcSelector := buildFunctionServiceLabelSelector(function, revision)

	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            revision.GetName(),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, revision.GetObjectKind().GroupVersionKind()))},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: svcSelector,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      function.GetName(),
					Namespace: namespace,
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
						RunAsUser:    &runAsUser,
						RunAsGroup:   &runAsGroup,
					},
					ServiceAccountName: s.GetName(),
					ImagePullSecrets:   revision.GetPackagePullSecrets(),
					Containers: []corev1.Container{
						{
							Name:            function.GetName(),
							Image:           image,
							ImagePullPolicy: pullPolicy,
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
								{
									Name:          grpcPortName,
									ContainerPort: servicePort,
								},
							},
						},
					},
				},
			},
		},
	}

	if revision.GetTLSServerSecretName() != nil {
		mountTLSSecret(*revision.GetTLSServerSecretName(), tlsServerCertsVolumeName, tlsServerCertsDir, tlsServerCertDirEnvVar, d)
	}

	templateLabels := make(map[string]string)

	if cc != nil {
		setControllerConfigConfigurations(s, cc, d, templateLabels)
	}

	for k, v := range d.Spec.Selector.MatchLabels { // ensure the template matches the selector
		templateLabels[k] = v
	}
	d.Spec.Template.Labels = templateLabels

	return s, d
}

func buildFunctionSecret(revision v1.PackageWithRuntimeRevision, namespace string) (serverSec *corev1.Secret) {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            *revision.GetTLSServerSecretName(),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, revision.GetObjectKind().GroupVersionKind()))},
		},
	}
}

func buildFunctionService(function *pkgmetav1beta1.Function, revision v1.PackageRevision, namespace string) *corev1.Service {
	svc := getService(
		revision.GetLabels()[v1.LabelParentPackage],
		namespace,
		[]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, revision.GetObjectKind().GroupVersionKind()))},
		buildFunctionServiceLabelSelector(function, revision),
	)
	// We want a headless service so that our gRPC client (i.e. the Crossplane
	// FunctionComposer) can load balance across the endpoints.
	// https://kubernetes.io/docs/concepts/services-networking/service/#headless-services
	svc.Spec.ClusterIP = corev1.ClusterIPNone

	return svc
}

func buildFunctionServiceLabelSelector(function *pkgmetav1beta1.Function, revision v1.PackageRevision) map[string]string {
	return map[string]string{
		"pkg.crossplane.io/revision": revision.GetName(),
		"pkg.crossplane.io/function": function.GetName(),
	}
}

//nolint:gocyclo // Note: ControlerConfig is deprecated and following code will be removed in the future.
func setControllerConfigConfigurations(s *corev1.ServiceAccount, cc *v1alpha1.ControllerConfig, d *appsv1.Deployment, templateLabels map[string]string) {
	s.Labels = cc.Labels
	s.Annotations = cc.Annotations
	d.Labels = cc.Labels
	d.Annotations = cc.Annotations
	if cc.Spec.ServiceAccountName != nil {
		s.Name = *cc.Spec.ServiceAccountName
	}
	if cc.Spec.Metadata != nil {
		d.Spec.Template.Annotations = cc.Spec.Metadata.Annotations
	}

	if cc.Spec.Metadata != nil {
		for k, v := range cc.Spec.Metadata.Labels {
			templateLabels[k] = v
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

func getService(name, namespace string, owners []metav1.OwnerReference, matchLabels map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			OwnerReferences: owners,
		},
		Spec: corev1.ServiceSpec{
			Selector: matchLabels,
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       servicePort,
					TargetPort: intstr.FromInt(servicePort),
				},
			},
		},
	}
}
