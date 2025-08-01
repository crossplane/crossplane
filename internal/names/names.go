/*
Copyright 2025 The Crossplane Authors.

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

// Forked from https://github.com/knative/pkg/blob/ee1db869c7ef25eb4ac5c9ba0ab73fdc3f1b9dfa/kmeta/names.go

/*
copyright 2019 the knative authors

licensed under the apache license, version 2.0 (the "license");
you may not use this file except in compliance with the license.
you may obtain a copy of the license at

    http://www.apache.org/licenses/license-2.0

unless required by applicable law or agreed to in writing, software
distributed under the license is distributed on an "as is" basis,
without warranties or conditions of any kind, either express or implied.
see the license for the specific language governing permissions and
limitations under the license.
*/

package names

import (
	"crypto/md5" //nolint:gosec // No strong cryptography needed.
	"encoding/hex"
	"fmt"
	"regexp"
)

// The longest name supported by the K8s is 63.
// These constants.
const (
	longest = 63
	md5Len  = 32
	head    = longest - md5Len // How much to truncate to fit the hash.
)

var isAlphanumeric = regexp.MustCompile(`^[a-zA-Z0-9]*$`)

// ChildName generates a name for the resource based upon the parent resource and suffix.
// If the concatenated name is longer than K8s permits the name is hashed and truncated to permit
// construction of the resource, but still keeps it unique.
// If the suffix itself is longer than 31 characters, then the whole string will be hashed
// and `parent|hash|suffix` will be returned, where parent and suffix will be trimmed to
// fit (prefix of parent at most of length 31, and prefix of suffix at most length 30).
func ChildName(parent, suffix string) string {
	n := parent
	if len(parent) > (longest - len(suffix)) {
		// If the suffix is longer than the longest allowed suffix, then
		// we hash the whole combined string and use that as the suffix.
		if head-len(suffix) <= 0 {
			//nolint:gosec // No strong cryptography needed.
			h := md5.Sum([]byte(parent + suffix))
			// 1. trim parent, if needed
			if head < len(parent) {
				parent = parent[:head]
			}
			// Format the return string, if it's shorter than longest: pad with
			// beginning of the suffix. This happens, for example, when parent is
			// short, but the suffix is very long.
			ret := parent + hex.EncodeToString(h[:])
			if d := longest - len(ret); d > 0 {
				ret += suffix[:d]
			}
			return makeValidName(ret)
		}
		//nolint:gosec // No strong cryptography needed.
		n = fmt.Sprintf("%s%x", parent[:head-len(suffix)], md5.Sum([]byte(parent)))
	}
	return n + suffix
}

// If due to trimming above we're terminating the string with a non-alphanumeric
// character, remove it.
func makeValidName(n string) string {
	for i := len(n) - 1; !isAlphanumeric.MatchString(string(n[i])); i-- {
		n = n[:len(n)-1]
	}
	return n
}
