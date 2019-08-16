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

	azuredbv1alpha1 "github.com/crossplaneio/crossplane/azure/apis/database/v1alpha1"
	azureclients "github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/logging"
)

const (
	mysqlFinalizer = "finalizer.mysqlservers." + controllerName
)

// MysqlServerController is responsible for adding the MysqlServer
// controller and its corresponding reconciler to the manager with any runtime configuration.
type MysqlServerController struct {
	Reconciler reconcile.Reconciler
}

// SetupWithManager creates a Controller that reconciles MysqlServer resources.
func (c *MysqlServerController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("mysqlservers." + controllerName).
		For(&azuredbv1alpha1.MysqlServer{}).
		Complete(c.Reconciler)
}

// NewMysqlServerReconciler returns a new reconcile.Reconciler
func NewMysqlServerReconciler(mgr manager.Manager, sqlServerAPIFactory azureclients.SQLServerAPIFactory,
	clientset kubernetes.Interface) *MySQLReconciler {

	r := &MySQLReconciler{}
	r.SQLReconciler = &SQLReconciler{
		Client:              mgr.GetClient(),
		clientset:           clientset,
		sqlServerAPIFactory: sqlServerAPIFactory,
		findInstance:        r.findMySQLInstance,
		scheme:              mgr.GetScheme(),
		finalizer:           mysqlFinalizer,
	}

	return r
}

// MySQLReconciler reconciles a MysqlServer object
type MySQLReconciler struct {
	*SQLReconciler
}

// Reconcile reads that state of the cluster for a MysqlServer object and makes changes based on the state read
// and what is in the MysqlServer.Spec
func (r *MySQLReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", azuredbv1alpha1.MysqlServerKindAPIVersion, "request", request)
	instance := &azuredbv1alpha1.MysqlServer{}

	// Fetch the MysqlServer instance
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

func (r *MySQLReconciler) findMySQLInstance(instance azuredbv1alpha1.SQLServer) (azuredbv1alpha1.SQLServer, error) {
	fetchedInstance := &azuredbv1alpha1.MysqlServer{}
	namespacedName := apitypes.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()}
	if err := r.Get(ctx, namespacedName, fetchedInstance); err != nil {
		return nil, err
	}

	return fetchedInstance, nil
}
