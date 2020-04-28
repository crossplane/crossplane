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

// Package ccrd generates CustomResourceDefinitions from Crossplane definitions.
//
// v1beta1.JSONSchemaProps is incompatible with controller-tools (as of 0.2.4)
// because it is missing JSON tags and uses float64, which is a disallowed type.
// We thus copy the entire struct as CRDSpecTemplate. See the below issue:
// https://github.com/kubernetes-sigs/controller-tools/issues/291
package ccrd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

func TestIsEstablished(t *testing.T) {
	cases := map[string]struct {
		s    v1beta1.CustomResourceDefinitionStatus
		want bool
	}{
		"IsEstablished": {
			s: v1beta1.CustomResourceDefinitionStatus{
				Conditions: []v1beta1.CustomResourceDefinitionCondition{{
					Type:   v1beta1.Established,
					Status: v1beta1.ConditionTrue,
				}},
			},
			want: true,
		},
		"IsNotEstablished": {
			s:    v1beta1.CustomResourceDefinitionStatus{},
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

func TestForInfrastructureDefinition(t *testing.T) {
	name := "coolcomposites.example.org"
	labels := map[string]string{"cool": "very"}
	annotations := map[string]string{"example.org/cool": "very"}

	group := "example.org"
	version := "v1alpha1"
	kind := "CoolComposite"
	listKind := "CoolCompositeList"
	singular := "coolcomposite"
	plural := "coolcomposites"

	schema := `{"properties":{"spec":{"properties":{"engineVersion":{"enum":["5.6","5.7"],"type":"string"},"storageGB":{"type":"integer"}},"type":"object"}},"type":"object"}`

	d := &v1alpha1.InfrastructureDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
			UID:         types.UID("you-you-eye-dee"),
		},
		Spec: v1alpha1.InfrastructureDefinitionSpec{
			CRDSpecTemplate: v1alpha1.CRDSpecTemplate{
				Group:   group,
				Version: version,
				Names: v1beta1.CustomResourceDefinitionNames{
					Plural:   plural,
					Singular: singular,
					Kind:     kind,
					ListKind: listKind,
				},
				Validation: &v1alpha1.CustomResourceValidation{
					OpenAPIV3Schema: runtime.RawExtension{Raw: []byte(schema)},
				},
			},
		},
	}

	want := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				meta.AsController(meta.ReferenceTo(d, v1alpha1.InfrastructureDefinitionGroupVersionKind)),
			},
		},
		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:   group,
			Version: version,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural:   plural,
				Singular: singular,
				Kind:     kind,
				ListKind: listKind,
			},
			Scope:                 v1beta1.ClusterScoped,
			PreserveUnknownFields: pointer.BoolPtr(false),
			Subresources: &v1beta1.CustomResourceSubresources{
				Status: &v1beta1.CustomResourceSubresourceStatus{},
			},
			Validation: &v1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &v1beta1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]v1beta1.JSONSchemaProps{
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
							Type: "object",
							Properties: map[string]v1beta1.JSONSchemaProps{
								// From CRDSpecTemplate.Validation
								"storageGB": {Type: "integer"},
								"engineVersion": {
									Type: "string",
									Enum: []v1beta1.JSON{
										{Raw: []byte(`"5.6"`)},
										{Raw: []byte(`"5.7"`)},
									},
								},

								// From DefinedInfrastructureSpecProps()
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
							},
						},
						"status": {
							Type: "object",
							Properties: map[string]v1beta1.JSONSchemaProps{

								// From InfrastructureStatusProps()
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
							},
						},
					},
				},
			},
		},
	}

	got, err := New(ForInfrastructureDefinition(d))
	if err != nil {
		t.Fatalf("New(ForInfrastructureDefinition(...): %s", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("New(ForInfrastructureDefinition(...): -want, +got:\n%s", diff)
	}
}

func TestPublishesInfrastructureDefinition(t *testing.T) {
	name := "coolcomposites.example.org"
	labels := map[string]string{"cool": "very"}
	annotations := map[string]string{"example.org/cool": "very"}

	group := "example.org"
	version := "v1alpha1"
	kind := "CoolComposite"
	listKind := "CoolCompositeList"
	singular := "coolcomposite"
	plural := "coolcomposites"

	schema := `{"properties":{"spec":{"properties":{"engineVersion":{"enum":["5.6","5.7"],"type":"string"},"storageGB":{"type":"integer"}},"type":"object"}},"type":"object"}`

	p := &v1alpha1.InfrastructurePublication{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
			UID:         types.UID("you-you-eye-dee"),
		},
	}
	d := &v1alpha1.InfrastructureDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
			UID:         types.UID("you-you-eye-dee"),
		},
		Spec: v1alpha1.InfrastructureDefinitionSpec{
			CRDSpecTemplate: v1alpha1.CRDSpecTemplate{
				Group:   group,
				Version: version,
				Names: v1beta1.CustomResourceDefinitionNames{
					Plural:   plural,
					Singular: singular,
					Kind:     kind,
					ListKind: listKind,
				},
				Validation: &v1alpha1.CustomResourceValidation{
					OpenAPIV3Schema: runtime.RawExtension{Raw: []byte(schema)},
				},
			},
		},
	}

	want := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:        singular + PublishedInfrastructureSuffixPlural + "." + group,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				meta.AsController(meta.ReferenceTo(d, v1alpha1.InfrastructureDefinitionGroupVersionKind)),
			},
		},
		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:   group,
			Version: version,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural:   singular + PublishedInfrastructureSuffixPlural,
				Singular: singular + PublishedInfrastructureSuffixSingular,
				Kind:     kind + PublishedInfrastructureSuffixKind,
				ListKind: kind + PublishedInfrastructureSuffixListKind,
			},
			Scope:                 v1beta1.NamespaceScoped,
			PreserveUnknownFields: pointer.BoolPtr(false),
			Subresources: &v1beta1.CustomResourceSubresources{
				Status: &v1beta1.CustomResourceSubresourceStatus{},
			},
			Validation: &v1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &v1beta1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]v1beta1.JSONSchemaProps{
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
							Type: "object",
							Properties: map[string]v1beta1.JSONSchemaProps{
								// From CRDSpecTemplate.Validation
								"storageGB": {Type: "integer"},
								"engineVersion": {
									Type: "string",
									Enum: []v1beta1.JSON{
										{Raw: []byte(`"5.6"`)},
										{Raw: []byte(`"5.7"`)},
									},
								},

								// From PublishedInfrastructureSpecProps()
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
							},
						},
						"status": {
							Type: "object",
							Properties: map[string]v1beta1.JSONSchemaProps{

								// From InfrastructureStatusProps()
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
							},
						},
					},
				},
			},
		},
	}

	got, err := New(PublishesInfrastructureDefinition(d, p))
	if err != nil {
		t.Fatalf("New(PublishesInfrastructureDefinition(...): %s", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("New(PublishesInfrastructureDefinition(...): -want, +got:\n%s", diff)
	}
}
