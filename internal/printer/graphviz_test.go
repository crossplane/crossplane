package printer

import (
	"errors"
	"os"
	"testing"

	"github.com/crossplane/crossplane/internal/k8s"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Define a test for SaveGraph
func TestSaveGraph(t *testing.T) {
	// Remove file in case it already exists
	os.Remove("graph.png")
	// Create a mock Resource
	mockResource := k8s.Resource{
		Manifest: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "test.cloud/v1alpha1",
				"kind":       "ObjectStorage",
				"metadata": map[string]interface{}{
					"name":      "test-resource",
					"namespace": "default",
				},
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"status": "True",
							"type":   "Synced",
						},
						map[string]interface{}{
							"status": "True",
							"type":   "Ready",
						},
					},
				},
			},
		},
		Event: "Successfully selected composition",
		Children: []k8s.Resource{
			{
				Manifest: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "test.cloud/v1alpha1",
						"kind":       "XObjectStorage",
						"metadata": map[string]interface{}{
							"name":      "test-resource-cl4tv",
							"namespace": "default",
						},
						"status": map[string]interface{}{
							"conditions": []interface{}{
								map[string]interface{}{
									"status": "True",
									"type":   "Synced",
								},
								map[string]interface{}{
									"status": "True",
									"type":   "Ready",
								},
							},
						},
					},
				},
			},
		},
	}

	// Create a GraphPrinter with a buffer writer
	graphPrinter := &GraphPrinter{}

	// Define fields for labeling
	fields := []string{"name", "kind", "namespace", "apiversion", "synced", "ready", "event"}

	// Call the SaveGraph function
	err := graphPrinter.SaveGraph(mockResource, fields, "graph.png")
	if err != nil {
		t.Errorf("SaveGraph returned an error: %v", err)
	}

	// Check if png exists
	if _, err := os.Stat("graph.png"); errors.Is(err, os.ErrNotExist) {
		t.Errorf("graph.png was not created: %v", err)
	}

}
