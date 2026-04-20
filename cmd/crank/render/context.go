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

package render

import (
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// BuildContextData merges --context-files and --context-values into a single
// map. Files are read first; values take precedence when a key appears in
// both.
func BuildContextData(fs afero.Fs, ctxFiles, ctxValues map[string]string) (map[string][]byte, error) {
	out := map[string][]byte{}
	for k, filename := range ctxFiles {
		v, err := afero.ReadFile(fs, filename)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot read context value for key %q", k)
		}
		out[k] = v
	}
	for k, v := range ctxValues {
		out[k] = []byte(v)
	}
	return out, nil
}

// ParseContextData parses each raw context value as YAML/JSON and returns a
// map suitable for seeding the pipeline context.
func ParseContextData(raw map[string][]byte) (map[string]any, error) {
	out := map[string]any{}
	for k, v := range raw {
		var parsed any
		if err := yaml.Unmarshal(v, &parsed); err != nil {
			return nil, errors.Wrapf(err, "cannot unmarshal context value for key %q", k)
		}
		out[k] = parsed
	}
	return out, nil
}
