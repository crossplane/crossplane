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

// Package ccrd generates CustomResourceDefinitions from Crossplane definitions.
//
// v1beta1.JSONSchemaProps is incompatible with controller-tools (as of 0.2.4)
// because it is missing JSON tags and uses float64, which is a disallowed type.
// We thus copy the entire struct as CRDSpecTemplate. See the below issue:
// https://github.com/kubernetes-sigs/controller-tools/issues/291
package ccrd

import (
	"encoding/json"
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

// The kind of a published infrastructure resource is the kind of the defined
// infrastructure resource combined with these suffixes.
const (
	PublishedInfrastructureSuffixKind     = "Requirement"
	PublishedInfrastructureSuffixListKind = "RequirementList"
	PublishedInfrastructureSuffixSingular = "requirement"
	PublishedInfrastructureSuffixPlural   = "requirements"
)

const (
	errNewSpec = "cannot generate CustomResourceDefinition from crdSpecTemplate"
)

// NOTE(muvaf): We use v1beta1.CustomResourceDefinition for backward
// compatibility with clusters pre-1.16

// TODO(muvaf): Every field on top level spec could be a DefinitionOption that is
// reused, although it is known that only two different kinds will be generated.

// An Option configures the supplied CustomResourceDefinition.
type Option func(*v1beta1.CustomResourceDefinition) error

// New produces a new CustomResourceDefinition.
func New(o ...Option) (*v1beta1.CustomResourceDefinition, error) {
	// TODO(muvaf): Add proper descriptions.
	crd := &v1beta1.CustomResourceDefinition{
		Spec: v1beta1.CustomResourceDefinitionSpec{
			PreserveUnknownFields: pointer.BoolPtr(false),
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
	for _, f := range o {
		if err := f(crd); err != nil {
			return nil, err
		}
	}
	return crd, nil
}

// ForInfrastructureDefinition configures the CustomResourceDefinition for the
// supplied InfrastructureDefinition.
func ForInfrastructureDefinition(d *v1alpha1.InfrastructureDefinition) Option {
	return func(crd *v1beta1.CustomResourceDefinition) error {
		spec, err := NewSpec(d.Spec.CRDSpecTemplate)
		if err != nil {
			return errors.Wrap(err, errNewSpec)
		}

		crd.SetName(d.GetName())
		crd.SetLabels(d.GetLabels())
		crd.SetAnnotations(d.GetAnnotations())
		crd.SetOwnerReferences([]metav1.OwnerReference{meta.AsController(
			meta.ReferenceTo(d, v1alpha1.InfrastructureDefinitionGroupVersionKind),
		)})

		crd.Spec.Group = spec.Group
		crd.Spec.Version = spec.Version
		crd.Spec.Versions = spec.Versions
		crd.Spec.Names = spec.Names
		crd.Spec.AdditionalPrinterColumns = spec.AdditionalPrinterColumns
		crd.Spec.Conversion = spec.Conversion
		for k, v := range getSpecProps(*spec) {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["spec"].Properties[k] = v
		}

		return nil
	}
}

// PublishesInfrastructureDefinition configures the CustomResourceDefinition
// that publishes the supplied InfrastructureDefinition.
func PublishesInfrastructureDefinition(d *v1alpha1.InfrastructureDefinition, p *v1alpha1.InfrastructurePublication) Option {
	return func(crd *v1beta1.CustomResourceDefinition) error {
		spec, err := NewSpec(d.Spec.CRDSpecTemplate)
		if err != nil {
			return errors.Wrap(err, errNewSpec)
		}

		crd.SetName(spec.Names.Singular + PublishedInfrastructureSuffixPlural + "." + spec.Group)
		crd.SetLabels(p.GetLabels())
		crd.SetAnnotations(p.GetAnnotations())
		crd.SetOwnerReferences([]metav1.OwnerReference{meta.AsController(
			meta.ReferenceTo(p, v1alpha1.InfrastructureDefinitionGroupVersionKind),
		)})

		crd.Spec.Names = v1beta1.CustomResourceDefinitionNames{
			Kind:     spec.Names.Kind + PublishedInfrastructureSuffixKind,
			ListKind: spec.Names.Kind + PublishedInfrastructureSuffixListKind,
			Singular: spec.Names.Singular + PublishedInfrastructureSuffixSingular,
			Plural:   spec.Names.Singular + PublishedInfrastructureSuffixPlural,
		}

		crd.Spec.Group = spec.Group
		crd.Spec.Version = spec.Version
		crd.Spec.Versions = spec.Versions
		crd.Spec.AdditionalPrinterColumns = spec.AdditionalPrinterColumns
		crd.Spec.Conversion = spec.Conversion
		for k, v := range getSpecProps(*spec) {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["spec"].Properties[k] = v
		}

		return nil
	}
}

// NewSpec produces a CustomResourceDefinitionSpec from the supplied template.
func NewSpec(t v1alpha1.CRDSpecTemplate) (*v1beta1.CustomResourceDefinitionSpec, error) {
	out := &v1beta1.CustomResourceDefinitionSpec{
		Group:                    t.Group,
		Version:                  t.Version,
		Names:                    t.Names,
		AdditionalPrinterColumns: t.AdditionalPrinterColumns,
		Conversion:               t.Conversion,
	}
	if t.Validation != nil {
		s := &v1beta1.JSONSchemaProps{}
		if err := json.Unmarshal(t.Validation.OpenAPIV3Schema.Raw, s); err != nil {
			return nil, err
		}
		out.Validation = &v1beta1.CustomResourceValidation{OpenAPIV3Schema: s}
	}
	for _, version := range t.Versions {
		v := v1beta1.CustomResourceDefinitionVersion{
			Name:                     version.Name,
			Served:                   version.Served,
			Storage:                  version.Storage,
			AdditionalPrinterColumns: version.AdditionalPrinterColumns,
		}
		if version.Schema != nil {
			s := &v1beta1.JSONSchemaProps{}
			if err := json.Unmarshal(version.Schema.OpenAPIV3Schema.Raw, s); err != nil {
				return nil, err
			}
			v.Schema = &v1beta1.CustomResourceValidation{OpenAPIV3Schema: s}
		}
		out.Versions = append(out.Versions, v)
	}
	return out, nil
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

// DefinesCompositeInfrastructure adds the validation fields required by all
// defined infrastructure resources to a CustomResourceDefinition.
func DefinesCompositeInfrastructure() Option {
	return func(crd *v1beta1.CustomResourceDefinition) error {
		crd.Spec.Scope = v1beta1.ClusterScoped
		spec := &map[string]v1beta1.JSONSchemaProps{}
		if err := yaml.Unmarshal([]byte(DefinedInfrastructureSpecProps), spec); err != nil {
			panic(fmt.Sprintf("constant infrastructure composite spec props could not be parsed: %s", err.Error()))
		}
		for k, v := range *spec {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["spec"].Properties[k] = v
		}
		status := &map[string]v1beta1.JSONSchemaProps{}
		if err := yaml.Unmarshal([]byte(DefinedInfrastructureStatusProps), status); err != nil {
			panic(fmt.Sprintf("constant infrastructure composite status props could not be parsed: %s", err.Error()))
		}
		for k, v := range *status {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["status"].Properties[k] = v
		}
		return nil
	}
}

// PublishesCompositeInfrastructure adds the validation fields required by all
// published infrastructure resources to a CustomResourceDefinition.
func PublishesCompositeInfrastructure() Option {
	return func(crd *v1beta1.CustomResourceDefinition) error {
		crd.Spec.Scope = v1beta1.NamespaceScoped
		spec := &map[string]v1beta1.JSONSchemaProps{}
		if err := yaml.Unmarshal([]byte(PublishedInfrastructureSpecProps), spec); err != nil {
			panic(fmt.Sprintf("constant infrastructure composite spec props could not be parsed: %s", err.Error()))
		}
		for k, v := range *spec {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["spec"].Properties[k] = v
		}
		status := &map[string]v1beta1.JSONSchemaProps{}
		if err := yaml.Unmarshal([]byte(PublishedInfrastructureStatusProps), status); err != nil {
			panic(fmt.Sprintf("constant infrastructure composite status props could not be parsed: %s", err.Error()))
		}
		for k, v := range *status {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["status"].Properties[k] = v
		}
		return nil
	}
}

// IsEstablished is a helper function to check whether api-server is ready
// to accept the instances of registered CRD.
func IsEstablished(s v1beta1.CustomResourceDefinitionStatus) bool {
	for _, c := range s.Conditions {
		if c.Type == v1beta1.Established {
			return c.Status == v1beta1.ConditionTrue
		}
	}
	return false
}
