package revision

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

func providerDeploymentOverrides(providerMeta *pkgmetav1.Provider, pr v1.PackageWithRuntimeRevision) []DeploymentOverrides {
	do := []DeploymentOverrides{
		DeploymentRuntimeWithAdditionalEnvironments([]corev1.EnvVar{
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
		}),
	}

	if providerMeta.Spec.Controller.Image != nil {
		do = append(do, DeploymentRuntimeWithImage(*providerMeta.Spec.Controller.Image))
	}

	if pr.GetTLSServerSecretName() != nil {
		do = append(do, DeploymentRuntimeWithAdditionalPorts([]corev1.ContainerPort{
			{
				Name:          webhookPortName,
				ContainerPort: servicePort,
			},
		}), DeploymentRuntimeWithAdditionalEnvironments([]corev1.EnvVar{
			// for backward compatibility with existing providers, we set the
			// environment variable WEBHOOK_TLS_CERT_DIR to the same value as
			// TLS_SERVER_CERTS_DIR to ease the transition to the new certificates.
			{
				Name:  webhookTLSCertDirEnvVar,
				Value: fmt.Sprintf("$(%s)", tlsServerCertDirEnvVar),
			},
		}))
	}

	if pr.GetTLSClientSecretName() != nil {
		do = append(do, DeploymentRuntimeWithAdditionalEnvironments([]corev1.EnvVar{
			// for backward compatibility with existing providers, we set the
			// environment variable ESS_TLS_CERTS_DIR to the same value as
			// TLS_CLIENT_CERTS_DIR to ease the transition to the new certificates.
			{
				Name:  essTLSCertDirEnvVar,
				Value: fmt.Sprintf("$(%s)", tlsClientCertDirEnvVar),
			},
		}))
	}

	return do
}
