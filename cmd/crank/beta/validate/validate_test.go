/*
Copyright 2024 The Crossplane Authors.

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

package validate

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	testCRD = &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: "test.org",
			Names: extv1.CustomResourceDefinitionNames{
				Kind:     "Test",
				ListKind: "TestList",
				Plural:   "tests",
				Singular: "test",
			},
			Scope: "Cluster",
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: true,
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"spec": {
									Type: "object",
									Properties: map[string]extv1.JSONSchemaProps{
										"replicas": {
											Type: "integer",
										},
									},
									Required: []string{
										"replicas",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	testCRDWithCEL = &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: "test.org",
			Names: extv1.CustomResourceDefinitionNames{
				Kind:     "Test",
				ListKind: "TestList",
				Plural:   "tests",
				Singular: "test",
			},
			Scope: "Cluster",
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: true,
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"spec": {
									Type: "object",
									XValidations: extv1.ValidationRules{
										extv1.ValidationRule{
											Rule:    "self.minReplicas <= self.replicas && self.replicas <= self.maxReplicas",
											Message: "replicas should be in between minReplicas and maxReplicas",
										},
									},
									Properties: map[string]extv1.JSONSchemaProps{
										"replicas": {
											Type: "integer",
										},
										"minReplicas": {
											Type: "integer",
										},
										"maxReplicas": {
											Type: "integer",
										},
									},
									Required: []string{
										"replicas",
										"minReplicas",
										"maxReplicas",
									},
								},
							},
						},
					},
				},
			},
		},
	}
)

func TestConvertToCRDs(t *testing.T) {
	type args struct {
		schemas []*unstructured.Unstructured
	}

	type want struct {
		crd []*extv1.CustomResourceDefinition
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"UnstructuredCRD": {
			reason: "Should convert an unstructured CRD to a CustomResourceDefinition",
			args: args{
				schemas: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.k8s.io/v1",
							"kind":       "CustomResourceDefinition",
							"metadata": map[string]interface{}{
								"name": "test",
							},
							"spec": map[string]interface{}{
								"group": "test.org",
								"names": map[string]interface{}{
									"kind":     "Test",
									"listKind": "TestList",
									"plural":   "tests",
									"singular": "test",
								},
								"scope": "Cluster",
								"versions": []interface{}{
									map[string]interface{}{
										"name":    "v1alpha1",
										"served":  true,
										"storage": true,
										"schema": map[string]interface{}{
											"openAPIV3Schema": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"spec": map[string]interface{}{
														"type": "object",
														"properties": map[string]interface{}{
															"replicas": map[string]interface{}{
																"type": "integer",
															},
														},
														"required": []interface{}{
															"replicas",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				crd: []*extv1.CustomResourceDefinition{
					testCRD,
				},
			},
		},
		"UnstructuredXRD": {
			reason: "Should convert an unstructured XRD to a CustomResourceDefinition",
			args: args{
				schemas: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.crossplane.io/v1alpha1",
							"kind":       "CompositeResourceDefinition",
							"metadata": map[string]interface{}{
								"name": "test",
							},
							"spec": map[string]interface{}{
								"group": "test.org",
								"names": map[string]interface{}{
									"kind":     "Test",
									"listKind": "TestList",
									"plural":   "tests",
									"singular": "test",
								},
								"versions": []interface{}{
									map[string]interface{}{
										"name":    "v1alpha1",
										"served":  true,
										"storage": true,
										"schema": map[string]interface{}{
											"openAPIV3Schema": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"spec": map[string]interface{}{
														"type": "object",
														"properties": map[string]interface{}{
															"replicas": map[string]interface{}{
																"type": "integer",
															},
														},
													},
													"status": map[string]interface{}{
														"type": "object",
														"properties": map[string]interface{}{
															"replicas": map[string]interface{}{
																"type": "integer",
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				crd: []*extv1.CustomResourceDefinition{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion:         "apiextensions.crossplane.io/v2",
									Kind:               "CompositeResourceDefinition",
									Name:               "test",
									Controller:         ptr.To[bool](true),
									BlockOwnerDeletion: ptr.To[bool](true),
								},
							},
						},
						Spec: extv1.CustomResourceDefinitionSpec{
							Group: "test.org",
							Names: extv1.CustomResourceDefinitionNames{
								Kind:     "Test",
								ListKind: "TestList",
								Plural:   "tests",
								Singular: "test",
								Categories: []string{
									"composite",
								},
							},
							Scope: "Cluster",
							Versions: []extv1.CustomResourceDefinitionVersion{
								{
									Name:    "v1alpha1",
									Served:  true,
									Storage: false,
									Subresources: &extv1.CustomResourceSubresources{
										Status: &extv1.CustomResourceSubresourceStatus{},
									},
									AdditionalPrinterColumns: []extv1.CustomResourceColumnDefinition{
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
									},

									Schema: &extv1.CustomResourceValidation{
										OpenAPIV3Schema: &extv1.JSONSchemaProps{
											Type: "object",
											Required: []string{
												"spec",
											},
											Properties: map[string]extv1.JSONSchemaProps{
												"apiVersion": {
													Type: "string",
												},
												"kind": {
													Type: "string",
												},
												"metadata": {
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"name": {
															Type:      "string",
															MaxLength: ptr.To[int64](63),
														},
													},
												},
												"spec": {
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"replicas": {
															Type: "integer",
														},
														"claimRef": {
															Type: "object",
															Required: []string{
																"apiVersion", "kind", "namespace", "name",
															},
															Properties: map[string]extv1.JSONSchemaProps{
																"apiVersion": {
																	Type: "string",
																},
																"kind": {
																	Type: "string",
																},
																"name": {
																	Type: "string",
																},
																"namespace": {
																	Type: "string",
																},
															},
														},
														"compositionRef": {
															Type: "object",
															Required: []string{
																"name",
															},
															Properties: map[string]extv1.JSONSchemaProps{
																"name": {
																	Type: "string",
																},
															},
														},
														"compositionRevisionRef": {
															Type: "object",
															Required: []string{
																"name",
															},
															Properties: map[string]extv1.JSONSchemaProps{
																"name": {
																	Type: "string",
																},
															},
														},
														"compositionRevisionSelector": {
															Type: "object",
															Required: []string{
																"matchLabels",
															},
															Properties: map[string]extv1.JSONSchemaProps{
																"matchLabels": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{
																			Type: "string",
																		},
																	},
																},
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
														"compositionUpdatePolicy": {
															Type: "string",
															Enum: []extv1.JSON{
																{Raw: []byte(`"Automatic"`)},
																{Raw: []byte(`"Manual"`)},
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
																		"namespace":  {Type: "string"},
																		"kind":       {Type: "string"},
																	},
																	Required: []string{"apiVersion", "kind"},
																},
															},
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
													},
												},
												"status": {
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"replicas": {
															Type: "integer",
														},
														"conditions": {
															Description:  "Conditions of the resource.",
															Type:         "array",
															XListType:    ptr.To("map"),
															XListMapKeys: []string{"type"},
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
																		"observedGeneration": {Type: "integer", Format: "int64"},
																	},
																},
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
														"connectionDetails": {
															Type: "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"lastPublishedTime": {Type: "string", Format: "date-time"},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"UnstructuredXRDWithClaim": {
			reason: "Should convert an unstructured XRD to a CustomResourceDefinition",
			args: args{
				schemas: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.crossplane.io/v1alpha1",
							"kind":       "CompositeResourceDefinition",
							"metadata": map[string]interface{}{
								"name": "test",
							},
							"spec": map[string]interface{}{
								"claimNames": map[string]interface{}{
									"kind":   "TestClaim",
									"plural": "testclaims",
								},
								"group": "test.org",
								"names": map[string]interface{}{
									"kind":     "Test",
									"listKind": "TestList",
									"plural":   "tests",
									"singular": "test",
								},
								"versions": []interface{}{
									map[string]interface{}{
										"name":    "v1alpha1",
										"served":  true,
										"storage": true,
										"schema": map[string]interface{}{
											"openAPIV3Schema": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"spec": map[string]interface{}{
														"type": "object",
														"properties": map[string]interface{}{
															"replicas": map[string]interface{}{
																"type": "integer",
															},
														},
													},
													"status": map[string]interface{}{
														"type": "object",
														"properties": map[string]interface{}{
															"replicas": map[string]interface{}{
																"type": "integer",
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				crd: []*extv1.CustomResourceDefinition{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion:         "apiextensions.crossplane.io/v2",
									Kind:               "CompositeResourceDefinition",
									Name:               "test",
									Controller:         ptr.To[bool](true),
									BlockOwnerDeletion: ptr.To[bool](true),
								},
							},
						},
						Spec: extv1.CustomResourceDefinitionSpec{
							Group: "test.org",
							Names: extv1.CustomResourceDefinitionNames{
								Kind:     "Test",
								ListKind: "TestList",
								Plural:   "tests",
								Singular: "test",
								Categories: []string{
									"composite",
								},
							},
							Scope: "Cluster",
							Versions: []extv1.CustomResourceDefinitionVersion{
								{
									Name:    "v1alpha1",
									Served:  true,
									Storage: false,
									Subresources: &extv1.CustomResourceSubresources{
										Status: &extv1.CustomResourceSubresourceStatus{},
									},
									AdditionalPrinterColumns: []extv1.CustomResourceColumnDefinition{
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
									},

									Schema: &extv1.CustomResourceValidation{
										OpenAPIV3Schema: &extv1.JSONSchemaProps{
											Type: "object",
											Required: []string{
												"spec",
											},
											Properties: map[string]extv1.JSONSchemaProps{
												"apiVersion": {
													Type: "string",
												},
												"kind": {
													Type: "string",
												},
												"metadata": {
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"name": {
															Type:      "string",
															MaxLength: ptr.To[int64](63),
														},
													},
												},
												"spec": {
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"replicas": {
															Type: "integer",
														},
														"claimRef": {
															Type: "object",
															Required: []string{
																"apiVersion", "kind", "namespace", "name",
															},
															Properties: map[string]extv1.JSONSchemaProps{
																"apiVersion": {
																	Type: "string",
																},
																"kind": {
																	Type: "string",
																},
																"name": {
																	Type: "string",
																},
																"namespace": {
																	Type: "string",
																},
															},
														},
														"compositionRef": {
															Type: "object",
															Required: []string{
																"name",
															},
															Properties: map[string]extv1.JSONSchemaProps{
																"name": {
																	Type: "string",
																},
															},
														},
														"compositionRevisionRef": {
															Type: "object",
															Required: []string{
																"name",
															},
															Properties: map[string]extv1.JSONSchemaProps{
																"name": {
																	Type: "string",
																},
															},
														},
														"compositionRevisionSelector": {
															Type: "object",
															Required: []string{
																"matchLabels",
															},
															Properties: map[string]extv1.JSONSchemaProps{
																"matchLabels": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{
																			Type: "string",
																		},
																	},
																},
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
														"compositionUpdatePolicy": {
															Type: "string",
															Enum: []extv1.JSON{
																{Raw: []byte(`"Automatic"`)},
																{Raw: []byte(`"Manual"`)},
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
																		"namespace":  {Type: "string"},
																		"kind":       {Type: "string"},
																	},
																	Required: []string{"apiVersion", "kind"},
																},
															},
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
													},
												},
												"status": {
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"replicas": {
															Type: "integer",
														},
														"conditions": {
															Description:  "Conditions of the resource.",
															Type:         "array",
															XListType:    ptr.To("map"),
															XListMapKeys: []string{"type"},
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
																		"observedGeneration": {Type: "integer", Format: "int64"},
																	},
																},
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
														"connectionDetails": {
															Type: "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"lastPublishedTime": {Type: "string", Format: "date-time"},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "testclaims.test.org",
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion:         "apiextensions.crossplane.io/v2",
									Kind:               "CompositeResourceDefinition",
									Name:               "test",
									Controller:         ptr.To[bool](true),
									BlockOwnerDeletion: ptr.To[bool](true),
								},
							},
						},
						Spec: extv1.CustomResourceDefinitionSpec{
							Group: "test.org",
							Names: extv1.CustomResourceDefinitionNames{
								Kind:   "TestClaim",
								Plural: "testclaims",
								Categories: []string{
									"claim",
								},
							},
							Scope: "Namespaced",
							Versions: []extv1.CustomResourceDefinitionVersion{
								{
									Name:    "v1alpha1",
									Served:  true,
									Storage: false,
									Subresources: &extv1.CustomResourceSubresources{
										Status: &extv1.CustomResourceSubresourceStatus{},
									},
									AdditionalPrinterColumns: []extv1.CustomResourceColumnDefinition{
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
									},

									Schema: &extv1.CustomResourceValidation{
										OpenAPIV3Schema: &extv1.JSONSchemaProps{
											Type: "object",
											Required: []string{
												"spec",
											},
											Properties: map[string]extv1.JSONSchemaProps{
												"apiVersion": {
													Type: "string",
												},
												"kind": {
													Type: "string",
												},
												"metadata": {
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"name": {
															Type:      "string",
															MaxLength: ptr.To[int64](63),
														},
													},
												},
												"spec": {
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"replicas": {
															Type: "integer",
														},
														"compositeDeletePolicy": {
															Type: "string",
															Enum: []extv1.JSON{
																{Raw: []byte(`"Background"`)},
																{Raw: []byte(`"Foreground"`)},
															},
														},
														"compositionRef": {
															Type: "object",
															Required: []string{
																"name",
															},
															Properties: map[string]extv1.JSONSchemaProps{
																"name": {
																	Type: "string",
																},
															},
														},
														"compositionRevisionRef": {
															Type: "object",
															Required: []string{
																"name",
															},
															Properties: map[string]extv1.JSONSchemaProps{
																"name": {
																	Type: "string",
																},
															},
														},
														"compositionRevisionSelector": {
															Type: "object",
															Required: []string{
																"matchLabels",
															},
															Properties: map[string]extv1.JSONSchemaProps{
																"matchLabels": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{
																			Type: "string",
																		},
																	},
																},
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
														"compositionUpdatePolicy": {
															Type: "string",
															Enum: []extv1.JSON{
																{Raw: []byte(`"Automatic"`)},
																{Raw: []byte(`"Manual"`)},
															},
														},
														"resourceRef": {
															Type:     "object",
															Required: []string{"apiVersion", "kind", "name"},
															Properties: map[string]extv1.JSONSchemaProps{
																"apiVersion": {Type: "string"},
																"name":       {Type: "string"},
																"kind":       {Type: "string"},
															},
														},
														"writeConnectionSecretToRef": {
															Type:     "object",
															Required: []string{"name"},
															Properties: map[string]extv1.JSONSchemaProps{
																"name": {Type: "string"},
															},
														},
													},
												},
												"status": {
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"replicas": {
															Type: "integer",
														},
														"conditions": {
															Description:  "Conditions of the resource.",
															Type:         "array",
															XListType:    ptr.To("map"),
															XListMapKeys: []string{"type"},
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
																		"observedGeneration": {Type: "integer", Format: "int64"},
																	},
																},
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
														"connectionDetails": {
															Type: "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"lastPublishedTime": {Type: "string", Format: "date-time"},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"WrongKind": {
			reason: "Should skip an unstructured object that is not a CRD or XRD",
			args: args{
				schemas: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.k8s.io/v1",
							"kind":       "WrongKind",
							"metadata": map[string]interface{}{
								"name": "test",
							},
						},
					},
				},
			},
			want: want{
				crd: []*extv1.CustomResourceDefinition{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := &bytes.Buffer{}
			m := NewManager("", nil, w)
			err := m.PrepExtensions(tc.args.schemas)

			if diff := cmp.Diff(tc.want.crd, m.crds); diff != "" {
				t.Errorf("%s\nconvertToCRDs(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nconvertToCRDs(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestValidateResources(t *testing.T) {
	type args struct {
		resources             []*unstructured.Unstructured
		crds                  []*extv1.CustomResourceDefinition
		errorOnMissingSchemas bool
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Valid": {
			reason: "Should not return an error if the resources are valid",
			args: args{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1alpha1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "test",
							},
							"spec": map[string]interface{}{
								"replicas": 1,
							},
						},
					},
				},
				crds: []*extv1.CustomResourceDefinition{
					testCRD,
				},
			},
		},
		"ValidWithCEL": {
			reason: "Should not return an error if the resources are valid",
			args: args{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1alpha1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "test",
							},
							"spec": map[string]interface{}{
								"replicas":    5,
								"minReplicas": 3,
								"maxReplicas": 10,
							},
						},
					},
				},
				crds: []*extv1.CustomResourceDefinition{
					testCRDWithCEL,
				},
			},
		},
		"ValidWithMissingSchemasEnabled": {
			reason: "Should not return an error if the resources are valid and schemas are not missing",
			args: args{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1alpha1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "test",
							},
							"spec": map[string]interface{}{
								"replicas": 1,
							},
						},
					},
				},
				crds: []*extv1.CustomResourceDefinition{
					testCRD,
				},
				errorOnMissingSchemas: true,
			},
		},
		"ErrorOnMissingSchemas": {
			reason: "Should return an error if schemas are missing",
			args: args{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1alpha1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "test",
							},
							"spec": map[string]interface{}{
								"replicas": 1,
							},
						},
					},
				},
				crds:                  []*extv1.CustomResourceDefinition{},
				errorOnMissingSchemas: true,
			},
			want: want{
				err: errors.New("could not validate all resources, schema(s) missing"),
			},
		},
		"Invalid": {
			reason: "Should return an error if the resources are invalid",
			args: args{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1alpha1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "test",
							},
							"spec": map[string]interface{}{
								"replicas": "non-integer",
							},
						},
					},
				},
				crds: []*extv1.CustomResourceDefinition{
					testCRD,
				},
			},
			want: want{
				err: errors.New("could not validate all resources"),
			},
		},
		"InvalidWithCEL": {
			reason: "Should not return an error if the resources are valid",
			args: args{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1alpha1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "test",
							},
							"spec": map[string]interface{}{
								"replicas":    50,
								"minReplicas": 3,
								"maxReplicas": 10,
							},
						},
					},
				},
				crds: []*extv1.CustomResourceDefinition{
					testCRDWithCEL,
				},
			},
			want: want{
				err: errors.New("could not validate all resources"),
			},
		},
		"MissingCRD": {
			reason: "Should not return an error if the CRD/XRD is missing",
			args: args{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1alpha1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "test",
							},
							"spec": map[string]interface{}{
								"replicas": 1,
							},
						},
					},
				},
				crds: []*extv1.CustomResourceDefinition{},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := &bytes.Buffer{}

			got := SchemaValidation(tc.args.resources, tc.args.crds, tc.args.errorOnMissingSchemas, false, w)
			if diff := cmp.Diff(tc.want.err, got, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nvalidateResources(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestValidateUnknownFields(t *testing.T) {
	type args struct {
		mr  map[string]interface{}
		sch *schema.Structural
	}

	type want struct {
		errs field.ErrorList
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"UnknownFieldPresent": {
			reason: "Should detect unknown fields in the resource and return an error",
			args: args{
				mr: map[string]interface{}{
					"apiVersion": "test.org/v1alpha1",
					"kind":       "Test",
					"metadata": map[string]interface{}{
						"name": "test-instance",
					},
					"spec": map[string]interface{}{
						"replicas":     3,
						"unknownField": "should fail", // This field is not defined in the CRD schema
					},
				},
				sch: &schema.Structural{
					Properties: map[string]schema.Structural{
						"spec": {
							Properties: map[string]schema.Structural{
								"replicas": {
									Generic: schema.Generic{Type: "integer"},
								},
							},
						},
					},
				},
			},
			want: want{
				errs: field.ErrorList{
					field.Invalid(field.NewPath("spec.unknownField"), "unknownField", `unknown field: "unknownField"`),
				},
			},
		},
		"UnknownFieldNotPresent": {
			reason: "Should not return an error when no unknown fields are present",
			args: args{
				mr: map[string]interface{}{
					"apiVersion": "test.org/v1alpha1",
					"kind":       "Test",
					"metadata": map[string]interface{}{
						"name": "test-instance",
					},
					"spec": map[string]interface{}{
						"replicas": 3, // No unknown fields
					},
				},
				sch: &schema.Structural{
					Properties: map[string]schema.Structural{
						"spec": {
							Properties: map[string]schema.Structural{
								"replicas": {
									Generic: schema.Generic{Type: "integer"},
								},
							},
						},
					},
				},
			},
			want: want{
				errs: field.ErrorList{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			errs := validateUnknownFields(tc.args.mr, tc.args.sch)
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nvalidateUnknownFields(...): -want errs, +got errs:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	type args struct {
		resource *unstructured.Unstructured
		gvk      runtimeschema.GroupVersionKind
		crds     []*extv1.CustomResourceDefinition
	}

	type want struct {
		resource *unstructured.Unstructured
		err      error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoCRDFound": {
			reason: "Should return nil when no matching CRD is found (skip defaulting)",
			args: args{
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1alpha1",
						"kind":       "Test",
						"spec": map[string]interface{}{
							"replicas": 3,
						},
					},
				},
				gvk: runtimeschema.GroupVersionKind{
					Group:   "test.org",
					Version: "v1alpha1",
					Kind:    "Test",
				},
				crds: []*extv1.CustomResourceDefinition{},
			},
			want: want{
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1alpha1",
						"kind":       "Test",
						"spec": map[string]interface{}{
							"replicas": 3,
						},
					},
				},
				err: nil,
			},
		},
		"ApplySimpleDefault": {
			reason: "Should apply default value to missing property",
			args: args{
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1alpha1",
						"kind":       "Test",
						"spec": map[string]interface{}{
							"replicas": 3,
						},
					},
				},
				gvk: runtimeschema.GroupVersionKind{
					Group:   "test.org",
					Version: "v1alpha1",
					Kind:    "Test",
				},
				crds: []*extv1.CustomResourceDefinition{
					{
						Spec: extv1.CustomResourceDefinitionSpec{
							Group: "test.org",
							Names: extv1.CustomResourceDefinitionNames{
								Kind: "Test",
							},
							Versions: []extv1.CustomResourceDefinitionVersion{
								{
									Name: "v1alpha1",
									Schema: &extv1.CustomResourceValidation{
										OpenAPIV3Schema: &extv1.JSONSchemaProps{
											Type: "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"spec": {
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"replicas": {
															Type: "integer",
														},
														"deletionPolicy": {
															Type:    "string",
															Default: &extv1.JSON{Raw: []byte(`"Delete"`)},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1alpha1",
						"kind":       "Test",
						"spec": map[string]interface{}{
							"replicas":       3,
							"deletionPolicy": "Delete",
						},
					},
				},
				err: nil,
			},
		},
		"DoNotOverrideExisting": {
			reason: "Should not override existing values with defaults",
			args: args{
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1alpha1",
						"kind":       "Test",
						"spec": map[string]interface{}{
							"replicas":       3,
							"deletionPolicy": "Retain",
						},
					},
				},
				gvk: runtimeschema.GroupVersionKind{
					Group:   "test.org",
					Version: "v1alpha1",
					Kind:    "Test",
				},
				crds: []*extv1.CustomResourceDefinition{
					{
						Spec: extv1.CustomResourceDefinitionSpec{
							Group: "test.org",
							Names: extv1.CustomResourceDefinitionNames{
								Kind: "Test",
							},
							Versions: []extv1.CustomResourceDefinitionVersion{
								{
									Name: "v1alpha1",
									Schema: &extv1.CustomResourceValidation{
										OpenAPIV3Schema: &extv1.JSONSchemaProps{
											Type: "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"spec": {
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"replicas": {
															Type: "integer",
														},
														"deletionPolicy": {
															Type:    "string",
															Default: &extv1.JSON{Raw: []byte(`"Delete"`)},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1alpha1",
						"kind":       "Test",
						"spec": map[string]interface{}{
							"replicas":       3,
							"deletionPolicy": "Retain",
						},
					},
				},
				err: nil,
			},
		},
		"NestedDefaults": {
			reason: "Should apply defaults to nested objects",
			args: args{
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1alpha1",
						"kind":       "Test",
						"spec": map[string]interface{}{
							"forProvider": map[string]interface{}{
								"region": "us-east-1",
							},
						},
					},
				},
				gvk: runtimeschema.GroupVersionKind{
					Group:   "test.org",
					Version: "v1alpha1",
					Kind:    "Test",
				},
				crds: []*extv1.CustomResourceDefinition{
					{
						Spec: extv1.CustomResourceDefinitionSpec{
							Group: "test.org",
							Names: extv1.CustomResourceDefinitionNames{
								Kind: "Test",
							},
							Versions: []extv1.CustomResourceDefinitionVersion{
								{
									Name: "v1alpha1",
									Schema: &extv1.CustomResourceValidation{
										OpenAPIV3Schema: &extv1.JSONSchemaProps{
											Type: "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"spec": {
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"forProvider": {
															Type: "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"region": {
																	Type: "string",
																},
																"instanceType": {
																	Type:    "string",
																	Default: &extv1.JSON{Raw: []byte(`"t3.micro"`)},
																},
															},
														},
														"deletionPolicy": {
															Type:    "string",
															Default: &extv1.JSON{Raw: []byte(`"Delete"`)},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1alpha1",
						"kind":       "Test",
						"spec": map[string]interface{}{
							"forProvider": map[string]interface{}{
								"region":       "us-east-1",
								"instanceType": "t3.micro",
							},
							"deletionPolicy": "Delete",
						},
					},
				},
				err: nil,
			},
		},
		"ComplexDefaults": {
			reason: "Should apply complex default values (objects, arrays)",
			args: args{
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1alpha1",
						"kind":       "Test",
						"spec": map[string]interface{}{
							"name": "test",
						},
					},
				},
				gvk: runtimeschema.GroupVersionKind{
					Group:   "test.org",
					Version: "v1alpha1",
					Kind:    "Test",
				},
				crds: []*extv1.CustomResourceDefinition{
					{
						Spec: extv1.CustomResourceDefinitionSpec{
							Group: "test.org",
							Names: extv1.CustomResourceDefinitionNames{
								Kind: "Test",
							},
							Versions: []extv1.CustomResourceDefinitionVersion{
								{
									Name: "v1alpha1",
									Schema: &extv1.CustomResourceValidation{
										OpenAPIV3Schema: &extv1.JSONSchemaProps{
											Type: "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"spec": {
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"name": {
															Type: "string",
														},
														"metadata": {
															Type:    "object",
															Default: &extv1.JSON{Raw: []byte(`{"labels":{"app":"default-app"}}`)},
														},
														"tags": {
															Type:    "array",
															Default: &extv1.JSON{Raw: []byte(`["default","tag"]`)},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1alpha1",
						"kind":       "Test",
						"spec": map[string]interface{}{
							"name": "test",
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"app": "default-app",
								},
							},
							"tags": []interface{}{"default", "tag"},
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := applyDefaults(tc.args.resource, tc.args.gvk, tc.args.crds)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\napplyDefaults(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.resource, tc.args.resource); diff != "" {
				t.Errorf("%s\napplyDefaults(...): -want resource, +got resource:\n%s", tc.reason, diff)
			}
		})
	}
}
