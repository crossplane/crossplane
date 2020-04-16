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

package api

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// ComposableResourceOption modifies the composable resource.
type ComposableResourceOption func(resource *ComposableResource)

// FromReference sets the metadata of ComposableResource.
func FromReference(ref corev1.ObjectReference) ComposableResourceOption {
	return func(cr *ComposableResource) {
		cr.SetGroupVersionKind(ref.GroupVersionKind())
		cr.SetName(ref.Name)
		cr.SetNamespace(ref.Namespace)
		cr.SetUID(ref.UID)
	}
}

// NewComposableResource returns a new *ComposableResource.
func NewComposableResource(opts ...ComposableResourceOption) *ComposableResource {
	cr := &ComposableResource{}
	for _, f := range opts {
		f(cr)
	}
	return cr
}

// ComposableResource is used to operate on the composable resources whose schema
// is not known beforehand.
type ComposableResource struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (cr *ComposableResource) GetUnstructured() *unstructured.Unstructured {
	return &cr.Unstructured
}

// GetCondition of this ComposableResource.
func (cr *ComposableResource) GetCondition(ct v1alpha1.ConditionType) v1alpha1.Condition {
	conditioned := v1alpha1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := getObject(cr, "status", &conditioned); err != nil {
		return v1alpha1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this ComposableResource.
func (cr *ComposableResource) SetConditions(c ...v1alpha1.Condition) {
	conditioned := v1alpha1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = getObject(cr, "status", &conditioned)
	conditioned.SetConditions(c...)
	for i, ref := range conditioned.Conditions {
		_ = setObject(cr, fmt.Sprintf("status.conditions[%d]", i), ref)
	}
}

// GetWriteConnectionSecretToReference of this ComposableResource.
func (cr *ComposableResource) GetWriteConnectionSecretToReference() *v1alpha1.SecretReference {
	out := &v1alpha1.SecretReference{}
	if err := getObject(cr, "spec.writeConnectionSecretToRef", out); err != nil {
		return nil
	}
	return out
}

// SetWriteConnectionSecretToReference of this ComposableResource.
func (cr *ComposableResource) SetWriteConnectionSecretToReference(r *v1alpha1.SecretReference) {
	_ = setObject(cr, "spec.writeConnectionSecretToRef", r)
}
