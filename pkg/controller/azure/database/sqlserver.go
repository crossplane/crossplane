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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	azuredbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/database/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	azureclients "github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	passwordDataLen  = 20
	firewallRuleName = "crossplane-sql-firewall-rule"

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
	ctx = context.TODO()
)

// SQLReconciler reconciles SQL resource specs with Azure.
type SQLReconciler struct {
	client.Client
	clientset           kubernetes.Interface
	sqlServerAPIFactory azureclients.SQLServerAPIFactory
	findInstance        func(instance azuredbv1alpha1.SQLServer) (azuredbv1alpha1.SQLServer, error)
	config              *rest.Config
	scheme              *runtime.Scheme
	finalizer           string
}

func (r *SQLReconciler) handleReconcile(instance azuredbv1alpha1.SQLServer) (reconcile.Result, error) {
	// look up the provider information for this instance
	provider := &azurev1alpha1.Provider{}
	providerNamespacedName := apitypes.NamespacedName{
		Namespace: instance.GetNamespace(),
		Name:      instance.GetSpec().ProviderRef.Name,
	}
	if err := r.Get(ctx, providerNamespacedName, provider); err != nil {
		return r.fail(instance, errorFetchingAzureProvider, fmt.Sprintf("failed to get provider %+v: %+v", providerNamespacedName, err))
	}

	// create a SQL Server client to perform management operations in Azure with
	sqlServersClient, err := r.sqlServerAPIFactory.CreateAPIInstance(provider, r.clientset)
	if err != nil {
		return r.fail(instance, errorCreatingClient, fmt.Sprintf("failed to create SQL Server client for instance %s: %+v", instance.GetName(), err))
	}

	// check for CRD deletion and handle it if needed
	if instance.GetDeletionTimestamp() != nil {
		if instance.GetStatus().Condition(corev1alpha1.Deleting) == nil {
			// we haven't started the deletion of the SQL Server resource yet, do it now
			log.Printf("sql server instance %s has been deleted, running finalizer now", instance.GetName())
			return r.handleDeletion(sqlServersClient, instance)
		}
		// we already started the deletion of the SQL Server resource, nothing more to do
		return reconcile.Result{}, nil
	}

	// Add finalizer to the CRD if it doesn't already exist
	if !util.HasFinalizer(instance, r.finalizer) {
		util.AddFinalizer(instance, r.finalizer)
		if err := r.Update(ctx, instance); err != nil {
			log.Printf("failed to add finalizer to instance %s: %+v", instance.GetName(), err)
			return reconcile.Result{}, err
		}
	}

	if instance.GetStatus().RunningOperation != "" {
		// there is a running operation on the instance, wait for it to complete
		return r.handleRunningOperation(sqlServersClient, instance)
	}

	// Get latest SQL Server instance from Azure to check the latest status
	server, err := sqlServersClient.GetServer(ctx, instance)
	if err != nil {
		if !azureclients.IsNotFound(err) {
			return r.fail(instance, errorFetchingInstance, fmt.Sprintf("failed to get SQL Server instance %s: %+v", instance.GetName(), err))
		}

		// the given sql server instance does not exist, create it now
		return r.handleCreation(sqlServersClient, instance)
	}

	if err := sqlServersClient.GetFirewallRule(ctx, instance, firewallRuleName); err != nil {
		if !azureclients.IsNotFound(err) {
			return r.fail(instance, errorFetchingInstance, fmt.Sprintf("failed to get firewall rule for SQL Server instance %s: %+v", instance.GetName(), err))
		}

		return r.handleFirewallRuleCreation(sqlServersClient, instance)
	}

	// SQL Server instance exists, update the CRD status now with its latest status
	stateChanged := instance.GetStatus().State != string(server.State)
	conditionType := azureclients.SQLServerConditionType(server.State)
	if err := r.updateStatus(instance, azureclients.SQLServerStatusMessage(instance.GetName(), server.State), server); err != nil {
		// updating the CRD status failed, return the error and try the next reconcile loop
		log.Printf("failed to update status of instance %s: %+v", instance.GetName(), err)
		return reconcile.Result{}, err
	}

	if stateChanged {
		// the state of the instance has changed, let's set a corresponding condition on the CRD and then
		// requeue another reconciliation attempt
		if conditionType == corev1alpha1.Ready {
			// when we hit the running condition, clear out all old conditions first
			instance.GetStatus().UnsetAllConditions()
		}

		conditionMessage := fmt.Sprintf("SQL Server instance %s is in the %s state", instance.GetName(), conditionType)
		log.Printf(conditionMessage)
		instance.GetStatus().SetCondition(corev1alpha1.NewCondition(conditionType, conditionStateChanged, conditionMessage))
		return reconcile.Result{Requeue: true}, r.Update(ctx, instance)
	}

	if conditionType != corev1alpha1.Ready {
		// the instance isn't running still, requeue another reconciliation attempt
		return reconcile.Result{Requeue: true}, nil
	}

	// ensure all the connection information is set on the secret
	if err := r.createOrUpdateConnectionSecret(instance, ""); err != nil {
		return r.fail(instance, errorSettingConnectionSecret, fmt.Sprintf("failed to set connection secret for SQL Server instance %s: %+v", instance.GetName(), err))
	}

	return reconcile.Result{}, nil
}

