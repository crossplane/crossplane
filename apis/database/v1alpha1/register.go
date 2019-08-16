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
// +kubebuilder:object:generate=true
// +groupName=database.crossplane.io
// +versionName=v1alpha1

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

// MySQLInstancePolicy type metadata.
var (
	MySQLInstancePolicyKind             = reflect.TypeOf(MySQLInstancePolicy{}).Name()
	MySQLInstancePolicyKindAPIVersion   = MySQLInstancePolicyKind + "." + SchemeGroupVersion.String()
	MySQLInstancePolicyGroupVersionKind = SchemeGroupVersion.WithKind(MySQLInstancePolicyKind)
)

// MySQLInstancePolicyList type metadata.
var (
	MySQLInstancePolicyListKind             = reflect.TypeOf(MySQLInstancePolicyList{}).Name()
	MySQLInstancePolicyListKindAPIVersion   = MySQLInstancePolicyListKind + "." + SchemeGroupVersion.String()
	MySQLInstancePolicyListGroupVersionKind = SchemeGroupVersion.WithKind(MySQLInstancePolicyListKind)
)

// PostgreSQLInstance type metadata.
var (
	PostgreSQLInstanceKind             = reflect.TypeOf(PostgreSQLInstance{}).Name()
	PostgreSQLInstanceKindAPIVersion   = PostgreSQLInstanceKind + "." + SchemeGroupVersion.String()
	PostgreSQLInstanceGroupVersionKind = SchemeGroupVersion.WithKind(PostgreSQLInstanceKind)
)

// PostgreSQLInstancePolicy type metadata.
var (
	PostgreSQLInstancePolicyKind             = reflect.TypeOf(PostgreSQLInstancePolicy{}).Name()
	PostgreSQLInstancePolicyKindAPIVersion   = PostgreSQLInstancePolicyKind + "." + SchemeGroupVersion.String()
	PostgreSQLInstancePolicyGroupVersionKind = SchemeGroupVersion.WithKind(PostgreSQLInstancePolicyKind)
)

// PostgreSQLInstancePolicyList type metadata.
var (
	PostgreSQLInstancePolicyListKind             = reflect.TypeOf(PostgreSQLInstancePolicyList{}).Name()
	PostgreSQLInstancePolicyListKindAPIVersion   = PostgreSQLInstancePolicyListKind + "." + SchemeGroupVersion.String()
	PostgreSQLInstancePolicyListGroupVersionKind = SchemeGroupVersion.WithKind(PostgreSQLInstancePolicyListKind)
)

func init() {
	SchemeBuilder.Register(&MySQLInstance{}, &MySQLInstanceList{})
	SchemeBuilder.Register(&MySQLInstancePolicy{}, &MySQLInstancePolicyList{})
	SchemeBuilder.Register(&PostgreSQLInstance{}, &PostgreSQLInstanceList{})
	SchemeBuilder.Register(&PostgreSQLInstancePolicy{}, &PostgreSQLInstancePolicyList{})
}
