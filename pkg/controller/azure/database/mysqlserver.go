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
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	azuredbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/database/v1alpha1"
	azureclients "github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/logging"
)

const (
	mysqlFinalizer = "finalizer.mysqlservers." + controllerName
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
func newMysqlServerReconciler(mgr manager.Manager, sqlServerAPIFactory azureclients.SQLServerAPIFactory,
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

// addMysqlServerReconciler adds a new Controller to mgr with r as the reconcile.Reconciler
func addMysqlServerReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("mysqlservers."+controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MysqlServer
	err = c.Watch(&source.Kind{Type: &azuredbv1alpha1.MysqlServer{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
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
