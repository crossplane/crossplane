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
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	azurecomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/compute/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
)

var (
	ctx           = context.Background()
	resultDone    = reconcile.Result{}
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
	azure.AKSClientsetFactoryAPI

	connect   func(*azurecomputev1alpha1.AKSCluster) (azure.AKSClientsetAPI, error)
	create    func(*azurecomputev1alpha1.AKSCluster, azure.AKSClientsetAPI) (reconcile.Result, error)
	createApp func(*azurecomputev1alpha1.AKSCluster, azure.AKSClientsetAPI) (string, string, error)
	delete    func(*azurecomputev1alpha1.AKSCluster, azure.AKSClientsetAPI) (reconcile.Result, error)
	sync      func(*azurecomputev1alpha1.AKSCluster, azure.AKSClientsetAPI) (reconcile.Result, error)
	secret    func(*azurecomputev1alpha1.AKSCluster, azure.AKSClientsetAPI) (*corev1.Secret, error)
	wait      func(*azurecomputev1alpha1.AKSCluster, azure.AKSClientsetAPI) (reconcile.Result, error)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &Reconciler{
		Client:                 mgr.GetClient(),
		scheme:                 mgr.GetScheme(),
		kubeclient:             kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:               mgr.GetRecorder(controllerName),
		AKSClientsetFactoryAPI: &azure.AKSClientsetFactory{},
	}

	r.connect = r._connect
	r.create = r._create
	r.createApp = r._createApp
	r.sync = r._sync
	r.delete = r._delete
	r.secret = r._secret
	r.wait = r._wait

	return r
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &azurecomputev1alpha1.AKSCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

const reasonCreateClusterClientFailure = "Failed to create cluster client"

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
			return resultDone, nil
		}
		// Error reading the object - requeue the request.
		return resultDone, err
	}

	// CreateApplication AKS AKSClientAPI
	aksClient, err := r.connect(instance)
	if err != nil {
		return r.fail(instance, reasonCreateClusterClientFailure, err.Error())
	}

	// Check for deletion
	if instance.DeletionTimestamp != nil {
		return r.delete(instance, aksClient)
	}

	// Add finalizer
	util.AddFinalizer(&instance.ObjectMeta, finalizer)

	// Check for running operations and wait if needed
	if instance.Status.RunningOperation != nil {
		return r.wait(instance, aksClient)
	}

	// CreateApplication cluster instance
	if instance.Status.ClusterName == "" {
		return r.create(instance, aksClient)
	}

	// Sync cluster instance status with cluster status
	return r.sync(instance, aksClient)
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *azurecomputev1alpha1.AKSCluster, reason, msg string) (reconcile.Result, error) {
	instance.Status.UnsetAllConditions()
	instance.Status.SetFailed(reason, msg)
	return resultRequeue, r.Update(context.TODO(), instance)
}

// _connect create AKS clientset using provider reference
func (r *Reconciler) _connect(instance *azurecomputev1alpha1.AKSCluster) (azure.AKSClientsetAPI, error) {
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

	return r.NewAKSClientset(config)
}

// _createApp - create AD Application with Service Principal
func (r *Reconciler) _createApp(instance *azurecomputev1alpha1.AKSCluster, client azure.AKSClientsetAPI) (string, string, error) {
	// check for existing application id - if exist: delete and recreate
	appID := instance.Status.ApplicationObjectID
	if appID != "" {
		if err := client.DeleteApplication(ctx, appID); err != nil && !azure.IsErrorNotFound(err) {
			return "", "", err
		}
	}

	// Generate new url for app using dns prefix and location
	url, err := applicationURL(instance.Spec.DNSNamePrefix, instance.Spec.Location)
	if err != nil {
		return "", "", err
	}

	// Generate new application password and save it as secret
	password := azure.NewPasswordCredential("rbac")
	if _, err := util.ApplySecret(r.kubeclient, applicationSecret(instance, []byte(*password.Value))); err != nil {
		return "", "", err
	}

	// Generate application name and create application
	name := fmt.Sprintf("%s%s", clusterNamePrefix, instance.UID)
	app, err := client.CreateApplication(ctx, name, url, password)
	if err != nil {
		return "", "", err
	}

	// Create Service Principal
	_, err = client.CreateServicePrincipal(ctx, *app.AppID)
	if err != nil {
		return "", "", err
	}

	instance.Status.ApplicationObjectID = *app.ObjectID

	// return back AD Application ID, and password
	return *app.AppID, *password.Value, nil
}

// reasonCreateClusterFailure to track failed create cluster calls
const reasonCreateClusterFailure = "Failed to create new cluster"

