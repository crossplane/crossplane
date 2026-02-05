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

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/cel/openapi/resolver"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// refPrefix is the prefix for local schema references in OpenAPI v3 documents.
const refPrefix = "#/components/schemas/"

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

// NewOpenAPIRequiredSchemasFetcher returns a new OpenAPISchemasFetcher that uses the
// supplied OpenAPIV3Client to fetch schemas.
func NewOpenAPIRequiredSchemasFetcher(c OpenAPIV3Client) *OpenAPISchemasFetcher {
	return &OpenAPISchemasFetcher{client: c}
}

// Fetch fetches the OpenAPI schema for the requested resource kind. It returns
// an empty schema if the kind is not found, rather than an error. References to
// other schemas are flattened inline.
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

	doc := &spec3.OpenAPI{}
	if err := json.Unmarshal(b, doc); err != nil {
		return nil, errors.Wrapf(err, "cannot parse OpenAPI schema for %s", pathKey)
	}

	if doc.Components == nil {
		return &fnv1.Schema{}, nil
	}

	// Find the schema matching our GVK. Each schema has an
	// x-kubernetes-group-version-kind annotation identifying what GVK it
	// represents. Schemas shared across GVKs (e.g. DeleteOptions) are skipped.
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
		if g != gv.Group || v != gv.Version || k != ss.GetKind() {
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

	// Kind not found. Return an empty schema.
	return &fnv1.Schema{}, nil
}
