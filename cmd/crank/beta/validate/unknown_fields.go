package validate

import (
	"fmt"
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

	unkFlds := pruning.PruneWithOptions(mr, sch, true, opts)
	for _, f := range unkFlds {
		errs = append(errs, field.InternalError(field.NewPath(f), fmt.Errorf("unknown field \"%s\"", f)))
	}
	return errs
}
