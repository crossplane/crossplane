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

package request

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/extensions"
	"github.com/crossplaneio/crossplane/pkg/apis/extensions/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	namespace    = "cool-namespace"
	uid          = types.UID("definitely-a-uuid")
	resourceName = "cool-extensionrequest"
	jobPodName   = "job-pod-123"

	extensionPackageImage = "cool/extension-package:rad"

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
apiVersion: extensions.crossplane.io/v1alpha1
kind: Extension
metadata:
  creationTimestamp: null
spec:
  company: Upbound
  controller:
    deployment:
      name: crossplane-sample-extension
      spec:
        replicas: 1
        selector:
          matchLabels:
            core.crossplane.io/name: crossplane-sample-extension
        strategy: {}
        template:
          metadata:
            labels:
              core.crossplane.io/name: crossplane-sample-extension
            name: sample-extension-controller
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
              image: crossplane/sample-extension:latest
              name: sample-extension-controller
  customresourcedefinitions:
    owns:
    - apiVersion: samples.upbound.io/v1alpha1
      kind: Mytype
  description: |
    Markdown describing this sample Crossplane extension project.
  icons:
  - base64Data: bW9jay1pY29uLWRh
    mediatype: image/jpeg
  keywords:
  - samples
  - examples
  - tutorials
  license: Apache-2.0
  links:
  - description: Website
    url: https://upbound.io
  - description: Source Code
    url: https://github.com/crossplaneio/sample-extension
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
      - secrets
      - serviceaccounts
      - events
      - namespaces
      verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
      - delete
  title: Sample Crossplane Extension
  version: 0.0.1
status:
 Conditions: null
