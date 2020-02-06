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

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	runtimeresource "github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/controller/stacks/hosted"
	"github.com/crossplaneio/crossplane/pkg/stacks"
)

const (
	reconcileTimeout      = 1 * time.Minute
	requeueAfterOnSuccess = 10 * time.Second
	installFinalizer      = "finalizer.stackinstall.crossplane.io"
)

var (
	resultRequeue    = reconcile.Result{Requeue: true}
	requeueOnSuccess = reconcile.Result{RequeueAfter: requeueAfterOnSuccess}
)

// k8sClients holds the clients for Kubernetes
type k8sClients struct {
	// kube is controller runtime client for resource (a.k.a tenant) Kubernetes where all custom resources live.
	kube client.Client
	// hostKube is controller runtime client for workload (a.k.a host) Kubernetes where jobs for stack installs and
	// stack controller deployments/jobs created.
	hostKube client.Client
	// hostClient is client-go kubernetes client to read logs of stack install pods.
	hostClient kubernetes.Interface
}

// Reconciler reconciles a Instance object
type Reconciler struct {
	sync.Mutex
	k8sClients
	hostedConfig           *hosted.Config
	stackinator            func() v1alpha1.StackInstaller
	executorInfoDiscoverer stacks.ExecutorInfoDiscoverer
	log                    logging.Logger

	factory
}

// SetupClusterStackInstall adds a controller that reconciles
// ClusterStackInstalls.
func SetupClusterStackInstall(mgr ctrl.Manager, l logging.Logger, hostControllerNamespace string) error {
	name := "stacks/" + strings.ToLower(v1alpha1.ClusterStackInstallKind)
	stackinator := func() v1alpha1.StackInstaller { return &v1alpha1.ClusterStackInstall{} }

	// Fail early if ClusterStackInstall is not registered with the scheme.
	stackinator()

	hostKube, hostClient, err := hosted.GetClients()
	if err != nil {
		return err
	}

	hc, err := hosted.NewConfigForHost(hostControllerNamespace, mgr.GetConfig().Host)
	if err != nil {
		return err
	}

	r := &Reconciler{
		k8sClients: k8sClients{
			kube:       mgr.GetClient(),
			hostKube:   hostKube,
			hostClient: hostClient,
		},
		hostedConfig:           hc,
		stackinator:            stackinator,
		factory:                &handlerFactory{},
		executorInfoDiscoverer: &stacks.KubeExecutorInfoDiscoverer{Client: hostKube},
		log:                    l.WithValues("controller", name),
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.ClusterStackInstall{}).
		Complete(r)
}

// SetupStackInstall adds a controller that reconciles StackInstalls.
func SetupStackInstall(mgr ctrl.Manager, l logging.Logger, hostControllerNamespace string) error {
	name := "stacks/" + strings.ToLower(v1alpha1.StackInstallKind)
	stackinator := func() v1alpha1.StackInstaller { return &v1alpha1.StackInstall{} }

	// Fail early if StackInstall is not registered with the scheme.
	stackinator()

	hostKube, hostClient, err := hosted.GetClients()
	if err != nil {
		return err
	}

	hc, err := hosted.NewConfigForHost(hostControllerNamespace, mgr.GetConfig().Host)
	if err != nil {
		return err
	}

	r := &Reconciler{
		k8sClients: k8sClients{
			kube:       mgr.GetClient(),
			hostKube:   hostKube,
			hostClient: hostClient,
		},
		hostedConfig:           hc,
		stackinator:            stackinator,
		factory:                &handlerFactory{},
		executorInfoDiscoverer: &stacks.KubeExecutorInfoDiscoverer{Client: hostKube},
		log:                    l.WithValues("controller", name),
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.StackInstall{}).
		Complete(r)
}

// Reconcile reads that state of the StackInstall for a Instance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	stackInstaller := r.stackinator()
	r.log.Debug("Reconciling", "request", req)

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

	handler := r.factory.newHandler(r.log, stackInstaller, k8sClients{
		kube:       r.kube,
		hostKube:   r.hostKube,
		hostClient: r.hostClient,
	}, r.hostedConfig, executorinfo)

	if meta.WasDeleted(stackInstaller) {
		return handler.delete(ctx)
	}

	return handler.sync(ctx)
}

// handler is an interface for handling reconciliation requests
type handler interface {
	sync(context.Context) (reconcile.Result, error)
	create(context.Context) (reconcile.Result, error)
	update(context.Context) (reconcile.Result, error)
	delete(context.Context) (reconcile.Result, error)
}

// stackInstallHandler is a concrete implementation of the handler interface
type stackInstallHandler struct {
	kube            client.Client
	hostKube        client.Client
	hostAwareConfig *hosted.Config
	jobCompleter    jobCompleter
	executorInfo    *stacks.ExecutorInfo
	ext             v1alpha1.StackInstaller
	log             logging.Logger
}

// factory is an interface for creating new handlers
type factory interface {
	newHandler(logging.Logger, v1alpha1.StackInstaller, k8sClients, *hosted.Config, *stacks.ExecutorInfo) handler
}

type handlerFactory struct{}

