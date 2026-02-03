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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

func mustStruct(v map[string]any) *structpb.Struct {
	s, err := structpb.NewStruct(v)
	if err != nil {
		panic(err)
	}
	return s
}

func TestFilteringSchemaFetcher(t *testing.T) {
	objectMetaSchema := map[string]any{
		"type":        "object",
		"description": "ObjectMeta is metadata that all persisted resources must have.",
		"properties": map[string]any{
			"name":      map[string]any{"type": "string"},
			"namespace": map[string]any{"type": "string"},
		},
	}

	deploymentSchema := map[string]any{
		"type":        "object",
		"description": "Deployment enables declarative updates for Pods.",
		"x-kubernetes-group-version-kind": []any{
			map[string]any{"group": "apps", "kind": "Deployment", "version": "v1"},
		},
		"properties": map[string]any{
			"metadata": map[string]any{
				"$ref": "#/components/schemas/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
			},
			"spec": map[string]any{"type": "object"},
		},
	}

	configMapSchema := map[string]any{
		"type":        "object",
		"description": "ConfigMap holds configuration data.",
		"x-kubernetes-group-version-kind": []any{
			map[string]any{"group": "", "kind": "ConfigMap", "version": "v1"},
		},
	}

	deploymentDoc := OpenAPIV3Schema{
		Components: OpenAPIV3Components{
			Schemas: map[string]map[string]any{
				"io.k8s.api.apps.v1.Deployment":                   deploymentSchema,
				"io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta": objectMetaSchema,
			},
		},
	}

	coreDoc := OpenAPIV3Schema{
		Components: OpenAPIV3Components{
			Schemas: map[string]map[string]any{
				"io.k8s.api.core.v1.ConfigMap": configMapSchema,
			},
		},
	}

	type args struct {
		docs []OpenAPIV3Schema
		ss   *fnv1.SchemaSelector
	}

	type want struct {
		schema *fnv1.Schema
		err    error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"DeploymentFoundWithRefs": {
			reason: "Should return the Deployment schema with referenced ObjectMeta in components",
			args: args{
				docs: []OpenAPIV3Schema{deploymentDoc},
				ss:   &fnv1.SchemaSelector{ApiVersion: "apps/v1", Kind: "Deployment"},
			},
			want: want{
				schema: &fnv1.Schema{
					OpenapiV3: mustStruct(map[string]any{
						"type":        "object",
						"description": "Deployment enables declarative updates for Pods.",
						"x-kubernetes-group-version-kind": []any{
							map[string]any{"group": "apps", "kind": "Deployment", "version": "v1"},
						},
						"properties": map[string]any{
							"metadata": map[string]any{
								"$ref": "#/components/schemas/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
							},
							"spec": map[string]any{"type": "object"},
						},
						"components": map[string]any{
							"schemas": map[string]any{
								"io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta": objectMetaSchema,
							},
						},
					}),
				},
			},
		},
		"ConfigMapFoundCoreType": {
			reason: "Should return the ConfigMap schema for core type with empty group",
			args: args{
				docs: []OpenAPIV3Schema{coreDoc},
				ss:   &fnv1.SchemaSelector{ApiVersion: "v1", Kind: "ConfigMap"},
			},
			want: want{
				schema: &fnv1.Schema{
					OpenapiV3: mustStruct(configMapSchema),
				},
			},
		},
		"NotFound": {
			reason: "Should return empty schema when GVK is not in documents",
			args: args{
				docs: []OpenAPIV3Schema{deploymentDoc},
				ss:   &fnv1.SchemaSelector{ApiVersion: "v1", Kind: "Pod"},
			},
			want: want{
				schema: &fnv1.Schema{},
			},
		},
		"NilSelector": {
			reason: "Should return empty schema for nil selector",
			args: args{
				docs: []OpenAPIV3Schema{deploymentDoc},
				ss:   nil,
			},
			want: want{
				schema: &fnv1.Schema{},
			},
		},
		"MultipleDocuments": {
			reason: "Should find schemas across multiple documents",
			args: args{
				docs: []OpenAPIV3Schema{deploymentDoc, coreDoc},
				ss:   &fnv1.SchemaSelector{ApiVersion: "v1", Kind: "ConfigMap"},
			},
			want: want{
				schema: &fnv1.Schema{
					OpenapiV3: mustStruct(configMapSchema),
				},
			},
		},
		"EmptyDocuments": {
			reason: "Should return empty schema when no documents provided",
			args: args{
				docs: []OpenAPIV3Schema{},
				ss:   &fnv1.SchemaSelector{ApiVersion: "apps/v1", Kind: "Deployment"},
			},
			want: want{
				schema: &fnv1.Schema{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := NewFilteringSchemaFetcher(tc.args.docs)

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

func TestLoadRequiredSchemas(t *testing.T) {
	deploymentJSON := `{
		"components": {
			"schemas": {
				"io.k8s.api.apps.v1.Deployment": {
					"type": "object",
					"x-kubernetes-group-version-kind": [{"group": "apps", "kind": "Deployment", "version": "v1"}]
				}
			}
		}
	}`

	deploymentSchema := OpenAPIV3Schema{
		Components: OpenAPIV3Components{
			Schemas: map[string]map[string]any{
				"io.k8s.api.apps.v1.Deployment": {
					"type": "object",
					"x-kubernetes-group-version-kind": []any{
						map[string]any{"group": "apps", "kind": "Deployment", "version": "v1"},
					},
				},
			},
		},
	}

	type args struct {
		fs   afero.Fs
		file string
	}
	type want struct {
		schemas []OpenAPIV3Schema
		err     error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SingleFile": {
			reason: "Should load a single JSON file",
			args: args{
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = afero.WriteFile(fs, "/schemas/apps-v1.json", []byte(deploymentJSON), 0o644)
					return fs
				}(),
				file: "/schemas/apps-v1.json",
			},
			want: want{
				schemas: []OpenAPIV3Schema{deploymentSchema},
			},
		},
		"Directory": {
			reason: "Should load all JSON files from a directory",
			args: args{
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.MkdirAll("/schemas", 0o755)
					_ = afero.WriteFile(fs, "/schemas/apps-v1.json", []byte(deploymentJSON), 0o644)
					_ = afero.WriteFile(fs, "/schemas/core-v1.json", []byte(deploymentJSON), 0o644)
					_ = afero.WriteFile(fs, "/schemas/readme.txt", []byte("ignore me"), 0o644)
					return fs
				}(),
				file: "/schemas",
			},
			want: want{
				schemas: []OpenAPIV3Schema{deploymentSchema, deploymentSchema},
			},
		},
		"FileNotFound": {
			reason: "Should return error for non-existent file",
			args: args{
				fs:   afero.NewMemMapFs(),
				file: "/does-not-exist.json",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"InvalidJSON": {
			reason: "Should return error for invalid JSON",
			args: args{
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = afero.WriteFile(fs, "/bad.json", []byte("not valid json"), 0o644)
					return fs
				}(),
				file: "/bad.json",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"EmptyDirectory": {
			reason: "Should return error for empty directory",
			args: args{
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.MkdirAll("/empty", 0o755)
					return fs
				}(),
				file: "/empty",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			schemas, err := LoadRequiredSchemas(tc.args.fs, tc.args.file)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nLoadRequiredSchemas(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.schemas, schemas, cmpopts.SortSlices(func(a, b OpenAPIV3Schema) bool {
				// Sort by first schema name for deterministic comparison.
				for k := range a.Components.Schemas {
					for k2 := range b.Components.Schemas {
						return k < k2
					}
				}
				return false
			})); diff != "" {
				t.Errorf("\n%s\nLoadRequiredSchemas(...): -want schemas, +got schemas:\n%s", tc.reason, diff)
			}
		})
	}
}
