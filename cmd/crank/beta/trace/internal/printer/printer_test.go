// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package printer

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
)

// Returns an unstructured that has basic fields set to be used by other tests.
func DummyManifest(kind, name, namespace string, conds ...xpv1.Condition) unstructured.Unstructured {
	m := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "test.cloud/v1alpha1",
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"status": map[string]interface{}{
				"conditions": conds,
			},
		},
	}

	return m
}

func GetComplexResource() *resource.Resource {
	return &resource.Resource{
		Unstructured: DummyManifest("ObjectStorage", "test-resource", "default", xpv1.Condition{
			Type:   "Synced",
			Status: "True",
		}, xpv1.Condition{
			Type:   "Ready",
			Status: "True",
		}),
		Children: []*resource.Resource{
			{
				Unstructured: DummyManifest("XObjectStorage", "test-resource-hash", "", xpv1.Condition{
					Type:   "Synced",
					Status: "True",
				}, xpv1.Condition{
					Type:   "Ready",
					Status: "True",
				}),
				Children: []*resource.Resource{
					{
						Unstructured: DummyManifest("Bucket", "test-resource-bucket-hash", "", xpv1.Condition{
							Type:   "Synced",
							Status: "True",
						}, xpv1.Condition{
							Type:   "Ready",
							Status: "True",
						}),
						Children: []*resource.Resource{
							{
								Unstructured: DummyManifest("User", "test-resource-child-1-bucket-hash", "", xpv1.Condition{
									Type:   "Synced",
									Status: "True",
								}, xpv1.Condition{
									Type:    "Ready",
									Status:  "False",
									Reason:  "SomethingWrongHappened",
									Message: "Error with bucket child 1",
								}),
							},
							{
								Unstructured: DummyManifest("User", "test-resource-child-mid-bucket-hash", "", xpv1.Condition{
									Type:    "Synced",
									Status:  "False",
									Reason:  "CantSync",
									Message: "Sync error with bucket child mid",
								}, xpv1.Condition{
									Type:   "Ready",
									Status: "True",
									Reason: "AllGood",
								}),
							},
							{
								Unstructured: DummyManifest("User", "test-resource-child-2-bucket-hash", "", xpv1.Condition{
									Type:   "Synced",
									Status: "True",
								}, xpv1.Condition{
									Type:    "Ready",
									Reason:  "SomethingWrongHappened",
									Status:  "False",
									Message: "Error with bucket child 2",
								}),
								Children: []*resource.Resource{
									{
										Unstructured: DummyManifest("User", "test-resource-child-2-1-bucket-hash", "", xpv1.Condition{
											Type:   "Synced",
											Status: "True",
										}),
									},
								},
							},
						},
					},
					{
						Unstructured: DummyManifest("User", "test-resource-user-hash", "", xpv1.Condition{
							Type:   "Ready",
							Status: "True",
						}, xpv1.Condition{
							Type:   "Synced",
							Status: "Unknown",
						}),
					},
				},
			},
		},
	}
}
