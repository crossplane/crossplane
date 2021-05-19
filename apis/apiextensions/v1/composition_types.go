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
	"fmt"
	"math/rand"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

const (
	errMathNoMultiplier   = "no input is given"
	errMathInputNonNumber = "input is required to be a number for math transformer"
	errPatchSetType       = "a patch in a PatchSet cannot be of type PatchSet"
	errRequiredField      = "%s is required by type %s"
	errUndefinedPatchSet  = "cannot find PatchSet by name %s"
	errInvalidPatchType   = "patch type %s is unsupported"

	errFmtConvertInputTypeNotSupported = "input type %s is not supported"
	errFmtConversionPairNotSupported   = "conversion from %s to %s is not supported"
	errFmtTransformAtIndex             = "transform at index %d returned error"
	errFmtTypeNotSupported             = "transform type %s is not supported"
	errFmtConfigMissing                = "given type %s requires configuration"
	errFmtTransformTypeFailed          = "%s transform could not resolve"
	errFmtMapTypeNotSupported          = "type %s is not supported for map transform"
	errFmtMapNotFound                  = "key %s is not found in map"
	errFmtFromFnTypeNotSupported       = "fromFn type %s is not supported"

	errFromFnTypeNotDefined = "fromFn type not defined"
)

const (
	defaultRandomStringCharset       = "abcdedfghijklmnopqrstABCDEFGHIJKLMNOP0123456789+*%&"
	defaultRandomStringLength  int32 = 32
)

// CompositionSpec specifies the desired state of the definition.
type CompositionSpec struct {
	// CompositeTypeRef specifies the type of composite resource that this
	// composition is compatible with.
	// +immutable
	CompositeTypeRef TypeReference `json:"compositeTypeRef"`

	// PatchSets define a named set of patches that may be included by
	// any resource in this Composition.
	// PatchSets cannot themselves refer to other PatchSets.
	// +optional
	PatchSets []PatchSet `json:"patchSets,omitempty"`

	// Resources is the list of resource templates that will be used when a
	// composite resource referring to this composition is created.
	Resources []ComposedTemplate `json:"resources"`

	// WriteConnectionSecretsToNamespace specifies the namespace in which the
	// connection secrets of composite resource dynamically provisioned using
	// this composition will be created.
	// +optional
	WriteConnectionSecretsToNamespace *string `json:"writeConnectionSecretsToNamespace,omitempty"`
}

// InlinePatchSets dereferences PatchSets and includes their patches inline. The
// updated CompositionSpec should not be persisted to the API server.
func (cs *CompositionSpec) InlinePatchSets() error {
	pn := make(map[string][]Patch)
	for _, s := range cs.PatchSets {
		for _, p := range s.Patches {
			if p.Type == PatchTypePatchSet {
				return errors.New(errPatchSetType)
			}
		}
		pn[s.Name] = s.Patches
	}

	for i, r := range cs.Resources {
		po := []Patch{}
		for _, p := range r.Patches {
			if p.Type != PatchTypePatchSet {
				po = append(po, p)
				continue
			}
			if p.PatchSetName == nil {
				return errors.Errorf(errRequiredField, "PatchSetName", p.Type)
			}
			ps, ok := pn[*p.PatchSetName]
			if !ok {
				return errors.Errorf(errUndefinedPatchSet, *p.PatchSetName)
			}
			po = append(po, ps...)
		}
		cs.Resources[i].Patches = po
	}
	return nil
}

// A PatchSet is a set of patches that can be reused from all resources within
// a Composition.
type PatchSet struct {
	// Name of this PatchSet.
	Name string `json:"name"`

	// Patches will be applied as an overlay to the base resource.
	Patches []Patch `json:"patches"`
}

// TypeReference is used to refer to a type for declaring compatibility.
type TypeReference struct {
	// APIVersion of the type.
	APIVersion string `json:"apiVersion"`

	// Kind of the type.
	Kind string `json:"kind"`
}

// TypeReferenceTo returns a reference to the supplied GroupVersionKind
func TypeReferenceTo(gvk schema.GroupVersionKind) TypeReference {
	return TypeReference{APIVersion: gvk.GroupVersion().String(), Kind: gvk.Kind}
}

