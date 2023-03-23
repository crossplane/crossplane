/*
Copyright 2022 The Crossplane Authors.

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

// Generated from apiextensions/v1beta1/revision_types.go by ../hack/duplicate_api_type.sh. DO NOT EDIT.

package v1alpha1

import (
	"encoding/json"
	"reflect"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

const (
	errFmtMapNotFound         = "key %s is not found in map"
	errFmtMapTypeNotSupported = "type %s is not supported for map transform"
)

// An EnvironmentConfiguration specifies the environment for rendering composed
// resources.
type EnvironmentConfiguration struct {
	// EnvironmentConfigs selects a list of `EnvironmentConfig`s. The resolved
	// resources are stored in the composite resource at
	// `spec.environmentConfigRefs` and is only updated if it is null.
	//
	// The list of references is used to compute an in-memory environment at
	// compose time. The data of all object is merged in the order they are
	// listed, meaning the values of EnvironmentConfigs with a larger index take
	// priority over ones with smaller indices.
	//
	// The computed environment can be accessed in a composition using
	// `FromEnvironmentFieldPath` and `CombineFromEnvironment` patches.
	// +optional
	EnvironmentConfigs []EnvironmentSource `json:"environmentConfigs,omitempty"`

	// Patches is a list of environment patches that are executed before a
	// composition's resources are composed.
	Patches []EnvironmentPatch `json:"patches,omitempty"`
}

// EnvironmentSourceType specifies the way the EnvironmentConfig is selected.
type EnvironmentSourceType string

const (
	// EnvironmentSourceTypeReference by name.
	EnvironmentSourceTypeReference EnvironmentSourceType = "Reference"
	// EnvironmentSourceTypeSelector by labels.
	EnvironmentSourceTypeSelector EnvironmentSourceType = "Selector"
)

// EnvironmentSource selects a EnvironmentConfig resource.
type EnvironmentSource struct {
	// Type specifies the way the EnvironmentConfig is selected.
	// Default is `Reference`
	// +optional
	// +kubebuilder:validation:Enum=Reference;Selector
	// +kubebuilder:default=Reference
	Type EnvironmentSourceType `json:"type,omitempty"`

	// Ref is a named reference to a single EnvironmentConfig.
	// Either Ref or Selector is required.
	// +optional
	Ref *EnvironmentSourceReference `json:"ref,omitempty"`

	// Selector selects one EnvironmentConfig via labels.
	// +optional
	Selector *EnvironmentSourceSelector `json:"selector,omitempty"`
}

// An EnvironmentSourceReference references an EnvironmentConfig by it's name.
type EnvironmentSourceReference struct {
	// The name of the object.
	Name string `json:"name"`
}

// An EnvironmentSourceSelector selects an EnvironmentConfig via labels.
type EnvironmentSourceSelector struct {
	// MatchLabels ensures an object with matching labels is selected.
	MatchLabels []EnvironmentSourceSelectorLabelMatcher `json:"matchLabels,omitempty"`
}

// EnvironmentSourceSelectorLabelMatcherType specifies where the value for a
// label comes from.
type EnvironmentSourceSelectorLabelMatcherType string

const (
	// EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath extracts
	// the label value from a composite fieldpath.
	EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath EnvironmentSourceSelectorLabelMatcherType = "FromCompositeFieldPath"
	// EnvironmentSourceSelectorLabelMatcherTypeValue uses a literal as label
	// value.
	EnvironmentSourceSelectorLabelMatcherTypeValue EnvironmentSourceSelectorLabelMatcherType = "Value"
)

// An EnvironmentSourceSelectorLabelMatcher acts like a k8s label selector but
// can draw the label value from a different path.
type EnvironmentSourceSelectorLabelMatcher struct {
	// Type specifies where the value for a label comes from.
	// +optional
	// +kubebuilder:validation:Enum=FromCompositeFieldPath;Value
	// +kubebuilder:default=FromCompositeFieldPath
	Type EnvironmentSourceSelectorLabelMatcherType `json:"type"`

	// Key of the label to match.
	Key string `json:"key"`

	// ValueFromFieldPath specifies the field path to look for the label value.
	ValueFromFieldPath *string `json:"valueFromFieldPath,omitempty"`

	// Value specifies a literal label value.
	Value *string `json:"value,omitempty"`
}

// EnvironmentPatch is a patch for a Composition environment.
type EnvironmentPatch struct {
	// Type sets the patching behaviour to be used. Each patch type may require
	// its own fields to be set on the Patch object.
	// +optional
	// +kubebuilder:validation:Enum=FromCompositeFieldPath;ToCompositeFieldPath;CombineFromComposite;CombineToComposite
	// +kubebuilder:default=FromCompositeFieldPath
	Type PatchType `json:"type,omitempty"`

	// FromFieldPath is the path of the field on the resource whose value is
	// to be used as input. Required when type is FromCompositeFieldPath or
	// ToCompositeFieldPath.
	// +optional
	FromFieldPath *string `json:"fromFieldPath,omitempty"`

	// Combine is the patch configuration for a CombineFromComposite or
	// CombineToComposite patch.
	// +optional
	Combine *Combine `json:"combine,omitempty"`

	// ToFieldPath is the path of the field on the resource whose value will
	// be changed with the result of transforms. Leave empty if you'd like to
	// propagate to the same path as fromFieldPath.
	// +optional
	ToFieldPath *string `json:"toFieldPath,omitempty"`

	// Transforms are the list of functions that are used as a FIFO pipe for the
	// input to be transformed.
	// +optional
	Transforms []Transform `json:"transforms,omitempty"`

	// Policy configures the specifics of patching behaviour.
	// +optional
	Policy *PatchPolicy `json:"policy,omitempty"`
}

const (
	// LabelCompositionName is the name of the Composition used to create
	// this CompositionRevision.
	LabelCompositionName = "crossplane.io/composition-name"

	// LabelCompositionHash is a hash of the Composition label, annotation
	// and spec used to create this CompositionRevision. Used to identify
	// identical revisions.
	LabelCompositionHash = "crossplane.io/composition-hash"
)

// CompositionRevisionSpec specifies the desired state of the composition
// revision.
type CompositionRevisionSpec struct {
	// CompositeTypeRef specifies the type of composite resource that this
	// composition is compatible with.
	// +immutable
	CompositeTypeRef TypeReference `json:"compositeTypeRef"`

	// PatchSets define a named set of patches that may be included by
	// any resource in this Composition.
	// PatchSets cannot themselves refer to other PatchSets.
	// +optional
	PatchSets []PatchSet `json:"patchSets,omitempty"`

	// Environment configures the environment in which resources are rendered.
	// +optional
	Environment *EnvironmentConfiguration `json:"environment,omitempty"`

	// Resources is the list of resource templates that will be used when a
	// composite resource referring to this composition is created.
	// +optional
	Resources []ComposedTemplate `json:"resources"`

	// Functions is list of Composition Functions that will be used when a
	// composite resource referring to this composition is created. At least one
	// of resources and functions must be specified. If both are specified the
	// resources will be rendered first, then passed to the functions for
	// further processing.
	// +optional
	Functions []Function `json:"functions,omitempty"`

	// WriteConnectionSecretsToNamespace specifies the namespace in which the
	// connection secrets of composite resource dynamically provisioned using
	// this composition will be created.
	// This field is planned to be removed in a future release in favor of
	// PublishConnectionDetailsWithStoreConfigRef. Currently, both could be
	// set independently and connection details would be published to both
	// without affecting each other as long as related fields at MR level
	// specified.
	// +optional
	WriteConnectionSecretsToNamespace *string `json:"writeConnectionSecretsToNamespace,omitempty"`

	// PublishConnectionDetailsWithStoreConfig specifies the secret store config
	// with which the connection details of composite resources dynamically
	// provisioned using this composition will be published.
	// +optional
	// +kubebuilder:default={"name": "default"}
	PublishConnectionDetailsWithStoreConfigRef *StoreConfigReference `json:"publishConnectionDetailsWithStoreConfigRef,omitempty"`

	// Revision number. Newer revisions have larger numbers.
	// +immutable
	Revision int64 `json:"revision"`
}

// A StoreConfigReference references a secret store config that may be used to
// write connection details.
type StoreConfigReference struct {
	// Name of the referenced StoreConfig.
	Name string `json:"name"`
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
	// +immutable
	Name *string `json:"name,omitempty"`

	// Base is the target resource that the patches will be applied on.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:EmbeddedResource
	// +immutable
	Base runtime.RawExtension `json:"base"`

	// Patches will be applied as overlay to the base resource.
	// +optional
	// +immutable
	Patches []Patch `json:"patches,omitempty"`

	// ConnectionDetails lists the propagation secret keys from this target
	// resource to the composition instance connection secret.
	// +optional
	// +immutable
	ConnectionDetails []ConnectionDetail `json:"connectionDetails,omitempty"`

	// ReadinessChecks allows users to define custom readiness checks. All checks
	// have to return true in order for resource to be considered ready. The
	// default readiness check is to have the "Ready" condition to be "True".
	// +optional
	// +immutable
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
	// +immutable
	Type ReadinessCheckType `json:"type"`

	// FieldPath shows the path of the field whose value will be used.
	// +optional
	// +immutable
	FieldPath string `json:"fieldPath,omitempty"`

	// MatchString is the value you'd like to match if you're using "MatchString" type.
	// +optional
	// +immutable
	MatchString string `json:"matchString,omitempty"`

	// MatchInt is the value you'd like to match if you're using "MatchInt" type.
	// +optional
	// +immutable
	MatchInteger int64 `json:"matchInteger,omitempty"`
}

// A PatchType is a type of patch.
type PatchType string

// Patch types.
const (
	PatchTypeFromCompositeFieldPath PatchType = "FromCompositeFieldPath" // Default
	PatchTypePatchSet               PatchType = "PatchSet"
	PatchTypeToCompositeFieldPath   PatchType = "ToCompositeFieldPath"
	PatchTypeCombineFromComposite   PatchType = "CombineFromComposite"
	PatchTypeCombineToComposite     PatchType = "CombineToComposite"
)

// Patch objects are applied between composite and composed resources. Their
// behaviour depends on the Type selected. The default Type,
// FromCompositeFieldPath, copies a value from the composite resource to
// the composed resource, applying any defined transformers.
type Patch struct {
	// Type sets the patching behaviour to be used. Each patch type may require
	// its own fields to be set on the Patch object.
	// +optional
	// +immutable
	// +kubebuilder:validation:Enum=FromCompositeFieldPath;FromEnvironmentFieldPath;PatchSet;ToCompositeFieldPath;ToEnvironmentFieldPath;CombineFromEnvironment;CombineFromComposite;CombineToComposite;CombineToEnvironment
	// +kubebuilder:default=FromCompositeFieldPath
	Type PatchType `json:"type,omitempty"`

	// FromFieldPath is the path of the field on the resource whose value is
	// to be used as input. Required when type is FromCompositeFieldPath or
	// ToCompositeFieldPath.
	// +optional
	// +immutable
	FromFieldPath *string `json:"fromFieldPath,omitempty"`

	// Combine is the patch configuration for a CombineFromComposite or
	// CombineToComposite patch.
	// +optional
	// +immutable
	Combine *Combine `json:"combine,omitempty"`

	// ToFieldPath is the path of the field on the resource whose value will
	// be changed with the result of transforms. Leave empty if you'd like to
	// propagate to the same path as fromFieldPath.
	// +optional
	ToFieldPath *string `json:"toFieldPath,omitempty"`

	// PatchSetName to include patches from. Required when type is PatchSet.
	// +optional
	// +immutable
	PatchSetName *string `json:"patchSetName,omitempty"`

	// Transforms are the list of functions that are used as a FIFO pipe for the
	// input to be transformed.
	// +optional
	// +immutable
	Transforms []Transform `json:"transforms,omitempty"`

	// Policy configures the specifics of patching behaviour.
	// +optional
	// +immutable
	Policy *PatchPolicy `json:"policy,omitempty"`
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
	// +immutable
	FromFieldPath *FromFieldPathPolicy `json:"fromFieldPath,omitempty"`
	MergeOptions  *xpv1.MergeOptions   `json:"mergeOptions,omitempty"`
}

// A Combine configures a patch that combines more than
// one input field into a single output field.
type Combine struct {
	// Variables are the list of variables whose values will be retrieved and
	// combined.
	// +kubebuilder:validation:MinItems=1
	// +immutable
	Variables []CombineVariable `json:"variables"`

	// Strategy defines the strategy to use to combine the input variable values.
	// Currently only string is supported.
	// +kubebuilder:validation:Enum=string
	// +immutable
	Strategy CombineStrategy `json:"strategy"`

	// String declares that input variables should be combined into a single
	// string, using the relevant settings for formatting purposes.
	// +optional
	// +immutable
	String *StringCombine `json:"string,omitempty"`
}

// A CombineVariable defines the source of a value that is combined with
// others to form and patch an output value. Currently, this only supports
// retrieving values from a field path.
type CombineVariable struct {
	// FromFieldPath is the path of the field on the source whose value is
	// to be used as input.
	// +immutable
	FromFieldPath string `json:"fromFieldPath"`
}

// A CombineStrategy determines what strategy will be applied to combine
// variables.
type CombineStrategy string

// CombineStrategy strategy definitions.
const (
	CombineStrategyString CombineStrategy = "string"
)

// A StringCombine combines multiple input values into a single string.
type StringCombine struct {
	// Format the input using a Go format string. See
	// https://golang.org/pkg/fmt/ for details.
	// +immutable
	Format string `json:"fmt"`
}

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
	// +immutable
	Type TransformType `json:"type"`

	// Math is used to transform the input via mathematical operations such as
	// multiplication.
	// +optional
	// +immutable
	Math *MathTransform `json:"math,omitempty"`

	// Map uses the input as a key in the given map and returns the value.
	// +optional
	// +immutable
	Map *MapTransform `json:"map,omitempty"`

	// Match is a more complex version of Map that matches a list of patterns.
	// +optional
	Match *MatchTransform `json:"match,omitempty"`

	// String is used to transform the input into a string or a different kind
	// of string. Note that the input does not necessarily need to be a string.
	// +optional
	// +immutable
	String *StringTransform `json:"string,omitempty"`

	// Convert is used to cast the input into the given output type.
	// +optional
	// +immutable
	Convert *ConvertTransform `json:"convert,omitempty"`
}

// MathTransform conducts mathematical operations on the input with the given
// configuration in its properties.
type MathTransform struct {
	// Multiply the value.
	// +optional
	// +immutable
	Multiply *int64 `json:"multiply,omitempty"`
}

// MapTransform returns a value for the input from the given map.
type MapTransform struct {
	// TODO(negz): Are Pairs really optional if a MapTransform was specified?

	// Pairs is the map that will be used for transform.
	// +optional
	// +immutable
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

// Resolve runs the Map transform.
func (m *MapTransform) Resolve(input any) (any, error) {
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

// A StringTransformType transforms a string.
type StringTransformType string

// Accepted StringTransformTypes.
const (
	StringTransformTypeFormat     StringTransformType = "Format" // Default
	StringTransformTypeConvert    StringTransformType = "Convert"
	StringTransformTypeTrimPrefix StringTransformType = "TrimPrefix"
	StringTransformTypeTrimSuffix StringTransformType = "TrimSuffix"
	StringTransformTypeRegexp     StringTransformType = "Regexp"
)

// A StringConversionType converts a string.
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

// A ConvertTransformType defines the type of a conversion transform.
type ConvertTransformType string

// The list of supported ConvertTransform input and output types.
const (
	TransformIOTypeString  ConvertTransformType = "string"
	TransformIOTypeBool    ConvertTransformType = "bool"
	TransformIOTypeInt     ConvertTransformType = "int"
	TransformIOTypeInt64   ConvertTransformType = "int64"
	TransformIOTypeFloat64 ConvertTransformType = "float64"
)

// ConvertTransformFormat defines the expected format of an input value of a
// conversion transform.
type ConvertTransformFormat string

// Possible ConvertTransformFormat values.
const (
	ConvertTransformFormatNone     ConvertTransformFormat = "none"
	ConvertTransformFormatQuantity ConvertTransformFormat = "quantity"
)

// A ConvertTransform converts the input into a new object whose type is supplied.
type ConvertTransform struct {
	// ToType is the type of the output of this transform.
	// +kubebuilder:validation:Enum=string;int;int64;bool;float64
	ToType ConvertTransformType `json:"toType"`

	// The expected input format.
	//
	// * `quantity` - parses the input as a K8s [`resource.Quantity`](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/resource#Quantity).
	// Only used during `string -> float64` conversions.
	//
	// If this property is null, the default conversion is applied.
	//
	// +kubebuilder:validation:Enum=quantity
	Format *ConvertTransformFormat `json:"format,omitempty"`
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
	// +immutable
	Name *string `json:"name,omitempty"`

	// Type sets the connection detail fetching behaviour to be used. Each
	// connection detail type may require its own fields to be set on the
	// ConnectionDetail object. If the type is omitted Crossplane will attempt
	// to infer it based on which other fields were specified.
	// +optional
	// +immutable
	// +kubebuilder:validation:Enum=FromConnectionSecretKey;FromFieldPath;FromValue
	Type *ConnectionDetailType `json:"type,omitempty"`

	// FromConnectionSecretKey is the key that will be used to fetch the value
	// from the given target resource's secret.
	// +optional
	// +immutable
	FromConnectionSecretKey *string `json:"fromConnectionSecretKey,omitempty"`

	// FromFieldPath is the path of the field on the composed resource whose
	// value to be used as input. Name must be specified if the type is
	// FromFieldPath is specified.
	// +optional
	// +immutable
	FromFieldPath *string `json:"fromFieldPath,omitempty"`

	// Value that will be propagated to the connection secret of the composition
	// instance. Typically you should use FromConnectionSecretKey instead, but
	// an explicit value may be set to inject a fixed, non-sensitive connection
	// secret values, for example a well-known port. Supercedes
	// FromConnectionSecretKey when set.
	// +optional
	// +immutable
	Value *string `json:"value,omitempty"`
}

// A Function represents a Composition Function.
type Function struct {
	// Name of this function. Must be unique within its Composition.
	Name string `json:"name"`

	// Type of this function.
	// +kubebuilder:validation:Enum=Container
	Type FunctionType `json:"type"`

	// Config is an optional, arbitrary Kubernetes resource (i.e. a resource
	// with an apiVersion and kind) that will be passed to the Composition
	// Function as the 'config' block of its FunctionIO.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:EmbeddedResource
	Config *runtime.RawExtension `json:"config,omitempty"`

	// Container configuration of this function.
	// +optional
	Container *ContainerFunction `json:"container,omitempty"`
}

// A FunctionType is a type of Composition Function.
type FunctionType string

// FunctionType types.
const (
	// FunctionTypeContainer represents a Composition Function that is packaged
	// as an OCI image and run in a container.
	FunctionTypeContainer FunctionType = "Container"
)

// A ContainerFunction represents an Composition Function that is packaged as an
// OCI image and run in a container.
type ContainerFunction struct {
	// Image specifies the OCI image in which the function is packaged. The
	// image should include an entrypoint that reads a FunctionIO from stdin and
	// emits it, optionally mutated, to stdout.
	Image string `json:"image"`

	// ImagePullPolicy defines the pull policy for the function image.
	// +optional
	// +kubebuilder:default=IfNotPresent
	// +kubebuilder:validation:Enum="IfNotPresent";"Always";"Never"
	ImagePullPolicy *corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Timeout after which the Composition Function will be killed.
	// +optional
	// +kubebuilder:default="20s"
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Network configuration for the Composition Function.
	// +optional
	Network *ContainerFunctionNetwork `json:"network,omitempty"`

	// Resources that may be used by the Composition Function.
	// +optional
	Resources *ContainerFunctionResources `json:"resources,omitempty"`

	// Runner configuration for the Composition Function.
	// +optional
	Runner *ContainerFunctionRunner `json:"runner,omitempty"`
}

// A ContainerFunctionNetworkPolicy specifies the network policy under which
// a containerized Composition Function will run.
type ContainerFunctionNetworkPolicy string

const (
	// ContainerFunctionNetworkPolicyIsolated specifies that the Composition
	// Function will not have network access; i.e. invoked inside an isolated
	// network namespace.
	ContainerFunctionNetworkPolicyIsolated ContainerFunctionNetworkPolicy = "Isolated"

	// ContainerFunctionNetworkPolicyRunner specifies that the Composition
	// Function will have the same network access as its runner, i.e. share its
	// runner's network namespace.
	ContainerFunctionNetworkPolicyRunner ContainerFunctionNetworkPolicy = "Runner"
)

// ContainerFunctionNetwork represents configuration for a Composition Function.
type ContainerFunctionNetwork struct {
	// Policy specifies the network policy under which the Composition Function
	// will run. Defaults to 'Isolated' - i.e. no network access. Specify
	// 'Runner' to allow the function the same network access as
	// its runner.
	// +optional
	// +kubebuilder:validation:Enum="Isolated";"Runner"
	// +kubebuilder:default=Isolated
	Policy *ContainerFunctionNetworkPolicy `json:"policy,omitempty"`
}

// ContainerFunctionResources represents compute resources that may be used by a
// Composition Function.
type ContainerFunctionResources struct {
	// Limits specify the maximum compute resources that may be used by the
	// Composition Function.
	// +optional
	Limits *ContainerFunctionResourceLimits `json:"limits,omitempty"`

	// NOTE(negz): We don't presently have any runners that support scheduling,
	// so we omit Requests for the time being.
}

// ContainerFunctionResourceLimits specify the maximum compute resources
// that may be used by a Composition Function.
type ContainerFunctionResourceLimits struct {
	// CPU, in cores. (500m = .5 cores)
	// +kubebuilder:default="100m"
	// +optional
	CPU *resource.Quantity `json:"cpu,omitempty"`

	// Memory, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
	// +kubebuilder:default="128Mi"
	// +optional
	Memory *resource.Quantity `json:"memory,omitempty"`
}

// ContainerFunctionRunner represents runner configuration for a Composition
// Function.
type ContainerFunctionRunner struct {
	// Endpoint specifies how and where Crossplane should reach the runner it
	// uses to invoke containerized Composition Functions.
	// +optional
	// +kubebuilder:default="unix:///@crossplane/fn/default.sock"
	Endpoint *string `json:"endpoint,omitempty"`
}

// CompositionRevisionStatus shows the observed state of the composition
// revision.
type CompositionRevisionStatus struct {
	xpv1.ConditionedStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +genclient
// +genclient:nonNamespaced

// A CompositionRevision represents a revision in time of a Composition.
// Revisions are created by Crossplane; they should be treated as immutable.
// +kubebuilder:printcolumn:name="REVISION",type="string",JSONPath=".spec.revision"
// +kubebuilder:printcolumn:name="XR-KIND",type="string",JSONPath=".spec.compositeTypeRef.kind"
// +kubebuilder:printcolumn:name="XR-APIVERSION",type="string",JSONPath=".spec.compositeTypeRef.apiVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories=crossplane
// +kubebuilder:subresource:status
type CompositionRevision struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CompositionRevisionSpec   `json:"spec,omitempty"`
	Status CompositionRevisionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CompositionRevisionList contains a list of CompositionRevisions.
type CompositionRevisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CompositionRevision `json:"items"`
}
