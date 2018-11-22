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

package sql

import (
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	corecontroller "github.com/crossplaneio/crossplane/pkg/controller/core"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	mysqlControllerName = "mysql-storage"
	mysqlFinalizerName  = "finalizer.mysql.storage.crossplane.io"
)

// MySQLReconciler is the reconciler for MySQLInstance objects
type MySQLReconciler struct {
	*corecontroller.Reconciler
}

// AddMySQL creates a new MySQLInstance Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func AddMySQL(mgr manager.Manager) error {
	return addMySQL(mgr, newMySQLReconciler(mgr))
}

// newMySQLReconciler returns a new MySQL reconcile.Reconciler
func newMySQLReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &MySQLReconciler{
		Reconciler: corecontroller.NewReconciler(mgr, mysqlControllerName, mysqlFinalizerName, handlers),
	}
	return r
}

// addMySQL adds a new Controller to mgr with r as the reconcile.Reconciler
func addMySQL(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(mysqlControllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Instance
	err = c.Watch(&source.Kind{Type: &storagev1alpha1.MySQLInstance{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a MySQLInstance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *MySQLReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// fetch the CRD instance
	instance := &storagev1alpha1.MySQLInstance{}

	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return corecontroller.Result, nil
		}
		return corecontroller.Result, err
	}

	return r.DoReconcile(instance)
}
