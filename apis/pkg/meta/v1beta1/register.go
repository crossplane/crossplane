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

package v1beta1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Package type metadata.
const (
	Group   = "meta.pkg.crossplane.io"
	Version = "v1beta1"
)

var (
	// SchemeGroupVersion is group version used to register these objects.
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds all registered types to scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// Function type metadata.
var (
	FunctionKind             = reflect.TypeFor[Function]().Name()
	FunctionGroupKind        = schema.GroupKind{Group: Group, Kind: FunctionKind}.String()
	FunctionKindAPIVersion   = FunctionKind + "." + SchemeGroupVersion.String()
	FunctionGroupVersionKind = SchemeGroupVersion.WithKind(FunctionKind)
)

// addKnownTypes adds this package's types to the supplied scheme.
func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(SchemeGroupVersion,
		&Function{},
	)
	metav1.AddToGroupVersion(s, SchemeGroupVersion)

	return nil
}
