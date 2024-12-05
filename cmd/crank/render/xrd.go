package render

import (
	"encoding/json"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func ConstructCRDSchema(crd extv1.CustomResourceDefinition) map[string]interface{} {
	schema := crd.Spec.Versions[0].Schema.OpenAPIV3Schema
	return crdSchema(schema, make(map[string]interface{}))
}

// crdSchema recursively constructs a map of the CRD schema with default values.
func crdSchema(schema *extv1.JSONSchemaProps, crdSchemaWithDefaults map[string]interface{}) map[string]interface{} {
	for k, v := range schema.Properties {
		if v.Default != nil {
			var jsonObj interface{}
			if err := json.Unmarshal(v.Default.Raw, &jsonObj); err == nil {
				// If unmarshalling is successful, assign the JSON object
				crdSchemaWithDefaults[k] = jsonObj
			} else {
				// Otherwise, assign the raw string
				crdSchemaWithDefaults[k] = string(v.Default.Raw)
			}
		} else if v.Type == "object" {
			crdSchemaWithDefaults[k] = make(map[string]interface{})
			crdSchema(&v, crdSchemaWithDefaults[k].(map[string]interface{}))
		}
	}
	return crdSchemaWithDefaults
}

// MergeXRDDefaultsIntoXR merges the default values from the CRD schema into the XR.
func MergeXRDDefaultsIntoXR(xr, xrd map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	// First copy all from xr
	for k, v := range xr {
		merged[k] = v
	}

	// Then check xrds for missing values
	for k, xrdVal := range xrd {

		// Skip if value is nil
		if xrdVal == nil {
			continue
		}

		// If key doesn't exist in merged, add it
		if mergedVal, exists := merged[k]; !exists {
			// Try to parse JSON string before adding
			if strVal, isString := xrdVal.(string); isString && len(strVal) > 0 && strVal[0] == '{' {
				var jsonMap map[string]interface{}
				if err := json.Unmarshal([]byte(strVal), &jsonMap); err == nil {
					merged[k] = jsonMap
				} else {
					merged[k] = xrdVal
				}
			} else {
				merged[k] = xrdVal
			}
		} else {
			// If both are maps, recursively merge them
			if xrdMap, isXrdMap := xrdVal.(map[string]interface{}); isXrdMap {
				if mergedMap, isMergedMap := mergedVal.(map[string]interface{}); isMergedMap {
					merged[k] = MergeXRDDefaultsIntoXR(mergedMap, xrdMap)
				}
			}
			// If not maps, keep the existing value from xr
		}
	}

	return cleanEmptyValues(merged)
}

// cleanEmptyValues removes empty values from the map.
// They are introduced when object has multiple properties and
// one of them is non-empty and the other is empty.
func cleanEmptyValues(m map[string]interface{}) map[string]interface{} {
	cleaned := make(map[string]interface{})

	for k, v := range m {
		switch v := v.(type) {
		case map[string]interface{}:
			// Recursively clean nested maps
			if nestedMap := cleanEmptyValues(v); len(nestedMap) > 0 {
				cleaned[k] = nestedMap
			}
		case []interface{}:
			// Clean array values
			if len(v) > 0 {
				cleaned[k] = v
			}
		case string:
			// Remove empty strings and "{}" strings
			if v != "" && v != "{}" {
				cleaned[k] = v
			}
		default:
			// Keep non-nil values
			if v != nil {
				cleaned[k] = v
			}
		}
	}
	return cleaned
}
