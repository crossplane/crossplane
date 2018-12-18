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
	"fmt"
	"log"

	azurecomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/compute/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/clients/azure/aks"
	"github.com/crossplaneio/crossplane/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName    = "aks.compute.azure.crossplane.io"
	finalizer         = "finalizer." + controllerName
	clusterNamePrefix = "aks-"

	errorClusterClient = "Failed to create cluster client"
	errorCreateCluster = "Failed to create new cluster"
	errorSyncCluster   = "Failed to sync cluster state"
	errorDeleteCluster = "Failed to delete cluster"
)

var (
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

	connect func(*azurecomputev1alpha1.AKSCluster) (aks.Client, error)
	create  func(*azurecomputev1alpha1.AKSCluster, aks.Client) (reconcile.Result, error)
	delete  func(*azurecomputev1alpha1.AKSCluster, aks.Client) (reconcile.Result, error)
	sync    func(*azurecomputev1alpha1.AKSCluster, aks.Client) (reconcile.Result, error)
	secret  func(*azurecomputev1alpha1.AKSCluster, aks.Client) (*corev1.Secret, error)
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
	r.secret = r._secret
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
	err = c.Watch(&source.Kind{Type: &azurecomputev1alpha1.AKSCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a Provider object and makes changes based on the state read
// and what is in the Provider.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("reconciling %s: %v", azurecomputev1alpha1.AKSClusterKindAPIVersion, request)
	// Fetch the Provider instance
	instance := &azurecomputev1alpha1.AKSCluster{}
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
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

	// Add finalizer
	util.AddFinalizer(&instance.ObjectMeta, finalizer)

	// Create cluster instance
	if instance.Status.ClusterName == "" {
		return r.create(instance, gkeClient)
	}

	// Sync cluster instance status with cluster status
	return r.sync(instance, gkeClient)
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *azurecomputev1alpha1.AKSCluster, reason, msg string) (reconcile.Result, error) {
	instance.Status.UnsetAllConditions()
	instance.Status.SetFailed(reason, msg)
	return resultRequeue, r.Update(context.TODO(), instance)
}

func (r *Reconciler) _connect(instance *azurecomputev1alpha1.AKSCluster) (aks.Client, error) {
	// Fetch Provider
	p := &azurev1alpha1.Provider{}
	providerNamespacedName := types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.ProviderRef.Name,
	}
	err := r.Get(ctx, providerNamespacedName, p)
	if err != nil {
		return nil, err
	}

	// Check provider status
	if !p.IsValid() {
		return nil, fmt.Errorf("provider status is invalid")
	}

	data, err := util.SecretData(r.kubeclient, p.Namespace, p.Spec.Secret)
	if err != nil {
		return nil, err
	}

	config, err := azure.NewClientCredentialsConfig(data)
	if err != nil {
		return nil, err
	}

	return aks.NewAKSClient(config)
}

func (r *Reconciler) _create(instance *azurecomputev1alpha1.AKSCluster, client aks.Client) (reconcile.Result, error) {
	clusterName := fmt.Sprintf("%s%s", clusterNamePrefix, instance.UID)

	cluster, err := client.Create(clusterName, instance.Spec)
	if err != nil {
		if azure.IsErrorBadRequest(err) {
			instance.Status.SetFailed(errorCreateCluster, err.Error())
			// do not requeue on bad requests
			return result, r.Update(ctx, instance)
		}
		return r.fail(instance, errorCreateCluster, err.Error())
	}

	instance.Status.UnsetAllConditions()
	instance.Status.SetCreating()
	instance.Status.ClusterName = util.StringValue(cluster.Name)
	instance.Status.State = azurecomputev1alpha1.ClusterStateProvisioning

	return result, r.Update(ctx, instance)
}

func (r *Reconciler) _sync(instance *azurecomputev1alpha1.AKSCluster, client aks.Client) (reconcile.Result, error) {
	cluster, err := client.Get(instance.Spec.ResourceGroupName, instance.Status.ClusterName)
	if err != nil {
		return r.fail(instance, errorSyncCluster, err.Error())
	}

	if util.StringValue(cluster.ProvisioningState) != azurecomputev1alpha1.ClusterStateSucceeded {
		return resultRequeue, nil
	}

	// create connection secret
	secret, err := r.secret(instance, client)
	if err != nil {
		return r.fail(instance, errorSyncCluster, err.Error())
	}

	// save secret
	if _, err := util.ApplySecret(r.kubeclient, secret); err != nil {
		return r.fail(instance, errorSyncCluster, err.Error())
	}

	// update resource status
	instance.Status.Endpoint = util.StringValue(cluster.Fqdn)
	instance.Status.ProviderID = util.StringValue(cluster.ID)
	instance.Status.State = util.StringValue(cluster.ProvisioningState)

	if !instance.Status.IsReady() {
		instance.Status.UnsetAllConditions()
		instance.Status.SetReady()
	}

	// TODO: figure out how we going to handle cluster statuses other than RUNNING
	return result, r.Update(ctx, instance)
}

func (r *Reconciler) _secret(instance *azurecomputev1alpha1.AKSCluster, client aks.Client) (*corev1.Secret, error) {
	creds, err := client.ListCredentials(instance.Spec.ResourceGroupName, instance.Status.ClusterName)
	if err != nil {
		return nil, err
	}

	config, err := clientcmd.Load(*creds.Value)
	if err != nil {
		return nil, err
	}

	// cluster name - used as a context key
	name := instance.Status.ClusterName

	kubeContext, ok := config.Contexts[name]
	if !ok {
		return nil, fmt.Errorf("context configuration is not found for cluster: %s", name)
	}
	cluster, ok := config.Clusters[kubeContext.Cluster]
	if !ok {
		return nil, fmt.Errorf("cluster configuration is not found: %s", kubeContext.Cluster)
	}
	auth, ok := config.AuthInfos[kubeContext.AuthInfo]
	if !ok {
		return nil, fmt.Errorf("auth-info configuration is not found: %s", kubeContext.AuthInfo)
	}

	// create the connection secret with the credentials data now
	connectionSecret := instance.ConnectionSecret()
	data := make(map[string][]byte)
	data[corev1alpha1.ResourceCredentialsSecretEndpointKey] = []byte(cluster.Server)
	data[corev1alpha1.ResourceCredentialsSecretCAKey] = cluster.CertificateAuthorityData
	data[corev1alpha1.ResourceCredentialsSecretClientCertKey] = auth.ClientCertificateData
	data[corev1alpha1.ResourceCredentialsSecretClientKeyKey] = auth.ClientKeyData
	connectionSecret.Data = data

	return connectionSecret, nil
}

// _delete check reclaim policy and if needed delete the gke cluster resource
func (r *Reconciler) _delete(instance *azurecomputev1alpha1.AKSCluster, client aks.Client) (reconcile.Result, error) {
	if instance.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		if err := client.Delete(instance.Spec.ResourceGroupName, instance.Status.ClusterName); err != nil {
			return r.fail(instance, errorDeleteCluster, err.Error())
		}
	}
	util.RemoveFinalizer(&instance.ObjectMeta, finalizer)
	instance.Status.UnsetAllConditions()
	instance.Status.SetDeleting()
	return result, r.Update(ctx, instance)
}
