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

package xpkg

import (
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// parseAnnotations parses a slice of "key=value" strings into a map. Returns
// an error if any entry is not in key=value format.
func parseAnnotations(kvs []string) (map[string]string, error) {
	anns := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, errors.Errorf("invalid annotation %q: must be in key=value format", kv)
		}
		anns[k] = v
	}
	return anns, nil
}

// mergeAnnotations merges metadata annotations with flag annotations. Flag
// annotations take precedence over metadata annotations when keys overlap.
func mergeAnnotations(metaAnnotations map[string]string, flagAnnotations []string) (map[string]string, error) {
	anns := make(map[string]string)
	for k, v := range metaAnnotations {
		anns[k] = v
	}
	flagAnns, err := parseAnnotations(flagAnnotations)
	if err != nil {
		return nil, err
	}
	for k, v := range flagAnns {
		anns[k] = v
	}
	return anns, nil
}
