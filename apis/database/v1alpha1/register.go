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

package v1alpha1

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/runtime/scheme"
)

// Package type metadata.
const (
	Group   = "database.crossplane.io"
	Version = "v1alpha1"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)

// MySQLInstance type metadata.
var (
	MySQLInstanceKind             = reflect.TypeOf(MySQLInstance{}).Name()
	MySQLInstanceKindAPIVersion   = MySQLInstanceKind + "." + SchemeGroupVersion.String()
	MySQLInstanceGroupVersionKind = SchemeGroupVersion.WithKind(MySQLInstanceKind)
)

// MySQLInstanceClass type metadata.
var (
	MySQLInstanceClassKind             = reflect.TypeOf(MySQLInstanceClass{}).Name()
	MySQLInstanceClassKindAPIVersion   = MySQLInstanceClassKind + "." + SchemeGroupVersion.String()
	MySQLInstanceClassGroupVersionKind = SchemeGroupVersion.WithKind(MySQLInstanceClassKind)
)

// MySQLInstanceClassList type metadata.
var (
	MySQLInstanceClassListKind             = reflect.TypeOf(MySQLInstanceClassList{}).Name()
	MySQLInstanceClassListKindAPIVersion   = MySQLInstanceClassListKind + "." + SchemeGroupVersion.String()
	MySQLInstanceClassListGroupVersionKind = SchemeGroupVersion.WithKind(MySQLInstanceClassListKind)
)

// PostgreSQLInstance type metadata.
var (
	PostgreSQLInstanceKind             = reflect.TypeOf(PostgreSQLInstance{}).Name()
	PostgreSQLInstanceKindAPIVersion   = PostgreSQLInstanceKind + "." + SchemeGroupVersion.String()
	PostgreSQLInstanceGroupVersionKind = SchemeGroupVersion.WithKind(PostgreSQLInstanceKind)
)

// PostgreSQLInstanceClass type metadata.
var (
	PostgreSQLInstanceClassKind             = reflect.TypeOf(PostgreSQLInstanceClass{}).Name()
	PostgreSQLInstanceClassKindAPIVersion   = PostgreSQLInstanceClassKind + "." + SchemeGroupVersion.String()
	PostgreSQLInstanceClassGroupVersionKind = SchemeGroupVersion.WithKind(PostgreSQLInstanceClassKind)
)

// PostgreSQLInstanceClassList type metadata.
var (
	PostgreSQLInstanceClassListKind             = reflect.TypeOf(PostgreSQLInstanceClassList{}).Name()
	PostgreSQLInstanceClassListKindAPIVersion   = PostgreSQLInstanceClassListKind + "." + SchemeGroupVersion.String()
	PostgreSQLInstanceClassListGroupVersionKind = SchemeGroupVersion.WithKind(PostgreSQLInstanceClassListKind)
)

func init() {
	SchemeBuilder.Register(&MySQLInstance{}, &MySQLInstanceList{})
	SchemeBuilder.Register(&MySQLInstanceClass{}, &MySQLInstanceClassList{})
	SchemeBuilder.Register(&PostgreSQLInstance{}, &PostgreSQLInstanceList{})
	SchemeBuilder.Register(&PostgreSQLInstanceClass{}, &PostgreSQLInstanceClassList{})
}
