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

	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/compute/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	azureclients "github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/util"
	"github.com/google/uuid"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "aks.compute.azure.crossplane.io"
	finalizer      = "finalizer." + controllerName
	spSecretFmt    = "%s-service-principal"
	spSecretKey    = "clientSecret"
	adAppNameFmt   = "%s-crossplane-aks-app"

	errorClusterClient   = "Failed to create AKS cluster client"
	errorCreatingCluster = "failed to create cluster"
	errorSyncingCluster  = "Failed to sync cluster state"
	errorDeletingCluster = "Failed to delete cluster"
)

var (
	_             reconcile.Reconciler = &Reconciler{}
	ctx                                = context.Background()
	result                             = reconcile.Result{}
	resultRequeue                      = reconcile.Result{Requeue: true}
)

// Reconciler reconciles a AKSCluster object
type Reconciler struct {
	client.Client
	clientset          kubernetes.Interface
	aksSetupAPIFactory azureclients.AKSSetupAPIFactory
	config             *rest.Config
	scheme             *runtime.Scheme
}

// AddAKSCluster creates a new AKSCluster Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func AddAKSCluster(mgr manager.Manager) error {
	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create clientset: %+v", err)
	}

	r := newAKSClusterReconciler(mgr, &azureclients.AKSSetupClientFactory{}, clientset)
	return AddAKSClusterReconciler(mgr, r)
}

// newAKSClusterReconciler returns a new reconcile.Reconciler
func newAKSClusterReconciler(mgr manager.Manager, aksSetupAPIFactory azureclients.AKSSetupAPIFactory,
	clientset kubernetes.Interface) *Reconciler {

	return &Reconciler{
		Client:             mgr.GetClient(),
		clientset:          clientset,
		aksSetupAPIFactory: aksSetupAPIFactory,
		scheme:             mgr.GetScheme(),
	}
}

// AddAKSClusterReconciler adds a new Controller to mgr with r as the reconcile.Reconciler
func AddAKSClusterReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("AKSCluster-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to AKSCluster
	err = c.Watch(&source.Kind{Type: &computev1alpha1.AKSCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a AKSCluster object and makes changes based on the state read
// and what is in its spec.
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the CRD instance
	instance := &computev1alpha1.AKSCluster{}
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

	// Create AKS Client
	aksClient, err := r.connect(instance)
	if err != nil {
		return r.fail(instance, errorClusterClient, err.Error())
	}

	// Check for deletion
	if instance.DeletionTimestamp != nil {
		if instance.Status.Condition(corev1alpha1.Deleting) == nil {
			// we haven't started the deletion of the AKS cluster yet, do it now
			log.Printf("AKS cluster %s has been deleted, running finalizer now", instance.Name)
			return r.delete(instance, aksClient)
		}
		// we already started the deletion of the AKS cluster, nothing more to do
		return result, nil
	}

	// Add finalizer
	if !util.HasFinalizer(&instance.ObjectMeta, finalizer) {
		util.AddFinalizer(&instance.ObjectMeta, finalizer)
		if err := r.Update(ctx, instance); err != nil {
			return resultRequeue, err
		}
	}

	if instance.Status.RunningOperation != "" {
		// there is a running operation on the instance, wait for it to complete
		return r.waitForCompletion(instance, aksClient)
	}

	// Create cluster instance
	if !r.created(instance) {
		return r.create(instance, aksClient)
	}

	// Sync cluster instance status with cluster status
	return r.sync(instance, aksClient)
}

func (r *Reconciler) connect(instance *computev1alpha1.AKSCluster) (*azureclients.AKSSetupClient, error) {
	// Fetch Provider
	p := &azurev1alpha1.Provider{}
	providerNamespacedName := types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.ProviderRef.Name,
	}
	err := r.Get(ctx, providerNamespacedName, p)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %+v", err)
	}

	// Check provider status
	if !p.IsValid() {
		return nil, fmt.Errorf("provider status is invalid: %+v", p)
	}

	return r.aksSetupAPIFactory.CreateSetupClient(p, r.clientset)
}

