/*
Copyright 2023 The Crossplane Authors.

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
