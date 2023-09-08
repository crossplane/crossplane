/*
Copyright 2020 The Crossplane Authors.

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

package parser

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"
	appsv1 "k8s.io/api/apps/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

var _ Parser = &PackageParser{}

var (
	crdBytes = []byte(`apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: test`)

	whitespaceBytes = []byte(`---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: test
---

---

---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: test`)

	deployBytes = []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test`)

	commentedOutBytes = []byte(`# apiVersion: apps/v1
# kind: Deployment
# metadata:
#   name: test`)
	manifestWithComments = []byte(`
apiVersion: apiextensions.k8s.io/v1beta1
# Some Comment
kind: CustomResourceDefinition
metadata:
  name: test`)

	crd    = &apiextensions.CustomResourceDefinition{}
	_      = yaml.Unmarshal(crdBytes, crd)
	deploy = &appsv1.Deployment{}
	_      = yaml.Unmarshal(deployBytes, deploy)
)

func TestParser(t *testing.T) {
	allBytes := bytes.Join([][]byte{crdBytes, deployBytes}, []byte("\n---\n"))
	fs := afero.NewMemMapFs()
	_ = afero.WriteFile(fs, "crd.yaml", crdBytes, 0o644)
	_ = afero.WriteFile(fs, "whitespace.yaml", whitespaceBytes, 0o644)
	_ = afero.WriteFile(fs, "deployment.yaml", deployBytes, 0o644)
	_ = afero.WriteFile(fs, "some/nested/dir/crd.yaml", crdBytes, 0o644)
	_ = afero.WriteFile(fs, ".crossplane/bad.yaml", crdBytes, 0o644)
	allFs := afero.NewMemMapFs()
	_ = afero.WriteFile(allFs, "all.yaml", allBytes, 0o644)
	errFs := afero.NewMemMapFs()
	_ = afero.WriteFile(errFs, "bad.yaml", []byte("definitely not yaml"), 0o644)
	emptyFs := afero.NewMemMapFs()
	_ = afero.WriteFile(emptyFs, "empty.yaml", []byte(""), 0o644)
	_ = afero.WriteFile(emptyFs, "bad.yam", []byte("definitely not yaml"), 0o644)
	commentedFs := afero.NewMemMapFs()
	_ = afero.WriteFile(commentedFs, "commented.yaml", commentedOutBytes, 0o644)
	_ = afero.WriteFile(commentedFs, ".crossplane/realmanifest.yaml", manifestWithComments, 0o644)
	objScheme := runtime.NewScheme()
	_ = apiextensions.AddToScheme(objScheme)
	metaScheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(metaScheme)

	cases := map[string]struct {
		reason  string
		parser  Parser
		backend Backend
		pkg     *Package
		wantErr bool
	}{
		"EchoBackendEmpty": {
			reason:  "should have empty output with empty input",
			parser:  New(metaScheme, objScheme),
			backend: NewEchoBackend(""),
			pkg:     NewPackage(),
		},
		"EchoBackendError": {
			reason:  "should have error with invalid yaml",
			parser:  New(metaScheme, objScheme),
			backend: NewEchoBackend("definitely not yaml"),
			pkg:     NewPackage(),
			wantErr: true,
		},
		"EchoBackend": {
			reason:  "should parse input stream successfully",
			parser:  New(metaScheme, objScheme),
			backend: NewEchoBackend(string(allBytes)),
			pkg: &Package{
				meta:    []runtime.Object{deploy},
				objects: []runtime.Object{crd},
			},
		},
		"NopBackend": {
			reason:  "should never parse any objects and never return an error",
			parser:  New(metaScheme, objScheme),
			backend: NewNopBackend(),
			pkg:     NewPackage(),
		},
		"FsBackend": {
			reason:  "should parse filesystem successfully",
			parser:  New(metaScheme, objScheme),
			backend: NewFsBackend(fs, FsDir("."), FsFilters(SkipDirs(), SkipNotYAML(), SkipPath(".crossplane/*"))),
			pkg: &Package{
				meta:    []runtime.Object{deploy},
				objects: []runtime.Object{crd, crd, crd, crd},
			},
		},
		"FsBackendCommentedOut": {
			reason:  "should parse filesystem successfully even if all the files are commented out",
			parser:  New(metaScheme, objScheme),
			backend: NewFsBackend(commentedFs, FsDir("."), FsFilters(SkipDirs(), SkipNotYAML(), SkipPath(".crossplane/*"))),
			pkg: &Package{
				meta:    nil,
				objects: nil,
			},
		},
		"FsBackendWithComments": {
			reason:  "should parse filesystem successfully when some of the manifests contain comments",
			parser:  New(metaScheme, objScheme),
			backend: NewFsBackend(commentedFs, FsDir("."), FsFilters(SkipDirs(), SkipNotYAML())),
			pkg: &Package{
				meta:    nil,
				objects: []runtime.Object{crd},
			},
		},
		"FsBackendAll": {
			reason:  "should parse filesystem successfully with multiple objects in single file",
			parser:  New(metaScheme, objScheme),
			backend: NewFsBackend(allFs, FsDir("."), FsFilters(SkipDirs(), SkipNotYAML(), SkipPath(".crossplane/*"))),
			pkg: &Package{
				meta:    []runtime.Object{deploy},
				objects: []runtime.Object{crd},
			},
		},
		"FsBackendError": {
			reason:  "should error if yaml file with invalid yaml",
			parser:  New(metaScheme, objScheme),
			backend: NewFsBackend(fs, FsDir(".")),
			pkg:     NewPackage(),
			wantErr: true,
		},
		"FsBackendSkip": {
			reason:  "should skip empty files and files without yaml extension",
			parser:  New(metaScheme, objScheme),
			backend: NewFsBackend(emptyFs, FsDir("."), FsFilters(SkipDirs(), SkipEmpty(), SkipNotYAML())),
			pkg:     NewPackage(),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r, err := tc.backend.Init(context.TODO())
			if err != nil {
				t.Errorf("backend.Init(...): unexpected error: %s", err)
			}
			pkg, err := tc.parser.Parse(context.TODO(), r)
			if err != nil && !tc.wantErr {
				t.Errorf("parser.Parse(...): unexpected error: %s", err)
			}
			if tc.wantErr {
				return
			}
			if diff := cmp.Diff(tc.pkg.GetObjects(), pkg.GetObjects(), cmpopts.SortSlices(func(i, j runtime.Object) bool {
				return i.GetObjectKind().GroupVersionKind().String() > j.GetObjectKind().GroupVersionKind().String()
			})); diff != "" {
				t.Errorf("Objects: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.pkg.GetMeta(), pkg.GetMeta(), cmpopts.SortSlices(func(i, j runtime.Object) bool {
				return i.GetObjectKind().GroupVersionKind().String() > j.GetObjectKind().GroupVersionKind().String()
			})); diff != "" {
				t.Errorf("Meta: -want, +got:\n%s", diff)
			}
		})
	}
}
