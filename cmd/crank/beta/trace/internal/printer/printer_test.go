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

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/cmd/crank/common/resource"
	"github.com/crossplane/crossplane/v2/internal/xcrd"
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

func WithCompositionResourceName(n string) DummyManifestOpt {
	return func(m *unstructured.Unstructured) {
		meta.AddAnnotations(m, map[string]string{xcrd.AnnotationKeyCompositionResourceName: n})
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

// DummyNamespacedResource returns an unstructured that has basic fields set to be used by other tests.
func DummyNamespacedResource(kind, name, namespace string, conds ...xpv1.Condition) unstructured.Unstructured {
	return DummyManifest(kind, name, WithConditions(conds...), WithNamespace(namespace))
}

func DummyClusterScopedResource(kind, name string, conds ...xpv1.Condition) unstructured.Unstructured {
	return DummyManifest(kind, name, WithConditions(conds...))
}

func DummyComposedResource(kind, name, resourceName string, conds ...xpv1.Condition) unstructured.Unstructured {
	return DummyManifest(kind, name, WithConditions(conds...), WithCompositionResourceName(resourceName))
}

// DummyPackage returns an unstructured that has basic fields set to be used by other tests.
func DummyPackage(gvk schema.GroupVersionKind, name string, opts ...DummyManifestOpt) unstructured.Unstructured {
	return DummyManifest(gvk.Kind, name, append([]DummyManifestOpt{WithAPIVersion(gvk.GroupVersion().String())}, opts...)...)
}

// GetComplexResource returns a complex resource with children.
func GetComplexResource() *resource.Resource {
	return &resource.Resource{
		Unstructured: DummyNamespacedResource("ObjectStorage", "test-resource", "default", xpv1.Condition{
			Type:   "Synced",
			Status: "True",
		}, xpv1.Condition{
			Type:   "Ready",
			Status: "True",
		}),
		Children: []*resource.Resource{
			{
				Unstructured: DummyClusterScopedResource("XObjectStorage", "test-resource-hash", xpv1.Condition{
					Type:   "Synced",
					Status: "True",
				}, xpv1.Condition{
					Type:   "Ready",
					Status: "True",
				}),
				Children: []*resource.Resource{
					{
						Unstructured: DummyComposedResource("Bucket", "test-resource-bucket-hash", "one", xpv1.Condition{
							Type:   "Synced",
							Status: "True",
						}, xpv1.Condition{
							Type:   "Ready",
							Status: "True",
						}),
						Children: []*resource.Resource{
							{
								Unstructured: DummyComposedResource("User", "test-resource-child-1-bucket-hash", "two", xpv1.Condition{
									Type:   "Synced",
									Status: "True",
								}, xpv1.Condition{
									Type:    "Ready",
									Status:  "False",
									Reason:  "SomethingWrongHappened",
									Message: "Error with bucket child 1: Sint eu mollit tempor ad minim do commodo irure. Magna labore irure magna. Non cillum id nulla. Anim culpa do duis consectetur.",
								}),
							},
							{
								Unstructured: DummyComposedResource("User", "test-resource-child-mid-bucket-hash", "three", xpv1.Condition{
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
								Unstructured: DummyComposedResource("User", "test-resource-child-2-bucket-hash", "four", xpv1.Condition{
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
										Unstructured: DummyComposedResource("User", "test-resource-child-2-1-bucket-hash", "", xpv1.Condition{
											Type:   "Synced",
											Status: "True",
										}),
									},
								},
							},
						},
					},
					{
						Unstructured: DummyClusterScopedResource("User", "test-resource-user-hash", xpv1.Condition{
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
			WithPackage("xpkg.crossplane.io/crossplane/platform-ref-aws:v0.9.0")),
		Children: []*resource.Resource{
			{
				Unstructured: DummyPackage(v1.ConfigurationRevisionGroupVersionKind, "platform-ref-aws-9ad7b5db2899",
					WithConditions(v1.Active(), v1.Healthy()),
					WithImage("xpkg.crossplane.io/crossplane/platform-ref-aws:v0.9.0"),
					WithDesiredState(v1.PackageRevisionActive)),
			},
			{
				Unstructured: DummyPackage(v1.ConfigurationGroupVersionKind, "crossplane-configuration-aws-network",
					WithConditions(v1.Active(), v1.Healthy()),
					WithPackage("xpkg.crossplane.io/crossplane/configuration-aws-network:v0.7.0")),
				Children: []*resource.Resource{
					{
						Unstructured: DummyPackage(v1.ConfigurationRevisionGroupVersionKind, "crossplane-configuration-aws-network-97be9100cfe1",
							WithConditions(v1.Active(), v1.Healthy()),
							WithImage("xpkg.crossplane.io/crossplane/configuration-aws-network:v0.7.0"),
							WithDesiredState(v1.PackageRevisionActive)),
					},
					{
						Unstructured: DummyPackage(v1.ProviderGroupVersionKind, "crossplane-provider-aws-ec2",
							WithConditions(v1.Active(), v1.UnknownHealth().WithMessage("cannot resolve package dependencies: incompatible dependencies: [xpkg.crossplane.io/crossplane-contrib/provider-helm xpkg.crossplane.io/crossplane-contrib/provider-kubernetes]")),
							WithPackage("xpkg.crossplane.io/crossplane/provider-aws-ec2:v0.47.0"),
						),
						Children: []*resource.Resource{
							{
								Unstructured: DummyPackage(v1.ProviderRevisionGroupVersionKind, "crossplane-provider-aws-ec2-9ad7b5db2899",
									WithConditions(v1.Active(), v1.Unhealthy().WithMessage("post establish runtime hook failed for package: provider package deployment has no condition of type \"Available\" yet")),
									WithImage("xpkg.crossplane.io/crossplane/provider-aws-ec2:v0.47.0"),
									WithDesiredState(v1.PackageRevisionActive)),
							},
							{
								Unstructured: DummyPackage(v1.ProviderGroupVersionKind, "crossplane-provider-aws-something",
									WithConditions(v1.Active()), // Missing healthy condition on purpose.
									WithPackage("xpkg.crossplane.io/crossplane/provider-aws-something:v0.47.0"),
								),
							},
						},
					},
				},
			},
		},
	}
}