// handle the creation of the given SQL Server instance
func (r *SQLReconciler) handleCreation(sqlServersClient azureclients.SQLServerAPI, instance azuredbv1alpha1.SQLServer) (reconcile.Result, error) {
	ctx := context.Background()

	// generate a password for the admin user
	adminPassword, err := util.GeneratePassword(passwordDataLen)
	if err != nil {
		return r.fail(instance, errorCreatingPassword, fmt.Sprintf("failed to create password for SQL Server instance %s: %+v", instance.GetName(), err))
	}

	// save the password to the connection info secret, we'll update the secret later with the
	// server FQDN once we have that
	if err := r.createOrUpdateConnectionSecret(instance, adminPassword); err != nil {
		return r.fail(instance, errorSettingConnectionSecret, fmt.Sprintf("failed to set connection secret for SQL Server instance %s: %+v", instance.GetName(), err))
	}

	// make the API call to start the create server operation
	log.Printf("starting create of SQL Server instance %s", instance.GetName())
	createOp, err := sqlServersClient.CreateServerBegin(ctx, instance, adminPassword)
	if err != nil {
		return r.fail(instance, errorCreatingInstance, fmt.Sprintf("failed to start create operation for SQL Server instance %s: %+v", instance.GetName(), err))
	}

	log.Printf("started create of SQL Server instance %s, operation: %s", instance.GetName(), string(createOp))

	// save the create operation to the CRD status
	status := instance.GetStatus()
	status.SetCreating()
	status.RunningOperation = string(createOp)
	status.RunningOperationType = azuredbv1alpha1.OperationCreateServer

	// wait until the important status fields we just set have become committed/consistent
	updateWaitErr := wait.ExponentialBackoff(util.DefaultUpdateRetry, func() (done bool, err error) {
		if err := r.Update(ctx, instance); err != nil {
			return false, nil
		}

		// the update went through, let's do a get to verify the fields are committed/consistent
		fetchedInstance, err := r.findInstance(instance)
		if err != nil {
			return false, nil
		}

		if fetchedInstance.GetStatus().RunningOperation != "" {
			// the running operation field has been committed, we can stop retrying
			return true, nil
		}

		// the instance hasn't reached consistency yet, retry
		log.Printf("SQL Server instance %s hasn't reached consistency yet, retrying", instance.GetName())
		return false, nil
	})

	return reconcile.Result{Requeue: true}, updateWaitErr
}

// handle the deletion of the given SQL Server instance
func (r *SQLReconciler) handleDeletion(sqlServersClient azureclients.SQLServerAPI, instance azuredbv1alpha1.SQLServer) (reconcile.Result, error) {
	ctx := context.Background()

	// first get the latest status of the SQL Server resource that needs to be deleted
	_, err := sqlServersClient.GetServer(ctx, instance)
	if err != nil {
		if !azureclients.IsNotFound(err) {
			return r.fail(instance, errorFetchingInstance, fmt.Sprintf("failed to get SQL Server instance %s for deletion: %+v", instance.GetName(), err))
		}

		// SQL Server instance doesn't exist, it's already deleted
		log.Printf("SQL Server instance %s does not exist, it must be already deleted", instance.GetName())
		return r.markAsDeleting(instance)
	}

	// attempt to delete the SQL Server instance now
	deleteFuture, err := sqlServersClient.DeleteServer(ctx, instance)
	if err != nil {
		return r.fail(instance, errorDeletingInstance, fmt.Sprintf("failed to start delete operation for SQL Server instance %s: %+v", instance.GetName(), err))
	}

	deleteFutureJSON, _ := deleteFuture.MarshalJSON()
	log.Printf("started delete of SQL Server instance %s, operation: %s", instance.GetName(), string(deleteFutureJSON))
	return r.markAsDeleting(instance)
}

func (r *SQLReconciler) markAsDeleting(instance azuredbv1alpha1.SQLServer) (reconcile.Result, error) {
	ctx := context.Background()
	instance.GetStatus().SetCondition(corev1alpha1.NewCondition(corev1alpha1.Deleting, "", ""))
	util.RemoveFinalizer(instance, r.finalizer)
	return reconcile.Result{}, r.Update(ctx, instance)
}

func (r *SQLReconciler) handleFirewallRuleCreation(sqlServersClient azureclients.SQLServerAPI, instance azuredbv1alpha1.SQLServer) (reconcile.Result, error) {
	ctx := context.Background()

	log.Printf("starting create of firewall rules for SQL Server instance %s", instance.GetName())
	createOp, err := sqlServersClient.CreateFirewallRulesBegin(ctx, instance, firewallRuleName)
	if err != nil {
		return r.fail(instance, errorCreatingInstance, fmt.Sprintf("failed to start create firewall rules operation for SQL Server instance %s: %+v", instance.GetName(), err))
	}

	log.Printf("started create of firewall rules for SQL Server instance %s, operation: %s", instance.GetName(), string(createOp))

	// save the create operation to the CRD status
	status := instance.GetStatus()
	status.RunningOperation = string(createOp)
	status.RunningOperationType = azuredbv1alpha1.OperationCreateFirewallRules

	return reconcile.Result{Requeue: true}, r.Update(ctx, instance)
}

