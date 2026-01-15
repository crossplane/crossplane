/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package xfn

import (
	"context"
	"encoding/json"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/openapi"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// A RequiredSchemasFetcher gets OpenAPI schemas for requested resource kinds.
type RequiredSchemasFetcher interface {
	Fetch(ctx context.Context, ss *fnv1.SchemaSelector) (*fnv1.Schema, error)
}

// A RequiredSchemasFetcherFn gets OpenAPI schemas for requested resource kinds.
type RequiredSchemasFetcherFn func(ctx context.Context, ss *fnv1.SchemaSelector) (*fnv1.Schema, error)

// Fetch gets the OpenAPI schema for the requested resource kind.
func (fn RequiredSchemasFetcherFn) Fetch(ctx context.Context, ss *fnv1.SchemaSelector) (*fnv1.Schema, error) {
	return fn(ctx, ss)
}

// NopRequiredSchemasFetcher is a RequiredSchemasFetcher that always returns an
// empty schema. It's useful for callers that don't need schema fetching.
type NopRequiredSchemasFetcher struct{}

// Fetch always returns an empty schema.
func (NopRequiredSchemasFetcher) Fetch(_ context.Context, _ *fnv1.SchemaSelector) (*fnv1.Schema, error) {
	return &fnv1.Schema{}, nil
}

// An OpenAPIV3Client can fetch OpenAPI v3 schemas from the Kubernetes API
// server.
type OpenAPIV3Client interface {
	Paths() (map[string]openapi.GroupVersion, error)
}

// OpenAPISchemasFetcher fetches OpenAPI schemas using the Kubernetes discovery
// API. It implements RequiredSchemasFetcher.
type OpenAPISchemasFetcher struct {
	client OpenAPIV3Client
}

// openAPIDocument is the portion of an OpenAPI v3 document we need to parse.
type openAPIDocument struct {
	Components openAPIComponents `json:"components"`
}

type openAPIComponents struct {
	Schemas map[string]map[string]any `json:"schemas"`
}

// NewOpenAPIRequiredSchemasFetcher returns a new OpenAPISchemasFetcher that uses the
// supplied OpenAPIV3Client to fetch schemas.
func NewOpenAPIRequiredSchemasFetcher(c OpenAPIV3Client) *OpenAPISchemasFetcher {
	return &OpenAPISchemasFetcher{client: c}
}

// Fetch fetches the OpenAPI schema for the requested resource kind. It returns
// an empty schema if the kind is not found, rather than an error.
func (f *OpenAPISchemasFetcher) Fetch(_ context.Context, ss *fnv1.SchemaSelector) (*fnv1.Schema, error) {
	if ss == nil {
		return nil, errors.New("you must specify a schema selector")
	}

	gv, err := schema.ParseGroupVersion(ss.GetApiVersion())
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse apiVersion %q", ss.GetApiVersion())
	}

	// Map apiVersion to OpenAPI path. Core types use "api/v1", others use
	// "apis/group/version".
	pathKey := "apis/" + ss.GetApiVersion()
	if gv.Group == "" {
		pathKey = "api/" + gv.Version
	}

	paths, err := f.client.Paths()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get OpenAPI paths")
	}

	client, ok := paths[pathKey]
	if !ok {
		// Group-version not found. Return an empty schema rather than an error,
		// consistent with how required resources returns nil for not found.
		return &fnv1.Schema{}, nil
	}

	b, err := client.Schema("application/json")
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get OpenAPI schema for %s", pathKey)
	}

	doc := &openAPIDocument{}
	if err := json.Unmarshal(b, doc); err != nil {
		return nil, errors.Wrapf(err, "cannot parse OpenAPI schema for %s", pathKey)
	}

	// Find the schema matching our GVK. Each schema has an
	// x-kubernetes-group-version-kind annotation identifying what GVK it
	// represents. Schemas shared across GVKs (e.g. DeleteOptions) are skipped.
	for _, s := range doc.Components.Schemas {
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
		if !strings.EqualFold(g, gv.Group) {
			continue
		}
		if !strings.EqualFold(v, gv.Version) {
			continue
		}
		if k != ss.GetKind() {
			continue
		}
		st, err := structpb.NewStruct(s)
		if err != nil {
			return nil, errors.Wrap(err, "cannot convert schema to protobuf Struct")
		}
		return &fnv1.Schema{OpenapiV3: st}, nil
	}

	// Kind not found. Return an empty schema.
	return &fnv1.Schema{}, nil
}
