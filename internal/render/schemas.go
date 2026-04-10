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
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/spec3"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// InMemoryOpenAPIClient serves OpenAPI v3 documents from memory. It implements
// the xfn.OpenAPIV3Client interface used by xfn.OpenAPISchemasFetcher.
type InMemoryOpenAPIClient struct {
	paths map[string]openapi.GroupVersion
}

// NewInMemoryOpenAPIClient returns an InMemoryOpenAPIClient that serves the
// supplied OpenAPI v3 documents. Each document is provided as a protobuf
// Struct. The client discovers which API group-versions each document covers
// by inspecting x-kubernetes-group-version-kind annotations on the schemas.
func NewInMemoryOpenAPIClient(docs []*structpb.Struct) (*InMemoryOpenAPIClient, error) {
	paths := make(map[string]openapi.GroupVersion)

	for _, doc := range docs {
		b, err := protojson.Marshal(doc)
		if err != nil {
			return nil, errors.Wrap(err, "cannot marshal OpenAPI document to JSON")
		}

		// Parse to extract group-versions from the schemas' GVK annotations.
		parsed := &spec3.OpenAPI{}
		if err := json.Unmarshal(b, parsed); err != nil {
			return nil, errors.Wrap(err, "cannot parse OpenAPI document")
		}

		if parsed.Components == nil {
			continue
		}

		// Discover which API group-versions this document covers by
		// inspecting x-kubernetes-group-version-kind on each schema.
		for _, s := range parsed.Components.Schemas {
			gvks, ok := s.Extensions["x-kubernetes-group-version-kind"].([]any)
			if !ok {
				continue
			}
			for _, raw := range gvks {
				gvk, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				g, _ := gvk["group"].(string)
				v, _ := gvk["version"].(string)

				var pathKey string
				if g == "" {
					pathKey = "api/" + v
				} else {
					pathKey = "apis/" + g + "/" + v
				}

				// Store the document bytes under this path key. If
				// multiple schemas in the same doc share a group-version,
				// this is a no-op (same doc, same path).
				if _, ok := paths[pathKey]; !ok {
					paths[pathKey] = &inMemoryGroupVersion{data: b}
				}
			}
		}
	}

	return &InMemoryOpenAPIClient{paths: paths}, nil
}

// Paths returns the available API group-versions.
func (c *InMemoryOpenAPIClient) Paths() (map[string]openapi.GroupVersion, error) {
	if c.paths == nil {
		return map[string]openapi.GroupVersion{}, nil
	}
	return c.paths, nil
}

// inMemoryGroupVersion serves a single OpenAPI v3 document from memory.
type inMemoryGroupVersion struct {
	data []byte
}

// Schema returns the stored document bytes.
func (g *inMemoryGroupVersion) Schema(_ string) ([]byte, error) {
	return g.data, nil
}

// ServerRelativeURL returns an empty string. Not used during render.
func (g *inMemoryGroupVersion) ServerRelativeURL() string {
	return ""
}
