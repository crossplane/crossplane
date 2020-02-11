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
	Group   = "stacks.crossplane.io"
	Version = "v1alpha1"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)

// StackInstall type metadata.
var (
	StackInstallKind             = reflect.TypeOf(StackInstall{}).Name()
	StackInstallGroupKind        = schema.GroupKind{Group: Group, Kind: StackInstallKind}.String()
	StackInstallKindAPIVersion   = StackInstallKind + "." + SchemeGroupVersion.String()
	StackInstallGroupVersionKind = SchemeGroupVersion.WithKind(StackInstallKind)
)

// ClusterStackInstall type metadata.
var (
	ClusterStackInstallKind             = reflect.TypeOf(ClusterStackInstall{}).Name()
	ClusterStackInstallGroupKind        = schema.GroupKind{Group: Group, Kind: ClusterStackInstallKind}.String()
	ClusterStackInstallKindAPIVersion   = ClusterStackInstallKind + "." + SchemeGroupVersion.String()
	ClusterStackInstallGroupVersionKind = SchemeGroupVersion.WithKind(ClusterStackInstallKind)
)

// Stack type metadata.
var (
	StackKind             = reflect.TypeOf(Stack{}).Name()
	StackGroupKind        = schema.GroupKind{Group: Group, Kind: StackKind}.String()
	StackKindAPIVersion   = StackKind + "." + SchemeGroupVersion.String()
	StackGroupVersionKind = SchemeGroupVersion.WithKind(StackKind)
)

// StackConfiguration type metadata
var (
	StackConfigurationKind             = reflect.TypeOf(StackConfiguration{}).Name()
	StackConfigurationGroupKind        = schema.GroupKind{Group: Group, Kind: StackConfigurationKind}.String()
	StackConfigurationKindAPIVersion   = StackConfigurationKind + "." + SchemeGroupVersion.String()
	StackConfigurationGroupVersionKind = SchemeGroupVersion.WithKind(StackConfigurationKind)
)

// StackDefinition type metadata
var (
	StackDefinitionKind             = reflect.TypeOf(StackDefinition{}).Name()
	StackDefinitionGroupKind        = schema.GroupKind{Group: Group, Kind: StackDefinitionKind}.String()
	StackDefinitionKindAPIVersion   = StackDefinitionKind + "." + SchemeGroupVersion.String()
	StackDefinitionGroupVersionKind = SchemeGroupVersion.WithKind(StackDefinitionKind)
)

func init() {
	SchemeBuilder.Register(&ClusterStackInstall{}, &ClusterStackInstallList{})
	SchemeBuilder.Register(&StackInstall{}, &StackInstallList{})
	SchemeBuilder.Register(&Stack{}, &StackList{})
	SchemeBuilder.Register(&StackConfiguration{}, &StackConfigurationList{})
	SchemeBuilder.Register(&StackDefinition{}, &StackDefinitionList{})
}
