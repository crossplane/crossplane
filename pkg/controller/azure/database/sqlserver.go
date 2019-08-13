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

package database

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/mysql/mgmt/2017-12-01/mysql"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pkg/errors"

	corev1alpha1 "github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	azuredbv1alpha1 "github.com/crossplaneio/crossplane/azure/apis/database/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/azure/apis/v1alpha1"
	azureclients "github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/resource"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	controllerName = "database.azure.crossplane.io"

	passwordDataLen  = 20
	firewallRuleName = "crossplane-sql-firewall-rule"
)

var (
	log = logging.Logger.WithName("controller." + controllerName)
	ctx = context.TODO()
)

// SQLReconciler reconciles SQL resource specs with Azure.
type SQLReconciler struct {
	client.Client
	clientset           kubernetes.Interface
	sqlServerAPIFactory azureclients.SQLServerAPIFactory
	findInstance        func(instance azuredbv1alpha1.SQLServer) (azuredbv1alpha1.SQLServer, error)
	scheme              *runtime.Scheme
	finalizer           string
}

// TODO(negz): This method's cyclomatic complexity is very high. Consider
// refactoring it if you touch it.
// nolint:gocyclo
func (r *SQLReconciler) handleReconcile(instance azuredbv1alpha1.SQLServer) (reconcile.Result, error) {

	// look up the provider information for this instance
	provider := &azurev1alpha1.Provider{}
	n := meta.NamespacedNameOf(instance.GetSpec().ProviderReference)
	if err := r.Get(ctx, n, provider); err != nil {
		return r.fail(instance, errors.Wrapf(err, "failed to get provider %s", n))
	}

	// create a SQL Server client to perform management operations in Azure with
	sqlServersClient, err := r.sqlServerAPIFactory.CreateAPIInstance(provider, r.clientset)
	if err != nil {
		return r.fail(instance, errors.Wrapf(err, "failed to create SQL Server client for instance %s", instance.GetName()))
	}

	// check for CRD deletion and handle it if needed
	if instance.GetDeletionTimestamp() != nil {
		log.V(logging.Debug).Info("sql server has been deleted, running finalizer now", "instance", instance)
		return r.handleDeletion(sqlServersClient, instance)
	}

	// TODO(negz): Move finalizer creation into the create method?
	// Add finalizer to the CRD if it doesn't already exist
	meta.AddFinalizer(instance, r.finalizer)
	if err := r.Update(ctx, instance); err != nil {
		log.Error(err, "failed to add finalizer to instance", "instance", instance)
		return reconcile.Result{}, err
	}

	if instance.GetStatus().RunningOperation != "" {
		// there is a running operation on the instance, wait for it to complete
		return r.handleRunningOperation(sqlServersClient, instance)
	}

	// Get latest SQL Server instance from Azure to check the latest status
	server, err := sqlServersClient.GetServer(ctx, instance)
	if err != nil {
		if !azureclients.IsNotFound(err) {
			return r.fail(instance, errors.Wrapf(err, "failed to get SQL Server instance %s", instance.GetName()))
		}

		// the given sql server instance does not exist, create it now
		return r.handleCreation(sqlServersClient, instance)
	}

	if err := sqlServersClient.GetFirewallRule(ctx, instance, firewallRuleName); err != nil {
		if !azureclients.IsNotFound(err) {
			return r.fail(instance, errors.Wrapf(err, "failed to get firewall rule for SQL Server instance %s", instance.GetName()))
		}

		return r.handleFirewallRuleCreation(sqlServersClient, instance)
	}

	if err := r.updateStatus(instance, azureclients.SQLServerStatusMessage(instance.GetName(), server.State), server); err != nil {
		// updating the CRD status failed, return the error and try the next reconcile loop
		log.Error(err, "failed to update status of instance", "instance", instance)
		return reconcile.Result{}, err
	}

	if mysql.ServerState(server.State) != mysql.ServerStateReady {
		// the instance isn't running still, requeue another reconciliation attempt
		instance.GetStatus().SetConditions(corev1alpha1.ReconcileSuccess())
		return reconcile.Result{Requeue: true}, r.Update(ctx, instance)
	}

	// ensure all the connection information is set on the secret
	if err := r.createOrUpdateConnectionSecret(instance, ""); err != nil {
		return r.fail(instance, errors.Wrapf(err, "failed to set connection secret for SQL Server instance %s", instance.GetName()))
	}

	instance.GetStatus().SetConditions(corev1alpha1.ReconcileSuccess())
	return reconcile.Result{}, r.Update(ctx, instance)
}

