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

package render

import (
	"context"
	"encoding/json"
	"maps"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// refPrefix is the prefix for local schema references in OpenAPI v3 documents.
const refPrefix = "#/components/schemas/"

// OpenAPIV3Schema is a parsed OpenAPI v3 document containing component schemas.
// It represents the schema format returned by /openapi/v3/<group-version>.
type OpenAPIV3Schema struct {
	Components OpenAPIV3Components `json:"components"`
}

// OpenAPIV3Components holds the components section of an OpenAPI v3 document.
type OpenAPIV3Components struct {
	Schemas map[string]map[string]any `json:"schemas"`
}

// LoadRequiredSchemas loads OpenAPI v3 schema documents from a file or
// directory. Each file should contain a single OpenAPI v3 document in JSON
// format (as returned by /openapi/v3/<group-version>).
func LoadRequiredSchemas(fs afero.Fs, fileOrDir string) ([]OpenAPIV3Schema, error) {
	var files []string

	info, err := fs.Stat(fileOrDir)
	if err != nil {
		return nil, errors.Wrap(err, "cannot stat file")
	}

	if info.IsDir() {
		entries, err := afero.ReadDir(fs, fileOrDir)
		if err != nil {
			return nil, errors.Wrap(err, "cannot read directory")
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			// Accept .json files only for OpenAPI docs.
			if filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			files = append(files, filepath.Join(fileOrDir, entry.Name()))
		}
		if len(files) == 0 {
			return nil, errors.Errorf("no JSON files found in %q", fileOrDir)
		}
	} else {
		files = append(files, fileOrDir)
	}

	schemas := make([]OpenAPIV3Schema, 0, len(files))
	for _, file := range files {
		data, err := afero.ReadFile(fs, file)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot read file %q", file)
		}

		s := OpenAPIV3Schema{}
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, errors.Wrapf(err, "cannot parse OpenAPI JSON from %q", file)
		}

		schemas = append(schemas, s)
	}

	return schemas, nil
}

// FilteringSchemaFetcher is a RequiredSchemasFetcher that returns schemas from
// pre-loaded OpenAPI v3 documents. It's intended for use in offline rendering.
type FilteringSchemaFetcher struct {
	docs []OpenAPIV3Schema
}

// NewFilteringSchemaFetcher creates a FilteringSchemaFetcher from the supplied
// OpenAPI v3 documents.
func NewFilteringSchemaFetcher(docs []OpenAPIV3Schema) *FilteringSchemaFetcher {
	return &FilteringSchemaFetcher{docs: docs}
}

// Fetch returns the schema for the requested GVK, or an empty schema if not
// found. This matches the behavior of Crossplane's OpenAPISchemasFetcher.
func (f *FilteringSchemaFetcher) Fetch(_ context.Context, ss *fnv1.SchemaSelector) (*fnv1.Schema, error) {
	if ss == nil {
		return &fnv1.Schema{}, nil
	}

	gv, err := schema.ParseGroupVersion(ss.GetApiVersion())
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse apiVersion %q", ss.GetApiVersion())
	}

	// Search through all documents for the requested GVK.
	for _, doc := range f.docs {
		for name, s := range doc.Components.Schemas {
			gvks, ok := s["x-kubernetes-group-version-kind"].([]any)
			if !ok || len(gvks) != 1 {
				continue
			}
			gvk, ok := gvks[0].(map[string]any)
			if !ok {
				continue
			}
			g, _ := gvk["group"].(string)
			v, _ := gvk["version"].(string)
			k, _ := gvk["kind"].(string)

			if !strings.EqualFold(g, gv.Group) || !strings.EqualFold(v, gv.Version) || k != ss.GetKind() {
				continue
			}

			// Found a match. Make a copy so we don't modify the original.
			sCopy := make(map[string]any, len(s))
			maps.Copy(sCopy, s)

			// Collect all schemas referenced by $ref and include them in the
			// result so functions can resolve them.
			collected := make(map[string]map[string]any)
			visited := make(map[string]bool)
			collectRefs(sCopy, doc.Components.Schemas, collected, visited)
			// Remove self-reference.
			delete(collected, name)

			if len(collected) > 0 {
				schemas := make(map[string]any, len(collected))
				for k, v := range collected {
					schemas[k] = v
				}
				sCopy["components"] = map[string]any{"schemas": schemas}
			}

			st, err := structpb.NewStruct(sCopy)
			if err != nil {
				return nil, errors.Wrap(err, "cannot convert schema to protobuf Struct")
			}

			return &fnv1.Schema{OpenapiV3: st}, nil
		}
	}

	// Not found, return empty schema (matches Crossplane behavior).
	return &fnv1.Schema{}, nil
}

// collectRefs recursively walks node looking for $ref fields and adds the
// referenced schemas to collected. The visited map prevents infinite recursion.
func collectRefs(node any, definitions, collected map[string]map[string]any, visited map[string]bool) {
	switch val := node.(type) {
	case map[string]any:
		if ref, ok := val["$ref"].(string); ok {
			name, found := strings.CutPrefix(ref, refPrefix)
			if !found || visited[name] {
				return
			}
			visited[name] = true
			schema, ok := definitions[name]
			if !ok {
				return
			}
			collected[name] = schema
			collectRefs(schema, definitions, collected, visited)
			return
		}
		for _, child := range val {
			collectRefs(child, definitions, collected, visited)
		}
	case []any:
		for _, item := range val {
			collectRefs(item, definitions, collected, visited)
		}
	}
}
