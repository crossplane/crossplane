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
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// Package type metadata.
const (
	Group   = "packages.crossplane.io"
	Version = "v1alpha1"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)

// PackageInstall type metadata.
var (
	PackageInstallKind             = reflect.TypeOf(PackageInstall{}).Name()
	PackageInstallGroupKind        = schema.GroupKind{Group: Group, Kind: PackageInstallKind}.String()
	PackageInstallKindAPIVersion   = PackageInstallKind + "." + SchemeGroupVersion.String()
	PackageInstallGroupVersionKind = SchemeGroupVersion.WithKind(PackageInstallKind)
)

// ClusterPackageInstall type metadata.
var (
	ClusterPackageInstallKind             = reflect.TypeOf(ClusterPackageInstall{}).Name()
	ClusterPackageInstallGroupKind        = schema.GroupKind{Group: Group, Kind: ClusterPackageInstallKind}.String()
	ClusterPackageInstallKindAPIVersion   = ClusterPackageInstallKind + "." + SchemeGroupVersion.String()
	ClusterPackageInstallGroupVersionKind = SchemeGroupVersion.WithKind(ClusterPackageInstallKind)
)

// Package type metadata.
var (
	PackageKind             = reflect.TypeOf(Package{}).Name()
	PackageGroupKind        = schema.GroupKind{Group: Group, Kind: PackageKind}.String()
	PackageKindAPIVersion   = PackageKind + "." + SchemeGroupVersion.String()
	PackageGroupVersionKind = SchemeGroupVersion.WithKind(PackageKind)
)

// StackDefinition type metadata
var (
	StackDefinitionKind             = reflect.TypeOf(StackDefinition{}).Name()
	StackDefinitionGroupKind        = schema.GroupKind{Group: Group, Kind: StackDefinitionKind}.String()
	StackDefinitionKindAPIVersion   = StackDefinitionKind + "." + SchemeGroupVersion.String()
	StackDefinitionGroupVersionKind = SchemeGroupVersion.WithKind(StackDefinitionKind)
)

func init() {
	SchemeBuilder.Register(&ClusterPackageInstall{}, &ClusterPackageInstallList{})
	SchemeBuilder.Register(&PackageInstall{}, &PackageInstallList{})
	SchemeBuilder.Register(&Package{}, &PackageList{})
	SchemeBuilder.Register(&StackDefinition{}, &StackDefinitionList{})
}
