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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"k8s.io/client-go/openapi"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

var _ RequiredSchemasFetcher = &OpenAPISchemasFetcher{}

// MockGroupVersion is a mock implementation of openapi.GroupVersion.
type MockGroupVersion struct {
	SchemaFn            func(contentType string) ([]byte, error)
	ServerRelativeURLFn func() string
}

func (m *MockGroupVersion) Schema(contentType string) ([]byte, error) {
	return m.SchemaFn(contentType)
}

func (m *MockGroupVersion) ServerRelativeURL() string {
	if m.ServerRelativeURLFn != nil {
		return m.ServerRelativeURLFn()
	}
	return ""
}

// MockOpenAPIV3Client is a mock implementation of OpenAPIV3Client.
type MockOpenAPIV3Client struct {
	PathsFn func() (map[string]openapi.GroupVersion, error)
}

func (m *MockOpenAPIV3Client) Paths() (map[string]openapi.GroupVersion, error) {
	return m.PathsFn()
}

// openAPIDoc creates a minimal OpenAPI document with the given schemas.
func openAPIDoc(schemas map[string]map[string]any) []byte {
	doc := map[string]any{
		"components": map[string]any{
			"schemas": schemas,
		},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return b
}

func TestOpenAPISchemasFetcherFetch(t *testing.T) {
	errBoom := errors.New("boom")

	// Example schema with x-kubernetes-group-version-kind annotation.
	podSchema := map[string]any{
		"type": "object",
		"x-kubernetes-group-version-kind": []any{
			map[string]any{
				"group":   "",
				"version": "v1",
				"kind":    "Pod",
			},
		},
		"properties": map[string]any{
			"apiVersion": map[string]any{"type": "string"},
			"kind":       map[string]any{"type": "string"},
		},
	}

	deploymentSchema := map[string]any{
		"type": "object",
		"x-kubernetes-group-version-kind": []any{
			map[string]any{
				"group":   "apps",
				"version": "v1",
				"kind":    "Deployment",
			},
		},
		"properties": map[string]any{
			"apiVersion": map[string]any{"type": "string"},
			"kind":       map[string]any{"type": "string"},
		},
	}

	// Schema shared across multiple GVKs (like DeleteOptions).
	multiGVKSchema := map[string]any{
		"type": "object",
		"x-kubernetes-group-version-kind": []any{
			map[string]any{
				"group":   "",
				"version": "v1",
				"kind":    "DeleteOptions",
			},
			map[string]any{
				"group":   "apps",
				"version": "v1",
				"kind":    "DeleteOptions",
			},
		},
	}

	// Schema with a $ref to ObjectMeta.
	schemaWithRef := map[string]any{
		"type": "object",
		"x-kubernetes-group-version-kind": []any{
			map[string]any{
				"group":   "example.com",
				"version": "v1",
				"kind":    "MyResource",
			},
		},
		"properties": map[string]any{
			"metadata": map[string]any{
				"$ref": "#/components/schemas/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
			},
		},
	}

	objectMetaSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":      map[string]any{"type": "string"},
			"namespace": map[string]any{"type": "string"},
		},
	}

	// Schemas with nested refs: MyResource -> ObjectMeta -> OwnerReference.
	schemaWithNestedRef := map[string]any{
		"type": "object",
		"x-kubernetes-group-version-kind": []any{
			map[string]any{
				"group":   "example.com",
				"version": "v1",
				"kind":    "NestedResource",
			},
		},
		"properties": map[string]any{
			"metadata": map[string]any{
				"$ref": "#/components/schemas/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
			},
		},
	}

	objectMetaWithOwnerRef := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
			"ownerReferences": map[string]any{
				"type": "array",
				"items": map[string]any{
					"$ref": "#/components/schemas/io.k8s.apimachinery.pkg.apis.meta.v1.OwnerReference",
				},
			},
		},
	}

	ownerReferenceSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"uid":  map[string]any{"type": "string"},
			"name": map[string]any{"type": "string"},
		},
	}

	type args struct {
		ss *fnv1.SchemaSelector
	}

	type want struct {
		schema *fnv1.Schema
		err    error
	}

	cases := map[string]struct {
		reason string
		client OpenAPIV3Client
		args   args
		want   want
	}{
		"NilSelector": {
			reason: "We should return an error if the selector is nil",
			client: &MockOpenAPIV3Client{},
			args: args{
				ss: nil,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"InvalidApiVersion": {
			reason: "We should return an error if the apiVersion cannot be parsed",
			client: &MockOpenAPIV3Client{},
			args: args{
				ss: &fnv1.SchemaSelector{
					ApiVersion: "not/valid/version",
					Kind:       "Foo",
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"PathsError": {
			reason: "We should return an error if Paths() fails",
			client: &MockOpenAPIV3Client{
				PathsFn: func() (map[string]openapi.GroupVersion, error) {
					return nil, errBoom
				},
			},
			args: args{
				ss: &fnv1.SchemaSelector{
					ApiVersion: "v1",
					Kind:       "Pod",
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"GVNotFound": {
			reason: "We should return an empty schema if the group-version is not found",
			client: &MockOpenAPIV3Client{
				PathsFn: func() (map[string]openapi.GroupVersion, error) {
					return map[string]openapi.GroupVersion{
						"api/v1": &MockGroupVersion{},
					}, nil
				},
			},
			args: args{
				ss: &fnv1.SchemaSelector{
					ApiVersion: "fake.io/v1",
					Kind:       "Foo",
				},
			},
			want: want{
				schema: &fnv1.Schema{},
			},
		},
		"SchemaFetchError": {
			reason: "We should return an error if Schema() fails",
			client: &MockOpenAPIV3Client{
				PathsFn: func() (map[string]openapi.GroupVersion, error) {
					return map[string]openapi.GroupVersion{
						"api/v1": &MockGroupVersion{
							SchemaFn: func(_ string) ([]byte, error) {
								return nil, errBoom
							},
						},
					}, nil
				},
			},
			args: args{
				ss: &fnv1.SchemaSelector{
					ApiVersion: "v1",
					Kind:       "Pod",
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"InvalidJSON": {
			reason: "We should return an error if the OpenAPI document is not valid JSON",
			client: &MockOpenAPIV3Client{
				PathsFn: func() (map[string]openapi.GroupVersion, error) {
					return map[string]openapi.GroupVersion{
						"api/v1": &MockGroupVersion{
							SchemaFn: func(_ string) ([]byte, error) {
								return []byte("not valid json"), nil
							},
						},
					}, nil
				},
			},
			args: args{
				ss: &fnv1.SchemaSelector{
					ApiVersion: "v1",
					Kind:       "Pod",
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"KindNotFound": {
			reason: "We should return an empty schema if the kind is not found",
			client: &MockOpenAPIV3Client{
				PathsFn: func() (map[string]openapi.GroupVersion, error) {
					return map[string]openapi.GroupVersion{
						"api/v1": &MockGroupVersion{
							SchemaFn: func(_ string) ([]byte, error) {
								return openAPIDoc(map[string]map[string]any{
									"io.k8s.api.core.v1.Pod": podSchema,
								}), nil
							},
						},
					}, nil
				},
			},
			args: args{
				ss: &fnv1.SchemaSelector{
					ApiVersion: "v1",
					Kind:       "NotReal",
				},
			},
			want: want{
				schema: &fnv1.Schema{},
			},
		},
		"MultiGVKSchemaSkipped": {
			reason: "We should skip schemas with multiple GVKs (like DeleteOptions)",
			client: &MockOpenAPIV3Client{
				PathsFn: func() (map[string]openapi.GroupVersion, error) {
					return map[string]openapi.GroupVersion{
						"api/v1": &MockGroupVersion{
							SchemaFn: func(_ string) ([]byte, error) {
								return openAPIDoc(map[string]map[string]any{
									"io.k8s.apimachinery.pkg.apis.meta.v1.DeleteOptions": multiGVKSchema,
								}), nil
							},
						},
					}, nil
				},
			},
			args: args{
				ss: &fnv1.SchemaSelector{
					ApiVersion: "v1",
					Kind:       "DeleteOptions",
				},
			},
			want: want{
				schema: &fnv1.Schema{},
			},
		},
		"SuccessCoreGVK": {
			reason: "We should return the schema for a core GVK (v1/Pod)",
			client: &MockOpenAPIV3Client{
				PathsFn: func() (map[string]openapi.GroupVersion, error) {
					return map[string]openapi.GroupVersion{
						"api/v1": &MockGroupVersion{
							SchemaFn: func(_ string) ([]byte, error) {
								return openAPIDoc(map[string]map[string]any{
									"io.k8s.api.core.v1.Pod": podSchema,
								}), nil
							},
						},
					}, nil
				},
			},
			args: args{
				ss: &fnv1.SchemaSelector{
					ApiVersion: "v1",
					Kind:       "Pod",
				},
			},
			want: want{
				schema: &fnv1.Schema{
					OpenapiV3: MustStruct(podSchema),
				},
			},
		},
		"SuccessNonCoreGVK": {
			reason: "We should return the schema for a non-core GVK (apps/v1/Deployment)",
			client: &MockOpenAPIV3Client{
				PathsFn: func() (map[string]openapi.GroupVersion, error) {
					return map[string]openapi.GroupVersion{
						"apis/apps/v1": &MockGroupVersion{
							SchemaFn: func(_ string) ([]byte, error) {
								return openAPIDoc(map[string]map[string]any{
									"io.k8s.api.apps.v1.Deployment": deploymentSchema,
								}), nil
							},
						},
					}, nil
				},
			},
			args: args{
				ss: &fnv1.SchemaSelector{
					ApiVersion: "apps/v1",
					Kind:       "Deployment",
				},
			},
			want: want{
				schema: &fnv1.Schema{
					OpenapiV3: MustStruct(deploymentSchema),
				},
			},
		},
		"CaseInsensitiveGroupMatch": {
			reason: "Group matching should be case-insensitive",
			client: &MockOpenAPIV3Client{
				PathsFn: func() (map[string]openapi.GroupVersion, error) {
					// Schema with uppercase group.
					schemaWithUppercaseGroup := map[string]any{
						"type": "object",
						"x-kubernetes-group-version-kind": []any{
							map[string]any{
								"group":   "APPS",
								"version": "v1",
								"kind":    "Deployment",
							},
						},
					}
					return map[string]openapi.GroupVersion{
						"apis/apps/v1": &MockGroupVersion{
							SchemaFn: func(_ string) ([]byte, error) {
								return openAPIDoc(map[string]map[string]any{
									"io.k8s.api.apps.v1.Deployment": schemaWithUppercaseGroup,
								}), nil
							},
						},
					}, nil
				},
			},
			args: args{
				ss: &fnv1.SchemaSelector{
					ApiVersion: "apps/v1",
					Kind:       "Deployment",
				},
			},
			want: want{
				schema: &fnv1.Schema{
					OpenapiV3: MustStruct(map[string]any{
						"type": "object",
						"x-kubernetes-group-version-kind": []any{
							map[string]any{
								"group":   "APPS",
								"version": "v1",
								"kind":    "Deployment",
							},
						},
					}),
				},
			},
		},
		"FlattensReferencedSchemas": {
			reason: "We should flatten schemas referenced via $ref inline",
			client: &MockOpenAPIV3Client{
				PathsFn: func() (map[string]openapi.GroupVersion, error) {
					return map[string]openapi.GroupVersion{
						"apis/example.com/v1": &MockGroupVersion{
							SchemaFn: func(_ string) ([]byte, error) {
								return openAPIDoc(map[string]map[string]any{
									"example.com.v1.MyResource":                           schemaWithRef,
									"io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta":     objectMetaSchema,
									"io.k8s.apimachinery.pkg.apis.meta.v1.OwnerReference": ownerReferenceSchema,
								}), nil
							},
						},
					}, nil
				},
			},
			args: args{
				ss: &fnv1.SchemaSelector{
					ApiVersion: "example.com/v1",
					Kind:       "MyResource",
				},
			},
			want: want{
				schema: &fnv1.Schema{
					OpenapiV3: MustStruct(map[string]any{
						"type": "object",
						"x-kubernetes-group-version-kind": []any{
							map[string]any{
								"group":   "example.com",
								"version": "v1",
								"kind":    "MyResource",
							},
						},
						"properties": map[string]any{
							"metadata": objectMetaSchema,
						},
					}),
				},
			},
		},
		"FlattensNestedReferencedSchemas": {
			reason: "We should transitively flatten schemas referenced via $ref",
			client: &MockOpenAPIV3Client{
				PathsFn: func() (map[string]openapi.GroupVersion, error) {
					return map[string]openapi.GroupVersion{
						"apis/example.com/v1": &MockGroupVersion{
							SchemaFn: func(_ string) ([]byte, error) {
								return openAPIDoc(map[string]map[string]any{
									"example.com.v1.NestedResource":                       schemaWithNestedRef,
									"io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta":     objectMetaWithOwnerRef,
									"io.k8s.apimachinery.pkg.apis.meta.v1.OwnerReference": ownerReferenceSchema,
								}), nil
							},
						},
					}, nil
				},
			},
			args: args{
				ss: &fnv1.SchemaSelector{
					ApiVersion: "example.com/v1",
					Kind:       "NestedResource",
				},
			},
			want: want{
				schema: &fnv1.Schema{
					OpenapiV3: MustStruct(map[string]any{
						"type": "object",
						"x-kubernetes-group-version-kind": []any{
							map[string]any{
								"group":   "example.com",
								"version": "v1",
								"kind":    "NestedResource",
							},
						},
						"properties": map[string]any{
							"metadata": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"name": map[string]any{"type": "string"},
									"ownerReferences": map[string]any{
										"type":  "array",
										"items": ownerReferenceSchema,
									},
								},
							},
						},
					}),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := NewOpenAPIRequiredSchemasFetcher(tc.client)

			schema, err := f.Fetch(context.Background(), tc.args.ss)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFetch(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.schema, schema, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nFetch(...): -want schema, +got schema:\n%s", tc.reason, diff)
			}
		})
	}
}