// handle the creation of the given SQL Server instance
func (r *SQLReconciler) handleCreation(sqlServersClient azureclients.SQLServerAPI, instance azuredbv1alpha1.SQLServer) (reconcile.Result, error) {
	// TODO(negz): Why not use the package scoped context?
	ctx := context.Background()
	instance.GetStatus().SetConditions(corev1alpha1.Creating())

	// generate a password for the admin user
	adminPassword, err := util.GeneratePassword(passwordDataLen)
	if err != nil {
		return r.fail(instance, errors.Wrapf(err, "failed to create password for SQL Server instance %s", instance.GetName()))
	}

	// save the password to the connection info secret, we'll update the secret later with the
	// server FQDN once we have that
	if err := r.createOrUpdateConnectionSecret(instance, adminPassword); err != nil {
		return r.fail(instance, errors.Wrapf(err, "failed to set connection secret for SQL Server instance %s", instance.GetName()))
	}

	// make the API call to start the create server operation
	log.V(logging.Debug).Info("starting create of SQL Server instance", "instance", instance)
	createOp, err := sqlServersClient.CreateServerBegin(ctx, instance, adminPassword)
	if err != nil {
		return r.fail(instance, errors.Wrapf(err, "failed to start create operation for SQL Server instance %s", instance.GetName()))
	}

	log.V(logging.Debug).Info("started create of SQL Server instance", "instance", instance, "operation", string(createOp))

	// save the create operation to the CRD status
	status := instance.GetStatus()
	status.RunningOperation = string(createOp)
	status.RunningOperationType = azuredbv1alpha1.OperationCreateServer
	status.SetConditions(corev1alpha1.ReconcileSuccess())

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
		log.V(logging.Debug).Info("SQL Server instance hasn't reached consistency yet, retrying", "instance", instance)
		return false, nil
	})

	return reconcile.Result{Requeue: true}, updateWaitErr
}

// handle the deletion of the given SQL Server instance
func (r *SQLReconciler) handleDeletion(sqlServersClient azureclients.SQLServerAPI, instance azuredbv1alpha1.SQLServer) (reconcile.Result, error) {
	// TODO(negz): Why not use the package scoped context?
	ctx := context.Background()
	instance.GetStatus().SetConditions(corev1alpha1.Deleting())

	// first get the latest status of the SQL Server resource that needs to be deleted
	_, err := sqlServersClient.GetServer(ctx, instance)
	if err != nil {
		if !azureclients.IsNotFound(err) {
			return r.fail(instance, errors.Wrapf(err, "failed to get SQL Server instance %s for deletion", instance.GetName()))
		}

		// SQL Server instance doesn't exist, it's already deleted
		log.V(logging.Debug).Info("SQL Server instance does not exist, it must be already deleted", "instance", instance)
		meta.RemoveFinalizer(instance, r.finalizer)
		instance.GetStatus().SetConditions(corev1alpha1.ReconcileSuccess())
		return reconcile.Result{}, r.Update(ctx, instance)
	}

	// attempt to delete the SQL Server instance now
	deleteFuture, err := sqlServersClient.DeleteServer(ctx, instance)
	if err != nil {
		return r.fail(instance, errors.Wrapf(err, "failed to start delete operation for SQL Server instance %s", instance.GetName()))
	}

	deleteFutureJSON, _ := deleteFuture.MarshalJSON()
	log.V(logging.Debug).Info("started delete of SQL Server instance", "instance", instance.GetName(), "operation", string(deleteFutureJSON))
	meta.RemoveFinalizer(instance, r.finalizer)
	instance.GetStatus().SetConditions(corev1alpha1.ReconcileSuccess())
	return reconcile.Result{}, r.Update(ctx, instance)
}

