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

package v1

import (
	"regexp"
	"strings"
)

// Known function capabilities. This shouldn't be treated as an exhaustive list.
// A package's capabilities array may contain arbitrary entries that aren't
// meaningful to Crossplane but might be meaningful to some other consumer.
const (
	// FunctionCapabilityComposition is a capability key for a function that
	// can be used in a composition.
	FunctionCapabilityComposition = "composition"

	// FunctionCapabilityOperation is a capability key for function that can be
	// used in an operation.
	FunctionCapabilityOperation = "operation"

	// ProviderCapabilitySafeStart is a capability key for a provider that
	// supports "safe" starting of its controller gated on the existence of
	// dependent kinds in the cluster.
	ProviderCapabilitySafeStart = "safe-start"
)

// CapabilitiesContainFuzzyMatch will look in the list of capabilities and fuzzy match it to the given key.
// Fuzzy match is defined as stripping all non-alphanumeric characters and ignoring case.
func CapabilitiesContainFuzzyMatch(capabilities []string, key string) bool {
	// fuz will match only alphanumeric characters.
	fuz := regexp.MustCompile("[^a-zA-Z0-9]+")
	key = fuz.ReplaceAllString(key, "")
	key = strings.ToLower(key)
	for _, c := range capabilities {
		c = fuz.ReplaceAllString(c, "")
		c = strings.ToLower(c)
		if c == key {
			return true
		}
	}
	return false
}
