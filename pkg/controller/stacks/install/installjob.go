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
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/stacks"
)

var (
	jobBackoff                = int32(0)
	registryDirName           = ".registry"
	packageContentsVolumeName = "package-contents"
)

// JobCompleter is an interface for handling job completion
type jobCompleter interface {
	handleJobCompletion(ctx context.Context, i v1alpha1.StackInstaller, job *batchv1.Job) error
}

// StackInstallJobCompleter is a concrete implementation of the jobCompleter interface
type stackInstallJobCompleter struct {
	client       client.Client
	podLogReader Reader
}

func createInstallJob(i v1alpha1.StackInstaller, executorInfo *stacks.ExecutorInfo) *batchv1.Job {
	ref := meta.AsOwner(meta.ReferenceTo(i, v1alpha1.StackGroupVersionKind))
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            i.GetName(),
			Namespace:       i.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{ref},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &jobBackoff,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						{
							Name:    "stack-package",
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
							Name:  "stack-executor",
							Image: executorInfo.Image,
							// "--debug" can be added to this list of Args to get debug output from the job,
							// but note that will be included in the stdout from the pod, which makes it
							// impossible to create the resources that the job unpacks.
							Args: []string{
								"stack",
								"unpack",
								fmt.Sprintf("--content-dir=%s", filepath.Join("/ext-pkg", registryDirName)),
								"--permission-scope=" + i.PermissionScope(),
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

func (jc *stackInstallJobCompleter) handleJobCompletion(ctx context.Context, i v1alpha1.StackInstaller, job *batchv1.Job) error {
	var stackRecord *v1alpha1.Stack

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

		if isStackObject(obj) {
			// we just created the stack record, try to fetch it now so that it can be returned
			stackRecord = &v1alpha1.Stack{}
			n := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
			if err := jc.client.Get(ctx, n, stackRecord); err != nil {
				return errors.Wrapf(err, "failed to retrieve created stack record %s/%s from job %s", obj.GetNamespace(), obj.GetName(), job.Name)
			}
		}
	}

	if stackRecord == nil {
		return errors.Errorf("failed to find a stack record from job %s", job.Name)
	}

	// save a reference to the stack record in the status of the stack install
	i.SetStackRecord(&corev1.ObjectReference{
		APIVersion: stackRecord.APIVersion,
		Kind:       stackRecord.Kind,
		Name:       stackRecord.Name,
		Namespace:  stackRecord.Namespace,
		UID:        stackRecord.ObjectMeta.UID,
	})

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
	if err := jc.client.List(ctx, podList, labelSelector, nsSelector); err != nil {
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

func (jc *stackInstallJobCompleter) createJobOutputObject(ctx context.Context, obj *unstructured.Unstructured,
	i v1alpha1.StackInstaller, job *batchv1.Job) error {

	// if we decoded a non-nil unstructured object, try to create it now
	if obj == nil {
		return nil
	}

	if isStackObject(obj) {
		// the current object is a Stack object, make sure the name and namespace are
		// set to match the current StackInstall (if they haven't already been set)
		if obj.GetName() == "" {
			obj.SetName(i.GetName())
		}
		if obj.GetNamespace() == "" {
			obj.SetNamespace(i.GetNamespace())
		}
	}

	// set an owner reference on the object
	obj.SetOwnerReferences([]metav1.OwnerReference{
		meta.AsOwner(meta.ReferenceTo(i, i.GroupVersionKind())),
	})

	// TODO(displague) pass/inject a controller specific logger
	log.V(logging.Debug).Info(
		"creating object from job output",
		"job", job.Name,
		"name", obj.GetName(),
		"namespace", obj.GetNamespace(),
		"apiVersion", obj.GetAPIVersion(),
		"kind", obj.GetKind(),
		"ownerRefs", obj.GetOwnerReferences())
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
