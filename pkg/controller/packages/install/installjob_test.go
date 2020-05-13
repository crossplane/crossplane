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
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/packages/v1alpha1"
	"github.com/crossplane/crossplane/pkg/controller/packages/hosted"
	"github.com/crossplane/crossplane/pkg/packages"
)

const (
	jobPodName                   = "job-pod-123"
	podLogOutputMalformed        = `)(&not valid yaml?()!`
	packageInstallSource         = "example.host"
	packageEnvelopeImage         = "crossplane/sample-package:latest"
	stackDefinitionEnvelopeImage = "crossplane/app-wordpress:0.1.0"
	crdName                      = "mytypes.samples.upbound.io"

	crdRaw = `---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: ` + crdName + `
spec:
  group: samples.upbound.io
  conversion:
    strategy: None
  preserveUnknownFields: true
  names:
    kind: Mytype
    listKind: MytypeList
    plural: mytypes
    singular: mytype
  scope: Namespaced
  version: v1alpha1
`

	expectedStackDefinitionRaw = `---
apiVersion: packages.crossplane.io/v1alpha1
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
      image: crossplane/sample-package:latest
      path: helm-chart
  company: Upbound
  controller:
    deployment:
      name: crossplane-sample-package
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
                  value: cool-packageinstall
              image: crossplane/templating-controller:v0.2.1
              name: package-behavior-manager
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
              image: crossplane/app-wordpress:0.1.0
              name: package-behavior-copy-to-manager
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
    Markdown describing this sample Crossplane package project.
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
  source: https://github.com/crossplane/sample-package
  title: Sample Crossplane Stack
  version: 0.0.1
  website: https://upbound.io
status: {}
`
)

var (
	_            jobCompleter = &packageInstallJobCompleter{}
	podLogOutput              = crdRaw + "\n" + packageRaw("crossplane/sample-package:latest")
)

