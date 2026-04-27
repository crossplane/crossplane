/*
Copyright 2026 The Crossplane Authors.

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
	"text/template"

	yaml "gopkg.in/yaml.v3"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// packageMetadataFuncMap is bound to package metadata text/template execution. It is
// generic: no keys or types are special-cased for a particular provider or field name.
func packageMetadataFuncMap() template.FuncMap {
	return template.FuncMap{
		"dict":   dict,
		"indent": indent,
		"toYAML": toYAML,
	}
}

// dict builds map[string]any from alternating string keys and values (k1, v1, k2, v2, ...).
// Used with toYAML to emit structured YAML from template data without string concatenation.
func dict(pairs ...any) (map[string]any, error) {
	if len(pairs)%2 != 0 {
		return nil, errors.New("dict: requires an even number of arguments")
	}
	out := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		ks, ok := pairs[i].(string)
		if !ok {
			return nil, errors.Errorf("dict: argument %d must be a string key", i)
		}
		out[ks] = pairs[i+1]
	}
	return out, nil
}

// toYAML marshals v with gopkg.in/yaml.v3 for use inside templates (e.g. block scalars).
func toYAML(v any) (string, error) {
	b, err := yaml.Marshal(v)
	if err != nil {
		return "", errors.Wrap(err, "toYAML")
	}
	return strings.TrimSuffix(string(b), "\n"), nil
}

// indent prefixes each line of s with n spaces. Used to nest a YAML fragment under a
// parent key (e.g. after meta...: |).
func indent(n int, s string) string {
	if s == "" {
		return ""
	}
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = pad + lines[i]
	}
	return strings.Join(lines, "\n")
}
