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

package v1alpha2

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// Package type metadata.
const (
	Group   = "core.oam.dev"
	Version = "v1alpha2"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)

// WorkloadDefinition type metadata.
var (
	WorkloadDefinitionKind             = reflect.TypeOf(WorkloadDefinition{}).Name()
	WorkloadDefinitionGroupKind        = schema.GroupKind{Group: Group, Kind: WorkloadDefinitionKind}.String()
	WorkloadDefinitionKindAPIVersion   = WorkloadDefinitionKind + "." + SchemeGroupVersion.String()
	WorkloadDefinitionGroupVersionKind = SchemeGroupVersion.WithKind(WorkloadDefinitionKind)
)

// TraitDefinition type metadata.
var (
	TraitDefinitionKind             = reflect.TypeOf(TraitDefinition{}).Name()
	TraitDefinitionGroupKind        = schema.GroupKind{Group: Group, Kind: TraitDefinitionKind}.String()
	TraitDefinitionKindAPIVersion   = TraitDefinitionKind + "." + SchemeGroupVersion.String()
	TraitDefinitionGroupVersionKind = SchemeGroupVersion.WithKind(TraitDefinitionKind)
)

// ScopeDefinition type metadata.
var (
	ScopeDefinitionKind             = reflect.TypeOf(ScopeDefinition{}).Name()
	ScopeDefinitionGroupKind        = schema.GroupKind{Group: Group, Kind: ScopeDefinitionKind}.String()
	ScopeDefinitionKindAPIVersion   = ScopeDefinitionKind + "." + SchemeGroupVersion.String()
	ScopeDefinitionGroupVersionKind = SchemeGroupVersion.WithKind(ScopeDefinitionKind)
)

// Component type metadata.
var (
	ComponentKind             = reflect.TypeOf(Component{}).Name()
	ComponentGroupKind        = schema.GroupKind{Group: Group, Kind: ComponentKind}.String()
	ComponentKindAPIVersion   = ComponentKind + "." + SchemeGroupVersion.String()
	ComponentGroupVersionKind = SchemeGroupVersion.WithKind(ComponentKind)
)

// ApplicationConfiguration type metadata.
var (
	ApplicationConfigurationKind             = reflect.TypeOf(ApplicationConfiguration{}).Name()
	ApplicationConfigurationGroupKind        = schema.GroupKind{Group: Group, Kind: ApplicationConfigurationKind}.String()
	ApplicationConfigurationKindAPIVersion   = ApplicationConfigurationKind + "." + SchemeGroupVersion.String()
	ApplicationConfigurationGroupVersionKind = SchemeGroupVersion.WithKind(ApplicationConfigurationKind)
)

// ContainerizedWorkload type metadata.
var (
	ContainerizedWorkloadKind             = reflect.TypeOf(ContainerizedWorkload{}).Name()
	ContainerizedWorkloadGroupKind        = schema.GroupKind{Group: Group, Kind: ContainerizedWorkloadKind}.String()
	ContainerizedWorkloadKindAPIVersion   = ContainerizedWorkloadKind + "." + SchemeGroupVersion.String()
	ContainerizedWorkloadGroupVersionKind = SchemeGroupVersion.WithKind(ContainerizedWorkloadKind)
)

// ManualScalerTrait type metadata.
var (
	ManualScalerTraitKind             = reflect.TypeOf(ManualScalerTrait{}).Name()
	ManualScalerTraitGroupKind        = schema.GroupKind{Group: Group, Kind: ManualScalerTraitKind}.String()
	ManualScalerTraitKindAPIVersion   = ManualScalerTraitKind + "." + SchemeGroupVersion.String()
	ManualScalerTraitGroupVersionKind = SchemeGroupVersion.WithKind(ManualScalerTraitKind)
)

func init() {
	SchemeBuilder.Register(&WorkloadDefinition{}, &WorkloadDefinitionList{})
	SchemeBuilder.Register(&TraitDefinition{}, &TraitDefinitionList{})
	SchemeBuilder.Register(&ScopeDefinition{}, &ScopeDefinitionList{})
	SchemeBuilder.Register(&Component{}, &ComponentList{})
	SchemeBuilder.Register(&ApplicationConfiguration{}, &ApplicationConfigurationList{})
	SchemeBuilder.Register(&ContainerizedWorkload{}, &ContainerizedWorkloadList{})
	SchemeBuilder.Register(&ManualScalerTrait{}, &ManualScalerTraitList{})
}
