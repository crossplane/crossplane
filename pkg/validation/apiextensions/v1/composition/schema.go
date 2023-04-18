package composition

import (
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"

	"github.com/crossplane/crossplane/pkg/validation/internal/schema"
)

func getMetadataSchema() *apiextensions.JSONSchemaProps {
	// hardcoded metadata schema as CRDs usually don't contain it, but we need the information to be able
	// to validate patches from `metadata.uid` or similar fields
	return &apiextensions.JSONSchemaProps{
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
					Allows: true,
					Schema: &apiextensions.JSONSchemaProps{
						Type: string(schema.KnownJSONTypeString),
					},
				},
			},
			"annotations": {
				Type: string(schema.KnownJSONTypeObject),
				AdditionalProperties: &apiextensions.JSONSchemaPropsOrBool{
					Allows: true,
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
}

func defaultMetadataSchema(in *apiextensions.JSONSchemaProps) {
	if in == nil {
		in = &apiextensions.JSONSchemaProps{}
	}
	if in.Type == "" {
		in.Type = string(schema.KnownJSONTypeObject)
	}
	if in.Properties == nil {
		in.Properties = map[string]apiextensions.JSONSchemaProps{}
	}
	if _, exists := in.Properties["metadata"]; !exists {
		in.Properties["metadata"] = apiextensions.JSONSchemaProps{
			Type: string(schema.KnownJSONTypeObject),
		}
	}
	metadata := in.Properties["metadata"]
	if metadata.Properties == nil {
		metadata.Properties = map[string]apiextensions.JSONSchemaProps{}
	}
	for name, prop := range getMetadataSchema().Properties {
		if _, exists := metadata.Properties[name]; !exists {
			metadata.Properties[name] = prop
			continue
		}
		t := metadata.Properties[name]
		if metadata.Properties[name].Type == "" {
			t.Type = prop.Type
		}
		if metadata.Properties[name].AdditionalProperties == nil {
			t.AdditionalProperties = prop.AdditionalProperties
		}
		metadata.Properties[name] = t

	}
	in.Properties["metadata"] = metadata
}
