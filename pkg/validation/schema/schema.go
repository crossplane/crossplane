/*
Copyright 2023 The Crossplane Authors.

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

// Package schema defines helpers for working with JSON schema.
// As defined by https://datatracker.ietf.org/doc/html/draft-zyp-json-schema-04
package schema

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
	return t == t2 || (t == KnownJSONTypeInteger && t2 == KnownJSONTypeNumber)
}

// IsKnownJSONType returns true if the supplied string is a known JSON type.
func IsKnownJSONType(t string) bool {
	switch KnownJSONType(t) {
	case KnownJSONTypeArray, KnownJSONTypeBoolean, KnownJSONTypeInteger, KnownJSONTypeNull, KnownJSONTypeNumber, KnownJSONTypeObject, KnownJSONTypeString:
		return true
	default:
		return false
	}
}