func packageRaw(controllerImage string) string {
	tmpl := `---
apiVersion: packages.crossplane.io/v1alpha1
kind: Package
metadata:
  creationTimestamp: null
spec:
  company: Upbound
  controller:
    deployment:
      name: crossplane-sample-package
      spec:
        replicas: 1
        selector:
          matchLabels:
            core.crossplane.io/name: crossplane-sample-package
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              core.crossplane.io/name: crossplane-sample-package
            name: sample-package-controller
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
              %sname: sample-package-controller
              resources: {}
  customresourcedefinitions:
  - apiVersion: samples.upbound.io/v1alpha1
    kind: Mytype
  overview: |
    Markdown describing this sample Crossplane package project.
  icons:
  - base64Data: bW9jay1pY29uLWRh
    mediatype: image/jpeg
  keywords:
  - samples
  - examples
  - tutorials
  license: Apache-2.0
  website: https://upbound.io
  source: https://github.com/crossplane/sample-package
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
	if controllerImage != "" {
		// The spaces are used for formatting the next line. This is a quick and dirty way
		// to optionally insert an additional line into the output.
		return fmt.Sprintf(tmpl, fmt.Sprintf("image: %s\n              ", controllerImage))
	}

	return fmt.Sprintf(tmpl, "")
}

func stackDefinitionRaw(controllerImage string) string {
	tmpl := `---
apiVersion: packages.crossplane.io/v1alpha1
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
      image: crossplane/sample-package:latest
      path: helm-chart
  company: Upbound
  controller:
    deployment:
      name: crossplane-sample-package
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
              name: package-behavior-manager
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
              %sname: package-behavior-copy-to-manager
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
    Markdown describing this sample Crossplane package project.
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
  source: https://github.com/crossplane/sample-package
  title: Sample Crossplane Stack
  version: 0.0.1
  website: https://upbound.io
status: {}
`
	if controllerImage != "" {
		// The spaces are used for formatting the next line. This is a quick and dirty way
		// to optionally insert an additional line into the output.
		return fmt.Sprintf(tmpl, fmt.Sprintf("image: %s\n              ", controllerImage))
	}

	return fmt.Sprintf(tmpl, "")
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

// withJobPullPolicy sets the image pull policy on the InitContainer, whose
// container is derived from the package. The non-init containers are not affected
// since they reflect the imagePullPolicy of the package-manager itself
func withJobPullPolicy(pullPolicy corev1.PullPolicy) jobModifier {
	return func(j *batchv1.Job) {
		ics := j.Spec.Template.Spec.InitContainers
		for i := range ics {
			ics[i].ImagePullPolicy = pullPolicy
		}
	}
}

// withJobExpectations sets default job expectations
func withJobExpectations() jobModifier {
	return func(j *batchv1.Job) {
		zero := int32(0)
		j.SetResourceVersion("1")
		j.SetGroupVersionKind(batchv1.SchemeGroupVersion.WithKind("Job"))
		j.SetLabels(packages.ParentLabels(packageInstallResource()))
		j.Spec.BackoffLimit = &zero
		j.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name:         "package-contents",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}}
		j.Spec.Template.Spec.InitContainers = []corev1.Container{{
			Name:         "package-copy-to-volume",
			Command:      []string{"cp", "-R", "/.registry", "/ext-pkg/"},
			VolumeMounts: []corev1.VolumeMount{{Name: "package-contents", MountPath: "/ext-pkg"}},
		}}
		j.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:  "package-unpack-and-output",
				Image: packagePackageImage,
				Args: []string{
					"package",
					"unpack",
					"--content-dir=/ext-pkg/.registry",
					"--permission-scope=Namespaced",
					"--templating-controller-image=",
				},
				Env:          []corev1.EnvVar{{Name: "STACK_IMAGE"}},
				VolumeMounts: []corev1.VolumeMount{{Name: "package-contents", MountPath: "/ext-pkg"}},
			},
		}
		j.Spec.Template.Spec.RestartPolicy = "Never"
	}
}

func job(jm ...jobModifier) *batchv1.Job {
	j := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      resourceName,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
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

func unstructuredAsCRD(mods ...crdModifier) unstructuredObjModifier {
	return func(u *unstructured.Unstructured) {
		crd, err := convertToCRD(u)
		if err != nil {
			panic(err)
		}

		for _, m := range mods {
			m(crd)
		}
		o, err := runtime.DefaultUnstructuredConverter.ToUnstructured(crd)
		if err != nil {
			panic(err)
		}
		u.Object = o
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

func secret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}}
}

type mockJobCompleter struct {
	MockHandleJobCompletion func(ctx context.Context, i v1alpha1.PackageInstaller, job *batchv1.Job) error
}

func (m *mockJobCompleter) handleJobCompletion(ctx context.Context, i v1alpha1.PackageInstaller, job *batchv1.Job) error {
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
		ext *v1alpha1.PackageInstall
		err error
	}

	tests := []struct {
		name string
		jc   *packageInstallJobCompleter
		ext  *v1alpha1.PackageInstall
		job  *batchv1.Job
		want want
	}{
		{
			name: "NoPodsFoundForJob",
			jc: &packageInstallJobCompleter{
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
			ext: packageInstallResource(),
			job: job(),
			want: want{
				ext: packageInstallResource(),
				err: errors.Errorf("pod list for job %s should only have 1 item, actual: 0", resourceName),
			},
		},
		{
			name: "FailToGetJobPodLogs",
			jc: &packageInstallJobCompleter{
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
			ext: packageInstallResource(),
			job: job(),
			want: want{
				ext: packageInstallResource(),
				err: errors.Wrapf(errBoom, "failed to get logs request stream from pod %s", jobPodName),
			},
		},
		{
			name: "FailToReadJobPodLogStream",
			jc: &packageInstallJobCompleter{
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
			ext: packageInstallResource(),
			job: job(),
			want: want{
				ext: packageInstallResource(),
				err: errors.Wrapf(errBoom, "failed to copy logs request stream from pod %s", jobPodName),
			},
		},
		{
			name: "FailToParseJobPodLogOutput",
			jc: &packageInstallJobCompleter{
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
			ext: packageInstallResource(),
			job: job(),
			want: want{
				ext: packageInstallResource(),
				err: errors.WithStack(errors.Errorf("failed to parse output from job %s: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}", resourceName)),
			},
		},
		{
			name: "FailToCreate",
			jc: &packageInstallJobCompleter{
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
			ext: packageInstallResource(),
			job: job(),
			want: want{
				ext: packageInstallResource(),
				err: errors.Wrapf(errBoom, "failed to create object %s from job output %s", crdName, resourceName),
			},
		},
		{
			name: "HandleJobCompletionSuccess",
			jc: &packageInstallJobCompleter{
				client: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						return nil
					},
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET package returns the package instance that was created from the pod log output
						*obj.(*v1alpha1.Package) = v1alpha1.Package{
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
			ext: packageInstallResource(),
			job: job(),
			want: want{
				ext: packageInstallResource(),
				err: nil,
			},
		},
		{
			name: "HandleJobCompletionWithSource",
			jc: &packageInstallJobCompleter{
				client: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						if u, ok := obj.(*unstructured.Unstructured); ok {
							if isPackageObject(u) {
								s, err := convertToPackage(u)
								if err != nil {
									return err
								}
								d := s.Spec.Controller.Deployment
								if d == nil {
									return errors.New("expected Package controller deployment")
								}
								if len(d.Spec.Template.Spec.Containers) == 0 {
									return errors.New("expected Package controller deployment containers")
								}
								for _, c := range d.Spec.Template.Spec.Containers {
									if strings.Index(c.Image, packageInstallSource) != 0 {
										return errors.New("expected Package controller deployment containers to start with packageinstall source")
									}
								}
							}
						}
						return nil
					},
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET package returns the package instance that was created from the pod log output
						*obj.(*v1alpha1.Package) = v1alpha1.Package{
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
			ext: packageInstallResource(withSource(packageInstallSource)),
			job: job(withJobSource(packageInstallSource)),
			want: want{
				ext: packageInstallResource(withSource(packageInstallSource)),
				err: nil,
			},
		},
		{
			name: "HandleJobCompletionWithPullPolicy",
			jc: &packageInstallJobCompleter{
				client: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						if u, ok := obj.(*unstructured.Unstructured); ok {
							if isPackageObject(u) {
								s, err := convertToPackage(u)
								if err != nil {
									return err
								}
								d := s.Spec.Controller.Deployment
								if d == nil {
									return errors.New("expected Package controller deployment")
								}
								if len(d.Spec.Template.Spec.Containers) == 0 {
									return errors.New("expected Package controller deployment containers")
								}
								for _, c := range d.Spec.Template.Spec.Containers {
									if c.ImagePullPolicy != corev1.PullAlways {
										return errors.New("expected Package controller deployment containers to have Always pull policy")
									}
								}
							}
						}
						return nil
					},
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET package returns the package instance that was created from the pod log output
						*obj.(*v1alpha1.Package) = v1alpha1.Package{
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
			ext: packageInstallResource(
				withSource(packageInstallSource),
				withImagePullPolicy(corev1.PullAlways),
				withImagePullSecrets([]corev1.LocalObjectReference{{Name: "foo"}}),
			),
			job: job(withJobSource(packageInstallSource)),
			want: want{
				ext: packageInstallResource(
					withSource(packageInstallSource),
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
		ext    *v1alpha1.PackageInstall
		job    *batchv1.Job
	}

	noJobs := func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
		return kerrors.NewNotFound(schema.GroupResource{Group: "batch", Resource: "Job"}, key.String())
	}

	tests := []struct {
		name    string
		handler *packageInstallHandler
		want    want
	}{
		{
			name: "CreateInstallJob",
			handler: &packageInstallHandler{
				kube: &test.MockClient{
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
				},
				hostKube: &test.MockClient{
					MockGet:    noJobs,
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error { return nil },
				},
				executorInfo: &packages.ExecutorInfo{Image: packagePackageImage},
				ext:          packageInstallResource(),
				log:          logging.NewNopLogger(),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: packageInstallResource(
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{
						Name:       resourceName,
						Namespace:  namespace,
						Kind:       "Job",
						APIVersion: batchv1.SchemeGroupVersion.String(),
					}),
				),
			},
		},
		{
			name: "CreateInstallJobForcedPullPolicy",
			handler: &packageInstallHandler{
				forceImagePullPolicy: string(corev1.PullAlways),
				kube: &test.MockClient{
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
				},
				hostKube:     fake.NewFakeClient(),
				executorInfo: &packages.ExecutorInfo{Image: packagePackageImage},
				ext:          packageInstallResource(withImagePullPolicy(corev1.PullNever)),
				log:          logging.NewNopLogger(),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: packageInstallResource(
					withImagePullPolicy(corev1.PullNever),
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{
						Name:       resourceName,
						Namespace:  namespace,
						Kind:       "Job",
						APIVersion: batchv1.SchemeGroupVersion.String(),
					}),
				),
				job: job(
					withJobExpectations(),
					withJobPullPolicy(corev1.PullAlways),
				),
			},
		},
		{
			name: "CreateInstallJobHosted",
			handler: &packageInstallHandler{
				kube: &test.MockClient{

					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
				},
				hostKube: &test.MockClient{
					MockGet:    noJobs,
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error { return nil },
					MockList:   func(ctx context.Context, obj runtime.Object, _ ...client.ListOption) error { return nil },
				},
				hostAwareConfig: &hosted.Config{HostControllerNamespace: hostControllerNamespace},
				executorInfo:    &packages.ExecutorInfo{Image: packagePackageImage},
				ext:             packageInstallResource(),
				log:             logging.NewNopLogger(),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: packageInstallResource(
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{
						Name:       fmt.Sprintf("%s.%s", namespace, resourceName),
						Namespace:  hostControllerNamespace,
						Kind:       "Job",
						APIVersion: batchv1.SchemeGroupVersion.String(),
					}),
				),
			},
		},
		{
			name: "CreateInstallJobHostedWithPullSecrets",
			handler: &packageInstallHandler{
				kube: fake.NewFakeClient(
					secret("secret", namespace),
					packageInstallResource(withImagePullSecrets([]corev1.LocalObjectReference{{Name: "secret"}})),
				),
				hostKube:        fake.NewFakeClient(),
				hostAwareConfig: &hosted.Config{HostControllerNamespace: hostControllerNamespace},
				executorInfo:    &packages.ExecutorInfo{Image: packagePackageImage},
				ext:             packageInstallResource(withImagePullSecrets([]corev1.LocalObjectReference{{Name: "secret"}})),
				log:             logging.NewNopLogger(),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: packageInstallResource(
					withFinalizers(installFinalizer),
					withImagePullSecrets([]corev1.LocalObjectReference{{Name: "secret"}}),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{
						Name:       fmt.Sprintf("%s.%s", namespace, resourceName),
						Namespace:  hostControllerNamespace,
						Kind:       "Job",
						APIVersion: batchv1.SchemeGroupVersion.String(),
					}),
					withGVK(v1alpha1.PackageInstallGroupVersionKind),
					withResourceVersion("2"),
				),
			},
		},
		{
			name: "ExistingInstallJobHosted",
			handler: &packageInstallHandler{
				kube: &test.MockClient{
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						return nil
					},
				},
				hostKube: func() client.Client {
					c := fake.NewFakeClient(job(withJobName(types.NamespacedName{Name: fmt.Sprintf("%s.%s", namespace, resourceName), Namespace: hostControllerNamespace})))
					return &test.MockClient{
						MockList:   c.List,
						MockCreate: test.NewMockCreateFn(errBoom),
						MockGet:    c.Get,
					}
				}(),
				hostAwareConfig: &hosted.Config{HostControllerNamespace: hostControllerNamespace},
				executorInfo:    &packages.ExecutorInfo{Image: packagePackageImage},
				ext:             packageInstallResource(),
				log:             logging.NewNopLogger(),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: packageInstallResource(
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{
						Name:       fmt.Sprintf("%s.%s", namespace, resourceName),
						Namespace:  hostControllerNamespace,
						Kind:       "Job",
						APIVersion: batchv1.SchemeGroupVersion.String(),
					}),
				),
			},
		},
		{
			name: "HandleFailedInstallJobHosted",
			handler: &packageInstallHandler{
				kube: &test.MockClient{
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						return nil
					},
				},
				hostKube: &test.MockClient{
					MockGet:    noJobs,
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error { return errBoom },
				},
				hostAwareConfig: &hosted.Config{HostControllerNamespace: hostControllerNamespace},
				executorInfo:    &packages.ExecutorInfo{Image: packagePackageImage},
				ext:             packageInstallResource(),
				log:             logging.NewNopLogger(),
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				ext: packageInstallResource(
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileError(errBoom)),
					withInstallJob(nil),
				),
			},
		},
		{
			name: "FailedToGetInstallJob",
			handler: &packageInstallHandler{
				kube: &test.MockClient{
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						return nil
					},
				},
				hostKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return errBoom
					},
				},
				ext: packageInstallResource(
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
				log: logging.NewNopLogger(),
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				ext: packageInstallResource(
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileError(errBoom)),
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace}),
				),
			},
		},
		{
			name: "InstallJobNotCompleted",
			handler: &packageInstallHandler{
				kube: &test.MockClient{
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						return nil
					},
				},
				hostKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET Job returns an uncompleted job
						return nil
					},
				},
				ext: packageInstallResource(
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
				log: logging.NewNopLogger(),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: packageInstallResource(
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace}),
				),
			},
		},
		{
			name: "HandleSuccessfulInstallJob",
			handler: &packageInstallHandler{
				kube: &test.MockClient{
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						return nil
					},
				},
				hostKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET Job returns a successful/completed job
						*obj.(*batchv1.Job) = *(job(withJobConditions(batchv1.JobComplete, "")))
						return nil
					},
				},
				jobCompleter: &mockJobCompleter{
					MockHandleJobCompletion: func(ctx context.Context, i v1alpha1.PackageInstaller, job *batchv1.Job) error { return nil },
				},
				executorInfo: &packages.ExecutorInfo{Image: packagePackageImage},
				ext: packageInstallResource(
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
				log: logging.NewNopLogger(),
			},
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				ext: packageInstallResource(
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileSuccess()),
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace}),
				),
			},
		},
		{
			name: "HandleFailedInstallJob",
			handler: &packageInstallHandler{
				kube: &test.MockClient{
					MockPatch: func(_ context.Context, obj runtime.Object, patch client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockStatusUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						return nil
					},
				},
				hostKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						// GET Job returns a failed job
						*obj.(*batchv1.Job) = *(job(withJobConditions(batchv1.JobFailed, "mock job failure message")))
						return nil
					},
				},
				jobCompleter: &mockJobCompleter{
					MockHandleJobCompletion: func(ctx context.Context, i v1alpha1.PackageInstaller, job *batchv1.Job) error { return nil },
				},
				executorInfo: &packages.ExecutorInfo{Image: packagePackageImage},
				ext: packageInstallResource(
					withInstallJob(&corev1.ObjectReference{Name: resourceName, Namespace: namespace})),
				log: logging.NewNopLogger(),
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				ext: packageInstallResource(
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

			if tt.want.job != nil {
				gotJob := &batchv1.Job{}
				gotErr := tt.handler.hostKube.Get(context.TODO(), types.NamespacedName{Name: tt.want.job.GetName(), Namespace: tt.want.job.GetNamespace()}, gotJob)

				if diff := cmp.Diff(nil, gotErr, test.EquateErrors()); diff != "" {
					t.Errorf("create() -want job error, +got job error:\n%s", diff)
				}

				if diff := cmp.Diff(tt.want.job, gotJob); diff != "" {
					t.Errorf("create() -want job, +got job:\n%v", diff)
				}
			}

			if diff := cmp.Diff(tt.want.ext, tt.handler.ext, test.EquateConditions()); diff != "" {
				t.Errorf("create() -want, +got:\n%v", diff)
			}

		})
	}
}

func TestCreateJobOutputObject(t *testing.T) {
	wantedParentLabels := map[string]string{
		packages.LabelParentGroup:     "packages.crossplane.io",
		packages.LabelParentVersion:   "v1alpha1",
		packages.LabelParentKind:      "PackageInstall",
		packages.LabelParentNamespace: namespace,
		packages.LabelParentName:      resourceName,
	}

	type want struct {
		err error
		obj *unstructured.Unstructured
	}

	tests := []struct {
		name             string
		jobCompleter     *packageInstallJobCompleter
		packageInstaller *v1alpha1.PackageInstall
		job              *batchv1.Job
		obj              *unstructured.Unstructured
		want             want
	}{
		{
			name: "NilObj",
			jobCompleter: &packageInstallJobCompleter{
				client: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error { return nil },
				},
				log: logging.NewNopLogger(),
			},
			packageInstaller: packageInstallResource(),
			job:              job(),
			obj:              nil,
			want: want{
				err: nil,
				obj: nil,
			},
		},
		{
			name: "CreateError",
			jobCompleter: &packageInstallJobCompleter{
				client: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						return errBoom
					},
				},
				log: logging.NewNopLogger(),
			},
			packageInstaller: packageInstallResource(),
			job:              job(),
			obj: unstructuredObj(crdRaw,
				withUnstructuredObjLabels(wantedParentLabels),
			),
			want: want{
				err: errors.Wrapf(errBoom, "failed to create object %s from job output %s", crdName, resourceName),
				obj: nil,
			},
		},
		{
			name: "CreateSuccess",
			jobCompleter: &packageInstallJobCompleter{
				client: fake.NewFakeClient(),
				log:    logging.NewNopLogger(),
			},
			packageInstaller: packageInstallResource(),
			job:              job(),
			obj:              unstructuredObj(crdRaw),
			want: want{
				err: nil,
				obj: unstructuredObj(crdRaw),
			},
		},
		{
			name: "CreateSuccessfulPackage",
			jobCompleter: &packageInstallJobCompleter{
				client: fake.NewFakeClient(),
				log:    logging.NewNopLogger(),
			},
			packageInstaller: packageInstallResource(),
			job:              job(),
			obj:              unstructuredObj(packageRaw("crossplane/sample-package:latest")),
			want: want{
				err: nil,
				obj: unstructuredObj(packageRaw("crossplane/sample-package:latest"),
					withUnstructuredObjLabels(wantedParentLabels),
					withUnstructuredObjNamespacedName(types.NamespacedName{Namespace: namespace, Name: resourceName}),
				),
			},
		},
		{
			name: "CreateSuccessfulStackDefinition",
			jobCompleter: &packageInstallJobCompleter{
				client: fake.NewFakeClient(),
				log:    logging.NewNopLogger(),
			},
			packageInstaller: packageInstallResource(),
			job:              job(),
			obj:              unstructuredObj(stackDefinitionRaw("crossplane/app-wordpress:0.1.0")),
			want: want{
				err: nil,
				obj: unstructuredObj(expectedStackDefinitionRaw,
					withUnstructuredObjLabels(wantedParentLabels),
					withUnstructuredObjNamespacedName(types.NamespacedName{Namespace: namespace, Name: resourceName}),
				),
			},
		},
		{
			name: "CreateSuccessfulPackageWithInjectedControllerImage",
			jobCompleter: &packageInstallJobCompleter{
				client: fake.NewFakeClient(),
				log:    logging.NewNopLogger(),
			},
			packageInstaller: packageInstallResource(withPackage(packageEnvelopeImage)),
			job:              job(),
			obj:              unstructuredObj(packageRaw("")),
			want: want{
				err: nil,
				obj: unstructuredObj(packageRaw("crossplane/sample-package:latest"),
					withUnstructuredObjLabels(wantedParentLabels),
					withUnstructuredObjNamespacedName(types.NamespacedName{Namespace: namespace, Name: resourceName}),
				),
			},
		},
		{
			name: "CreateSuccessfulStackDefinitionWithInjectedControllerImage",
			jobCompleter: &packageInstallJobCompleter{
				client: fake.NewFakeClient(),
				log:    logging.NewNopLogger(),
			},
			packageInstaller: packageInstallResource(withPackage(stackDefinitionEnvelopeImage)),
			job:              job(),
			obj:              unstructuredObj(stackDefinitionRaw("")),
			want: want{
				err: nil,
				obj: unstructuredObj(expectedStackDefinitionRaw,
					withUnstructuredObjLabels(wantedParentLabels),
					withUnstructuredObjNamespacedName(types.NamespacedName{Namespace: namespace, Name: resourceName}),
				),
			},
		},
		{
			name: "CreateSuccessfulPackageWithDifferentControllerImage",
			jobCompleter: &packageInstallJobCompleter{
				client: fake.NewFakeClient(),
				log:    logging.NewNopLogger(),
			},
			packageInstaller: packageInstallResource(withPackage("thisImageShouldBeIgnored:becauseThePackageSpecifiesAnExplicitImage")),
			job:              job(),
			obj:              unstructuredObj(packageRaw("crossplane/sample-package:latest")),
			want: want{
				err: nil,
				obj: unstructuredObj(packageRaw("crossplane/sample-package:latest"),
					withUnstructuredObjLabels(wantedParentLabels),
					withUnstructuredObjNamespacedName(types.NamespacedName{Namespace: namespace, Name: resourceName}),
				),
			},
		},
		{
			name: "CreateSuccessfulStackDefinitionWithDifferentControllerImage",
			jobCompleter: &packageInstallJobCompleter{
				client: fake.NewFakeClient(),
				log:    logging.NewNopLogger(),
			},
			packageInstaller: packageInstallResource(withPackage("thisImageShouldBeIgnored:becauseTheStackSpecifiesAnExplicitImage")),
			job:              job(),
			obj:              unstructuredObj(stackDefinitionRaw("crossplane/app-wordpress:0.1.0")),
			want: want{
				err: nil,
				obj: unstructuredObj(expectedStackDefinitionRaw,
					withUnstructuredObjLabels(wantedParentLabels),
					withUnstructuredObjNamespacedName(types.NamespacedName{Namespace: namespace, Name: resourceName}),
				),
			},
		},
		{
			name: "FailedCantGetCRD",
			jobCompleter: &packageInstallJobCompleter{
				client: func() client.Client {
					crd := crd(withCRDGroupKind("samples.upbound.io", "Mytype"))
					fc := fake.NewFakeClient(&crd)
					return &test.MockClient{
						MockCreate: fc.Create,
						MockGet:    test.NewMockGetFn(errBoom),
						MockUpdate: test.NewMockUpdateFn(errors.New("trap")),
					}
				}(),
				log: logging.NewNopLogger(),
			},
			packageInstaller: packageInstallResource(),
			job:              job(),
			obj:              unstructuredObj(crdRaw),
			want: want{
				err: errors.Wrapf(errors.Wrap(errBoom, "failed to fetch existing crd"), "can not update existing CRD %s from job %s", "mytypes.samples.upbound.io", "cool-packageinstall"),
				obj: nil,
			},
		},
		{
			name: "FailedCRDBeingDeleted",
			jobCompleter: &packageInstallJobCompleter{
				client: func() client.Client {
					crd := crd(withCRDGroupKind("samples.upbound.io", "Mytype"), withCRDDeletionTimestamp(time.Date(2020, 04, 01, 0, 0, 0, 0, time.UTC)))
					return fake.NewFakeClient(&crd)
				}(),
				log: logging.NewNopLogger(),
			},
			packageInstaller: packageInstallResource(),
			job:              job(),
			obj:              unstructuredObj(crdRaw),
			want: want{
				err: errors.Wrapf(errors.New("failed due to pending deletion of existing crd"), "can not update existing CRD %s from job %s", "mytypes.samples.upbound.io", "cool-packageinstall"),
				obj: nil,
			},
		},
		{
			name: "FailedIncompatibleCRDExists",
			jobCompleter: &packageInstallJobCompleter{
				client: func() client.Client {
					crd := crd(withCRDGroupKind("samples.upbound.io", "Mytype"), withCRDVersion("existing"))
					return fake.NewFakeClient(&crd)
				}(),
				log: logging.NewNopLogger(),
			},
			packageInstaller: packageInstallResource(),
			job:              job(),
			obj:              unstructuredObj(crdRaw),
			want: want{
				err: errors.Wrapf(errors.New("failed due to replacement crd lacking required versions"), "can not update existing CRD %s from job %s", "mytypes.samples.upbound.io", "cool-packageinstall"),
				obj: nil,
			},
		},
		{
			name: "SuccessUpdatingCRD",
			jobCompleter: &packageInstallJobCompleter{
				client: func() client.Client {
					crd := crd(withCRDGroupKind("samples.upbound.io", "Mytype"), withCRDLabels(map[string]string{"foo": "bar"}))
					// NOTE(muvaf): There is a bug in controller-runtime fake
					// client where it sets the resource version to 1 even if
					// it returns AlreadyExists error later on. So, you end up
					// with having resource version 0 in api-server but Create
					// changes the local object's version to 1.
					// See https://github.com/kubernetes-sigs/controller-runtime/issues/918
					crd.SetResourceVersion("1")
					return fake.NewFakeClient(&crd)
				}(),
				log: logging.NewNopLogger(),
			},
			packageInstaller: packageInstallResource(),
			job:              job(),
			obj:              unstructuredObj(crdRaw, unstructuredAsCRD(withCRDVersion("new"))),
			want: want{
				err: nil,
				obj: unstructuredObj(crdRaw, unstructuredAsCRD(withCRDVersion("new"), withCRDLabels(map[string]string{"foo": "bar"}))),
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			gotErr := tt.jobCompleter.createJobOutputObject(ctx, tt.obj, tt.packageInstaller, tt.job)

			if diff := cmp.Diff(tt.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("createJobOutputObject(): -want error, +got error:\n%s", diff)
			}

			if tt.want.obj != nil {
				gvk := tt.want.obj.GroupVersionKind()
				got := &unstructured.Unstructured{}
				got.SetGroupVersionKind(gvk)
				assertKubernetesObject(t, g, got, tt.want.obj, tt.jobCompleter.client)
			}

		})
	}
}
