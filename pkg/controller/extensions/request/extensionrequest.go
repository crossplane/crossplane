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
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	controllerHandler "sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/extensions/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	controllerName = "extensionrequest.extensions.crossplane.io"

	reconcileTimeout      = 1 * time.Minute
	requeueAfterOnSuccess = 10 * time.Second

	packageContentsVolumeName = "package-contents"

	reasonCreatingJob             = "failed to create extension manager job"
	reasonFetchingJob             = "failed to fetch extension manager job"
	reasonJobFailed               = "extension manager job failed"
	reasonHandlingJobCompletion   = "failed to handle the extension manager job completion"
	reasonDiscoveringExecutorInfo = "failed to discover package executor info"
)

var (
	log              = logging.Logger.WithName(controllerName)
	resultRequeue    = reconcile.Result{Requeue: true}
	requeueOnSuccess = reconcile.Result{RequeueAfter: requeueAfterOnSuccess}
	jobBackoff       = int32(0)
)

// Reconciler reconciles a Instance object
type Reconciler struct {
	sync.Mutex
	kube       client.Client
	kubeclient kubernetes.Interface
	factory
	executorInfoDiscovery
	executorInfo *executorInfo
}

// Add creates a new ExtensionRequest Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		kube:                  mgr.GetClient(),
		kubeclient:            kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		factory:               &handlerFactory{},
		executorInfoDiscovery: &executorInfoDiscoverer{kube: mgr.GetClient()},
	}

	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to ExtensionRequest
	return c.Watch(&source.Kind{Type: &v1alpha1.ExtensionRequest{}}, &controllerHandler.EnqueueRequestForObject{})
}

// Reconcile reads that state of the ExtensionRequest for a Instance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", v1alpha1.ExtensionRequestKindAPIVersion, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	// fetch the CRD instance
	i := &v1alpha1.ExtensionRequest{}
	if err := r.kube.Get(ctx, req.NamespacedName, i); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if err := r.discoverExecutorInfo(ctx); err != nil {
		return fail(ctx, r.kube, i, reasonDiscoveringExecutorInfo, err.Error())
	}

	handler := r.factory.newHandler(ctx, i, r.kube, r.kubeclient, *r.executorInfo)

	return handler.sync(ctx)
}

// handler is an interface for handling reconciliation requests
type handler interface {
	sync(context.Context) (reconcile.Result, error)
	create(context.Context) (reconcile.Result, error)
	update(context.Context) (reconcile.Result, error)
}

// extensionRequestHandler is a concrete implementation of the handler interface
type extensionRequestHandler struct {
	kube         client.Client
	jobCompleter jobCompleter
	executorInfo executorInfo
	ext          *v1alpha1.ExtensionRequest
}

// jobCompleter is an interface for handling job completion
type jobCompleter interface {
	handleJobCompletion(ctx context.Context, i *v1alpha1.ExtensionRequest, job *batchv1.Job) error
}

// extensionRequestJobCompleter is a concrete implementation of the jobCompleter interface
type extensionRequestJobCompleter struct {
	kube         client.Client
	podLogReader podLogReader
}

// podLogReader is an interface for reading pod logs
type podLogReader interface {
	getPodLogReader(string, string) (io.ReadCloser, error)
}

// k8sPodLogReader is a concrete implementation of the podLogReader interface
type k8sPodLogReader struct {
	kubeclient kubernetes.Interface
}

// factory is an interface for creating new handlers
type factory interface {
	newHandler(context.Context, *v1alpha1.ExtensionRequest, client.Client, kubernetes.Interface, executorInfo) handler
}

type handlerFactory struct{}

func (f *handlerFactory) newHandler(ctx context.Context, ext *v1alpha1.ExtensionRequest,
	kube client.Client, kubeclient kubernetes.Interface, ei executorInfo) handler {

	return &extensionRequestHandler{
		ext:          ext,
		kube:         kube,
		executorInfo: ei,
		jobCompleter: &extensionRequestJobCompleter{
			kube: kube,
			podLogReader: &k8sPodLogReader{
				kubeclient: kubeclient,
			},
		},
	}
}

