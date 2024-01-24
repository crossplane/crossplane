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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	coolResource = map[string]interface{}{
		"apiVersion": "example.org/v1alpha1",
		"kind":       "ComposedResource",
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"crossplane.io/composition-resource-name": "resource-a",
			},
			"name": "test-validate-a",
		},
		"spec": map[string]interface{}{
			"coolField": "I'm cool!",
		},
	}
	coolerResource = map[string]interface{}{
		"apiVersion": "example.org/v1alpha1",
		"kind":       "ComposedResource",
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"crossplane.io/composition-resource-name": "resource-b",
			},
			"name": "test-validate-b",
		},
		"spec": map[string]interface{}{
			"coolerField": "I'm cooler!",
		},
	}
)

func TestFileLoaderLoad(t *testing.T) {
	type args struct {
		Path string
	}
	type want struct {
		resources []*unstructured.Unstructured
		err       error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Successfully load resources from file",
			args: args{
				Path: "testdata/resources.yaml",
			},
			want: want{
				resources: []*unstructured.Unstructured{
					{
						Object: coolResource,
					},
					{
						Object: coolerResource,
					},
				},
			},
		},
		"Error": {
			reason: "Error loading resources from file",
			args: args{
				Path: "testdata/does-not-exist.yaml",
			},
			want: want{
				resources: nil,
				err:       cmpopts.AnyError,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &FileLoader{
				path: tc.args.Path,
			}
			got, err := f.Load()
			if diff := cmp.Diff(tc.want.resources, got); diff != "" {
				t.Errorf("%s\nLoad(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nLoad(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFolderLoaderLoad(t *testing.T) {
	type args struct {
		Path string
	}
	type want struct {
		resources []*unstructured.Unstructured
		err       error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Successfully load resources from folder",
			args: args{
				Path: "testdata/folder",
			},
			want: want{
				resources: []*unstructured.Unstructured{
					{
						Object: coolResource,
					},
					{
						Object: coolerResource,
					},
				},
			},
		},
		"Error": {
			reason: "Error loading resources from folder",
			args: args{
				Path: "testdata/does-not-exist",
			},
			want: want{
				resources: nil,
				err:       cmpopts.AnyError,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &FolderLoader{
				path: tc.args.Path,
			}
			got, err := f.Load()
			if diff := cmp.Diff(tc.want.resources, got); diff != "" {
				t.Errorf("%s\nLoad(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nLoad(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestStreamToUnstructured(t *testing.T) {
	type args struct {
		stream [][]byte
	}
	type want struct {
		resources []*unstructured.Unstructured
		err       error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Successfully parse stream to unstructured resources",
			args: args{
				stream: [][]byte{
					[]byte("apiVersion: v1\nkind: Pod\nmetadata:\n  name: test"),
				},
			},
			want: want{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Pod",
							"metadata": map[string]interface{}{
								"name": "test",
							},
						},
					},
				},
			},
		},
		"Error": {
			reason: "Error parsing stream to unstructured resources",
			args: args{
				stream: [][]byte{
					[]byte("this is not a yaml"),
				},
			},
			want: want{
				resources: nil,
				err:       cmpopts.AnyError,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := streamToUnstructured(tc.args.stream)
			if diff := cmp.Diff(tc.want.resources, got); diff != "" {
				t.Errorf("%s\nstreamToUnstructured(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nstreamToUnstructured(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