func (r *Reconciler) create(instance *computev1alpha1.AKSCluster, aksClient *azureclients.AKSSetupClient) (reconcile.Result, error) {
	// create or fetch the secret for the AD application and its service principal the cluster will use for Azure APIs
	spSecret, err := r.servicePrincipalSecret(instance)
	if err != nil {
		return r.fail(instance, errorCreatingCluster, fmt.Sprintf("failed to get service principal secret for AKS cluster %s: %+v", instance.Name, err))
	}

	// create the AD application that the cluster will use for the Azure APIs
	appParams := azureclients.ApplicationParameters{
		Name:          fmt.Sprintf(adAppNameFmt, instance.Name),
		DNSNamePrefix: instance.Spec.DNSNamePrefix,
		Location:      instance.Spec.Location,
		ObjectID:      instance.Status.ApplicationObjectID,
		ClientSecret:  spSecret,
	}
	log.Printf("starting create of app for AKS cluster %s", instance.Name)
	app, err := aksClient.ApplicationAPI.CreateApplication(ctx, appParams)
	if err != nil {
		return r.fail(instance, errorCreatingCluster, fmt.Sprintf("failed to create app for AKS cluster %s: %+v", instance.Name, err))
	}

	if instance.Status.ApplicationObjectID == "" {
		// save the application object ID on the CRD status now
		instance.Status.ApplicationObjectID = *app.ObjectID
		// TODO: retry this CRD update upon conflict
		r.Update(ctx, instance)
	}

	// create the service principal for the AD application
	log.Printf("starting create of service principal for AKS cluster %s", instance.Name)
	sp, err := aksClient.ServicePrincipalAPI.CreateServicePrincipal(ctx, instance.Status.ServicePrincipalID, *app.AppID)
	if err != nil {
		return r.fail(instance, errorCreatingCluster, fmt.Sprintf("failed to create service principal for AKS cluster %s: %+v", instance.Name, err))
	}

	if instance.Status.ServicePrincipalID == "" {
		// save the service principal ID on the CRD status now
		instance.Status.ServicePrincipalID = *sp.ObjectID
		// TODO: retry this CRD update upon conflict
		r.Update(ctx, instance)
	}

	// start the creation of the AKS cluster
	log.Printf("starting create of AKS cluster %s", instance.Name)
	clusterName := azureclients.SanitizeClusterName(instance.Name)
	createOp, err := aksClient.AKSClusterAPI.CreateOrUpdateBegin(ctx, *instance, clusterName, *app.AppID, spSecret)
	if err != nil {
		return r.fail(instance, errorCreatingCluster, fmt.Sprintf("failed to start create operation for AKS cluster %s: %+v", instance.Name, err))
	}

	log.Printf("started create of AKS cluster %s, operation: %s", instance.Name, string(createOp))

	// save the create operation to the CRD status
	instance.Status.RunningOperation = string(createOp)

	// set the creating/provisioning state to the CRD status
	instance.Status.UnsetAllConditions()
	instance.Status.SetCreating()
	instance.Status.ClusterName = clusterName

	// wait until the important status fields we just set have become committed/consistent
	updateWaitErr := wait.ExponentialBackoff(util.DefaultUpdateRetry, func() (done bool, err error) {
		if err := r.Update(ctx, instance); err != nil {
			return false, nil
		}

		// the update went through, let's do a get to verify the fields are committed/consistent
		fetchedInstance := &computev1alpha1.AKSCluster{}
		if err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, fetchedInstance); err != nil {
			return false, nil
		}

		if fetchedInstance.Status.RunningOperation != "" && fetchedInstance.Status.ClusterName != "" {
			// both the running operation field and the cluster name field have been committed, we can stop retrying
			return true, nil
		}

		// the instance hasn't reached consistency yet, retry
		log.Printf("AKS cluster %s hasn't reached consistency yet, retrying", instance.Name)
		return false, nil
	})

	return resultRequeue, updateWaitErr
}

func (r *Reconciler) waitForCompletion(instance *computev1alpha1.AKSCluster, aksClient *azureclients.AKSSetupClient) (reconcile.Result, error) {
	// check if the operation is done yet and if there was any error
	done, err := aksClient.AKSClusterAPI.CreateOrUpdateEnd([]byte(instance.Status.RunningOperation))
	if !done {
		// not done yet, check again on the next reconcile
		log.Printf("waiting on create of AKS cluster %s, err: %+v", instance.Name, err)
		return resultRequeue, err
	}

	// the operation is done, clear out the running operation on the CRD status
	instance.Status.RunningOperation = ""

	if err != nil {
		// the operation completed, but there was an error
		return r.fail(instance, errorCreatingCluster, fmt.Sprintf("failure result returned from create operation for AKS cluster %s: %+v", instance.Name, err))
	}

	log.Printf("AKS cluster %s successfully created", instance.Name)
	return resultRequeue, r.Update(ctx, instance)
}

func (r *Reconciler) created(instance *computev1alpha1.AKSCluster) bool {
	return instance.Status.ClusterName != ""
}

func (r *Reconciler) sync(instance *computev1alpha1.AKSCluster, aksClient *azureclients.AKSSetupClient) (reconcile.Result, error) {
	cluster, err := aksClient.AKSClusterAPI.Get(ctx, *instance)
	if err != nil {
		return r.fail(instance, errorSyncingCluster, err.Error())
	}

	// create the connection secret
	if err := r.setConnectionSecret(instance, aksClient); err != nil {
		return r.fail(instance, errorSyncingCluster, fmt.Sprintf("failed to set connection secret for AKS cluster %s: %+v", instance.Name, err))
	}

	// update resource status
	if cluster.ID != nil {
		instance.Status.ProviderID = *cluster.ID
	}
	if cluster.ProvisioningState != nil {
		instance.Status.State = *cluster.ProvisioningState
	}
	if cluster.Fqdn != nil {
		instance.Status.Endpoint = *cluster.Fqdn
	}
	if instance.IsAvailable() {
		instance.Status.UnsetAllConditions()
		instance.Status.SetReady()
	}

	return result, r.Update(ctx, instance)
}

