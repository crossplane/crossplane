/*
Copyright 2018 The Crossplane Authors.

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

package util

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// NamespaceNameFromObjectRef helper function to create NamespacedName
func NamespaceNameFromObjectRef(or *corev1.ObjectReference) types.NamespacedName {
	return types.NamespacedName{
		Namespace: or.Namespace,
		Name:      or.Name,
	}
}

// AddOwnerReference to the object metadata, only if this owner reference
// is not in the existing owner references list
func AddOwnerReference(om *metav1.ObjectMeta, or metav1.OwnerReference) {
	if om == nil {
		return
	}
	if om.OwnerReferences == nil {
		om.OwnerReferences = []metav1.OwnerReference{or}
		return
	}
	for _, v := range om.OwnerReferences {
		if reflect.DeepEqual(v, or) {
			return
		}
	}
	om.OwnerReferences = append(om.OwnerReferences, or)
}
