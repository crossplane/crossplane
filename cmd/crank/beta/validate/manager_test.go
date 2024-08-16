/*
Copyright 2024 The Crossplane Authors.

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

package validate

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	conregv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	// config-pkg:v1.3.0.
	configPkg = []byte(`apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Configuration
metadata:
  name: config-pkg
spec:
  dependsOn:
    - provider: provider-dep-1
      version: "v1.3.0"
---

`)

	// provider-dep-1:v1.3.0.
	providerYaml = []byte(`apiVersion: meta.pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-dep-1
---

`)

	// function-dep-1:v1.3.0.
	funcYaml = []byte(`apiVersion: meta.pkg.crossplane.io/v1beta1
kind: Function
metadata:
  name: function-dep-1
---

`)

	// config-dep-1:v1.3.0.
	configDep1Yaml = []byte(`apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Configuration
metadata:
  name: config-dep-1
spec:
  dependsOn:
    - configuration: config-dep-2
      version: "v1.3.0"
---

`)

	// config-dep-2:v1.3.0.
	configDep2Yaml = []byte(`apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Configuration
metadata:
  name: config-dep-2
spec:
  dependsOn:
    - provider: provider-dep-1
      version: "v1.3.0"
    - function: function-dep-1
      version: "v1.3.0"
---

`)
)

func TestConfigurationTypeSupport(t *testing.T) {
	confpkg := static.NewLayer(configPkg, types.OCILayer)
	pd := static.NewLayer(providerYaml, types.OCILayer)
	fd := static.NewLayer(funcYaml, types.OCILayer)

	fetchMockFunc := func(image string) (*conregv1.Layer, error) {
		switch image {
		case "config-pkg:v1.3.0":
			return &confpkg, nil
		case "provider-dep-1:v1.3.0":
			return &pd, nil
		case "function-dep-1:v1.3.0":
			return &fd, nil
		default:
			return nil, fmt.Errorf("unknown image: %s", image)
		}
	}

	type args struct {
		extensions []*unstructured.Unstructured
		fetchMock  func(image string) (*conregv1.Layer, error)
	}
	type want struct {
		err   error
		confs int
		deps  int
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulConfigPkg": {
			// config-pkg
			// └─►provider-dep-1
			reason: "All dependencies should be successfully added from Configuration.pkg",
			args: args{
				extensions: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "pkg.crossplane.io/v1alpha1",
							"kind":       "Configuration",
							"metadata": map[string]interface{}{
								"name": "config-pkg",
							},
							"spec": map[string]interface{}{
								"package": "config-pkg:v1.3.0",
							},
						},
					},
				},
				fetchMock: fetchMockFunc,
			},
			want: want{
				err:   nil,
				confs: 1, // Configuration.pkg from remote
				deps:  2, // 1 provider, 1 Configuration.pkg dependency
			},
		},
		"SuccessfulConfigMeta": {
			// config-meta
			// └─►function-dep-1
			reason: "All dependencies should be successfully added from Configuration.meta",
			args: args{
				extensions: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "meta.pkg.crossplane.io/v1alpha1",
							"kind":       "Configuration",
							"metadata": map[string]interface{}{
								"name": "config-meta",
							},
							"spec": map[string]interface{}{
								"dependsOn": []map[string]interface{}{
									{
										"function": "function-dep-1",
										"version":  "v1.3.0",
									},
								},
							},
						},
					},
				},
				fetchMock: fetchMockFunc,
			},
			want: want{
				err:   nil,
				confs: 1, // Configuration.meta
				deps:  1, // Not adding Configuration.meta itself to not send it to cacheDependencies() for download
			},
		},
		"SuccessfulConfigMetaAndPkg": {
			// config-meta
			// └─►function-dep-1
			// config-pkg
			// └─►provider-dep-1
			reason: "All dependencies should be successfully added from both Configuration.meta and Configuration.pkg",
			args: args{
				extensions: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "meta.pkg.crossplane.io/v1alpha1",
							"kind":       "Configuration",
							"metadata": map[string]interface{}{
								"name": "config-meta",
							},
							"spec": map[string]interface{}{
								"dependsOn": []map[string]interface{}{
									{
										"function": "function-dep-1",
										"version":  "v1.3.0",
									},
								},
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "pkg.crossplane.io/v1alpha1",
							"kind":       "Configuration",
							"metadata": map[string]interface{}{
								"name": "config-pkg",
							},
							"spec": map[string]interface{}{
								"package": "config-pkg:v1.3.0",
							},
						},
					},
				},
				fetchMock: fetchMockFunc,
			},
			want: want{
				err:   nil,
				confs: 2, // Configuration.meta and Configuration.pkg
				deps:  3, // 1 Configuration.pkg, 1 provider, 1 function
			},
		},
	}
	for name, tc := range cases {
		fs := afero.NewMemMapFs()
		w := &bytes.Buffer{}

		m := NewManager("", fs, w)
		t.Run(name, func(t *testing.T) {
			m.fetcher = &MockFetcher{tc.args.fetchMock}
			err := m.PrepExtensions(tc.args.extensions)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nPrepExtensions(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			err = m.addDependencies(m.confs)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\naddDependencies(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.confs, len(m.confs)); diff != "" {
				t.Errorf("\n%s\naddDependencies(...): -want confs, +got confs:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.deps, len(m.deps)); diff != "" {
				t.Errorf("\n%s\naddDependencies(...): -want deps, +got deps:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAddDependencies(t *testing.T) {
	cd1 := static.NewLayer(configDep1Yaml, types.OCILayer)
	cd2 := static.NewLayer(configDep2Yaml, types.OCILayer)
	pd1 := static.NewLayer(providerYaml, types.OCILayer)
	fd1 := static.NewLayer(funcYaml, types.OCILayer)

	fetchMockFunc := func(image string) (*conregv1.Layer, error) {
		switch image {
		case "config-dep-1:v1.3.0":
			return &cd1, nil
		case "config-dep-2:v1.3.0":
			return &cd2, nil
		case "provider-dep-1:v1.3.0":
			return &pd1, nil
		case "function-dep-1:v1.3.0":
			return &fd1, nil
		default:
			return nil, fmt.Errorf("unknown image: %s", image)
		}
	}

	type args struct {
		extensions []*unstructured.Unstructured
		fetchMock  func(image string) (*conregv1.Layer, error)
	}
	type want struct {
		confs int
		deps  int
		err   error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulDependenciesAddition": {
			// config-dep-1
			// └─►config-dep-2
			//   ├─►provider-dep-1
			//   └─►function-dep-1
			reason: "All dependencies should be successfully fetched and added",
			args: args{
				fetchMock: fetchMockFunc,
				extensions: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "pkg.crossplane.io/v1alpha1",
							"kind":       "Configuration",
							"metadata": map[string]interface{}{
								"name": "config-dep-1",
							},
							"spec": map[string]interface{}{
								"package": "config-dep-1:v1.3.0",
							},
						},
					},
				},
			},
			want: want{
				confs: 2, // 1 Base configuration (config-dep-1), 1 child configuration (config-dep-2)
				deps:  4, // 2 configurations (config-dep-1, config-dep-2), 1 provider (provider-dep-1), 1 function (function-dep-1)
				err:   nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			w := &bytes.Buffer{}

			m := NewManager("", fs, w)
			_ = m.PrepExtensions(tc.args.extensions)

			m.fetcher = &MockFetcher{tc.args.fetchMock}
			err := m.addDependencies(m.confs)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\naddDependencies(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.confs, len(m.confs)); diff != "" {
				t.Errorf("\n%s\naddDependencies(...): -want confs, +got confs:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.deps, len(m.deps)); diff != "" {
				t.Errorf("\n%s\naddDependencies(...): -want deps, +got deps:\n%s", tc.reason, diff)
			}
		})
	}
}

type MockFetcher struct {
	fetch func(image string) (*conregv1.Layer, error)
}

func (m *MockFetcher) FetchBaseLayer(image string) (*conregv1.Layer, error) {
	return m.fetch(image)
}
