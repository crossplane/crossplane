package validate

import (
	"fmt"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func validateRecursive(fields map[string]interface{}, sp map[string]extv1.JSONSchemaProps, p *field.Path, errs *field.ErrorList, stdFlds map[string]bool) error {
	for key, val := range fields {
		if stdFlds[key] {
			continue
		}
		s, ok := sp[key]
		if ok {
			if s.Type == "object" && s.Properties != nil {
				cf, _ := val.(map[string]interface{})
				if err := validateRecursive(cf, s.Properties, p.Child(key), errs, stdFlds); err != nil {
					return err
				}
			}
		} else {
			e := field.InternalError(p.Child(key), fmt.Errorf("unknown field \"%s\"", p.Child(key).String()))

			*errs = append(*errs, e)
		}
	}
	return nil
}

// Validate validates the given resource against the CRD schema.
func Validate(crd extv1.CustomResourceDefinition, mr unstructured.Unstructured) (field.ErrorList, error) {
	// Standard Kubernetes fields in all resources that should not be validated against CRD specific schema
	stdFlds := map[string]bool{
		"apiVersion": true,
		"kind":       true,
		"metadata":   true,
	}

	s := crd.Spec.Versions[0].Schema.OpenAPIV3Schema
	fields := mr.UnstructuredContent()
	errs := field.ErrorList{}
	err := validateRecursive(fields, s.Properties, nil, &errs, stdFlds)

	return errs, err
}
