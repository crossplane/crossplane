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

// Package xcrd generates CustomResourceDefinitions from Crossplane definitions.
//
// v1.JSONSchemaProps is incompatible with controller-tools (as of 0.2.4)
// because it is missing JSON tags and uses float64, which is a disallowed type.
// We thus copy the entire struct as CRDSpecTemplate. See the below issue:
// https://github.com/kubernetes-sigs/controller-tools/issues/291
package xcrd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

var (
	name        = "coolcomposites.example.org"
	labels      = map[string]string{"cool": "very"}
	annotations = map[string]string{"example.org/cool": "very"}

	group    = "example.org"
	version  = "v1"
	kind     = "CoolComposite"
	listKind = "CoolCompositeList"
	singular = "coolcomposite"
	plural   = "coolcomposites"

	d = &v1.CompositeResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
			UID:         types.UID("you-you-eye-dee"),
		},
		Spec: v1.CompositeResourceDefinitionSpec{
			Group: group,
			Names: extv1.CustomResourceDefinitionNames{
				Plural:   plural,
				Singular: singular,
				Kind:     kind,
				ListKind: listKind,
			},
			Versions: []v1.CompositeResourceDefinitionVersion{{
				Name:          version,
				Referenceable: true,
				Served:        true,
			}},
		},
	}

	schema = `
{
	"required": [
		"spec"
	],
	"properties": {
		"spec": {
			"description": "Specification of the resource.",
			"required": [
				"storageGB",
				"engineVersion"
			],
			"properties": {
				"engineVersion": {
					"enum": [
						"5.6",
						"5.7"
					],
					"type": "string"
				},
				"storageGB": {
					"type": "integer",
					"description": "Pretend this is useful."
				},
				"someField": {
					"type": "string",
					"description": "Pretend this is useful."
				},
				"someOtherField": {
					"type": "string",
					"description": "Pretend this is useful."
				}
			},
			"x-kubernetes-validations": [
				{
					"message": "Cannot change engine version",
					"rule": "self.engineVersion == oldSelf.engineVersion"
				}
			],
			"type": "object",
			"oneOf": [
				{
					"required": ["someField"]
				},
				{
					"required": ["someOtherField"]
				}
			]
		},
		"status": {
			"properties": {
				"phase": {
					"type": "string"
				},
				"something": {
					"type": "string"
				}
			},
			"x-kubernetes-validations": [
				{
					"message": "Phase is required once set",
					"rule": "!has(oldSelf.phase) || has(self.phase)"
				}
			],
			"oneOf": [
				{ "required": ["phase"] },
				{ "required": ["something"] }
			],
			"type": "object",
			"description": "Status of the resource."
		}
	},
	"type": "object",
	"description": "What the resource is for."
}`
)

