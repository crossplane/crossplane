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
	"k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	azuredbv1alpha1 "github.com/crossplaneio/crossplane/azure/apis/database/v1alpha1"
	azureclients "github.com/crossplaneio/crossplane/pkg/clients/azure"
)

const (
	postgresqlFinalizer = "finalizer.postgresqlservers." + controllerName
)

// PostgresqlServerController is responsible for adding the PostgresqlServer
// controller and its corresponding reconciler to the manager with any runtime configuration.
type PostgresqlServerController struct {
	Reconciler reconcile.Reconciler
}

// SetupWithManager creates a Controller that reconciles PostgresqlServer resources.
func (c *PostgresqlServerController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("Postgresqlservers." + controllerName).
		For(&azuredbv1alpha1.PostgresqlServer{}).
		Complete(c.Reconciler)
}

// NewPostgreSQLServerReconciler returns a new reconcile.Reconciler
func NewPostgreSQLServerReconciler(mgr manager.Manager, sqlServerAPIFactory azureclients.SQLServerAPIFactory,
	clientset kubernetes.Interface) *PostgreSQLReconciler {

	r := &PostgreSQLReconciler{}
	r.SQLReconciler = &SQLReconciler{
		Client:              mgr.GetClient(),
		clientset:           clientset,
		sqlServerAPIFactory: sqlServerAPIFactory,
		findInstance:        r.findPostgreSQLInstance,
		scheme:              mgr.GetScheme(),
		finalizer:           postgresqlFinalizer,
	}

	return r
}

// PostgreSQLReconciler reconciles a PostgreSQLServer object
type PostgreSQLReconciler struct {
	*SQLReconciler
}

// Reconcile reads that state of the cluster for a PostgreSQLServer object and makes changes based on the state read
// and what is in the PostgreSQLServer.Spec
func (r *PostgreSQLReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", azuredbv1alpha1.PostgresqlServerKindAPIVersion, "request", request)
	instance := &azuredbv1alpha1.PostgresqlServer{}

	// Fetch the PostgresqlServer instance
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		log.Error(err, "failed to get object at start of reconcile loop")
		return reconcile.Result{}, err
	}

	return r.SQLReconciler.handleReconcile(instance)
}

func (r *PostgreSQLReconciler) findPostgreSQLInstance(instance azuredbv1alpha1.SQLServer) (azuredbv1alpha1.SQLServer, error) {
	fetchedInstance := &azuredbv1alpha1.PostgresqlServer{}
	namespacedName := apitypes.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()}
	if err := r.Get(ctx, namespacedName, fetchedInstance); err != nil {
		return nil, err
	}

	return fetchedInstance, nil
}
