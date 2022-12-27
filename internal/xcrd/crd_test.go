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
	"testing"

	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
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
	name := "coolcomposites.example.org"
	labels := map[string]string{"cool": "very"}
	annotations := map[string]string{"example.org/cool": "very"}

	group := "example.org"
	version := "v1"
	kind := "CoolComposite"
	listKind := "CoolCompositeList"
	singular := "coolcomposite"
	plural := "coolcomposites"

	schema := `
{
  "required": [
    "spec"
  ],
  "properties": {
    "spec": {
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
          "type": "integer"
        }
      },
      "type": "object"
    },
    "status": {
      "properties": {
        "phase": {
          "type": "string"
        }
      },
      "type": "object"
    }
  },
  "type": "object"
}`

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
								Type:     "object",
								Required: []string{"storageGB", "engineVersion"},
								Properties: map[string]extv1.JSONSchemaProps{
									// From CRDSpecTemplate.Validation
									"storageGB": {Type: "integer"},
									"engineVersion": {
										Type: "string",
										Enum: []extv1.JSON{
											{Raw: []byte(`"5.6"`)},
											{Raw: []byte(`"5.7"`)},
										},
									},

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
										Default: &extv1.JSON{Raw: []byte(`"Automatic"`)},
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
								Type: "object",
								Properties: map[string]extv1.JSONSchemaProps{
									"phase": {Type: "string"},

									// From CompositeResourceStatusProps()
									"conditions": {
										Description: "Conditions of the resource.",
										Type:        "array",
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
	}

	got, err := ForCompositeResource(d)
	if err != nil {
		t.Fatalf("ForCompositeResource(...): %s", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ForCompositeResource(...): -want, +got:\n%s", diff)
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

	schema := `
{
	"properties": {
		"spec": {
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
					"type": "integer"
				}
			},
			"type": "object"
		},
		"status": {
      "properties": {
        "phase": {
          "type": "string"
        }
      },
      "type": "object"
    }
	},
	"type": "object"
}`

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
									Type:     "object",
									Required: []string{"storageGB", "engineVersion"},
									Properties: map[string]extv1.JSONSchemaProps{
										// From CRDSpecTemplate.Validation
										"storageGB": {Type: "integer"},
										"engineVersion": {
											Type: "string",
											Enum: []extv1.JSON{
												{Raw: []byte(`"5.6"`)},
												{Raw: []byte(`"5.7"`)},
											},
										},
										"compositeDeletePolicy": {
											Type:    "string",
											Default: &extv1.JSON{Raw: []byte(`"Background"`)},
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
											Default: &extv1.JSON{Raw: []byte(`"Automatic"`)},
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
									Type: "object",
									Properties: map[string]extv1.JSONSchemaProps{
										"phase": {Type: "string"},

										// From CompositeResourceStatusProps()
										"conditions": {
											Description: "Conditions of the resource.",
											Type:        "array",
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
