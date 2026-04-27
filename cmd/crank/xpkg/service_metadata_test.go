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

package xpkg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestValidateTemplateMetadataValue(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in      any
		want    any
		wantErr error
	}{
		"Nil":                {in: nil, want: nil},
		"String":             {in: "hello", want: "hello"},
		"Bool":               {in: true, want: true},
		"Float":              {in: float64(3.5), want: float64(3.5)},
		"StringSlice":        {in: []any{"a", "b"}, want: []any{"a", "b"}},
		"MixedSlice":         {in: []any{"x", float64(1)}, want: []any{"x", float64(1)}},
		"UnsupportedMap":     {in: map[string]any{"k": "v"}, wantErr: cmpopts.AnyError},
		"UnsupportedInSlice": {in: []any{map[string]any{"k": "v"}}, wantErr: cmpopts.AnyError},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := validateTemplateMetadataValue(tc.in)
			if diff := cmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("validateTemplateMetadataValue(...): -want +got errors:\n%s", diff)
			}
			if err != nil {
				return
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("validateTemplateMetadataValue(...): -want +got:\n%s", diff)
			}
		})
	}
}

func TestBatchCmdLoadServiceMetadata(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		reason      string
		prepare     func(t *testing.T) *batchCmd
		wantErr     error
		wantVars    map[string]map[string]any
		wantVarsNil bool
	}{
		"Success": {
			reason: "valid YAML with two services populates batchCmd.perServiceTemplateVars",
			prepare: func(t *testing.T) *batchCmd {
				t.Helper()
				dir := t.TempDir()
				path := filepath.Join(dir, "meta.yaml")
				content := `elb:
  ServiceDescription: "Elastic Load Balancing manages load balancers."
  ServiceCategories:
    - networking
    - load balancing
ec2:
  Count: 3
`
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
				return &batchCmd{ServiceMetadataFile: path}
			},
			wantVars: map[string]map[string]any{
				"elb": {
					"ServiceDescription": "Elastic Load Balancing manages load balancers.",
					"ServiceCategories":  []any{"networking", "load balancing"},
				},
				"ec2": {
					"Count": float64(3),
				},
			},
		},
		"EmptyPath": {
			reason: "empty ServiceMetadataFile leaves batchCmd.perServiceTemplateVars nil",
			prepare: func(t *testing.T) *batchCmd {
				t.Helper()
				return &batchCmd{}
			},
			wantVarsNil: true,
		},
		"FileNotFound": {
			reason: "missing file errors from batchCmd.loadServiceMetadata and clears prior perServiceTemplateVars",
			prepare: func(t *testing.T) *batchCmd {
				t.Helper()
				return &batchCmd{
					ServiceMetadataFile: filepath.Join(t.TempDir(), "does-not-exist.yaml"),
					perServiceTemplateVars: map[string]map[string]any{
						"prior": {"k": "v"},
					},
				}
			},
			wantErr:     cmpopts.AnyError,
			wantVarsNil: true,
		},
		"MalformedYAML": {
			reason: "malformed YAML errors and clears prior batchCmd.perServiceTemplateVars",
			prepare: func(t *testing.T) *batchCmd {
				t.Helper()
				dir := t.TempDir()
				path := filepath.Join(dir, "bad.yaml")
				if err := os.WriteFile(path, []byte("[ not valid yaml {{{"), 0o600); err != nil {
					t.Fatal(err)
				}
				return &batchCmd{
					ServiceMetadataFile: path,
					perServiceTemplateVars: map[string]map[string]any{
						"prior": {"k": "v"},
					},
				}
			},
			wantErr:     cmpopts.AnyError,
			wantVarsNil: true,
		},
		"UnsupportedNestedMap": {
			reason: "nested map in metadata errors via validateTemplateMetadataValue and clears perServiceTemplateVars",
			prepare: func(t *testing.T) *batchCmd {
				t.Helper()
				dir := t.TempDir()
				path := filepath.Join(dir, "nested-map.yaml")
				content := `ec2:
  Config:
    nested: true
`
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
				return &batchCmd{
					ServiceMetadataFile: path,
					perServiceTemplateVars: map[string]map[string]any{
						"prior": {"k": "v"},
					},
				}
			},
			wantErr:     cmpopts.AnyError,
			wantVarsNil: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cmd := tc.prepare(t)
			err := cmd.loadServiceMetadata()
			if diff := cmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("(*batchCmd).loadServiceMetadata (%s): -want +got errors:\n%s", tc.reason, diff)
			}
			if tc.wantVarsNil {
				if cmd.perServiceTemplateVars != nil {
					t.Fatalf("perServiceTemplateVars (%s): want nil, got %#v", tc.reason, cmd.perServiceTemplateVars)
				}
				return
			}
			if diff := cmp.Diff(tc.wantVars, cmd.perServiceTemplateVars); diff != "" {
				t.Errorf("perServiceTemplateVars (%s): -want +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestBatchCmd_getPackageMetadata_mergeOrder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "crossplane.yaml.tmpl")
	if err := os.WriteFile(tmplPath, []byte(`{{ .Service }}|{{ .Name }}|{{ .ServiceDescription }}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &batchCmd{
		PackageMetadataTemplate: tmplPath,
		ProviderName:            "provider-aws",
		perServiceTemplateVars: map[string]map[string]any{
			"elb": {"ServiceDescription": "from-yaml"},
		},
		TemplateVar: map[string]string{"ServiceDescription": "from-cli"},
	}
	got, err := c.getPackageMetadata("elb")
	if err != nil {
		t.Fatalf("getPackageMetadata: %v", err)
	}
	if want := "elb|provider-aws-elb|from-cli"; got != want {
		t.Fatalf("getPackageMetadata with CLI override: got %q, want %q", got, want)
	}

	c.TemplateVar = nil
	got, err = c.getPackageMetadata("elb")
	if err != nil {
		t.Fatalf("getPackageMetadata: %v", err)
	}
	if want := "elb|provider-aws-elb|from-yaml"; got != want {
		t.Fatalf("getPackageMetadata from YAML only: got %q, want %q", got, want)
	}
}

func TestBatchCmd_getPackageMetadata_ServiceCategoriesList(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "meta.yaml")
	if err := os.WriteFile(metaPath, []byte(`ec2:
  ServiceCategories:
    - compute
    - networking
`), 0o600); err != nil {
		t.Fatal(err)
	}
	tmplPath := filepath.Join(dir, "crossplane.yaml.tmpl")
	tmpl := `categories:
{{ indent 6 (toYAML .ServiceCategories) }}
`
	if err := os.WriteFile(tmplPath, []byte(tmpl), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &batchCmd{
		PackageMetadataTemplate: tmplPath,
		ProviderName:            "provider-aws",
		ServiceMetadataFile:     metaPath,
	}
	if err := c.loadServiceMetadata(); err != nil {
		t.Fatalf("loadServiceMetadata: %v", err)
	}
	got, err := c.getPackageMetadata("ec2")
	if err != nil {
		t.Fatalf("getPackageMetadata: %v", err)
	}
	want := `categories:
      - compute
      - networking
`
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("getPackageMetadata: -want +got:\n%s", diff)
	}
}

// TestBatchCmd_getPackageMetadata_perServiceDistinct exercises loadServiceMetadata
// and getPackageMetadata together: each smaller provider must render its own
// ServiceDescription (same path as xpkg batch uses when building packages).
func TestBatchCmd_getPackageMetadata_perServiceDistinct(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "service-metadata.yaml")
	if err := os.WriteFile(metaPath, []byte(`elb:
  ServiceDescription: "ELB-only text for classic load balancers."
s3:
  ServiceDescription: "S3-only text for buckets and objects."
`), 0o600); err != nil {
		t.Fatal(err)
	}
	tmplPath := filepath.Join(dir, "crossplane.yaml.tmpl")
	tmpl := `apiVersion: meta.pkg.crossplane.io/v1
kind: Provider
metadata:
  name: {{ .Name }}
  annotations:
    meta.crossplane.io/description: |
      {{ .ServiceDescription }}
`
	if err := os.WriteFile(tmplPath, []byte(tmpl), 0o600); err != nil {
		t.Fatal(err)
	}

	c := &batchCmd{
		PackageMetadataTemplate: tmplPath,
		ProviderName:            "provider-aws",
		ServiceMetadataFile:     metaPath,
	}
	if err := c.loadServiceMetadata(); err != nil {
		t.Fatalf("loadServiceMetadata: %v", err)
	}

	elbYAML, err := c.getPackageMetadata("elb")
	if err != nil {
		t.Fatalf("getPackageMetadata(elb): %v", err)
	}
	s3YAML, err := c.getPackageMetadata("s3")
	if err != nil {
		t.Fatalf("getPackageMetadata(s3): %v", err)
	}

	if !strings.Contains(elbYAML, "ELB-only text for classic load balancers.") {
		t.Fatalf("elb metadata missing expected description:\n%s", elbYAML)
	}
	if strings.Contains(elbYAML, "S3-only") {
		t.Fatalf("elb metadata leaked s3 text:\n%s", elbYAML)
	}
	if !strings.Contains(s3YAML, "S3-only text for buckets and objects.") {
		t.Fatalf("s3 metadata missing expected description:\n%s", s3YAML)
	}
	if strings.Contains(s3YAML, "ELB-only") {
		t.Fatalf("s3 metadata leaked elb text:\n%s", s3YAML)
	}
}
