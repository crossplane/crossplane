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
	"encoding/json"

	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// CompositeResourceOption is used to configure *CompositeResource
type CompositeResourceOption func(*CompositeResource)

// WithGroupVersionKind sets the GroupVersionKind.
func WithGroupVersionKind(gvk schema.GroupVersionKind) CompositeResourceOption {
	return func(c *CompositeResource) {
		c.SetGroupVersionKind(gvk)
	}
}

// NewCompositeResource returns a new *CompositeResource configured via opts.
func NewCompositeResource(opts ...CompositeResourceOption) *CompositeResource {
	c := &CompositeResource{}
	for _, f := range opts {
		f(c)
	}
	return c
}

// An CompositeResource is the internal representation of the resource generated
// via Crossplane definition types. It is only used for operations in the controller,
// it's not intended to be stored in the api-server.
type CompositeResource struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (c *CompositeResource) GetUnstructured() *unstructured.Unstructured {
	return &c.Unstructured
}

// GetCompositionSelector returns the composition selector.
func (c *CompositeResource) GetCompositionSelector() *v1.LabelSelector {
	out := &v1.LabelSelector{}
	if err := getObject(c, "spec.compositionSelector", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionSelector sets the composition selector.
func (c *CompositeResource) SetCompositionSelector(sel *v1.LabelSelector) {
	_ = setObject(c, "spec.compositionSelector", sel)
}

// GetCompositionReference returns the composition reference.
func (c *CompositeResource) GetCompositionReference() *corev1.ObjectReference {
	out := &corev1.ObjectReference{}
	if err := getObject(c, "spec.compositionRef", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionReference sets the composition reference.
func (c *CompositeResource) SetCompositionReference(ref *corev1.ObjectReference) {
	_ = setObject(c, "spec.compositionRef", ref)
}

// GetResourceReferences returns the references of composed resources.
func (c *CompositeResource) GetResourceReferences() []corev1.ObjectReference {
	out := &[]corev1.ObjectReference{}
	_ = getObject(c, "spec.resourceRefs", out)
	return *out
}

// SetResourceReferences sets the references of composed resources.
func (c *CompositeResource) SetResourceReferences(refs []corev1.ObjectReference) {
	empty := corev1.ObjectReference{}
	finalRefs := []corev1.ObjectReference{}
	for _, ref := range refs {
		// TODO(muvaf): temporary workaround.
		if ref.String() == empty.String() {
			continue
		}
		finalRefs = append(finalRefs, ref)
	}
	_ = setArray(c, "spec.resourceRefs", finalRefs)
}

// GetWriteConnectionSecretToReference returns the connection secret reference.
func (c *CompositeResource) GetWriteConnectionSecretToReference() *v1alpha1.SecretReference {
	out := &v1alpha1.SecretReference{}
	if err := getObject(c, "spec.writeConnectionSecretToRef", out); err != nil {
		return nil
	}
	return out
}

// SetWriteConnectionSecretToReference sets the connection secret reference.
func (c *CompositeResource) SetWriteConnectionSecretToReference(ref *v1alpha1.SecretReference) {
	_ = setObject(c, "spec.writeConnectionSecretToRef", ref)
}

// GetCondition of this CompositeResource.
func (c *CompositeResource) GetCondition(ct v1alpha1.ConditionType) v1alpha1.Condition {
	conditioned := v1alpha1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := getObject(c, "status", &conditioned); err != nil {
		return v1alpha1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this CompositeResource.
func (c *CompositeResource) SetConditions(conditions ...v1alpha1.Condition) {
	conditioned := v1alpha1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = getObject(c, "status", &conditioned)
	conditioned.SetConditions(conditions...)
	_ = setArray(c, "status.conditions", conditioned.Conditions)
}

// SetBindingPhase of this CompositeResource.
func (c *CompositeResource) SetBindingPhase(p v1alpha1.BindingPhase) {
	_ = setObject(c, "status.bindingPhase", p)
}

// GetBindingPhase of this CompositeResource.
func (c *CompositeResource) GetBindingPhase() v1alpha1.BindingPhase {
	bp := ""
	_ = getObject(c, "status.bindingPhase", &bp)
	return v1alpha1.BindingPhase(bp)
}

// CompositeResourceList contains a list of CompositeResources.
type CompositeResourceList struct {
	unstructured.UnstructuredList
}

// GetUnstructuredList returns the underlying *unstructured.UnstructuredList.
func (c *CompositeResourceList) GetUnstructuredList() *unstructured.UnstructuredList {
	return &c.UnstructuredList
}

func getObject(in interface{ UnstructuredContent() map[string]interface{} }, path string, target interface{}) error {
	p := fieldpath.Pave(in.UnstructuredContent())
	obj, err := p.GetValue(path)
	if err != nil {
		return err
	}
	if obj == nil {
		return nil
	}
	js, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return json.Unmarshal(js, target)
}

func setObject(in interface{ UnstructuredContent() map[string]interface{} }, path string, input interface{}) error {
	p := fieldpath.Pave(in.UnstructuredContent())
	if input == nil {
		return p.SetValue(path, nil)
	}
	js, err := json.Marshal(input)
	if err != nil {
		return err
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(js, &out); err != nil {
		return err
	}
	return p.SetValue(path, out)
}

func setArray(in interface{ UnstructuredContent() map[string]interface{} }, path string, input interface{}) error {
	p := fieldpath.Pave(in.UnstructuredContent())
	if input == nil {
		return p.SetValue(path, nil)
	}
	js, err := json.Marshal(input)
	if err != nil {
		return err
	}
	out := []interface{}{}
	if err := json.Unmarshal(js, &out); err != nil {
		return err
	}
	return p.SetValue(path, out)
}