// delete performs a deletion of the AKS cluster if needed
func (r *Reconciler) delete(instance *computev1alpha1.AKSCluster, aksClient *azureclients.AKSSetupClient) (reconcile.Result, error) {
	if instance.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		// delete the AKS cluster
		log.Printf("deleting AKS cluster %s", instance.Name)
		deleteFuture, err := aksClient.AKSClusterAPI.Delete(ctx, *instance)
		if err != nil && !azureclients.IsNotFound(err) {
			return r.fail(instance, errorDeletingCluster, fmt.Sprintf("failed to delete AKS cluster %s: %+v", instance.Name, err))
		}
		deleteFutureJSON, _ := deleteFuture.MarshalJSON()
		log.Printf("started delete of AKS cluster %s, operation: %s", instance.Name, string(deleteFutureJSON))

		// delete the service principal
		log.Printf("deleting service principal for AKS cluster %s", instance.Name)
		err = aksClient.ServicePrincipalAPI.DeleteServicePrincipal(ctx, instance.Status.ServicePrincipalID)
		if err != nil && !azureclients.IsNotFound(err) {
			return r.fail(instance, errorDeletingCluster, fmt.Sprintf("failed to service principal: %+v", err))
		}

		// delete the AD application
		log.Printf("deleting app for AKS cluster %s", instance.Name)
		err = aksClient.ApplicationAPI.DeleteApplication(ctx, instance.Status.ApplicationObjectID)
		if err != nil && !azureclients.IsNotFound(err) {
			return r.fail(instance, errorDeletingCluster, fmt.Sprintf("failed to AD application: %+v", err))
		}

		log.Printf("all resources deleted for AKS cluster %s", instance.Name)
	}

	util.RemoveFinalizer(&instance.ObjectMeta, finalizer)
	instance.Status.UnsetAllConditions()
	instance.Status.SetDeleting()
	return result, r.Update(ctx, instance)
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *computev1alpha1.AKSCluster, reason, msg string) (reconcile.Result, error) {
	log.Printf("%s: %s", reason, msg)
	instance.Status.UnsetAllConditions()
	instance.Status.SetFailed(reason, msg)
	return resultRequeue, r.Update(ctx, instance)
}

func (r *Reconciler) servicePrincipalSecret(instance *computev1alpha1.AKSCluster) (string, error) {
	// check to see if the secret already exists
	secretName := fmt.Sprintf(spSecretFmt, instance.Name)
	selector := v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: secretName}, Key: spSecretKey}
	spSecretValue, err := util.SecretData(r.clientset, instance.Namespace, selector)
	if err == nil {
		return string(spSecretValue), nil
	}

	// service principal secret must not exist yet, generate a new one
	newSPSecretValue, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("failed to generate client secret: %+v", err)
	}

	// save the service principal secret
	spSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            secretName,
			Namespace:       instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
		},
		Data: map[string][]byte{spSecretKey: []byte(newSPSecretValue.String())},
	}

	if _, err := r.clientset.CoreV1().Secrets(instance.Namespace).Create(spSecret); err != nil {
		return "", fmt.Errorf("failed to create service principal secret: %+v", err)
	}

	return newSPSecretValue.String(), nil
}

func (r *Reconciler) setConnectionSecret(instance *computev1alpha1.AKSCluster, aksClient *azureclients.AKSSetupClient) error {
	secretName := instance.ConnectionSecretName()
	_, err := r.clientset.CoreV1().Secrets(instance.Namespace).Get(secretName, metav1.GetOptions{})
	if err == nil {
		// secret is already created, nothing more to do
		return nil
	} else if !errors.IsNotFound(err) {
		return err
	}

	// the secret doesn't exist yet, create it now. First look up the cluster's admin credentials
	clusterCreds, err := aksClient.AKSClusterAPI.ListClusterAdminCredentials(ctx, *instance)
	if err != nil {
		return err
	}

	if clusterCreds.Kubeconfigs == nil || len(*clusterCreds.Kubeconfigs) == 0 {
		return fmt.Errorf("zero kubeconfig credentials returned")
	}

	kubeconfigs := *clusterCreds.Kubeconfigs
	kubeconfigData := map[string][]byte{
		corev1alpha1.ResourceCredentialsSecretKubeconfigFileKey: *kubeconfigs[0].Value,
	}

	// create the connection secret with the credentials data now
	connectionSecret := instance.ConnectionSecret()
	connectionSecret.Data = kubeconfigData

	if _, err := r.clientset.CoreV1().Secrets(instance.Namespace).Create(connectionSecret); err != nil {
		return err
	}

	log.Printf("created connection secret for AKS cluster %s", instance.Name)
	return nil
}