// ComposedTemplate is used to provide information about how the composed resource
// should be processed.
type ComposedTemplate struct {
	// TODO(negz): Name should be a required field in v2 of this API.

	// A Name uniquely identifies this entry within its Composition's resources
	// array. Names are optional but *strongly* recommended. When all entries in
	// the resources array are named entries may added, deleted, and reordered
	// as long as their names do not change. When entries are not named the
	// length and order of the resources array should be treated as immutable.
	// Either all or no entries must be named.
	// +optional
	Name *string `json:"name,omitempty"`

	// Base is the target resource that the patches will be applied on.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:EmbeddedResource
	Base runtime.RawExtension `json:"base"`

	// Patches will be applied as overlay to the base resource.
	// +optional
	Patches []Patch `json:"patches,omitempty"`

	// ConnectionDetails lists the propagation secret keys from this target
	// resource to the composition instance connection secret.
	// +optional
	ConnectionDetails []ConnectionDetail `json:"connectionDetails,omitempty"`

	// ReadinessChecks allows users to define custom readiness checks. All checks
	// have to return true in order for resource to be considered ready. The
	// default readiness check is to have the "Ready" condition to be "True".
	// +optional
	ReadinessChecks []ReadinessCheck `json:"readinessChecks,omitempty"`
}

// ReadinessCheckType is used for readiness check types.
type ReadinessCheckType string

// The possible values for readiness check type.
const (
	ReadinessCheckTypeNonEmpty     ReadinessCheckType = "NonEmpty"
	ReadinessCheckTypeMatchString  ReadinessCheckType = "MatchString"
	ReadinessCheckTypeMatchInteger ReadinessCheckType = "MatchInteger"
	ReadinessCheckTypeNone         ReadinessCheckType = "None"
)

// ReadinessCheck is used to indicate how to tell whether a resource is ready
// for consumption
type ReadinessCheck struct {
	// Type indicates the type of probe you'd like to use.
	// +kubebuilder:validation:Enum="MatchString";"MatchInteger";"NonEmpty";"None"
	Type ReadinessCheckType `json:"type"`

	// FieldPath shows the path of the field whose value will be used.
	// +optional
	FieldPath string `json:"fieldPath,omitempty"`

	// MatchString is the value you'd like to match if you're using "MatchString" type.
	// +optional
	MatchString string `json:"matchString,omitempty"`

	// MatchInt is the value you'd like to match if you're using "MatchInt" type.
	// +optional
	MatchInteger int64 `json:"matchInteger,omitempty"`
}

// A PatchType is a type of patch.
type PatchType string

// Patch types.
const (
	PatchTypeFromCompositeFieldPath PatchType = "FromCompositeFieldPath" // Default
	PatchTypePatchSet               PatchType = "PatchSet"
	PatchTypeToCompositeFieldPath   PatchType = "ToCompositeFieldPath"
	PatchTypeFromFn                 PatchType = "FromFn"
)

// Patch objects are applied between composite and composed resources. Their
// behaviour depends on the Type selected. The default Type,
// FromCompositeFieldPath, copies a value from the composite resource to
// the composed resource, applying any defined transformers.
type Patch struct {
	// Type sets the patching behaviour to be used. Each patch type may require
	// its' own fields to be set on the Patch object.
	// +optional
	// +kubebuilder:validation:Enum=FromCompositeFieldPath;PatchSet;ToCompositeFieldPath;FromFn
	// +kubebuilder:default=FromCompositeFieldPath
	Type PatchType `json:"type,omitempty"`

	// FromFieldPath is the path of the field on the resource whose value is
	// to be used as input. Required when type is FromCompositeFieldPath or
	// ToCompositeFieldPath.
	// +optional
	FromFieldPath *string `json:"fromFieldPath,omitempty"`

	// ToFieldPath is the path of the field on the resource whose value will
	// be changed with the result of transforms. Leave empty if you'd like to
	// propagate to the same path as fromFieldPath.
	// +optional
	ToFieldPath *string `json:"toFieldPath,omitempty"`

	// PatchSetName to include patches from. Required when type is PatchSet.
	// +optional
	PatchSetName *string `json:"patchSetName,omitempty"`

	// Transforms are the list of functions that are used as a FIFO pipe for the
	// input to be transformed.
	// +optional
	Transforms []Transform `json:"transforms,omitempty"`

	// Policy configures the specifics of patching behaviour.
	// +optional
	Policy *PatchPolicy `json:"policy,omitempty"`

	// FromFn is a function whose output is patched instead of a reference to a
	// field path
	// +optional
	FromFn *FromFn `json:"fromFn,omitempty"`
}

// FromFnType is the type of FromFn to be used
// +kubebuilder:validation:Enum=randomString
type FromFnType string

// Accepted FromFnTypes
const (
	FromFnTypeRandomString FromFnType = "randomString"
)

// FromFn is the function to be used for patch
type FromFn struct {
	// Type of FromFn to be executed
	// +optional
	Type FromFnType `json:"type,omitempty"`

	// RandomString is function to generate random string using the input charset
	// and length; required when fromFnType is randomString
	// +optional
	RandomString *RandomString `json:"randomString,omitempty"`
}

