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
	"strings"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	gcpdbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/database/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	corecontroller "github.com/crossplaneio/crossplane/pkg/controller/core"
	"github.com/crossplaneio/crossplane/pkg/util"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CloudSQLServerHandler is a dynamic provisioning handler for CloudSQL instance
type CloudSQLServerHandler struct{}

// Find CloudSQL instance
func (h *CloudSQLServerHandler) Find(name types.NamespacedName, c client.Client) (corev1alpha1.ConcreteResource, error) {
	cloudsqlInstance := &gcpdbv1alpha1.CloudsqlInstance{}
	err := c.Get(ctx, name, cloudsqlInstance)
	return cloudsqlInstance, err
}

// Provision (create) a new CloudSQL instance
func (h *CloudSQLServerHandler) Provision(class *corev1alpha1.ResourceClass, instance corev1alpha1.AbstractResource, c client.Client) (corev1alpha1.ConcreteResource, error) {
	// construct CloudSQL instance spec from class definition/parameters
	cloudsqlInstanceSpec := gcpdbv1alpha1.NewCloudSQLInstanceSpec(class.Parameters)

	// resolve the resource class params and the abstract instance values
	if err := resolveGCPClassInstanceValues(cloudsqlInstanceSpec, instance); err != nil {
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
			Name:            util.GenerateName(instance.GetObjectMeta().Name),
			OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
		},
		Spec: *cloudsqlInstanceSpec,
	}

	err := c.Create(ctx, cloudsqlInstance)
	return cloudsqlInstance, err
}

// Bind updates resource state binding phase
// - state = true: bound
// - state = false: unbound
// TODO: this setBindStatus function could be refactored to 1 common implementation for all providers
func (h *CloudSQLServerHandler) SetBindStatus(name types.NamespacedName, c client.Client, state bool) error {
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

func resolveGCPClassInstanceValues(cloudsqlInstanceSpec *gcpdbv1alpha1.CloudsqlInstanceSpec, instance corev1alpha1.AbstractResource) error {
	var engineVersion string
	var versionPrefix string

	switch instance.(type) {
	case *storagev1alpha1.MySQLInstance:
		engineVersion = instance.(*storagev1alpha1.MySQLInstance).Spec.EngineVersion
		versionPrefix = gcpdbv1alpha1.MysqlDBVersionPrefix
	case *storagev1alpha1.PostgreSQLInstance:
		engineVersion = instance.(*storagev1alpha1.PostgreSQLInstance).Spec.EngineVersion
		versionPrefix = gcpdbv1alpha1.PostgresqlDBVersionPrefix
	default:
		return fmt.Errorf("unexpected instance type: %+v", reflect.TypeOf(instance))
	}

	// translate and validate engine version
	translatedEngineVersion := translateVersion(engineVersion, versionPrefix)
	resolvedEngineVersion, err := corecontroller.ResolveClassInstanceValues(
		cloudsqlInstanceSpec.DatabaseVersion, translatedEngineVersion)
	if err != nil {
		return err
	}

	cloudsqlInstanceSpec.DatabaseVersion = resolvedEngineVersion
	return nil
}

func translateVersion(version, versionPrefix string) string {
	if version == "" {
		return ""
	}
	return fmt.Sprintf("%s_%s", versionPrefix, strings.Replace(version, ".", "_", -1))
}
