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

	cronOperationYAML := `apiVersion: ops.crossplane.io/v1alpha1
kind: CronOperation
metadata:
  name: test-cron
spec:
  schedule: "*/5 * * * *"
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
					_ = afero.WriteFile(fs, "cronop.yaml", []byte(cronOperationYAML), 0o644)
					return fs
				}(),
				path: "cronop.yaml",
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
