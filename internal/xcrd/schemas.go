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

package xcrd

import (
	"fmt"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// Label keys.
const (
	LabelKeyNamePrefixForComposed = "crossplane.io/composite"
	LabelKeyClaimName             = "crossplane.io/claim-name"
	LabelKeyClaimNamespace        = "crossplane.io/claim-namespace"
)

// CompositionRevisionRef should be propagated dynamically.
const CompositionRevisionRef = "compositionRevisionRef"

// PropagateSpecProps is the list of XRC spec properties to propagate
// when translating an XRC into an XR.
var PropagateSpecProps = []string{"compositionRef", "compositionSelector", "compositionUpdatePolicy", "compositionRevisionSelector"} //nolint:gochecknoglobals // We treat this as a constant.

// TODO(negz): Add descriptions to schema fields.

// BaseProps is a partial OpenAPIV3Schema for the spec fields that Crossplane
// expects to be present for all CRDs that it creates.
func BaseProps() *extv1.JSONSchemaProps {
	return &extv1.JSONSchemaProps{
		Type:     "object",
		Required: []string{"spec"},
		Properties: map[string]extv1.JSONSchemaProps{
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
				Properties: map[string]extv1.JSONSchemaProps{},
			},
			"status": {
				Type:       "object",
				Properties: map[string]extv1.JSONSchemaProps{},
			},
		},
	}
}

// CompositeResourceSpecProps is a partial OpenAPIV3Schema for the spec fields
// that Crossplane expects to be present for all defined composite resources.
func CompositeResourceSpecProps(defaultPol *v1.UpdatePolicy) map[string]extv1.JSONSchemaProps {
	return map[string]extv1.JSONSchemaProps{
		"crossplane": {
			Type:        "object",
			Description: "Configures how Crossplane will reconcile this composite resource",
			Properties:  LegacyCompositeResourceSpecProps(defaultPol),
		},
	}
}

// LegacyCompositeResourceSpecProps is a partial OpenAPIV3Schema for the spec
// fields that Crossplane expects to be present for all defined composite
// resources. It's used for Crossplane v1 style ClusterLegacy XRs.
func LegacyCompositeResourceSpecProps(defaultPol *v1.UpdatePolicy) map[string]extv1.JSONSchemaProps {
	return map[string]extv1.JSONSchemaProps{
		"compositionRef": {
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]extv1.JSONSchemaProps{
				"name": {Type: "string"},
			},
		},
		"compositionSelector": {
			Type:     "object",
			Required: []string{"matchLabels"},
			Properties: map[string]extv1.JSONSchemaProps{
				"matchLabels": {
					Type: "object",
					AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
						Allows: true,
						Schema: &extv1.JSONSchemaProps{Type: "string"},
					},
				},
			},
		},
		"compositionRevisionRef": {
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]extv1.JSONSchemaProps{
				"name": {Type: "string"},
			},
		},
		"compositionRevisionSelector": {
			Type:     "object",
			Required: []string{"matchLabels"},
			Properties: map[string]extv1.JSONSchemaProps{
				"matchLabels": {
					Type: "object",
					AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
						Allows: true,
						Schema: &extv1.JSONSchemaProps{Type: "string"},
					},
				},
			},
		},
		"compositionUpdatePolicy": {
			Type: "string",
			Enum: []extv1.JSON{
				{Raw: []byte(`"Automatic"`)},
				{Raw: []byte(`"Manual"`)},
			},
			Default: func() *extv1.JSON {
				if defaultPol == nil {
					return nil
				}
				return &extv1.JSON{Raw: []byte(fmt.Sprintf("\"%s\"", *defaultPol))}
			}(),
		},
		"claimRef": {
			Type:     "object",
			Required: []string{"apiVersion", "kind", "namespace", "name"},
			Properties: map[string]extv1.JSONSchemaProps{
				"apiVersion": {Type: "string"},
				"kind":       {Type: "string"},
				"namespace":  {Type: "string"},
				"name":       {Type: "string"},
			},
		},
		"resourceRefs": {
			Type: "array",
			Items: &extv1.JSONSchemaPropsOrArray{
				Schema: &extv1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]extv1.JSONSchemaProps{
						"apiVersion": {Type: "string"},
						"name":       {Type: "string"},
						"kind":       {Type: "string"},
					},
					Required: []string{"apiVersion", "kind"},
				},
			},
			// Controllers should replace the entire resourceRefs array.
			XListType: ptr.To("atomic"),
		},
		"writeConnectionSecretToRef": {
			Type:     "object",
			Required: []string{"name", "namespace"},
			Properties: map[string]extv1.JSONSchemaProps{
				"name":      {Type: "string"},
				"namespace": {Type: "string"},
			},
		},
	}
}

