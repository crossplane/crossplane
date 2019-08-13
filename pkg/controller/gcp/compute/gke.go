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

package compute

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/api/container/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	gcpcomputev1alpha1 "github.com/crossplaneio/crossplane/gcp/apis/compute/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/gcp/apis/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp/gke"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/resource"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	controllerName    = "gke.compute.gcp.crossplane.io"
	finalizer         = "finalizer." + controllerName
	clusterNamePrefix = "gke-"

	requeueOnWait   = 30 * time.Second
	requeueOnSucces = 2 * time.Minute

	updateErrorMessageFormat = "failed to update cluster object: %s"
)

var (
	log           = logging.Logger.WithName("controller." + controllerName)
	ctx           = context.Background()
	result        = reconcile.Result{}
	resultRequeue = reconcile.Result{Requeue: true}
)

// Reconciler reconciles a Provider object
type Reconciler struct {
	client.Client
	scheme     *runtime.Scheme
	kubeclient kubernetes.Interface
	recorder   record.EventRecorder

	connect func(*gcpcomputev1alpha1.GKECluster) (gke.Client, error)
	create  func(*gcpcomputev1alpha1.GKECluster, gke.Client) (reconcile.Result, error)
	sync    func(*gcpcomputev1alpha1.GKECluster, gke.Client) (reconcile.Result, error)
	delete  func(*gcpcomputev1alpha1.GKECluster, gke.Client) (reconcile.Result, error)
}

// GKEClusterController is responsible for adding the GKECluster
// controller and its corresponding reconciler to the manager with any runtime configuration.
type GKEClusterController struct{}

// SetupWithManager creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (c *GKEClusterController) SetupWithManager(mgr ctrl.Manager) error {
	r := &Reconciler{
		Client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		kubeclient: kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:   mgr.GetEventRecorderFor(controllerName),
	}
	r.connect = r._connect
	r.create = r._create
	r.sync = r._sync
	r.delete = r._delete

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&gcpcomputev1alpha1.GKECluster{}).
		Complete(r)
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *gcpcomputev1alpha1.GKECluster, err error) (reconcile.Result, error) {
	instance.Status.SetConditions(corev1alpha1.ReconcileError(err))
	return resultRequeue, r.Update(context.TODO(), instance)
}

// connectionSecret return secret object for cluster instance
func (r *Reconciler) connectionSecret(instance *gcpcomputev1alpha1.GKECluster, cluster *container.Cluster) (*corev1.Secret, error) {
	secret := resource.ConnectionSecretFor(instance, gcpcomputev1alpha1.GKEClusterGroupVersionKind)

	secret.Data = map[string][]byte{
		corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(cluster.Endpoint),
		corev1alpha1.ResourceCredentialsSecretUserKey:     []byte(cluster.MasterAuth.Username),
		corev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(cluster.MasterAuth.Password),
	}

	val, err := base64.StdEncoding.DecodeString(cluster.MasterAuth.ClusterCaCertificate)
	if err != nil {
		return nil, err
	}
	secret.Data[corev1alpha1.ResourceCredentialsSecretCAKey] = val

	val, err = base64.StdEncoding.DecodeString(cluster.MasterAuth.ClientCertificate)
	if err != nil {
		return nil, err
	}
	secret.Data[corev1alpha1.ResourceCredentialsSecretClientCertKey] = val

	val, err = base64.StdEncoding.DecodeString(cluster.MasterAuth.ClientKey)
	if err != nil {
		return nil, err
	}
	secret.Data[corev1alpha1.ResourceCredentialsSecretClientKeyKey] = val

	return secret, nil
}

func (r *Reconciler) _connect(instance *gcpcomputev1alpha1.GKECluster) (gke.Client, error) {
	// Fetch Provider
	p := &gcpv1alpha1.Provider{}
	err := r.Get(ctx, meta.NamespacedNameOf(instance.Spec.ProviderReference), p)
	if err != nil {
		return nil, err
	}

	creds, err := gcp.ProviderCredentials(r.kubeclient, p, gke.DefaultScope)
	if err != nil {
		return nil, err
	}

	return gke.NewClusterClient(ctx, creds)
}

