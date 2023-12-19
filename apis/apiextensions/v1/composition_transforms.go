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
	"regexp"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	verrors "github.com/crossplane/crossplane/internal/validation/errors"
)

// TransformType is type of the transform function to be chosen.
type TransformType string

// Accepted TransformTypes.
const (
	ErrFmtConvertFormatPairNotSupported = "conversion from %s to %s is not supported with format %s"

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
		return verrors.WrapFieldError(t.Math.Validate(), field.NewPath("math"))
	case TransformTypeMap:
		if t.Map == nil {
			return field.Required(field.NewPath("map"), "given transform type map requires configuration")
		}
		return verrors.WrapFieldError(t.Map.Validate(), field.NewPath("map"))
	case TransformTypeMatch:
		if t.Match == nil {
			return field.Required(field.NewPath("match"), "given transform type match requires configuration")
		}
		return verrors.WrapFieldError(t.Match.Validate(), field.NewPath("match"))
	case TransformTypeString:
		if t.String == nil {
			return field.Required(field.NewPath("string"), "given transform type string requires configuration")
		}
		return verrors.WrapFieldError(t.String.Validate(), field.NewPath("string"))
	case TransformTypeConvert:
		if t.Convert == nil {
			return field.Required(field.NewPath("convert"), "given transform type convert requires configuration")
		}
		if err := t.Convert.Validate(); err != nil {
			return verrors.WrapFieldError(err, field.NewPath("convert"))
		}
	default:
		// Should never happen
		return field.Invalid(field.NewPath("type"), t.Type, "unknown transform type")
	}

	return nil
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

// MathTransformType conducts mathematical operations.
type MathTransformType string

// Accepted MathTransformType.
const (
	MathTransformTypeMultiply MathTransformType = "Multiply" // Default
	MathTransformTypeClampMin MathTransformType = "ClampMin"
	MathTransformTypeClampMax MathTransformType = "ClampMax"
)

// MathTransform conducts mathematical operations on the input with the given
// configuration in its properties.
type MathTransform struct {
	// Type of the math transform to be run.
	// +optional
	// +kubebuilder:validation:Enum=Multiply;ClampMin;ClampMax
	// +kubebuilder:default=Multiply
	Type MathTransformType `json:"type,omitempty"`

	// Multiply the value.
	// +optional
	Multiply *int64 `json:"multiply,omitempty"`
	// ClampMin makes sure that the value is not smaller than the given value.
	// +optional
	ClampMin *int64 `json:"clampMin,omitempty"`
	// ClampMax makes sure that the value is not bigger than the given value.
	// +optional
	ClampMax *int64 `json:"clampMax,omitempty"`
}

// GetType returns the type of the math transform, returning the default if not specified.
func (m *MathTransform) GetType() MathTransformType {
	if m.Type == "" {
		return MathTransformTypeMultiply
	}
	return m.Type
}

// Validate checks this MathTransform is valid.
func (m *MathTransform) Validate() *field.Error {
	switch m.GetType() {
	case MathTransformTypeMultiply:
		if m.Multiply == nil {
			return field.Required(field.NewPath("multiply"), "must specify a value if a multiply math transform is specified")
		}
	case MathTransformTypeClampMin:
		if m.ClampMin == nil {
			return field.Required(field.NewPath("clampMin"), "must specify a value if a clamp min math transform is specified")
		}
	case MathTransformTypeClampMax:
		if m.ClampMax == nil {
			return field.Required(field.NewPath("clampMax"), "must specify a value if a clamp max math transform is specified")
		}
	default:
		return field.Invalid(field.NewPath("type"), m.Type, "unknown math transform type")
	}
	return nil
}

// MapTransform returns a value for the input from the given map.
type MapTransform struct {
	// Pairs is the map that will be used for transform.
	// +optional
	Pairs map[string]extv1.JSON `json:",inline"`
}

// Validate checks this MapTransform is valid.
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
func (m *MapTransform) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Pairs)
}

// MatchFallbackTo defines how a match operation will fallback.
type MatchFallbackTo string

// Valid MatchFallbackTo.
const (
	MatchFallbackToTypeValue MatchFallbackTo = "Value"
	MatchFallbackToTypeInput MatchFallbackTo = "Input"
)

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
	// Determines to what value the transform should fallback if no pattern matches.
	// +optional
	// +kubebuilder:validation:Enum=Value;Input
	// +kubebuilder:default=Value
	FallbackTo MatchFallbackTo `json:"fallbackTo,omitempty"`
}