// CompositeResourceClaimSpecProps is a partial OpenAPIV3Schema for the spec
// fields that Crossplane expects to be present for all published infrastructure
// resources.
func CompositeResourceClaimSpecProps(defaultPol *v1.CompositeDeletePolicy) map[string]extv1.JSONSchemaProps {
	return map[string]extv1.JSONSchemaProps{
		"compositionRef": {
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]extv1.JSONSchemaProps{
				"name": {Type: "string"},
			},
		},
		"compositionSelector": {
			Type:     "object",
			Required: []string{"matchLabels"},
			Properties: map[string]extv1.JSONSchemaProps{
				"matchLabels": {
					Type: "object",
					AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
						Allows: true,
						Schema: &extv1.JSONSchemaProps{Type: "string"},
					},
				},
			},
		},
		"compositionRevisionRef": {
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]extv1.JSONSchemaProps{
				"name": {Type: "string"},
			},
		},
		"compositionRevisionSelector": {
			Type:     "object",
			Required: []string{"matchLabels"},
			Properties: map[string]extv1.JSONSchemaProps{
				"matchLabels": {
					Type: "object",
					AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
						Allows: true,
						Schema: &extv1.JSONSchemaProps{Type: "string"},
					},
				},
			},
		},
		"compositionUpdatePolicy": {
			Type: "string",
			Enum: []extv1.JSON{
				{Raw: []byte(`"Automatic"`)},
				{Raw: []byte(`"Manual"`)},
			},
		},
		"compositeDeletePolicy": {
			Type: "string",
			Enum: []extv1.JSON{
				{Raw: []byte(`"Background"`)},
				{Raw: []byte(`"Foreground"`)},
			},
			Default: func() *extv1.JSON {
				if defaultPol == nil {
					return nil
				}
				return &extv1.JSON{Raw: []byte(fmt.Sprintf("\"%s\"", *defaultPol))}
			}(),
		},
		"resourceRef": {
			Type:     "object",
			Required: []string{"apiVersion", "kind", "name"},
			Properties: map[string]extv1.JSONSchemaProps{
				"apiVersion": {Type: "string"},
				"kind":       {Type: "string"},
				"name":       {Type: "string"},
			},
		},
		"writeConnectionSecretToRef": {
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]extv1.JSONSchemaProps{
				"name": {Type: "string"},
			},
		},
	}
}

// CompositeResourceStatusProps is a partial OpenAPIV3Schema for the status
// fields that Crossplane expects to be present for all composite resources.
func CompositeResourceStatusProps() map[string]extv1.JSONSchemaProps {
	return map[string]extv1.JSONSchemaProps{
		"conditions": {
			Description: "Conditions of the resource.",
			Type:        "array",
			XListMapKeys: []string{
				"type",
			},
			XListType: ptr.To("map"),
			Items: &extv1.JSONSchemaPropsOrArray{
				Schema: &extv1.JSONSchemaProps{
					Type:     "object",
					Required: []string{"lastTransitionTime", "reason", "status", "type"},
					Properties: map[string]extv1.JSONSchemaProps{
						"lastTransitionTime": {Type: "string", Format: "date-time"},
						"message":            {Type: "string"},
						"reason":             {Type: "string"},
						"status":             {Type: "string"},
						"type":               {Type: "string"},
					},
				},
			},
		},
		"crossplane": {
			Type:        "object",
			Description: "Indicates how Crossplane is reconciling this composite resource",
			Properties: map[string]extv1.JSONSchemaProps{
				"connectionDetails": {
					Type: "object",
					Properties: map[string]extv1.JSONSchemaProps{
						"lastPublishedTime": {Type: "string", Format: "date-time"},
					},
				},
			},
		},
	}
}

