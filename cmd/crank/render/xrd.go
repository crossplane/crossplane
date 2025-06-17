package render

import (
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	schema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	structuraldefaulting "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// DefaultValues sets default values on the XR based on the CRD schema.
func DefaultValues(xr map[string]any, apiVersion string, crd extv1.CustomResourceDefinition) error {
	var k apiextensions.JSONSchemaProps
	var version *extv1.CustomResourceDefinitionVersion
	for _, vr := range crd.Spec.Versions {
		checkAPIVersion := crd.Spec.Group + "/" + vr.Name
		if checkAPIVersion == apiVersion {
			version = &vr
			break
		}
	}
	if version == nil {
		return errors.Errorf("the specified API version '%s' does not exist in the XRD", apiVersion)
	}
	if err := extv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(version.Schema.OpenAPIV3Schema, &k, nil); err != nil {
		return err
	}
	crdWithDefaults, err := schema.NewStructural(&k)
	if err != nil {
		return err
	}
	structuraldefaulting.Default(xr, crdWithDefaults)
	return nil
}
