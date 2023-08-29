// Package schema defines helpers for working with JSON schema.
// As defined by https://datatracker.ietf.org/doc/html/draft-zyp-json-schema-04
package schema

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

const (
	errFmtUnknownJSONType     = "unknown JSON type: %q"
	errFmtUnsupportedJSONType = "JSON type not supported: %q"
)

// KnownJSONType is all the known JSON types.
// See https://datatracker.ietf.org/doc/html/draft-zyp-json-schema-04#section-3.5
type KnownJSONType string

const (
	// KnownJSONTypeArray is the JSON type for arrays.
	KnownJSONTypeArray KnownJSONType = "array"
	// KnownJSONTypeBoolean is the JSON type for booleans.
	KnownJSONTypeBoolean KnownJSONType = "boolean"
	// KnownJSONTypeInteger is the JSON type for integers.
	KnownJSONTypeInteger KnownJSONType = "integer"
	// KnownJSONTypeNull is the JSON type for null.
	KnownJSONTypeNull KnownJSONType = "null"
	// KnownJSONTypeNumber is the JSON type for numbers.
	KnownJSONTypeNumber KnownJSONType = "number"
	// KnownJSONTypeObject is the JSON type for objects.
	KnownJSONTypeObject KnownJSONType = "object"
	// KnownJSONTypeString is the JSON type for strings.
	KnownJSONTypeString KnownJSONType = "string"
)

// IsEquivalent returns true if the two supplied types are equal, or if the first
// type is an integer and the second is a number. This is because the JSON
// schema spec allows integers to be used in place of numbers.
func (t KnownJSONType) IsEquivalent(t2 KnownJSONType) bool {
	// integer is a subset of number per JSON specification:
	// https://datatracker.ietf.org/doc/html/draft-zyp-json-schema-04#section-3.5
	return t == t2 || (t == KnownJSONTypeInteger && t2 == KnownJSONTypeNumber)
}

// IsValid returns true if the supplied string is a known JSON type.
func IsValid(t string) bool {
	switch KnownJSONType(t) {
	case KnownJSONTypeArray, KnownJSONTypeBoolean, KnownJSONTypeInteger, KnownJSONTypeNull, KnownJSONTypeNumber, KnownJSONTypeObject, KnownJSONTypeString:
		return true
	default:
		return false
	}
}

// FromTransformIOType returns the matching JSON type for the given TransformIOType.
// It returns an empty string if the type is not valid, call IsValid() before
// calling this method.
func FromTransformIOType(c v1.TransformIOType) KnownJSONType {
	switch c {
	case v1.TransformIOTypeString:
		return KnownJSONTypeString
	case v1.TransformIOTypeBool:
		return KnownJSONTypeBoolean
	case v1.TransformIOTypeInt, v1.TransformIOTypeInt64:
		return KnownJSONTypeInteger
	case v1.TransformIOTypeFloat64:
		return KnownJSONTypeNumber
	case v1.TransformIOTypeObject:
		return KnownJSONTypeObject
	case v1.TransformIOTypeArray:
		return KnownJSONTypeArray
	}
	// should never happen
	return ""
}

// FromKnownJSONType returns the TransformIOType for the given KnownJSONType.
func FromKnownJSONType(t KnownJSONType) (v1.TransformIOType, error) {
	switch t {
	case KnownJSONTypeString:
		return v1.TransformIOTypeString, nil
	case KnownJSONTypeBoolean:
		return v1.TransformIOTypeBool, nil
	case KnownJSONTypeInteger:
		return v1.TransformIOTypeInt64, nil
	case KnownJSONTypeNumber:
		return v1.TransformIOTypeFloat64, nil
	case KnownJSONTypeObject:
		return v1.TransformIOTypeObject, nil
	case KnownJSONTypeArray:
		return v1.TransformIOTypeObject, nil
	case KnownJSONTypeNull:
		return "", errors.Errorf(errFmtUnsupportedJSONType, t)
	default:
		return "", errors.Errorf(errFmtUnknownJSONType, t)
	}
}