// ************************************************************************************************
// Syncing/Creating functions
// ************************************************************************************************
func (h *extensionRequestHandler) sync(ctx context.Context) (reconcile.Result, error) {
	if h.ext.Status.ExtensionRecord == nil {
		return h.create(ctx)
	}

	return h.update(ctx)
}

// create performs the operation of creating the associated Extension.  This function assumes
// that the Extension does not yet exist, so the caller should confirm that before calling.
func (h *extensionRequestHandler) create(ctx context.Context) (reconcile.Result, error) {
	jobRef := h.ext.Status.InstallJob

	if jobRef == nil {
		// there is no install job created yet, create it now
		job := createInstallJob(h.ext, h.executorInfo)
		if err := h.kube.Create(ctx, job); err != nil {
			return fail(ctx, h.kube, h.ext, reasonCreatingJob, err.Error())
		}

		jobRef = &corev1.ObjectReference{
			Name:      job.Name,
			Namespace: job.Namespace,
		}

		// set a Creating condition on the status and save a reference to the install job we just created
		h.ext.Status.SetCreating()
		h.ext.Status.InstallJob = jobRef
		log.V(logging.Debug).Info("created install job", "jobRef", jobRef, "jobOwnerRefs", job.OwnerReferences)

		return requeueOnSuccess, h.kube.Status().Update(ctx, h.ext)
	}

	// the install job already exists, let's check its status and completion
	job := &batchv1.Job{}
	n := types.NamespacedName{Namespace: jobRef.Namespace, Name: jobRef.Name}
	if err := h.kube.Get(ctx, n, job); err != nil {
		return fail(ctx, h.kube, h.ext, reasonFetchingJob, err.Error())
	}

	log.V(logging.Debug).Info(
		"checking install job status",
		"job", fmt.Sprintf("%s/%s", job.Namespace, job.Name),
		"conditions", job.Status.Conditions)

	for _, c := range job.Status.Conditions {
		if c.Status == corev1.ConditionTrue {
			switch c.Type {
			case batchv1.JobComplete:
				// the install job succeeded, process the output
				if err := h.jobCompleter.handleJobCompletion(ctx, h.ext, job); err != nil {
					return fail(ctx, h.kube, h.ext, reasonHandlingJobCompletion, err.Error())
				}

				// the install job's completion was handled successfully, this extension request is ready
				h.ext.Status.UnsetAllDeprecatedConditions()
				h.ext.Status.SetReady()
				return requeueOnSuccess, h.kube.Status().Update(ctx, h.ext)
			case batchv1.JobFailed:
				// the install job failed, report the failure
				return fail(ctx, h.kube, h.ext, reasonJobFailed, c.Message)
			}
		}
	}

	// the job hasn't completed yet, so requeue and check again next time
	log.V(logging.Debug).Info("install job not complete", "job", fmt.Sprintf("%s/%s", job.Namespace, job.Name))
	return requeueOnSuccess, h.kube.Status().Update(ctx, h.ext)
}

func (h *extensionRequestHandler) update(ctx context.Context) (reconcile.Result, error) {
	// TODO: should updates of the ExtensionRequest be supported? what would that even mean, they
	// changed the package they wanted installed? Shouldn't they delete the ExtensionRequest and
	// create a new one?
	log.V(logging.Debug).Info("updating not supported yet", "extensionRequest", h.ext.Name)
	return reconcile.Result{}, nil
}