// _create new AKS cluster and required artifacts (AD App and SP)
func (r *Reconciler) _create(instance *azurecomputev1alpha1.AKSCluster, client azure.AKSClientsetAPI) (reconcile.Result, error) {
	// create AKS Cluster
	clusterName := strings.TrimSuffix(util.TrimToSize(fmt.Sprintf("%s%s", clusterNamePrefix, instance.UID), azure.MaxClusterNameLength), "-")
	instance.Status.ClusterName = clusterName
	instance.Status.UnsetAllConditions()
	instance.Status.SetCreating()

	// check if cluster with this name already exists
	cluster, err := client.GetCluster(ctx, instance.Spec.ResourceGroupName, clusterName)
	if err == nil {
		// this could be as a result of one of the following:
		// 1. We have an unlikely dirty read and this is a duplicate `create` event, log and event it since we want
		//    to keep track of these. Note: out-of-the-order write "update" should fail as well.
		// 2. We have collided on the cluster name (even more unlikely), handle it in the same way
		// in either case - fail this reconcile event
		msg := fmt.Sprintf("clsuter already exists: %v", cluster)
		log.Println(msg)
		r.recorder.Event(instance, corev1.EventTypeWarning, reasonCreateClusterFailure, msg)
		return r.fail(instance, reasonCreateClusterFailure, msg)
	}

	// cluster is not found - normal or expected case
	if !azure.IsErrorNotFound(err) {
		// other than non-found error
		return r.fail(instance, reasonCreateClusterFailure, err.Error())
	}

	// create AD Application
	appID, appPassword, err := r.createApp(instance, client)
	if err != nil {
		return r.fail(instance, reasonCreateClusterFailure, err.Error())
	}

	future, err := client.CreateCluster(ctx, clusterName, appID, appPassword, instance.Spec)
	if err != nil {
		if azure.IsErrorBadRequest(err) {
			instance.Status.SetFailed(reasonCreateClusterFailure, err.Error())
			// do not requeue on bad requests
			return resultDone, r.Update(ctx, instance)
		}
		return r.fail(instance, reasonCreateClusterFailure, err.Error())
	}

	instance.Status.State = azurecomputev1alpha1.ClusterStateCreating
	instance.Status.RunningOperation = future
	return resultDone, r.Update(ctx, instance)
}

// reasonDeleteClusterFailure to keep track of failed delete calls
const reasonDeleteClusterFailure = "Failed to delete cluster"

// _delete check reclaim policy and if needed delete the aks cluster resource
func (r *Reconciler) _delete(instance *azurecomputev1alpha1.AKSCluster, client azure.AKSClientsetAPI) (reconcile.Result, error) {
	if instance.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		status := instance.Status
		if err := client.Delete(ctx, instance.Spec.ResourceGroupName, status.ClusterName, status.ApplicationObjectID); err != nil {
			return r.fail(instance, reasonDeleteClusterFailure, err.Error())
		}
	}
	util.RemoveFinalizer(&instance.ObjectMeta, finalizer)
	instance.Status.UnsetAllConditions()
	instance.Status.SetDeleting()
	return resultDone, r.Update(ctx, instance)
}

// sync constants
const (
	syncWaitStatusCreating          = 10 * time.Second
	syncWaitStatusUnexpected        = 1 * time.Minute
	errorSyncFailCluster            = "Failed to sync cluster status"
	errorSyncUnexpectedClusterState = "Unexpected sync cluster state"
)

