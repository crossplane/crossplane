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

package install

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/stacks"
)

const (
	jobPodName            = "job-pod-123"
	podLogOutputMalformed = `)(&not valid yaml?()!`
	podLogOutput          = `
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: mytypes.samples.upbound.io
spec:
  group: samples.upbound.io
  names:
  kind: Mytype
  listKind: MytypeList
  plural: mytypes
  singular: mytype
  scope: Namespaced
  version: v1alpha1

---
apiVersion: stacks.crossplane.io/v1alpha1
kind: Stack
metadata:
  creationTimestamp: null
spec:
  company: Upbound
  controller:
    deployment:
      name: crossplane-sample-stack
      spec:
        replicas: 1
        selector:
          matchLabels:
            core.crossplane.io/name: crossplane-sample-stack
        strategy: {}
        template:
          metadata:
            labels:
              core.crossplane.io/name: crossplane-sample-stack
            name: sample-stack-controller
          spec:
            containers:
            - env:
              - name: POD_NAME
                valueFrom:
                  fieldRef:
                    fieldPath: metadata.name
              - name: POD_NAMESPACE
                valueFrom:
                  fieldRef:
                    fieldPath: metadata.namespace
              image: crossplane/sample-stack:latest
              name: sample-stack-controller
  customresourcedefinitions:
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Mytype
  description: |
    Markdown describing this sample Crossplane stack project.
  icons:
  - base64Data: bW9jay1pY29uLWRh
    mediatype: image/jpeg
  keywords:
  - samples
  - examples
  - tutorials
  license: Apache-2.0
  website: https://upbound.io
  source: https://github.com/crossplaneio/sample-stack
  maintainers:
  - email: jared@upbound.io
    name: Jared Watts
  owners:
  - email: bassam@upbound.io
    name: Bassam Tabbara
  permissions:
    rules:
    - apiGroups:
      - ""
      resources:
      - configmaps
      - events
      - secrets
      verbs:
      - '*'
    - apiGroups:
      - samples.upbound.io/v1alpha1
      resources:
      - mytypes
      verbs:
      - '*'
  permissionScope: Namespaced
  title: Sample Crossplane Stack
  version: 0.0.1
status:
 Conditions: null
`
)

var (
	_ jobCompleter = &stackInstallJobCompleter{}
)

// Job modifiers
type jobModifier func(*batchv1.Job)

func withJobConditions(jobConditionType batchv1.JobConditionType, message string) jobModifier {
	return func(j *batchv1.Job) {
		j.Status.Conditions = []batchv1.JobCondition{
			{
				Type:    jobConditionType,
				Status:  corev1.ConditionTrue,
				Message: message,
			},
		}
	}
}

func job(jm ...jobModifier) *batchv1.Job {
	j := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      resourceName,
		},
	}

	for _, m := range jm {
		m(j)
	}

	return j
}

type mockJobCompleter struct {
	MockHandleJobCompletion func(ctx context.Context, i v1alpha1.StackInstaller, job *batchv1.Job) error
}

func (m *mockJobCompleter) handleJobCompletion(ctx context.Context, i v1alpha1.StackInstaller, job *batchv1.Job) error {
	return m.MockHandleJobCompletion(ctx, i, job)
}

type mockPodLogReader struct {
	MockGetPodLogReader func(string, string) (io.ReadCloser, error)
}

func (m *mockPodLogReader) GetReader(namespace, name string) (io.ReadCloser, error) {
	return m.MockGetPodLogReader(namespace, name)
}

type mockReadCloser struct {
	MockRead  func(p []byte) (n int, err error)
	MockClose func() error
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	return m.MockRead(p)
}

func (m *mockReadCloser) Close() (err error) {
	return m.MockClose()
}

