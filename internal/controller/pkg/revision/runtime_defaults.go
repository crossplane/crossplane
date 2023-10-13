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
