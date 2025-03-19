package diff

import (
	"encoding/json"
	"fmt"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// Testing data for integration tests

// createTestCompositionWithExtraResources creates a test Composition with a function-extra-resources step
func createTestCompositionWithExtraResources() *apiextensionsv1.Composition {
	pipelineMode := apiextensionsv1.CompositionModePipeline

	// Create the extra resources function input
	extraResourcesInput := map[string]interface{}{
		"apiVersion": "function.crossplane.io/v1beta1",
		"kind":       "ExtraResources",
		"spec": map[string]interface{}{
			"extraResources": []interface{}{
				map[string]interface{}{
					"apiVersion": "example.org/v1",
					"kind":       "ExtraResource",
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "test-app",
						},
					},
				},
			},
		},
	}

	extraResourcesRaw, _ := json.Marshal(extraResourcesInput)

	// Create template function input to create composed resources
	templateInput := map[string]interface{}{
		"apiVersion": "apiextensions.crossplane.io/v1",
		"kind":       "Composition",
		"spec": map[string]interface{}{
			"resources": []interface{}{
				map[string]interface{}{
					"name": "composed-resource",
					"base": map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ComposedResource",
						"metadata": map[string]interface{}{
							"name": "test-composed-resource",
							"labels": map[string]interface{}{
								"app": "crossplane",
							},
						},
						"spec": map[string]interface{}{
							"coolParam": "{{ .observed.composite.spec.coolParam }}",
							"replicas":  "{{ .observed.composite.spec.replicas }}",
							"extraData": "{{ index .observed.resources \"extra-resource-0\" \"spec\" \"data\" }}",
						},
					},
				},
			},
		},
	}

	templateRaw, _ := json.Marshal(templateInput)

	return &apiextensionsv1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-composition",
		},
		Spec: apiextensionsv1.CompositionSpec{
			CompositeTypeRef: apiextensionsv1.TypeReference{
				APIVersion: "example.org/v1",
				Kind:       "XExampleResource",
			},
			Mode: &pipelineMode,
			Pipeline: []apiextensionsv1.PipelineStep{
				{
					Step:        "extra-resources",
					FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
					Input:       &runtime.RawExtension{Raw: extraResourcesRaw},
				},
				{
					Step:        "templating",
					FunctionRef: apiextensionsv1.FunctionReference{Name: "function-patch-and-transform"},
					Input:       &runtime.RawExtension{Raw: templateRaw},
				},
			},
		},
	}
}

// createTestXRD creates a test XRD for the XR
func createTestXRD() *apiextensionsv1.CompositeResourceDefinition {
	return &apiextensionsv1.CompositeResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "xexampleresources.example.org",
		},
		Spec: apiextensionsv1.CompositeResourceDefinitionSpec{
			Group: "example.org",
			Names: extv1.CustomResourceDefinitionNames{
				Kind:     "XExampleResource",
				Plural:   "xexampleresources",
				Singular: "xexampleresource",
			},
			Versions: []apiextensionsv1.CompositeResourceDefinitionVersion{
				{
					Name:          "v1",
					Served:        true,
					Referenceable: true,
					Schema: &apiextensionsv1.CompositeResourceValidation{
						OpenAPIV3Schema: runtime.RawExtension{
							Raw: []byte(`{
								"type": "object",
								"properties": {
									"spec": {
										"type": "object",
										"properties": {
											"coolParam": {
												"type": "string"
											},
											"replicas": {
												"type": "integer"
											}
										}
									},
									"status": {
										"type": "object",
										"properties": {
											"coolStatus": {
												"type": "string"
											}
										}
									}
								}
							}`),
						},
					},
				},
			},
		},
	}
}

// createExtraResource creates a test extra resource
func createExtraResource() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.org/v1",
			"kind":       "ExtraResource",
			"metadata": map[string]interface{}{
				"name": "test-extra-resource",
				"labels": map[string]interface{}{
					"app": "test-app",
				},
			},
			"spec": map[string]interface{}{
				"data": "extra-resource-data",
			},
		},
	}
}

// createExistingComposedResource creates an existing composed resource with different values
func createExistingComposedResource() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.org/v1",
			"kind":       "ComposedResource",
			"metadata": map[string]interface{}{
				"name": "test-xr-composed-resource",
				"labels": map[string]interface{}{
					"app":                     "crossplane",
					"crossplane.io/composite": "test-xr",
				},
				"annotations": map[string]interface{}{
					"crossplane.io/composition-resource-name": "composed-resource",
				},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "example.org/v1",
						"kind":               "XExampleResource",
						"name":               "test-xr",
						"controller":         true,
						"blockOwnerDeletion": true,
					},
				},
			},
			"spec": map[string]interface{}{
				"coolParam": "old-value", // Different from what will be rendered
				"replicas":  2,           // Different from what will be rendered
				"extraData": "old-data",  // Different from what will be rendered
			},
		},
	}
}

// createMatchingComposedResource creates a composed resource that matches what would be rendered
func createMatchingComposedResource() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.org/v1",
			"kind":       "ComposedResource",
			"metadata": map[string]interface{}{
				"name": "test-xr-composed-resource",
				"labels": map[string]interface{}{
					"app":                     "crossplane",
					"crossplane.io/composite": "test-xr",
				},
				"annotations": map[string]interface{}{
					"crossplane.io/composition-resource-name": "composed-resource",
				},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "example.org/v1",
						"kind":               "XExampleResource",
						"name":               "test-xr",
						"controller":         true,
						"blockOwnerDeletion": true,
					},
				},
			},
			"spec": map[string]interface{}{
				"coolParam": "test-value",          // Matches what would be rendered
				"replicas":  3,                     // Matches what would be rendered
				"extraData": "extra-resource-data", // Matches what would be rendered
			},
		},
	}
}

// Define a var for fprintf to allow test overriding
var fprintf = fmt.Fprintf
