/*
Copyright 2018 The Conductor Authors.

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

	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	gcpdbv1alpha1 "github.com/upbound/conductor/pkg/apis/gcp/database/v1alpha1"
	mysqlv1alpha1 "github.com/upbound/conductor/pkg/apis/storage/v1alpha1"
	"github.com/upbound/conductor/pkg/util"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CloudSQLServerHandler is a dynamic provisioning handler for CloudSQL instance
type CloudSQLServerHandler struct{}

// find CloudSQL instance
func (h *CloudSQLServerHandler) find(name types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	cloudsqlInstance := &gcpdbv1alpha1.CloudsqlInstance{}
	err := c.Get(ctx, name, cloudsqlInstance)
	return cloudsqlInstance, err
}

// provision (create) a new CloudSQL instance
func (h *CloudSQLServerHandler) provision(class *corev1alpha1.ResourceClass, instance *mysqlv1alpha1.MySQLInstance, c client.Client) (corev1alpha1.Resource, error) {
	// construct CloudSQL instance spec from class definition/parameters
	cloudsqlInstanceSpec := gcpdbv1alpha1.NewCloudSQLInstanceSpec(class.Parameters)

	// translate mysql spec fields to CloudSQL instance spec
	if err := translateToCloudSQL(instance.Spec, cloudsqlInstanceSpec); err != nil {
		return nil, err
	}

	// assign provider reference and reclaim policy from the resource class
	cloudsqlInstanceSpec.ProviderRef = class.ProviderRef
	cloudsqlInstanceSpec.ReclaimPolicy = class.ReclaimPolicy

	// set class and claim references
	cloudsqlInstanceSpec.ClassRef = class.ObjectReference()
	cloudsqlInstanceSpec.ClaimRef = instance.ObjectReference()

	// create and save CloudSQL instance instance
	cloudsqlInstance := &gcpdbv1alpha1.CloudsqlInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gcpdbv1alpha1.APIVersion,
			Kind:       gcpdbv1alpha1.CloudsqlInstanceKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            util.GenerateName(instance.Name),
			OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
		},
		Spec: *cloudsqlInstanceSpec,
	}

	err := c.Create(ctx, cloudsqlInstance)
	return cloudsqlInstance, err
}

// bind updates resource state binding phase
// - state = true: bound
// - state = false: unbound
// TODO: this setBindStatus function could be refactored to 1 common implementation for all providers
func (h *CloudSQLServerHandler) setBindStatus(name types.NamespacedName, c client.Client, state bool) error {
	cloudsqlInstance := &gcpdbv1alpha1.CloudsqlInstance{}
	err := c.Get(ctx, name, cloudsqlInstance)
	if err != nil {
		// TODO: the CRD is not found and the binding state is supposed to be unbound. is this OK?
		if errors.IsNotFound(err) && !state {
			return nil
		}
		return err
	}
	if state {
		cloudsqlInstance.Status.SetBound()
	} else {
		cloudsqlInstance.Status.SetUnbound()
	}
	return c.Update(ctx, cloudsqlInstance)
}

func translateToCloudSQL(instanceSpec mysqlv1alpha1.MySQLInstanceSpec, cloudsqlSpec *gcpdbv1alpha1.CloudsqlInstanceSpec) error {
	if instanceSpec.EngineVersion != "" {
		// the user has specified an engine version on the abstract spec, check if it's valid
		version, ok := gcpdbv1alpha1.ValidVersionValues()[instanceSpec.EngineVersion]
		if !ok {
			return fmt.Errorf("invalid engine version %s", instanceSpec.EngineVersion)
		}

		// specified engine version on the abstract instance spec is valid, set it on the concrete spec
		cloudsqlSpec.DatabaseVersion = version
	}

	return nil
}
