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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"
)

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

	type args struct {
		fs  afero.Fs
		dir string
	}
	type want struct {
		count int
		err   error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
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
				dir: "/schemas",
			},
			want: want{
				count: 2,
			},
		},
		"NestedDirectory": {
			reason: "Should load JSON files from nested directories",
			args: args{
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.MkdirAll("/schemas/apps", 0o755)
					_ = fs.MkdirAll("/schemas/core", 0o755)
					_ = afero.WriteFile(fs, "/schemas/apps/v1.json", []byte(deploymentJSON), 0o644)
					_ = afero.WriteFile(fs, "/schemas/core/v1.json", []byte(deploymentJSON), 0o644)
					return fs
				}(),
				dir: "/schemas",
			},
			want: want{
				count: 2,
			},
		},
		"NotFound": {
			reason: "Should return error for non-existent directory",
			args: args{
				fs:  afero.NewMemMapFs(),
				dir: "/does-not-exist",
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
					_ = fs.MkdirAll("/schemas", 0o755)
					_ = afero.WriteFile(fs, "/schemas/bad.json", []byte("not valid json"), 0o644)
					return fs
				}(),
				dir: "/schemas",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"EmptyDirectory": {
			reason: "Should return error for directory with no JSON files",
			args: args{
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.MkdirAll("/empty", 0o755)
					return fs
				}(),
				dir: "/empty",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			schemas, err := LoadRequiredSchemas(tc.args.fs, tc.args.dir)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nLoadRequiredSchemas(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.count, len(schemas)); diff != "" {
				t.Errorf("\n%s\nLoadRequiredSchemas(...): -want count, +got count:\n%s", tc.reason, diff)
			}
		})
	}
}