func TestIsEstablished(t *testing.T) {
	cases := map[string]struct {
		s    extv1.CustomResourceDefinitionStatus
		want bool
	}{
		"IsEstablished": {
			s: extv1.CustomResourceDefinitionStatus{
				Conditions: []extv1.CustomResourceDefinitionCondition{{
					Type:   extv1.Established,
					Status: extv1.ConditionTrue,
				}},
			},
			want: true,
		},
		"IsNotEstablished": {
			s:    extv1.CustomResourceDefinitionStatus{},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsEstablished(tc.s)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("IsEstablished(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestForCompositeResource(t *testing.T) {
	defaultCompositionUpdatePolicy := xpv1.UpdatePolicy("Automatic")
	type args struct {
		xrd *v1.CompositeResourceDefinition
		v   *v1.CompositeResourceValidation
	}
	type want struct {
		c   *extv1.CustomResourceDefinition
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Successful": {
			reason: "A CRD should be generated from a CompositeResourceDefinitionVersion.",
			args: args{
				v: &v1.CompositeResourceValidation{
					OpenAPIV3Schema: runtime.RawExtension{Raw: []byte(schema)},
				},
			},
			want: want{
				c: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name:   name,
						Labels: labels,
						OwnerReferences: []metav1.OwnerReference{
							meta.AsController(meta.TypedReferenceTo(d, v1.CompositeResourceDefinitionGroupVersionKind)),
						},
					},
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: group,
						Names: extv1.CustomResourceDefinitionNames{
							Plural:     plural,
							Singular:   singular,
							Kind:       kind,
							ListKind:   listKind,
							Categories: []string{CategoryComposite},
						},
						Scope: extv1.ClusterScoped,
						Versions: []extv1.CustomResourceDefinitionVersion{{
							Name:    version,
							Served:  true,
							Storage: true,
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
									Name:     "AGE",
									Type:     "date",
									JSONPath: ".metadata.creationTimestamp",
								},
							},
							Schema: &extv1.CustomResourceValidation{
								OpenAPIV3Schema: &extv1.JSONSchemaProps{
									Type:        "object",
									Description: "What the resource is for.",
									Required:    []string{"spec"},
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
											Properties: map[string]extv1.JSONSchemaProps{
												"name": {
													Type:      "string",
													MaxLength: ptr.To[int64](63),
												},
											},
										},
										"spec": {
											Type:        "object",
											Required:    []string{"storageGB", "engineVersion"},
											Description: "Specification of the resource.",
											Properties: map[string]extv1.JSONSchemaProps{
												// From CRDSpecTemplate.Validation
												"storageGB": {Type: "integer", Description: "Pretend this is useful."},
												"engineVersion": {
													Type: "string",
													Enum: []extv1.JSON{
														{Raw: []byte(`"5.6"`)},
														{Raw: []byte(`"5.7"`)},
													},
												},
												"someField":      {Type: "string", Description: "Pretend this is useful."},
												"someOtherField": {Type: "string", Description: "Pretend this is useful."},

												// From CompositeResourceSpecProps()
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
												"environmentConfigRefs": {
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
													XListType: ptr.To("atomic"),
												},
												"publishConnectionDetailsTo": {
													Type:     "object",
													Required: []string{"name"},
													Properties: map[string]extv1.JSONSchemaProps{
														"name": {Type: "string"},
														"configRef": {
															Type:    "object",
															Default: &extv1.JSON{Raw: []byte(`{"name": "default"}`)},
															Properties: map[string]extv1.JSONSchemaProps{
																"name": {
																	Type: "string",
																},
															},
														},
														"metadata": {
															Type: "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"labels": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{Type: "string"},
																	},
																},
																"annotations": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{Type: "string"},
																	},
																},
																"type": {
																	Type: "string",
																},
															},
														},
													},
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
											XValidations: extv1.ValidationRules{
												{
													Message: "Cannot change engine version",
													Rule:    "self.engineVersion == oldSelf.engineVersion",
												},
											},
											OneOf: []extv1.JSONSchemaProps{
												{Required: []string{"someField"}},
												{Required: []string{"someOtherField"}},
											},
										},
										"status": {
											Type:        "object",
											Description: "Status of the resource.",
											Properties: map[string]extv1.JSONSchemaProps{
												"phase":     {Type: "string"},
												"something": {Type: "string"},

												// From CompositeResourceStatusProps()
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
											},
											XValidations: extv1.ValidationRules{
												{
													Message: "Phase is required once set",
													Rule:    "!has(oldSelf.phase) || has(self.phase)",
												},
											},
											OneOf: []extv1.JSONSchemaProps{
												{Required: []string{"phase"}},
												{Required: []string{"something"}},
											},
										},
									},
								},
							},
						}},
					},
				},
			},
		},
		"DefaultCompositionUpdatePolicyIsSet": {
			reason: "A CRD should be generated from a CompositeResourceDefinitionVersion.",
			args: args{
				xrd: &v1.CompositeResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name:        name,
						Labels:      labels,
						Annotations: annotations,
						UID:         types.UID("you-you-eye-dee"),
					},
					Spec: v1.CompositeResourceDefinitionSpec{
						Group: group,
						Names: extv1.CustomResourceDefinitionNames{
							Plural:   plural,
							Singular: singular,
							Kind:     kind,
							ListKind: listKind,
						},
						Versions: []v1.CompositeResourceDefinitionVersion{{
							Name:          version,
							Referenceable: true,
							Served:        true,
						}},
						DefaultCompositionUpdatePolicy: &defaultCompositionUpdatePolicy,
					},
				},
				v: &v1.CompositeResourceValidation{
					OpenAPIV3Schema: runtime.RawExtension{Raw: []byte(schema)},
				},
			},
			want: want{
				c: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name:   name,
						Labels: labels,
						OwnerReferences: []metav1.OwnerReference{
							meta.AsController(meta.TypedReferenceTo(d, v1.CompositeResourceDefinitionGroupVersionKind)),
						},
					},
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: group,
						Names: extv1.CustomResourceDefinitionNames{
							Plural:     plural,
							Singular:   singular,
							Kind:       kind,
							ListKind:   listKind,
							Categories: []string{CategoryComposite},
						},
						Scope: extv1.ClusterScoped,
						Versions: []extv1.CustomResourceDefinitionVersion{{
							Name:    version,
							Served:  true,
							Storage: true,
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
									Name:     "AGE",
									Type:     "date",
									JSONPath: ".metadata.creationTimestamp",
								},
							},
							Schema: &extv1.CustomResourceValidation{
								OpenAPIV3Schema: &extv1.JSONSchemaProps{
									Type:        "object",
									Description: "What the resource is for.",
									Required:    []string{"spec"},
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
											Properties: map[string]extv1.JSONSchemaProps{
												"name": {
													Type:      "string",
													MaxLength: ptr.To[int64](63),
												},
											},
										},
										"spec": {
											Type:        "object",
											Required:    []string{"storageGB", "engineVersion"},
											Description: "Specification of the resource.",
											Properties: map[string]extv1.JSONSchemaProps{
												// From CRDSpecTemplate.Validation
												"storageGB": {Type: "integer", Description: "Pretend this is useful."},
												"engineVersion": {
													Type: "string",
													Enum: []extv1.JSON{
														{Raw: []byte(`"5.6"`)},
														{Raw: []byte(`"5.7"`)},
													},
												},
												"someField":      {Type: "string", Description: "Pretend this is useful."},
												"someOtherField": {Type: "string", Description: "Pretend this is useful."},

												// From CompositeResourceSpecProps()
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
													Type:    "string",
													Default: &extv1.JSON{Raw: []byte(fmt.Sprintf("\"%s\"", defaultCompositionUpdatePolicy))},
													Enum: []extv1.JSON{
														{Raw: []byte(`"Automatic"`)},
														{Raw: []byte(`"Manual"`)},
													},
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
												"environmentConfigRefs": {
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
													XListType: ptr.To("atomic"),
												},
												"publishConnectionDetailsTo": {
													Type:     "object",
													Required: []string{"name"},
													Properties: map[string]extv1.JSONSchemaProps{
														"name": {Type: "string"},
														"configRef": {
															Type:    "object",
															Default: &extv1.JSON{Raw: []byte(`{"name": "default"}`)},
															Properties: map[string]extv1.JSONSchemaProps{
																"name": {
																	Type: "string",
																},
															},
														},
														"metadata": {
															Type: "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"labels": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{Type: "string"},
																	},
																},
																"annotations": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{Type: "string"},
																	},
																},
																"type": {
																	Type: "string",
																},
															},
														},
													},
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
											XValidations: extv1.ValidationRules{
												{
													Message: "Cannot change engine version",
													Rule:    "self.engineVersion == oldSelf.engineVersion",
												},
											},
											OneOf: []extv1.JSONSchemaProps{
												{Required: []string{"someField"}},
												{Required: []string{"someOtherField"}},
											},
										},
										"status": {
											Type:        "object",
											Description: "Status of the resource.",
											Properties: map[string]extv1.JSONSchemaProps{
												"phase":     {Type: "string"},
												"something": {Type: "string"},

												// From CompositeResourceStatusProps()
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
											},
											XValidations: extv1.ValidationRules{
												{
													Message: "Phase is required once set",
													Rule:    "!has(oldSelf.phase) || has(self.phase)",
												},
											},
											OneOf: []extv1.JSONSchemaProps{
												{Required: []string{"phase"}},
												{Required: []string{"something"}},
											},
										},
									},
								},
							},
						}},
					},
				},
			},
		},
		"EmptyOpenAPIV3Schema": {
			reason: "A CRD should be generated from a CompositeResourceDefinitionVersion when schema is empty.",
			args: args{
				v: &v1.CompositeResourceValidation{
					OpenAPIV3Schema: runtime.RawExtension{Raw: []byte(`{}`)},
				},
			},
			want: want{
				c: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name:   name,
						Labels: labels,
						OwnerReferences: []metav1.OwnerReference{
							meta.AsController(meta.TypedReferenceTo(d, v1.CompositeResourceDefinitionGroupVersionKind)),
						},
					},
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: group,
						Names: extv1.CustomResourceDefinitionNames{
							Plural:     plural,
							Singular:   singular,
							Kind:       kind,
							ListKind:   listKind,
							Categories: []string{CategoryComposite},
						},
						Scope: extv1.ClusterScoped,
						Versions: []extv1.CustomResourceDefinitionVersion{{
							Name:    version,
							Served:  true,
							Storage: true,
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
									Name:     "AGE",
									Type:     "date",
									JSONPath: ".metadata.creationTimestamp",
								},
							},
							Schema: &extv1.CustomResourceValidation{
								OpenAPIV3Schema: &extv1.JSONSchemaProps{
									Type:        "object",
									Description: "",
									Required:    []string{"spec"},
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
											Properties: map[string]extv1.JSONSchemaProps{
												"name": {
													Type:      "string",
													MaxLength: ptr.To[int64](63),
												},
											},
										},
										"spec": {
											Type:        "object",
											Description: "",
											Properties: map[string]extv1.JSONSchemaProps{
												// From CompositeResourceSpecProps()
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
												"environmentConfigRefs": {
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
													XListType: ptr.To("atomic"),
												},
												"publishConnectionDetailsTo": {
													Type:     "object",
													Required: []string{"name"},
													Properties: map[string]extv1.JSONSchemaProps{
														"name": {Type: "string"},
														"configRef": {
															Type:    "object",
															Default: &extv1.JSON{Raw: []byte(`{"name": "default"}`)},
															Properties: map[string]extv1.JSONSchemaProps{
																"name": {
																	Type: "string",
																},
															},
														},
														"metadata": {
															Type: "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"labels": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{Type: "string"},
																	},
																},
																"annotations": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{Type: "string"},
																	},
																},
																"type": {
																	Type: "string",
																},
															},
														},
													},
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
											Type:        "object",
											Description: "",
											Properties: map[string]extv1.JSONSchemaProps{

												// From CompositeResourceStatusProps()
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
											},
										},
									},
								},
							},
						}},
					},
				},
			},
		},
		"RestrictingNameLength": {
			reason: "A CRD should be generated from a CompositeResourceDefinitionVersion.",
			args: args{
				v: &v1.CompositeResourceValidation{
					OpenAPIV3Schema: runtime.RawExtension{Raw: []byte(strings.Replace(schema, `"spec":`, `"metadata":{"type":"object","properties":{"name":{"type":"string","maxLength":10}}},"spec":`, 1))},
				},
			},
			want: want{
				c: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name:   name,
						Labels: labels,
						OwnerReferences: []metav1.OwnerReference{
							meta.AsController(meta.TypedReferenceTo(d, v1.CompositeResourceDefinitionGroupVersionKind)),
						},
					},
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: group,
						Names: extv1.CustomResourceDefinitionNames{
							Plural:     plural,
							Singular:   singular,
							Kind:       kind,
							ListKind:   listKind,
							Categories: []string{CategoryComposite},
						},
						Scope: extv1.ClusterScoped,
						Versions: []extv1.CustomResourceDefinitionVersion{{
							Name:    version,
							Served:  true,
							Storage: true,
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
									Name:     "AGE",
									Type:     "date",
									JSONPath: ".metadata.creationTimestamp",
								},
							},
							Schema: &extv1.CustomResourceValidation{
								OpenAPIV3Schema: &extv1.JSONSchemaProps{
									Type:        "object",
									Description: "What the resource is for.",
									Required:    []string{"spec"},
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
											Properties: map[string]extv1.JSONSchemaProps{
												"name": {
													Type:      "string",
													MaxLength: ptr.To[int64](10),
												},
											},
										},
										"spec": {
											Type:        "object",
											Required:    []string{"storageGB", "engineVersion"},
											Description: "Specification of the resource.",
											Properties: map[string]extv1.JSONSchemaProps{
												// From CRDSpecTemplate.Validation
												"storageGB": {Type: "integer", Description: "Pretend this is useful."},
												"engineVersion": {
													Type: "string",
													Enum: []extv1.JSON{
														{Raw: []byte(`"5.6"`)},
														{Raw: []byte(`"5.7"`)},
													},
												},
												"someField":      {Type: "string", Description: "Pretend this is useful."},
												"someOtherField": {Type: "string", Description: "Pretend this is useful."},

												// From CompositeResourceSpecProps()
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
												"environmentConfigRefs": {
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
													XListType: ptr.To("atomic"),
												},
												"publishConnectionDetailsTo": {
													Type:     "object",
													Required: []string{"name"},
													Properties: map[string]extv1.JSONSchemaProps{
														"name": {Type: "string"},
														"configRef": {
															Type:    "object",
															Default: &extv1.JSON{Raw: []byte(`{"name": "default"}`)},
															Properties: map[string]extv1.JSONSchemaProps{
																"name": {
																	Type: "string",
																},
															},
														},
														"metadata": {
															Type: "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"labels": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{Type: "string"},
																	},
																},
																"annotations": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{Type: "string"},
																	},
																},
																"type": {
																	Type: "string",
																},
															},
														},
													},
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
											XValidations: extv1.ValidationRules{
												{
													Message: "Cannot change engine version",
													Rule:    "self.engineVersion == oldSelf.engineVersion",
												},
											},
											OneOf: []extv1.JSONSchemaProps{
												{Required: []string{"someField"}},
												{Required: []string{"someOtherField"}},
											},
										},
										"status": {
											Type:        "object",
											Description: "Status of the resource.",
											Properties: map[string]extv1.JSONSchemaProps{
												"phase":     {Type: "string"},
												"something": {Type: "string"},

												// From CompositeResourceStatusProps()
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
											},
											XValidations: extv1.ValidationRules{
												{
													Message: "Phase is required once set",
													Rule:    "!has(oldSelf.phase) || has(self.phase)",
												},
											},
											OneOf: []extv1.JSONSchemaProps{
												{Required: []string{"phase"}},
												{Required: []string{"something"}},
											},
										},
									},
								},
							},
						}},
					},
				},
			},
		},
		"WeaklyRestrictingNameLength": {
			reason: "A CRD should be generated from a CompositeResourceDefinitionVersion.",
			args: args{
				v: &v1.CompositeResourceValidation{
					OpenAPIV3Schema: runtime.RawExtension{Raw: []byte(strings.Replace(schema, `"spec":`, `"metadata":{"type":"object","properties":{"name":{"type":"string","maxLength":100}}},"spec":`, 1))},
				},
			},
			want: want{
				c: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name:   name,
						Labels: labels,
						OwnerReferences: []metav1.OwnerReference{
							meta.AsController(meta.TypedReferenceTo(d, v1.CompositeResourceDefinitionGroupVersionKind)),
						},
					},
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: group,
						Names: extv1.CustomResourceDefinitionNames{
							Plural:     plural,
							Singular:   singular,
							Kind:       kind,
							ListKind:   listKind,
							Categories: []string{CategoryComposite},
						},
						Scope: extv1.ClusterScoped,
						Versions: []extv1.CustomResourceDefinitionVersion{{
							Name:    version,
							Served:  true,
							Storage: true,
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
									Name:     "AGE",
									Type:     "date",
									JSONPath: ".metadata.creationTimestamp",
								},
							},
							Schema: &extv1.CustomResourceValidation{
								OpenAPIV3Schema: &extv1.JSONSchemaProps{
									Type:        "object",
									Description: "What the resource is for.",
									Required:    []string{"spec"},
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
											Properties: map[string]extv1.JSONSchemaProps{
												"name": {
													Type:      "string",
													MaxLength: ptr.To[int64](63),
												},
											},
										},
										"spec": {
											Type:        "object",
											Required:    []string{"storageGB", "engineVersion"},
											Description: "Specification of the resource.",
											Properties: map[string]extv1.JSONSchemaProps{
												// From CRDSpecTemplate.Validation
												"storageGB": {Type: "integer", Description: "Pretend this is useful."},
												"engineVersion": {
													Type: "string",
													Enum: []extv1.JSON{
														{Raw: []byte(`"5.6"`)},
														{Raw: []byte(`"5.7"`)},
													},
												},
												"someField":      {Type: "string", Description: "Pretend this is useful."},
												"someOtherField": {Type: "string", Description: "Pretend this is useful."},

												// From CompositeResourceSpecProps()
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
												"environmentConfigRefs": {
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
													XListType: ptr.To("atomic"),
												},
												"publishConnectionDetailsTo": {
													Type:     "object",
													Required: []string{"name"},
													Properties: map[string]extv1.JSONSchemaProps{
														"name": {Type: "string"},
														"configRef": {
															Type:    "object",
															Default: &extv1.JSON{Raw: []byte(`{"name": "default"}`)},
															Properties: map[string]extv1.JSONSchemaProps{
																"name": {
																	Type: "string",
																},
															},
														},
														"metadata": {
															Type: "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"labels": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{Type: "string"},
																	},
																},
																"annotations": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{Type: "string"},
																	},
																},
																"type": {
																	Type: "string",
																},
															},
														},
													},
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
											XValidations: extv1.ValidationRules{
												{
													Message: "Cannot change engine version",
													Rule:    "self.engineVersion == oldSelf.engineVersion",
												},
											},
											OneOf: []extv1.JSONSchemaProps{
												{Required: []string{"someField"}},
												{Required: []string{"someOtherField"}},
											},
										},
										"status": {
											Type:        "object",
											Description: "Status of the resource.",
											Properties: map[string]extv1.JSONSchemaProps{
												"phase":     {Type: "string"},
												"something": {Type: "string"},

												// From CompositeResourceStatusProps()
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
											},
											XValidations: extv1.ValidationRules{
												{
													Message: "Phase is required once set",
													Rule:    "!has(oldSelf.phase) || has(self.phase)",
												},
											},
											OneOf: []extv1.JSONSchemaProps{
												{Required: []string{"phase"}},
												{Required: []string{"something"}},
											},
										},
									},
								},
							},
						}},
					},
				},
			},
		},
		"NilCompositeResourceValidation": {
			reason: "Error should be returned if composite resource validation is nil.",
			args: args{
				v: nil,
			},
			want: want{
				err: errors.Wrap(errors.New(errCustomResourceValidationNil), fmt.Sprintf(errFmtGenCrd, "Composite Resource", name)),
				c:   nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var xrd *v1.CompositeResourceDefinition
			if tc.args.xrd != nil {
				xrd = tc.args.xrd
			} else {
				xrd = d
			}
			xrd.Spec.Versions[0].Schema = tc.args.v
			got, err := ForCompositeResource(xrd)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nForCompositeResource(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.c, got, test.EquateErrors()); diff != "" {
				t.Errorf("ForCompositeResource(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestValidateClaimNames(t *testing.T) {
	cases := map[string]struct {
		d    *v1.CompositeResourceDefinition
		want error
	}{
		"MissingClaimNames": {
			d:    &v1.CompositeResourceDefinition{},
			want: errors.New(errMissingClaimNames),
		},
		"KindConflict": {
			d: &v1.CompositeResourceDefinition{
				Spec: v1.CompositeResourceDefinitionSpec{
					ClaimNames: &extv1.CustomResourceDefinitionNames{
						Kind:     "a",
						ListKind: "a",
						Singular: "a",
						Plural:   "a",
					},
					Names: extv1.CustomResourceDefinitionNames{
						Kind:     "a",
						ListKind: "b",
						Singular: "b",
						Plural:   "b",
					},
				},
			},
			want: errors.Errorf(errFmtConflictingClaimName, "a"),
		},
		"ListKindConflict": {
			d: &v1.CompositeResourceDefinition{
				Spec: v1.CompositeResourceDefinitionSpec{
					ClaimNames: &extv1.CustomResourceDefinitionNames{
						Kind:     "a",
						ListKind: "a",
						Singular: "a",
						Plural:   "a",
					},
					Names: extv1.CustomResourceDefinitionNames{
						Kind:     "b",
						ListKind: "a",
						Singular: "b",
						Plural:   "b",
					},
				},
			},
			want: errors.Errorf(errFmtConflictingClaimName, "a"),
		},
		"SingularConflict": {
			d: &v1.CompositeResourceDefinition{
				Spec: v1.CompositeResourceDefinitionSpec{
					ClaimNames: &extv1.CustomResourceDefinitionNames{
						Kind:     "a",
						ListKind: "a",
						Singular: "a",
						Plural:   "a",
					},
					Names: extv1.CustomResourceDefinitionNames{
						Kind:     "b",
						ListKind: "b",
						Singular: "a",
						Plural:   "b",
					},
				},
			},
			want: errors.Errorf(errFmtConflictingClaimName, "a"),
		},
		"PluralConflict": {
			d: &v1.CompositeResourceDefinition{
				Spec: v1.CompositeResourceDefinitionSpec{
					ClaimNames: &extv1.CustomResourceDefinitionNames{
						Kind:     "a",
						ListKind: "a",
						Singular: "a",
						Plural:   "a",
					},
					Names: extv1.CustomResourceDefinitionNames{
						Kind:       "b",
						ListKind:   "b",
						Singular:   "b",
						Plural:     "a",
						Categories: []string{CategoryClaim},
					},
				},
			},
			want: errors.Errorf(errFmtConflictingClaimName, "a"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := validateClaimNames(tc.d)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("validateClaimNames(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestForCompositeResourceClaim(t *testing.T) {
	name := "coolcomposites.example.org"
	labels := map[string]string{"cool": "very"}
	annotations := map[string]string{"example.org/cool": "very"}

	group := "example.org"
	version := "v1"

	kind := "CoolComposite"
	listKind := "CoolCompositeList"
	singular := "coolcomposite"
	plural := "coolcomposites"

	claimKind := "CoolClaim"
	claimListKind := "CoolClaimList"
	claimSingular := "coolclaim"
	claimPlural := "coolclaims"

	defaultPolicy := xpv1.CompositeDeletePolicy("Background")
	schema := `
{
	"properties": {
		"spec": {
			"description": "Specification of the resource.",
			"required": [
				"storageGB",
				"engineVersion"
			],
			"properties": {
				"engineVersion": {
					"enum": [
						"5.6",
						"5.7"
					],
					"type": "string"
				},
				"storageGB": {
					"type": "integer",
					"description": "Pretend this is useful."
				}
			},
			"x-kubernetes-validations": [
				{
					"message": "Cannot change engine version",
					"rule": "self.engineVersion == oldSelf.engineVersion"
				}
			],
			"type": "object"
		},
		"status": {
			"properties": {
				"phase": {
					"type": "string"
				}
			},
			"x-kubernetes-validations": [
				{
					"message": "Phase is required once set",
					"rule": "!has(oldSelf.phase) || has(self.phase)"
				}
			],
			"type": "object",
			"description": "Status of the resource."
		}
	},
	"type": "object",
	"description": "Description of the resource."
}`

	cases := map[string]struct {
		reason string
		crd    *v1.CompositeResourceDefinition
		want   *extv1.CustomResourceDefinition
	}{
		"CompositeDeletionPolicyUnspecified": {
			reason: "If default composite deletion unspecified on XRD, set no default value on claim's spec.compositeDeletionPolicy",
			crd: &v1.CompositeResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:        name,
					Labels:      labels,
					Annotations: annotations,
					UID:         types.UID("you-you-eye-dee"),
				},
				Spec: v1.CompositeResourceDefinitionSpec{
					Group: group,
					Names: extv1.CustomResourceDefinitionNames{
						Plural:   plural,
						Singular: singular,
						Kind:     kind,
						ListKind: listKind,
					},
					ClaimNames: &extv1.CustomResourceDefinitionNames{
						Plural:   claimPlural,
						Singular: claimSingular,
						Kind:     claimKind,
						ListKind: claimListKind,
					},
					Versions: []v1.CompositeResourceDefinitionVersion{{
						Name:          version,
						Referenceable: true,
						Served:        true,
						Schema: &v1.CompositeResourceValidation{
							OpenAPIV3Schema: runtime.RawExtension{Raw: []byte(schema)},
						},
					}},
				},
			},

			want: &extv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:   claimPlural + "." + group,
					Labels: labels,
					OwnerReferences: []metav1.OwnerReference{
						meta.AsController(meta.TypedReferenceTo(d, v1.CompositeResourceDefinitionGroupVersionKind)),
					},
				},
				Spec: extv1.CustomResourceDefinitionSpec{
					Group: group,
					Names: extv1.CustomResourceDefinitionNames{
						Plural:     claimPlural,
						Singular:   claimSingular,
						Kind:       claimKind,
						ListKind:   claimListKind,
						Categories: []string{CategoryClaim},
					},
					Scope: extv1.NamespaceScoped,
					Versions: []extv1.CustomResourceDefinitionVersion{
						{
							Name:    version,
							Served:  true,
							Storage: true,
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
									Type:        "object",
									Required:    []string{"spec"},
									Description: "Description of the resource.",
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
											Properties: map[string]extv1.JSONSchemaProps{
												"name": {
													Type:      "string",
													MaxLength: ptr.To[int64](63),
												},
											},
										},
										"spec": {
											Type:        "object",
											Required:    []string{"storageGB", "engineVersion"},
											Description: "Specification of the resource.",
											Properties: map[string]extv1.JSONSchemaProps{
												// From CRDSpecTemplate.Validation
												"storageGB": {Type: "integer", Description: "Pretend this is useful."},
												"engineVersion": {
													Type: "string",
													Enum: []extv1.JSON{
														{Raw: []byte(`"5.6"`)},
														{Raw: []byte(`"5.7"`)},
													},
												},
												"compositeDeletePolicy": {
													Type: "string",
													Enum: []extv1.JSON{{Raw: []byte(`"Background"`)},
														{Raw: []byte(`"Foreground"`)}},
												},
												// From CompositeResourceClaimSpecProps()
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
												"resourceRef": {
													Type:     "object",
													Required: []string{"apiVersion", "kind", "name"},
													Properties: map[string]extv1.JSONSchemaProps{
														"apiVersion": {Type: "string"},
														"kind":       {Type: "string"},
														"name":       {Type: "string"},
													},
												},
												"publishConnectionDetailsTo": {
													Type:     "object",
													Required: []string{"name"},
													Properties: map[string]extv1.JSONSchemaProps{
														"name": {Type: "string"},
														"configRef": {
															Type:    "object",
															Default: &extv1.JSON{Raw: []byte(`{"name": "default"}`)},
															Properties: map[string]extv1.JSONSchemaProps{
																"name": {
																	Type: "string",
																},
															},
														},
														"metadata": {
															Type: "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"labels": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{Type: "string"},
																	},
																},
																"annotations": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{Type: "string"},
																	},
																},
																"type": {
																	Type: "string",
																},
															},
														},
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
											XValidations: extv1.ValidationRules{
												{
													Message: "Cannot change engine version",
													Rule:    "self.engineVersion == oldSelf.engineVersion",
												},
											},
										},
										"status": {
											Type:        "object",
											Description: "Status of the resource.",
											Properties: map[string]extv1.JSONSchemaProps{
												"phase": {Type: "string"},

												// From CompositeResourceStatusProps()
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
											},
											XValidations: extv1.ValidationRules{
												{
													Message: "Phase is required once set",
													Rule:    "!has(oldSelf.phase) || has(self.phase)",
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
		"CompositeDeletionPolicySetToDefault": {
			reason: "Propagate default composite deletion set on XRD as the default value on claim's spec.compositeDeletionPolicy",
			crd: &v1.CompositeResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:        name,
					Labels:      labels,
					Annotations: annotations,
					UID:         types.UID("you-you-eye-dee"),
				},
				Spec: v1.CompositeResourceDefinitionSpec{
					Group:                        group,
					DefaultCompositeDeletePolicy: &defaultPolicy,
					Names: extv1.CustomResourceDefinitionNames{
						Plural:   plural,
						Singular: singular,
						Kind:     kind,
						ListKind: listKind,
					},
					ClaimNames: &extv1.CustomResourceDefinitionNames{
						Plural:   claimPlural,
						Singular: claimSingular,
						Kind:     claimKind,
						ListKind: claimListKind,
					},
					Versions: []v1.CompositeResourceDefinitionVersion{{
						Name:          version,
						Referenceable: true,
						Served:        true,
						Schema: &v1.CompositeResourceValidation{
							OpenAPIV3Schema: runtime.RawExtension{Raw: []byte(schema)},
						},
					}},
				},
			},

			want: &extv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:   claimPlural + "." + group,
					Labels: labels,
					OwnerReferences: []metav1.OwnerReference{
						meta.AsController(meta.TypedReferenceTo(d, v1.CompositeResourceDefinitionGroupVersionKind)),
					},
				},
				Spec: extv1.CustomResourceDefinitionSpec{
					Group: group,
					Names: extv1.CustomResourceDefinitionNames{
						Plural:     claimPlural,
						Singular:   claimSingular,
						Kind:       claimKind,
						ListKind:   claimListKind,
						Categories: []string{CategoryClaim},
					},
					Scope: extv1.NamespaceScoped,
					Versions: []extv1.CustomResourceDefinitionVersion{
						{
							Name:    version,
							Served:  true,
							Storage: true,
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
									Type:        "object",
									Required:    []string{"spec"},
									Description: "Description of the resource.",
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
											Properties: map[string]extv1.JSONSchemaProps{
												"name": {
													Type:      "string",
													MaxLength: ptr.To[int64](63),
												},
											},
										},
										"spec": {
											Type:        "object",
											Required:    []string{"storageGB", "engineVersion"},
											Description: "Specification of the resource.",
											Properties: map[string]extv1.JSONSchemaProps{
												// From CRDSpecTemplate.Validation
												"storageGB": {Type: "integer", Description: "Pretend this is useful."},
												"engineVersion": {
													Type: "string",
													Enum: []extv1.JSON{
														{Raw: []byte(`"5.6"`)},
														{Raw: []byte(`"5.7"`)},
													},
												},
												"compositeDeletePolicy": {
													Type:    "string",
													Default: &extv1.JSON{Raw: []byte(fmt.Sprintf("\"%s\"", defaultPolicy))},
													Enum: []extv1.JSON{{Raw: []byte(`"Background"`)},
														{Raw: []byte(`"Foreground"`)}},
												},
												// From CompositeResourceClaimSpecProps()
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
												"resourceRef": {
													Type:     "object",
													Required: []string{"apiVersion", "kind", "name"},
													Properties: map[string]extv1.JSONSchemaProps{
														"apiVersion": {Type: "string"},
														"kind":       {Type: "string"},
														"name":       {Type: "string"},
													},
												},
												"publishConnectionDetailsTo": {
													Type:     "object",
													Required: []string{"name"},
													Properties: map[string]extv1.JSONSchemaProps{
														"name": {Type: "string"},
														"configRef": {
															Type:    "object",
															Default: &extv1.JSON{Raw: []byte(`{"name": "default"}`)},
															Properties: map[string]extv1.JSONSchemaProps{
																"name": {
																	Type: "string",
																},
															},
														},
														"metadata": {
															Type: "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"labels": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{Type: "string"},
																	},
																},
																"annotations": {
																	Type: "object",
																	AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																		Allows: true,
																		Schema: &extv1.JSONSchemaProps{Type: "string"},
																	},
																},
																"type": {
																	Type: "string",
																},
															},
														},
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
											XValidations: extv1.ValidationRules{
												{
													Message: "Cannot change engine version",
													Rule:    "self.engineVersion == oldSelf.engineVersion",
												},
											},
										},
										"status": {
											Type:        "object",
											Description: "Status of the resource.",
											Properties: map[string]extv1.JSONSchemaProps{
												"phase": {Type: "string"},

												// From CompositeResourceStatusProps()
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
											},
											XValidations: extv1.ValidationRules{
												{
													Message: "Phase is required once set",
													Rule:    "!has(oldSelf.phase) || has(self.phase)",
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
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ForCompositeResourceClaim(tc.crd)
			if err != nil {
				t.Fatalf("ForCompositeResourceClaim(...): %s", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ForCompositeResourceClaim(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestForCompositeResourceClaimEmptyXrd(t *testing.T) {
	name := "coolcomposites.example.org"
	labels := map[string]string{"cool": "very"}
	annotations := map[string]string{"example.org/cool": "very"}

	group := "example.org"
	version := "v1"

	kind := "CoolComposite"
	listKind := "CoolCompositeList"
	singular := "coolcomposite"
	plural := "coolcomposites"

	claimKind := "CoolClaim"
	claimListKind := "CoolClaimList"
	claimSingular := "coolclaim"
	claimPlural := "coolclaims"

	schema := "{}"

	d := &v1.CompositeResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
			UID:         types.UID("you-you-eye-dee"),
		},
		Spec: v1.CompositeResourceDefinitionSpec{
			Group: group,
			Names: extv1.CustomResourceDefinitionNames{
				Plural:   plural,
				Singular: singular,
				Kind:     kind,
				ListKind: listKind,
			},
			ClaimNames: &extv1.CustomResourceDefinitionNames{
				Plural:   claimPlural,
				Singular: claimSingular,
				Kind:     claimKind,
				ListKind: claimListKind,
			},
			Versions: []v1.CompositeResourceDefinitionVersion{{
				Name:          version,
				Referenceable: true,
				Served:        true,
				Schema: &v1.CompositeResourceValidation{
					OpenAPIV3Schema: runtime.RawExtension{Raw: []byte(schema)},
				},
			}},
		},
	}

	want := &extv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:   claimPlural + "." + group,
			Labels: labels,
			OwnerReferences: []metav1.OwnerReference{
				meta.AsController(meta.TypedReferenceTo(d, v1.CompositeResourceDefinitionGroupVersionKind)),
			},
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: extv1.CustomResourceDefinitionNames{
				Plural:     claimPlural,
				Singular:   claimSingular,
				Kind:       claimKind,
				ListKind:   claimListKind,
				Categories: []string{CategoryClaim},
			},
			Scope: extv1.NamespaceScoped,
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    version,
					Served:  true,
					Storage: true,
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
							Type:        "object",
							Required:    []string{"spec"},
							Description: "",
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
									Properties: map[string]extv1.JSONSchemaProps{
										"name": {
											Type:      "string",
											MaxLength: ptr.To[int64](63),
										},
									},
								},
								"spec": {
									Type:        "object",
									Description: "",
									Properties: map[string]extv1.JSONSchemaProps{
										"compositeDeletePolicy": {
											Type: "string",
											Enum: []extv1.JSON{{Raw: []byte(`"Background"`)},
												{Raw: []byte(`"Foreground"`)}},
										},
										// From CompositeResourceClaimSpecProps()
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
										"resourceRef": {
											Type:     "object",
											Required: []string{"apiVersion", "kind", "name"},
											Properties: map[string]extv1.JSONSchemaProps{
												"apiVersion": {Type: "string"},
												"kind":       {Type: "string"},
												"name":       {Type: "string"},
											},
										},
										"publishConnectionDetailsTo": {
											Type:     "object",
											Required: []string{"name"},
											Properties: map[string]extv1.JSONSchemaProps{
												"name": {Type: "string"},
												"configRef": {
													Type:    "object",
													Default: &extv1.JSON{Raw: []byte(`{"name": "default"}`)},
													Properties: map[string]extv1.JSONSchemaProps{
														"name": {
															Type: "string",
														},
													},
												},
												"metadata": {
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"labels": {
															Type: "object",
															AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																Allows: true,
																Schema: &extv1.JSONSchemaProps{Type: "string"},
															},
														},
														"annotations": {
															Type: "object",
															AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																Allows: true,
																Schema: &extv1.JSONSchemaProps{Type: "string"},
															},
														},
														"type": {
															Type: "string",
														},
													},
												},
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
									Type:        "object",
									Description: "",
									Properties: map[string]extv1.JSONSchemaProps{
										// From CompositeResourceStatusProps()
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
									},
								},
							},
						},
					},
				},
			},
		},
	}

	got, err := ForCompositeResourceClaim(d)
	if err != nil {
		t.Fatalf("ForCompositeResourceClaim(...): %s", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ForCompositeResourceClaim(...): -want, +got:\n%s", diff)
	}
}

func TestSetCrdMetadata(t *testing.T) {
	type args struct {
		crd *extv1.CustomResourceDefinition
		xrd *v1.CompositeResourceDefinition
	}
	tests := map[string]struct {
		reason string
		args   args
		want   *extv1.CustomResourceDefinition
	}{
		"SetAnnotations": {
			reason: "Should set CRD annotations only from XRD spec",
			args: args{
				crd: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				xrd: &v1.CompositeResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Annotations: map[string]string{
							"example.com/some-xrd-annotation": "not-propagated",
						},
					},
					Spec: v1.CompositeResourceDefinitionSpec{Metadata: &v1.CompositeResourceDefinitionSpecMetadata{
						Annotations: map[string]string{
							"cert-manager.io/inject-ca-from": "example1-ns/webhook1-certificate",
						},
					}},
				},
			},
			want: &extv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						"cert-manager.io/inject-ca-from": "example1-ns/webhook1-certificate",
					},
				},
			},
		},
		"SetLabelsFromXRDSpec": {
			reason: "Should set CRD labels from XRD spec",
			args: args{
				crd: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				xrd: &v1.CompositeResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: v1.CompositeResourceDefinitionSpec{Metadata: &v1.CompositeResourceDefinitionSpecMetadata{
						Labels: map[string]string{
							"example.com/some-crd-label":            "value1",
							"example.com/some-additional-crd-label": "value2",
						},
					}},
				},
			},
			want: &extv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Labels: map[string]string{
						"example.com/some-crd-label":            "value1",
						"example.com/some-additional-crd-label": "value2",
					},
				},
			},
		},
		"AppendLabelsFromXRDSpec": {
			reason: "Should set CRD labels by appending labels from the XRD spec to the ones of the XRD itself",
			args: args{
				crd: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				xrd: &v1.CompositeResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							"example.com/some-xrd-label":            "value1",
							"example.com/some-additional-xrd-label": "value2",
						},
					},
					Spec: v1.CompositeResourceDefinitionSpec{Metadata: &v1.CompositeResourceDefinitionSpecMetadata{
						Labels: map[string]string{
							"example.com/some-crd-label":            "value3",
							"example.com/some-additional-crd-label": "value4",
						},
					}},
				},
			},
			want: &extv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Labels: map[string]string{
						"example.com/some-xrd-label":            "value1",
						"example.com/some-additional-xrd-label": "value2",
						"example.com/some-crd-label":            "value3",
						"example.com/some-additional-crd-label": "value4",
					},
				},
			},
		},
		"SetLabelsAndAnnotations": {
			reason: "Should set CRD labels and annotations from XRD spec and XRD itself",
			args: args{
				crd: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				xrd: &v1.CompositeResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Annotations: map[string]string{
							"example.com/some-xrd-annotation":                  "not-propagated",
							"example.com/some-additional-xrd-label-annotation": "not-propagated",
						},
						Labels: map[string]string{
							"example.com/some-xrd-label":            "value1",
							"example.com/some-additional-xrd-label": "value2",
						},
					},
					Spec: v1.CompositeResourceDefinitionSpec{Metadata: &v1.CompositeResourceDefinitionSpecMetadata{
						Annotations: map[string]string{
							"example.com/some-crd-annotation":                  "value1",
							"example.com/some-additional-crd-label-annotation": "value2",
						},
						Labels: map[string]string{
							"example.com/some-crd-label":            "value3",
							"example.com/some-additional-crd-label": "value4",
						},
					}},
				},
			},
			want: &extv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						"example.com/some-crd-annotation":                  "value1",
						"example.com/some-additional-crd-label-annotation": "value2",
					},
					Labels: map[string]string{
						"example.com/some-xrd-label":            "value1",
						"example.com/some-additional-xrd-label": "value2",
						"example.com/some-crd-label":            "value3",
						"example.com/some-additional-crd-label": "value4",
					},
				},
			},
		},
		"NoLabelsAndAnnotations": {
			reason: "Should do nothing if no annotations or labels are set in XRD spec or XRD itself",
			args: args{
				crd: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				xrd: &v1.CompositeResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			want: &extv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := setCrdMetadata(tt.args.crd, tt.args.xrd)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("\n%s\nsetCrdMetadata(...): -want, +got:\n%s", tt.reason, diff)
			}
		})
	}
}
