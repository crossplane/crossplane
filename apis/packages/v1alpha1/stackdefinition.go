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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
)

// StackDefinitionSpec defines the desired state of StackDefinition
type StackDefinitionSpec struct {
	PackageSpec `json:",inline"`

	Behavior Behavior `json:"behavior,omitempty"`
}

// StackDefinitionSource is the stack image which this stack configuration is from.
// In the future, other source types may be supported, such as a URL.
type StackDefinitionSource struct {
	// a container image id
	Image string `json:"image,omitempty"`
	// The path to the files to process in the source
	Path string `json:"path"`
}

// Behavior specifies the behavior for the stack, if the stack has behaviors instead of a controller
type Behavior struct {
	CRD    BehaviorCRD                      `json:"crd,omitempty"`
	Engine StackResourceEngineConfiguration `json:"engine,omitempty"`
	// Theoretically, source and engine could be specified at a per-hook level
	// as well.
	Source StackDefinitionSource `json:"source,omitempty"`
}

// BehaviorCRD represents the CRD which the stack's behavior controller will watch. When CRs of this
// CRD kind appear and are modified in the cluster, the behavior will execute.
type BehaviorCRD struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

// StackResourceEngineConfiguration represents a configuration for a resource engine, such as helm2 or kustomize.
type StackResourceEngineConfiguration struct {
	// ControllerImage is the image of the generic controller used to reconcile
	// the instances of the given CRDs. If empty, it is populated by package manager
	// during unpack with default value.
	// +optional
	ControllerImage string `json:"controllerImage,omitempty"`
	// Type is the engine type, such as "helm2" or "kustomize"
	Type string `json:"type"`
	// Because different engine configurations could specify conflicting field names,
	// we want to namespace the engines with engine-specific keys
	// +optional
	Kustomize *KustomizeEngineConfiguration `json:"kustomize,omitempty"`
}

// KustomizeEngineConfiguration provides kustomize-specific engine configuration.
type KustomizeEngineConfiguration struct {
	Overlays      []KustomizeEngineOverlay   `json:"overlays,omitempty"`
	Kustomization *unstructured.Unstructured `json:"kustomization,omitempty"`
}

// KustomizeEngineOverlay configures the stack behavior controller to transform the input CR into some output objects
// for the underlying resource engine. This is expected to be interpreted by the engine-specific logic in the controller.
type KustomizeEngineOverlay struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Name       string         `json:"name"`
	Bindings   []FieldBinding `json:"bindings"`
}

// FieldBinding describes a field binding of a transformation from the triggering CR to an object for configuring the resource engine.
// It connects a field in the source object to a field in the destination object.
type FieldBinding struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// StackDefinitionStatus defines the observed state of StackDefinition
type StackDefinitionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// StackDefinition is the Schema for the StackDefinitions API
// +kubebuilder:resource:categories=crossplane
type StackDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StackDefinitionSpec   `json:"spec,omitempty"`
	Status StackDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StackDefinitionList contains a list of StackDefinition
type StackDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StackDefinition `json:"items"`
}

// DeepCopyIntoPackage copies a StackDefinition to a Package
func (sd *StackDefinition) DeepCopyIntoPackage(p *Package) {
	sd.Spec.AppMetadataSpec.DeepCopyInto(&p.Spec.AppMetadataSpec)
	sd.Spec.CRDs.DeepCopyInto(&p.Spec.CRDs)
	sd.Spec.Controller.DeepCopyInto(&p.Spec.Controller)
	sd.Spec.Permissions.DeepCopyInto(&p.Spec.Permissions)
	meta.AddLabels(p, sd.GetLabels())
}

// DeepCopyIntoStackDefinition copies a Stack to a StackDefinition
// TODO(displague) should this reside in types.go with the Stack type
func (p *Package) DeepCopyIntoStackDefinition(sd *StackDefinition) {
	p.Spec.AppMetadataSpec.DeepCopyInto(&sd.Spec.AppMetadataSpec)
	p.Spec.CRDs.DeepCopyInto(&sd.Spec.CRDs)
	p.Spec.Controller.DeepCopyInto(&sd.Spec.Controller)
	p.Spec.Permissions.DeepCopyInto(&sd.Spec.Permissions)
	meta.AddLabels(sd, p.GetLabels())
}