func (f *handlerFactory) newHandler(log logging.Logger, ext v1alpha1.StackInstaller, k8s k8sClients, hostAwareConfig *hosted.Config, ei *stacks.ExecutorInfo) handler {

	return &stackInstallHandler{
		ext:             ext,
		kube:            k8s.kube,
		hostKube:        k8s.hostKube,
		hostAwareConfig: hostAwareConfig,
		executorInfo:    ei,
		jobCompleter: &stackInstallJobCompleter{
			client:     k8s.kube,
			hostClient: k8s.hostKube,
			podLogReader: &K8sReader{
				Client: k8s.hostClient,
			},
			log: log,
		},
		log: log,
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

	meta.AddFinalizer(h.ext, installFinalizer)
	if err := h.kube.Update(ctx, h.ext); err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	jobRef := h.ext.InstallJob()

	if jobRef == nil {
		// there is no install job created yet, create it now
		job := createInstallJob(h.ext, h.executorInfo, h.hostAwareConfig)

		if err := h.hostKube.Create(ctx, job); err != nil {
			return fail(ctx, h.kube, h.ext, err)
		}

		jobRef = &corev1.ObjectReference{
			Name:      job.Name,
			Namespace: job.Namespace,
		}

		// Save a reference to the install job we just created
		h.ext.SetInstallJob(jobRef)
		h.ext.SetConditions(runtimev1alpha1.ReconcileSuccess())
		h.log.Debug("created install job", "jobRef", jobRef, "jobOwnerRefs", job.OwnerReferences)

		return requeueOnSuccess, h.kube.Status().Update(ctx, h.ext)
	}

	// the install job already exists, let's check its status and completion
	job := &batchv1.Job{}

	if err := h.hostKube.Get(ctx, meta.NamespacedNameOf(jobRef), job); err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	h.log.Debug(
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
	h.log.Debug("install job not complete", "job", fmt.Sprintf("%s/%s", job.Namespace, job.Name))

	return requeueOnSuccess, h.kube.Status().Update(ctx, h.ext)
}

func (h *stackInstallHandler) update(ctx context.Context) (reconcile.Result, error) {
	// TODO: should updates of the StackInstall be supported? what would that even mean, they
	// changed the package they wanted installed? Shouldn't they delete the StackInstall and
	// create a new one?
	groupversion, kind := h.ext.GroupVersionKind().ToAPIVersionAndKind()
	h.log.Debug("updating not supported yet", strings.ToLower(kind)+"."+groupversion, fmt.Sprintf("%s/%s", h.ext.GetNamespace(), h.ext.GetName()))
	return reconcile.Result{}, nil
}

// delete performs clean up (finalizer) actions when a StackInstall is being deleted.
// This function ensures that all the resources (e.g., CRDs) that this StackInstall owns
// are also cleaned up.
func (h *stackInstallHandler) delete(ctx context.Context) (reconcile.Result, error) {
	// Delete all Stacks created by this StackInstall or ClusterStackInstall
	labels := stacks.ParentLabels(h.ext)
	if err := h.kube.DeleteAllOf(ctx, &v1alpha1.Stack{}, client.InNamespace(h.ext.GetNamespace()), client.MatchingLabels(labels)); runtimeresource.IgnoreNotFound(err) != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	// Waiting for all Stacks to clear their finalizers and delete before
	// deleting the CRDs that they depend on
	stackList := &v1alpha1.StackList{}
	if err := h.kube.List(ctx, stackList, client.MatchingLabels(labels)); err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	if len(stackList.Items) != 0 {
		err := errors.New("Stack resources have not been deleted")
		return fail(ctx, h.kube, h.ext, err)
	}

	stackControllerNamespace := h.ext.GetNamespace()
	if h.hostAwareConfig != nil {
		stackControllerNamespace = h.hostAwareConfig.HostControllerNamespace
	}

	// Once the Stacks are gone, we can remove install job associated with the StackInstall using hostKube since jobs
	// were deployed into host Kubernetes cluster.
	if err := h.hostKube.DeleteAllOf(ctx, &batchv1.Job{}, client.MatchingLabels(labels),
		client.InNamespace(stackControllerNamespace), client.PropagationPolicy(metav1.DeletePropagationForeground)); runtimeresource.IgnoreNotFound(err) != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	// Once the Stacks are gone, we can remove all of the CRDs associated
	// with the StackInstall
	if err := h.kube.DeleteAllOf(ctx, &apiextensionsv1beta1.CustomResourceDefinition{}, client.MatchingLabels(labels)); runtimeresource.IgnoreNotFound(err) != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	// And finally clear the StackInstall's own finalizer
	meta.RemoveFinalizer(h.ext, installFinalizer)
	if err := h.kube.Update(ctx, h.ext); err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	return reconcile.Result{}, nil
}

// ************************************************************************************************
// Helper functions
// ************************************************************************************************

// fail - helper function to set fail condition with reason and message
func fail(ctx context.Context, kube client.StatusClient, i v1alpha1.StackInstaller, err error) (reconcile.Result, error) {
	i.SetConditions(runtimev1alpha1.ReconcileError(err))
	return resultRequeue, kube.Status().Update(ctx, i)
}
