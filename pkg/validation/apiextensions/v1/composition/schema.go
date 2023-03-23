package composition

import (
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"

	"github.com/crossplane/crossplane/pkg/validation/schema"
)

var (
	// hardcoded metadata schema as CRDs usually don't contain it, but we need the information to be able
	// to validate patches from `metadata.uid` or similar fields
	metadataSchema = apiextensions.JSONSchemaProps{
		Type: string(schema.KnownJSONTypeObject),
		AdditionalProperties: &apiextensions.JSONSchemaPropsOrBool{
			Allows: true,
		},
		Properties: map[string]apiextensions.JSONSchemaProps{
			"name": {
				Type: string(schema.KnownJSONTypeString),
			},
			"namespace": {
				Type: string(schema.KnownJSONTypeString),
			},
			"labels": {
				Type: string(schema.KnownJSONTypeObject),
				AdditionalProperties: &apiextensions.JSONSchemaPropsOrBool{
					Schema: &apiextensions.JSONSchemaProps{
						Type: string(schema.KnownJSONTypeString),
					},
				},
			},
			"annotations": {
				Type: string(schema.KnownJSONTypeObject),
				AdditionalProperties: &apiextensions.JSONSchemaPropsOrBool{
					Schema: &apiextensions.JSONSchemaProps{
						Type: string(schema.KnownJSONTypeString),
					},
				},
			},
			"uid": {
				Type: string(schema.KnownJSONTypeString),
			},
		},
	}
)
