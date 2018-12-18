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
	"strings"
)

func ToLowerRemoveSpaces(input string) string {
	return strings.ToLower(strings.Replace(input, " ", "", -1))
}

// IfEmptyString test input string and if empty, i.e = "", return a replacement string
func IfEmptyString(s, r string) string {
	if s == "" {
		return r
	}
	return s
}

func String(s string) *string {
	return &s
}

func StringValue(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}
