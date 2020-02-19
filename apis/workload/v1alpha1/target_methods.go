/*
Copyright 2019 The Crossplane Authors.

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

// TODO(hasheddan): generate these methods with angryjet
// Ref: https://github.com/crossplane/crossplane-tools/issues/14

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// GetCondition of this KubernetesTarget.
func (tr *KubernetesTarget) GetCondition(ct runtimev1alpha1.ConditionType) runtimev1alpha1.Condition {
	return tr.Status.GetCondition(ct)
}

// GetResourceReference of this KubernetesTarget.
func (tr *KubernetesTarget) GetResourceReference() *corev1.ObjectReference {
	return tr.Spec.ResourceReference
}

// GetWriteConnectionSecretToReference of this KubernetesTarget.
func (tr *KubernetesTarget) GetWriteConnectionSecretToReference() *runtimev1alpha1.LocalSecretReference {
	return tr.Spec.WriteConnectionSecretToReference
}

// SetConditions of this KubernetesTarget.
func (tr *KubernetesTarget) SetConditions(c ...runtimev1alpha1.Condition) {
	tr.Status.SetConditions(c...)
}

// SetResourceReference of this KubernetesTarget.
func (tr *KubernetesTarget) SetResourceReference(r *corev1.ObjectReference) {
	tr.Spec.ResourceReference = r
}

// SetWriteConnectionSecretToReference of this KubernetesTarget.
func (tr *KubernetesTarget) SetWriteConnectionSecretToReference(r *runtimev1alpha1.LocalSecretReference) {
	tr.Spec.WriteConnectionSecretToReference = r
}