func (r *Reconciler) _create(instance *gcpcomputev1alpha1.GKECluster, client gke.Client) (reconcile.Result, error) {
	instance.Status.SetConditions(corev1alpha1.Creating())
	clusterName := fmt.Sprintf("%s%s", clusterNamePrefix, instance.UID)

	meta.AddFinalizer(instance, finalizer)

	_, err := client.CreateCluster(clusterName, instance.Spec)
	if err != nil && !gcp.IsErrorAlreadyExists(err) {
		if gcp.IsErrorBadRequest(err) {
			instance.Status.SetConditions(corev1alpha1.ReconcileError(err))
			// do not requeue on bad requests
			return result, r.Update(ctx, instance)
		}
		return r.fail(instance, err)
	}

	instance.Status.State = gcpcomputev1alpha1.ClusterStateProvisioning
	instance.Status.ClusterName = clusterName
	instance.Status.SetConditions(corev1alpha1.ReconcileSuccess())

	return reconcile.Result{}, errors.Wrapf(r.Update(ctx, instance), updateErrorMessageFormat, instance.GetName())
}

func (r *Reconciler) _sync(instance *gcpcomputev1alpha1.GKECluster, client gke.Client) (reconcile.Result, error) {
	cluster, err := client.GetCluster(instance.Spec.Zone, instance.Status.ClusterName)
	if err != nil {
		return r.fail(instance, err)
	}

	if cluster.Status != gcpcomputev1alpha1.ClusterStateRunning {
		return reconcile.Result{RequeueAfter: requeueOnWait}, nil
	}

	// create connection secret
	secret, err := r.connectionSecret(instance, cluster)
	if err != nil {
		return r.fail(instance, err)
	}

	// save secret
	if _, err := util.ApplySecret(r.kubeclient, secret); err != nil {
		return r.fail(instance, err)
	}

	// update resource status
	instance.Status.Endpoint = cluster.Endpoint
	instance.Status.State = gcpcomputev1alpha1.ClusterStateRunning
	instance.Status.SetConditions(corev1alpha1.Available(), corev1alpha1.ReconcileSuccess())
	resource.SetBindable(instance)

	return reconcile.Result{RequeueAfter: requeueOnSucces},
		errors.Wrapf(r.Update(ctx, instance), updateErrorMessageFormat, instance.GetName())
}

// _delete check reclaim policy and if needed delete the gke cluster resource
func (r *Reconciler) _delete(instance *gcpcomputev1alpha1.GKECluster, client gke.Client) (reconcile.Result, error) {
	instance.Status.SetConditions(corev1alpha1.Deleting())
	if instance.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		if err := client.DeleteCluster(instance.Spec.Zone, instance.Status.ClusterName); err != nil {
			return r.fail(instance, err)
		}
	}
	meta.RemoveFinalizer(instance, finalizer)
	instance.Status.SetConditions(corev1alpha1.ReconcileSuccess())
	return result, errors.Wrapf(r.Update(ctx, instance), updateErrorMessageFormat, instance.GetName())
}

// Reconcile reads that state of the cluster for a Provider object and makes changes based on the state read
// and what is in the Provider.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", gcpcomputev1alpha1.GKEClusterKindAPIVersion, "request", request)
	// Fetch the Provider instance
	instance := &gcpcomputev1alpha1.GKECluster{}
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Create GKE Client
	gkeClient, err := r.connect(instance)
	if err != nil {
		return r.fail(instance, err)
	}

	// Check for deletion
	if instance.DeletionTimestamp != nil {
		return r.delete(instance, gkeClient)
	}

	// Create cluster instance
	if instance.Status.ClusterName == "" {
		return r.create(instance, gkeClient)
	}

	// Sync cluster instance status with cluster status
	return r.sync(instance, gkeClient)
}