// _sync keep track of the cluster status and generate connection secret when cluster is ready
func (r *Reconciler) _sync(instance *azurecomputev1alpha1.AKSCluster, client azure.AKSClientsetAPI) (reconcile.Result, error) {
	cluster, err := client.GetCluster(ctx, instance.Spec.ResourceGroupName, instance.Status.ClusterName)
	if err != nil {
		return r.fail(instance, errorSyncFailCluster, err.Error())
	}

	switch util.StringValue(cluster.ProvisioningState) {

	case azurecomputev1alpha1.ClusterStateSucceeded:
		// continue

	case azurecomputev1alpha1.ClusterStateCreating:
		r.recorder.Event(instance, corev1.EventTypeWarning, errorSyncUnexpectedClusterState, fmt.Sprintf("waiting for %v", syncWaitStatusCreating))
		return reconcile.Result{RequeueAfter: syncWaitStatusCreating}, nil

	case azurecomputev1alpha1.ClusterStateFailed:
		if instance.Status.IsFailed() {
			return resultDone, nil
		}
		r.recorder.Event(instance, corev1.EventTypeWarning, errorSyncUnexpectedClusterState, "failed")
		instance.Status.SetFailed(errorSyncUnexpectedClusterState, "failed")
		return resultDone, r.Update(ctx, instance)

	default:
		if instance.Status.IsFailed() {
			return reconcile.Result{RequeueAfter: syncWaitStatusUnexpected}, nil
		}
		msg := fmt.Sprintf("unexpected cluster status: %s", util.StringValue(cluster.ProvisioningState))
		r.recorder.Event(instance, corev1.EventTypeWarning, errorSyncUnexpectedClusterState, msg)
		instance.Status.SetFailed(errorSyncFailCluster, msg)
		return reconcile.Result{RequeueAfter: syncWaitStatusUnexpected}, r.Update(ctx, instance)
	}

	// create connection secret
	secret, err := r.secret(instance, client)
	if err != nil {
		return r.fail(instance, errorSyncFailCluster, err.Error())
	}

	// save secret
	if _, err := util.ApplySecret(r.kubeclient, secret); err != nil {
		return r.fail(instance, errorSyncFailCluster, err.Error())
	}

	// update resource status
	instance.Status.Endpoint = util.StringValue(cluster.Fqdn)
	instance.Status.ProviderID = util.StringValue(cluster.ID)
	instance.Status.State = util.StringValue(cluster.ProvisioningState)

	if !instance.Status.IsReady() {
		instance.Status.UnsetAllConditions()
	}
	instance.Status.SetReady()

	return resultDone, r.Update(ctx, instance)
}

// _secret AKS connection credentials
func (r *Reconciler) _secret(instance *azurecomputev1alpha1.AKSCluster, client azure.AKSClientsetAPI) (*corev1.Secret, error) {
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

// wait constants
const (
	waitDelay        = 10 * time.Second
	reasonWaitFailed = "wait failed"
)

// _wait for cluster running operation (future) to complete
func (r *Reconciler) _wait(instance *azurecomputev1alpha1.AKSCluster, client azure.AKSClientsetAPI) (reconcile.Result, error) {
	// check on the status of operation
	done, err := client.DoneWithContext(ctx, instance.Status.RunningOperation)
	if err != nil {
		return r.fail(instance, reasonWaitFailed, err.Error())
	}

	if !done {
		r.recorder.Event(instance, corev1.EventTypeNormal, "waiting", "")
		return reconcile.Result{RequeueAfter: waitDelay}, nil
	}

	// check the future result to retrieve the detail error if any
	if rs, err := client.GetResult(instance.Status.RunningOperation); err != nil {
		// error getting the result (does not mean the result itself is a failure) - requeue
		r.recorder.Event(instance, corev1.EventTypeWarning, "get result error", err.Error())
		return resultDone, err
	} else if !(rs.StatusCode == http.StatusOK || rs.StatusCode == http.StatusCreated) {
		// result is not successful - dump response body into the event and log
		b, _ := ioutil.ReadAll(rs.Body)
		r.recorder.Event(instance, corev1.EventTypeWarning, "result not ok", string(b))
		log.Printf("AKS Operation failure: %s", b)
	}

	instance.Status.RunningOperation = nil
	return resultDone, r.Update(ctx, instance)
}

// applicationURL helper - returns URL string in format:
// https://ab12fa.dns.location.cloudapp.crossplane.io
// where 'ab12fa` is randomly generated hex string to avoid collisions
// dns - is provided dns prefix value
// location - is provided location string value.
// - *note*: location is transformed if needed (remove spaces and convert to lower-case)
func applicationURL(dnsPrefix, location string) (string, error) {
	location = strings.Replace(strings.ToLower(location), " ", "", -1)

	salt, err := util.GenerateHex(3)
	if err != nil {
		return "", err
	}
	// TODO: are we sure we want to add `crossplane.io` domain to the client AD Application URL?
	// - suggestion - use full dns value or suffix instead of prefix, i.e.
	// - dns = 'myapp.mydomain.io' -> https://ab12cd.locaiont.myapp.mydomain.io
	return fmt.Sprintf("https://%s.%s.%s.cloudapp.crossplane.io", salt, dnsPrefix, location), nil
}

// ApplicationSecret constants
const (
	ApplicationSecretFmt = "%s-service-principal"
	ApplicationSecretKey = "clientSecret"
)

// applicationSecret helper
func applicationSecret(instance *azurecomputev1alpha1.AKSCluster, value []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       instance.Namespace,
			Name:            fmt.Sprintf(ApplicationSecretFmt, instance.Name),
			OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
		},
		Data: map[string][]byte{
			ApplicationSecretKey: []byte(value),
		},
	}
}
