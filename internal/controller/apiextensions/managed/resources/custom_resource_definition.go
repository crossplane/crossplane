/*
Copyright 2025 The Crossplane Authors.

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

// Package resources contains static methods to help render resources for the MRD reconciler.
package resources

import (
	"encoding/json"
	"slices"
	"strings"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

const (
	errParseValidation             = "cannot parse custom resource validation OpenAPIV3Schema"
	errCustomResourceValidationNil = "custom resource validation cannot be nil"
)

// EmptyCustomResourceDefinition returns an empty CRD named in line with the MRD.
func EmptyCustomResourceDefinition(mrd *v1alpha1.ManagedResourceDefinition) *extv1.CustomResourceDefinition {
	return &extv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: mrd.GetName(),
		},
	}
}

// MergeCustomResourceDefinitionInto updates the given crd to match the given mrd.
func MergeCustomResourceDefinitionInto(mrd *v1alpha1.ManagedResourceDefinition, crd *extv1.CustomResourceDefinition) error {
	want := mrd.Spec.CustomResourceDefinitionSpec

	crd.Spec.Group = want.Group
	crd.Spec.Names = want.Names
	crd.Spec.Scope = want.Scope
	crd.Spec.Conversion = want.Conversion
	crd.Spec.PreserveUnknownFields = want.PreserveUnknownFields

	crd.Spec.Versions = make([]extv1.CustomResourceDefinitionVersion, len(want.Versions))
	for i, version := range want.Versions {
		schema, err := toCustomResourceValidation(version.Schema)
		if err != nil {
			return err
		}
		crd.Spec.Versions[i] = extv1.CustomResourceDefinitionVersion{
			Name:                     version.Name,
			Served:                   version.Served,
			Storage:                  version.Storage,
			Deprecated:               version.Deprecated,
			DeprecationWarning:       version.DeprecationWarning,
			Schema:                   schema,
			Subresources:             version.Subresources,
			AdditionalPrinterColumns: version.AdditionalPrinterColumns,
			SelectableFields:         version.SelectableFields,
		}
	}
	// TODO: We are not merging above, we are replacing. If there is something that mutates the CRD, we will fight.

	slices.SortFunc(crd.Spec.Versions, func(a, b extv1.CustomResourceDefinitionVersion) int {
		return strings.Compare(a.Name, b.Name)
	})

	return nil
}

func toCustomResourceValidation(given *v1alpha1.CustomResourceValidation) (*extv1.CustomResourceValidation, error) {
	if given == nil {
		return nil, errors.New(errCustomResourceValidationNil)
	}

	schema := &extv1.JSONSchemaProps{}
	if err := json.Unmarshal(given.OpenAPIV3Schema.Raw, schema); err != nil {
		return nil, errors.Wrap(err, errParseValidation)
	}

	// TODO: We could choose to validate some of the CRD schema here or later.

	return &extv1.CustomResourceValidation{
		OpenAPIV3Schema: schema,
	}, nil
}
