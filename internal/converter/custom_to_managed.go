/*
Copyright 2025 The Crossplane Authors.

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

// Package converter holds resource converter helpers.
package converter

import (
	"encoding/json"
	"reflect"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

// CustomResourceDefinition type metadata.
//
//nolint:gochecknoglobals // These should be globals.
var (
	customResourceDefinition     = reflect.TypeOf(extv1.CustomResourceDefinition{}).Name()
	customResourceDefinitionKind = extv1.SchemeGroupVersion.WithKind(customResourceDefinition)
)

// CustomToManagedResourceDefinitions in place converts and returns any
// CustomResourceDefinition runtime object into a ManagedResourceDefinition.
func CustomToManagedResourceDefinitions(defaultActive bool, objects ...runtime.Object) ([]runtime.Object, error) {
	var errs []error

	extv1.Kind(customResourceDefinition)
	for i, obj := range objects {
		if obj.GetObjectKind().GroupVersionKind() == customResourceDefinitionKind {
			// Don't mutate the original object just in-case it is used elsewhere.
			obj := obj.DeepCopyObject()
			// The object has to be either an unstructured.Unstructured object or a CustomResourceDefinition
			switch o := obj.(type) {
			// to covert, all we need to worry about is the metadata and spec.state.
			case *unstructured.Unstructured:
				mrdObject, err := convertCRDToMRD(defaultActive, o.Object)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				objects[i] = mrdObject
			default:
				b, err := json.Marshal(o)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				u := &unstructured.Unstructured{}
				if err := json.Unmarshal(b, u); err != nil {
					errs = append(errs, err)
					continue
				}
				mrdObject, err := convertCRDToMRD(defaultActive, u.Object)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				objects[i] = mrdObject
			}
		}
	}
	return objects, errors.Join(errs...)
}

func convertCRDToMRD(defaultActive bool, in map[string]any) (*v1alpha1.ManagedResourceDefinition, error) {
	in["apiVersion"] = v1alpha1.SchemeGroupVersion.String()
	in["kind"] = v1alpha1.ManagedResourceDefinitionKind

	var mrd v1alpha1.ManagedResourceDefinition

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(in, &mrd); err != nil {
		return nil, errors.Wrap(err, "failed converting CRD to MRD")
	}
	if defaultActive {
		mrd.Spec.State = v1alpha1.ManagedResourceDefinitionActive
	}
	return &mrd, nil
}
