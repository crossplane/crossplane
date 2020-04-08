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

package crds

import (
	"bytes"
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

const (
	errDecodeCRDTemplate = "cannot decode given crd spec template"
)

// CRDOption is used to manipulate base crd.
type CRDOption func(*v1beta1.CustomResourceDefinition)

// NOTE(muvaf): We use v1beta1.CustomResourceDefinition for backward compatibility
// with clusters pre-1.16

// BaseCRD returns a base template for generating a CRD.
func BaseCRD(opts ...CRDOption) *v1beta1.CustomResourceDefinition {
	falseVal := false
	// TODO(muvaf): Add proper descriptions.
	crd := &v1beta1.CustomResourceDefinition{
		Spec: v1beta1.CustomResourceDefinitionSpec{
			PreserveUnknownFields: &falseVal,
			Subresources: &v1beta1.CustomResourceSubresources{
				Status: &v1beta1.CustomResourceSubresourceStatus{},
			},
			Validation: &v1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &v1beta1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]v1beta1.JSONSchemaProps{
						"apiVersion": {
							Type: "string",
						},
						"kind": {
							Type: "string",
						},
						"metadata": {
							// NOTE(muvaf): api-server takes care of validating
							// metadata.
							Type: "object",
						},
						"spec": {
							Type:       "object",
							Properties: map[string]v1beta1.JSONSchemaProps{},
						},
						"status": {
							Type:       "object",
							Properties: map[string]v1beta1.JSONSchemaProps{},
						},
					},
				},
			},
		},
	}
	for _, f := range opts {
		f(crd)
	}
	return crd
}

// TODO(muvaf): move to a more programmatic approach where we have a func
// that accepts a type and returns validation.

// InfraValidation returns a CRDOption that adds infrastructure related fields
// to the base CRD.
func InfraValidation() CRDOption {
	return func(crd *v1beta1.CustomResourceDefinition) {
		crd.Spec.Scope = v1beta1.ClusterScoped
		spec := &map[string]v1beta1.JSONSchemaProps{}
		if err := yaml.Unmarshal([]byte(v1alpha1.InfraSpecProps), spec); err != nil {
			// TODO(muvaf): never panic.
			panic(fmt.Sprintf("constant string could not be parsed: %s", err.Error()))
		}
		for k, v := range *spec {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["spec"].Properties[k] = v
		}
		status := &map[string]v1beta1.JSONSchemaProps{}
		if err := yaml.Unmarshal([]byte(v1alpha1.InfraStatusProps), status); err != nil {
			// TODO(muvaf): never panic.
			panic(fmt.Sprintf("constant string could not be parsed: %s", err.Error()))
		}
		for k, v := range *status {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["status"].Properties[k] = v
		}
	}
}

// GenerateInfraCRD returns a CRD that is generated with the information in cr.
func GenerateInfraCRD(cr *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
	dec := kyaml.NewYAMLOrJSONDecoder(bytes.NewReader(cr.Spec.CRDSpecTemplate.Raw), 4096)
	crdSpec := &v1beta1.CustomResourceDefinitionSpec{}
	if err := dec.Decode(crdSpec); err != nil {
		return nil, errors.Wrap(err, errDecodeCRDTemplate)
	}
	base := BaseCRD(InfraValidation())
	base.SetName(cr.GetName())
	base.Spec.Group = crdSpec.Group
	base.Spec.Version = crdSpec.Version
	base.Spec.Versions = crdSpec.Versions
	base.Spec.Names = crdSpec.Names
	base.Spec.AdditionalPrinterColumns = crdSpec.AdditionalPrinterColumns
	base.Spec.Conversion = crdSpec.Conversion
	for k, v := range getSpecProps(*crdSpec) {
		base.Spec.Validation.OpenAPIV3Schema.Properties["spec"].Properties[k] = v
	}
	return base, nil
}

func getSpecProps(template v1beta1.CustomResourceDefinitionSpec) map[string]v1beta1.JSONSchemaProps {
	switch {
	case template.Validation == nil:
		return nil
	case template.Validation.OpenAPIV3Schema == nil:
		return nil
	case len(template.Validation.OpenAPIV3Schema.Properties) == 0:
		return nil
	case len(template.Validation.OpenAPIV3Schema.Properties["spec"].Properties) == 0:
		return nil
	}
	return template.Validation.OpenAPIV3Schema.Properties["spec"].Properties
}
