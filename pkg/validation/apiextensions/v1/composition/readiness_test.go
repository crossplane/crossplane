package composition

import (
	"context"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func withReadinessChecks(index int, rcs ...v1.ReadinessCheck) compositionBuilderOption {
	return func(c *v1.Composition) {
		c.Spec.Resources[index].ReadinessChecks = rcs
	}
}

// TODO(lsviben): check the errors - just paths are enough
func TestValidateReadinessCheck(t *testing.T) {
	type args struct {
		comp     *v1.Composition
		gvkToCRD map[schema.GroupVersionKind]apiextensions.CustomResourceDefinition
	}
	tests := []struct {
		name     string
		args     args
		wantErrs bool
	}{
		{
			name: "should accept empty readiness check",
			args: args{
				comp:     buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil),
				gvkToCRD: defaultGVKToCRDs(),
			},
			wantErrs: false,
		},
		{
			name: "should accept valid readiness check - none type",
			args: args{
				comp: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type: v1.ReadinessCheckTypeNone,
					},
				)),
				gvkToCRD: defaultGVKToCRDs(),
			},
			wantErrs: false,
		},
		{
			name: "should accept valid readiness check - nonEmpty type",
			args: args{
				comp: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:      v1.ReadinessCheckTypeNonEmpty,
						FieldPath: "spec.someOtherField",
					},
				)),
				gvkToCRD: defaultGVKToCRDs(),
			},
			wantErrs: false,
		},
		{
			name: "should reject an invalid readiness check - nonEmpty type",
			args: args{
				comp: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:      v1.ReadinessCheckTypeNonEmpty,
						FieldPath: "spec.doesNotExist",
					},
				)),
				gvkToCRD: defaultGVKToCRDs(),
			},
			wantErrs: true,
		},
		{
			name: "should accept valid readiness check - matchString type",
			args: args{
				comp: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:        v1.ReadinessCheckTypeMatchString,
						MatchString: "bob",
						FieldPath:   "spec.someOtherField",
					},
				)),
				gvkToCRD: defaultGVKToCRDs(),
			},
			wantErrs: false,
		},
		{
			name: "should reject invalid readiness check - matchString type",
			args: args{
				comp: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:        v1.ReadinessCheckTypeMatchString,
						MatchString: "",
						FieldPath:   "spec.someOtherField",
					},
				)),
				gvkToCRD: defaultGVKToCRDs(),
			},
			wantErrs: true,
		},
		{
			name: "should reject invalid readiness check - matchInteger type",
			args: args{
				comp: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:         v1.ReadinessCheckTypeMatchInteger,
						MatchInteger: 0,
						FieldPath:    "spec.someField",
					},
				)),
				gvkToCRD: buildGvkToCRDs(
					defaultManagedCrdBuilder().withOption(func(crd *extv1.CustomResourceDefinition) {
						crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["someField"] = extv1.JSONSchemaProps{
							Type: "integer",
						}
					}).build()),
			},
			wantErrs: true,
		},
		{
			name: "should accept valid readiness check - matchInteger type",
			args: args{
				comp: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:         v1.ReadinessCheckTypeMatchInteger,
						MatchInteger: 15,
						FieldPath:    "spec.someField",
					},
				)),
				gvkToCRD: buildGvkToCRDs(
					defaultManagedCrdBuilder().withOption(func(crd *extv1.CustomResourceDefinition) {
						crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["someField"] = extv1.JSONSchemaProps{
							Type: "integer",
						}
					}).build()),
			},
			wantErrs: false,
		},
		{
			name: "should reject invalid readiness check - matchInteger type - type mismatch",
			args: args{
				comp: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:         v1.ReadinessCheckTypeMatchInteger,
						MatchInteger: 10,
						FieldPath:    "spec.someField",
					},
				)),
				gvkToCRD: buildGvkToCRDs(
					defaultManagedCrdBuilder().withOption(func(crd *extv1.CustomResourceDefinition) {
						crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["someField"] = extv1.JSONSchemaProps{
							Type: "string",
						}
					}).build()),
			},
			wantErrs: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewValidator(WithCRDGetterFromMap(tt.args.gvkToCRD))
			if err != nil {
				t.Fatalf("NewValidator() error = %v", err)
			}
			if gotErrs := v.validateReadinessCheckWithSchemas(context.TODO(), tt.args.comp); (len(gotErrs) != 0) != tt.wantErrs {
				t.Errorf("validateReadinessCheckWithSchemas() = %v, want %v", gotErrs, tt.wantErrs)
			}
		})
	}
}
