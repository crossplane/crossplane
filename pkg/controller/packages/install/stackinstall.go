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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	runtimeresource "github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane/apis/packages/v1alpha1"
	"github.com/crossplane/crossplane/pkg/controller/packages/hosted"
	"github.com/crossplane/crossplane/pkg/packages"
)

const (
	reconcileTimeout      = 1 * time.Minute
	requeueAfterOnSuccess = 10 * time.Second
	installFinalizer      = "finalizer.packageinstall.crossplane.io"
)

var (
	resultRequeue    = reconcile.Result{Requeue: true}
	requeueOnSuccess = reconcile.Result{RequeueAfter: requeueAfterOnSuccess}
)

// k8sClients holds the clients for Kubernetes
type k8sClients struct {
	// kube is controller runtime client for resource (a.k.a tenant) Kubernetes where all custom resources live.
	kube client.Client
	// hostKube is controller runtime client for workload (a.k.a host) Kubernetes where jobs for package installs and
	// package controller deployments/jobs created.
	hostKube client.Client
	// hostClient is client-go kubernetes client to read logs of package install pods.
	hostClient kubernetes.Interface
}

// Reconciler reconciles a Instance object
type Reconciler struct {
	sync.Mutex
	k8sClients
	hostedConfig             *hosted.Config
	packinator               func() v1alpha1.PackageInstaller
	executorInfoDiscoverer   packages.ExecutorInfoDiscoverer
	templatesControllerImage string
	forceImagePullPolicy     string
	log                      logging.Logger

	factory
}

// SetupClusterPackageInstall adds a controller that reconciles
// ClusterPackageInstalls.
func SetupClusterPackageInstall(mgr ctrl.Manager, l logging.Logger, hostControllerNamespace, tsControllerImage string) error {
	name := "packages/" + strings.ToLower(v1alpha1.ClusterPackageInstallGroupKind)
	packinator := func() v1alpha1.PackageInstaller { return &v1alpha1.ClusterPackageInstall{} }

	// Fail early if ClusterPackageInstall is not registered with the scheme.
	packinator()

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
		hostedConfig:             hc,
		packinator:               packinator,
		factory:                  &handlerFactory{},
		executorInfoDiscoverer:   &packages.KubeExecutorInfoDiscoverer{Client: hostKube},
		templatesControllerImage: tsControllerImage,
		log:                      l.WithValues("controller", name),
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.ClusterPackageInstall{}).
		Complete(r)
}

// SetupPackageInstall adds a controller that reconciles PackageInstalls.
func SetupPackageInstall(mgr ctrl.Manager, l logging.Logger, hostControllerNamespace, tsControllerImage, forceImagePullPolicy string) error {
	name := "packages/" + strings.ToLower(v1alpha1.PackageInstallGroupKind)
	packinator := func() v1alpha1.PackageInstaller { return &v1alpha1.PackageInstall{} }

	// Fail early if PackageInstall is not registered with the scheme.
	packinator()

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
		hostedConfig:             hc,
		packinator:               packinator,
		factory:                  &handlerFactory{},
		executorInfoDiscoverer:   &packages.KubeExecutorInfoDiscoverer{Client: hostKube},
		templatesControllerImage: tsControllerImage,
		forceImagePullPolicy:     forceImagePullPolicy,
		log:                      l.WithValues("controller", name),
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.PackageInstall{}).
		Complete(r)
}

