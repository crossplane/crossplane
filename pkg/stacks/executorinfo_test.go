/*
Copyright 2019 The Crossplane Authors.

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

package stacks

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/pkg/test"
	"github.com/crossplaneio/crossplane-runtime/pkg/util"
)

var (
	_ ExecutorInfoDiscoverer = &KubeExecutorInfoDiscoverer{}
)

func TestExecutorInfoImage(t *testing.T) {
	type fields struct {
		image string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"Empty", fields{image: ""}, ""},
		{"Simple", fields{image: "foo"}, "foo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ei := &ExecutorInfo{
				Image: tt.fields.image,
			}
			if got := ei.Image; got != tt.want {
				t.Errorf("executorInfo.Image() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExecutorInfoDiscoverer_Discover(t *testing.T) {
	type want struct {
		ei  *ExecutorInfo
		err error
	}

	tests := []struct {
		name      string
		imageName string
		d         ExecutorInfoDiscoverer
		want      want
	}{
		{
			name: "FailedGetRunningPod",
			d: &KubeExecutorInfoDiscoverer{
				Client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return errors.New("test-get-pod-error")
					},
				},
			},
			want: want{
				ei:  nil,
				err: errors.New("test-get-pod-error"),
			},
		},
		{
			name: "FailedGetContainerImage",
			d: &KubeExecutorInfoDiscoverer{
				Client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*corev1.Pod) = corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace}}
						return nil
					},
				},
			},
			want: want{
				ei:  nil,
				err: errors.New("failed to find image for container "),
			},
		},
		{
			name: "SuccessfulDiscovery",
			d: &KubeExecutorInfoDiscoverer{
				Client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*corev1.Pod) = corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{{Name: "foo", Image: "foo-image"}},
							},
						}
						return nil
					},
				},
			},
			want: want{
				ei:  &ExecutorInfo{Image: "foo-image"},
				err: nil,
			},
		},
		{
			name:      "SuccessfulDebugOverride",
			imageName: "foo-image",
			d: &KubeExecutorInfoDiscoverer{
				Client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return fmt.Errorf("test should not call Get")
					},
				},
			},
			want: want{
				ei:  &ExecutorInfo{Image: "foo-image"},
				err: nil,
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialEnvVars := saveEnvVars()
			defer restoreEnvVars(initialEnvVars)

			os.Setenv(util.PodNameEnvVar, "podName")
			os.Setenv(util.PodNamespaceEnvVar, "podNamespace")
			os.Setenv(PodImageNameEnvVar, tt.imageName)

			got, gotErr := tt.d.Discover(ctx)

			if diff := cmp.Diff(tt.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("Discover() -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.ei, got); diff != "" {
				t.Errorf("Discover() -want, +got:\n%v\n\n%v\n%v", diff, tt.want.ei, got)
			}
		})
	}
}

type envvars struct {
	podName      string
	podNamespace string
}

func saveEnvVars() envvars {
	return envvars{
		podName:      os.Getenv(util.PodNameEnvVar),
		podNamespace: os.Getenv(util.PodNamespaceEnvVar),
	}
}

func restoreEnvVars(initialEnvVars envvars) {
	os.Setenv(util.PodNameEnvVar, initialEnvVars.podName)
	os.Setenv(util.PodNamespaceEnvVar, initialEnvVars.podNamespace)
}
