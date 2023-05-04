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

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	verrors "github.com/crossplane/crossplane/internal/validation/errors"
)

// validateConnectionDetailsWithSchemas validates the connection details of a composition. It only checks the
// FromFieldPath as that is the only one we are able to validate with certainty.
func (v *Validator) validateConnectionDetailsWithSchemas(ctx context.Context, comp *v1.Composition) (errs field.ErrorList) {
	for i, resource := range comp.Spec.Resources {
		if len(resource.ConnectionDetails) == 0 {
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
		for j, con := range resource.ConnectionDetails {
			if err := validateConnectionDetail(con, getSchemaForVersion(crd, gvk.Version)); err != nil {
				errs = append(errs, verrors.WrapFieldError(err, field.NewPath("spec", "resources").Index(i).Child("connectionDetails").Index(j)))
			}
		}
	}

	return errs
}

func validateConnectionDetail(con v1.ConnectionDetail, schema *apiextensions.JSONSchemaProps) *field.Error {
	if schema == nil {
		return nil
	}
	// If defined we validate it, logical validation should enforce consistency if needed.
	if con.FromFieldPath != nil {
		if _, err := validateFieldPath(schema, *con.FromFieldPath); err != nil {
			return field.Invalid(field.NewPath("fromFieldPath"), *con.FromFieldPath, err.Error())
		}
	}
	// We don't validate other fields now as they do not have a schema to validate against.
	return nil
}
