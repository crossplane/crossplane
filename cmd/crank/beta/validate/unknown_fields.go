/*
Copyright 2024 The Crossplane Authors.

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

package validate

import (
	"fmt"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/pruning"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// validateUnknownFields Validates the resource's unknown fields against the given schema and returns a list of errors.
func validateUnknownFields(mr map[string]interface{}, sch *schema.Structural) field.ErrorList {
	opts := schema.UnknownFieldPathOptions{
		TrackUnknownFieldPaths: true, // to get the list of pruned unknown fields
	}
	errs := field.ErrorList{}

	uf := pruning.PruneWithOptions(mr, sch, true, opts)
	for _, f := range uf {
		strPath := strings.Split(f, ".")
		child := strPath[len(strPath)-1]
		errs = append(errs, field.Invalid(field.NewPath(f), child, fmt.Sprintf("unknown field: \"%s\"", child)))
	}
	return errs
}
