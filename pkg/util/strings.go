/*
Copyright 2018 The Crossplane Authors.

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

package util

import (
	"strconv"
	"strings"
)

// ToLowerRemoveSpaces returns the supplied string in lowercase with all spaces
// (not all whitespace) removed.
func ToLowerRemoveSpaces(input string) string {
	return strings.ToLower(strings.Replace(input, " ", "", -1))
}

// String returns a pointer to the string value passed in.
func String(v string) *string {
	return &v
}

// StringValue returns the value of the string pointer passed in or
// "" if the pointer is nil.
func StringValue(v *string) string {
	if v != nil {
		return *v
	}
	return ""
}

// ParseMap string encoded map values
// example: "foo:bar,one:two" -> map[string]string{"foo":"bar","one":"two"}
func ParseMap(s string) map[string]string {
	m := map[string]string{}
	for _, cfg := range strings.Split(s, ",") {
		if kv := strings.SplitN(cfg, ":", 2); len(kv) == 2 {
			m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return m
}

// ParseBool returns true IFF string value is "true" or "True"
func ParseBool(s string) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false
	}
	return b
}