// Validate checks this MatchTransform is valid.
func (m *MatchTransform) Validate() *field.Error {
	if len(m.Patterns) == 0 {
		return field.Required(field.NewPath("patterns"), "at least one pattern must be specified if a match transform is specified")
	}
	for i, p := range m.Patterns {
		if err := p.Validate(); err != nil {
			return verrors.WrapFieldError(err, field.NewPath("patterns").Index(i))
		}
	}
	return nil
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

// Validate checks this MatchTransformPattern is valid.
func (m *MatchTransformPattern) Validate() *field.Error {
	switch m.Type {
	case MatchTransformPatternTypeLiteral, "":
		if m.Literal == nil {
			return field.Required(field.NewPath("literal"), "literal pattern type requires a literal")
		}
	case MatchTransformPatternTypeRegexp:
		if m.Regexp == nil {
			return field.Required(field.NewPath("regexp"), "regexp pattern type requires a regexp")
		}
		if _, err := regexp.Compile(*m.Regexp); err != nil {
			return field.Invalid(field.NewPath("regexp"), *m.Regexp, "invalid regexp")
		}
	default:
		return field.Invalid(field.NewPath("type"), m.Type, "unknown pattern type")
	}
	return nil
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
	StringTransformTypeJoin       StringTransformType = "Join"
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
	StringConversionTypeToAdler32  StringConversionType = "ToAdler32"
)

// A StringTransform returns a string given the supplied input.
type StringTransform struct {

	// Type of the string transform to be run.
	// +optional
	// +kubebuilder:validation:Enum=Format;Convert;TrimPrefix;TrimSuffix;Regexp;Join
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

	// Join defines parameters to join a slice of values to a string.
	// +optional
	Join *StringTransformJoin `json:"join,omitempty"`
}

// Validate checks this StringTransform is valid.
//
//nolint:gocyclo // just a switch
func (s *StringTransform) Validate() *field.Error {
	switch s.Type {
	case StringTransformTypeFormat, "":
		if s.Format == nil {
			return field.Required(field.NewPath("fmt"), "format transform requires a format")
		}
	case StringTransformTypeConvert:
		if s.Convert == nil {
			return field.Required(field.NewPath("convert"), "convert transform requires a conversion type")
		}
	case StringTransformTypeTrimPrefix, StringTransformTypeTrimSuffix:
		if s.Trim == nil {
			return field.Required(field.NewPath("trim"), "trim transform requires a trim value")
		}
	case StringTransformTypeRegexp:
		if s.Regexp == nil {
			return field.Required(field.NewPath("regexp"), "regexp transform requires a regexp")
		}
		if s.Regexp.Match == "" {
			return field.Required(field.NewPath("regexp", "match"), "regexp transform requires a match")
		}
		if _, err := regexp.Compile(s.Regexp.Match); err != nil {
			return field.Invalid(field.NewPath("regexp", "match"), s.Regexp.Match, "invalid regexp")
		}
	case StringTransformTypeJoin:
		if s.Join == nil {
			return field.Required(field.NewPath("join"), "join transform requires a join")
		}
	default:
		return field.Invalid(field.NewPath("type"), s.Type, "unknown string transform type")
	}
	return nil

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

	TransformIOTypeObject TransformIOType = "object"
	TransformIOTypeArray  TransformIOType = "array"
)

// IsValid checks if the given TransformIOType is valid.
func (c TransformIOType) IsValid() bool {
	switch c {
	case TransformIOTypeString, TransformIOTypeBool, TransformIOTypeInt, TransformIOTypeInt64, TransformIOTypeFloat64, TransformIOTypeObject, TransformIOTypeArray:
		return true
	}
	return false
}

// ConvertTransformFormat defines the expected format of an input value of a
// conversion transform.
type ConvertTransformFormat string

// Possible ConvertTransformFormat values.
const (
	ConvertTransformFormatNone     ConvertTransformFormat = "none"
	ConvertTransformFormatQuantity ConvertTransformFormat = "quantity"
	ConvertTransformFormatJSON     ConvertTransformFormat = "json"
)

// IsValid returns true if the format is valid.
func (c ConvertTransformFormat) IsValid() bool {
	switch c {
	case ConvertTransformFormatNone, ConvertTransformFormatQuantity, ConvertTransformFormatJSON:
		return true
	}
	return false
}

// StringTransformJoin defines parameters to join a slice of values to a string.
type StringTransformJoin struct {
	// Separator defines the character that should separate the values from each
	// other in the joined string.
	Separator string `json:"separator"`
}

// A ConvertTransform converts the input into a new object whose type is supplied.
type ConvertTransform struct {
	// ToType is the type of the output of this transform.
	// +kubebuilder:validation:Enum=string;int;int64;bool;float64;object;array
	ToType TransformIOType `json:"toType"`

	// The expected input format.
	//
	// * `quantity` - parses the input as a K8s [`resource.Quantity`](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/resource#Quantity).
	// Only used during `string -> float64` conversions.
	// * `json` - parses the input as a JSON string.
	// Only used during `string -> object` or `string -> list` conversions.
	//
	// If this property is null, the default conversion is applied.
	//
	// +kubebuilder:validation:Enum=none;quantity;json
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
