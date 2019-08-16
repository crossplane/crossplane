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
	"fmt"
	"strings"

	core "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	databasev1alpha1 "github.com/crossplaneio/crossplane/apis/database/v1alpha1"
	"github.com/crossplaneio/crossplane/gcp/apis/database/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
)

// CloudsqlController is responsible for adding the Cloudsql
// controller and its corresponding reconciler to the manager with any runtime configuration.
type CloudsqlController struct{}

// SetupWithManager creates a Controller that reconciles CloudsqlInstance resources.
func (c *CloudsqlController) SetupWithManager(mgr ctrl.Manager) error {
	r := &Reconciler{
		client:  mgr.GetClient(),
		factory: &operationsFactory{mgr.GetClient()},
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&v1alpha1.CloudsqlInstance{}).
		Owns(&core.Secret{}).
		Complete(r)
}

// PostgreSQLInstanceClaimController is responsible for adding the PostgreSQLInstance
// claim controller and its corresponding reconciler to the manager with any runtime configuration.
type PostgreSQLInstanceClaimController struct{}

// SetupWithManager adds a controller that reconciles PostgreSQLInstance instance claims.
func (c *PostgreSQLInstanceClaimController) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(databasev1alpha1.PostgreSQLInstanceGroupVersionKind),
		resource.ClassKind(corev1alpha1.ResourceClassGroupVersionKind),
		resource.ManagedKind(v1alpha1.CloudsqlInstanceGroupVersionKind),
		resource.WithManagedBinder(resource.NewAPIManagedStatusBinder(mgr.GetClient())),
		resource.WithManagedFinalizer(resource.NewAPIManagedStatusUnbinder(mgr.GetClient())),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigurePostgreSQLCloudsqlInstance),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	name := strings.ToLower(fmt.Sprintf("%s.%s", databasev1alpha1.PostgreSQLInstanceKind, controllerName))

	p := v1alpha1.CloudsqlInstanceKindAPIVersion

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		Watches(&source.Kind{Type: &v1alpha1.CloudsqlInstance{}}, &resource.EnqueueRequestForClaim{}).
		For(&databasev1alpha1.PostgreSQLInstance{}).
		WithEventFilter(resource.NewPredicates(resource.ObjectHasProvisioner(mgr.GetClient(), p))).
		Complete(r)
}

// MySQLInstanceClaimController is responsible for adding the MySQLInstance
// claim controller and its corresponding reconciler to the manager with any runtime configuration.
type MySQLInstanceClaimController struct{}

// SetupWithManager adds a controller that reconciles MySQLInstance instance claims.
func (c *MySQLInstanceClaimController) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(databasev1alpha1.MySQLInstanceGroupVersionKind),
		resource.ClassKind(corev1alpha1.ResourceClassGroupVersionKind),
		resource.ManagedKind(v1alpha1.CloudsqlInstanceGroupVersionKind),
		resource.WithManagedBinder(resource.NewAPIManagedStatusBinder(mgr.GetClient())),
		resource.WithManagedFinalizer(resource.NewAPIManagedStatusUnbinder(mgr.GetClient())),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigureMyCloudsqlInstance),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	name := strings.ToLower(fmt.Sprintf("%s.%s", databasev1alpha1.MySQLInstanceKind, controllerName))

	p := v1alpha1.CloudsqlInstanceKindAPIVersion

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		Watches(&source.Kind{Type: &v1alpha1.CloudsqlInstance{}}, &resource.EnqueueRequestForClaim{}).
		For(&databasev1alpha1.MySQLInstance{}).
		WithEventFilter(resource.NewPredicates(resource.ObjectHasProvisioner(mgr.GetClient(), p))).
		Complete(r)
}
