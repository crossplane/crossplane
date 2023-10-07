package printer

import (
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/internal/k8s"
	"github.com/google/go-cmp/cmp"
	"github.com/olekukonko/tablewriter"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCliTable(t *testing.T) {
	resourceWithChildren := k8s.Resource{
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

	type args struct {
		resource k8s.Resource
		fields   []string
	}

	type want struct {
		output string
		err    error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		// Test valid resource
		"ResourceWithChildren": {
			reason: "CLI table should be able to print Resource struct containing children",
			args: args{
				resource: resourceWithChildren,
				fields:   []string{"parent", "name", "kind", "namespace", "apiversion", "synced", "ready", "message", "event"},
			},
			want: want{
				output: `
+----------------+--------------------------+----------------+-----------+---------------------+--------+-------+---------+--------------------------------+
|     PARENT     |           NAME           |      KIND      | NAMESPACE |     APIVERSION      | SYNCED | READY | MESSAGE |             EVENT              |
+----------------+--------------------------+----------------+-----------+---------------------+--------+-------+---------+--------------------------------+
|                | test-resource            | ObjectStorage  | default   | test.cloud/v1alpha1 | True   | True  |         | Successfully selected          |
|                |                          |                |           |                     |        |       |         | composition                    |
| ObjectStorage  | test-resource-cl4tv      | XObjectStorage | default   | test.cloud/v1alpha1 | True   | True  |         |                                |
| XObjectStorage | test-resource-cl4tv-123  | Bucket         | default   | test.cloud/v1alpha1 | True   | True  |         |                                |
| XObjectStorage | test-resource-user-cl4tv | User           | default   | test.cloud/v1alpha1 | True   | True  |         |                                |
+----------------+--------------------------+----------------+-----------+---------------------+--------+-------+---------+--------------------------------+
				`,
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a strings.Builder to capture the output
			tableString := &strings.Builder{}

			// Build new table
			table := tablewriter.NewWriter(tableString)
			table.SetHeader(tc.args.fields)
			err := CliTableAddResource(table, tc.args.fields, tc.args.resource, "")

			// Capture the output of the table
			table.Render()
			got := tableString.String()

			// Check error
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nExample(...): -want, +got:\n%s", tc.reason, diff)
			}
			// Check table
			if diff := cmp.Diff(strings.TrimSpace(tc.want.output), strings.TrimSpace(got)); diff != "" {
				t.Errorf("%s\nExample(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}

}