func createInstallJob(i *v1alpha1.ExtensionRequest, executorInfo executorInfo) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            i.Name,
			Namespace:       i.Namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsOwner(meta.ReferenceTo(i))},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &jobBackoff,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						{
							Name:    "extension-package",
							Image:   getPackageImage(i.Spec),
							Command: []string{"cp", "-R", "/.registry/", "/ext-pkg/"},
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
							Name:  "extension-executor",
							Image: executorInfo.image,
							// "--debug" can be added to this list of Args to get debug output from the job,
							// but note that will be included in the stdout from the pod, which makes it
							// impossible to create the resources that the job unpacks.
							Args: []string{"extension", "unpack", "--content-dir=/ext-pkg"},
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

func (jc *extensionRequestJobCompleter) handleJobCompletion(ctx context.Context, i *v1alpha1.ExtensionRequest, job *batchv1.Job) error {
	var extensionRecord *v1alpha1.Extension

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
			return fmt.Errorf("failed to parse output from job %s: %+v", job.Name, err)
		}

		// process and create the object that we just decoded
		if err := jc.createJobOutputObject(ctx, obj, i, job); err != nil {
			return err
		}

		if isExtensionObject(obj) {
			// we just created the extension record, try to fetch it now so that it can be returned
			extensionRecord = &v1alpha1.Extension{}
			n := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
			if err := jc.kube.Get(ctx, n, extensionRecord); err != nil {
				return fmt.Errorf("failed to retrieve created extension record %s from job %s: %+v", obj.GetName(), job.Name, err)
			}
		}
	}

	if extensionRecord == nil {
		return fmt.Errorf("failed to find an extension record from job %s", job.Name)
	}

	// save a reference to the extension record in the status of the extension request
	i.Status.ExtensionRecord = &corev1.ObjectReference{
		APIVersion: extensionRecord.APIVersion,
		Kind:       extensionRecord.Kind,
		Name:       extensionRecord.Name,
		Namespace:  extensionRecord.Namespace,
		UID:        extensionRecord.ObjectMeta.UID,
	}

	return nil
}

// findPodNameForJob finds the pod name associated with the given job.  Note that this functions
// assumes only a single pod will be associated with the job.
func (jc *extensionRequestJobCompleter) findPodNameForJob(ctx context.Context, job *batchv1.Job) (string, error) {
	podList, err := jc.findPodsForJob(ctx, job)
	if err != nil {
		return "", err
	}

	if len(podList.Items) != 1 {
		return "", fmt.Errorf("pod list for job %s should only have 1 item, actual: %d", job.Name, len(podList.Items))
	}

	return podList.Items[0].Name, nil
}

func (jc *extensionRequestJobCompleter) findPodsForJob(ctx context.Context, job *batchv1.Job) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	labelSelector := labels.Set{"job-name": job.Name}
	podListOptions := &client.ListOptions{
		Namespace:     job.Namespace,
		LabelSelector: labelSelector.AsSelector(),
	}
	if err := jc.kube.List(ctx, podListOptions, podList); err != nil {
		return nil, err
	}

	return podList, nil
}

func (jc *extensionRequestJobCompleter) readPodLogs(namespace, name string) (*bytes.Buffer, error) {
	podLogs, err := jc.podLogReader.getPodLogReader(namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs request stream from pod %s: %+v", name, err)
	}
	defer func() { _ = podLogs.Close() }()

	b := new(bytes.Buffer)
	if _, err = io.Copy(b, podLogs); err != nil {
		return nil, fmt.Errorf("failed to copy logs request stream from pod %s: %+v", name, err)
	}

	return b, nil
}

func (jc *extensionRequestJobCompleter) createJobOutputObject(ctx context.Context, obj *unstructured.Unstructured,
	i *v1alpha1.ExtensionRequest, job *batchv1.Job) error {

	// if we decoded a non-nil unstructured object, try to create it now
	if obj == nil {
		return nil
	}

	if isExtensionObject(obj) {
		// the current object is an Extension object, make sure the name and namespace are
		// set to match the current ExtensionRequest (if they haven't already been set)
		if obj.GetName() == "" {
			obj.SetName(i.Name)
		}
		if obj.GetNamespace() == "" {
			obj.SetNamespace(i.Namespace)
		}
	}

	// set an owner reference on the object
	obj.SetOwnerReferences([]metav1.OwnerReference{meta.AsOwner(meta.ReferenceTo(i))})

	log.V(logging.Debug).Info(
		"creating object from job output",
		"job", job.Name,
		"name", obj.GetName(),
		"namespace", obj.GetNamespace(),
		"apiVersion", obj.GetAPIVersion(),
		"kind", obj.GetKind(),
		"ownerRefs", obj.GetOwnerReferences())

	if err := jc.kube.Create(ctx, obj); err != nil && !kerrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create object %s from job output %s: %+v", obj.GetName(), job.Name, err)
	}

	return nil
}

