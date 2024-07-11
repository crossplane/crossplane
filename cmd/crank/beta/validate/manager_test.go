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
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	conregv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
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

	providerYaml = []byte(`apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-dep-1
spec:
  package: provider-dep-1:v1.3.0
---

`)

	funcYaml = []byte(`apiVersion: pkg.crossplane.io/v1beta1
kind: Function
metadata:
  name: function-dep-1
spec:
  package: function-dep-1:v1.3.0
---

`)
)

func TestConfigurationTypeSupport(t *testing.T) {
	fs := afero.NewMemMapFs()
	w := &bytes.Buffer{}

	m := NewManager(".crossplane/cache", fs, w)
	confpkg := static.NewLayer(configDep2Yaml, types.OCILayer)
	pd := static.NewLayer(providerYaml, types.OCILayer)
	fd := static.NewLayer(funcYaml, types.OCILayer)

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
		"SuccessfulDependenciesAddition": {
			reason: "All dependencies should be successfully added from both Configuration.pkg and Configuration.meta",
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
										"provider": "provider-dep-1",
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
								"package": "config-dep-2:v1.3.0",
							},
						},
					},
				}, fetchMock: func(image string) (*conregv1.Layer, error) {
					switch image {
					case "config-dep-2:v1.3.0":
						return &confpkg, nil
					case "provider-dep-1:v1.3.0":
						return &pd, nil
					case "function-dep-1:v1.3.0":
						return &fd, nil
					default:
						return nil, fmt.Errorf("unknown image: %s", image)
					}
				},
			},
			want: want{
				err:   nil,
				confs: 2,
				deps:  3,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m.fetcher = &MockFetcher{tc.args.fetchMock}
			err := m.PrepExtensions(tc.args.extensions)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nPrepExtensions(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			err = m.addDependencies()
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