func (r *SQLReconciler) handleFirewallRuleCreation(sqlServersClient azureclients.SQLServerAPI, instance azuredbv1alpha1.SQLServer) (reconcile.Result, error) {
	ctx := context.Background()

	log.V(logging.Debug).Info("starting create of firewall rules for SQL Server instance", "instance", instance)
	createOp, err := sqlServersClient.CreateFirewallRulesBegin(ctx, instance, firewallRuleName)
	if err != nil {
		return r.fail(instance, errors.Wrapf(err, "failed to start create firewall rules operation for SQL Server instance %s", instance.GetName()))
	}

	log.V(logging.Debug).Info("started create of firewall rules for SQL Server instance", "instance", instance.GetName(), "operation", string(createOp))

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
		return r.fail(instance,
			errors.Errorf("unknown running operation type for SQL Server instance %s: %s", instance.GetName(), opType))
	}

	if !done {
		// not done yet, check again on the next reconcile
		log.Error(err, "waiting on create operation for SQL Server instance",
			"instance", instance,
			"operation", instance.GetStatus().RunningOperationType)
		return reconcile.Result{Requeue: true}, err
	}

	// the operation is done, clear out the running operation on the CRD status
	status := instance.GetStatus()
	status.RunningOperation = ""
	status.RunningOperationType = ""

	if err != nil {
		// the operation completed, but there was an error
		return r.fail(instance, errors.Wrapf(err, "failure result returned from create operation for SQL Server instance %s", instance.GetName()))
	}

	log.V(logging.Debug).Info("successfully finished operation type for SQL Server", "instance", instance.GetName(), "operation", opType)
	instance.GetStatus().SetConditions(corev1alpha1.ReconcileSuccess())
	return reconcile.Result{Requeue: true}, r.Update(ctx, instance)
}

// fail - helper function to set fail condition with reason and message
func (r *SQLReconciler) fail(instance azuredbv1alpha1.SQLServer, err error) (reconcile.Result, error) {
	// TODO(negz): Why don't we just use the package scoped ctx here?
	ctx := context.Background()

	instance.GetStatus().SetConditions(corev1alpha1.ReconcileError(err))
	return reconcile.Result{Requeue: true}, r.Update(ctx, instance)
}

func (r *SQLReconciler) updateStatus(instance azuredbv1alpha1.SQLServer, message string, server *azureclients.SQLServer) error {
	ctx := context.Background()

	oldStatus := instance.GetStatus()
	status := &azuredbv1alpha1.SQLServerStatus{
		ResourceStatus:       oldStatus.ResourceStatus,
		Message:              message,
		State:                server.State,
		ProviderID:           server.ID,
		Endpoint:             server.FQDN,
		RunningOperation:     oldStatus.RunningOperation,
		RunningOperationType: oldStatus.RunningOperationType,
	}
	status.SetConditions(azureclients.SQLServerCondition(server.State))
	if mysql.ServerState(server.State) == mysql.ServerStateReady {
		resource.SetBindable(status)
	}
	instance.SetStatus(status)

	if err := r.Update(ctx, instance); err != nil {
		return errors.Wrapf(err, "failed to update status of CRD instance %s", instance.GetName())
	}

	return nil
}

func (r *SQLReconciler) createOrUpdateConnectionSecret(instance azuredbv1alpha1.SQLServer, password string) error {
	// TODO(negz): Replace with with a MustGetKind function using the scheme?
	var kind schema.GroupVersionKind
	switch instance.(type) {
	case *azuredbv1alpha1.MysqlServer:
		kind = azuredbv1alpha1.MysqlServerGroupVersionKind
	case *azuredbv1alpha1.PostgresqlServer:
		kind = azuredbv1alpha1.PostgresqlServerGroupVersionKind
	}

	s := resource.ConnectionSecretFor(instance, kind)
	return errors.Wrapf(util.CreateOrUpdate(ctx, r.Client, s, func() error {
		// TODO(negz): Make sure we own any existing secret before overwriting it.
		s.Data[corev1alpha1.ResourceCredentialsSecretEndpointKey] = []byte(instance.GetStatus().Endpoint)
		s.Data[corev1alpha1.ResourceCredentialsSecretUserKey] = []byte(fmt.Sprintf("%s@%s", instance.GetSpec().AdminLoginName, instance.GetName()))

		// Don't overwrite the password if it has already been set.
		if _, ok := s.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey]; !ok && password != "" {
			s.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey] = []byte(password)
		}
		return nil
	}), "could not create or update connection secret %s", s.GetName())
}
