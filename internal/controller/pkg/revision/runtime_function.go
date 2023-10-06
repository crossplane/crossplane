package revision

import (
	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	corev1 "k8s.io/api/core/v1"
)

func functionDeploymentOverrides(functionMeta *pkgmetav1beta1.Function, pr v1.PackageWithRuntimeRevision) []DeploymentOverrides {
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
