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

package database

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/go-autorest/autorest/to"

	"github.com/Azure/azure-sdk-for-go/services/mysql/mgmt/2017-12-01/mysql"
	databasev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/database/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	azureclients "github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/util"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
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
	finalizer = "finalizer.mysqlservers.database.azure.crossplane.io"

	passwordDataLen            = 20
	backupRetentionDaysDefault = int32(7)
	firewallRuleName           = "crossbound-mysql-firewall-rule"

	errorFetchingAzureProvider   = "failed to fetch Azure Provider"
	errorCreatingClient          = "Failed to create Azure client"
	errorFetchingInstance        = "failed to fetch instance"
	errorDeletingInstance        = "failed to delete instance"
	errorCreatingInstance        = "failed to create instance"
	errorWaitingForCreate        = "failed to wait for completion of create instance"
	errorCreatingPassword        = "failed to create password"
	errorSettingConnectionSecret = "failed to set connection secret"
	conditionStateChanged        = "instance state changed"
)

var (
	_ reconcile.Reconciler = &Reconciler{}
)

// AddMysqlServer creates a new MysqlServer Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func AddMysqlServer(mgr manager.Manager) error {
	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create clientset: %+v", err)
	}

	r := newMysqlServerReconciler(mgr, &azureclients.MySQLServerClientFactory{}, clientset)
	return addMysqlServerReconciler(mgr, r)
}

// newMysqlServerReconciler returns a new reconcile.Reconciler
func newMysqlServerReconciler(mgr manager.Manager, mysqlServerAPIFactory azureclients.MySQLServerAPIFactory,
	clientset kubernetes.Interface) *Reconciler {

	return &Reconciler{
		Client:                mgr.GetClient(),
		clientset:             clientset,
		mysqlServerAPIFactory: mysqlServerAPIFactory,
		scheme:                mgr.GetScheme(),
	}
}

