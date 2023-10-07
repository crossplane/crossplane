package k8s

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDiagnose(t *testing.T) {
	// Create a mockResource with "status: False" for "Ready" and "Synced" conditions
	mockResource1 := Resource{
		Manifest: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "test.cloud/v1alpha1",
				"kind":       "ObjectStorage",
				"metadata": map[string]interface{}{
					"name":      "test-resource-1",
					"namespace": "default",
				},
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"status": "False",
							"type":   "Synced",
						},
						map[string]interface{}{
							"status": "False",
							"type":   "Ready",
						},
					},
				},
			},
		},
		Event: "Synced and Ready are False for test-resource-1",
	}

	// Create another mockResource with "status: False" for "Ready" and "Synced" conditions
	mockResource2 := Resource{
		Manifest: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "test.cloud/v1alpha1",
				"kind":       "ObjectStorage",
				"metadata": map[string]interface{}{
					"name":      "test-resource-2",
					"namespace": "default",
				},
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"status": "False",
							"type":   "Synced",
						},
						map[string]interface{}{
							"status": "False",
							"type":   "Ready",
						},
					},
				},
			},
		},
		Event: "Synced and Ready are False for test-resource-2",
	}

}
