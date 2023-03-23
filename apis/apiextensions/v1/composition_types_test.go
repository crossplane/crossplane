package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

func TestComposition_validateResourceName(t *testing.T) {
	type fields struct {
		Spec CompositionSpec
	}
	tests := []struct {
		name     string
		fields   fields
		wantErrs field.ErrorList
	}{
		{
			name: "Valid: all named",
			fields: fields{
				Spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{
							Name: pointer.String("foo"),
						},
						{
							Name: pointer.String("bar"),
						},
					},
				},
			},
		},
		{
			name: "Valid: all anonymous",
			fields: fields{
				Spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{},
						{},
					},
				},
			},
		},
		{
			name: "Invalid: mixed names expecting anonymous",
			fields: fields{
				Spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{},
						{Name: pointer.String("bar")},
					},
				},
			},
			wantErrs: field.ErrorList{
				{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.resources[1].name",
					BadValue: "bar",
				},
			},
		},
		{
			name: "Invalid: mixed names expecting named",
			fields: fields{
				Spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{Name: pointer.String("bar")},
						{},
					},
				},
			},
			wantErrs: field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.resources[1].name",
					BadValue: "",
				},
			},
		},
		{
			name: "Valid: named with functions",
			fields: fields{
				Spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{Name: pointer.String("foo")},
						{Name: pointer.String("bar")},
					},
					Functions: []Function{
						{
							Name: "baz",
						},
					},
				},
			},
		},
		{
			name: "Invalid: anonymous with functions",
			fields: fields{
				Spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{},
					},
					Functions: []Function{
						{
							Name: "foo",
						},
					},
				},
			},
			wantErrs: field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.resources[0].name",
					BadValue: "",
				},
			},
		},
		{
			name: "Invalid: duplicate names",
			fields: fields{
				Spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{Name: pointer.String("foo")},
						{Name: pointer.String("bar")},
						{Name: pointer.String("foo")},
					},
				},
			},
			wantErrs: field.ErrorList{
				{
					Type:     field.ErrorTypeDuplicate,
					Field:    "spec.resources[2].name",
					BadValue: "foo",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Composition{
				Spec: tt.fields.Spec,
			}
			gotErrs := c.validateResourceNames()
			if diff := cmp.Diff(tt.wantErrs, gotErrs, cmpopts.IgnoreFields(field.Error{}, "Detail")); diff != "" {
				t.Errorf("\n%s\nvalidateResourceName(...): -want error, +got error: \n%s", tt.name, diff)
			}
		})
	}
}

func TestComposedTemplate_GetBaseObject(t *testing.T) {
	tests := []struct {
		name    string
		ct      *ComposedTemplate
		want    client.Object
		wantErr bool
	}{
		{
			name: "Valid base object",
			ct: &ComposedTemplate{
				Base: runtime.RawExtension{
					Raw: []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"foo"}}`),
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
					"metadata": map[string]interface{}{
						"name": "foo",
					},
				},
			},
		},
		{
			name: "Invalid base object",
			ct: &ComposedTemplate{
				Base: runtime.RawExtension{
					Raw: []byte(`{$$$WRONG$$$:"v1","kind":"Service","metadata":{"name":"foo"}}`),
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.ct.GetBaseObject()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBaseObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GetBaseObject(...): -want, +got: \n%s", diff)
			}
		})
	}
}

func TestReadinessCheck_Validate(t *testing.T) {
	tests := []struct {
		name string
		r    *ReadinessCheck
		want *field.Error
	}{
		{
			name: "Valid type none",
			r: &ReadinessCheck{
				Type: ReadinessCheckTypeNone,
			},
		},
		{
			name: "Valid type matchLabels",
			r: &ReadinessCheck{
				Type:        ReadinessCheckTypeMatchString,
				MatchString: "foo",
				FieldPath:   "spec.foo",
			},
		},
		{
			name: "Invalid type",
			r: &ReadinessCheck{
				Type: "foo",
			},
			want: &field.Error{
				Type:     field.ErrorTypeInvalid,
				Field:    "type",
				BadValue: "foo",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Validate()
			if diff := cmp.Diff(got, tt.want, cmpopts.IgnoreFields(field.Error{}, "Detail")); diff != "" {
				t.Errorf("Validate(...): -want, +got:\n%s", diff)
			}
		})
	}
}
