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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/initializer"
)

var (
	replicas                 = int32(1)
	runAsUser                = int64(2000)
	runAsGroup               = int64(2000)
	allowPrivilegeEscalation = false
	privileged               = false
	runAsNonRoot             = true
)

// Providers are expected to use port 8080 if they expose Prometheus metrics,
// which any provider built using controller-runtime will do by default.
const (
	promPortName   = "metrics"
	promPortNumber = 8080

	webhookVolumeName       = "webhook-tls-secret"
	webhookTLSCertDirEnvVar = "WEBHOOK_TLS_CERT_DIR"
	webhookTLSCertDir       = "/webhook/tls"
	webhookPortName         = "webhook"

	grpcPortName       = "grpc"
	servicePort        = 9443
	serviceEndpointFmt = "https://%s.%s:%d"

	essTLSCertDirEnvVar = "ESS_TLS_CERTS_DIR"
	essCertsVolumeName  = "ess-client-certs"
	essCertsDir         = "/ess/tls"

	tlsServerCertDirEnvVar   = "TLS_SERVER_CERTS_DIR"
	tlsServerCertsVolumeName = "tls-server-certs"
	tlsServerCertsDir        = "/tls/server"

	tlsClientCertDirEnvVar   = "TLS_CLIENT_CERTS_DIR"
	tlsClientCertsVolumeName = "tls-client-certs"
	tlsClientCertsDir        = "/tls/client"
)

// Returns the service account, deployment, service, server and client TLS secrets of the provider.
func buildProviderDeployment(provider *pkgmetav1.Provider, revision v1.PackageRevision, cc *v1alpha1.ControllerConfig, namespace string, pullSecrets []corev1.LocalObjectReference) (*corev1.ServiceAccount, *appsv1.Deployment, *corev1.Service, *corev1.Secret, *corev1.Secret) {
	s := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            revision.GetName(),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, v1.ProviderRevisionGroupVersionKind))},
		},
		ImagePullSecrets: pullSecrets,
	}
	secSer := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            *revision.GetTLSServerSecretName(),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, v1.ProviderRevisionGroupVersionKind))},
		},
	}

	secCli := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            *revision.GetTLSClientSecretName(),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, v1.ProviderRevisionGroupVersionKind))},
		},
	}
	pullPolicy := corev1.PullIfNotPresent
	if revision.GetPackagePullPolicy() != nil {
		pullPolicy = *revision.GetPackagePullPolicy()
	}
	image := revision.GetSource()
	if provider.Spec.Controller.Image != nil {
		image = *provider.Spec.Controller.Image
	}
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            revision.GetName(),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, v1.ProviderRevisionGroupVersionKind))},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"pkg.crossplane.io/revision": revision.GetName(),
					"pkg.crossplane.io/provider": provider.GetName(),
				},
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
									Name:          promPortName,
									ContainerPort: promPortNumber,
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
	}

	if revision.GetTLSClientSecretName() != nil {
		mountTLSSecret(*revision.GetTLSClientSecretName(), tlsClientCertsVolumeName, tlsClientCertsDir, tlsClientCertDirEnvVar, d)
	}

	if revision.GetWebhookTLSSecretName() != nil {
		v := corev1.Volume{
			Name: webhookVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: *revision.GetWebhookTLSSecretName(),
					Items: []corev1.KeyToPath{
						// These are known and validated keys in TLS secrets.
						{Key: "tls.crt", Path: "tls.crt"},
						{Key: "tls.key", Path: "tls.key"},
					},
				},
			},
		}
		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, v)

		vm := corev1.VolumeMount{
			Name:      webhookVolumeName,
			ReadOnly:  true,
			MountPath: webhookTLSCertDir,
		}
		d.Spec.Template.Spec.Containers[0].VolumeMounts =
			append(d.Spec.Template.Spec.Containers[0].VolumeMounts, vm)

		envs := []corev1.EnvVar{
			{Name: webhookTLSCertDirEnvVar, Value: webhookTLSCertDir},
		}
		d.Spec.Template.Spec.Containers[0].Env =
			append(d.Spec.Template.Spec.Containers[0].Env, envs...)

		port := corev1.ContainerPort{
			Name:          webhookPortName,
			ContainerPort: servicePort,
		}
		d.Spec.Template.Spec.Containers[0].Ports = append(d.Spec.Template.Spec.Containers[0].Ports,
			port)
	}

	if revision.GetESSTLSSecretName() != nil {
		mountTLSSecret(*revision.GetESSTLSSecretName(), essCertsVolumeName, essCertsDir, essTLSCertDirEnvVar, d)
	}

	templateLabels := make(map[string]string)
	if cc != nil {
		setControllerConfigConfigurations(s, cc, d, templateLabels)
	}
	for k, v := range d.Spec.Selector.MatchLabels { // ensure the template matches the selector
		templateLabels[k] = v
	}
	d.Spec.Template.Labels = templateLabels

	svc := getService(revision.GetName(), namespace, []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, v1.ProviderRevisionGroupVersionKind))}, d.Spec.Selector.MatchLabels)

	return s, d, svc, secSer, secCli
}

func buildFunctionDeployment(function *pkgmetav1beta1.Function, revision v1.PackageRevision, cc *v1alpha1.ControllerConfig, namespace string, pullSecrets []corev1.LocalObjectReference) (*corev1.ServiceAccount, *appsv1.Deployment, *corev1.Service, *corev1.Secret) {
	s := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            revision.GetName(),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, v1beta1.FunctionRevisionGroupVersionKind))},
		},
		ImagePullSecrets: pullSecrets,
	}
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            *revision.GetTLSServerSecretName(),
			Namespace:       namespace,
			OwnerReferences: revision.GetOwnerReferences(),
		},
	}

	pullPolicy := corev1.PullIfNotPresent
	if revision.GetPackagePullPolicy() != nil {
		pullPolicy = *revision.GetPackagePullPolicy()
	}

	image := revision.GetSource()
	if function.Spec.Image != nil {
		image = *function.Spec.Image
	}

	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      revision.GetName(),
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				meta.AsController(meta.TypedReferenceTo(revision, v1beta1.FunctionRevisionGroupVersionKind)),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"pkg.crossplane.io/revision": revision.GetName(),
					"pkg.crossplane.io/function": function.GetName(),
				},
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
									Name:          promPortName,
									ContainerPort: promPortNumber,
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

	pkgName := revision.GetLabels()[v1.LabelParentPackage]
	svc := getService(pkgName, namespace, revision.GetOwnerReferences(), d.Spec.Selector.MatchLabels)
	return s, d, svc, sec
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
	d.Spec.Template.Spec.Containers[0].VolumeMounts =
		append(d.Spec.Template.Spec.Containers[0].VolumeMounts, vm)

	envs := []corev1.EnvVar{
		{Name: envName, Value: mountPath},
	}
	d.Spec.Template.Spec.Containers[0].Env =
		append(d.Spec.Template.Spec.Containers[0].Env, envs...)
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