// GetFnValue gets the output value of the fromFn specified
func (fn *FromFn) GetFnValue() (interface{}, error) {
	switch fn.Type {
	case "":
		return nil, errors.New(errFromFnTypeNotDefined)
	case FromFnTypeRandomString:
		if fn.RandomString == nil {
			length, charset := defaultRandomStringLength, defaultRandomStringCharset
			return (&RandomString{&length, charset}).GetValue()
		}
		return fn.RandomString.GetValue()
	default:
		return nil, errors.Errorf(errFmtFromFnTypeNotSupported, fn.Type)
	}
}

// RandomString is one of FromFn type to generate a random string
// using the provided charset and length
type RandomString struct {
	// Length is the length of the random string to be generated
	// +kubebuilder:default=32
	// +optional
	Length *int32 `json:"length,omitempty"` //default: 32

	// Charset is the character set to be used to generate random string
	// +kubebuilder:default=abcdedfghijklmnopqrstABCDEFGHIJKLMNOP0123456789+*%&
	// +optional
	Charset string `json:"charset,omitempty"`
}

// GetValue returns a random string using the charset and length
// from RandomString object
func (rs *RandomString) GetValue() (string, error) {
	charRunes := []rune(rs.Charset)
	outputRunes := make([]rune, *rs.Length)
	for i := range outputRunes {
		// No lint exception added to allow the use of math/rand
		// instead of crypto/rand. Usecase here doesn't require
		// a strong random number generator.
		outputRunes[i] = charRunes[rand.Intn(len(charRunes))] //nolint: gosec
	}
	return string(outputRunes), nil
}

// A FromFieldPathPolicy determines how to patch from a field path.
type FromFieldPathPolicy string

// FromFieldPath patch policies.
const (
	FromFieldPathPolicyOptional FromFieldPathPolicy = "Optional"
	FromFieldPathPolicyRequired FromFieldPathPolicy = "Required"
)

// A PatchPolicy configures the specifics of patching behaviour.
type PatchPolicy struct {
	// FromFieldPath specifies how to patch from a field path. The default is
	// 'Optional', which means the patch will be a no-op if the specified
	// fromFieldPath does not exist. Use 'Required' if the patch should fail if
	// the specified path does not exist.
	// +kubebuilder:validation:Enum=Optional;Required
	// +optional
	FromFieldPath *FromFieldPathPolicy `json:"fromFieldPath,omitempty"`
}

// Apply executes a patching operation between the from and to resources.
// Applies all patch types unless an 'only' filter is supplied.
func (c *Patch) Apply(from, to runtime.Object, only ...PatchType) error {
	if c.filterPatch(only...) {
		return nil
	}

	switch c.Type {
	case PatchTypeFromCompositeFieldPath:
		return c.applyFromFieldPathPatch(from, to)
	case PatchTypeToCompositeFieldPath:
		return c.applyFromFieldPathPatch(to, from)
	case PatchTypeFromFn:
		return c.applyFromFnPatch(to)
	case PatchTypePatchSet:
		// Already resolved - nothing to do.
	}
	return errors.Errorf(errInvalidPatchType, c.Type)
}

// filterPatch returns true if patch should be filtered (not applied)
func (c *Patch) filterPatch(only ...PatchType) bool {
	// filter does not apply if not set
	if len(only) == 0 {
		return false
	}

	for _, patchType := range only {
		if patchType == c.Type {
			return false
		}
	}
	return true
}

// applyFromFieldPathPatch patches the "to" resource, using a source field
// on the "from" resource. Values may be transformed if any are defined on
// the patch.
func (c *Patch) applyFromFieldPathPatch(from, to runtime.Object) error { // nolint:gocyclo
	// NOTE(benagricola): The cyclomatic complexity here is from error checking
	// at each stage of the patching process, in addition to Apply methods now
	// being responsible for checking the validity of their input fields
	// (necessary because with multiple patch types, the input fields
	// must be +optional).
	if c.FromFieldPath == nil {
		return errors.Errorf(errRequiredField, "FromFieldPath", c.Type)
	}

	// Default to patching the same field on the composed resource.
	if c.ToFieldPath == nil {
		c.ToFieldPath = c.FromFieldPath
	}

	fromMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(from)
	if err != nil {
		return err
	}

	in, err := fieldpath.Pave(fromMap).GetValue(*c.FromFieldPath)
	if IsOptionalFieldPathNotFound(err, c.Policy) {
		return nil
	}
	if err != nil {
		return err
	}
	out := in
	for i, f := range c.Transforms {
		if out, err = f.Transform(out); err != nil {
			return errors.Wrapf(err, errFmtTransformAtIndex, i)
		}
	}

	if u, ok := to.(interface{ UnstructuredContent() map[string]interface{} }); ok {
		return fieldpath.Pave(u.UnstructuredContent()).SetValue(*c.ToFieldPath, out)
	}

	toMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(to)
	if err != nil {
		return err
	}
	if err := fieldpath.Pave(toMap).SetValue(*c.ToFieldPath, out); err != nil {
		return err
	}
	return runtime.DefaultUnstructuredConverter.FromUnstructured(toMap, to)
}