func TestHandleJobCompletion(t *testing.T) {
	errBoom := errors.New("boom")

	type want struct {
		ext *v1alpha1.StackInstall
		err error
	}

	tests := []struct {
		name string
		jc   *stackInstallJobCompleter
		ext  *v1alpha1.StackInstall
		job  *batchv1.Job
		want want
	}{
		{
			name: "NoPodsFoundForJob",
			jc: &stackInstallJobCompleter{
				client: &test.MockClient{
					MockList: func(ctx context.Context, list runtime.Object, _ ...client.ListOption) error {
						// LIST pods returns an empty list
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
			},
			ext: resource(),
			job: job(),
			want: want{
				ext: resource(),
				err: errors.Errorf("pod list for job %s should only have 1 item, actual: 0", resourceName),
			},
		},
		{
			name: "FailToGetJobPodLogs",
			jc: &stackInstallJobCompleter{
				client: &test.MockClient{
					MockList: func(ctx context.Context, list runtime.Object, _ ...client.ListOption) error {
						// LIST pods returns a pod for the job
						*list.(*corev1.PodList) = corev1.PodList{
							Items: []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: jobPodName}}},
						}
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				podLogReader: &mockPodLogReader{
					MockGetPodLogReader: func(string, string) (io.ReadCloser, error) {
						return nil, errBoom
					},
				},
			},
			ext: resource(),
			job: job(),
			want: want{
				ext: resource(),
				err: errors.Wrapf(errBoom, "failed to get logs request stream from pod %s", jobPodName),
			},
		},
		{
			name: "FailToReadJobPodLogStream",
			jc: &stackInstallJobCompleter{
				client: &test.MockClient{
					MockList: func(ctx context.Context, list runtime.Object, _ ...client.ListOption) error {
						// LIST pods returns a pod for the job
						*list.(*corev1.PodList) = corev1.PodList{
							Items: []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: jobPodName}}},
						}
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				podLogReader: &mockPodLogReader{
					MockGetPodLogReader: func(string, string) (io.ReadCloser, error) {
						return &mockReadCloser{
							MockRead: func(p []byte) (n int, err error) {
								return 0, errBoom
							},
							MockClose: func() error { return nil },
						}, nil
					},
				},
			},
			ext: resource(),
			job: job(),
			want: want{
				ext: resource(),
				err: errors.Wrapf(errBoom, "failed to copy logs request stream from pod %s", jobPodName),
			},
		},
		{
			name: "FailToParseJobPodLogOutput",
			jc: &stackInstallJobCompleter{
				client: &test.MockClient{
					MockList: func(ctx context.Context, list runtime.Object, _ ...client.ListOption) error {
						// LIST pods returns a pod for the job
						*list.(*corev1.PodList) = corev1.PodList{
							Items: []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: jobPodName}}},
						}
						return nil
					},
				},
				podLogReader: &mockPodLogReader{
					MockGetPodLogReader: func(string, string) (io.ReadCloser, error) {
						return ioutil.NopCloser(bytes.NewReader([]byte(podLogOutputMalformed))), nil
					},
				},
			},
			ext: resource(),
			job: job(),
			want: want{
				ext: resource(),
				err: errors.WithStack(errors.Errorf("failed to parse output from job %s: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}", resourceName)),
			},
		},
		{
			name: "HandleJobCompletionSuccess",
			jc: &stackInstallJobCompleter{
				client: &test.MockClient{
					MockList: func(ctx context.Context, list runtime.Object, _ ...client.ListOption) error {
						// LIST pods returns a pod for the job
						*list.(*corev1.PodList) = corev1.PodList{
							Items: []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: jobPodName}}},
						}
						return nil
					},
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error { return nil },
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET stack returns the stack instance that was created from the pod log output
						*obj.(*v1alpha1.Stack) = v1alpha1.Stack{
							ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: namespace},
						}
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				podLogReader: &mockPodLogReader{
					MockGetPodLogReader: func(string, string) (io.ReadCloser, error) {
						return ioutil.NopCloser(bytes.NewReader([]byte(podLogOutput))), nil
					},
				},
			},
			ext: resource(),
			job: job(),
			want: want{
				ext: resource(withStackRecord(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
				err: nil,
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := tt.jc.handleJobCompletion(ctx, tt.ext, tt.job)

			if diff := cmp.Diff(tt.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("handleJobCompletion(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.ext, tt.ext, test.EquateConditions()); diff != "" {
				t.Errorf("handleJobCompletion(): -want, +got:\n%v", diff)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type want struct {
		result reconcile.Result
		err    error
		ext    *v1alpha1.StackInstall
	}

	tests := []struct {
		name    string
		handler *stackInstallHandler
		want    want
	}{
		{
			name: "CreateInstallJob",
			handler: &stackInstallHandler{
				kube: &test.MockClient{
					MockCreate:       func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error { return nil },
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				executorInfo: &stacks.ExecutorInfo{Image: stackPackageImage},
				ext:          resource(),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: resource(
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace}),
				),
			},
		},
		{
			name: "InstallJobNotCompleted",
			handler: &stackInstallHandler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET Job returns an uncompleted job
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				ext: resource(
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: resource(
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace}),
				),
			},
		},
		{
			name: "HandleSuccessfulInstallJob",
			handler: &stackInstallHandler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET Job returns a successful/completed job
						*obj.(*batchv1.Job) = *(job(withJobConditions(batchv1.JobComplete, "")))
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				jobCompleter: &mockJobCompleter{
					MockHandleJobCompletion: func(ctx context.Context, i v1alpha1.StackInstaller, job *batchv1.Job) error { return nil },
				},
				executorInfo: &stacks.ExecutorInfo{Image: stackPackageImage},
				ext: resource(
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: resource(
					withConditions(runtimev1alpha1.Available(), runtimev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace}),
				),
			},
		},
		{
			name: "HandleFailedInstallJob",
			handler: &stackInstallHandler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET Job returns a failed job
						*obj.(*batchv1.Job) = *(job(withJobConditions(batchv1.JobFailed, "mock job failure message")))
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				jobCompleter: &mockJobCompleter{
					MockHandleJobCompletion: func(ctx context.Context, i v1alpha1.StackInstaller, job *batchv1.Job) error { return nil },
				},
				executorInfo: &stacks.ExecutorInfo{Image: stackPackageImage},
				ext: resource(
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				ext: resource(
					withConditions(
						runtimev1alpha1.Creating(),
						runtimev1alpha1.ReconcileError(errors.New("mock job failure message")),
					),
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace}),
				),
			},
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, gotErr := tt.handler.create(ctx)

			if diff := cmp.Diff(tt.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("create() -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.result, gotResult); diff != "" {
				t.Errorf("create() -want, +got:\n%v", diff)
			}

			if diff := cmp.Diff(tt.want.ext, tt.handler.ext, test.EquateConditions()); diff != "" {
				t.Errorf("create() -want, +got:\n%v", diff)
			}
		})
	}
}
