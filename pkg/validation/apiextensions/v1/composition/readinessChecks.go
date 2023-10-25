/*
Copyright 2023 the Crossplane Authors.
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

package composition

import (
	"context"
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	verrors "github.com/crossplane/crossplane/internal/validation/errors"
	xpschema "github.com/crossplane/crossplane/pkg/validation/internal/schema"
)

// validateReadinessChecksWithSchemas validates the readiness check of a composition, given the CRDs of the composed resources.
// It checks that the readiness check field path is valid and that the fields required for the readiness check type are set and valid.
func (v *Validator) validateReadinessChecksWithSchemas(ctx context.Context, comp *v1.Composition) (errs field.ErrorList) {
	for i, resource := range comp.Spec.Resources {
		if len(resource.ReadinessChecks) == 0 {
			continue
		}
		gvk, err := GetBaseObjectGVK(&comp.Spec.Resources[i])
		if err != nil {
			return append(errs, field.InternalError(field.NewPath("spec", "resources").Index(i), errors.Wrap(err, "cannot get object gvk")))
		}
		crd, err := v.crdGetter.Get(ctx, gvk.GroupKind())
		if err != nil {
			return append(errs, field.InternalError(
				field.NewPath("spec", "resources").Index(i),
				err,
			))
		}
		errs = append(errs, verrors.WrapFieldErrorList(validateReadinessChecks(resource, getSchemaForVersion(crd, gvk.Version)), field.NewPath("spec", "resources").Index(i))...)
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func validateReadinessChecks(resource v1.ComposedTemplate, schema *apiextensions.JSONSchemaProps) (errs field.ErrorList) {
	if schema == nil {
		return nil
	}
	for j, r := range resource.ReadinessChecks {
		if r.FieldPath == "" {
			continue
		}
		fieldType, err := validateFieldPath(schema, r.FieldPath)
		if err != nil {
			errs = append(errs, field.Invalid(field.NewPath("readinessCheck").Index(j).Child("fieldPath"), r.FieldPath, err.Error()))
			continue
		}
		if fieldType == "" {
			// nothing to do, we don't have a type defined for the field
			continue
		}
		if matchType := getReadinessCheckExpectedType(r); matchType != "" && matchType != fieldType {
			errs = append(errs, field.Invalid(field.NewPath("readinessCheck").Index(j).Child("fieldPath"), r.FieldPath, fmt.Sprintf("expected field path to be of type %s", matchType)))
			continue
		}
	}
	return errs
}

func getReadinessCheckExpectedType(r v1.ReadinessCheck) xpschema.KnownJSONType {
	var matchType xpschema.KnownJSONType
	switch r.Type {
	case v1.ReadinessCheckTypeMatchString:
		matchType = xpschema.KnownJSONTypeString
	case v1.ReadinessCheckTypeMatchInteger:
		matchType = xpschema.KnownJSONTypeInteger
	case v1.ReadinessCheckTypeMatchTrue, v1.ReadinessCheckTypeMatchFalse:
		matchType = xpschema.KnownJSONTypeBoolean
	case v1.ReadinessCheckTypeNone, v1.ReadinessCheckTypeNonEmpty, v1.ReadinessCheckTypeMatchCondition:
	}
	return matchType
}
