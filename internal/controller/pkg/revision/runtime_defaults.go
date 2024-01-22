// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package revision

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

func serviceAccountFromRuntimeConfig(tmpl *v1beta1.ServiceAccountTemplate) *corev1.ServiceAccount {
	sa := &corev1.ServiceAccount{}

	if tmpl == nil || tmpl.Metadata == nil {
		return sa
	}

	if tmpl.Metadata.Name != nil {
		sa.Name = *tmpl.Metadata.Name
	}

	sa.Annotations = tmpl.Metadata.Annotations
	sa.Labels = tmpl.Metadata.Labels

	return sa
}

func deploymentFromRuntimeConfig(tmpl *v1beta1.DeploymentTemplate) *appsv1.Deployment {
	d := &appsv1.Deployment{}

	if tmpl == nil {
		return d
	}

	if meta := tmpl.Metadata; meta != nil {
		if meta.Name != nil {
			d.Name = *meta.Name
		}
		d.Annotations = meta.Annotations
		d.Labels = meta.Labels
	}

	if spec := tmpl.Spec; spec != nil {
		d.Spec = *spec
	}

	return d
}

func serviceFromRuntimeConfig(tmpl *v1beta1.ServiceTemplate) *corev1.Service {
	svc := &corev1.Service{}

	if tmpl == nil || tmpl.Metadata == nil {
		return svc
	}

	if tmpl.Metadata.Name != nil {
		svc.Name = *tmpl.Metadata.Name
	}

	svc.Annotations = tmpl.Metadata.Annotations
	svc.Labels = tmpl.Metadata.Labels

	return svc
}
