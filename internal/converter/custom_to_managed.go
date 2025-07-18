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

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"

	"github.com/crossplane/crossplane/apis/apiextensions/v2alpha1"
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
func CustomToManagedResourceDefinitions(objects ...runtime.Object) []runtime.Object {
	extv1.Kind(customResourceDefinition)
	for i, obj := range objects {
		if obj.GetObjectKind().GroupVersionKind() == customResourceDefinitionKind {
			// Don't mutate the original object just in-case it is used elsewhere.
			obj := obj.DeepCopyObject()
			// The object has to be either an unstructured.Unstructured object or a CustomResourceDefinition
			switch o := obj.(type) {
			// to covert, all we need to worry about is the metadata. The spec.state value will be defaulted.
			case *unstructured.Unstructured:
				o.Object = convertCRDToMRD(o.Object)
				objects[i] = o
			default:
				b, err := json.Marshal(o)
				if err != nil {
					panic(err)
				}
				u := &unstructured.Unstructured{}
				if err := json.Unmarshal(b, u); err != nil {
					panic(err)
				}
				u.Object = convertCRDToMRD(u.Object)
				objects[i] = u
			}
		}
	}
	return objects
}

func convertCRDToMRD(in map[string]any) map[string]any {
	paved := fieldpath.Pave(in)
	if err := paved.SetValue("apiVersion", v2alpha1.SchemeGroupVersion.String()); err != nil {
		panic(err)
	}
	if err := paved.SetValue("kind", v2alpha1.ManagedResourceDefinitionKind); err != nil {
		panic(err)
	}
	// We don't have to set spec.state directly. We will use the default or existing resource to get this value.
	return paved.UnstructuredContent()
}
