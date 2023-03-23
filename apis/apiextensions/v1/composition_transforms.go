/*
Copyright 2020 The Crossplane Authors.

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

package v1

import (
	"encoding/json"
	"strconv"

	xperrors "github.com/crossplane/crossplane/pkg/validation/errors"
	"github.com/crossplane/crossplane/pkg/validation/schema"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// TransformType is type of the transform function to be chosen.
type TransformType string

// Accepted TransformTypes.
const (
	TransformTypeMap     TransformType = "map"
	TransformTypeMatch   TransformType = "match"
	TransformTypeMath    TransformType = "math"
	TransformTypeString  TransformType = "string"
	TransformTypeConvert TransformType = "convert"
)

// Transform is a unit of process whose input is transformed into an output with
// the supplied configuration.
type Transform struct {

	// Type of the transform to be run.
	// +kubebuilder:validation:Enum=map;match;math;string;convert
	Type TransformType `json:"type"`

	// Math is used to transform the input via mathematical operations such as
	// multiplication.
	// +optional
	Math *MathTransform `json:"math,omitempty"`

	// Map uses the input as a key in the given map and returns the value.
	// +optional
	Map *MapTransform `json:"map,omitempty"`

	// Match is a more complex version of Map that matches a list of patterns.
	// +optional
	Match *MatchTransform `json:"match,omitempty"`

	// String is used to transform the input into a string or a different kind
	// of string. Note that the input does not necessarily need to be a string.
	// +optional
	String *StringTransform `json:"string,omitempty"`

	// Convert is used to cast the input into the given output type.
	// +optional
	Convert *ConvertTransform `json:"convert,omitempty"`
}

// Validate this Transform is valid.
//
//nolint:gocyclo // This is a long but simple/same-y switch.
func (t *Transform) Validate() *field.Error {
	switch t.Type {
	case TransformTypeMath:
		if t.Math == nil {
			return field.Required(field.NewPath("math"), "given transform type math requires configuration")
		}
	case TransformTypeMap:
		if t.Map == nil {
			return field.Required(field.NewPath("map"), "given transform type map requires configuration")
		}
		return xperrors.WrapFieldError(t.Map.Validate(), field.NewPath("map"))
	case TransformTypeMatch:
		if t.Match == nil {
			return field.Required(field.NewPath("match"), "given transform type match requires configuration")
		}
	case TransformTypeString:
		if t.String == nil {
			return field.Required(field.NewPath("string"), "given transform type string requires configuration")
		}
	case TransformTypeConvert:
		if t.Convert == nil {
			return field.Required(field.NewPath("convert"), "given transform type convert requires configuration")
		}
		if err := t.Convert.Validate(); err != nil {
			return xperrors.WrapFieldError(err, field.NewPath("convert"))
		}
	default:
		return field.Invalid(field.NewPath("type"), t.Type, "unknown transform type")
	}

	return nil
}

// IsValidInput validates the supplied Transform type, taking into consideration also the input type.
//
//nolint:gocyclo // This is a long but simple/same-y switch.
func (t *Transform) IsValidInput(fromType TransformIOType) error {
	switch t.Type {
	case TransformTypeMath:
		if fromType != TransformIOTypeInt && fromType != TransformIOTypeInt64 && fromType != TransformIOTypeFloat64 {
			return errors.Errorf("math transform can only be used with numeric types, got %s", fromType)
		}
	case TransformTypeMap:
		if fromType != TransformIOTypeString {
			return errors.Errorf("map transform can only be used with string types, got %s", fromType)
		}
	case TransformTypeMatch:
		if fromType != TransformIOTypeString {
			return errors.Errorf("match transform can only be used with string input types, got %s", fromType)
		}
	case TransformTypeString:
		if fromType != TransformIOTypeString {
			return errors.Errorf("string transform can only be used with string input types, got %s", fromType)
		}
	case TransformTypeConvert:
		if _, err := t.Convert.GetConversionFunc(fromType); err != nil {
			return err
		}
	default:
		return errors.Errorf("unknown transform type %s", t.Type)
	}
	return nil
}

type conversionPair struct {
	from   TransformIOType
	to     TransformIOType
	format ConvertTransformFormat
}

// GetConversionFunc returns the conversion function for the given input and output types, or an error if no conversion is
// supported. Will return a no-op conversion if the input and output types are the same.
func (t *ConvertTransform) GetConversionFunc(from TransformIOType) (func(any) (any, error), error) {
	originalFrom := from
	to := t.ToType
	if to == TransformIOTypeInt {
		to = TransformIOTypeInt64
	}
	if from == TransformIOTypeInt {
		from = TransformIOTypeInt64
	}
	if to == from {
		return func(input any) (any, error) {
			return input, nil
		}, nil
	}
	f, ok := conversions[conversionPair{from: from, to: to, format: t.GetFormat()}]
	if !ok {
		return nil, errors.Errorf("conversion from %s to %s is not supported with format %s", originalFrom, to, t.GetFormat())
	}
	return f, nil
}

// The unparam linter is complaining that these functions always return a nil
// error, but we need this to be the case given some other functions in the map
// may return an error.
var conversions = map[conversionPair]func(any) (any, error){
	{from: TransformIOTypeString, to: TransformIOTypeInt64, format: ConvertTransformFormatNone}: func(i any) (any, error) {

		return strconv.ParseInt(i.(string), 10, 64)
	},
	{from: TransformIOTypeString, to: TransformIOTypeBool, format: ConvertTransformFormatNone}: func(i any) (any, error) {
		return strconv.ParseBool(i.(string))
	},
	{from: TransformIOTypeString, to: TransformIOTypeFloat64, format: ConvertTransformFormatNone}: func(i any) (any, error) {
		return strconv.ParseFloat(i.(string), 64)
	},
	{from: TransformIOTypeString, to: TransformIOTypeFloat64, format: ConvertTransformFormatQuantity}: func(i any) (any, error) {
		q, err := resource.ParseQuantity(i.(string))
		if err != nil {
			return nil, err
		}
		return q.AsApproximateFloat64(), nil
	},

	{from: TransformIOTypeInt64, to: TransformIOTypeString, format: ConvertTransformFormatNone}: func(i any) (any, error) { //nolint:unparam // See note above.
		return strconv.FormatInt(i.(int64), 10), nil
	},
	{from: TransformIOTypeInt64, to: TransformIOTypeBool, format: ConvertTransformFormatNone}: func(i any) (any, error) { //nolint:unparam // See note above.
		return i.(int64) == 1, nil
	},
	{from: TransformIOTypeInt64, to: TransformIOTypeFloat64, format: ConvertTransformFormatNone}: func(i any) (any, error) { //nolint:unparam // See note above.
		return float64(i.(int64)), nil
	},

	{from: TransformIOTypeBool, to: TransformIOTypeString, format: ConvertTransformFormatNone}: func(i any) (any, error) { //nolint:unparam // See note above.
		return strconv.FormatBool(i.(bool)), nil
	},
	{from: TransformIOTypeBool, to: TransformIOTypeInt64, format: ConvertTransformFormatNone}: func(i any) (any, error) { //nolint:unparam // See note above.
		if i.(bool) {
			return int64(1), nil
		}
		return int64(0), nil
	},
	{from: TransformIOTypeBool, to: TransformIOTypeFloat64, format: ConvertTransformFormatNone}: func(i any) (any, error) { //nolint:unparam // See note above.
		if i.(bool) {
			return float64(1), nil
		}
		return float64(0), nil
	},

	{from: TransformIOTypeFloat64, to: TransformIOTypeString, format: ConvertTransformFormatNone}: func(i any) (any, error) { //nolint:unparam // See note above.
		return strconv.FormatFloat(i.(float64), 'f', -1, 64), nil
	},
	{from: TransformIOTypeFloat64, to: TransformIOTypeInt64, format: ConvertTransformFormatNone}: func(i any) (any, error) { //nolint:unparam // See note above.
		return int64(i.(float64)), nil
	},
	{from: TransformIOTypeFloat64, to: TransformIOTypeBool, format: ConvertTransformFormatNone}: func(i any) (any, error) { //nolint:unparam // See note above.
		return i.(float64) == float64(1), nil
	},
}

// GetFormat returns the format of the transform.
func (t *ConvertTransform) GetFormat() ConvertTransformFormat {
	if t.Format != nil {
		return *t.Format
	}
	return ConvertTransformFormatNone
}

// GetOutputType returns the output type of the transform.
// It returns an error if the transform type is unknown.
// It returns nil if the output type is not known.
func (t *Transform) GetOutputType() (*TransformIOType, error) {
	var out TransformIOType
	switch t.Type {
	case TransformTypeMap, TransformTypeMatch:
		return nil, nil
	case TransformTypeMath:
		out = TransformIOTypeFloat64
	case TransformTypeString:
		out = TransformIOTypeString
	case TransformTypeConvert:
		out = t.Convert.ToType
	default:
		return nil, errors.Errorf("unable to get output type, unknown transform type: %s", t.Type)
	}
	return &out, nil
}

// MathTransform conducts mathematical operations on the input with the given
// configuration in its properties.
type MathTransform struct {
	// Multiply the value.
	// +optional
	Multiply *int64 `json:"multiply,omitempty"`
}

// MapTransform returns a value for the input from the given map.
type MapTransform struct {
	// Pairs is the map that will be used for transform.
	// +optional
	Pairs map[string]extv1.JSON `json:",inline"`
}

// Validate this MapTransform.
func (m *MapTransform) Validate() *field.Error {
	if len(m.Pairs) == 0 {
		return field.Required(field.NewPath("pairs"), "at least one pair must be specified if a map transform is specified")
	}
	return nil
}

// NOTE(negz): The Kubernetes JSON decoder doesn't seem to like inlining a map
// into a struct - doing so results in a seemingly successful unmarshal of the
// data, but an empty map. We must keep the ,inline tag nevertheless in order to
// trick the CRD generator into thinking MapTransform is an arbitrary map (i.e.
// generating a validation schema with string additionalProperties), but the
// actual marshalling is handled by the marshal methods below.

// UnmarshalJSON into this MapTransform.
func (m *MapTransform) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &m.Pairs)
}

// MarshalJSON from this MapTransform.
func (m MapTransform) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Pairs)
}

// MatchTransform is a more complex version of a map transform that matches a
// list of patterns.
type MatchTransform struct {
	// The patterns that should be tested against the input string.
	// Patterns are tested in order. The value of the first match is used as
	// result of this transform.
	Patterns []MatchTransformPattern `json:"patterns,omitempty"`

	// The fallback value that should be returned by the transform if now pattern
	// matches.
	FallbackValue extv1.JSON `json:"fallbackValue,omitempty"`
}

// MatchTransformPatternType defines the type of a MatchTransformPattern.
type MatchTransformPatternType string

// Valid MatchTransformPatternTypes.
const (
	MatchTransformPatternTypeLiteral MatchTransformPatternType = "literal"
	MatchTransformPatternTypeRegexp  MatchTransformPatternType = "regexp"
)

// MatchTransformPattern is a transform that returns the value that matches a
// pattern.
type MatchTransformPattern struct {
	// Type specifies how the pattern matches the input.
	//
	// * `literal` - the pattern value has to exactly match (case sensitive) the
	// input string. This is the default.
	//
	// * `regexp` - the pattern treated as a regular expression against
	// which the input string is tested. Crossplane will throw an error if the
	// key is not a valid regexp.
	//
	// +kubebuilder:validation:Enum=literal;regexp
	// +kubebuilder:default=literal
	Type MatchTransformPatternType `json:"type"`

	// Literal exactly matches the input string (case sensitive).
	// Is required if `type` is `literal`.
	Literal *string `json:"literal,omitempty"`

	// Regexp to match against the input string.
	// Is required if `type` is `regexp`.
	Regexp *string `json:"regexp,omitempty"`

	// The value that is used as result of the transform if the pattern matches.
	Result extv1.JSON `json:"result"`
}

// StringTransformType transforms a string.
type StringTransformType string

// Accepted StringTransformTypes.
const (
	StringTransformTypeFormat     StringTransformType = "Format" // Default
	StringTransformTypeConvert    StringTransformType = "Convert"
	StringTransformTypeTrimPrefix StringTransformType = "TrimPrefix"
	StringTransformTypeTrimSuffix StringTransformType = "TrimSuffix"
	StringTransformTypeRegexp     StringTransformType = "Regexp"
)

// StringConversionType converts a string.
type StringConversionType string

// Accepted StringConversionTypes.
const (
	StringConversionTypeToUpper    StringConversionType = "ToUpper"
	StringConversionTypeToLower    StringConversionType = "ToLower"
	StringConversionTypeToJSON     StringConversionType = "ToJson"
	StringConversionTypeToBase64   StringConversionType = "ToBase64"
	StringConversionTypeFromBase64 StringConversionType = "FromBase64"
	StringConversionTypeToSHA1     StringConversionType = "ToSha1"
	StringConversionTypeToSHA256   StringConversionType = "ToSha256"
	StringConversionTypeToSHA512   StringConversionType = "ToSha512"
)

// A StringTransform returns a string given the supplied input.
type StringTransform struct {

	// Type of the string transform to be run.
	// +optional
	// +kubebuilder:validation:Enum=Format;Convert;TrimPrefix;TrimSuffix;Regexp
	// +kubebuilder:default=Format
	Type StringTransformType `json:"type,omitempty"`

	// Format the input using a Go format string. See
	// https://golang.org/pkg/fmt/ for details.
	// +optional
	Format *string `json:"fmt,omitempty"`

	// Optional conversion method to be specified.
	// `ToUpper` and `ToLower` change the letter case of the input string.
	// `ToBase64` and `FromBase64` perform a base64 conversion based on the input string.
	// `ToJson` converts any input value into its raw JSON representation.
	// `ToSha1`, `ToSha256` and `ToSha512` generate a hash value based on the input
	// converted to JSON.
	// +optional
	// +kubebuilder:validation:Enum=ToUpper;ToLower;ToBase64;FromBase64;ToJson;ToSha1;ToSha256;ToSha512
	Convert *StringConversionType `json:"convert,omitempty"`

	// Trim the prefix or suffix from the input
	// +optional
	Trim *string `json:"trim,omitempty"`

	// Extract a match from the input using a regular expression.
	// +optional
	Regexp *StringTransformRegexp `json:"regexp,omitempty"`
}

// A StringTransformRegexp extracts a match from the input using a regular
// expression.
type StringTransformRegexp struct {
	// Match string. May optionally include submatches, aka capture groups.
	// See https://pkg.go.dev/regexp/ for details.
	Match string `json:"match"`

	// Group number to match. 0 (the default) matches the entire expression.
	// +optional
	Group *int `json:"group,omitempty"`
}

// TransformIOType defines the type of a ConvertTransform.
type TransformIOType string

// The list of supported Transform input and output types.
const (
	TransformIOTypeString  TransformIOType = "string"
	TransformIOTypeBool    TransformIOType = "bool"
	TransformIOTypeInt     TransformIOType = "int"
	TransformIOTypeInt64   TransformIOType = "int64"
	TransformIOTypeFloat64 TransformIOType = "float64"
)

// IsValid checks if the given TransformIOType is valid.
func (c TransformIOType) IsValid() bool {
	switch c {
	case TransformIOTypeString, TransformIOTypeBool, TransformIOTypeInt, TransformIOTypeInt64, TransformIOTypeFloat64:
		return true
	default:
		return false
	}
}

// ToKnownJSONType returns the matching JSON type for the given TransformIOType.
// It returns an empty string if the type is not valid, call IsValid() before
// calling this method.
func (c TransformIOType) ToKnownJSONType() schema.KnownJSONType {
	switch c {
	case TransformIOTypeString:
		return schema.KnownJSONTypeString
	case TransformIOTypeBool:
		return schema.KnownJSONTypeBoolean
	case TransformIOTypeInt, TransformIOTypeInt64:
		return schema.KnownJSONTypeInteger
	case TransformIOTypeFloat64:
		return schema.KnownJSONTypeNumber
	}
	// should never happen
	return ""
}

// FromKnownJSONType returns the TransformIOType for the given KnownJSONType.
func FromKnownJSONType(t schema.KnownJSONType) (TransformIOType, error) {
	switch t {
	case schema.KnownJSONTypeString:
		return TransformIOTypeString, nil
	case schema.KnownJSONTypeBoolean:
		return TransformIOTypeBool, nil
	case schema.KnownJSONTypeInteger:
		return TransformIOTypeInt64, nil
	case schema.KnownJSONTypeNumber:
		return TransformIOTypeFloat64, nil
	case schema.KnownJSONTypeObject, schema.KnownJSONTypeArray, schema.KnownJSONTypeNull:
		return "", errors.Errorf("JSON type not supported: %q", t)
	default:
		return "", errors.Errorf("unknown JSON type: %q", t)
	}
}

// ConvertTransformFormat defines the expected format of an input value of a
// conversion transform.
type ConvertTransformFormat string

// Possible ConvertTransformFormat values.
const (
	ConvertTransformFormatNone     ConvertTransformFormat = "none"
	ConvertTransformFormatQuantity ConvertTransformFormat = "quantity"
)

// IsValid returns true if the format is valid.
func (c ConvertTransformFormat) IsValid() bool {
	switch c {
	case ConvertTransformFormatNone, ConvertTransformFormatQuantity:
		return true
	}
	return false
}

// A ConvertTransform converts the input into a new object whose type is supplied.
type ConvertTransform struct {
	// ToType is the type of the output of this transform.
	// +kubebuilder:validation:Enum=string;int;int64;bool;float64
	ToType TransformIOType `json:"toType"`

	// The expected input format.
	//
	// * `quantity` - parses the input as a K8s [`resource.Quantity`](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/resource#Quantity).
	// Only used during `string -> float64` conversions.
	//
	// If this property is null, the default conversion is applied.
	//
	// +kubebuilder:validation:Enum=none;quantity
	// +kubebuilder:validation:Default=none
	Format *ConvertTransformFormat `json:"format,omitempty"`
}

// Validate returns an error if the ConvertTransform is invalid.
func (t ConvertTransform) Validate() *field.Error {
	if !t.GetFormat().IsValid() {
		return field.Invalid(field.NewPath("format"), t.Format, "invalid format")
	}
	if !t.ToType.IsValid() {
		return field.Invalid(field.NewPath("toType"), t.ToType, "invalid type")
	}
	return nil
}
