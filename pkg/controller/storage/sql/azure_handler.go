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
	"fmt"
	"reflect"

	azuredbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/database/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	corecontroller "github.com/crossplaneio/crossplane/pkg/controller/core"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AzureMySQLServerHandler is a dynamic provisioning handler for Azure MySQLServer
type AzureMySQLServerHandler struct{}

// AzurePostgreSQLServerHandler is a dynamic provisioning handler for Azure PostgreSQLServer
type AzurePostgreSQLServerHandler struct{}

// Find Azure MysqlServer resource
func (h *AzureMySQLServerHandler) Find(name types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	azureMySQLServer := &azuredbv1alpha1.MysqlServer{}
	err := c.Get(ctx, name, azureMySQLServer)
	return azureMySQLServer, err
}

func (h *AzurePostgreSQLServerHandler) Find(name types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	azurePostgreSQLServer := &azuredbv1alpha1.PostgresqlServer{}
	err := c.Get(ctx, name, azurePostgreSQLServer)
	return azurePostgreSQLServer, err
}

// Provision (create) a new Azure SQL Server resource
func (h *AzureMySQLServerHandler) Provision(class *corev1alpha1.ResourceClass, claim corev1alpha1.ResourceClaim, c client.Client) (corev1alpha1.Resource, error) {
	return provisionAzureSQL(class, claim, c)
}

// Provision (create) a new Azure SQL Server resource
func (h *AzurePostgreSQLServerHandler) Provision(class *corev1alpha1.ResourceClass, claim corev1alpha1.ResourceClaim, c client.Client) (corev1alpha1.Resource, error) {
	return provisionAzureSQL(class, claim, c)
}

func provisionAzureSQL(class *corev1alpha1.ResourceClass, claim corev1alpha1.ResourceClaim,
	c client.Client) (corev1alpha1.Resource, error) {

	// construct Azure MySQL Server spec from class definition/parameters
	sqlServerSpec := azuredbv1alpha1.NewSQLServerSpec(class.Parameters)

	// resolve the resource class params and the resource claim values
	if err := resolveAzureClassInstanceValues(sqlServerSpec, claim); err != nil {
		return nil, err
	}

	// assign provider reference and reclaim policy from the resource class
	sqlServerSpec.ProviderRef = class.ProviderRef
	sqlServerSpec.ReclaimPolicy = class.ReclaimPolicy

	// set class and claim references
	sqlServerSpec.ClassRef = class.ObjectReference()
	sqlServerSpec.ClaimRef = claim.ObjectReference()

	objectMeta := metav1.ObjectMeta{
		Namespace:       class.Namespace,
		OwnerReferences: []metav1.OwnerReference{claim.OwnerReference()},
	}

	switch claim.(type) {
	case *storagev1alpha1.MySQLInstance:
		// create and save MySQL Server resource
		objectMeta.Name = fmt.Sprintf("mysql-%s", claim.GetObjectMeta().UID)
		mysqlServer := &azuredbv1alpha1.MysqlServer{
			TypeMeta: metav1.TypeMeta{
				APIVersion: azuredbv1alpha1.APIVersion,
				Kind:       azuredbv1alpha1.MysqlServerKind,
			},
			ObjectMeta: objectMeta,
			Spec:       *sqlServerSpec,
		}
		mysqlServer.Status.SetUnbound()

		err := c.Create(ctx, mysqlServer)
		return mysqlServer, err
	case *storagev1alpha1.PostgreSQLInstance:
		// create and save PostgreSQL Server resource
		objectMeta.Name = fmt.Sprintf("postgresql-%s", claim.GetObjectMeta().UID)
		postgresqlServer := &azuredbv1alpha1.PostgresqlServer{
			TypeMeta: metav1.TypeMeta{
				APIVersion: azuredbv1alpha1.APIVersion,
				Kind:       azuredbv1alpha1.PostgresqlServerKind,
			},
			ObjectMeta: objectMeta,
			Spec:       *sqlServerSpec,
		}
		postgresqlServer.Status.SetUnbound()

		err := c.Create(ctx, postgresqlServer)
		return postgresqlServer, err
	default:
		return nil, fmt.Errorf("unexpected claim type: %+v", reflect.TypeOf(claim))
	}
}

// SetBindStatus updates resource state binding phase
// - state = true: bound
// - state = false: unbound
// TODO: this setBindStatus function could be refactored to 1 common implementation for all providers
func (h *AzureMySQLServerHandler) SetBindStatus(name types.NamespacedName, c client.Client, state bool) error {
	mysqlServer := &azuredbv1alpha1.MysqlServer{}
	err := c.Get(ctx, name, mysqlServer)
	return setBindStatus(mysqlServer, err, c, state)
}

// SetBindStatus updates resource state binding phase
// - state = true: bound
// - state = false: unbound
// TODO: this setBindStatus function could be refactored to 1 common implementation for all providers
func (h *AzurePostgreSQLServerHandler) SetBindStatus(name types.NamespacedName, c client.Client, state bool) error {
	postgresqlServer := &azuredbv1alpha1.PostgresqlServer{}
	err := c.Get(ctx, name, postgresqlServer)
	return setBindStatus(postgresqlServer, err, c, state)
}

func setBindStatus(resource corev1alpha1.Resource, getErr error, c client.Client, state bool) error {
	if getErr != nil {
		// TODO: the CRD is not found and the binding state is supposed to be unbound. is this OK?
		if errors.IsNotFound(getErr) && !state {
			return nil
		}
		return getErr
	}

	resource.SetBound(state)

	return c.Update(ctx, resource)
}

func resolveAzureClassInstanceValues(sqlServerSpec *azuredbv1alpha1.SQLServerSpec, claim corev1alpha1.ResourceClaim) error {
	var engineVersion string

	switch claim.(type) {
	case *storagev1alpha1.MySQLInstance:
		engineVersion = claim.(*storagev1alpha1.MySQLInstance).Spec.EngineVersion
	case *storagev1alpha1.PostgreSQLInstance:
		engineVersion = claim.(*storagev1alpha1.PostgreSQLInstance).Spec.EngineVersion
	default:
		return fmt.Errorf("unexpected claim type: %+v", reflect.TypeOf(claim))
	}

	resolvedEngineVersion, err := corecontroller.ResolveClassClaimValues(
		sqlServerSpec.Version, engineVersion)
	if err != nil {
		return err
	}

	sqlServerSpec.Version = resolvedEngineVersion
	return nil
}
