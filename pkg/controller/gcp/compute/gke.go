/*
Copyright 2018 The Crossplane Authors.

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

	"github.com/pkg/errors"
	"google.golang.org/api/container/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	gcpcomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/compute/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp/gke"
	"github.com/crossplaneio/crossplane/pkg/controller/core"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	controllerName    = "gke.compute.gcp.crossplane.io"
	finalizer         = "finalizer." + controllerName
	clusterNamePrefix = "gke-"

	errorClusterClient = "Failed to create cluster client"
	errorCreateCluster = "Failed to create new cluster"
	errorSyncCluster   = "Failed to sync cluster state"
	errorDeleteCluster = "Failed to delete cluster"

	requeueOnWait   = core.RequeueOnWait
	requeueOnSucces = core.RequeueOnSuccess

	updateErrorMessageFormat = "failed to update cluster object: %s"
)

var (
	log           = logging.Logger.WithName("controller." + controllerName)
	ctx           = context.Background()
	result        = reconcile.Result{}
	resultRequeue = reconcile.Result{Requeue: true}
)

// Add creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

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

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &Reconciler{
		Client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		kubeclient: kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:   mgr.GetRecorder(controllerName),
	}
	r.connect = r._connect
	r.create = r._create
	r.sync = r._sync
	r.delete = r._delete
	return r
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Provider
	err = c.Watch(&source.Kind{Type: &gcpcomputev1alpha1.GKECluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *gcpcomputev1alpha1.GKECluster, reason, msg string) (reconcile.Result, error) {
	instance.Status.UnsetAllDeprecatedConditions()
	instance.Status.SetFailed(reason, msg)
	return resultRequeue, r.Update(context.TODO(), instance)
}

// connectionSecret return secret object for cluster instance
func (r *Reconciler) connectionSecret(instance *gcpcomputev1alpha1.GKECluster, cluster *container.Cluster) (*corev1.Secret, error) {
	secret := instance.ConnectionSecret()

	data := make(map[string][]byte)
	data[corev1alpha1.ResourceCredentialsSecretEndpointKey] = []byte(cluster.Endpoint)
	data[corev1alpha1.ResourceCredentialsSecretUserKey] = []byte(cluster.MasterAuth.Username)
	data[corev1alpha1.ResourceCredentialsSecretPasswordKey] = []byte(cluster.MasterAuth.Password)

	val, err := base64.StdEncoding.DecodeString(cluster.MasterAuth.ClusterCaCertificate)
	if err != nil {
		return nil, err
	}
	data[corev1alpha1.ResourceCredentialsSecretCAKey] = val

	val, err = base64.StdEncoding.DecodeString(cluster.MasterAuth.ClientCertificate)
	if err != nil {
		return nil, err
	}
	data[corev1alpha1.ResourceCredentialsSecretClientCertKey] = val

	val, err = base64.StdEncoding.DecodeString(cluster.MasterAuth.ClientKey)
	if err != nil {
		return nil, err
	}
	data[corev1alpha1.ResourceCredentialsSecretClientKeyKey] = val

	secret.Data = data

	return secret, nil
}

func (r *Reconciler) _connect(instance *gcpcomputev1alpha1.GKECluster) (gke.Client, error) {
	// Fetch Provider
	p := &gcpv1alpha1.Provider{}
	providerNamespacedName := types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.ProviderRef.Name,
	}
	err := r.Get(ctx, providerNamespacedName, p)
	if err != nil {
		return nil, err
	}

	creds, err := gcp.ProviderCredentials(r.kubeclient, p, gke.DefaultScope)
	if err != nil {
		return nil, err
	}

	return gke.NewClusterClient(creds)
}

func (r *Reconciler) _create(instance *gcpcomputev1alpha1.GKECluster, client gke.Client) (reconcile.Result, error) {
	clusterName := fmt.Sprintf("%s%s", clusterNamePrefix, instance.UID)

	util.AddFinalizer(&instance.ObjectMeta, finalizer)

	_, err := client.CreateCluster(clusterName, instance.Spec)
	if err != nil && !gcp.IsErrorAlreadyExists(err) {
		if gcp.IsErrorBadRequest(err) {
			instance.Status.SetFailed(errorCreateCluster, err.Error())
			// do not requeue on bad requests
			return result, r.Update(ctx, instance)
		}
		return r.fail(instance, errorCreateCluster, err.Error())
	}

	instance.Status.State = gcpcomputev1alpha1.ClusterStateProvisioning
	instance.Status.UnsetAllDeprecatedConditions()
	instance.Status.SetCreating()
	instance.Status.ClusterName = clusterName

	return reconcile.Result{}, errors.Wrapf(r.Update(ctx, instance), updateErrorMessageFormat, instance.GetName())
}

func (r *Reconciler) _sync(instance *gcpcomputev1alpha1.GKECluster, client gke.Client) (reconcile.Result, error) {
	cluster, err := client.GetCluster(instance.Spec.Zone, instance.Status.ClusterName)
	if err != nil {
		return r.fail(instance, errorSyncCluster, err.Error())
	}

	if cluster.Status != gcpcomputev1alpha1.ClusterStateRunning {
		return reconcile.Result{RequeueAfter: requeueOnWait}, nil
	}

	// create connection secret
	secret, err := r.connectionSecret(instance, cluster)
	if err != nil {
		return r.fail(instance, errorSyncCluster, err.Error())
	}

	// save secret
	if _, err := util.ApplySecret(r.kubeclient, secret); err != nil {
		return r.fail(instance, errorSyncCluster, err.Error())
	}

	// update resource status
	instance.Status.Endpoint = cluster.Endpoint
	instance.Status.State = gcpcomputev1alpha1.ClusterStateRunning
	instance.Status.UnsetAllDeprecatedConditions()
	instance.Status.SetReady()

	return reconcile.Result{RequeueAfter: requeueOnSucces},
		errors.Wrapf(r.Update(ctx, instance), updateErrorMessageFormat, instance.GetName())

}

// _delete check reclaim policy and if needed delete the gke cluster resource
func (r *Reconciler) _delete(instance *gcpcomputev1alpha1.GKECluster, client gke.Client) (reconcile.Result, error) {
	if instance.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		if err := client.DeleteCluster(instance.Spec.Zone, instance.Status.ClusterName); err != nil {
			return r.fail(instance, errorDeleteCluster, err.Error())
		}
	}
	util.RemoveFinalizer(&instance.ObjectMeta, finalizer)
	instance.Status.UnsetAllDeprecatedConditions()
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
		return r.fail(instance, errorClusterClient, err.Error())
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
