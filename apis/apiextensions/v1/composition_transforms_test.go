package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"

	"github.com/crossplane/crossplane/pkg/validation/schema"
)

func toPtr[T any](i T) *T {
	return &i
}

func TestTransform_Validate(t1 *testing.T) {

	tests := []struct {
		name      string
		transform *Transform
		want      *field.Error
	}{
		{
			name: "Valid Math",
			transform: &Transform{
				Type: TransformTypeMath,
				Math: &MathTransform{
					Multiply: pointer.Int64(2),
				},
			},
			want: nil,
		},
		{
			name: "Invalid Math",
			transform: &Transform{
				Type: TransformTypeMath,
				Math: nil,
			},
			want: &field.Error{
				Type:  field.ErrorTypeRequired,
				Field: "math",
			},
		},
		{
			name: "Valid Map",
			transform: &Transform{
				Type: TransformTypeMap,
				Map: &MapTransform{
					Pairs: map[string]extv1.JSON{
						"foo": {Raw: []byte(`"bar"`)},
					},
				},
			},
			want: nil,
		},
		{
			name: "Invalid Map, no map",
			transform: &Transform{
				Type: TransformTypeMap,
				Map:  nil,
			},
			want: &field.Error{
				Type:  field.ErrorTypeRequired,
				Field: "map",
			},
		},
		{
			name: "Invalid Map, no pairs in map",
			transform: &Transform{
				Type: TransformTypeMap,
				Map:  &MapTransform{},
			},
			want: &field.Error{
				Type:  field.ErrorTypeRequired,
				Field: "map.pairs",
			},
		},
		{
			name: "Invalid Match, no match",
			transform: &Transform{
				Type:  TransformTypeMatch,
				Match: nil,
			},
			want: &field.Error{
				Type:  field.ErrorTypeRequired,
				Field: "match",
			},
		},
		{
			name: "Valid Match",
			transform: &Transform{
				Type:  TransformTypeMatch,
				Match: &MatchTransform{},
			},
			want: nil,
		},
		{
			name: "Invalid String, no string",
			transform: &Transform{
				Type:   TransformTypeString,
				String: nil,
			},
			want: &field.Error{
				Type:  field.ErrorTypeRequired,
				Field: "string",
			},
		},
		{
			name: "Valid String",
			transform: &Transform{
				Type: TransformTypeString,
				String: &StringTransform{
					Format: pointer.String("foo"),
				},
			},
		},
		{
			name: "Invalid Convert, missing Convert",
			transform: &Transform{
				Type:    TransformTypeConvert,
				Convert: nil,
			},
			want: &field.Error{
				Type:  field.ErrorTypeRequired,
				Field: "convert",
			},
		},
		{
			name: "Invalid Convert, unknown format",
			transform: &Transform{
				Type: TransformTypeConvert,
				Convert: &ConvertTransform{
					Format: toPtr[ConvertTransformFormat]("foo"),
				},
			},
			want: &field.Error{
				Type:  field.ErrorTypeInvalid,
				Field: "convert.format",
			},
		},
		{
			name: "Invalid Convert, unknown type",
			transform: &Transform{
				Type: TransformTypeConvert,
				Convert: &ConvertTransform{
					ToType: TransformIOType("foo"),
				},
			},
			want: &field.Error{
				Type:  field.ErrorTypeInvalid,
				Field: "convert.toType",
			},
		},
		{
			name: "Valid Convert",
			transform: &Transform{
				Type: TransformTypeConvert,
				Convert: &ConvertTransform{
					Format: toPtr(ConvertTransformFormatNone),
					ToType: TransformIOTypeInt,
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			got := tt.transform.Validate()
			if diff := cmp.Diff(got, tt.want, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t1.Errorf("Validate(...) = -want, +got\n%s\n", diff)
			}
		})
	}
}

func TestConvertTransform_GetConversionFunc(t1 *testing.T) {
	tests := []struct {
		name    string
		ct      *ConvertTransform
		from    TransformIOType
		wantErr bool
	}{
		{
			name: "Int to String",
			ct: &ConvertTransform{
				ToType: TransformIOTypeString,
			},
			from: TransformIOTypeInt,
		},
		{
			name: "Int to Int",
			ct: &ConvertTransform{
				ToType: TransformIOTypeInt,
			},
			from: TransformIOTypeInt,
		},
		{
			name: "Int to Int64",
			ct: &ConvertTransform{
				ToType: TransformIOTypeInt,
			},
			from: TransformIOTypeInt64,
		},
		{
			name: "Int64 to Int",
			ct: &ConvertTransform{
				ToType: TransformIOTypeInt64,
			},
			from: TransformIOTypeInt,
		},
		{
			name: "Int to Float",
			ct: &ConvertTransform{
				ToType: TransformIOTypeInt,
			},
			from: TransformIOTypeFloat64,
		},
		{
			name: "Int to Bool",
			ct: &ConvertTransform{
				ToType: TransformIOTypeInt,
			},
			from: TransformIOTypeBool,
		},
		{
			name: "String to Int invalid format",
			ct: &ConvertTransform{
				ToType: TransformIOTypeInt,
				Format: toPtr[ConvertTransformFormat]("wrong"),
			},
			from:    TransformIOTypeString,
			wantErr: true,
		},
		{
			name: "Int to Int, invalid format ignored",
			ct: &ConvertTransform{
				ToType: TransformIOTypeInt,
				Format: toPtr[ConvertTransformFormat]("wrong"),
			},
			from:    TransformIOTypeInt,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			_, err := tt.ct.GetConversionFunc(tt.from)
			if (err != nil) != tt.wantErr {
				t1.Errorf("GetConversionFunc() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestConvertTransformType_ToKnownJSONType(t *testing.T) {
	tests := []struct {
		name string
		c    TransformIOType
		want schema.KnownJSONType
	}{
		{
			name: "Int",
			c:    TransformIOTypeInt,
			want: schema.KnownJSONTypeInteger,
		},
		{
			name: "Int64",
			c:    TransformIOTypeInt64,
			want: schema.KnownJSONTypeInteger,
		},
		{
			name: "Float64",
			c:    TransformIOTypeFloat64,
			want: schema.KnownJSONTypeNumber,
		},
		{
			name: "Unknown returns empty string, should never happen",
			c:    TransformIOType("foo"),
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.ToKnownJSONType(); got != tt.want {
				t.Errorf("ToKnownJSONType() = %v, want %v", got, tt.want)
			}
			if tt.want == "" && tt.c.IsValid() {
				t.Errorf("IsValid() should return false for unknown type: %s", tt.c)
			}
		})
	}
}

func TestFromKnownJSONType(t *testing.T) {
	tests := []struct {
		name    string
		t       schema.KnownJSONType
		want    TransformIOType
		wantErr bool
	}{
		{
			name: "Int",
			t:    schema.KnownJSONTypeInteger,
			want: TransformIOTypeInt64,
		},
		{
			name: "Number",
			t:    schema.KnownJSONTypeNumber,
			want: TransformIOTypeFloat64,
		},
		{
			name:    "Unknown",
			t:       schema.KnownJSONType("foo"),
			wantErr: true,
		},
		{
			name:    "Empty",
			t:       "",
			wantErr: true,
		},
		{
			name:    "Null",
			t:       schema.KnownJSONTypeNull,
			wantErr: true,
		},
		{
			name: "Boolean",
			t:    schema.KnownJSONTypeBoolean,
			want: TransformIOTypeBool,
		},
		{
			name:    "Array",
			t:       schema.KnownJSONTypeArray,
			wantErr: true,
		},
		{
			name:    "Object",
			t:       schema.KnownJSONTypeObject,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromKnownJSONType(tt.t)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromKnownJSONType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("FromKnownJSONType() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTransform_GetOutputType(t1 *testing.T) {
	tests := []struct {
		name      string
		transform *Transform
		want      *TransformIOType
		wantErr   bool
	}{
		{
			name: "Output of Math transform",
			transform: &Transform{
				Type: TransformTypeMath,
			},
			want: toPtr[TransformIOType](TransformIOTypeFloat64),
		},
		{
			name: "Output of Convert transform, no validation",
			transform: &Transform{
				Type:    TransformTypeConvert,
				Convert: &ConvertTransform{ToType: "fakeType"},
			},
			want: toPtr[TransformIOType]("fakeType"),
		},
		{
			name: "Output of Unknown transform type returns an error",
			transform: &Transform{
				Type: "fakeType",
			},
			wantErr: true,
		},
		{
			name: "Output of Map transform is nil",
			transform: &Transform{
				Type: TransformTypeMap,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "Output of Match transform is nil",
			transform: &Transform{
				Type: TransformTypeMatch,
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			got, err := tt.transform.GetOutputType()
			if (err != nil) != tt.wantErr {
				t1.Errorf("GetOutputType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(got, tt.want, cmpopts.EquateEmpty()); diff != "" {
				t1.Errorf("GetOutputType() -want/+got: %s", diff)
			}
		})
	}
}
