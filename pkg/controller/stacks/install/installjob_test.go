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
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/stacks/v1alpha1"
	"github.com/crossplane/crossplane/pkg/controller/stacks/hosted"
	"github.com/crossplane/crossplane/pkg/stacks"
)

const (
	jobPodName            = "job-pod-123"
	podLogOutputMalformed = `)(&not valid yaml?()!`
	podLogOutput          = crdRaw + "\n" + stackRaw
	stackInstallSource    = "example.host"
	crdName               = "mytypes.samples.upbound.io"

	crdRaw = `---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: ` + crdName + `
spec:
  group: samples.upbound.io
  names:
    kind: Mytype
    listKind: MytypeList
    plural: mytypes
    singular: mytype
  scope: Namespaced
  version: v1alpha1
`
	stackDefinitionRaw = `---
apiVersion: stacks.crossplane.io/v1alpha1
kind: StackDefinition
metadata:
  creationTimestamp: null
spec:
  behavior:
    crd:
      apiVersion: samples.upbound.io/v1alpha1
      kind: Mytype
    engine:
      type: helm3
    source:
      image: crossplane/sample-stack:latest
      path: helm-chart
  company: Upbound
  controller:
    deployment:
      name: crossplane-sample-stack
      spec:
        selector: {}
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
          spec:
            containers:
            - args:
              - --resources-dir
              - /behaviors
              - --stack-definition-namespace
              - $(SD_NAMESPACE)
              - --stack-definition-name
              - $(SD_NAME)
              command:
              - /manager
              image: crossplane/templating-controller:v0.2.1
              name: stack-behavior-manager
              resources: {}
              volumeMounts:
              - mountPath: /behaviors
                name: behaviors
            initContainers:
            - command:
              - cp
              - -R
              - helm-chart/.
              - /behaviors
              image: crossplane/sample-stack-wordpress:0.1.0
              name: stack-behavior-copy-to-manager
              resources: {}
              volumeMounts:
              - mountPath: /behaviors
                name: behaviors
            restartPolicy: Always
            volumes:
            - emptyDir: {}
              name: behaviors
  customresourcedefinitions:
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Mytype
  overview: |
    Markdown describing this sample Crossplane stack project.
  icons:
  - base64Data: bW9jay1pY29uLWRh
    mediatype: image/jpeg
  keywords:
  - samples
  - examples
  - tutorials
  license: Apache-2.0
  maintainers:
  - email: jared@upbound.io
    name: Jared Watts
  owners:
  - email: bassam@upbound.io
    name: Bassam Tabbara
  permissionScope: Namespaced
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
  readme: |-
    ### Readme
  source: https://github.com/crossplane/sample-stack
  title: Sample Crossplane Stack
  version: 0.0.1
  website: https://upbound.io
status: {}
`

	stackRaw = `---
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
            creationTimestamp: null
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
              resources: {}
  customresourcedefinitions:
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Mytype
  overview: |
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
  source: https://github.com/crossplane/sample-stack
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
 conditionedStatus: {}
`

	// TODO(displague) use crossplane-runtime.pavement to set the
	// container envs on stackDefinitionRaw instead of copying it here
	expectedStackDefinitionRaw = `---
apiVersion: stacks.crossplane.io/v1alpha1
kind: StackDefinition
metadata:
  creationTimestamp: null
spec:
  behavior:
    crd:
      apiVersion: samples.upbound.io/v1alpha1
      kind: Mytype
    engine:
      type: helm3
    source:
      image: crossplane/sample-stack:latest
      path: helm-chart
  company: Upbound
  controller:
    deployment:
      name: crossplane-sample-stack
      spec:
        selector: {}
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
          spec:
            containers:
            - args:
              - --resources-dir
              - /behaviors
              - --stack-definition-namespace
              - $(SD_NAMESPACE)
              - --stack-definition-name
              - $(SD_NAME)
              command:
              - /manager
              env:
                - name: SD_NAMESPACE
                  value: cool-namespace
                - name: SD_NAME
                  value: cool-stackinstall
              image: crossplane/templating-controller:v0.2.1
              name: stack-behavior-manager
              resources: {}
              volumeMounts:
              - mountPath: /behaviors
                name: behaviors
            initContainers:
            - command:
              - cp
              - -R
              - helm-chart/.
              - /behaviors
              image: crossplane/sample-stack-wordpress:0.1.0
              name: stack-behavior-copy-to-manager
              resources: {}
              volumeMounts:
              - mountPath: /behaviors
                name: behaviors
            restartPolicy: Always
            volumes:
            - emptyDir: {}
              name: behaviors
  customresourcedefinitions:
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Mytype
  overview: |
    Markdown describing this sample Crossplane stack project.
  icons:
  - base64Data: bW9jay1pY29uLWRh
    mediatype: image/jpeg
  keywords:
  - samples
  - examples
  - tutorials
  license: Apache-2.0
  maintainers:
  - email: jared@upbound.io
    name: Jared Watts
  owners:
  - email: bassam@upbound.io
    name: Bassam Tabbara
  permissionScope: Namespaced
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
  readme: |-
    ### Readme
  source: https://github.com/crossplane/sample-stack
  title: Sample Crossplane Stack
  version: 0.0.1
  website: https://upbound.io
status: {}
`
)

