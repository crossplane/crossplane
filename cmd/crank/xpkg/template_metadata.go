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
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

const (
	errTemplateMetadata = "failed to load template metadata file; check the file path and YAML syntax"
	errNestedMapping    = "template metadata value is a nested mapping or object; allowed YAML value types are string, number, boolean, null, and lists of those. " +
		"Flatten nested data or serialize it (for example to a string) before adding it to metadata YAML."
	errUnsupportedValue = "template metadata value has an unsupported form; allowed YAML value types are string, number, boolean, null, and lists of those. " +
		"Convert or serialize this value to one of those forms before including it in metadata YAML."
)

// loadTemplateMetadata reads TemplateMetadataFile into perServiceTemplateVars.
// Each top-level key is a smaller provider name; the value is a map of template
// variable names to values suitable for text/template (scalars and []any lists;
// lists are preserved so templates can use {{ range .ServiceCategories }}).
func (c *batchCmd) loadTemplateMetadata() error {
	c.perServiceTemplateVars = nil
	if c.TemplateMetadataFile == "" {
		return nil
	}
	b, err := os.ReadFile(filepath.Clean(c.TemplateMetadataFile))
	if err != nil {
		return errors.Wrap(err, errTemplateMetadata)
	}
	var raw map[string]map[string]any
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return errors.Wrap(err, errTemplateMetadata)
	}
	out := make(map[string]map[string]any, len(raw))
	for svc, vars := range raw {
		if vars == nil {
			continue
		}
		sm, err := validateTemplateMetadataVars(vars)
		if err != nil {
			return errors.Wrapf(err, "%s: service %q", errTemplateMetadata, svc)
		}
		out[svc] = sm
	}
	c.perServiceTemplateVars = out
	return nil
}

func validateTemplateMetadataVars(vars map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(vars))
	for k, v := range vars {
		nv, err := validateTemplateMetadataValue(v)
		if err != nil {
			return nil, errors.Wrapf(err, "variable %q", k)
		}
		out[k] = nv
	}
	return out, nil
}

func validateTemplateMetadataValue(v any) (any, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case string:
		return t, nil
	case bool:
		return t, nil
	case int:
		return t, nil
	case int32:
		return int(t), nil
	case int64:
		return t, nil
	case uint:
		return t, nil
	case uint64:
		return t, nil
	case float64:
		return t, nil
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			ne, err := validateTemplateMetadataValue(e)
			if err != nil {
				return nil, err
			}
			if _, ok := ne.([]any); ok {
				return nil, errors.New("nested lists in template metadata are not supported")
			}
			out[i] = ne
		}
		return out, nil
	case map[string]any:
		return nil, errors.New(errNestedMapping)
	default:
		return nil, errors.New(errUnsupportedValue)
	}
}
