package printer

import (
	"errors"
	"os"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/internal/k8s"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Define a test for SaveGraph
func TestSaveGraph(t *testing.T) {
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
		path     string
	}

	type want struct {
		fileExists bool
		err        error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		// Test valid resource
		"ResourceWithChildren": {
			reason: "Should created PNG file containing this structure: ObjectStorage -> XObjectStorage -> [Bucket, User]",
			args: args{
				resource: resourceWithChildren,
				fields:   []string{"parent", "name", "kind", "namespace", "apiversion", "synced", "ready", "message", "event"},
				path:     "graph.png",
			},
			want: want{
				fileExists: true,
				err:        nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			// Remove file in case it already exists
			os.Remove("graph.png")

			// Create a GraphPrinter with a buffer writer
			graphPrinter := &GraphPrinter{}
			err := graphPrinter.SaveGraph(tc.args.resource, tc.args.fields, tc.args.path)
			// Check error
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nExample(...): -want, +got:\n%s", tc.reason, diff)
			}

			// Check if png exists
			exists := true
			if _, err := os.Stat(tc.args.path); errors.Is(err, os.ErrNotExist) {
				exists = false
			}

			// Check if file exists
			if diff := cmp.Diff(tc.want.fileExists, exists); diff != "" {
				t.Errorf("%s\nExample(...): -want, +got:\n%s", tc.reason, diff)
			}

		})

	}
}