// Reconcile reads that state of the PackageInstall for a Instance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	packageInstaller := r.packinator()
	r.log.Debug("Reconciling", "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	// fetch the CRD instance
	if err := r.kube.Get(ctx, req.NamespacedName, packageInstaller); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	meta.AddFinalizer(packageInstaller, installFinalizer)
	err := r.kube.Update(ctx, packageInstaller)
	if err != nil {
		return fail(ctx, r.kube, packageInstaller, err)
	}

	executorinfo, err := r.executorInfoDiscoverer.Discover(ctx)
	if err != nil {
		return fail(ctx, r.kube, packageInstaller, err)
	}

	handler := r.factory.newHandler(r.log, packageInstaller,
		k8sClients{
			kube:       r.kube,
			hostKube:   r.hostKube,
			hostClient: r.hostClient,
		},
		r.hostedConfig,
		executorinfo,
		r.templatesControllerImage,
		r.forceImagePullPolicy,
	)

	if meta.WasDeleted(packageInstaller) {
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

// packageInstallHandler is a concrete implementation of the handler interface
type packageInstallHandler struct {
	kube                     client.Client
	hostKube                 client.Client
	hostAwareConfig          *hosted.Config
	jobCompleter             jobCompleter
	executorInfo             *packages.ExecutorInfo
	ext                      v1alpha1.PackageInstaller
	templatesControllerImage string
	forceImagePullPolicy     string

	log logging.Logger
}

// factory is an interface for creating new handlers
type factory interface {
	newHandler(logging.Logger, v1alpha1.PackageInstaller, k8sClients, *hosted.Config, *packages.ExecutorInfo, string, string) handler
}

type handlerFactory struct{}

func (f *handlerFactory) newHandler(log logging.Logger, ext v1alpha1.PackageInstaller, k8s k8sClients, hostAwareConfig *hosted.Config, ei *packages.ExecutorInfo, templatesControllerImage, forceImagePullPolicy string) handler {

	return &packageInstallHandler{
		ext:             ext,
		kube:            k8s.kube,
		hostKube:        k8s.hostKube,
		hostAwareConfig: hostAwareConfig,
		executorInfo:    ei,
		jobCompleter: &packageInstallJobCompleter{
			client:     k8s.kube,
			hostClient: k8s.hostKube,
			podLogReader: &K8sReader{
				Client: k8s.hostClient,
			},
			log: log,
		},
		log:                      log,
		templatesControllerImage: templatesControllerImage,
		forceImagePullPolicy:     forceImagePullPolicy,
	}
}

// ************************************************************************************************
// Syncing/Creating functions
// ************************************************************************************************
func (h *packageInstallHandler) sync(ctx context.Context) (reconcile.Result, error) {
	sr := h.ext.PackageRecord()
	if sr == nil || sr.UID == "" {
		// If we observe the Package, InstallJob succeeded and we're done.
		nn := types.NamespacedName{Name: h.ext.GetName(), Namespace: h.ext.GetNamespace()}
		s := &v1alpha1.Package{}

		if err := h.kube.Get(ctx, nn, s); runtimeresource.IgnoreNotFound(err) != nil {
			return fail(ctx, h.kube, h.ext, err)
		} else if err == nil {
			// Set a reference to the Package record in PackageInstall status
			h.ext.SetPackageRecord(&corev1.ObjectReference{
				APIVersion: s.APIVersion,
				Kind:       s.Kind,
				Name:       s.Name,
				Namespace:  s.Namespace,
				UID:        s.ObjectMeta.UID,
			})
			h.ext.SetConditions(runtimev1alpha1.Available(), runtimev1alpha1.ReconcileSuccess())

			return requeueOnSuccess, h.kube.Status().Update(ctx, h.ext)
		}

		return h.create(ctx)
	}

	return h.update(ctx)
}

// create resources (Job, StackDefinition) that yield an associated Package
// An installjob will be created to unpack the package image. A Package or
// StackDefinition and CRDs should then be output. The output will be awaited
// before calling handleJobCompletion.
// A Package is expected to be created by handleJobCompletion or by
// StackDefinition reconciliation.
// PackageInstall sync will then assume this Package in its PackageRef.
func (h *packageInstallHandler) create(ctx context.Context) (reconcile.Result, error) {
	h.ext.SetConditions(runtimev1alpha1.Creating())

	packageInstallPatch := client.MergeFrom(h.ext.DeepCopyObject())
	meta.AddFinalizer(h.ext, installFinalizer)

	err := h.kube.Patch(ctx, h.ext, packageInstallPatch)
	if err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	// Create the InstallJob that will produce the CRDs, Package, and
	// StackDefinition
	jobRef := h.ext.InstallJob()

	if jobRef == nil {
		if jobRef, err = h.findOrCreateInstallJob(ctx); err != nil {
			return fail(ctx, h.kube, h.ext, err)
		}

		// Save a reference to the install job we just created or found
		h.ext.SetInstallJob(jobRef)
		h.ext.SetConditions(runtimev1alpha1.ReconcileSuccess())
		h.log.Debug("created install job", "jobRef", jobRef)

		return requeueOnSuccess, h.kube.Status().Patch(ctx, h.ext, packageInstallPatch)
	}

	return h.awaitInstallJob(ctx, jobRef)
}

// findOrCreateInstallJob finds or creates an install job for the packageinstall.
// In host-aware configurations this will also create host copies of any
// imagePullSecrets.
//
// if an install job with the expected name already exists, compare the labels
// (specifically parent labels). If they match, adopt this job - we must have
// failed to update the jobref on a previous reconciliation. if the install job
// does not belong to this packageinstall, block reconciliation and report it
// until the problem is resolved.
func (h *packageInstallHandler) findOrCreateInstallJob(ctx context.Context) (*corev1.ObjectReference, error) {
	var annotations map[string]string
	i := h.ext

	name := i.GetName()
	namespace := i.GetNamespace()
	imagePullSecrets := append([]corev1.LocalObjectReference{}, i.GetImagePullSecrets()...)
	pLabels := packages.ParentLabels(i)

	if hCfg := h.hostAwareConfig; hCfg != nil {
		singular := "packageinstall"
		if i.PermissionScope() == string(apiextensionsv1beta1.ClusterScoped) {
			singular = "clusterpackageinstall"
		}

		annotations = hosted.ObjectReferenceAnnotationsOnHost(singular, name, namespace)

		// Generate names for the image pull secrets that will reside on the
		// host. Secret names are required for the Job, but the Job UID is
		// required for the secrets. After creating the Job, the list of host
		// secret names can be paired with their original tenant secrets names
		// allowing the source secrets to be copied into their host counterpart.
		// UIDs suffixes are used in the host secret name to prevent the
		// guessing of names which could be abused by other tenant Package
		// Deployments.
		hostImagePullSecrets, err := hosted.ImagePullSecretsOnHost(namespace, imagePullSecrets)
		if err != nil {
			return nil, err
		}
		imagePullSecrets = hostImagePullSecrets

		// In Hosted Mode, we need to map all install jobs on tenant
		// Kubernetes into a single namespace on host cluster.
		hostJobRef := hCfg.ObjectReferenceOnHost(name, namespace)
		name = hostJobRef.Name
		namespace = hostJobRef.Namespace
	}

	job := &batchv1.Job{}
	existingJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}

	switch getErr := h.hostKube.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existingJob); {
	case getErr == nil:
		// a matching job was found, unless the labels conflict, its ours
		if labels.Conflicts(existingJob.GetLabels(), pLabels) {
			return nil, errors.Errorf("stale job %s/%s prevents packageinstall", existingJob.Namespace, existingJob.Name)
		}
		*job = *existingJob
	case kerrors.IsNotFound(getErr):
		// there is no install job created yet, create it now
		job = h.prepareInstallJob(name, namespace, pLabels, annotations, imagePullSecrets)

		if createErr := h.hostKube.Create(ctx, job); createErr != nil {
			return nil, createErr
		}
		h.debugWithName("created job", "uid", job.UID)
	case getErr != nil:
		return nil, getErr
	}

	// Set the GVK explicitly so APIGroup and Kind are not empty strings
	// within SyncImagePullSecrets, resulting in an invalid owner reference
	jobGVK := batchv1.SchemeGroupVersion.WithKind("Job")
	job.SetGroupVersionKind(jobGVK)
	jobRef := meta.ReferenceTo(job, jobGVK)

	if err := h.finalizeHostInstallJob(ctx, job); err != nil {
		return nil, err
	}

	return jobRef, nil
}

