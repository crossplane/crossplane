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

// Package truncate provides functions for truncating Kubernetes values in a
// predictable way offering mildly collision safe values usable in
// deterministic field searches.
package truncate

import (
	// sha1 is not cryptographically secure, but that is not a goal. we require
	// predictable and uniform text transformation. Any checksum function with
	// even distribution would suffice.
	"crypto/sha1" // nolint:gosec
	"encoding/base32"
	"fmt"
	"strings"
)

const (
	// LabelNameLength is the max length of a name fragment in a label
	LabelNameLength = 63

	// LabelValueLength is the max length of a label value
	LabelValueLength = 63

	// ResourceNameLength is the max length of a resource name or a label with
	// optional prefix and name
	ResourceNameLength = 253

	// DefaultSuffixLength is the length of combined separator and checksum
	// characters that will be used as a truncation suffix in Label and Resource
	// truncation functions
	DefaultSuffixLength = 6

	sha1Length = 40
)

// Truncate replaces the suffixLength number of trailing characters from str
// with a consistent hash fragment based on that string. The suffix will include
// a leading hyphen.
//
// An error will be returned if the truncation length is less than the
// suffixLength, or truncation length is greater than sha1Length, or the suffix
// length is less than 2.
//
// Any final "." characters in the truncated text will be removed to satisfy
// DNS-1123 labeling.
//
// Example: If the base32 sum of a digest of "aaaaaaaaaaa" is "ovo", with a
// suffix length of 4:
//
// Truncating this string to a length of 11 would result in "aaaaaaaaaaa"
// Truncating this string to a length of 8 would result in "aaaa-ovo"
// Truncating this string to a length of 7 would result in "aaa-ovo"
// Truncating this string to a length of 5 would result in "a-ovo"
// Truncating this string to a length of 4 would result in "-ovo"
func Truncate(str string, length, suffixLength int) (string, error) {
	if len(str) <= length {
		return str, nil
	}

	if length < suffixLength {
		return "", fmt.Errorf("truncate length is less than suffixLength: %d < %d", length, suffixLength)
	}

	if suffixLength > sha1Length {
		return "", fmt.Errorf("truncate suffixLength exceeds max length: %d > %d", suffixLength, sha1Length)
	}

	if suffixLength < 2 {
		return "", fmt.Errorf("truncate suffixLength is less than minimum: %d < 2", suffixLength)
	}

	// See the import comment regarding use of sha1
	// nolint:gosec
	checksum := sha1.Sum([]byte(str))
	retainedLength := length - suffixLength
	retained := strings.TrimRight(str[0:retainedLength], ".")

	// base64 includes "/" which can only occur once in label names
	b32 := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(checksum[:])
	separator := "-"
	suffix := strings.ToLower(b32[0 : suffixLength-len(separator)])

	return fmt.Sprintf("%s%s%s", retained, separator, suffix), nil
}

// LabelName predictably truncates the supplied string using label name
// length restrictions and a uniform distribution suffix based on the string
func LabelName(str string) string {
	s, _ := Truncate(str, LabelNameLength, DefaultSuffixLength)
	return s
}

// LabelValue predictably truncates the supplied string using label value
// length restrictions and a uniform distribution suffix based on the string
func LabelValue(str string) string {
	s, _ := Truncate(str, LabelValueLength, DefaultSuffixLength)
	return s
}

// ResourceName predictably truncates the supplied string using resource name
// length restrictions and a uniform distribution suffix based on the string
func ResourceName(str string) string {
	s, _ := Truncate(str, ResourceNameLength, DefaultSuffixLength)
	return s
}
