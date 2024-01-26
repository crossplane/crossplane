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

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
)

// DummyManifestOpt can be passed to customize a dummy manifest.
type DummyManifestOpt func(*unstructured.Unstructured)

// DummyManifest returns an unstructured that has basic fields set to be used by
// other tests, can be customized with DummyManifestOpt.
func DummyManifest(kind, name string, opts ...DummyManifestOpt) unstructured.Unstructured {
	m := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "test.cloud/v1alpha1",
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}

	for _, opt := range opts {
		opt(&m)
	}
	return m
}

// WithAPIVersion sets the APIVersion of the manifest.
func WithAPIVersion(apiVersion string) DummyManifestOpt {
	return func(m *unstructured.Unstructured) {
		m.SetAPIVersion(apiVersion)
	}
}

// WithNamespace sets the Namespace of the manifest.
func WithNamespace(namespace string) DummyManifestOpt {
	return func(m *unstructured.Unstructured) {
		m.SetNamespace(namespace)
	}
}

// WithConditions sets the given conditions on the manifest.
func WithConditions(conds ...xpv1.Condition) DummyManifestOpt {
	return func(m *unstructured.Unstructured) {
		fieldpath.Pave(m.Object).SetValue("status.conditions", conds)
	}
}

// WithImage sets the image of the manifest.
func WithImage(image string) DummyManifestOpt {
	return func(m *unstructured.Unstructured) {
		fieldpath.Pave(m.Object).SetValue("spec.image", image)
	}
}

// WithPackage sets the package of the manifest.
func WithPackage(pkg string) DummyManifestOpt {
	return func(m *unstructured.Unstructured) {
		fieldpath.Pave(m.Object).SetValue("spec.package", pkg)
	}
}

// WithDesiredState sets the desired state of the manifest.
func WithDesiredState(state v1.PackageRevisionDesiredState) DummyManifestOpt {
	return func(m *unstructured.Unstructured) {
		fieldpath.Pave(m.Object).SetValue("spec.desiredState", state)
	}
}

// DummyNamespacedMR returns an unstructured that has basic fields set to be used by other tests.
func DummyNamespacedMR(kind, name, namespace string, conds ...xpv1.Condition) unstructured.Unstructured {
	return DummyManifest(kind, name, WithConditions(conds...), WithNamespace(namespace))
}

// DummyPackage returns an unstructured that has basic fields set to be used by other tests.
func DummyPackage(gvk schema.GroupVersionKind, name string, opts ...DummyManifestOpt) unstructured.Unstructured {
	return DummyManifest(gvk.Kind, name, append([]DummyManifestOpt{WithAPIVersion(gvk.GroupVersion().String())}, opts...)...)
}

// GetComplexResource returns a complex resource with children.
func GetComplexResource() *resource.Resource {
	return &resource.Resource{
		Unstructured: DummyNamespacedMR("ObjectStorage", "test-resource", "default", xpv1.Condition{
			Type:   "Synced",
			Status: "True",
		}, xpv1.Condition{
			Type:   "Ready",
			Status: "True",
		}),
		Children: []*resource.Resource{
			{
				Unstructured: DummyNamespacedMR("XObjectStorage", "test-resource-hash", "", xpv1.Condition{
					Type:   "Synced",
					Status: "True",
				}, xpv1.Condition{
					Type:   "Ready",
					Status: "True",
				}),
				Children: []*resource.Resource{
					{
						Unstructured: DummyNamespacedMR("Bucket", "test-resource-bucket-hash", "", xpv1.Condition{
							Type:   "Synced",
							Status: "True",
						}, xpv1.Condition{
							Type:   "Ready",
							Status: "True",
						}),
						Children: []*resource.Resource{
							{
								Unstructured: DummyNamespacedMR("User", "test-resource-child-1-bucket-hash", "", xpv1.Condition{
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
								Unstructured: DummyNamespacedMR("User", "test-resource-child-mid-bucket-hash", "", xpv1.Condition{
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
								Unstructured: DummyNamespacedMR("User", "test-resource-child-2-bucket-hash", "", xpv1.Condition{
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
										Unstructured: DummyNamespacedMR("User", "test-resource-child-2-1-bucket-hash", "", xpv1.Condition{
											Type:   "Synced",
											Status: "True",
										}),
									},
								},
							},
						},
					},
					{
						Unstructured: DummyNamespacedMR("User", "test-resource-user-hash", "", xpv1.Condition{
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

// GetComplexPackage returns a complex package with children.
func GetComplexPackage() *resource.Resource {
	return &resource.Resource{
		Unstructured: DummyPackage(v1.ConfigurationGroupVersionKind, "platform-ref-aws",
			WithConditions(v1.Active(), v1.Healthy()),
			WithPackage("xpkg.upbound.io/upbound/platform-ref-aws:v0.9.0")),
		Children: []*resource.Resource{
			{
				Unstructured: DummyPackage(v1.ConfigurationRevisionGroupVersionKind, "platform-ref-aws-9ad7b5db2899",
					WithConditions(v1.Active(), v1.Healthy()),
					WithImage("xpkg.upbound.io/upbound/platform-ref-aws:v0.9.0"),
					WithDesiredState(v1.PackageRevisionActive)),
			},
			{
				Unstructured: DummyPackage(v1.ConfigurationGroupVersionKind, "upbound-configuration-aws-network upbound-configuration-aws-network",
					WithConditions(v1.Active(), v1.Healthy()),
					WithPackage("xpkg.upbound.io/upbound/configuration-aws-network:v0.7.0")),
				Children: []*resource.Resource{
					{
						Unstructured: DummyPackage(v1.ConfigurationRevisionGroupVersionKind, "upbound-configuration-aws-network-97be9100cfe1",
							WithConditions(v1.Active(), v1.Healthy()),
							WithImage("xpkg.upbound.io/upbound/configuration-aws-network:v0.7.0"),
							WithDesiredState(v1.PackageRevisionActive)),
					},
					{
						Unstructured: DummyPackage(v1.ProviderGroupVersionKind, "upbound-provider-aws-ec2",
							WithConditions(v1.Active(), v1.Unhealthy().WithMessage("post establish runtime hook failed for package: provider package deployment has no condition of type \"Available\" yet")),
							WithPackage("xpkg.upbound.io/upbound/provider-aws-ec2:v0.47.0"),
							WithDesiredState(v1.PackageRevisionActive),
						),
						Children: []*resource.Resource{
							{
								Unstructured: DummyPackage(v1.ProviderRevisionGroupVersionKind, "upbound-provider-aws-ec2-9ad7b5db2899",
									WithConditions(v1.Active(), v1.Unhealthy().WithMessage("post establish runtime hook failed for package: provider package deployment has no condition of type \"Available\" yet")),
									WithImage("xpkg.upbound.io/upbound/provider-aws-ec2:v0.47.0"),
									WithDesiredState(v1.PackageRevisionActive)),
							},
						},
					},
				},
			},
		},
	}
}