// LegacyCompositeResourceStatusProps is a partial OpenAPIV3Schema for the
// status fields that Crossplane expects to be present for all composite
// resources and claims. It's used for v1 style ClusterLegacy XRs and claims.
func LegacyCompositeResourceStatusProps() map[string]extv1.JSONSchemaProps {
	return map[string]extv1.JSONSchemaProps{
		"conditions": {
			Description: "Conditions of the resource.",
			Type:        "array",
			XListMapKeys: []string{
				"type",
			},
			XListType: ptr.To("map"),
			Items: &extv1.JSONSchemaPropsOrArray{
				Schema: &extv1.JSONSchemaProps{
					Type:     "object",
					Required: []string{"lastTransitionTime", "reason", "status", "type"},
					Properties: map[string]extv1.JSONSchemaProps{
						"lastTransitionTime": {Type: "string", Format: "date-time"},
						"message":            {Type: "string"},
						"reason":             {Type: "string"},
						"status":             {Type: "string"},
						"type":               {Type: "string"},
					},
				},
			},
		},
		"connectionDetails": {
			Type: "object",
			Properties: map[string]extv1.JSONSchemaProps{
				"lastPublishedTime": {Type: "string", Format: "date-time"},
			},
		},
		"claimConditionTypes": {
			Type:      "array",
			XListType: ptr.To("set"),
			Items: &extv1.JSONSchemaPropsOrArray{
				Schema: &extv1.JSONSchemaProps{
					Type: "string",
				},
			},
		},
	}
}

// CompositeResourcePrinterColumns returns the set of default printer columns
// that should exist in all generated composite resource CRDs.
func CompositeResourcePrinterColumns() []extv1.CustomResourceColumnDefinition {
	return []extv1.CustomResourceColumnDefinition{
		{
			Name:     "SYNCED",
			Type:     "string",
			JSONPath: ".status.conditions[?(@.type=='Synced')].status",
		},
		{
			Name:     "READY",
			Type:     "string",
			JSONPath: ".status.conditions[?(@.type=='Ready')].status",
		},
		{
			Name:     "COMPOSITION",
			Type:     "string",
			JSONPath: ".spec.compositionRef.name",
		},
		{
			Name:     "COMPOSITIONREVISION",
			Type:     "string",
			JSONPath: ".spec.compositionRevisionRef.name",
			Priority: 1,
		},
		{
			Name:     "AGE",
			Type:     "date",
			JSONPath: ".metadata.creationTimestamp",
		},
	}
}

// CompositeResourceClaimPrinterColumns returns the set of default printer
// columns that should exist in all generated composite resource claim CRDs.
func CompositeResourceClaimPrinterColumns() []extv1.CustomResourceColumnDefinition {
	return []extv1.CustomResourceColumnDefinition{
		{
			Name:     "SYNCED",
			Type:     "string",
			JSONPath: ".status.conditions[?(@.type=='Synced')].status",
		},
		{
			Name:     "READY",
			Type:     "string",
			JSONPath: ".status.conditions[?(@.type=='Ready')].status",
		},
		{
			Name:     "CONNECTION-SECRET",
			Type:     "string",
			JSONPath: ".spec.writeConnectionSecretToRef.name",
		},
		{
			Name:     "AGE",
			Type:     "date",
			JSONPath: ".metadata.creationTimestamp",
		},
	}
}

// GetPropFields returns the fields from a map of schema properties.
func GetPropFields(props map[string]extv1.JSONSchemaProps) []string {
	propFields := make([]string, len(props))
	i := 0
	for k := range props {
		propFields[i] = k
		i++
	}
	return propFields
}
