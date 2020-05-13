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
	v1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane/apis/packages/v1alpha1"
	"github.com/crossplane/crossplane/pkg/packages"
)

var (
	jobBackoff                = int32(0)
	registryDirName           = "/.registry"
	packageContentsVolumeName = "package-contents"
)

// JobCompleter is an interface for handling job completion
type jobCompleter interface {
	handleJobCompletion(ctx context.Context, i v1alpha1.PackageInstaller, job *batchv1.Job) error
}

// PackageInstallJobCompleter is a concrete implementation of the jobCompleter interface
type packageInstallJobCompleter struct {
	client       client.Client
	hostClient   client.Client
	podLogReader Reader
	log          logging.Logger
}

type buildInstallJobParams struct {
	name                     string
	namespace                string
	permissionScope          string
	img                      string
	packageManagerImage      string
	tscImage                 string
	packageManagerPullPolicy corev1.PullPolicy
	imagePullPolicy          corev1.PullPolicy
	labels                   map[string]string
	annotations              map[string]string
	imagePullSecrets         []corev1.LocalObjectReference
}

func buildInstallJob(p buildInstallJobParams) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        p.name,
			Namespace:   p.namespace,
			Labels:      p.labels,
			Annotations: p.annotations,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &jobBackoff,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ImagePullSecrets: p.imagePullSecrets,
					RestartPolicy:    corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						{
							Name:            "package-copy-to-volume",
							Image:           p.img,
							ImagePullPolicy: p.imagePullPolicy,
							Command:         []string{"cp", "-R", registryDirName, "/ext-pkg/"},
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
							Name:            "package-unpack-and-output",
							Image:           p.packageManagerImage,
							ImagePullPolicy: p.packageManagerPullPolicy,
							// "--debug" can be added to this list of Args to get debug output from the job,
							// but note that will be included in the stdout from the pod, which makes it
							// impossible to create the resources that the job unpacks.
							Args: []string{
								"package",
								"unpack",
								fmt.Sprintf("--content-dir=%s", filepath.Join("/ext-pkg", registryDirName)),
								"--permission-scope=" + p.permissionScope,
								"--templating-controller-image=" + p.tscImage,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      packageContentsVolumeName,
									MountPath: "/ext-pkg",
								},
							},
							Env: []v1.EnvVar{
								{
									Name:  packages.PackageImageEnv,
									Value: p.img,
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

func (jc *packageInstallJobCompleter) handleJobCompletion(ctx context.Context, i v1alpha1.PackageInstaller, job *batchv1.Job) error {
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
func (jc *packageInstallJobCompleter) findPodNameForJob(ctx context.Context, job *batchv1.Job) (string, error) {
	podList, err := jc.findPodsForJob(ctx, job)
	if err != nil {
		return "", err
	}

	if len(podList.Items) != 1 {
		return "", errors.Errorf("pod list for job %s should only have 1 item, actual: %d", job.Name, len(podList.Items))
	}

	return podList.Items[0].Name, nil
}

func (jc *packageInstallJobCompleter) findPodsForJob(ctx context.Context, job *batchv1.Job) (*corev1.PodList, error) {
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

func (jc *packageInstallJobCompleter) readPodLogs(namespace, name string) (*bytes.Buffer, error) {
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
// Expected resources are CRD, Package, & StackDefinition
// nolint:gocyclo
func (jc *packageInstallJobCompleter) createJobOutputObject(ctx context.Context, obj *unstructured.Unstructured,
	i v1alpha1.PackageInstaller, job *batchv1.Job) error {

	// if we decoded a non-nil unstructured object, try to create it now
	if obj == nil {
		return nil
	}

	// Modify Package and StackDefinition resources based on PackageInstall
	isPackage := isPackageObject(obj)
	isStackDefinition := !isPackage && isStackDefinitionObject(obj)

	if isPackage || isStackDefinition {
		ns := i.GetNamespace()
		name := i.GetName()
		if obj.GetName() == "" {
			obj.SetName(name)
		}
		if obj.GetNamespace() == "" {
			obj.SetNamespace(ns)
		}

		packageImg := i.GetPackage()

		modifiers := []packageSpecModifier{
			controllerImageInjector(packageImg),
			controllerPullSetter(i.GetImagePullPolicy(), i.GetImagePullSecrets()),
			controllerImageSourcer(i),
			saAnnotationSetter(i.GetServiceAccountAnnotations()),
		}

		labels := packages.ParentLabels(i)
		meta.AddLabels(obj, labels)

		// StackDefinition controllers need the name of the StackDefinition
		// which, by design, matches the PackageInstall
		if isStackDefinition {
			modifiers = append(modifiers, controllerEnvSetter(ns, name))
			if err := setupStackDefinitionController(obj, modifiers...); err != nil {
				return err
			}
		} else if err := setupPackageController(obj, modifiers...); err != nil {
			return err
		}
	}

	jc.log.Debug(
		"creating object from job output",
		"job", job.Name,
		"name", obj.GetName(),
		"namespace", obj.GetNamespace(),
		"apiVersion", obj.GetAPIVersion(),
		"kind", obj.GetKind(),
	)

	if err := jc.client.Create(ctx, obj); err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create object %s from job output %s", obj.GetName(), job.Name)
		}

		if !isCRD(obj) {
			return nil
		}

		if err := jc.replaceCRD(ctx, obj); err != nil {
			return errors.Wrapf(err, "can not update existing CRD %s from job %s", obj.GetName(), job.Name)
		}
	}

	return nil
}

func (jc *packageInstallJobCompleter) replaceCRD(ctx context.Context, obj *unstructured.Unstructured) error {
	existing := &apiextensions.CustomResourceDefinition{}
	nsn := types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}

	if err := jc.client.Get(ctx, nsn, existing); err != nil {
		return errors.Wrapf(err, "failed to fetch existing crd")
	}

	if meta.WasDeleted(existing) {
		return errors.Errorf("failed due to pending deletion of existing crd")
	}

	crd, err := convertToCRD(obj)
	if err != nil {
		return errors.Wrapf(err, "failed to convert unstructured crd from job log")
	}

	if !crdIsVersionsInclusive(existing, crd) {
		return errors.Errorf("failed due to replacement crd lacking required versions")
	}

	// TODO(displague) reconsider preferring existing annotations over new
	// annotations (example: new ui metadata)
	meta.AddLabels(obj, existing.GetLabels())
	meta.AddAnnotations(obj, existing.GetAnnotations())

	return resource.NewAPIPatchingApplicator(jc.client).Apply(ctx, obj)
}

// TODO(displague) this is copied from packages. centralize.
func crdVersionExists(crd *apiextensions.CustomResourceDefinition, version string) bool {
	for _, v := range crd.Spec.Versions {
		if v.Name == version {
			return true
		}
	}
	return false
}

// crdIsVersionsInclusive verifies that all versions included in the existing
// crd are available in the replacement crd
func crdIsVersionsInclusive(existing, replacement *apiextensions.CustomResourceDefinition) bool {
	for _, v := range existing.Spec.Versions {
		if !crdVersionExists(replacement, v.Name) {
			return false
		}
	}
	return true
}

type imageWithSourcer interface {
	ImageWithSource(string) (string, error)
}

func isPackageObject(obj packages.KindlyIdentifier) bool {
	if obj == nil {
		return false
	}

	gvk := obj.GroupVersionKind()
	return gvk.Group == v1alpha1.Group && gvk.Version == v1alpha1.Version &&
		strings.EqualFold(gvk.Kind, v1alpha1.PackageKind)
}

func isCRD(obj packages.KindlyIdentifier) bool {
	if obj == nil {
		return false
	}

	gvk := obj.GroupVersionKind()
	return apiextensions.SchemeGroupVersion == gvk.GroupVersion() &&
		strings.EqualFold(gvk.Kind, "CustomResourceDefinition")
}

func isStackDefinitionObject(obj packages.KindlyIdentifier) bool {
	if obj == nil {
		return false
	}

	gvk := obj.GroupVersionKind()

	return gvk.Group == v1alpha1.Group && gvk.Version == v1alpha1.Version &&
		strings.EqualFold(gvk.Kind, v1alpha1.StackDefinitionKind)
}

func setupStackDefinitionController(obj *unstructured.Unstructured, modifiers ...packageSpecModifier) error {
	if len(modifiers) == 0 {
		return nil
	}

	// use convert functions because SetUnstructuredContent is unwieldy
	sd, err := convertToStackDefinition(obj)
	if err != nil {
		return err
	}

	spec := sd.Spec.DeepCopy()
	for _, m := range modifiers {
		if err := m(&spec.PackageSpec); err != nil {
			return err
		}
	}
	spec.PackageSpec.DeepCopyInto(&sd.Spec.PackageSpec)

	if u, err := convertStackDefinitionToUnstructured(sd); err == nil {
		u.DeepCopyInto(obj)
	}

	return err
}

func setupPackageController(obj *unstructured.Unstructured, modifiers ...packageSpecModifier) error {
	if len(modifiers) == 0 {
		return nil
	}

	// use convert functions because SetUnstructuredContent is unwieldy
	s, err := convertToPackage(obj)
	if err != nil {
		return err
	}

	spec := s.Spec.DeepCopy()
	for _, m := range modifiers {
		if err := m(spec); err != nil {
			return err
		}
	}
	spec.DeepCopyInto(&s.Spec)

	if u, err := convertPackageToUnstructured(s); err == nil {
		u.DeepCopyInto(obj)
	}

	return err
}

func controllerEnvSetter(namespace, name string) packageSpecModifier {
	return func(spec *v1alpha1.PackageSpec) error {
		env := []corev1.EnvVar{{
			Name:  packages.StackDefinitionNamespaceEnv,
			Value: namespace,
		}, {
			Name:  packages.StackDefinitionNameEnv,
			Value: name,
		}}

		if d := spec.Controller.Deployment; d != nil {
			cs := d.Spec.Template.Spec.Containers
			cs[0].Env = append(cs[0].Env, env...)
		}
		return nil
	}
}

func controllerImageSourcer(src imageWithSourcer) packageSpecModifier {
	return func(spec *v1alpha1.PackageSpec) error {
		d := spec.Controller.Deployment
		if d == nil {
			return nil
		}

		ics := d.Spec.Template.Spec.InitContainers
		for i := range ics {
			if img, err := src.ImageWithSource(ics[i].Image); err == nil {
				ics[i].Image = img
			}
		}

		cs := d.Spec.Template.Spec.Containers
		for i := range cs {
			if img, err := src.ImageWithSource(cs[i].Image); err == nil {
				cs[i].Image = img
			}
		}
		return nil
	}
}

// controllerImageInjector adds a package image to a package or stack definition
// spec, if there isn't one specified. The reason this exists is so that a package
// author can use the same image for their package envelope (the image which is unpacked)
// and their package controller (the image which runs in the cluster), because that
// is a common pattern, and it's inconvenient to manage the image names separately
// if there are two sources of truth instead of a single source of truth.
func controllerImageInjector(packageImage string) packageSpecModifier {
	return func(spec *v1alpha1.PackageSpec) error {
		// If the package image is empty, we don't need to propagate an empty string
		// down into more fields
		if packageImage == "" {
			return nil
		}

		if d := spec.Controller.Deployment; d != nil {
			spec := &d.Spec.Template.Spec

			for i := range spec.InitContainers {
				if spec.InitContainers[i].Image == "" {
					spec.InitContainers[i].Image = packageImage
				}
			}

			for i := range spec.Containers {
				if spec.Containers[i].Image == "" {
					spec.Containers[i].Image = packageImage
				}
			}
		}

		return nil
	}
}

func controllerPullSetter(imagePullPolicy v1.PullPolicy, imagePullSecrets []v1.LocalObjectReference) packageSpecModifier {
	return func(spec *v1alpha1.PackageSpec) error {
		if d := spec.Controller.Deployment; d != nil {
			spec := &d.Spec.Template.Spec

			spec.ImagePullSecrets = append(spec.ImagePullSecrets, imagePullSecrets...)

			for i := range spec.InitContainers {
				spec.InitContainers[i].ImagePullPolicy = imagePullPolicy
			}

			for i := range spec.Containers {
				spec.Containers[i].ImagePullPolicy = imagePullPolicy
			}
		}

		return nil
	}
}

func saAnnotationSetter(annotations map[string]string) packageSpecModifier {
	return func(spec *v1alpha1.PackageSpec) error {
		if len(annotations) == 0 {
			return nil
		}

		if spec.Controller.ServiceAccount == nil {
			spec.Controller.ServiceAccount = &v1alpha1.ServiceAccountOptions{Annotations: map[string]string{}}
		}

		for k, v := range annotations {
			spec.Controller.ServiceAccount.Annotations[k] = v
		}

		return nil
	}
}

type packageSpecModifier func(spec *v1alpha1.PackageSpec) error

// convertStackDefinitionToUnstructured takes a StackDefinition and converts it
// to *unstructured.Unstructured
func convertStackDefinitionToUnstructured(o *v1alpha1.StackDefinition) (*unstructured.Unstructured, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: u}, nil
}

// convertPackageToUnstructured takes a Package and converts it to
// *unstructured.Unstructured
func convertPackageToUnstructured(o *v1alpha1.Package) (*unstructured.Unstructured, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: u}, nil
}

// convertToPackage takes a Kubernetes object and converts it into
// *v1alpha1.Package
func convertToPackage(o *unstructured.Unstructured) (*v1alpha1.Package, error) {
	s := &v1alpha1.Package{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.UnstructuredContent(), s); err != nil {
		return &v1alpha1.Package{}, err
	}
	return s, nil
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

// convertToCRD takes a Kubernetes object and converts it into
// *apiextensions.CustomResourceDefinition
func convertToCRD(o *unstructured.Unstructured) (*apiextensions.CustomResourceDefinition, error) {
	sd := &apiextensions.CustomResourceDefinition{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.UnstructuredContent(), sd); err != nil {
		return nil, err
	}
	return sd, nil
}
