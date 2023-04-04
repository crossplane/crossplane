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

// Package xcrd generates CustomResourceDefinitions from Crossplane definitions.
//
// v1.JSONSchemaProps is incompatible with controller-tools (as of 0.2.4)
// because it is missing JSON tags and uses float64, which is a disallowed type.
// We thus copy the entire struct as CRDSpecTemplate. See the below issue:
// https://github.com/kubernetes-sigs/controller-tools/issues/291
package xcrd

import (
	"encoding/json"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// Category names for generated claim and composite CRDs.
const (
	CategoryClaim     = "claim"
	CategoryComposite = "composite"
)

const (
	errFmtGetProps             = "cannot get %q properties from validation schema"
	errParseValidation         = "cannot parse validation schema"
	errInvalidClaimNames       = "invalid resource claim names"
	errMissingClaimNames       = "missing names"
	errFmtConflictingClaimName = "%q conflicts with composite resource name"
)

// ForCompositeResource derives the CustomResourceDefinition for a composite
// resource from the supplied CompositeResourceDefinition.
func ForCompositeResource(xrd *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
	crd := &extv1.CustomResourceDefinition{
		Spec: extv1.CustomResourceDefinitionSpec{
			Scope:      extv1.ClusterScoped,
			Group:      xrd.Spec.Group,
			Names:      xrd.Spec.Names,
			Versions:   make([]extv1.CustomResourceDefinitionVersion, len(xrd.Spec.Versions)),
			Conversion: xrd.Spec.Conversion,
		},
	}

	crd.SetName(xrd.GetName())
	crd.SetLabels(xrd.GetLabels())
	crd.SetOwnerReferences([]metav1.OwnerReference{meta.AsController(
		meta.TypedReferenceTo(xrd, v1.CompositeResourceDefinitionGroupVersionKind),
	)})

	crd.Spec.Names.Categories = append(crd.Spec.Names.Categories, CategoryComposite)

	for i, vr := range xrd.Spec.Versions {
		crd.Spec.Versions[i] = extv1.CustomResourceDefinitionVersion{
			Name:                     vr.Name,
			Served:                   vr.Served,
			Storage:                  vr.Referenceable,
			Deprecated:               pointer.BoolDeref(vr.Deprecated, false),
			DeprecationWarning:       vr.DeprecationWarning,
			AdditionalPrinterColumns: append(vr.AdditionalPrinterColumns, CompositeResourcePrinterColumns()...),
			Schema: &extv1.CustomResourceValidation{
				OpenAPIV3Schema: BaseProps(),
			},
			Subresources: &extv1.CustomResourceSubresources{
				Status: &extv1.CustomResourceSubresourceStatus{},
			},
		}

		p, required, err := getProps("spec", vr.Schema)
		if err != nil {
			return nil, errors.Wrapf(err, errFmtGetProps, "spec")
		}
		specProps := crd.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["spec"]
		specProps.Required = append(specProps.Required, required...)
		for k, v := range p {
			specProps.Properties[k] = v
		}
		for k, v := range CompositeResourceSpecProps() {
			specProps.Properties[k] = v
		}
		crd.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["spec"] = specProps

		statusP, statusRequired, err := getProps("status", vr.Schema)
		if err != nil {
			return nil, errors.Wrapf(err, errFmtGetProps, "status")
		}
		statusProps := crd.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["status"]
		statusProps.Required = statusRequired
		for k, v := range statusP {
			statusProps.Properties[k] = v
		}
		for k, v := range CompositeResourceStatusProps() {
			statusProps.Properties[k] = v
		}
		crd.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["status"] = statusProps
	}

	return crd, nil
}

// ForCompositeResourceClaim derives the CustomResourceDefinition for a
// composite resource claim from the supplied CompositeResourceDefinition.
func ForCompositeResourceClaim(xrd *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
	if err := validateClaimNames(xrd); err != nil {
		return nil, errors.Wrap(err, errInvalidClaimNames)
	}

	crd := &extv1.CustomResourceDefinition{
		Spec: extv1.CustomResourceDefinitionSpec{
			Scope:      extv1.NamespaceScoped,
			Group:      xrd.Spec.Group,
			Names:      *xrd.Spec.ClaimNames,
			Versions:   make([]extv1.CustomResourceDefinitionVersion, len(xrd.Spec.Versions)),
			Conversion: xrd.Spec.Conversion,
		},
	}

	crd.SetName(xrd.Spec.ClaimNames.Plural + "." + xrd.Spec.Group)
	crd.SetLabels(xrd.GetLabels())
	crd.SetOwnerReferences([]metav1.OwnerReference{meta.AsController(
		meta.TypedReferenceTo(xrd, v1.CompositeResourceDefinitionGroupVersionKind),
	)})

	crd.Spec.Names.Categories = append(crd.Spec.Names.Categories, CategoryClaim)

	for i, vr := range xrd.Spec.Versions {
		crd.Spec.Versions[i] = extv1.CustomResourceDefinitionVersion{
			Name:                     vr.Name,
			Served:                   vr.Served,
			Storage:                  vr.Referenceable,
			Deprecated:               pointer.BoolDeref(vr.Deprecated, false),
			DeprecationWarning:       vr.DeprecationWarning,
			AdditionalPrinterColumns: append(vr.AdditionalPrinterColumns, CompositeResourceClaimPrinterColumns()...),
			Schema: &extv1.CustomResourceValidation{
				OpenAPIV3Schema: BaseProps(),
			},
			Subresources: &extv1.CustomResourceSubresources{
				Status: &extv1.CustomResourceSubresourceStatus{},
			},
		}

		p, required, err := getProps("spec", vr.Schema)
		if err != nil {
			return nil, errors.Wrapf(err, errFmtGetProps, "spec")
		}
		specProps := crd.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["spec"]
		specProps.Required = append(specProps.Required, required...)
		for k, v := range p {
			specProps.Properties[k] = v
		}
		for k, v := range CompositeResourceClaimSpecProps() {
			specProps.Properties[k] = v
		}
		crd.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["spec"] = specProps

		statusP, statusRequired, err := getProps("status", vr.Schema)
		if err != nil {
			return nil, errors.Wrapf(err, errFmtGetProps, "status")
		}
		statusProps := crd.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["status"]
		statusProps.Required = statusRequired
		for k, v := range statusP {
			statusProps.Properties[k] = v
		}
		for k, v := range CompositeResourceStatusProps() {
			statusProps.Properties[k] = v
		}
		crd.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["status"] = statusProps
	}

	return crd, nil
}

func validateClaimNames(d *v1.CompositeResourceDefinition) error {
	if d.Spec.ClaimNames == nil {
		return errors.New(errMissingClaimNames)
	}

	if n := d.Spec.ClaimNames.Kind; n == d.Spec.Names.Kind {
		return errors.Errorf(errFmtConflictingClaimName, n)
	}

	if n := d.Spec.ClaimNames.Plural; n == d.Spec.Names.Plural {
		return errors.Errorf(errFmtConflictingClaimName, n)
	}

	if n := d.Spec.ClaimNames.Singular; n != "" && n == d.Spec.Names.Singular {
		return errors.Errorf(errFmtConflictingClaimName, n)
	}

	if n := d.Spec.ClaimNames.ListKind; n != "" && n == d.Spec.Names.ListKind {
		return errors.Errorf(errFmtConflictingClaimName, n)
	}

	return nil
}

func getProps(field string, v *v1.CompositeResourceValidation) (map[string]extv1.JSONSchemaProps, []string, error) {
	if v == nil {
		return nil, nil, nil
	}

	s := &extv1.JSONSchemaProps{}
	if err := json.Unmarshal(v.OpenAPIV3Schema.Raw, s); err != nil {
		return nil, nil, errors.Wrap(err, errParseValidation)
	}

	spec, ok := s.Properties[field]
	if !ok {
		return nil, nil, nil
	}

	return spec.Properties, spec.Required, nil
}

// IsEstablished is a helper function to check whether api-server is ready
// to accept the instances of registered CRD.
func IsEstablished(s extv1.CustomResourceDefinitionStatus) bool {
	for _, c := range s.Conditions {
		if c.Type == extv1.Established {
			return c.Status == extv1.ConditionTrue
		}
	}
	return false
}
