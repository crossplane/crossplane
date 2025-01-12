package render

import (
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	schema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	structuraldefaulting "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
)

// DefaultValues sets default values on the XR based on the CRD schema.
func DefaultValues(xr map[string]any, crd extv1.CustomResourceDefinition) error {
	var k apiextensions.JSONSchemaProps
	err := extv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(crd.Spec.Versions[0].Schema.OpenAPIV3Schema, &k, nil)
	if err != nil {
		return err
	}
	crdWithDefaults, err := schema.NewStructural(&k)
	if err != nil {
		return err
	}
	structuraldefaulting.Default(xr, crdWithDefaults)
	return nil
}
