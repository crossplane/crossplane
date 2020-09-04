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
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	apiextensionsv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	pkgmetav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
)

var xrd = []byte(`apiVersion: apiextensions.crossplane.io/v1alpha1
kind: CompositeResourceDefinition
metadata:
  name: test`)

var comp = []byte(`apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Composition
metadata:
  name: test`)

var crd = []byte(`apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: test`)

var provider = []byte(`apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Provider
metadata:
  name: test`)

var configuration = []byte(`apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Configuration
metadata:
  name: test`)

func TestParser(t *testing.T) {
	uXRD := &unstructured.Unstructured{}
	_ = yaml.Unmarshal(xrd, uXRD)
	uComp := &unstructured.Unstructured{}
	_ = yaml.Unmarshal(comp, uComp)
	uCRD := &unstructured.Unstructured{}
	_ = yaml.Unmarshal(crd, uCRD)
	uConfiguration := &unstructured.Unstructured{}
	_ = yaml.Unmarshal(configuration, uConfiguration)
	uProvider := &unstructured.Unstructured{}
	_ = yaml.Unmarshal(provider, uProvider)
	allBytes := bytes.Join([][]byte{provider, configuration, xrd, comp, crd}, []byte("\n---\n"))
	fs := afero.NewMemMapFs()
	_ = afero.WriteFile(fs, "xrd.yaml", xrd, 0o644)
	_ = afero.WriteFile(fs, "comp.yaml", comp, 0o644)
	_ = afero.WriteFile(fs, "crd.yaml", crd, 0o644)
	_ = afero.WriteFile(fs, "provider.yaml", provider, 0o644)
	_ = afero.WriteFile(fs, "some/nested/dir/configuration.yaml", configuration, 0o644)
	_ = afero.WriteFile(fs, ".crossplane/bad.yaml", configuration, 0o644)
	allFs := afero.NewMemMapFs()
	_ = afero.WriteFile(allFs, "all.yaml", allBytes, 0o644)
	errFs := afero.NewMemMapFs()
	_ = afero.WriteFile(errFs, "bad.yaml", []byte("definitely not yaml"), 0o644)
	emptyFs := afero.NewMemMapFs()
	_ = afero.WriteFile(emptyFs, "empty.yaml", []byte(""), 0o644)
	_ = afero.WriteFile(emptyFs, "bad.yam", []byte("definitely not yaml"), 0o644)
	objScheme := runtime.NewScheme()
	metaScheme := runtime.NewScheme()
	_ = apiextensions.AddToScheme(objScheme)
	_ = apiextensionsv1alpha1.SchemeBuilder.AddToScheme(objScheme)
	_ = pkgmetav1alpha1.SchemeBuilder.AddToScheme(metaScheme)

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
				meta:    []runtime.Object{uProvider, uConfiguration},
				objects: []runtime.Object{uXRD, uComp, uCRD},
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
				meta:    []runtime.Object{uProvider, uConfiguration},
				objects: []runtime.Object{uXRD, uComp, uCRD},
			},
		},
		"FsBackendAll": {
			reason:  "should parse filesystem successfully with multiple objects in single file",
			parser:  New(metaScheme, objScheme),
			backend: NewFsBackend(allFs, FsDir("."), FsFilters(SkipDirs(), SkipNotYAML(), SkipPath(".crossplane/*"))),
			pkg: &Package{
				meta:    []runtime.Object{uProvider, uConfiguration},
				objects: []runtime.Object{uXRD, uComp, uCRD},
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
			backend: NewFsBackend(emptyFs, FsDir("."), FsFilters(SkipDirs(), SkipNotYAML())),
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
				t.Errorf("Provider: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.pkg.GetMeta(), pkg.GetMeta(), cmpopts.SortSlices(func(i, j runtime.Object) bool {
				return i.GetObjectKind().GroupVersionKind().String() > j.GetObjectKind().GroupVersionKind().String()
			})); diff != "" {
				t.Errorf("Provider: -want, +got:\n%s", diff)
			}
		})
	}
}
