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

package v2

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Package type metadata.
const (
	Group   = "apiextensions.crossplane.io"
	Version = "v2"
)

var (
	// SchemeGroupVersion is group version used to register these objects.
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds all registered types to scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// CompositeResourceDefinition type metadata.
var (
	CompositeResourceDefinitionKind             = reflect.TypeFor[CompositeResourceDefinition]().Name()
	CompositeResourceDefinitionGroupKind        = schema.GroupKind{Group: Group, Kind: CompositeResourceDefinitionKind}.String()
	CompositeResourceDefinitionKindAPIVersion   = CompositeResourceDefinitionKind + "." + SchemeGroupVersion.String()
	CompositeResourceDefinitionGroupVersionKind = SchemeGroupVersion.WithKind(CompositeResourceDefinitionKind)
)

// addKnownTypes adds this package's types to the supplied scheme.
func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(SchemeGroupVersion,
		&CompositeResourceDefinition{}, &CompositeResourceDefinitionList{},
	)
	metav1.AddToGroupVersion(s, SchemeGroupVersion)

	return nil
}
