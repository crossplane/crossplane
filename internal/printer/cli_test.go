package printer

import (
	"strings"
	"testing"

	"github.com/crossplane/crossplane/internal/k8s"
	"github.com/olekukonko/tablewriter"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCliTable(t *testing.T) {
	// Create a mockResource
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
				Children: []k8s.Resource{
					{
						Manifest: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"apiVersion": "test.cloud/v1alpha1",
								"kind":       "Bucket",
								"metadata": map[string]interface{}{
									"name":      "test-resource-cl4tv-123",
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
					{
						Manifest: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"apiVersion": "test.cloud/v1alpha1",
								"kind":       "User",
								"metadata": map[string]interface{}{
									"name":      "test-resource-user-cl4tv",
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
			},
		},
	}

	// Define the expected output
	expectedOutput := `
+----------------+--------------------------+----------------+-----------+---------------------+--------+-------+---------+--------------------------------+
|     PARENT     |           NAME           |      KIND      | NAMESPACE |     APIVERSION      | SYNCED | READY | MESSAGE |             EVENT              |
+----------------+--------------------------+----------------+-----------+---------------------+--------+-------+---------+--------------------------------+
|                | test-resource            | ObjectStorage  | default   | test.cloud/v1alpha1 | True   | True  |         | Successfully selected          |
|                |                          |                |           |                     |        |       |         | composition                    |
| ObjectStorage  | test-resource-cl4tv      | XObjectStorage | default   | test.cloud/v1alpha1 | True   | True  |         |                                |
| XObjectStorage | test-resource-cl4tv-123  | Bucket         | default   | test.cloud/v1alpha1 | True   | True  |         |                                |
| XObjectStorage | test-resource-user-cl4tv | User           | default   | test.cloud/v1alpha1 | True   | True  |         |                                |
+----------------+--------------------------+----------------+-----------+---------------------+--------+-------+---------+--------------------------------+
`

	// Define output fields
	fields := []string{"parent", "name", "kind", "namespace", "apiversion", "synced", "ready", "message", "event"}

	// Create a strings.Builder to capture the output
	tableString := &strings.Builder{}
	table := tablewriter.NewWriter(tableString)
	table.SetHeader(fields)
	err := CliTableAddResource(table, fields, mockResource, "")
	if err != nil {
		t.Errorf("CliTableAddResource returned an error: %v", err)
	}

	// Capture the output of the mockTable
	table.Render()
	mockTableOutputString := tableString.String()

	// Compare the expected output with the actual output
	if strings.TrimSpace(mockTableOutputString) != strings.TrimSpace(expectedOutput) {
		t.Errorf("CliTable output does not match expected output.\nExpected:\n%s\nActual:\n%s", expectedOutput, mockTableOutputString)
	}
}