var (
	_ jobCompleter = &stackInstallJobCompleter{}
)

func nsLabel(ns string) string {
	return fmt.Sprintf(stacks.LabelNamespaceFmt, ns)
}

// Job modifiers
type jobModifier func(*batchv1.Job)

func withJobName(nn types.NamespacedName) jobModifier {
	return func(j *batchv1.Job) {
		j.SetNamespace(nn.Namespace)
		j.SetName(nn.Name)
	}
}

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

func withJobSource(src string) jobModifier {
	return func(j *batchv1.Job) {
		for _, c := range j.Spec.Template.Spec.InitContainers {
			c.Image = src + "/" + c.Image
		}
		for _, c := range j.Spec.Template.Spec.Containers {
			c.Image = src + "/" + c.Image
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

// unstructuredObj modifiers
type unstructuredObjModifier func(*unstructured.Unstructured)

// withUnstructuredObjLabels modifies an existing unstructured object with the given labels
func withUnstructuredObjLabels(labels map[string]string) unstructuredObjModifier {
	return func(u *unstructured.Unstructured) {
		meta.AddLabels(u, labels)
	}
}

// withUnstructuredObjLabels modifies an existing unstructured object with the given labels
func withUnstructuredObjNamespacedName(nn types.NamespacedName) unstructuredObjModifier {
	return func(u *unstructured.Unstructured) {
		u.SetNamespace(nn.Namespace)
		u.SetName(nn.Name)
	}
}

// unstructuredObj creates a new default unstructured object (derived from yaml)
func unstructuredObj(raw string, uom ...unstructuredObjModifier) *unstructured.Unstructured {
	r := strings.NewReader(raw)
	d := yaml.NewYAMLOrJSONDecoder(r, 4096)
	obj := &unstructured.Unstructured{}
	d.Decode(&obj)

	for _, m := range uom {
		m(obj)
	}

	return obj
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
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostClient: &test.MockClient{
					MockList: func(ctx context.Context, list runtime.Object, _ ...client.ListOption) error {
						// LIST pods returns an empty list
						return nil
					},
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
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostClient: &test.MockClient{
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
						return nil, errBoom
					},
				},
				log: logging.NewNopLogger(),
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
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostClient: &test.MockClient{
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
						return &mockReadCloser{
							MockRead: func(p []byte) (n int, err error) {
								return 0, errBoom
							},
							MockClose: func() error { return nil },
						}, nil
					},
				},
				log: logging.NewNopLogger(),
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
				hostClient: &test.MockClient{
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
				log: logging.NewNopLogger(),
			},
			ext: resource(),
			job: job(),
			want: want{
				ext: resource(),
				err: errors.WithStack(errors.Errorf("failed to parse output from job %s: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}", resourceName)),
			},
		},
		{
			name: "FailToCreate",
			jc: &stackInstallJobCompleter{
				client: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						return errBoom
					},
				},
				podLogReader: &mockPodLogReader{
					MockGetPodLogReader: func(string, string) (io.ReadCloser, error) {
						return ioutil.NopCloser(bytes.NewReader([]byte(podLogOutput))), nil
					},
				},
				hostClient: &test.MockClient{
					MockList: func(ctx context.Context, list runtime.Object, _ ...client.ListOption) error {
						// LIST pods returns a pod for the job
						*list.(*corev1.PodList) = corev1.PodList{
							Items: []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: jobPodName}}},
						}
						return nil
					},
				},
				log: logging.NewNopLogger(),
			},
			ext: resource(),
			job: job(),
			want: want{
				ext: resource(),
				err: errors.Wrapf(errBoom, "failed to create object %s from job output %s", crdName, resourceName),
			},
		},
		{
			name: "HandleJobCompletionSuccess",
			jc: &stackInstallJobCompleter{
				client: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						return nil
					},
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET stack returns the stack instance that was created from the pod log output
						*obj.(*v1alpha1.Stack) = v1alpha1.Stack{
							ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: namespace},
						}
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostClient: &test.MockClient{
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
						return ioutil.NopCloser(bytes.NewReader([]byte(podLogOutput))), nil
					},
				},
				log: logging.NewNopLogger(),
			},
			ext: resource(),
			job: job(),
			want: want{
				ext: resource(),
				err: nil,
			},
		},
		{
			name: "HandleJobCompletionWithSource",
			jc: &stackInstallJobCompleter{
				client: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						if u, ok := obj.(*unstructured.Unstructured); ok {
							if isStackObject(u) {
								s, err := convertToStack(u)
								if err != nil {
									return err
								}
								d := s.Spec.Controller.Deployment
								if d == nil {
									return errors.New("expected Stack controller deployment")
								}
								if len(d.Spec.Template.Spec.Containers) == 0 {
									return errors.New("expected Stack controller deployment containers")
								}
								for _, c := range d.Spec.Template.Spec.Containers {
									if strings.Index(c.Image, stackInstallSource) != 0 {
										return errors.New("expected Stack controller deployment containers to start with stackinstall source")
									}
								}
							}
						}
						return nil
					},
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET stack returns the stack instance that was created from the pod log output
						*obj.(*v1alpha1.Stack) = v1alpha1.Stack{
							ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: namespace},
						}
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostClient: &test.MockClient{
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
						return ioutil.NopCloser(bytes.NewReader([]byte(podLogOutput))), nil
					},
				},
				log: logging.NewNopLogger(),
			},
			ext: resource(withSource(stackInstallSource)),
			job: job(withJobSource(stackInstallSource)),
			want: want{
				ext: resource(withSource(stackInstallSource)),
				err: nil,
			},
		},
		{
			name: "HandleJobCompletionWithPullPolicy",
			jc: &stackInstallJobCompleter{
				client: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						if u, ok := obj.(*unstructured.Unstructured); ok {
							if isStackObject(u) {
								s, err := convertToStack(u)
								if err != nil {
									return err
								}
								d := s.Spec.Controller.Deployment
								if d == nil {
									return errors.New("expected Stack controller deployment")
								}
								if len(d.Spec.Template.Spec.Containers) == 0 {
									return errors.New("expected Stack controller deployment containers")
								}
								for _, c := range d.Spec.Template.Spec.Containers {
									if c.ImagePullPolicy != corev1.PullAlways {
										return errors.New("expected Stack controller deployment containers to have Always pull policy")
									}
								}
							}
						}
						return nil
					},
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET stack returns the stack instance that was created from the pod log output
						*obj.(*v1alpha1.Stack) = v1alpha1.Stack{
							ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: namespace},
						}
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostClient: &test.MockClient{
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
						return ioutil.NopCloser(bytes.NewReader([]byte(podLogOutput))), nil
					},
				},
				log: logging.NewNopLogger(),
			},
			ext: resource(
				withSource(stackInstallSource),
				withImagePullPolicy(corev1.PullAlways),
				withImagePullSecrets([]corev1.LocalObjectReference{{Name: "foo"}}),
			),
			job: job(withJobSource(stackInstallSource)),
			want: want{
				ext: resource(
					withSource(stackInstallSource),
					withImagePullPolicy(corev1.PullAlways),
					withImagePullSecrets([]corev1.LocalObjectReference{{Name: "foo"}}),
				),
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

	noJobs := func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
		return kerrors.NewNotFound(schema.GroupResource{Group: "batch", Resource: "Job"}, key.String())
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
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostKube: &test.MockClient{
					MockGet:    noJobs,
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error { return nil },
				},
				executorInfo: &stacks.ExecutorInfo{Image: stackPackageImage},
				ext:          resource(),
				log:          logging.NewNopLogger(),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: resource(
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace}),
				),
			},
		},
		{
			name: "CreateInstallJobHosted",
			handler: &stackInstallHandler{
				kube: &test.MockClient{

					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostKube: &test.MockClient{
					MockGet:    noJobs,
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error { return nil },
				},
				hostAwareConfig: &hosted.Config{HostControllerNamespace: hostControllerNamespace},
				executorInfo:    &stacks.ExecutorInfo{Image: stackPackageImage},
				ext:             resource(),
				log:             logging.NewNopLogger(),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: resource(
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{Name: fmt.Sprintf("%s.%s", namespace, resourceName), Namespace: hostControllerNamespace}),
				),
			},
		},
		{
			name: "ExistingInstallJobHosted",
			handler: &stackInstallHandler{
				kube: &test.MockClient{
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*batchv1.Job) = *(job(withJobName(types.NamespacedName{Name: fmt.Sprintf("%s.%s", namespace, resourceName), Namespace: hostControllerNamespace})))
						return nil
					},
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error { return nil },
				},
				hostAwareConfig: &hosted.Config{HostControllerNamespace: hostControllerNamespace},
				executorInfo:    &stacks.ExecutorInfo{Image: stackPackageImage},
				ext:             resource(),
				log:             logging.NewNopLogger(),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: resource(
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{Name: fmt.Sprintf("%s.%s", namespace, resourceName), Namespace: hostControllerNamespace}),
				),
			},
		},
		{
			name: "HandleFailedInstallJobHosted",
			handler: &stackInstallHandler{
				kube: &test.MockClient{
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostKube: &test.MockClient{
					MockGet:    noJobs,
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error { return errBoom },
				},
				hostAwareConfig: &hosted.Config{HostControllerNamespace: hostControllerNamespace},
				executorInfo:    &stacks.ExecutorInfo{Image: stackPackageImage},
				ext:             resource(),
				log:             logging.NewNopLogger(),
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				ext: resource(
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileError(errBoom)),
					withInstallJob(nil),
				),
			},
		},
		{
			name: "FailedToGetInstallJob",
			handler: &stackInstallHandler{
				kube: &test.MockClient{
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return errBoom
					},
				},
				ext: resource(
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
				log: logging.NewNopLogger(),
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				ext: resource(
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileError(errBoom)),
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace}),
				),
			},
		},
		{
			name: "InstallJobNotCompleted",
			handler: &stackInstallHandler{
				kube: &test.MockClient{
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET Job returns an uncompleted job
						return nil
					},
				},
				ext: resource(
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
				log: logging.NewNopLogger(),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: resource(
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace}),
				),
			},
		},
		{
			name: "HandleSuccessfulInstallJob",
			handler: &stackInstallHandler{
				kube: &test.MockClient{
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET Job returns a successful/completed job
						*obj.(*batchv1.Job) = *(job(withJobConditions(batchv1.JobComplete, "")))
						return nil
					},
				},
				jobCompleter: &mockJobCompleter{
					MockHandleJobCompletion: func(ctx context.Context, i v1alpha1.StackInstaller, job *batchv1.Job) error { return nil },
				},
				executorInfo: &stacks.ExecutorInfo{Image: stackPackageImage},
				ext: resource(
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
				log: logging.NewNopLogger(),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: resource(
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace}),
				),
			},
		},
		{
			name: "HandleFailedInstallJob",
			handler: &stackInstallHandler{
				kube: &test.MockClient{
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET Job returns a failed job
						*obj.(*batchv1.Job) = *(job(withJobConditions(batchv1.JobFailed, "mock job failure message")))
						return nil
					},
				},
				jobCompleter: &mockJobCompleter{
					MockHandleJobCompletion: func(ctx context.Context, i v1alpha1.StackInstaller, job *batchv1.Job) error { return nil },
				},
				executorInfo: &stacks.ExecutorInfo{Image: stackPackageImage},
				ext: resource(
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
				log: logging.NewNopLogger(),
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				ext: resource(
					withFinalizers(installFinalizer),
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

func TestCreateJobOutputObject(t *testing.T) {
	wantedParentLabels := map[string]string{
		stacks.LabelParentGroup:     "stacks.crossplane.io",
		stacks.LabelParentVersion:   "v1alpha1",
		stacks.LabelParentKind:      "StackInstall",
		stacks.LabelParentNamespace: namespace,
		stacks.LabelParentName:      resourceName,
		stacks.LabelParentUID:       uidString,
	}

	type want struct {
		err error
		obj *unstructured.Unstructured
	}

	tests := []struct {
		name           string
		jobCompleter   *stackInstallJobCompleter
		stackInstaller *v1alpha1.StackInstall
		job            *batchv1.Job
		obj            *unstructured.Unstructured
		want           want
	}{
		{
			name: "NilObj",
			jobCompleter: &stackInstallJobCompleter{
				client: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error { return nil },
				},
				log: logging.NewNopLogger(),
			},
			stackInstaller: resource(),
			job:            job(),
			obj:            nil,
			want: want{
				err: nil,
				obj: nil,
			},
		},
		{
			name: "CreateError",
			jobCompleter: &stackInstallJobCompleter{
				client: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						return errBoom
					},
				},
				log: logging.NewNopLogger(),
			},
			stackInstaller: resource(),
			job:            job(),
			obj: unstructuredObj(crdRaw,
				withUnstructuredObjLabels(wantedParentLabels),
			),
			want: want{
				err: errors.Wrapf(errBoom, "failed to create object %s from job output %s", crdName, resourceName),
				obj: unstructuredObj(crdRaw,
					withUnstructuredObjLabels(wantedParentLabels),
				),
			},
		},
		{
			name: "CreateSuccess",
			jobCompleter: &stackInstallJobCompleter{
				client: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error { return nil },
				},
				log: logging.NewNopLogger(),
			},
			stackInstaller: resource(),
			job:            job(),
			obj:            unstructuredObj(crdRaw),
			want: want{
				err: nil,
				obj: unstructuredObj(crdRaw,
					withUnstructuredObjLabels(wantedParentLabels),
				),
			},
		},
		{
			name: "CreateSuccessfulStack",
			jobCompleter: &stackInstallJobCompleter{
				client: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error { return nil },
				},
				log: logging.NewNopLogger(),
			},
			stackInstaller: resource(),
			job:            job(),
			obj:            unstructuredObj(stackRaw),
			want: want{
				err: nil,
				obj: unstructuredObj(stackRaw,
					withUnstructuredObjLabels(wantedParentLabels),
					withUnstructuredObjNamespacedName(types.NamespacedName{Namespace: namespace, Name: resourceName}),
				),
			},
		},
		{
			name: "CreateSuccessfulStackDefinition",
			jobCompleter: &stackInstallJobCompleter{
				client: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error { return nil },
				},
				log: logging.NewNopLogger(),
			},
			stackInstaller: resource(),
			job:            job(),
			obj:            unstructuredObj(stackDefinitionRaw),
			want: want{
				err: nil,
				obj: unstructuredObj(expectedStackDefinitionRaw,
					withUnstructuredObjLabels(wantedParentLabels),
					withUnstructuredObjNamespacedName(types.NamespacedName{Namespace: namespace, Name: resourceName}),
				),
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := tt.jobCompleter.createJobOutputObject(ctx, tt.obj, tt.stackInstaller, tt.job)

			if diff := cmp.Diff(tt.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("createJobOutputObject(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.obj, tt.obj, test.EquateConditions()); diff != "" {
				t.Errorf("createJobOutputObject(): -want obj, +got obj:\n%v", diff)
			}
		})
	}
}
