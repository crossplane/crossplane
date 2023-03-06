package validation

import (
	"context"
	"encoding/json"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"strings"
	"testing"
)

func TestClientValidator_ValidateCreate(t *testing.T) {
	type args struct {
		obj          runtime.Object
		existingObjs []runtime.Object
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "Should accept a Composition if validation mode is loose and no CRDs are found",
			wantErr: false,
			args: args{
				obj:          buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil),
				existingObjs: nil,
			},
		}, {
			name:    "Should reject a Composition if validation mode is strict and no CRDs are found",
			wantErr: true,
			args: args{
				obj:          buildDefaultComposition(t, v1.CompositionValidationModeStrict, map[string]any{"someOtherField": "test"}),
				existingObjs: nil,
			},
		}, {
			name:    "Should accept a valid Composition if validation mode is strict and all CRDs are found",
			wantErr: false,
			args: args{
				existingObjs: defaultCRDs(),
				obj:          buildDefaultComposition(t, v1.CompositionValidationModeStrict, map[string]any{"someOtherField": "test"}),
			},
		}, {
			name:    "Should reject a Composition not defining a required field in a resource if validation mode is strict and all CRDs are found",
			wantErr: true,
			args: args{
				existingObjs: defaultCRDs(),
				obj:          buildDefaultComposition(t, v1.CompositionValidationModeStrict, nil),
			},
		}, {
			name:    "Should accept a Composition with a required field defined only by a patch if validation mode is strict and all CRDs are found",
			wantErr: false,
			args: args{
				existingObjs: defaultCRDs(),
				obj: buildDefaultComposition(t, v1.CompositionValidationModeStrict, nil, v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: toPointer("spec.someField"),
					ToFieldPath:   toPointer("spec.someOtherField"),
				}),
			},
		}, {
			name:    "Should reject a Composition with a patch using a field not allowed by the the Composite resource, if validation mode is strict and all CRDs are found",
			wantErr: true,
			args: args{
				existingObjs: defaultCRDs(),
				obj: buildDefaultComposition(t, v1.CompositionValidationModeStrict, nil, v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: toPointer("spec.someWrongField"),
					ToFieldPath:   toPointer("spec.someOtherField"),
				}),
			},
		}, {
			name:    "Should reject a Composition with a patch using a field not allowed by the schema of the Managed resource, if validation mode is strict and all CRDs are found",
			wantErr: true,
			args: args{
				existingObjs: defaultCRDs(),
				obj: buildDefaultComposition(t, v1.CompositionValidationModeStrict, map[string]any{"someOtherField": "test"}, v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: toPointer("spec.someField"),
					ToFieldPath:   toPointer("spec.someOtherWrongField"),
				}),
			},
		}, {
			name:    "Should reject a Composition with a patch between two different types, if validation mode is strict and all CRDs are found",
			wantErr: true,
			args: args{
				existingObjs: []runtime.Object{
					defaultCompositeCrdBuilder().withOption(func(crd *extv1.CustomResourceDefinition) {
						crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["someField"] = extv1.JSONSchemaProps{
							Type: "integer",
						}
					}).build(),
					defaultManagedCrdBuilder().build(),
				},
				obj: buildDefaultComposition(t, v1.CompositionValidationModeStrict, nil, v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: toPointer("spec.someField"),
					ToFieldPath:   toPointer("spec.someOtherField"),
				}),
			},
		}, {
			name:    "Should accept a Composition with valid patches, if validation mode is loose and only the Managed resource CRDs are found",
			wantErr: false,
			args: args{
				existingObjs: []runtime.Object{
					defaultManagedCrdBuilder().withOption(func(crd *extv1.CustomResourceDefinition) {
						crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["someOtherField"] = extv1.JSONSchemaProps{
							Type: "integer",
						}
					}).build(),
				},
				obj: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: toPointer("spec.someWrongField"),
					ToFieldPath:   toPointer("spec.someOtherField"),
				}),
			},
		}, {
			name:    "Should reject a Composition with an invalid patch due to a wrong field from a Managed resource, if validation mode is loose and only the Managed resource CRDs are found",
			wantErr: true,
			args: args{
				existingObjs: []runtime.Object{
					defaultManagedCrdBuilder().build(),
				},
				obj: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: toPointer("spec.someWrongField"),
					ToFieldPath:   toPointer("spec.someOtherWrongField"),
				}),
			},
		}, {
			name:    "Should reject a Composition with an invalid patch due to a wrong field from a Composite resource, if validation mode is loose and only the Composed resource CRDs are found",
			wantErr: true,
			args: args{
				existingObjs: []runtime.Object{
					defaultCompositeCrdBuilder().build(),
				},
				obj: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: toPointer("spec.someWrongField"),
					ToFieldPath:   toPointer("spec.someOtherWrongField"),
				}),
			},
		}, {
			name:    "Should accept a Composition with an invalid patch, if validation mode is loose and no CRDs are found",
			wantErr: false,
			args: args{
				existingObjs: nil,
				obj: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: toPointer("spec.someWrongField"),
					ToFieldPath:   toPointer("spec.someOtherWrongField"),
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ClientValidator{
				client: fake.NewClientBuilder().
					WithScheme(func() *runtime.Scheme {
						s := runtime.NewScheme()
						_ = extv1.AddToScheme(s)
						return s
					}()).
					WithIndex(&extv1.CustomResourceDefinition{}, "spec.group", func(object client.Object) []string {
						return []string{object.(*extv1.CustomResourceDefinition).Spec.Group}
					}).WithIndex(&extv1.CustomResourceDefinition{}, "spec.names.kind", func(object client.Object) []string {
					return []string{object.(*extv1.CustomResourceDefinition).Spec.Names.Kind}
				}).WithRuntimeObjects(tt.args.existingObjs...).Build(),
				renderValidator: NewPureValidator(),
			}
			if err := c.ValidateCreate(context.TODO(), tt.args.obj); (err != nil) != tt.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

const testGroup = "resources.test.com"
const testGroupSingular = "resource.test.com"

func marshalJSON(t *testing.T, obj interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(obj)
	if err != nil {
		t.Errorf("Failed to marshal object: %v", err)
	}
	return b
}

func toPointer[T any](v T) *T {
	return &v
}

func defaultCompositeCrdBuilder() *crdBuilder {
	return newCRDBuilder("Composite", "v1").withOption(specSchemaOption("v1", extv1.JSONSchemaProps{
		Type: "object",
		Required: []string{
			"someField",
		},
		Properties: map[string]extv1.JSONSchemaProps{
			"someField": {
				Type: "string",
			},
		},
	}))
}

func defaultManagedCrdBuilder() *crdBuilder {
	return newCRDBuilder("Managed", "v1").withOption(specSchemaOption("v1", extv1.JSONSchemaProps{
		Type: "object",
		Required: []string{
			"someOtherField",
		},
		Properties: map[string]extv1.JSONSchemaProps{
			"someOtherField": {
				Type: "string",
			},
		},
	}))
}

func defaultCRDs() []runtime.Object {
	return []runtime.Object{
		defaultCompositeCrdBuilder().build(),
		defaultManagedCrdBuilder().build(),
	}
}

type builderOption func(*extv1.CustomResourceDefinition)

type crdBuilder struct {
	kind, version string
	opts          []builderOption
}

func newCRDBuilder(kind, version string) *crdBuilder {
	return &crdBuilder{kind: kind, version: version}
}

func specSchemaOption(version string, schema extv1.JSONSchemaProps) builderOption {
	return func(crd *extv1.CustomResourceDefinition) {
		var storageFound bool
		for i, definitionVersion := range crd.Spec.Versions {
			storageFound = storageFound || definitionVersion.Storage
			if definitionVersion.Name == version {
				crd.Spec.Versions[i].Schema = &extv1.CustomResourceValidation{
					OpenAPIV3Schema: &extv1.JSONSchemaProps{
						Type: "object",
						Required: []string{
							"spec",
						},
						Properties: map[string]extv1.JSONSchemaProps{
							"spec": schema,
						},
					},
				}
				return
			}
		}
		crd.Spec.Versions = append(crd.Spec.Versions, extv1.CustomResourceDefinitionVersion{
			Name:    version,
			Served:  true,
			Storage: !storageFound,
			Schema: &extv1.CustomResourceValidation{
				OpenAPIV3Schema: &extv1.JSONSchemaProps{
					Type: "object",
					Required: []string{
						"spec",
					},
					Properties: map[string]extv1.JSONSchemaProps{
						"spec": schema,
					},
				},
			},
		})
	}
}

func (b *crdBuilder) withOption(f builderOption) *crdBuilder {
	b.opts = append(b.opts, f)
	return b
}

func (b *crdBuilder) build() *extv1.CustomResourceDefinition {
	crd := &extv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.ToLower(b.kind) + "s." + testGroupSingular,
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: testGroup,
			Names: extv1.CustomResourceDefinitionNames{
				Kind: b.kind,
			},
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    b.version,
					Served:  true,
					Storage: true,
				},
			},
		},
	}
	for _, opt := range b.opts {
		opt(crd)
	}
	return crd
}

func buildDefaultComposition(t *testing.T, validationMode v1.CompositionValidationMode, spec map[string]any, patches ...v1.Patch) *v1.Composition {
	t.Helper()
	if spec == nil {
		spec = map[string]any{}
	}
	return &v1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			Annotations: map[string]string{
				v1.CompositionValidationModeAnnotation: string(validationMode),
			},
		},
		Spec: v1.CompositionSpec{
			CompositeTypeRef: v1.TypeReference{
				APIVersion: testGroup + "/v1",
				Kind:       "Composite",
			},
			Resources: []v1.ComposedTemplate{
				{
					Name: toPointer("test"),
					Base: runtime.RawExtension{
						Raw: marshalJSON(t, map[string]any{
							"apiVersion": testGroup + "/v1",
							"kind":       "Managed",
							"metadata": map[string]any{
								"name":      "test",
								"namespace": "testns",
							},
							"spec": spec,
						}),
					},
					Patches: patches,
				},
			},
		},
	}
}