// ************************************************************************************************
// k8sPodLogReader
// ************************************************************************************************
func (r *k8sPodLogReader) getPodLogReader(namespace, name string) (io.ReadCloser, error) {
	req := r.kubeclient.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{})
	return req.Stream()
}

// ************************************************************************************************
// ExecutorInfo discovery
// ************************************************************************************************

// executorInfo represents the information needed to launch an executor for handling extension requests
type executorInfo struct {
	image string
}

// executorInfoDiscovery is an interface for an entity that can discover executionInfo
type executorInfoDiscovery interface {
	discoverExecutorInfo(ctx context.Context) (*executorInfo, error)
}

// executorInfoDiscoverer is a concrete implementation of the executorInfoDiscovery interface,
// looking up the executorInfo from the runtime environment.
type executorInfoDiscoverer struct {
	kube client.Client
}

// discoverExecutorInfo stores the executorInfo on the reconciler so that it does not have to
// look it up every reconcile loop.  If this info has already been retrieved then the cached
// info will be returned.
func (r *Reconciler) discoverExecutorInfo(ctx context.Context) error {
	// ensure that only 1 thread is touching the executorInfo field on this reconciler
	r.Lock()
	defer r.Unlock()

	if r.executorInfo != nil {
		// we've already cached the executorInfo, nothing else to do
		return nil
	}

	// look up the executorInfo using the given interface to handle the discovery
	ei, err := r.executorInfoDiscovery.discoverExecutorInfo(ctx)
	if err != nil {
		return err
	}

	// cache the executorInfo so we don't have to look it up again
	r.executorInfo = ei
	return nil
}

// discoverExecutorInfo is the concrete implementation that will lookup executorInfo from the runtime environment.
func (d *executorInfoDiscoverer) discoverExecutorInfo(ctx context.Context) (*executorInfo, error) {
	pod, err := util.GetRunningPod(ctx, d.kube)
	if err != nil {
		log.Error(err, "failed to get running pod")
		return nil, err
	}

	image, err := util.GetContainerImage(pod, "")
	if err != nil {
		log.Error(err, "failed to get image for pod", "image", image)
		return nil, err
	}

	return &executorInfo{image: image}, nil
}

// ************************************************************************************************
// Helper functions
// ************************************************************************************************

// fail - helper function to set fail condition with reason and message
func fail(ctx context.Context, kube client.StatusClient, i *v1alpha1.ExtensionRequest, reason, msg string) (reconcile.Result, error) {
	log.V(logging.Debug).Info("failed extension request", "i", i.Name, "reason", reason, "message", msg)
	i.Status.SetFailed(reason, msg)
	i.Status.UnsetDeprecatedCondition(corev1alpha1.DeprecatedReady)
	return resultRequeue, kube.Status().Update(ctx, i)
}

func isExtensionObject(obj *unstructured.Unstructured) bool {
	if obj == nil {
		return false
	}

	gvk := obj.GroupVersionKind()
	return gvk.Group == v1alpha1.Group && gvk.Version == v1alpha1.Version &&
		strings.EqualFold(gvk.Kind, v1alpha1.ExtensionKind)
}

// getPackageImage returns the fully qualified image name for the given package source and package name.
// based on the fully qualified image name format of hostname[:port]/username/reponame[:tag]
func getPackageImage(spec v1alpha1.ExtensionRequestSpec) string {
	if spec.Source == "" {
		// there is no package source, simply return the package name
		return spec.Package
	}

	return fmt.Sprintf("%s/%s", spec.Source, spec.Package)
}
