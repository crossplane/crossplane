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
// +kubebuilder:skip

package instance

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// An InfraInstance is the internal representation of the resource generated
// via InfrastructureDefinition. It is only used for operations in the controller,
// it's not intended to be stored in the api-server.
type InfraInstance struct {
	unstructured.Unstructured
}

// InfraInstanceList contains a list of InfraInstances.
type InfraInstanceList struct {
	unstructured.UnstructuredList
}

func (in *InfraInstanceList) GetUnstructuredList() *unstructured.UnstructuredList {
	return &in.UnstructuredList
}

func (in *InfraInstance) GetUnstructured() *unstructured.Unstructured {
	return &in.Unstructured
}

func (in *InfraInstance) GetCompositionSelector() *v1.LabelSelector {
	out := &v1.LabelSelector{}
	if err := in.getObject("spec.compositionSelector", out); err != nil {
		return nil
	}
	return out
}

func (in *InfraInstance) SetCompositionSelector(sel *v1.LabelSelector) {
	_ = in.setObject("spec.compositionSelector", sel)
}

func (in *InfraInstance) GetCompositionReference() *corev1.ObjectReference {
	out := &corev1.ObjectReference{}
	if err := in.getObject("spec.compositionRef", out); err != nil {
		return nil
	}
	return out
}

func (in *InfraInstance) SetCompositionReference(ref *corev1.ObjectReference) {
	_ = in.setObject("spec.compositionRef", ref)
}

func (in *InfraInstance) GetResourceReferences() []corev1.ObjectReference {
	path := "spec.resourceRefs"
	length, _ := in.getLen(path)
	out := make([]corev1.ObjectReference, length)
	for i := 0; i < length; i++ {
		_ = in.getObject(fmt.Sprintf("%s[%d]", path, i), &out[i])
	}
	return out
}

func (in *InfraInstance) SetResourceReferences(refs []corev1.ObjectReference) {
	for i, ref := range refs {
		_ = in.setObject(fmt.Sprintf("spec.resourceRefs[%d]", i), ref)
	}
}

func (in *InfraInstance) GetWriteConnectionSecretToReference() *v1alpha1.SecretReference {
	out := &v1alpha1.SecretReference{}
	if err := in.getObject("spec.writeConnectionSecretToRef", out); err != nil {
		return nil
	}
	return out
}

func (in *InfraInstance) SetWriteConnectionSecretToReference(ref *v1alpha1.SecretReference) {
	_ = in.setObject("spec.writeConnectionSecretToRef", ref)
}

// GetCondition of this InfraInstance.
func (in *InfraInstance) GetCondition(ct v1alpha1.ConditionType) v1alpha1.Condition {
	conditioned := v1alpha1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := in.getObject("status", &conditioned); err != nil {
		return v1alpha1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this InfraInstance.
func (in *InfraInstance) SetConditions(c ...v1alpha1.Condition) {
	conditioned := v1alpha1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = in.getObject("status", &conditioned)
	conditioned.SetConditions(c...)
	for i, ref := range conditioned.Conditions {
		_ = in.setObject(fmt.Sprintf("status.conditions[%d]", i), ref)
	}
}

func (in *InfraInstance) getObject(path string, target interface{}) error {
	p := fieldpath.Pave(in.UnstructuredContent())
	obj, err := p.GetValue(path)
	if err != nil {
		return err
	}
	if obj == nil {
		target = nil
		return nil
	}
	js, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return json.Unmarshal(js, target)
}

func (in *InfraInstance) getLen(path string) (int, error) {
	p := fieldpath.Pave(in.UnstructuredContent())
	v, err := p.GetValue(path)
	if err != nil {
		return 0, err
	}
	a, ok := v.([]interface{})
	if !ok {
		return 0, errors.Errorf("%s: not an array", path)
	}
	return len(a), nil
}

func (in *InfraInstance) setObject(path string, input interface{}) error {
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
