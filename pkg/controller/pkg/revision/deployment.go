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

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	metav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

func buildProviderDeployment(provider *metav1alpha1.Provider, revision v1alpha1.PackageRevision, namespace string) (*corev1.ServiceAccount, *appsv1.Deployment) { // nolint:interfacer
	s := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            revision.GetName(),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, v1alpha1.ProviderRevisionGroupVersionKind))},
		},
	}
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            revision.GetName(),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, v1alpha1.ProviderRevisionGroupVersionKind))},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"pkg.crossplane.io/revision": revision.GetName()},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      provider.GetName(),
					Namespace: namespace,
					Labels:    map[string]string{"pkg.crossplane.io/revision": revision.GetName()},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: s.GetName(),
					Containers: []corev1.Container{
						{
							Name:  provider.GetName(),
							Image: provider.Spec.Controller.Image,
						},
					},
				},
			},
		},
	}
	return s, d
}
