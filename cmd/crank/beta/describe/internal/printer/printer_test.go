/*
Copyright 2023 The Crossplane Authors.

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

package printer

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// Returns an unstructured that has basic fields set to be used by other tests.
func DummyManifest(kind, name, syncedStatus, readyStatus string) unstructured.Unstructured {
	m := unstructured.Unstructured{
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
