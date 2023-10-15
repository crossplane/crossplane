package printer

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// Returns an unstructured that has basic fields set to test the k8s package. Used to create manifests for resources in tests.
func DummyManifest(kind, name, syncedStatus, readyStatus string) *unstructured.Unstructured {
	m := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "test.cloud/v1alpha1",
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": "default",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"status": syncedStatus,
						"type":   "Synced",
					},
					map[string]interface{}{
						"status": readyStatus,
						"type":   "Ready",
					},
				},
			},
		},
	}

	return m
}
