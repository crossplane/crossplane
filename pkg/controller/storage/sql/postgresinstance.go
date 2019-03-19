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
	"log"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	corecontroller "github.com/crossplaneio/crossplane/pkg/controller/core"
)

const (
	postgresControllerName = "postgresql.storage.crossplane.io"
	postgresFinalizerName  = "finalizer." + postgresControllerName
)

// PostgreSQLReconciler is the reconciler for PostgreSQLInstance objects
type PostgreSQLReconciler struct {
	*corecontroller.Reconciler
}

// AddPostgreSQL creates a new PostgreSQLInstance Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func AddPostgreSQL(mgr manager.Manager) error {
	return addPostgreSQL(mgr, newPostgreSQLReconciler(mgr))
}

// newPostgreSQLReconciler returns a new PostgreSQL reconcile.Reconciler
func newPostgreSQLReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &PostgreSQLReconciler{
		Reconciler: corecontroller.NewReconciler(mgr, postgresControllerName, postgresFinalizerName, handlers),
	}
	return r
}

// addPostgreSQL adds a new Controller to mgr with r as the reconcile.Reconciler
func addPostgreSQL(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(postgresControllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to PostgreSQLInstance
	err = c.Watch(&source.Kind{Type: &storagev1alpha1.PostgreSQLInstance{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a PostgreSQLInstance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *PostgreSQLReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("reconciling %s: %v", storagev1alpha1.PostgreSQLInstanceKindAPIVersion, request)

	// fetch the CRD instance
	instance := &storagev1alpha1.PostgreSQLInstance{}
	if err := r.Get(ctx, request.NamespacedName, instance); err != nil {
		return corecontroller.HandleGetClaimError(err)
	}

	return r.DoReconcile(instance)
}
