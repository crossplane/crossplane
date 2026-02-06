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

package op

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	opsv1alpha1 "github.com/crossplane/crossplane/v2/apis/ops/v1alpha1"
)

func TestLoadOperation(t *testing.T) {
	type args struct {
		fs   afero.Fs
		path string
	}
	type want struct {
		op  *opsv1alpha1.Operation
		err error
	}

	invalidYAML := "invalid: yaml: content: ["

	notAnOperationYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-data
data:
  foo: bar`

	cronOperationYAML := `apiVersion: ops.crossplane.io/v1alpha1
kind: CronOperation
metadata:
  name: test-operation
spec:
  schedule: "*/5 * * * *"
  operationTemplate:
    spec:
      mode: Pipeline
      pipeline:
      - step: test-step
        functionRef:
          name: test-function`

	watchOperationYAML := `apiVersion: ops.crossplane.io/v1alpha1
kind: WatchOperation
metadata:
  name: test-operation
spec:
  watch:
    apiVersion: v1
    kind: Secret
    matchLabels:
      foo: bar
  operationTemplate:
    spec:
      mode: Pipeline
      pipeline:
      - step: test-step
        functionRef:
          name: test-function`

	wrongVersionYAML := `apiVersion: ops.crossplane.io/v1beta1
kind: Operation
metadata:
  name: test-op
spec:
  mode: Pipeline
  pipeline:
  - step: test-step
    functionRef:
      name: test-function`

	validOperationYAML := `apiVersion: ops.crossplane.io/v1alpha1
kind: Operation
metadata:
  name: test-operation
spec:
  mode: Pipeline
  pipeline:
  - step: test-step
    functionRef:
      name: test-function`

	validOperation := &opsv1alpha1.Operation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "ops.crossplane.io/v1alpha1",
			Kind:       "Operation",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-operation",
		},
		Spec: opsv1alpha1.OperationSpec{
			Mode: opsv1alpha1.OperationModePipeline,
			Pipeline: []opsv1alpha1.PipelineStep{
				{
					Step: "test-step",
					FunctionRef: opsv1alpha1.FunctionReference{
						Name: "test-function",
					},
				},
			},
		},
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"FileNotFound": {
			reason: "Should return an error if the operation file doesn't exist",
			args: args{
				fs:   afero.NewMemMapFs(),
				path: "nonexistent.yaml",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"InvalidYAML": {
			reason: "Should return an error if the file contains invalid YAML",
			args: args{
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = afero.WriteFile(fs, "invalid.yaml", []byte(invalidYAML), 0o644)
					return fs
				}(),
				path: "invalid.yaml",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"WrongKind": {
			reason: "Should return an error if the resource is not an Operation",
			args: args{
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = afero.WriteFile(fs, "notop.yaml", []byte(notAnOperationYAML), 0o644)
					return fs
				}(),
				path: "notop.yaml",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"WrongAPIVersion": {
			reason: "Should return an error if the API version is not supported",
			args: args{
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = afero.WriteFile(fs, "wrongversion.yaml", []byte(wrongVersionYAML), 0o644)
					return fs
				}(),
				path: "wrongversion.yaml",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ValidOperation": {
			reason: "Should successfully load a valid Operation",
			args: args{
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = afero.WriteFile(fs, "operation.yaml", []byte(validOperationYAML), 0o644)
					return fs
				}(),
				path: "operation.yaml",
			},
			want: want{
				op: validOperation,
			},
		},
		"ValidCronOperation": {
			reason: "Should successfully load a valid Operation from a CronOperation",
			args: args{
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = afero.WriteFile(fs, "cronoperation.yaml", []byte(cronOperationYAML), 0o644)
					return fs
				}(),
				path: "cronoperation.yaml",
			},
			want: want{
				op: validOperation,
			},
		},
		"ValidWatchOperation": {
			reason: "Should successfully load a valid Operation from a WatchOperation without injecting watched resource",
			args: args{
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = afero.WriteFile(fs, "watchoperation.yaml", []byte(watchOperationYAML), 0o644)
					return fs
				}(),
				path: "watchoperation.yaml",
			},
			want: want{
				op: validOperation,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := LoadOperation(tc.args.fs, tc.args.path)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nLoadOperation(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.op, got); diff != "" {
				t.Errorf("\n%s\nLoadOperation(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestInjectWatchedResource(t *testing.T) {
	type args struct {
		op      *opsv1alpha1.Operation
		watched *unstructured.Unstructured
	}

	sn := "cool-secret"
	sns := "default"

	cases := map[string]struct {
		reason string
		args   args
		want   *opsv1alpha1.Operation
	}{
		"InjectIntoAllSteps": {
			reason: "Should inject the watched resource selector into all pipeline steps",
			args: args{
				op: &opsv1alpha1.Operation{
					Spec: opsv1alpha1.OperationSpec{
						Mode: opsv1alpha1.OperationModePipeline,
						Pipeline: []opsv1alpha1.PipelineStep{
							{
								Step: "step-one",
								FunctionRef: opsv1alpha1.FunctionReference{
									Name: "fn-one",
								},
							},
							{
								Step: "step-two",
								FunctionRef: opsv1alpha1.FunctionReference{
									Name: "fn-two",
								},
							},
						},
					},
				},
				watched: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Secret",
						"metadata": map[string]any{
							"name":      "cool-secret",
							"namespace": "default",
						},
					},
				},
			},
			want: &opsv1alpha1.Operation{
				Spec: opsv1alpha1.OperationSpec{
					Mode: opsv1alpha1.OperationModePipeline,
					Pipeline: []opsv1alpha1.PipelineStep{
						{
							Step: "step-one",
							FunctionRef: opsv1alpha1.FunctionReference{
								Name: "fn-one",
							},
							Requirements: &opsv1alpha1.FunctionRequirements{
								RequiredResources: []opsv1alpha1.RequiredResourceSelector{
									{
										RequirementName: opsv1alpha1.RequirementNameWatchedResource,
										APIVersion:      "v1",
										Kind:            "Secret",
										Name:            &sn,
										Namespace:       &sns,
									},
								},
							},
						},
						{
							Step: "step-two",
							FunctionRef: opsv1alpha1.FunctionReference{
								Name: "fn-two",
							},
							Requirements: &opsv1alpha1.FunctionRequirements{
								RequiredResources: []opsv1alpha1.RequiredResourceSelector{
									{
										RequirementName: opsv1alpha1.RequirementNameWatchedResource,
										APIVersion:      "v1",
										Kind:            "Secret",
										Name:            &sn,
										Namespace:       &sns,
									},
								},
							},
						},
					},
				},
			},
		},
		"ClusterScopedResource": {
			reason: "Should not set namespace for cluster-scoped resources",
			args: args{
				op: &opsv1alpha1.Operation{
					Spec: opsv1alpha1.OperationSpec{
						Mode: opsv1alpha1.OperationModePipeline,
						Pipeline: []opsv1alpha1.PipelineStep{
							{
								Step: "test-step",
								FunctionRef: opsv1alpha1.FunctionReference{
									Name: "test-fn",
								},
							},
						},
					},
				},
				watched: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Node",
						"metadata": map[string]any{
							"name": "my-node",
						},
					},
				},
			},
			want: &opsv1alpha1.Operation{
				Spec: opsv1alpha1.OperationSpec{
					Mode: opsv1alpha1.OperationModePipeline,
					Pipeline: []opsv1alpha1.PipelineStep{
						{
							Step: "test-step",
							FunctionRef: opsv1alpha1.FunctionReference{
								Name: "test-fn",
							},
							Requirements: &opsv1alpha1.FunctionRequirements{
								RequiredResources: []opsv1alpha1.RequiredResourceSelector{
									{
										RequirementName: opsv1alpha1.RequirementNameWatchedResource,
										APIVersion:      "v1",
										Kind:            "Node",
										Name:            ptr.To("my-node"),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			InjectWatchedResource(tc.args.op, tc.args.watched)
			if diff := cmp.Diff(tc.want, tc.args.op); diff != "" {
				t.Errorf("\n%s\nInjectWatchedResource(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