// finalizeHostInstallJob prepares a paused host install job to run by copying
// any necessary image pull secrets to the host and finally enabling the job
// with the host copies of the image pull secrets
func (h *packageInstallHandler) finalizeHostInstallJob(ctx context.Context, job *batchv1.Job) error {
	if h.hostAwareConfig == nil {
		return nil
	}

	err := hosted.SyncImagePullSecrets(ctx,
		h.kube,
		h.hostKube,
		h.ext.GetNamespace(),
		h.ext.GetImagePullSecrets(),
		job.Spec.Template.Spec.ImagePullSecrets,
		job)

	return err
}

func (h *packageInstallHandler) prepareInstallJob(name, namespace string, labels, annotations map[string]string, imagePullSecrets []corev1.LocalObjectReference) *batchv1.Job {
	i := h.ext
	executorInfo := h.executorInfo
	tscImage := h.templatesControllerImage

	pkg := i.GetPackage()
	img, err := i.ImageWithSource(pkg)
	if err != nil {
		// Applying the source is best-effort
		h.log.Debug("not applying packageinstall source to installjob image due to error", "pkg", pkg, "err", err)
		img = pkg
	}

	imagePullPolicy := corev1.PullPolicy(h.forceImagePullPolicy)
	if imagePullPolicy == "" {
		imagePullPolicy = i.GetImagePullPolicy()
	}

	return buildInstallJob(buildInstallJobParams{
		name:                     name,
		namespace:                namespace,
		permissionScope:          i.PermissionScope(),
		img:                      img,
		packageManagerImage:      executorInfo.Image,
		tscImage:                 tscImage,
		packageManagerPullPolicy: executorInfo.ImagePullPolicy,
		imagePullPolicy:          imagePullPolicy,
		labels:                   labels,
		annotations:              annotations,
		imagePullSecrets:         imagePullSecrets})
}