// applyFromFnPatch patches the "to" resource using a function defined in
// the FromFn. Values may be transformed if any are defined on
// the patch.
func (c *Patch) applyFromFnPatch(to runtime.Object) error {
	if c.ToFieldPath == nil {
		return errors.Errorf(errRequiredField, "ToFieldPath", c.Type)
	}

	if c.FromFn == nil {
		return errors.Errorf(errRequiredField, "FromFn", c.Type)
	}

	fnValue, err := c.FromFn.GetFnValue()
	if err != nil {
		return err
	}

	out := fnValue
	for i, f := range c.Transforms {
		if out, err = f.Transform(out); err != nil {
			return errors.Wrapf(err, errFmtTransformAtIndex, i)
		}
	}

	toMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(to)
	if err != nil {
		return err
	}

	if err := fieldpath.Pave(toMap).SetValue(*c.ToFieldPath, out); err != nil {
		return err
	}

	return runtime.DefaultUnstructuredConverter.FromUnstructured(toMap, to)
}

// IsOptionalFieldPathNotFound returns true if the supplied error indicates a
// field path was not found, and the supplied policy indicates a patch from that
// field path was optional.
func IsOptionalFieldPathNotFound(err error, s *PatchPolicy) bool {
	switch {
	case s == nil:
		fallthrough
	case s.FromFieldPath == nil:
		fallthrough
	case *s.FromFieldPath == FromFieldPathPolicyOptional:
		return fieldpath.IsNotFound(err)
	default:
		return false
	}
}

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
		return nil, errors.Errorf(errFmtConfigMissing, string(t.Type))
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

// A StringTransform returns a string given the supplied input.
type StringTransform struct {
	// Format the input using a Go format string. See
	// https://golang.org/pkg/fmt/ for details.
	Format string `json:"fmt"`
}

// Resolve runs the String transform.
func (s *StringTransform) Resolve(input interface{}) (interface{}, error) {
	return fmt.Sprintf(s.Format, input), nil
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

// A ConvertTransform converts the input into a new object whose type is supplied.
type ConvertTransform struct {
	// ToType is the type of the output of this transform.
	// +kubebuilder:validation:Enum=string;int;int64;bool;float64
	ToType string `json:"toType"`
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

// A ConnectionDetailType is a type of connection detail.
type ConnectionDetailType string

// ConnectionDetailType types.
const (
	ConnectionDetailTypeUnknown                 ConnectionDetailType = "Unknown"
	ConnectionDetailTypeFromConnectionSecretKey ConnectionDetailType = "FromConnectionSecretKey"
	ConnectionDetailTypeFromFieldPath           ConnectionDetailType = "FromFieldPath"
	ConnectionDetailTypeFromValue               ConnectionDetailType = "FromValue"
)

// ConnectionDetail includes the information about the propagation of the connection
// information from one secret to another.
type ConnectionDetail struct {
	// Name of the connection secret key that will be propagated to the
	// connection secret of the composition instance. Leave empty if you'd like
	// to use the same key name.
	// +optional
	Name *string `json:"name,omitempty"`

	// Type sets the connection detail fetching behaviour to be used. Each
	// connection detail type may require its own fields to be set on the
	// ConnectionDetail object. If the type is omitted Crossplane will attempt
	// to infer it based on which other fields were specified.
	// +optional
	// +kubebuilder:validation:Enum=FromConnectionSecretKey;FromFieldPath;FromValue
	Type *ConnectionDetailType `json:"type,omitempty"`

	// FromConnectionSecretKey is the key that will be used to fetch the value
	// from the given target resource's secret.
	// +optional
	FromConnectionSecretKey *string `json:"fromConnectionSecretKey,omitempty"`

	// FromFieldPath is the path of the field on the composed resource whose
	// value to be used as input. Name must be specified if the type is
	// FromFieldPath is specified.
	// +optional
	FromFieldPath *string `json:"fromFieldPath,omitempty"`

	// Value that will be propagated to the connection secret of the composition
	// instance. Typically you should use FromConnectionSecretKey instead, but
	// an explicit value may be set to inject a fixed, non-sensitive connection
	// secret values, for example a well-known port. Supercedes
	// FromConnectionSecretKey when set.
	// +optional
	Value *string `json:"value,omitempty"`
}

// CompositionStatus shows the observed state of the composition.
type CompositionStatus struct {
	xpv1.ConditionedStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient
// +genclient:nonNamespaced

// Composition defines the group of resources to be created when a compatible
// type is created with reference to the composition.
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=crossplane
type Composition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CompositionSpec   `json:"spec,omitempty"`
	Status CompositionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CompositionList contains a list of Compositions.
type CompositionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Composition `json:"items"`
}
