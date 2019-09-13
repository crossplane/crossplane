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
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pkg/errors"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
	stacks "github.com/crossplaneio/crossplane/pkg/stacks"
)

const (
	reconcileTimeout      = 1 * time.Minute
	requeueAfterOnSuccess = 10 * time.Second
)

var (
	log              = logging.Logger.WithName("stackinstall.stacks.crossplane.io")
	resultRequeue    = reconcile.Result{Requeue: true}
	requeueOnSuccess = reconcile.Result{RequeueAfter: requeueAfterOnSuccess}
)

// Reconciler reconciles a Instance object
type Reconciler struct {
	sync.Mutex
	kube        client.Client
	kubeclient  kubernetes.Interface
	stackinator func() v1alpha1.StackInstaller
	factory
	executorInfoDiscoverer stacks.ExecutorInfoDiscoverer
}

// Controller is responsible for adding the StackInstall
// controller and its corresponding reconciler to the manager with any runtime configuration.
type Controller struct {
	StackInstallCreator func() (string, func() v1alpha1.StackInstaller)
}

// SetupWithManager creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	controllerName, stackInstaller := c.StackInstallCreator()

	kube := mgr.GetClient()
	client := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	discoverer := &stacks.KubeExecutorInfoDiscoverer{Client: kube}

	r := &Reconciler{
		kube:                   kube,
		kubeclient:             client,
		stackinator:            stackInstaller,
		factory:                &handlerFactory{},
		executorInfoDiscoverer: discoverer,
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(stackInstaller()).
		Complete(r)
}

// Reconcile reads that state of the StackInstall for a Instance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	stackInstaller := r.stackinator()
	log.V(logging.Debug).Info("reconciling", "kind", stackInstaller.GroupVersionKind(), "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	// fetch the CRD instance
	if err := r.kube.Get(ctx, req.NamespacedName, stackInstaller); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	executorinfo, err := r.executorInfoDiscoverer.Discover(ctx)
	if err != nil {
		return fail(ctx, r.kube, stackInstaller, err)
	}

	handler := r.factory.newHandler(ctx, stackInstaller, r.kube, r.kubeclient, executorinfo)

	return handler.sync(ctx)
}

// handler is an interface for handling reconciliation requests
type handler interface {
	sync(context.Context) (reconcile.Result, error)
	create(context.Context) (reconcile.Result, error)
	update(context.Context) (reconcile.Result, error)
}

// stackInstallHandler is a concrete implementation of the handler interface
type stackInstallHandler struct {
	kube         client.Client
	jobCompleter jobCompleter
	executorInfo *stacks.ExecutorInfo
	ext          v1alpha1.StackInstaller
}

// factory is an interface for creating new handlers
type factory interface {
	newHandler(context.Context, v1alpha1.StackInstaller, client.Client, kubernetes.Interface, *stacks.ExecutorInfo) handler
}

type handlerFactory struct{}

func (f *handlerFactory) newHandler(ctx context.Context, ext v1alpha1.StackInstaller,
	kube client.Client, kubeclient kubernetes.Interface, ei *stacks.ExecutorInfo) handler {

	return &stackInstallHandler{
		ext:          ext,
		kube:         kube,
		executorInfo: ei,
		jobCompleter: &stackInstallJobCompleter{
			client: kube,
			podLogReader: &K8sReader{
				Client: kubeclient,
			},
		},
	}
}

// ************************************************************************************************
// Syncing/Creating functions
// ************************************************************************************************
func (h *stackInstallHandler) sync(ctx context.Context) (reconcile.Result, error) {
	if h.ext.StackRecord() == nil {
		return h.create(ctx)
	}

	return h.update(ctx)
}

// create performs the operation of creating the associated Stack.  This function assumes
// that the Stack does not yet exist, so the caller should confirm that before calling.
func (h *stackInstallHandler) create(ctx context.Context) (reconcile.Result, error) {
	h.ext.SetConditions(runtimev1alpha1.Creating())
	jobRef := h.ext.InstallJob()

	if jobRef == nil {
		// there is no install job created yet, create it now
		job := createInstallJob(h.ext, h.executorInfo)
		if err := h.kube.Create(ctx, job); err != nil {
			return fail(ctx, h.kube, h.ext, err)
		}

		jobRef = &corev1.ObjectReference{
			Name:      job.Name,
			Namespace: job.Namespace,
		}

		// Save a reference to the install job we just created
		h.ext.SetInstallJob(jobRef)
		h.ext.SetConditions(runtimev1alpha1.ReconcileSuccess())
		log.V(logging.Debug).Info("created install job", "jobRef", jobRef, "jobOwnerRefs", job.OwnerReferences)

		return requeueOnSuccess, h.kube.Status().Update(ctx, h.ext)
	}

	// the install job already exists, let's check its status and completion
	job := &batchv1.Job{}
	if err := h.kube.Get(ctx, meta.NamespacedNameOf(jobRef), job); err != nil {
		return fail(ctx, h.kube, h.ext, err)
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
					return fail(ctx, h.kube, h.ext, err)
				}

				// the install job's completion was handled successfully, this stack install is ready
				h.ext.SetConditions(runtimev1alpha1.Available(), runtimev1alpha1.ReconcileSuccess())
				return requeueOnSuccess, h.kube.Status().Update(ctx, h.ext)
			case batchv1.JobFailed:
				// the install job failed, report the failure
				return fail(ctx, h.kube, h.ext, errors.New(c.Message))
			}
		}
	}

	// the job hasn't completed yet, so requeue and check again next time
	h.ext.SetConditions(runtimev1alpha1.ReconcileSuccess())
	log.V(logging.Debug).Info("install job not complete", "job", fmt.Sprintf("%s/%s", job.Namespace, job.Name))

	return requeueOnSuccess, h.kube.Status().Update(ctx, h.ext)
}

func (h *stackInstallHandler) update(ctx context.Context) (reconcile.Result, error) {
	// TODO: should updates of the StackInstall be supported? what would that even mean, they
	// changed the package they wanted installed? Shouldn't they delete the StackInstall and
	// create a new one?
	groupversion, kind := h.ext.GroupVersionKind().ToAPIVersionAndKind()
	log.V(logging.Debug).Info("updating not supported yet", strings.ToLower(kind)+"."+groupversion, fmt.Sprintf("%s/%s", h.ext.GetNamespace(), h.ext.GetName()))
	return reconcile.Result{}, nil
}

// ************************************************************************************************
// Helper functions
// ************************************************************************************************

// fail - helper function to set fail condition with reason and message
func fail(ctx context.Context, kube client.StatusClient, i v1alpha1.StackInstaller, err error) (reconcile.Result, error) {
	log.V(logging.Debug).Info("failed stack install", "i", i.GetName(), "error", err)
	i.SetConditions(runtimev1alpha1.ReconcileError(err))
	return resultRequeue, kube.Status().Update(ctx, i)
}
