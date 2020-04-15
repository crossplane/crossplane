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

package instance

import (
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type CompositionInstance interface {
	v1.Object
	runtime.Unstructured

	resource.Conditioned
	resource.ConnectionSecretWriterTo

	GetUnstructured() *unstructured.Unstructured

	SetCompositionSelector(*v1.LabelSelector)
	GetCompositionSelector() *v1.LabelSelector

	SetCompositionReference(*corev1.ObjectReference)
	GetCompositionReference() *corev1.ObjectReference

	SetResourceReferences([]corev1.ObjectReference)
	GetResourceReferences() []corev1.ObjectReference
}

type CompositionInstanceList interface {
	GetUnstructuredList() *unstructured.UnstructuredList
}
