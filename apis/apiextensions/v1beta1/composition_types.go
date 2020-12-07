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

package v1beta1

import (
	"encoding/json"
	"fmt"
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

// TypeReadinessCheck is used for readiness check types.
type TypeReadinessCheck string

// The possible values for readiness check type.
const (
	ReadinessCheckNonEmpty     TypeReadinessCheck = "NonEmpty"
	ReadinessCheckMatchString  TypeReadinessCheck = "MatchString"
	ReadinessCheckMatchInteger TypeReadinessCheck = "MatchInteger"
	ReadinessCheckNone         TypeReadinessCheck = "None"
)

// ReadinessCheck is used to indicate how to tell whether a resource is ready
// for consumption
type ReadinessCheck struct {
	// Type indicates the type of probe you'd like to use.
	// +kubebuilder:validation:Enum="MatchString";"MatchInteger";"NonEmpty";"None"
	Type TypeReadinessCheck `json:"type"`

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
)

// Patch objects are applied between composite and composed resources. Their
// behaviour depends on the Type selected. The default Type,
// FromCompositeFieldPath, copies a value from the composite resource to
// the composed resource, applying any defined transformers.
type Patch struct {
	// Type sets the patching behaviour to be used. Each patch type may require
	// its' own fields to be set on the Patch object.
	// +optional
	// +kubebuilder:validation:Enum=FromCompositeFieldPath;PatchSet
	// +kubebuilder:default=FromCompositeFieldPath
	Type PatchType `json:"type,omitempty"`

	// FromFieldPath is the path of the field on the upstream resource whose value
	// to be used as input. Required when type is FromCompositeFieldPath.
	// +optional
	FromFieldPath *string `json:"fromFieldPath,omitempty"`

	// ToFieldPath is the path of the field on the base resource whose value will
	// be changed with the result of transforms. Leave empty if you'd like to
	// propagate to the same path on the target resource.
	// +optional
	ToFieldPath *string `json:"toFieldPath,omitempty"`

	// PatchSetName to include patches from. Required when type is PatchSet.
	// +optional
	PatchSetName *string `json:"patchSetName,omitempty"`

	// Transforms are the list of functions that are used as a FIFO pipe for the
	// input to be transformed.
	// +optional
	Transforms []Transform `json:"transforms,omitempty"`
}

// Apply executes a patching operation between the from and to resources.
func (c *Patch) Apply(from, to runtime.Object) error {
	switch c.Type {
	case PatchTypeFromCompositeFieldPath:
		return c.applyFromCompositeFieldPatch(from, to)
	case PatchTypePatchSet:
		// Already resolved - nothing to do.
	}
	return errors.Errorf(errInvalidPatchType, c.Type)
}

// applyFromCompositeFieldPatch patches the composed resource, using a source field
// on the composite resource. Values may be transformed if any are defined on
// the patch.
func (c *Patch) applyFromCompositeFieldPatch(from, to runtime.Object) error { // nolint:gocyclo
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
	if fieldpath.IsNotFound(err) {
		// A composition may want to opportunistically patch from a field path
		// that may or may not exist in the composite, for example by patching
		// {fromFieldPath: metadata.labels, toFieldPath: metadata.labels}. We
		// don't consider a reference to a non-existent path to be an issue; if
		// the relevant toFieldPath is required by the composed resource we'll
		// report that fact when we attempt to reconcile the composite.
		return nil
	}
	if err != nil {
		return err
	}
	out := in
	for i, f := range c.Transforms {
		if out, err = f.Transform(out); err != nil {
			return errors.Wrap(err, fmt.Sprintf(errFmtTransformAtIndex, i))
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
		return nil, errors.New(fmt.Sprintf(errFmtTypeNotSupported, string(t.Type)))
	}
	// An interface equals nil only if both the type and value are nil. Above,
	// even if t.<Type> is nil, its type is assigned to "transformer" but we're
	// interested in whether only the value is nil or not.
	if reflect.ValueOf(transformer).IsNil() {
		return nil, errors.New(fmt.Sprintf(errFmtConfigMissing, string(t.Type)))
	}
	out, err := transformer.Resolve(input)
	return out, errors.Wrap(err, fmt.Sprintf(errFmtTransformTypeFailed, string(t.Type)))
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
			return nil, errors.New(fmt.Sprintf(errFmtMapNotFound, i))
		}
		return val, nil
	default:
		return nil, errors.New(fmt.Sprintf(errFmtMapTypeNotSupported, reflect.TypeOf(input).String()))
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
	ConvertTransformTypeFloat64 = "float64"
)

var (
	conversions = map[string]func(interface{}) (interface{}, error){
		"string.string": func(i interface{}) (interface{}, error) {
			return i, nil
		},
		"string.int": func(i interface{}) (interface{}, error) {
			return strconv.Atoi(i.(string))
		},
		"string.bool": func(i interface{}) (interface{}, error) {
			return strconv.ParseBool(i.(string))
		},
		"string.float64": func(i interface{}) (interface{}, error) {
			return strconv.ParseFloat(i.(string), 64)
		},

		"int.string": func(i interface{}) (interface{}, error) { // nolint:unparam
			return strconv.Itoa(i.(int)), nil
		},
		"int.int": func(i interface{}) (interface{}, error) { // nolint:unparam
			return i.(int), nil
		},
		"int.bool": func(i interface{}) (interface{}, error) { // nolint:unparam
			return i.(int) == 1, nil
		},
		"int.float64": func(i interface{}) (interface{}, error) { // nolint:unparam
			return float64(i.(int)), nil
		},

		"bool.string": func(i interface{}) (interface{}, error) { // nolint:unparam
			return strconv.FormatBool(i.(bool)), nil
		},
		"bool.int": func(i interface{}) (interface{}, error) { // nolint:unparam
			if i.(bool) {
				return 1, nil
			}
			return 0, nil
		},
		"bool.bool": func(i interface{}) (interface{}, error) { // nolint:unparam
			return i.(bool), nil
		},
		"bool.float64": func(i interface{}) (interface{}, error) { // nolint:unparam
			if i.(bool) {
				return float64(1), nil
			}
			return float64(0), nil
		},

		"float64.string": func(i interface{}) (interface{}, error) { // nolint:unparam
			return strconv.FormatFloat(i.(float64), 'f', -1, 64), nil
		},
		"float64.int": func(i interface{}) (interface{}, error) { // nolint:unparam
			return int(i.(float64)), nil
		},
		"float64.bool": func(i interface{}) (interface{}, error) { // nolint:unparam
			return i.(float64) == float64(1), nil
		},
		"float64.float64": func(i interface{}) (interface{}, error) {
			return strconv.ParseFloat(i.(string), 64)
		},
	}
)

// A ConvertTransform converts the input into a new object whose type is supplied.
type ConvertTransform struct {
	// ToType is the type of the output of this transform.
	// +kubebuilder:validation:Enum=string;int;bool;float64
	ToType string `json:"toType"`
}

// Resolve runs the String transform.
func (s *ConvertTransform) Resolve(input interface{}) (interface{}, error) {
	switch reflect.TypeOf(input).Kind().String() {
	case ConvertTransformTypeString, ConvertTransformTypeBool, ConvertTransformTypeInt, ConvertTransformTypeFloat64:
		break
	default:
		return nil, errors.New(fmt.Sprintf(errFmtConvertInputTypeNotSupported, reflect.TypeOf(input).Kind().String()))
	}
	f, ok := conversions[fmt.Sprintf("%s.%s", reflect.TypeOf(input).Kind().String(), s.ToType)]
	if !ok {
		return nil, errors.New(fmt.Sprintf(errFmtConversionPairNotSupported, reflect.TypeOf(input).Kind().String(), s.ToType))
	}
	return f(input)
}

// ConnectionDetail includes the information about the propagation of the connection
// information from one secret to another.
type ConnectionDetail struct {
	// Name of the connection secret key that will be propagated to the
	// connection secret of the composition instance. Leave empty if you'd like
	// to use the same key name.
	// +optional
	Name *string `json:"name,omitempty"`

	// FromConnectionSecretKey is the key that will be used to fetch the value
	// from the given target resource.
	// +optional
	FromConnectionSecretKey *string `json:"fromConnectionSecretKey,omitempty"`

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
