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

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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

// MathTransform conducts mathematical operations on the input with the given
// configuration in its properties.
type MathTransform struct {
	// Multiply the value.
	// +optional
	Multiply *int64 `json:"multiply,omitempty"`
}

// MapTransform returns a value for the input from the given map.
type MapTransform struct {
	// TODO(negz): Are Pairs really optional if a MapTransform was specified?

	// Pairs is the map that will be used for transform.
	// +optional
	Pairs map[string]extv1.JSON `json:",inline"`
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
	StringConversionTypeToBase64   StringConversionType = "ToBase64"
	StringConversionTypeFromBase64 StringConversionType = "FromBase64"
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

	// Convert the type of conversion to Upper/Lower case.
	// +optional
	// +kubebuilder:validation:Enum=ToUpper;ToLower;ToBase64;FromBase64
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

// The list of supported ConvertTransform input and output types.
const (
	ConvertTransformTypeString  = "string"
	ConvertTransformTypeBool    = "bool"
	ConvertTransformTypeInt     = "int"
	ConvertTransformTypeInt64   = "int64"
	ConvertTransformTypeFloat64 = "float64"
)

// A ConvertTransform converts the input into a new object whose type is supplied.
type ConvertTransform struct {
	// ToType is the type of the output of this transform.
	// +kubebuilder:validation:Enum=string;int;int64;bool;float64
	ToType string `json:"toType"`
}
