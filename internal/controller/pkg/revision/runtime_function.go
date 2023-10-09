package revision

import (
	corev1 "k8s.io/api/core/v1"

	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

func functionDeploymentOverrides(functionMeta *pkgmetav1beta1.Function, _ v1.PackageWithRuntimeRevision) []DeploymentOverrides {
	do := []DeploymentOverrides{
		DeploymentRuntimeWithAdditionalPorts([]corev1.ContainerPort{
			{
				Name:          grpcPortName,
				ContainerPort: servicePort,
			},
		}),
	}

	if functionMeta.Spec.Image != nil {
		do = append(do, DeploymentRuntimeWithImage(*functionMeta.Spec.Image))
	}

	return do
}

func functionServiceOverrides() []ServiceOverrides {
	return []ServiceOverrides{
		// We want a headless service so that our gRPC client (i.e. the Crossplane
		// FunctionComposer) can load balance across the endpoints.
		// https://kubernetes.io/docs/concepts/services-networking/service/#headless-services
		ServiceWithClusterIP(corev1.ClusterIPNone),
	}
}
