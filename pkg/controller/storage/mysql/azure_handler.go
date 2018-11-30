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

package mysql

import (
	"fmt"

	azuredbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/database/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	mysqlv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AzureMySQLServerHandler is a dynamic provisioning handler for Azure MySQLServer
type AzureMySQLServerHandler struct{}

// find Azure MysqlServer instance
func (h *AzureMySQLServerHandler) find(name types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	azureMySQLServer := &azuredbv1alpha1.MysqlServer{}
	err := c.Get(ctx, name, azureMySQLServer)
	return azureMySQLServer, err
}

// provision (create) a new Azure MySQLServer instance
func (h *AzureMySQLServerHandler) provision(class *corev1alpha1.ResourceClass, instance *mysqlv1alpha1.MySQLInstance, c client.Client) (corev1alpha1.Resource, error) {
	// construct Azure MySQL Server spec from class definition/parameters
	mysqlServerSpec := azuredbv1alpha1.NewMySQLServerSpec(class.Parameters)

	// validate and assign version
	var err error
	if mysqlServerSpec.Version, err = resolveClassInstanceValues(mysqlServerSpec.Version, instance.Spec.EngineVersion); err != nil {
		return nil, err
	}

	// assign provider reference and reclaim policy from the resource class
	mysqlServerSpec.ProviderRef = class.ProviderRef
	mysqlServerSpec.ReclaimPolicy = class.ReclaimPolicy

	// set class and claim references
	mysqlServerSpec.ClassRef = class.ObjectReference()
	mysqlServerSpec.ClaimRef = instance.ObjectReference()

	// create and save MySQL Server instance
	mysqlServer := &azuredbv1alpha1.MysqlServer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: azuredbv1alpha1.APIVersion,
			Kind:       azuredbv1alpha1.MysqlServerKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            fmt.Sprintf("mysql-%s", instance.UID),
			OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
		},
		Spec: *mysqlServerSpec,
	}

	err = c.Create(ctx, mysqlServer)
	return mysqlServer, err
}

// bind updates resource state binding phase
// - state = true: bound
// - state = false: unbound
// TODO: this setBindStatus function could be refactored to 1 common implementation for all providers
func (h *AzureMySQLServerHandler) setBindStatus(name types.NamespacedName, c client.Client, state bool) error {
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
