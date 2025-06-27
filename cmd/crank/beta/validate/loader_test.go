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

func TestNewLoader(t *testing.T) {
	type args struct {
		input string
	}
	type want struct {
		loader Loader
		err    error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SucessWithStdin": {
			reason: "Successfully create loader from stdin",
			args: args{
				input: "-",
			},
			want: want{
				loader: &StdinLoader{},
			},
		},
		"SucessWithFile": {
			reason: "Successfully create loader from file",
			args: args{
				input: "testdata/resources.yaml",
			},
			want: want{
				loader: &FileLoader{path: "testdata/resources.yaml"},
			},
		},
		"SucessWithDirectory": {
			reason: "Successfully create loader from directory",
			args: args{
				input: "testdata/folder",
			},
			want: want{
				loader: &FolderLoader{path: "testdata/folder"},
			},
		},
		"SucessWithMultiple": {
			reason: "Successfully create loader from multiple sources",
			args: args{
				input: "testdata/resources.yaml,testdata/folder",
			},
			want: want{
				loader: &MultiLoader{loaders: []Loader{
					&FileLoader{path: "testdata/resources.yaml"},
					&FolderLoader{path: "testdata/folder"},
				}},
			},
		},
		"ErrorWithFile": {
			reason: "Error creating loader from file that does not exist",
			args: args{
				input: "testdata/does-not-exist.yaml",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorWithFolder": {
			reason: "Error creating loader from folder that does not exist",
			args: args{
				input: "testdata/does-not-exist",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorWithMultiple": {
			reason: "Error creating loader from multiple sources that does not exist",
			args: args{
				input: "testdata/does-not-exist.yaml,testdata/does-not-exist",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := NewLoader(tc.args.input)
			if diff := cmp.Diff(
				tc.want.loader,
				got,
				cmpopts.IgnoreUnexported(FileLoader{}, FolderLoader{}, MultiLoader{}),
			); diff != "" {
				t.Errorf("%s\nLoad(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nLoad(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMultiLoaderLoad(t *testing.T) {
	type args struct {
		loaders []Loader
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
			reason: "Successfully load resources from file and folder loaders",
			args: args{
				loaders: []Loader{
					&FileLoader{
						path: "testdata/resources.yaml",
					},
					&FolderLoader{
						path: "testdata/folder",
					},
				},
			},
			want: want{
				resources: []*unstructured.Unstructured{
					{
						Object: coolResource,
					},
					{
						Object: coolerResource,
					},
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
			reason: "Error loading resources from invalid loader",
			args: args{
				[]Loader{
					&FileLoader{
						path: "testdata/does-not-exist.yaml",
					},
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
			f := &MultiLoader{
				loaders: tc.args.loaders,
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
		"CompositionWithPipelineResources": {
			reason: "Successfully parse Composition with pipeline input resources to unstructured resources",
			args: args{
				stream: [][]byte{
					[]byte(`
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example-composition
spec:
  compositeTypeRef:
    apiVersion: example.crossplane.io/v1alpha1
    kind: ExampleComposite
  pipeline:
    - step: patch-and-transform
      functionRef:
        name: example-function
      input:
        apiVersion: pt.fn.crossplane.io/v1beta1
        kind: Resources
        resources:
          - name: instanceNodeRole
            base:
              apiVersion: iam.aws.crossplane.io/v1beta1
              kind: Role
              spec: {}
`),
				},
			},
			want: want{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "pt.fn.crossplane.io/v1beta1",
							"kind":       "Resources",
							"resources": []interface{}{
								map[string]interface{}{
									"name": "instanceNodeRole",
									"base": map[string]interface{}{
										"apiVersion": "iam.aws.crossplane.io/v1beta1",
										"kind":       "Role",
										"spec":       map[string]interface{}{},
									},
								},
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.crossplane.io/v1",
							"kind":       "Composition",
							"metadata": map[string]interface{}{
								"name": "example-composition",
							},
							"spec": map[string]interface{}{
								"compositeTypeRef": map[string]interface{}{
									"apiVersion": "example.crossplane.io/v1alpha1",
									"kind":       "ExampleComposite",
								},
								"pipeline": []interface{}{
									map[string]interface{}{
										"step": "patch-and-transform",
										"functionRef": map[string]interface{}{
											"name": "example-function",
										},
										"input": map[string]interface{}{
											"apiVersion": "pt.fn.crossplane.io/v1beta1",
											"kind":       "Resources",
											"resources": []interface{}{
												map[string]interface{}{
													"name": "instanceNodeRole",
													"base": map[string]interface{}{
														"apiVersion": "iam.aws.crossplane.io/v1beta1",
														"kind":       "Role",
														"spec":       map[string]interface{}{},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				err: nil,
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
