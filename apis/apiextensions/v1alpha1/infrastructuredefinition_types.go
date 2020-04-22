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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// InfrastructureDefinitionSpec specifies the desired state of the definition.
type InfrastructureDefinitionSpec struct {

	// ConnectionSecretKeys is the list of keys that will be exposed to the end
	// user of the defined kind.
	ConnectionSecretKeys []string `json:"connectionSecretKeys,omitempty"`

	// CRDSpecTemplate is the base CRD template. The final CRD will have additional
	// fields to the base template to accommodate Crossplane machinery.
	CRDSpecTemplate CustomResourceDefinitionSpec `json:"crdSpecTemplate,omitempty"`
}

// InfrastructureDefinitionStatus shows the observed state of the definition.
type InfrastructureDefinitionStatus struct {
	v1alpha1.ConditionedStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// InfrastructureDefinition is used to define a resource claim that can be
// scheduled to one of the available compatible compositions.
// +kubebuilder:resource:categories={crossplane}
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
type InfrastructureDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfrastructureDefinitionSpec   `json:"spec,omitempty"`
	Status InfrastructureDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InfrastructureDefinitionList contains a list of InfrastructureDefinitions.
type InfrastructureDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfrastructureDefinition `json:"items"`
}

// GetDefinedGroupVersionKind returns the schema.GroupVersionKind of the CRD that this
// InfrastructureDefinition instance will define.
func (in InfrastructureDefinition) GetDefinedGroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   in.Spec.CRDSpecTemplate.Group,
		Version: in.Spec.CRDSpecTemplate.Version,
		Kind:    in.Spec.CRDSpecTemplate.Names.Kind,
	}
}

// GenerateCRD returns generated CRD with given CRD Spec Template applied as
// overlay.
func (in *InfrastructureDefinition) GenerateCRD() (*v1beta1.CustomResourceDefinition, error) {
	crdSpec, err := FromShallow(in.Spec.CRDSpecTemplate)
	if err != nil {
		return nil, errors.Wrap(err, errConvertCRDTemplate)
	}
	base := BaseCRD(InfraValidation())
	base.SetName(in.GetName())
	base.Spec.Group = crdSpec.Group
	base.Spec.Version = crdSpec.Version
	base.Spec.Versions = crdSpec.Versions
	base.Spec.Names = crdSpec.Names
	base.Spec.AdditionalPrinterColumns = crdSpec.AdditionalPrinterColumns
	base.Spec.Conversion = crdSpec.Conversion
	base.SetLabels(in.GetLabels())
	base.SetAnnotations(in.GetAnnotations())
	for k, v := range getSpecProps(*crdSpec) {
		base.Spec.Validation.OpenAPIV3Schema.Properties["spec"].Properties[k] = v
	}
	return base, nil
}

// GetConnectionSecretKeys returns the set of allowed keys to filter the connection
// secret.
func (in *InfrastructureDefinition) GetConnectionSecretKeys() []string {
	return in.Spec.ConnectionSecretKeys
}
