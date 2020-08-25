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

	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

const (
	errNewSpec                 = "cannot generate CustomResourceDefinition from crdSpecTemplate"
	errParseValidation         = "cannot parse validation schema"
	errInvalidClaimNames       = "invalid resource claim names"
	errMissingClaimNames       = "missing names"
	errFmtConflictingClaimName = "%q conflicts with composite resource name"
)

// NOTE(muvaf): We use v1beta1.CustomResourceDefinition for backward
// compatibility with clusters pre-1.16

// TODO(muvaf): Every field on top level spec could be a DefinitionOption that is
// reused, although it is known that only two different kinds will be generated.

// An Option configures the supplied CustomResourceDefinition.
type Option func(*v1beta1.CustomResourceDefinition) error

// New produces a new CustomResourceDefinition.
func New(o ...Option) (*v1beta1.CustomResourceDefinition, error) {
	crd := &v1beta1.CustomResourceDefinition{
		Spec: v1beta1.CustomResourceDefinitionSpec{
			PreserveUnknownFields: pointer.BoolPtr(false),
			Subresources: &v1beta1.CustomResourceSubresources{
				Status: &v1beta1.CustomResourceSubresourceStatus{},
			},
			Validation: &v1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &v1beta1.JSONSchemaProps{
					Type:       "object",
					Properties: BaseProps(),
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

// ForCompositeResource derives the CustomResourceDefinition for a composite
// resource from the supplied CompositeResourceDefinition.
func ForCompositeResource(d *v1alpha1.CompositeResourceDefinition) Option {
	return func(crd *v1beta1.CustomResourceDefinition) error {
		spec, err := NewSpec(d.Spec.CRDSpecTemplate)
		if err != nil {
			return errors.Wrap(err, errNewSpec)
		}

		crd.SetName(d.GetName())
		crd.SetLabels(d.GetLabels())
		crd.SetAnnotations(d.GetAnnotations())
		crd.SetOwnerReferences([]metav1.OwnerReference{meta.AsController(
			meta.ReferenceTo(d, v1alpha1.CompositeResourceDefinitionGroupVersionKind),
		)})
		crd.Spec.AdditionalPrinterColumns = CompositeResourcePrinterColumns()

		crd.Spec.Group = spec.Group
		crd.Spec.Version = spec.Version
		crd.Spec.Names = spec.Names
		crd.Spec.AdditionalPrinterColumns = append(crd.Spec.AdditionalPrinterColumns, spec.AdditionalPrinterColumns...)
		for k, v := range getSpecProps(spec) {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["spec"].Properties[k] = v
		}

		crd.Spec.Scope = v1beta1.ClusterScoped
		for k, v := range CompositeResourceSpecProps() {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["spec"].Properties[k] = v
		}
		for k, v := range CompositeResourceStatusProps() {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["status"].Properties[k] = v
		}

		return nil
	}
}

// ForCompositeResourceClaim derives the CustomResourceDefinition for a
// composite resource claim from the supplied CompositeResourceDefinition.
func ForCompositeResourceClaim(d *v1alpha1.CompositeResourceDefinition) Option {
	return func(crd *v1beta1.CustomResourceDefinition) error {
		spec, err := NewSpec(d.Spec.CRDSpecTemplate)
		if err != nil {
			return errors.Wrap(err, errNewSpec)
		}

		if err := validateClaimNames(d); err != nil {
			return errors.Wrap(err, errInvalidClaimNames)
		}

		crd.SetName(d.Spec.ClaimNames.Plural + "." + spec.Group)
		crd.SetLabels(d.GetLabels())
		crd.SetAnnotations(d.GetAnnotations())
		crd.SetOwnerReferences([]metav1.OwnerReference{meta.AsController(
			meta.ReferenceTo(d, v1alpha1.CompositeResourceDefinitionGroupVersionKind),
		)})

		crd.Spec.Names = v1beta1.CustomResourceDefinitionNames{
			Kind:     d.Spec.ClaimNames.Kind,
			ListKind: d.Spec.ClaimNames.ListKind,
			Singular: d.Spec.ClaimNames.Singular,
			Plural:   d.Spec.ClaimNames.Plural,
		}
		crd.Spec.AdditionalPrinterColumns = CompositeResourceClaimPrinterColumns()

		crd.Spec.Group = spec.Group
		crd.Spec.Version = spec.Version
		crd.Spec.AdditionalPrinterColumns = append(crd.Spec.AdditionalPrinterColumns, spec.AdditionalPrinterColumns...)
		for k, v := range getSpecProps(spec) {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["spec"].Properties[k] = v
		}

		crd.Spec.Scope = v1beta1.NamespaceScoped
		for k, v := range CompositeResourceClaimSpecProps() {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["spec"].Properties[k] = v
		}
		for k, v := range CompositeResourceStatusProps() {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["status"].Properties[k] = v
		}

		return nil
	}
}

func validateClaimNames(d *v1alpha1.CompositeResourceDefinition) error {
	if d.Spec.ClaimNames == nil {
		return errors.New(errMissingClaimNames)
	}

	if n := d.Spec.ClaimNames.Kind; n == d.Spec.CRDSpecTemplate.Names.Kind {
		return errors.Errorf(errFmtConflictingClaimName, n)
	}

	if n := d.Spec.ClaimNames.Plural; n == d.Spec.CRDSpecTemplate.Names.Plural {
		return errors.Errorf(errFmtConflictingClaimName, n)
	}

	if n := d.Spec.ClaimNames.Singular; n != "" && n == d.Spec.CRDSpecTemplate.Names.Singular {
		return errors.Errorf(errFmtConflictingClaimName, n)
	}

	if n := d.Spec.ClaimNames.ListKind; n != "" && n == d.Spec.CRDSpecTemplate.Names.ListKind {
		return errors.Errorf(errFmtConflictingClaimName, n)
	}

	return nil
}

// NewSpec produces a CustomResourceDefinitionSpec from the supplied template.
func NewSpec(t v1alpha1.CRDSpecTemplate) (v1beta1.CustomResourceDefinitionSpec, error) {
	out := v1beta1.CustomResourceDefinitionSpec{
		Group:                    t.Group,
		Version:                  t.Version,
		Names:                    t.Names,
		AdditionalPrinterColumns: t.AdditionalPrinterColumns,
	}
	if t.Validation != nil {
		s := &v1beta1.JSONSchemaProps{}
		if err := json.Unmarshal(t.Validation.OpenAPIV3Schema.Raw, s); err != nil {
			return v1beta1.CustomResourceDefinitionSpec{}, errors.Wrap(err, errParseValidation)
		}
		out.Validation = &v1beta1.CustomResourceValidation{OpenAPIV3Schema: s}
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
