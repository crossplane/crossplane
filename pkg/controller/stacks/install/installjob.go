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
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/controller/stacks/hosted"
	"github.com/crossplaneio/crossplane/pkg/stacks"
)

var (
	jobBackoff                = int32(0)
	registryDirName           = "/.registry"
	packageContentsVolumeName = "package-contents"
)

// JobCompleter is an interface for handling job completion
type jobCompleter interface {
	handleJobCompletion(ctx context.Context, i stacks.KindlyIdentifier, job *batchv1.Job) error
}

// StackInstallJobCompleter is a concrete implementation of the jobCompleter interface
type stackInstallJobCompleter struct {
	client       client.Client
	hostClient   client.Client
	podLogReader Reader
	log          logging.Logger
}

func createInstallJob(i v1alpha1.StackInstaller, executorInfo *stacks.ExecutorInfo, hCfg *hosted.Config, tscImage string) *batchv1.Job {
	name := i.GetName()
	namespace := i.GetNamespace()

	if hCfg != nil {
		// In Hosted Mode, we need to map all install jobs on tenant Kubernetes into a single namespace on host cluster.
		o := hCfg.ObjectReferenceOnHost(name, namespace)
		name = o.Name
		namespace = o.Namespace
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    stacks.ParentLabels(i),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &jobBackoff,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						{
							Name:    "stack-copy-to-volume",
							Image:   i.Image(),
							Command: []string{"cp", "-R", registryDirName, "/ext-pkg/"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      packageContentsVolumeName,
									MountPath: "/ext-pkg",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "stack-unpack-and-output",
							Image: executorInfo.Image,
							// "--debug" can be added to this list of Args to get debug output from the job,
							// but note that will be included in the stdout from the pod, which makes it
							// impossible to create the resources that the job unpacks.
							Args: []string{
								"stack",
								"unpack",
								fmt.Sprintf("--content-dir=%s", filepath.Join("/ext-pkg", registryDirName)),
								"--permission-scope=" + i.PermissionScope(),
								"--templating-controller-image=" + tscImage,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      packageContentsVolumeName,
									MountPath: "/ext-pkg",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: packageContentsVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
}

func (jc *stackInstallJobCompleter) handleJobCompletion(ctx context.Context, i stacks.KindlyIdentifier, job *batchv1.Job) error {
	// find the pod associated with the given job
	podName, err := jc.findPodNameForJob(ctx, job)
	if err != nil {
		return err
	}

	// read full output from job by retrieving the logs for the job's pod
	b, err := jc.readPodLogs(job.Namespace, podName)
	if err != nil {
		return err
	}

	// decode and process all resources from job output
	d := yaml.NewYAMLOrJSONDecoder(b, 4096)
	for {
		obj := &unstructured.Unstructured{}
		if err := d.Decode(&obj); err != nil {
			if err == io.EOF {
				// we reached the end of the job output
				break
			}
			return errors.Wrapf(err, "failed to parse output from job %s", job.Name)
		}

		// process and create the object that we just decoded
		if err := jc.createJobOutputObject(ctx, obj, i, job); err != nil {
			return err
		}
	}

	return nil
}

// findPodNameForJob finds the pod name associated with the given job.  Note that this functions
// assumes only a single pod will be associated with the job.
func (jc *stackInstallJobCompleter) findPodNameForJob(ctx context.Context, job *batchv1.Job) (string, error) {
	podList, err := jc.findPodsForJob(ctx, job)
	if err != nil {
		return "", err
	}

	if len(podList.Items) != 1 {
		return "", errors.Errorf("pod list for job %s should only have 1 item, actual: %d", job.Name, len(podList.Items))
	}

	return podList.Items[0].Name, nil
}

func (jc *stackInstallJobCompleter) findPodsForJob(ctx context.Context, job *batchv1.Job) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	labelSelector := client.MatchingLabels{
		"job-name": job.Name,
	}
	nsSelector := client.InNamespace(job.Namespace)
	if err := jc.hostClient.List(ctx, podList, labelSelector, nsSelector); err != nil {
		return nil, err
	}

	return podList, nil
}

func (jc *stackInstallJobCompleter) readPodLogs(namespace, name string) (*bytes.Buffer, error) {
	podLogs, err := jc.podLogReader.GetReader(namespace, name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get logs request stream from pod %s", name)
	}
	defer func() { _ = podLogs.Close() }()

	b := new(bytes.Buffer)
	if _, err = io.Copy(b, podLogs); err != nil {
		return nil, errors.Wrapf(err, "failed to copy logs request stream from pod %s", name)
	}

	return b, nil
}

// createJobOutputObject names, labels, and creates resources in the API
// Expected resources are CRD, Stack, StackDefinition, & StackConfiguration
// nolint:gocyclo
func (jc *stackInstallJobCompleter) createJobOutputObject(ctx context.Context, obj *unstructured.Unstructured,
	i stacks.KindlyIdentifier, job *batchv1.Job) error {

	// if we decoded a non-nil unstructured object, try to create it now
	if obj == nil {
		return nil
	}

	// when the current object is a Stack object, make sure the name and namespace are
	// set to match the current StackInstall (if they haven't already been set). Also,
	// set the owner reference of the Stack to be the StackInstall.
	if isStackObject(obj) || isStackDefinitionObject(obj) || isStackConfigurationObject(obj) {
		if obj.GetName() == "" {
			obj.SetName(i.GetName())
		}
		if obj.GetNamespace() == "" {
			obj.SetNamespace(i.GetNamespace())
		}
	}

	// StackDefinition controllers need the name of the StackDefinition
	// which, by design, matches the StackInstall
	if isStackDefinitionObject(obj) {
		if err := setStackDefinitionControllerEnv(obj, i.GetNamespace(), i.GetName()); err != nil {
			return err
		}
	}

	// We want to clean up any installed CRDS when we're deleted. We can't rely
	// on garbage collection because a namespaced object (StackInstall) can't
	// own a cluster scoped object (CustomResourceDefinition), so we use labels
	// instead.
	labels := stacks.ParentLabels(i)

	// CRDs are labeled with the namespaces of the stacks they are managed by.
	// This will allow for a single Namespaced stack to be installed in multiple
	// namespaces, or different stacks (possibly only differing by versions) to
	// provide the same CRDs without the risk that a single StackInstall removal
	// will delete a CRD until there are no remaining namespace labels.
	if isCRDObject(obj) {
		labelNamespace := fmt.Sprintf(stacks.LabelNamespaceFmt, i.GetNamespace())

		labels[labelNamespace] = "true"
	}

	meta.AddLabels(obj, labels)

	jc.log.Debug(
		"creating object from job output",
		"job", job.Name,
		"name", obj.GetName(),
		"namespace", obj.GetNamespace(),
		"apiVersion", obj.GetAPIVersion(),
		"kind", obj.GetKind(),
		"labels", labels,
	)
	if err := jc.client.Create(ctx, obj); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create object %s from job output %s", obj.GetName(), job.Name)
	}

	return nil
}

func isStackObject(obj *unstructured.Unstructured) bool {
	if obj == nil {
		return false
	}

	gvk := obj.GroupVersionKind()
	return gvk.Group == v1alpha1.Group && gvk.Version == v1alpha1.Version &&
		strings.EqualFold(gvk.Kind, v1alpha1.StackKind)
}

func isStackDefinitionObject(obj *unstructured.Unstructured) bool {
	if obj == nil {
		return false
	}

	gvk := obj.GroupVersionKind()

	return gvk.Group == v1alpha1.Group && gvk.Version == v1alpha1.Version &&
		strings.EqualFold(gvk.Kind, v1alpha1.StackDefinitionKind)
}

func isStackConfigurationObject(obj *unstructured.Unstructured) bool {
	if obj == nil {
		return false
	}

	gvk := obj.GroupVersionKind()

	if gvk.Group == v1alpha1.Group && gvk.Version == v1alpha1.Version &&
		strings.EqualFold(gvk.Kind, v1alpha1.StackConfigurationKind) {
		return true
	}

	return false
}

func isCRDObject(obj runtime.Object) bool {
	if obj == nil {
		return false
	}
	gvk := obj.GetObjectKind().GroupVersionKind()

	return apiextensions.SchemeGroupVersion == gvk.GroupVersion() &&
		strings.EqualFold(gvk.Kind, "CustomResourceDefinition")
}

func setStackDefinitionControllerEnv(obj *unstructured.Unstructured, namespace, name string) error {
	env := []corev1.EnvVar{{
		Name:  stacks.StackDefinitionNamespaceEnv,
		Value: namespace,
	}, {
		Name:  stacks.StackDefinitionNameEnv,
		Value: name,
	}}

	// use convert functions because SetUnstructuredContent is unwieldy
	sd, err := convertToStackDefinition(obj)
	if err != nil {
		return err
	}

	if d := sd.Spec.Controller.Deployment; d != nil {
		c := d.Spec.Template.Spec.Containers
		c[0].Env = append(c[0].Env, env...)

		if u, err := convertToUnstructured(sd); err == nil {
			u.DeepCopyInto(obj)
		}
	}

	return err
}

// convertToUnstructured takes a Kubernetes object and converts it into
// *unstructured.Unstructured that can be used as KubernetesApplication template.
func convertToUnstructured(o *v1alpha1.StackDefinition) (*unstructured.Unstructured, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: u}, nil
}

// convertToStackDefinition takes a Kubernetes object and converts it into
// *v1alpha1.StackDefinition
func convertToStackDefinition(o *unstructured.Unstructured) (*v1alpha1.StackDefinition, error) {
	sd := &v1alpha1.StackDefinition{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.UnstructuredContent(), sd); err != nil {
		return &v1alpha1.StackDefinition{}, err
	}
	return sd, nil
}
