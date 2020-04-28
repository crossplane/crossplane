/*
Copyright 2020 The Crossplane Authors.

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

package ccrd

import "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

// TODO(negz): Add descriptions to schema fields.

// BaseProps is a partial OpenAPIV3Schema for the spec fields that Crossplane
// expects to be present for all CRDs that it creates.
func BaseProps() map[string]v1beta1.JSONSchemaProps {
	return map[string]v1beta1.JSONSchemaProps{
		"apiVersion": {
			Type: "string",
		},
		"kind": {
			Type: "string",
		},
		"metadata": {
			// NOTE(muvaf): api-server takes care of validating
			// metadata.
			Type: "object",
		},
		"spec": {
			Type:       "object",
			Properties: map[string]v1beta1.JSONSchemaProps{},
		},
		"status": {
			Type:       "object",
			Properties: map[string]v1beta1.JSONSchemaProps{},
		},
	}
}

// DefinedInfrastructureSpecProps is a partial OpenAPIV3Schema for the spec
// fields that Crossplane expects to be present for all defined infrastructure
// resources.
func DefinedInfrastructureSpecProps() map[string]v1beta1.JSONSchemaProps {
	return map[string]v1beta1.JSONSchemaProps{
		"compositionRef": {
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]v1beta1.JSONSchemaProps{
				"name": {Type: "string"},
			},
		},
		"compositionSelector": {
			Type:     "object",
			Required: []string{"matchLabels"},
			Properties: map[string]v1beta1.JSONSchemaProps{
				"matchLabels": {
					Type: "object",
					AdditionalProperties: &v1beta1.JSONSchemaPropsOrBool{
						Allows: true,
						Schema: &v1beta1.JSONSchemaProps{Type: "string"},
					},
				},
			},
		},
		"requirementRef": {
			Type:     "object",
			Required: []string{"name", "namespace"},
			Properties: map[string]v1beta1.JSONSchemaProps{
				"name":      {Type: "string"},
				"namespace": {Type: "string"},
			},
		},
		"resourceRefs": {
			Type: "array",
			Items: &v1beta1.JSONSchemaPropsOrArray{
				Schema: &v1beta1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]v1beta1.JSONSchemaProps{
						"apiVersion": {Type: "string"},
						"name":       {Type: "string"},
						"kind":       {Type: "string"},
						"uid":        {Type: "string"},
					},
					Required: []string{"apiVersion", "kind", "name"},
				},
			},
		},
		"writeConnectionSecretToRef": {
			Type:     "object",
			Required: []string{"name", "namespace"},
			Properties: map[string]v1beta1.JSONSchemaProps{
				"name":      {Type: "string"},
				"namespace": {Type: "string"},
			},
		},
		"reclaimPolicy": {
			Type: "string",
			Enum: []v1beta1.JSON{
				{Raw: []byte(`"Retain"`)},
				{Raw: []byte(`"Delete"`)},
			},
		},
	}
}

// PublishedInfrastructureSpecProps is a partial OpenAPIV3Schema for the spec
// fields that Crossplane expects to be present for all published infrastructure
// resources.
func PublishedInfrastructureSpecProps() map[string]v1beta1.JSONSchemaProps {
	return map[string]v1beta1.JSONSchemaProps{
		"compositionRef": {
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]v1beta1.JSONSchemaProps{
				"name": {Type: "string"},
			},
		},
		"compositionSelector": {
			Type:     "object",
			Required: []string{"matchLabels"},
			Properties: map[string]v1beta1.JSONSchemaProps{
				"matchLabels": {
					Type: "object",
					AdditionalProperties: &v1beta1.JSONSchemaPropsOrBool{
						Allows: true,
						Schema: &v1beta1.JSONSchemaProps{Type: "string"},
					},
				},
			},
		},
		"resourceRef": {
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]v1beta1.JSONSchemaProps{
				"name": {Type: "string"},
			},
		},
		"writeConnectionSecretToRef": {
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]v1beta1.JSONSchemaProps{
				"name": {Type: "string"},
			},
		},
	}
}

// InfrastructureStatusProps is a partial OpenAPIV3Schema for the status fields
// that Crossplane expects to be present for all defined or published
// infrastructure resources.
func InfrastructureStatusProps() map[string]v1beta1.JSONSchemaProps {
	return map[string]v1beta1.JSONSchemaProps{
		"composedResources": {
			Type: "integer",
		},
		"readyResources": {
			Type: "integer",
		},
		"bindingPhase": {
			Type: "string",
			Enum: []v1beta1.JSON{
				{Raw: []byte(`"Unbindable"`)},
				{Raw: []byte(`"Unbound"`)},
				{Raw: []byte(`"Bound"`)},
				{Raw: []byte(`"Released"`)},
			},
		},
		"conditions": {
			Description: "Conditions of the resource.",
			Type:        "array",
			Items: &v1beta1.JSONSchemaPropsOrArray{
				Schema: &v1beta1.JSONSchemaProps{
					Type:     "object",
					Required: []string{"lastTransitionTime", "reason", "status", "type"},
					Properties: map[string]v1beta1.JSONSchemaProps{
						"lastTransitionTime": {Type: "string", Format: "date-time"},
						"message":            {Type: "string"},
						"reason":             {Type: "string"},
						"status":             {Type: "string"},
						"type":               {Type: "string"},
					},
				},
			},
		},
	}
}
