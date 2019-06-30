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
	"strings"

	"github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplaneio/crossplane/pkg/resource"

	"github.com/crossplaneio/crossplane/pkg/apis/gcp/database/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
)

// The main motivation for creating add.go and move `Add` controller-runtime Manager
// functions for CloudsqlInstance, PostgreSQLClaim and MySQLClaim is to add this
// file to the coverage "ignore" list

// Add creates a Controller that reconciles CloudsqlInstance resources
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		Client:  mgr.GetClient(),
		factory: &operationsFactory{mgr.GetClient()},
	}

	// Create a newSyncDeleter controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to cloudsql instance
	if err := c.Watch(&source.Kind{Type: &v1alpha1.CloudsqlInstance{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch for changes to instance connection secret
	return c.Watch(&source.Kind{Type: &core.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.CloudsqlInstance{},
	})
}

// AddPostgreSQLClaim adds a controller that reconciles PostgreSQLInstance instance claims by
// managing CloudsqlInstance resources to the supplied Manager.
func AddPostgreSQLClaim(mgr manager.Manager) error {
	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(storagev1alpha1.PostgreSQLInstanceGroupVersionKind),
		resource.ManagedKind(v1alpha1.CloudsqlInstanceGroupVersionKind),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigurePostgreCloudsqlInstance),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	name := strings.ToLower(fmt.Sprintf("%s.%s", storagev1alpha1.PostgreSQLInstanceKind, controllerName))
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrapf(err, "cannot create %s controller", name)
	}

	if err := c.Watch(&source.Kind{Type: &v1alpha1.CloudsqlInstance{}}, &resource.EnqueueRequestForClaim{}); err != nil {
		return errors.Wrapf(err, "cannot watch for %s", v1alpha1.CloudsqlInstanceGroupVersionKind)
	}

	p := v1alpha1.CloudsqlInstanceKindAPIVersion
	return errors.Wrapf(c.Watch(
		&source.Kind{Type: &storagev1alpha1.PostgreSQLInstance{}},
		&handler.EnqueueRequestForObject{},
		resource.NewPredicates(resource.ObjectHasProvisioner(mgr.GetClient(), p)),
	), "cannot watch for %s", storagev1alpha1.PostgreSQLInstanceGroupVersionKind)
}

// AddMySQLClaim adds a controller that reconciles MySQLInstance instance claims by
// managing CloudsqlInstance resources to the supplied Manager.
func AddMySQLClaim(mgr manager.Manager) error {
	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(storagev1alpha1.MySQLInstanceGroupVersionKind),
		resource.ManagedKind(v1alpha1.CloudsqlInstanceGroupVersionKind),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigureMyCloudsqlInstance),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	name := strings.ToLower(fmt.Sprintf("%s.%s", storagev1alpha1.MySQLInstanceKind, controllerName))
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrapf(err, "cannot create %s controller", name)
	}

	if err := c.Watch(
		&source.Kind{Type: &v1alpha1.CloudsqlInstance{}},
		&resource.EnqueueRequestForClaim{},
	); err != nil {
		return errors.Wrapf(err, "cannot watch for %s", v1alpha1.CloudsqlInstanceGroupVersionKind)
	}

	p := v1alpha1.CloudsqlInstanceKindAPIVersion
	return errors.Wrapf(c.Watch(
		&source.Kind{Type: &storagev1alpha1.MySQLInstance{}},
		&handler.EnqueueRequestForObject{},
		resource.NewPredicates(resource.ObjectHasProvisioner(mgr.GetClient(), p)),
	), "cannot watch for %s", storagev1alpha1.MySQLInstanceGroupVersionKind)
}