`
)

var (
	ctx = context.Background()
)

func init() {
	_ = extensions.AddToScheme(scheme.Scheme)
}

// Test that our Reconciler implementation satisfies the Reconciler interface.
var _ reconcile.Reconciler = &Reconciler{}

// ************************************************************************************************
// Resource modifiers
// ************************************************************************************************
type resourceModifier func(*v1alpha1.ExtensionRequest)

func withConditions(c ...corev1alpha1.Condition) resourceModifier {
	return func(r *v1alpha1.ExtensionRequest) { r.Status.SetConditions(c...) }
}

func withInstallJob(jobRef *corev1.ObjectReference) resourceModifier {
	return func(r *v1alpha1.ExtensionRequest) { r.Status.InstallJob = jobRef }
}

func withExtensionRecord(extensionRecord *corev1.ObjectReference) resourceModifier {
	return func(r *v1alpha1.ExtensionRequest) { r.Status.ExtensionRecord = extensionRecord }
}

func resource(rm ...resourceModifier) *v1alpha1.ExtensionRequest {
	r := &v1alpha1.ExtensionRequest{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       resourceName,
			UID:        uid,
			Finalizers: []string{},
		},
		Spec: v1alpha1.ExtensionRequestSpec{},
	}

	for _, m := range rm {
		m(r)
	}

	return r
}

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

// ************************************************************************************************
// mock implementations
// ************************************************************************************************
type mockFactory struct {
	MockNewHandler func(context.Context, *v1alpha1.ExtensionRequest, client.Client, kubernetes.Interface, executorInfo) handler
}

func (f *mockFactory) newHandler(ctx context.Context, i *v1alpha1.ExtensionRequest,
	kube client.Client, kubeclient kubernetes.Interface, ei executorInfo) handler {
	return f.MockNewHandler(ctx, i, kube, kubeclient, ei)
}

type mockHandler struct {
	MockSync   func(context.Context) (reconcile.Result, error)
	MockCreate func(context.Context) (reconcile.Result, error)
	MockUpdate func(context.Context) (reconcile.Result, error)
}

func (m *mockHandler) sync(ctx context.Context) (reconcile.Result, error) {
	return m.MockSync(ctx)
}

func (m *mockHandler) create(ctx context.Context) (reconcile.Result, error) {
	return m.MockCreate(ctx)
}

func (m *mockHandler) update(ctx context.Context) (reconcile.Result, error) {
	return m.MockUpdate(ctx)
}

type mockJobCompleter struct {
	MockHandleJobCompletion func(ctx context.Context, i *v1alpha1.ExtensionRequest, job *batchv1.Job) error
}

func (m *mockJobCompleter) handleJobCompletion(ctx context.Context, i *v1alpha1.ExtensionRequest, job *batchv1.Job) error {
	return m.MockHandleJobCompletion(ctx, i, job)
}

type mockPodLogReader struct {
	MockGetPodLogReader func(string, string) (io.ReadCloser, error)
}

func (m *mockPodLogReader) getPodLogReader(namespace, name string) (io.ReadCloser, error) {
	return m.MockGetPodLogReader(namespace, name)
}

type mockExecutorInfoDiscoverer struct {
	MockDiscoverExecutorInfo func(ctx context.Context) (*executorInfo, error)
}

func (m *mockExecutorInfoDiscoverer) discoverExecutorInfo(ctx context.Context) (*executorInfo, error) {
	return m.MockDiscoverExecutorInfo(ctx)
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

// ************************************************************************************************
// TestReconcile
// ************************************************************************************************
func TestReconcile(t *testing.T) {
	type want struct {
		result reconcile.Result
		err    error
	}

	tests := []struct {
		name string
		req  reconcile.Request
		rec  *Reconciler
		want want
	}{
		{
			name: "SuccessfulSync",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.ExtensionRequest) = *(resource())
						return nil
					},
				},
				executorInfoDiscovery: &mockExecutorInfoDiscoverer{
					MockDiscoverExecutorInfo: func(ctx context.Context) (*executorInfo, error) {
						return &executorInfo{image: extensionPackageImage}, nil
					},
				},
				factory: &mockFactory{
					MockNewHandler: func(context.Context, *v1alpha1.ExtensionRequest, client.Client, kubernetes.Interface, executorInfo) handler {
						return &mockHandler{
							MockSync: func(context.Context) (reconcile.Result, error) {
								return reconcile.Result{}, nil
							},
						}
					},
				},
			},
			want: want{result: reconcile.Result{}, err: nil},
		},
		{
			name: "DiscoverExecutorInfoFailed",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.ExtensionRequest) = *(resource())
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
				executorInfoDiscovery: &mockExecutorInfoDiscoverer{
					MockDiscoverExecutorInfo: func(ctx context.Context) (*executorInfo, error) {
						return nil, errors.New("test-discover-executorInfo-error")
					},
				},
				factory: nil,
			},
			want: want{result: resultRequeue, err: nil},
		},
		{
			name: "ResourceNotFound",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{Group: v1alpha1.Group}, key.Name)
					},
				},
				executorInfoDiscovery: nil,
				factory:               nil,
			},
			want: want{result: reconcile.Result{}, err: nil},
		},
		{
			name: "ResourceGetError",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return errors.New("test-get-error")
					},
				},
				executorInfoDiscovery: nil,
				factory:               nil,
			},
			want: want{result: reconcile.Result{}, err: errors.New("test-get-error")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, gotErr := tt.rec.Reconcile(tt.req)

			if diff := cmp.Diff(tt.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("Reconcile() -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.result, gotResult); diff != "" {
				t.Errorf("Reconcile() -want, +got:\n%v", diff)
			}
		})
	}
}

// ************************************************************************************************
// TestHandlerFactory
// ************************************************************************************************
func TestHandlerFactory(t *testing.T) {
	tests := []struct {
		name    string
		factory factory
		want    handler
	}{
		{
			name:    "SimpleCreate",
			factory: &handlerFactory{},
			want: &extensionRequestHandler{
				kube:         nil,
				jobCompleter: &extensionRequestJobCompleter{kube: nil, podLogReader: &k8sPodLogReader{kubeclient: nil}},
				executorInfo: executorInfo{image: extensionPackageImage},
				ext:          resource(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.factory.newHandler(ctx, resource(), nil, nil, executorInfo{image: extensionPackageImage})

			diff := cmp.Diff(tt.want, got,
				cmp.AllowUnexported(
					extensionRequestHandler{},
					extensionRequestJobCompleter{},
					k8sPodLogReader{},
					executorInfo{},
				))
			if diff != "" {
				t.Errorf("newHandler() -want, +got:\n%v", diff)
			}
		})
	}
}

// ************************************************************************************************
// TestCreate
// ************************************************************************************************
func TestCreate(t *testing.T) {
	type want struct {
		result reconcile.Result
		err    error
		ext    *v1alpha1.ExtensionRequest
	}

	tests := []struct {
		name    string
		handler *extensionRequestHandler
		want    want
	}{
		{
			name: "CreateInstallJob",
			handler: &extensionRequestHandler{
				kube: &test.MockClient{
					MockCreate:       func(ctx context.Context, obj runtime.Object) error { return nil },
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
				executorInfo: executorInfo{image: extensionPackageImage},
				ext:          resource(),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: resource(
					withConditions(corev1alpha1.Creating(), corev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace}),
				),
			},
		},
		{
			name: "InstallJobNotCompleted",
			handler: &extensionRequestHandler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET Job returns an uncompleted job
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
				ext: resource(
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: resource(
					withConditions(corev1alpha1.Creating(), corev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace}),
				),
			},
		},
		{
			name: "HandleSuccessfulInstallJob",
			handler: &extensionRequestHandler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET Job returns a successful/completed job
						*obj.(*batchv1.Job) = *(job(withJobConditions(batchv1.JobComplete, "")))
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
				jobCompleter: &mockJobCompleter{
					MockHandleJobCompletion: func(ctx context.Context, i *v1alpha1.ExtensionRequest, job *batchv1.Job) error { return nil },
				},
				executorInfo: executorInfo{image: extensionPackageImage},
				ext: resource(
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: resource(
					withConditions(corev1alpha1.Available(), corev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace}),
				),
			},
		},
		{
			name: "HandleFailedInstallJob",
			handler: &extensionRequestHandler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET Job returns a failed job
						*obj.(*batchv1.Job) = *(job(withJobConditions(batchv1.JobFailed, "mock job failure message")))
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
				jobCompleter: &mockJobCompleter{
					MockHandleJobCompletion: func(ctx context.Context, i *v1alpha1.ExtensionRequest, job *batchv1.Job) error { return nil },
				},
				executorInfo: executorInfo{image: extensionPackageImage},
				ext: resource(
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				ext: resource(
					withConditions(
						corev1alpha1.Creating(),
						corev1alpha1.ReconcileError(errors.New("mock job failure message")),
					),
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace}),
				),
			},
		},
	}

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

// ************************************************************************************************
// TestHandleJobCompletion
// ************************************************************************************************
func TestHandleJobCompletion(t *testing.T) {
	errBoom := errors.New("boom")

	type want struct {
		ext *v1alpha1.ExtensionRequest
		err error
	}

	tests := []struct {
		name string
		jc   *extensionRequestJobCompleter
		ext  *v1alpha1.ExtensionRequest
		job  *batchv1.Job
		want want
	}{
		{
			name: "NoPodsFoundForJob",
			jc: &extensionRequestJobCompleter{
				kube: &test.MockClient{
					MockList: func(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
						// LIST pods returns an empty list
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
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
			jc: &extensionRequestJobCompleter{
				kube: &test.MockClient{
					MockList: func(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
						// LIST pods returns a pod for the job
						*list.(*corev1.PodList) = corev1.PodList{
							Items: []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: jobPodName}}},
						}
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
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
			jc: &extensionRequestJobCompleter{
				kube: &test.MockClient{
					MockList: func(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
						// LIST pods returns a pod for the job
						*list.(*corev1.PodList) = corev1.PodList{
							Items: []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: jobPodName}}},
						}
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
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
			jc: &extensionRequestJobCompleter{
				kube: &test.MockClient{
					MockList: func(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
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
			jc: &extensionRequestJobCompleter{
				kube: &test.MockClient{
					MockList: func(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
						// LIST pods returns a pod for the job
						*list.(*corev1.PodList) = corev1.PodList{
							Items: []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: jobPodName}}},
						}
						return nil
					},
					MockCreate: func(ctx context.Context, obj runtime.Object) error { return nil },
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET extension returns the extension instance that was created from the pod log output
						*obj.(*v1alpha1.Extension) = v1alpha1.Extension{
							ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: namespace},
						}
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
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
				ext: resource(withExtensionRecord(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
				err: nil,
			},
		},
	}

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

// ************************************************************************************************
// TestDiscoverExecutorInfo
// ************************************************************************************************
func TestDiscoverExecutorInfo(t *testing.T) {
	type want struct {
		ei  *executorInfo
		err error
	}

	tests := []struct {
		name string
		d    *executorInfoDiscoverer
		want want
	}{
		{
			name: "FailedGetRunningPod",
			d: &executorInfoDiscoverer{
				kube: &test.MockClient{
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
			d: &executorInfoDiscoverer{
				kube: &test.MockClient{
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
			d: &executorInfoDiscoverer{
				kube: &test.MockClient{
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
				ei:  &executorInfo{image: "foo-image"},
				err: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialEnvVars := saveEnvVars()
			defer restoreEnvVars(initialEnvVars)

			os.Setenv(util.PodNameEnvVar, "podName")
			os.Setenv(util.PodNamespaceEnvVar, "podNamespace")

			got, gotErr := tt.d.discoverExecutorInfo(ctx)

			if diff := cmp.Diff(tt.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("discoverExecutorInfo() -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.ei, got, cmp.AllowUnexported(executorInfo{})); diff != "" {
				t.Errorf("discoverExecutorInfo() -want, +got:\n%v", diff)
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

// ************************************************************************************************
// TestGetPackageImage
// ************************************************************************************************
func TestGetPackageImage(t *testing.T) {
	tests := []struct {
		name string
		spec v1alpha1.ExtensionRequestSpec
		want string
	}{
		{
			name: "NoPackageSource",
			spec: v1alpha1.ExtensionRequestSpec{
				Package: "cool/package:rad",
			},
			want: "cool/package:rad",
		},
		{
			name: "PackageSourceSpecified",
			spec: v1alpha1.ExtensionRequestSpec{
				Source:  "registry.hub.docker.com",
				Package: "cool/package:rad",
			},
			want: "registry.hub.docker.com/cool/package:rad",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPackageImage(tt.spec)

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("getPackageImage() -want, +got:\n%v", diff)
			}
		})
	}
}
