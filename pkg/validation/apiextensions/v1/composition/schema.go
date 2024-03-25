package composition

import (
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"

	"github.com/crossplane/crossplane/pkg/validation/internal/schema"
)

// sets all the defaults in the given schema.
func defaultMetadataSchema(in *apiextensions.JSONSchemaProps) *apiextensions.JSONSchemaProps {
	out := in
	if out == nil {
		out = &apiextensions.JSONSchemaProps{}
	}
	if out.Type == "" {
		out.Type = string(schema.KnownJSONTypeObject)
	}
	if out.Properties == nil {
		out.Properties = map[string]apiextensions.JSONSchemaProps{}
	}
	if _, exists := out.Properties["metadata"]; !exists {
		out.Properties["metadata"] = apiextensions.JSONSchemaProps{}
	}
	metadata := out.Properties["metadata"]
	out.Properties["metadata"] = *defaultMetadataOnly(&metadata)

	return out
}

func defaultMetadataOnly(metadata *apiextensions.JSONSchemaProps) *apiextensions.JSONSchemaProps {
	setDefaultType(metadata)
	setDefaultProperty(metadata, "name", string(schema.KnownJSONTypeString))
	setDefaultProperty(metadata, "namespace", string(schema.KnownJSONTypeString))
	setDefaultProperty(metadata, "uid", string(schema.KnownJSONTypeString))
	setDefaultProperty(metadata, "generateName", string(schema.KnownJSONTypeString))
	setDefaultLabels(metadata)
	setDefaultAnnotations(metadata)
	return metadata
}

func setDefaultType(metadata *apiextensions.JSONSchemaProps) {
	if metadata.Type == "" {
		metadata.Type = string(schema.KnownJSONTypeObject)
	}
}

func setDefaultProperty(metadata *apiextensions.JSONSchemaProps, propertyName string, defaultType string) {
	if metadata.Properties == nil {
		metadata.Properties = map[string]apiextensions.JSONSchemaProps{}
	}
	if _, exists := metadata.Properties[propertyName]; !exists {
		metadata.Properties[propertyName] = apiextensions.JSONSchemaProps{}
	}
	property := metadata.Properties[propertyName]
	if property.Type == "" {
		property.Type = defaultType
	}
	metadata.Properties[propertyName] = property
}

func setDefaultLabels(metadata *apiextensions.JSONSchemaProps) {
	setDefaultProperty(metadata, "labels", string(schema.KnownJSONTypeObject))
	labels := metadata.Properties["labels"]
	if labels.AdditionalProperties == nil {
		labels.AdditionalProperties = &apiextensions.JSONSchemaPropsOrBool{}
	}
	if labels.AdditionalProperties.Schema == nil {
		labels.AdditionalProperties.Schema = &apiextensions.JSONSchemaProps{}
	}
	if labels.AdditionalProperties.Schema.Type == "" {
		labels.AdditionalProperties.Schema.Type = string(schema.KnownJSONTypeString)
	}
	metadata.Properties["labels"] = labels
}

func setDefaultAnnotations(metadata *apiextensions.JSONSchemaProps) {
	setDefaultProperty(metadata, "annotations", string(schema.KnownJSONTypeObject))
	annotations := metadata.Properties["annotations"]
	if annotations.AdditionalProperties == nil {
		annotations.AdditionalProperties = &apiextensions.JSONSchemaPropsOrBool{}
	}
	if annotations.AdditionalProperties.Schema == nil {
		annotations.AdditionalProperties.Schema = &apiextensions.JSONSchemaProps{}
	}
	if annotations.AdditionalProperties.Schema.Type == "" {
		annotations.AdditionalProperties.Schema.Type = string(schema.KnownJSONTypeString)
	}
	metadata.Properties["annotations"] = annotations
}
