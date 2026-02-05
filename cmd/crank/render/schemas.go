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
	iofs "io/fs"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/cel/openapi/resolver"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// refPrefix is the prefix for local schema references in OpenAPI v3 documents.
const refPrefix = "#/components/schemas/"

// LoadRequiredSchemas loads OpenAPI v3 schema documents from a directory,
// recursively. Each file should contain a single OpenAPI v3 document in JSON
// format (as returned by /openapi/v3/<group-version>).
func LoadRequiredSchemas(fs afero.Fs, dir string) ([]spec3.OpenAPI, error) {
	var files []string

	err := iofs.WalkDir(afero.NewIOFS(fs), dir, func(path string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".json" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "cannot walk directory")
	}

	if len(files) == 0 {
		return nil, errors.Errorf("no JSON files found in %q", dir)
	}

	schemas := make([]spec3.OpenAPI, 0, len(files))
	for _, file := range files {
		data, err := afero.ReadFile(fs, file)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot read file %q", file)
		}

		s := spec3.OpenAPI{}
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
	docs []spec3.OpenAPI
}

// NewFilteringSchemaFetcher creates a FilteringSchemaFetcher from the supplied
// OpenAPI v3 documents.
func NewFilteringSchemaFetcher(docs []spec3.OpenAPI) *FilteringSchemaFetcher {
	return &FilteringSchemaFetcher{docs: docs}
}

// Fetch returns the schema for the requested GVK, or an empty schema if not
// found. References are flattened inline. This matches the behavior of
// Crossplane's OpenAPISchemasFetcher.
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
		if doc.Components == nil {
			continue
		}

		for name, s := range doc.Components.Schemas {
			gvks, ok := s.Extensions["x-kubernetes-group-version-kind"].([]any)
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

			// Flatten all $ref references inline.
			schemaOf := func(ref string) (*spec.Schema, bool) {
				n := strings.TrimPrefix(ref, refPrefix)
				sch, ok := doc.Components.Schemas[n]
				return sch, ok
			}
			flattened, err := resolver.PopulateRefs(schemaOf, refPrefix+name)
			if err != nil {
				return nil, errors.Wrap(err, "cannot flatten schema references")
			}

			// Convert to protobuf Struct via JSON.
			jsonBytes, err := json.Marshal(flattened)
			if err != nil {
				return nil, errors.Wrap(err, "cannot marshal flattened schema")
			}
			st := &structpb.Struct{}
			if err := protojson.Unmarshal(jsonBytes, st); err != nil {
				return nil, errors.Wrap(err, "cannot convert schema to protobuf Struct")
			}
			return &fnv1.Schema{OpenapiV3: st}, nil
		}
	}

	// Not found, return empty schema (matches Crossplane behavior).
	return &fnv1.Schema{}, nil
}