// handle a running operation for the given SQL Server instance
func (r *SQLReconciler) handleRunningOperation(sqlServersClient azureclients.SQLServerAPI, instance azuredbv1alpha1.SQLServer) (reconcile.Result, error) {
	ctx := context.Background()

	var done bool
	var err error
	opType := instance.GetStatus().RunningOperationType

	// check if the operation is done yet and if there was any error
	switch opType {
	case azuredbv1alpha1.OperationCreateServer:
		done, err = sqlServersClient.CreateServerEnd([]byte(instance.GetStatus().RunningOperation))
	case azuredbv1alpha1.OperationCreateFirewallRules:
		done, err = sqlServersClient.CreateFirewallRulesEnd([]byte(instance.GetStatus().RunningOperation))
	default:
		return r.fail(instance, errorCreatingInstance,
			fmt.Sprintf("unknown running operation type for SQL Server instance %s: %s", instance.GetName(), opType))
	}

	if !done {
		// not done yet, check again on the next reconcile
		log.Printf("waiting on create operation type %s for SQL Server instance %s, err: %+v",
			instance.GetStatus().RunningOperationType, instance.GetName(), err)
		return reconcile.Result{Requeue: true}, err
	}

	// the operation is done, clear out the running operation on the CRD status
	status := instance.GetStatus()
	status.RunningOperation = ""
	status.RunningOperationType = ""

	if err != nil {
		// the operation completed, but there was an error
		return r.fail(instance, errorCreatingInstance, fmt.Sprintf("failure result returned from create operation for SQL Server instance %s: %+v", instance.GetName(), err))
	}

	log.Printf("successfully finished operation type %s for SQL Server instance %s", opType, instance.GetName())
	return reconcile.Result{Requeue: true}, r.Update(ctx, instance)
}

// fail - helper function to set fail condition with reason and message
func (r *SQLReconciler) fail(instance azuredbv1alpha1.SQLServer, reason, msg string) (reconcile.Result, error) {
	ctx := context.Background()

	log.Printf("instance %s failed: '%s': %s", instance.GetName(), reason, msg)
	instance.GetStatus().SetCondition(corev1alpha1.NewCondition(corev1alpha1.Failed, reason, msg))
	return reconcile.Result{Requeue: true}, r.Update(ctx, instance)
}

func (r *SQLReconciler) updateStatus(instance azuredbv1alpha1.SQLServer, message string, server *azureclients.SQLServer) error {
	ctx := context.Background()

	oldStatus := instance.GetStatus()
	status := &azuredbv1alpha1.SQLServerStatus{
		ConditionedStatus:    oldStatus.ConditionedStatus,
		BindingStatusPhase:   oldStatus.BindingStatusPhase,
		Message:              message,
		State:                string(server.State),
		ProviderID:           server.ID,
		Endpoint:             server.FQDN,
		RunningOperation:     oldStatus.RunningOperation,
		RunningOperationType: oldStatus.RunningOperationType,
	}
	instance.SetStatus(status)

	if err := r.Update(ctx, instance); err != nil {
		return fmt.Errorf("failed to update status of CRD instance %s: %+v", instance.GetName(), err)
	}

	return nil
}

func (r *SQLReconciler) createOrUpdateConnectionSecret(instance azuredbv1alpha1.SQLServer, password string) error {
	// first check if secret already exists
	secretName := instance.ConnectionSecretName()
	secretExists := false
	connectionSecret, err := r.clientset.CoreV1().Secrets(instance.GetNamespace()).Get(secretName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get connection secret %s for instance %s: %+v", secretName, instance.GetName(), err)
		}
		// secret doesn't exist yet, create it from scratch
		connectionSecret = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            secretName,
				Namespace:       instance.GetNamespace(),
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
		corev1alpha1.ResourceCredentialsSecretUserKey:     []byte(fmt.Sprintf("%s@%s", instance.GetSpec().AdminLoginName, instance.GetName())),
		corev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(password),
	}
	if instance.GetStatus().Endpoint != "" {
		connectionSecret.Data[corev1alpha1.ResourceCredentialsSecretEndpointKey] = []byte(instance.GetStatus().Endpoint)
	}

	if secretExists {
		if _, err := r.clientset.CoreV1().Secrets(instance.GetNamespace()).Update(connectionSecret); err != nil {
			return fmt.Errorf("failed to update connection secret %s: %+v", connectionSecret.Name, err)
		}
		log.Printf("updated connection secret %s for user '%s'", connectionSecret.Name, instance.GetSpec().AdminLoginName)
	} else {
		if _, err := r.clientset.CoreV1().Secrets(instance.GetNamespace()).Create(connectionSecret); err != nil {
			return fmt.Errorf("failed to create connection secret %s: %+v", connectionSecret.Name, err)
		}
		log.Printf("created connection secret %s for user '%s'", connectionSecret.Name, instance.GetSpec().AdminLoginName)
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
