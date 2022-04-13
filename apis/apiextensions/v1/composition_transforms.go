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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	errMathNoMultiplier   = "no input is given"
	errMathInputNonNumber = "input is required to be a number for math transformer"

	errFmtConvertInputTypeNotSupported = "input type %s is not supported"
	errFmtConversionPairNotSupported   = "conversion from %s to %s is not supported"
	errFmtTransformAtIndex             = "transform at index %d returned error"
	errFmtTypeNotSupported             = "transform type %s is not supported"
	errFmtTransformConfigMissing       = "given transform type %s requires configuration"
	errFmtTransformTypeFailed          = "%s transform could not resolve"
	errFmtMapTypeNotSupported          = "type %s is not supported for map transform"
	errFmtMapNotFound                  = "key %s is not found in map"

	errStringTransformTypeFailed  = "type %s is not supported for string transform type"
	errStringTransformTypeFormat  = "string transform of type %s fmt is not set"
	errStringTransformTypeConvert = "string transform of type %s convert is not set"
	errStringTransformTypeTrim    = "string transform of type %s trim is not set"
	errStringConvertTypeFailed    = "type %s is not supported for string convert"
	errDecodeString               = "string is not valid base64"
)

// TransformType is type of the transform function to be chosen.
type TransformType string

// Accepted TransformTypes.
const (
	TransformTypeMap     TransformType = "map"
	TransformTypeMath    TransformType = "math"
	TransformTypeString  TransformType = "string"
	TransformTypeConvert TransformType = "convert"
)

// Transform is a unit of process whose input is transformed into an output with
// the supplied configuration.
type Transform struct {

	// Type of the transform to be run.
	// +kubebuilder:validation:Enum=map;math;string;convert
	Type TransformType `json:"type"`

	// Math is used to transform the input via mathematical operations such as
	// multiplication.
	// +optional
	Math *MathTransform `json:"math,omitempty"`

	// Map uses the input as a key in the given map and returns the value.
	// +optional
	Map *MapTransform `json:"map,omitempty"`

	// String is used to transform the input into a string or a different kind
	// of string. Note that the input does not necessarily need to be a string.
	// +optional
	String *StringTransform `json:"string,omitempty"`

	// Convert is used to cast the input into the given output type.
	// +optional
	Convert *ConvertTransform `json:"convert,omitempty"`
}

// Transform calls the appropriate Transformer.
func (t *Transform) Transform(input interface{}) (interface{}, error) {
	var transformer interface {
		Resolve(input interface{}) (interface{}, error)
	}
	switch t.Type {
	case TransformTypeMath:
		transformer = t.Math
	case TransformTypeMap:
		transformer = t.Map
	case TransformTypeString:
		transformer = t.String
	case TransformTypeConvert:
		transformer = t.Convert
	default:
		return nil, errors.Errorf(errFmtTypeNotSupported, string(t.Type))
	}
	// An interface equals nil only if both the type and value are nil. Above,
	// even if t.<Type> is nil, its type is assigned to "transformer" but we're
	// interested in whether only the value is nil or not.
	if reflect.ValueOf(transformer).IsNil() {
		return nil, errors.Errorf(errFmtTransformConfigMissing, string(t.Type))
	}
	out, err := transformer.Resolve(input)
	return out, errors.Wrapf(err, errFmtTransformTypeFailed, string(t.Type))
}

// MathTransform conducts mathematical operations on the input with the given
// configuration in its properties.
type MathTransform struct {
	// Multiply the value.
	// +optional
	Multiply *int64 `json:"multiply,omitempty"`
}

// Resolve runs the Math transform.
func (m *MathTransform) Resolve(input interface{}) (interface{}, error) {
	if m.Multiply == nil {
		return nil, errors.New(errMathNoMultiplier)
	}
	switch i := input.(type) {
	case int64:
		return *m.Multiply * i, nil
	case int:
		return *m.Multiply * int64(i), nil
	default:
		return nil, errors.New(errMathInputNonNumber)
	}
}