// addMysqlServerReconciler adds a new Controller to mgr with r as the reconcile.Reconciler
func addMysqlServerReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("MysqlServer-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MysqlServer
	err = c.Watch(&source.Kind{Type: &databasev1alpha1.MysqlServer{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Reconciler reconciles a MysqlServer object
type Reconciler struct {
	client.Client
	clientset             kubernetes.Interface
	mysqlServerAPIFactory azureclients.MySQLServerAPIFactory
	config                *rest.Config
	scheme                *runtime.Scheme
}

// Reconcile reads that state of the cluster for a MysqlServer object and makes changes based on the state read
// and what is in the MysqlServer.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	instance := &databasev1alpha1.MysqlServer{}
	ctx := context.Background()

	// Fetch the MysqlServer instance
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Printf("failed to get object at start of reconcile loop: %+v", err)
		return reconcile.Result{}, err
	}

	// look up the provider information for this instance
	provider := &azurev1alpha1.Provider{}
	providerNamespacedName := apitypes.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.ProviderRef.Name,
	}
	if err = r.Get(ctx, providerNamespacedName, provider); err != nil {
		return r.fail(instance, errorFetchingAzureProvider, fmt.Sprintf("failed to get provider %+v: %+v", providerNamespacedName, err))
	}

	// create a MySQL Server client to perform management operations in Azure with
	mysqlServersClient, err := r.mysqlServerAPIFactory.CreateAPIInstance(provider, r.clientset)
	if err != nil {
		return r.fail(instance, errorCreatingClient, fmt.Sprintf("failed to create MySQL Server client for instance %s: %+v", instance.Name, err))
	}

	// check for CRD deletion and handle it if needed
	if instance.DeletionTimestamp != nil {
		if instance.Status.Condition(corev1alpha1.Deleting) == nil {
			// we haven't started the deletion of the MySQL Server resource yet, do it now
			log.Printf("mysql server instance %s has been deleted, running finalizer now", instance.Name)
			return r.handleDeletion(mysqlServersClient, instance)
		}
		// we already started the deletion of the MySQL Server resource, nothing more to do
		return reconcile.Result{}, nil
	}

	// Add finalizer to the CRD if it doesn't already exist
	if !util.HasFinalizer(&instance.ObjectMeta, finalizer) {
		util.AddFinalizer(&instance.ObjectMeta, finalizer)
		if err := r.Update(ctx, instance); err != nil {
			log.Printf("failed to add finalizer to instance %s: %+v", instance.Name, err)
			return reconcile.Result{}, err
		}
	}

	if instance.Status.RunningOperation != "" {
		// there is a running operation on the instance, wait for it to complete
		return r.handleRunningOperation(mysqlServersClient, instance)
	}

	// Get latest MySQL Server instance from Azure to check the latest status
	server, err := mysqlServersClient.GetServer(ctx, instance.Spec.ResourceGroupName, instance.Name)
	if err != nil {
		if !azureclients.IsNotFound(err) {
			return r.fail(instance, errorFetchingInstance, fmt.Sprintf("failed to get MySQL Server instance %s: %+v", instance.Name, err))
		}

		// the given mysql server instance does not exist, create it now
		return r.handleCreation(mysqlServersClient, instance)
	}

	if _, err := mysqlServersClient.GetFirewallRule(ctx, instance.Spec.ResourceGroupName, instance.Name, firewallRuleName); err != nil {
		if !azureclients.IsNotFound(err) {
			return r.fail(instance, errorFetchingInstance, fmt.Sprintf("failed to get firewall rule for MySQL Server instance %s: %+v", instance.Name, err))
		}

		return r.handleFirewallRuleCreation(mysqlServersClient, instance)
	}

	// MySQL Server instance exists, update the CRD status now with its latest status
	stateChanged := instance.Status.State != string(server.UserVisibleState)
	conditionType := azureclients.MySQLServerConditionType(server.UserVisibleState)
	if err := r.updateStatus(instance, azureclients.MySQLServerStatusMessage(instance.Name, server.UserVisibleState), server); err != nil {
		// updating the CRD status failed, return the error and try the next reconcile loop
		log.Printf("failed to update status of instance %s: %+v", instance.Name, err)
		return reconcile.Result{}, err
	}

	if stateChanged {
		// the state of the instance has changed, let's set a corresponding condition on the CRD and then
		// requeue another reconciliation attempt
		if conditionType == corev1alpha1.Ready {
			// when we hit the running condition, clear out all old conditions first
			instance.Status.UnsetAllConditions()
		}

		conditionMessage := fmt.Sprintf("MySQL Server instance %s is in the %s state", instance.Name, conditionType)
		log.Printf(conditionMessage)
		instance.Status.SetCondition(corev1alpha1.NewCondition(conditionType, conditionStateChanged, conditionMessage))
		return reconcile.Result{Requeue: true}, r.Update(ctx, instance)
	}

	if conditionType != corev1alpha1.Ready {
		// the instance isn't running still, requeue another reconciliation attempt
		return reconcile.Result{Requeue: true}, nil
	}

	// ensure all the connection information is set on the secret
	if err := r.createOrUpdateConnectionSecret(instance, ""); err != nil {
		return r.fail(instance, errorSettingConnectionSecret, fmt.Sprintf("failed to set connection secret for MySQL Server instance %s: %+v", instance.Name, err))
	}

	return reconcile.Result{}, nil
}

// handle the creation of the given MySQL Server instance
func (r *Reconciler) handleCreation(mysqlServersClient azureclients.MySQLServerAPI, instance *databasev1alpha1.MysqlServer) (reconcile.Result, error) {
	ctx := context.Background()

	// generate a password for the admin user
	adminPassword, err := util.GeneratePassword(passwordDataLen)
	if err != nil {
		return r.fail(instance, errorCreatingPassword, fmt.Sprintf("failed to create password for MySQL Server instance %s: %+v", instance.Name, err))
	}

	// save the password to the connection info secret, we'll update the secret later with the
	// server FQDN once we have that
	if err := r.createOrUpdateConnectionSecret(instance, adminPassword); err != nil {
		return r.fail(instance, errorSettingConnectionSecret, fmt.Sprintf("failed to set connection secret for MySQL Server instance %s: %+v", instance.Name, err))
	}

	// initialize all the parameters that specify how to configure the server during creation
	skuName, err := azureclients.MySQLServerSkuName(instance.Spec.PricingTier)
	if err != nil {
		return r.fail(instance, errorCreatingInstance, fmt.Sprintf("failed to create server SKU name: %+v", err))
	}
	capacity := int32(instance.Spec.PricingTier.VCores)
	storageMB := int32(instance.Spec.StorageProfile.StorageGB * 1024)
	backupRetentionDays := backupRetentionDaysDefault
	if instance.Spec.StorageProfile.BackupRetentionDays > 0 {
		backupRetentionDays = int32(instance.Spec.StorageProfile.BackupRetentionDays)
	}
	createParams := mysql.ServerForCreate{
		Sku: &mysql.Sku{
			Name:     &skuName,
			Tier:     mysql.SkuTier(instance.Spec.PricingTier.Tier),
			Capacity: &capacity,
			Family:   &instance.Spec.PricingTier.Family,
		},
		Properties: &mysql.ServerPropertiesForDefaultCreate{
			AdministratorLogin:         &instance.Spec.AdminLoginName,
			AdministratorLoginPassword: &adminPassword,
			Version:                    mysql.ServerVersion(instance.Spec.Version),
			SslEnforcement:             azureclients.ToSslEnforcement(instance.Spec.SSLEnforced),
			StorageProfile: &mysql.StorageProfile{
				BackupRetentionDays: &backupRetentionDays,
				GeoRedundantBackup:  azureclients.ToGeoRedundantBackup(instance.Spec.StorageProfile.GeoRedundantBackup),
				StorageMB:           &storageMB,
			},
			CreateMode: mysql.CreateModeDefault,
		},
		Location: &instance.Spec.Location,
	}

	// make the API call to start the create server operation
	log.Printf("starting create of MySQL Server instance %s", instance.Name)
	createOp, err := mysqlServersClient.CreateServerBegin(ctx, instance.Spec.ResourceGroupName, instance.Name, createParams)
	if err != nil {
		return r.fail(instance, errorCreatingInstance, fmt.Sprintf("failed to start create operation for MySQL Server instance %s: %+v", instance.Name, err))
	}

	log.Printf("started create of MySQL Server instance %s, operation: %s", instance.Name, string(createOp))

	// save the create operation to the CRD status
	instance.Status.RunningOperation = string(createOp)
	instance.Status.RunningOperationType = databasev1alpha1.OperationCreateServer

	// wait until the important status fields we just set have become committed/consistent
	updateWaitErr := wait.ExponentialBackoff(util.DefaultUpdateRetry, func() (done bool, err error) {
		if err := r.Update(ctx, instance); err != nil {
			return false, nil
		}

		// the update went through, let's do a get to verify the fields are committed/consistent
		fetchedInstance := &databasev1alpha1.MysqlServer{}
		if err := r.Get(ctx, apitypes.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, fetchedInstance); err != nil {
			return false, nil
		}

		if fetchedInstance.Status.RunningOperation != "" {
			// the running operation field has been committed, we can stop retrying
			return true, nil
		}

		// the instance hasn't reached consistency yet, retry
		log.Printf("MySQL Server instance %s hasn't reached consistency yet, retrying", instance.Name)
		return false, nil
	})

	return reconcile.Result{Requeue: true}, updateWaitErr
}

// handle the deletion of the given MySQL Server instance
func (r *Reconciler) handleDeletion(mysqlServersClient azureclients.MySQLServerAPI, instance *databasev1alpha1.MysqlServer) (reconcile.Result, error) {
	ctx := context.Background()

	// first get the latest status of the MySQL Server resource that needs to be deleted
	_, err := mysqlServersClient.GetServer(ctx, instance.Spec.ResourceGroupName, instance.Name)
	if err != nil {
		if !azureclients.IsNotFound(err) {
			return r.fail(instance, errorFetchingInstance, fmt.Sprintf("failed to get MySQL Server instance %s for deletion: %+v", instance.Name, err))
		}

		// MySQL Server instance doesn't exist, it's already deleted
		log.Printf("MySQL Server instance %s does not exist, it must be already deleted", instance.Name)
		return r.markAsDeleting(instance)
	}

	// attempt to delete the MySQL Server instance now
	deleteFuture, err := mysqlServersClient.DeleteServer(ctx, instance.Spec.ResourceGroupName, instance.Name)
	if err != nil {
		return r.fail(instance, errorDeletingInstance, fmt.Sprintf("failed to start delete operation for MySQL Server instance %s: %+v", instance.Name, err))
	}

	deleteFutureJSON, _ := deleteFuture.MarshalJSON()
	log.Printf("started delete of MySQL Server instance %s, operation: %s", instance.Name, string(deleteFutureJSON))
	return r.markAsDeleting(instance)
}

func (r *Reconciler) markAsDeleting(instance *databasev1alpha1.MysqlServer) (reconcile.Result, error) {
	ctx := context.Background()
	instance.Status.SetCondition(corev1alpha1.NewCondition(corev1alpha1.Deleting, "", ""))
	util.RemoveFinalizer(&instance.ObjectMeta, finalizer)
	return reconcile.Result{}, r.Update(ctx, instance)
}

func (r *Reconciler) handleFirewallRuleCreation(mysqlServersClient azureclients.MySQLServerAPI, instance *databasev1alpha1.MysqlServer) (reconcile.Result, error) {
	ctx := context.Background()

	createParams := mysql.FirewallRule{
		Name: to.StringPtr(firewallRuleName),
		FirewallRuleProperties: &mysql.FirewallRuleProperties{
			// TODO: this firewall rules allows inbound access to the Azure MySQL Server from anywhere.
			// we need to better model/abstract tighter inbound access rules.
			StartIPAddress: to.StringPtr("0.0.0.0"),
			EndIPAddress:   to.StringPtr("255.255.255.255"),
		},
	}

	log.Printf("starting create of firewall rules for MySQL Server instance %s", instance.Name)
	createOp, err := mysqlServersClient.CreateFirewallRulesBegin(ctx, instance.Spec.ResourceGroupName, instance.Name, firewallRuleName, createParams)
	if err != nil {
		return r.fail(instance, errorCreatingInstance, fmt.Sprintf("failed to start create firewall rules operation for MySQL Server instance %s: %+v", instance.Name, err))
	}

	log.Printf("started create of firewall rules for MySQL Server instance %s, operation: %s", instance.Name, string(createOp))

	// save the create operation to the CRD status
	instance.Status.RunningOperation = string(createOp)
	instance.Status.RunningOperationType = databasev1alpha1.OperationCreateFirewallRules

	return reconcile.Result{Requeue: true}, r.Update(ctx, instance)
}

// handle a running operation for the given MySQL Server instance
func (r *Reconciler) handleRunningOperation(mysqlServersClient azureclients.MySQLServerAPI, instance *databasev1alpha1.MysqlServer) (reconcile.Result, error) {
	ctx := context.Background()

	var done bool
	var err error
	opType := instance.Status.RunningOperationType

	// check if the operation is done yet and if there was any error
	switch opType {
	case databasev1alpha1.OperationCreateServer:
		done, err = mysqlServersClient.CreateServerEnd([]byte(instance.Status.RunningOperation))
	case databasev1alpha1.OperationCreateFirewallRules:
		done, err = mysqlServersClient.CreateFirewallRulesEnd([]byte(instance.Status.RunningOperation))
	default:
		return r.fail(instance, errorCreatingInstance,
			fmt.Sprintf("unknown running operation type for MySQL Server instance %s: %s", instance.Name, opType))
	}

	if !done {
		// not done yet, check again on the next reconcile
		log.Printf("waiting on create operation type %s for MySQL Server instance %s, err: %+v",
			instance.Status.RunningOperationType, instance.Name, err)
		return reconcile.Result{Requeue: true}, err
	}

	// the operation is done, clear out the running operation on the CRD status
	instance.Status.RunningOperation = ""
	instance.Status.RunningOperationType = ""

	if err != nil {
		// the operation completed, but there was an error
		return r.fail(instance, errorCreatingInstance, fmt.Sprintf("failure result returned from create operation for MySQL Server instance %s: %+v", instance.Name, err))
	}

	log.Printf("successfully finished operation type %s for MySQL Server instance %s", opType, instance.Name)
	return reconcile.Result{Requeue: true}, r.Update(ctx, instance)
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *databasev1alpha1.MysqlServer, reason, msg string) (reconcile.Result, error) {
	ctx := context.Background()

	log.Printf("instance %s failed: '%s': %s", instance.Name, reason, msg)
	instance.Status.SetCondition(corev1alpha1.NewCondition(corev1alpha1.Failed, reason, msg))
	return reconcile.Result{Requeue: true}, r.Update(ctx, instance)
}

func (r *Reconciler) updateStatus(instance *databasev1alpha1.MysqlServer, message string, server mysql.Server) error {
	ctx := context.Background()

	var providerID string
	if server.ID != nil {
		providerID = *server.ID
	}

	var endpoint string
	if server.FullyQualifiedDomainName != nil {
		endpoint = *server.FullyQualifiedDomainName
	}

	instance.Status = databasev1alpha1.MysqlServerStatus{
		ConditionedStatus:    instance.Status.ConditionedStatus,
		BindingStatusPhase:   instance.Status.BindingStatusPhase,
		Message:              message,
		State:                string(server.UserVisibleState),
		ProviderID:           providerID,
		Endpoint:             endpoint,
		RunningOperation:     instance.Status.RunningOperation,
		RunningOperationType: instance.Status.RunningOperationType,
	}

	if err := r.Update(ctx, instance); err != nil {
		return fmt.Errorf("failed to update status of CRD instance %s: %+v", instance.Name, err)
	}

	return nil
}

func (r *Reconciler) createOrUpdateConnectionSecret(instance *databasev1alpha1.MysqlServer, password string) error {
	// first check if secret already exists
	secretName := instance.ConnectionSecretName()
	secretExists := false
	connectionSecret, err := r.clientset.CoreV1().Secrets(instance.Namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get connection secret %s for instance %s: %+v", secretName, instance.Name, err)
		}
		// secret doesn't exist yet, create it from scratch
		connectionSecret = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            secretName,
				Namespace:       instance.Namespace,
				OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
			},
		}
	} else {
		// secret already exists, we'll update the missing information if that hasn't already been done
		secretExists = true
		if isConnectionSecretCompleted(connectionSecret) {
			// the connection secret is already filled out completely, we shouldn't overwrite it
			return nil
		}

		// reuse the password that has already been set
		password = string(connectionSecret.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey])
	}

	// fill in all of the connection details on the secret's data
	connectionSecret.Data = map[string][]byte{
		corev1alpha1.ResourceCredentialsSecretUserKey:     []byte(fmt.Sprintf("%s@%s", instance.Spec.AdminLoginName, instance.Name)),
		corev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(password),
	}
	if instance.Status.Endpoint != "" {
		connectionSecret.Data[corev1alpha1.ResourceCredentialsSecretEndpointKey] = []byte(instance.Status.Endpoint)
	}

	if secretExists {
		if _, err := r.clientset.CoreV1().Secrets(instance.Namespace).Update(connectionSecret); err != nil {
			return fmt.Errorf("failed to update connection secret %s: %+v", connectionSecret.Name, err)
		}
		log.Printf("updated connection secret %s for user '%s'", connectionSecret.Name, instance.Spec.AdminLoginName)
	} else {
		if _, err := r.clientset.CoreV1().Secrets(instance.Namespace).Create(connectionSecret); err != nil {
			return fmt.Errorf("failed to create connection secret %s: %+v", connectionSecret.Name, err)
		}
		log.Printf("created connection secret %s for user '%s'", connectionSecret.Name, instance.Spec.AdminLoginName)
	}

	return nil
}

func isConnectionSecretCompleted(connectionSecret *v1.Secret) bool {
	if connectionSecret == nil {
		return false
	}

	if !isSecretDataKeySet(corev1alpha1.ResourceCredentialsSecretEndpointKey, connectionSecret.Data) {
		return false
	}

	if !isSecretDataKeySet(corev1alpha1.ResourceCredentialsSecretUserKey, connectionSecret.Data) {
		return false
	}

	if !isSecretDataKeySet(corev1alpha1.ResourceCredentialsSecretPasswordKey, connectionSecret.Data) {
		return false
	}

	return true
}

func isSecretDataKeySet(key string, data map[string][]byte) bool {
	if data == nil {
		return false
	}

	// the key has been set if it exists and its value is not an empty string
	val, ok := data[key]
	return ok && string(val) != ""
}
