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
	"github.com/crossplaneio/crossplane/pkg/util"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AzureMySQLServerHandler is a dynamic provisioning handler for Azure MySQLServer
type AzureMySQLServerHandler struct{}

// Find Azure MysqlServer instance
func (h *AzureMySQLServerHandler) Find(name types.NamespacedName, c client.Client) (corev1alpha1.ConcreteResource, error) {
	azureMySQLServer := &azuredbv1alpha1.MysqlServer{}
	err := c.Get(ctx, name, azureMySQLServer)
	return azureMySQLServer, err
}

// Provision (create) a new Azure SQL Server instance
func (h *AzureMySQLServerHandler) Provision(class *corev1alpha1.ResourceClass, instance corev1alpha1.AbstractResource, c client.Client) (corev1alpha1.ConcreteResource, error) {
	// construct Azure MySQL Server spec from class definition/parameters
	sqlServerSpec := azuredbv1alpha1.NewSQLServerSpec(class.Parameters)

	// resolve the resource class params and the abstract instance values
	if err := resolveAzureClassInstanceValues(sqlServerSpec, instance); err != nil {
		return nil, err
	}

	// assign provider reference and reclaim policy from the resource class
	sqlServerSpec.ProviderRef = class.ProviderRef
	sqlServerSpec.ReclaimPolicy = class.ReclaimPolicy

	// set class and claim references
	sqlServerSpec.ClassRef = class.ObjectReference()
	sqlServerSpec.ClaimRef = instance.ObjectReference()

	objectMeta := metav1.ObjectMeta{
		Namespace:       class.Namespace,
		Name:            util.GenerateName(instance.GetObjectMeta().Name),
		OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
	}

	switch instance.(type) {
	case *storagev1alpha1.MySQLInstance:
		// create and save MySQL Server instance
		mysqlServer := &azuredbv1alpha1.MysqlServer{
			TypeMeta: metav1.TypeMeta{
				APIVersion: azuredbv1alpha1.APIVersion,
				Kind:       azuredbv1alpha1.MysqlServerKind,
			},
			ObjectMeta: objectMeta,
			Spec:       *sqlServerSpec,
		}

		err := c.Create(ctx, mysqlServer)
		return mysqlServer, err
	case *storagev1alpha1.PostgreSQLInstance:
		// create and save PostgreSQL Server instance
		postgresqlServer := &azuredbv1alpha1.PostgresqlServer{
			TypeMeta: metav1.TypeMeta{
				APIVersion: azuredbv1alpha1.APIVersion,
				Kind:       azuredbv1alpha1.PostgresqlServerKind,
			},
			ObjectMeta: objectMeta,
			Spec:       *sqlServerSpec,
		}

		err := c.Create(ctx, postgresqlServer)
		return postgresqlServer, err
	default:
		return nil, fmt.Errorf("unexpected instance type: %+v", reflect.TypeOf(instance))
	}
}

// SetBindStatus updates resource state binding phase
// - state = true: bound
// - state = false: unbound
// TODO: this setBindStatus function could be refactored to 1 common implementation for all providers
func (h *AzureMySQLServerHandler) SetBindStatus(name types.NamespacedName, c client.Client, state bool) error {
	mysqlServer := &azuredbv1alpha1.MysqlServer{}
	err := c.Get(ctx, name, mysqlServer)
	if err != nil {
		// TODO: the CRD is not found and the binding state is supposed to be unbound. is this OK?
		if errors.IsNotFound(err) && !state {
			return nil
		}
		return err
	}
	if state {
		mysqlServer.Status.SetBound()
	} else {
		mysqlServer.Status.SetUnbound()
	}
	return c.Update(ctx, mysqlServer)
}

func resolveAzureClassInstanceValues(sqlServerSpec *azuredbv1alpha1.SQLServerSpec, instance corev1alpha1.AbstractResource) error {
	var engineVersion string

	switch instance.(type) {
	case *storagev1alpha1.MySQLInstance:
		engineVersion = instance.(*storagev1alpha1.MySQLInstance).Spec.EngineVersion
	case *storagev1alpha1.PostgreSQLInstance:
		engineVersion = instance.(*storagev1alpha1.PostgreSQLInstance).Spec.EngineVersion
	default:
		return fmt.Errorf("unexpected instance type: %+v", reflect.TypeOf(instance))
	}

	resolvedEngineVersion, err := corecontroller.ResolveClassInstanceValues(
		sqlServerSpec.Version, engineVersion)
	if err != nil {
		return err
	}

	sqlServerSpec.Version = resolvedEngineVersion
	return nil
}