// MapTransform returns a value for the input from the given map.
type MapTransform struct {
	// TODO(negz): Are Pairs really optional if a MapTransform was specified?

	// Pairs is the map that will be used for transform.
	// +optional
	Pairs map[string]string `json:",inline"`
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

// Resolve runs the Map transform.
func (m *MapTransform) Resolve(input interface{}) (interface{}, error) {
	switch i := input.(type) {
	case string:
		val, ok := m.Pairs[i]
		if !ok {
			return nil, errors.Errorf(errFmtMapNotFound, i)
		}
		return val, nil
	default:
		return nil, errors.Errorf(errFmtMapTypeNotSupported, reflect.TypeOf(input).String())
	}
}

// StringTransformType is type of the string transform function to be executed fmt/convert.
type StringTransformType string

// Accepted StringTransformType.
const (
	StringTransformFormat     StringTransformType = "Format" // Default
	StringTransformConvert    StringTransformType = "Convert"
	StringTransformTrimPrefix StringTransformType = "TrimPrefix"
	StringTransformTrimSuffix StringTransformType = "TrimSuffix"
)

// StringConversionType is the type of string conversion, ToUpper/ToLower/ToBase64/FromBase64
type StringConversionType string

// ConversionType accepted values.
const (
	ConversionTypeToUpper    = "ToUpper"
	ConversionTypeToLower    = "ToLower"
	ConversionTypeToBase64   = "ToBase64"
	ConversionTypeFromBase64 = "FromBase64"
)

// A StringTransform returns a string given the supplied input.
type StringTransform struct {

	// Type of the string transform to be run.
	// +optional
	// +kubebuilder:validation:Enum=Format;Convert;TrimPrefix;TrimSuffix
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
}

// Resolve runs the String transform.
func (s *StringTransform) Resolve(input interface{}) (interface{}, error) {

	switch s.Type {
	case StringTransformFormat:
		if s.Format == nil {
			return nil, errors.Errorf(errStringTransformTypeFormat, string(s.Type))
		}
		return fmt.Sprintf(*s.Format, input), nil
	case StringTransformConvert:
		if s.Convert == nil {
			return nil, errors.Errorf(errStringTransformTypeConvert, string(s.Type))
		}
		return stringConvertTransform(input, s.Convert)

	case StringTransformTrimPrefix, StringTransformTrimSuffix:
		if s.Trim == nil {
			return nil, errors.Errorf(errStringTransformTypeTrim, string(s.Type))
		}
		return stringTrimTransform(input, s.Type, *s.Trim), nil
	default:
		return nil, errors.Errorf(errStringTransformTypeFailed, string(s.Type))
	}
}

func stringConvertTransform(input interface{}, t *StringConversionType) (interface{}, error) {
	str := fmt.Sprintf("%v", input)
	switch *t {
	case ConversionTypeToUpper:
		return strings.ToUpper(str), nil
	case ConversionTypeToLower:
		return strings.ToLower(str), nil
	case ConversionTypeToBase64:
		return base64.StdEncoding.EncodeToString([]byte(str)), nil
	case ConversionTypeFromBase64:
		s, err := base64.StdEncoding.DecodeString(str)
		return string(s), errors.Wrap(err, errDecodeString)
	default:
		return nil, errors.Errorf(errStringConvertTypeFailed, *t)
	}
}

func stringTrimTransform(input interface{}, t StringTransformType, trim string) string {
	str := fmt.Sprintf("%v", input)
	if t == StringTransformTrimPrefix {
		return strings.TrimPrefix(str, trim)
	}
	if t == StringTransformTrimSuffix {
		return strings.TrimSuffix(str, trim)
	}
	return str
}

// The list of supported ConvertTransform input and output types.
const (
	ConvertTransformTypeString  = "string"
	ConvertTransformTypeBool    = "bool"
	ConvertTransformTypeInt     = "int"
	ConvertTransformTypeInt64   = "int64"
	ConvertTransformTypeFloat64 = "float64"
)

type conversionPair struct {
	From string
	To   string
}

// A ConvertTransform converts the input into a new object whose type is supplied.
type ConvertTransform struct {
	// ToType is the type of the output of this transform.
	// +kubebuilder:validation:Enum=string;int;int64;bool;float64
	ToType string `json:"toType"`
}

var conversions = map[conversionPair]func(interface{}) (interface{}, error){
	{From: ConvertTransformTypeString, To: ConvertTransformTypeInt64}: func(i interface{}) (interface{}, error) {
		return strconv.ParseInt(i.(string), 10, 64)
	},
	{From: ConvertTransformTypeString, To: ConvertTransformTypeBool}: func(i interface{}) (interface{}, error) {
		return strconv.ParseBool(i.(string))
	},
	{From: ConvertTransformTypeString, To: ConvertTransformTypeFloat64}: func(i interface{}) (interface{}, error) {
		return strconv.ParseFloat(i.(string), 64)
	},

	{From: ConvertTransformTypeInt64, To: ConvertTransformTypeString}: func(i interface{}) (interface{}, error) { // nolint:unparam
		return strconv.FormatInt(i.(int64), 10), nil
	},
	{From: ConvertTransformTypeInt64, To: ConvertTransformTypeBool}: func(i interface{}) (interface{}, error) { // nolint:unparam
		return i.(int64) == 1, nil
	},
	{From: ConvertTransformTypeInt64, To: ConvertTransformTypeFloat64}: func(i interface{}) (interface{}, error) { // nolint:unparam
		return float64(i.(int64)), nil
	},

	{From: ConvertTransformTypeBool, To: ConvertTransformTypeString}: func(i interface{}) (interface{}, error) { // nolint:unparam
		return strconv.FormatBool(i.(bool)), nil
	},
	{From: ConvertTransformTypeBool, To: ConvertTransformTypeInt64}: func(i interface{}) (interface{}, error) { // nolint:unparam
		if i.(bool) {
			return int64(1), nil
		}
		return int64(0), nil
	},
	{From: ConvertTransformTypeBool, To: ConvertTransformTypeFloat64}: func(i interface{}) (interface{}, error) { // nolint:unparam
		if i.(bool) {
			return float64(1), nil
		}
		return float64(0), nil
	},

	{From: ConvertTransformTypeFloat64, To: ConvertTransformTypeString}: func(i interface{}) (interface{}, error) { // nolint:unparam
		return strconv.FormatFloat(i.(float64), 'f', -1, 64), nil
	},
	{From: ConvertTransformTypeFloat64, To: ConvertTransformTypeInt64}: func(i interface{}) (interface{}, error) { // nolint:unparam
		return int64(i.(float64)), nil
	},
	{From: ConvertTransformTypeFloat64, To: ConvertTransformTypeBool}: func(i interface{}) (interface{}, error) { // nolint:unparam
		return i.(float64) == float64(1), nil
	},
}

// Resolve runs the String transform.
func (s *ConvertTransform) Resolve(input interface{}) (interface{}, error) {
	from := reflect.TypeOf(input).Kind().String()
	if from == ConvertTransformTypeInt {
		from = ConvertTransformTypeInt64
	}
	to := s.ToType
	if to == ConvertTransformTypeInt {
		to = ConvertTransformTypeInt64
	}
	switch from {
	case to:
		return input, nil
	case ConvertTransformTypeString, ConvertTransformTypeBool, ConvertTransformTypeInt64, ConvertTransformTypeFloat64:
		break
	default:
		return nil, errors.Errorf(errFmtConvertInputTypeNotSupported, reflect.TypeOf(input).Kind().String())
	}
	f, ok := conversions[conversionPair{From: from, To: to}]
	if !ok {
		return nil, errors.Errorf(errFmtConversionPairNotSupported, reflect.TypeOf(input).Kind().String(), s.ToType)
	}
	return f(input)
}