func (h *packageInstallHandler) awaitInstallJob(ctx context.Context, jobRef *corev1.ObjectReference) (reconcile.Result, error) {
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
				// the installjob succeeded, process the output
				if err := h.jobCompleter.handleJobCompletion(ctx, h.ext, job); err != nil {
					return fail(ctx, h.kube, h.ext, err)
				}

				// the installjob output was handled successfully
				h.ext.SetConditions(runtimev1alpha1.ReconcileSuccess())
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

func (h *packageInstallHandler) update(ctx context.Context) (reconcile.Result, error) {
	h.debugWithName("updating not supported yet")
	return reconcile.Result{}, nil
}

// delete performs clean up (finalizer) actions when a PackageInstall is being
// deleted. This function ensures that all the resources (e.g., CRDs) that this
// PackageInstall owns are also cleaned up.
func (h *packageInstallHandler) delete(ctx context.Context) (reconcile.Result, error) {
	labels := packages.ParentLabels(h.ext)

	for _, df := range []deleteReq{
		h.packageDefinitionDeleter(labels),
		// clear finalizers before deleting the Packages they may co-manage
		h.packageDefinitionFinalizerWaiter(labels),
		h.packageDeleter(labels),
		// clear finalizers before deleting the CRDs they depend on
		h.packageFinalizerWaiter(labels),
		// Once the Packages are gone, we can remove install job associated with
		// the PackageInstall using hostKube since jobs were deployed into host
		// Kubernetes cluster.
		h.installJobDeleter(labels),
		// Clear out orphaned CRDs and orphaned parent labels on CRDs
		h.deleteOrphanedCRDs,
		h.removeCRDParentLabels(labels),
		// And finally clear the PackageInstall's own finalizer
		h.removeFinalizer,
	} {
		if err := df(ctx); err != nil {
			return fail(ctx, h.kube, h.ext, err)
		}
	}

	meta.RemoveFinalizer(h.ext, installFinalizer)
	if err := h.kube.Update(ctx, h.ext); err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	return reconcile.Result{}, nil
}

type deleteReq func(context.Context) error

// packageDefinitionDeleter deletes all StackDefintions created by this
// PackageInstall or ClusterPackageInstall
func (h *packageInstallHandler) packageDefinitionDeleter(labels map[string]string) deleteReq {
	return func(ctx context.Context) error {
		return h.kube.DeleteAllOf(ctx, &v1alpha1.StackDefinition{}, client.InNamespace(h.ext.GetNamespace()), client.MatchingLabels(labels))
	}
}

// packageDefinitionFinalizerWaiter waits for all StackDefinitions to clear their
// finalizers and delete
func (h *packageInstallHandler) packageDefinitionFinalizerWaiter(labels map[string]string) deleteReq {
	return func(ctx context.Context) error {
		sdList := &v1alpha1.StackDefinitionList{}
		return h.kube.List(ctx, sdList, client.MatchingLabels(labels))
	}
}

// packageDeleter deletes all Packages created by this PackageInstall or ClusterPackageInstall
func (h *packageInstallHandler) packageDeleter(labels map[string]string) deleteReq {
	return func(ctx context.Context) error {
		return h.kube.DeleteAllOf(ctx, &v1alpha1.Package{}, client.InNamespace(h.ext.GetNamespace()), client.MatchingLabels(labels))
	}
}

// Waiting for all Packages to clear their finalizers and delete
func (h *packageInstallHandler) packageFinalizerWaiter(labels map[string]string) deleteReq {
	return func(ctx context.Context) error {
		packageList := &v1alpha1.PackageList{}
		if err := h.kube.List(ctx, packageList, client.MatchingLabels(labels)); err != nil {
			return err
		}

		if len(packageList.Items) != 0 {
			return errors.New("Package resources have not been deleted")
		}
		return nil
	}
}

// installJobDeleter deletes the install jobs belonging to the packageinstall
// found on the host client
func (h *packageInstallHandler) installJobDeleter(labels map[string]string) deleteReq {
	return func(ctx context.Context) error {
		packageControllerNamespace := h.ext.GetNamespace()
		if h.hostAwareConfig != nil {
			packageControllerNamespace = h.hostAwareConfig.HostControllerNamespace
		}

		return h.hostKube.DeleteAllOf(ctx, &batchv1.Job{}, client.MatchingLabels(labels),
			client.InNamespace(packageControllerNamespace), client.PropagationPolicy(metav1.DeletePropagationForeground))
	}
}

// deleteOrphanedCRDs will delete CRDs with managed-by label and NO package parent
// labels. we can't predict these names, so fetch all crds from any packageinstall
// and then locally filter out any crds that still contain labels indicating
// they are in use.
//
// TODO(displague) packageinstall delete and install can race and delete CRDs
// whose Package resources have not yet claimed the CRDs.
func (h *packageInstallHandler) deleteOrphanedCRDs(ctx context.Context) error {
	crds := &apiextensionsv1beta1.CustomResourceDefinitionList{}
	if err := h.kube.List(ctx, crds, client.MatchingLabels{packages.LabelKubernetesManagedBy: packages.LabelValuePackageManager}); err != nil {
		h.debugWithName("failed to list CRDs")
		return err
	}
	for i := range crds.Items {
		// check for LabelNamespacePrefix to avoid deleting CRDs
		// installed before LabelMultiParentPrefix was introduced
		if packages.HasPrefixedLabel(&crds.Items[i], packages.LabelMultiParentPrefix, packages.LabelNamespacePrefix) {
			continue
		}

		h.debugWithName("deleting orphaned CRD", "crd", crds.Items[i].GetName())
		if err := h.kube.Delete(ctx, &crds.Items[i]); runtimeresource.IgnoreNotFound(err) != nil {
			h.debugWithName("failed to delete CRD", "crd", crds.Items[i].GetName())
			return err
		}
	}
	return nil
}

// removeCRDParentLabels will remove unused ParentLabels from CRDs, these labels
// are no longer used on CRDs by Crossplane, replaced with multi parent labels
func (h *packageInstallHandler) removeCRDParentLabels(labels map[string]string) deleteReq {
	return func(ctx context.Context) error {
		crdList := &apiextensionsv1beta1.CustomResourceDefinitionList{}
		if err := h.kube.List(ctx, crdList, client.MatchingLabels(labels)); err != nil {
			return err
		} else if len(crdList.Items) > 0 {
			crds := crdList.Items
			for i := range crds {
				crdLabels := crds[i].GetLabels()
				crdPatch := client.MergeFrom(crds[i].DeepCopy())

				for label := range labels {
					delete(crdLabels, label)
				}

				h.log.Debug("removing parent labels from CRD", "parent", "name", crds[i].GetName())
				if err := h.kube.Patch(ctx, &crds[i], crdPatch); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

// removeFinalizer removes the finalizer from the PackageInstall resource
func (h *packageInstallHandler) removeFinalizer(ctx context.Context) error {
	meta.RemoveFinalizer(h.ext, installFinalizer)
	return h.kube.Update(ctx, h.ext)
}

// ************************************************************************************************
// Helper functions
// ************************************************************************************************

// debugWithName reduces the noise of Debug statements by including PackageInstall
// details. This helper shouldn't be used if packageinstall naming details are
// obvious and redundant to other debug parameters.
func (h *packageInstallHandler) debugWithName(msg string, keysAndValues ...interface{}) {
	kv := []interface{}{"namespace", h.ext.GetNamespace(), "name", h.ext.GetName()}
	h.log.Debug(msg, append(kv, keysAndValues...)...)
}

// fail - helper function to set fail condition with reason and message
func fail(ctx context.Context, kube client.StatusClient, i v1alpha1.PackageInstaller, err error) (reconcile.Result, error) {
	i.SetConditions(runtimev1alpha1.ReconcileError(err))
	return resultRequeue, kube.Status().Update(ctx, i)
}
