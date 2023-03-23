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

	"k8s.io/apimachinery/pkg/util/validation/field"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// validateConnectionDetailsWithSchemas validates the connection details of a composition. It only checks the
// FromFieldPath as that is the only one we are able to validate with certainty.
func (v *Validator) validateConnectionDetailsWithSchemas(ctx context.Context, comp *v1.Composition) (errs field.ErrorList) {
	for i, resource := range comp.Spec.Resources {
		if len(resource.ConnectionDetails) == 0 {
			continue
		}
		gvk, err := resource.GetObjectGVK()
		if err != nil {
			return append(errs, field.InternalError(field.NewPath("spec", "resources").Index(i), errors.Wrap(err, "cannot get object gvk")))
		}
		crd, err := v.crdGetter.Get(ctx, gvk)
		if err != nil {
			return append(errs, field.InternalError(
				field.NewPath("spec", "resources").Index(i),
				err,
			))
		}
		for j, con := range resource.ConnectionDetails {
			// TODO(phisco): we should validate also other fields of the ConnectionDetail
			// if specified we validate only the FromFieldPath at the moment
			if con.FromFieldPath == nil {
				continue
			}
			_, _, err = validateFieldPath(crd.Spec.Validation.OpenAPIV3Schema, *con.FromFieldPath)
			if err != nil {
				errs = append(errs, field.Invalid(field.NewPath("spec", "resources").Index(i).Child("connectionDetails").Index(j).Child("fromFieldPath"), *con.FromFieldPath, err.Error()))
			}
		}
	}

	return errs
}
