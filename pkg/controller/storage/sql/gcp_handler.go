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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	gcpdbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/database/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	corecontroller "github.com/crossplaneio/crossplane/pkg/controller/core"
)

// CloudSQLServerHandler is a dynamic provisioning handler for CloudSQL resource
type CloudSQLServerHandler struct{}

// Find CloudSQL resource
func (h *CloudSQLServerHandler) Find(name types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	cloudsqlInstance := &gcpdbv1alpha1.CloudsqlInstance{}
	err := c.Get(ctx, name, cloudsqlInstance)
	return cloudsqlInstance, err
}

// Provision (create) a new CloudSQL resource
func (h *CloudSQLServerHandler) Provision(class *corev1alpha1.ResourceClass, claim corev1alpha1.ResourceClaim, c client.Client) (corev1alpha1.Resource, error) {
	// construct CloudSQL resource spec from class definition/parameters
	cloudsqlInstanceSpec := gcpdbv1alpha1.NewCloudSQLInstanceSpec(class.Parameters)

	// resolve the resource class params and the resource claim values
	if err := resolveGCPClassInstanceValues(cloudsqlInstanceSpec, claim); err != nil {
		return nil, err
	}

	// assign provider reference and reclaim policy from the resource class
	cloudsqlInstanceSpec.ProviderRef = class.ProviderRef
	cloudsqlInstanceSpec.ReclaimPolicy = class.ReclaimPolicy

	// set class and claim references
	cloudsqlInstanceSpec.ClassRef = class.ObjectReference()
	cloudsqlInstanceSpec.ClaimRef = claim.ObjectReference()

	var cloudsqlInstanceName string
	switch claim.(type) {
	case *storagev1alpha1.MySQLInstance:
		cloudsqlInstanceName = fmt.Sprintf("mysql-%s", claim.GetUID())
	case *storagev1alpha1.PostgreSQLInstance:
		cloudsqlInstanceName = fmt.Sprintf("postgresql-%s", claim.GetUID())
	default:
		return nil, fmt.Errorf("unexpected claim type: %+v", reflect.TypeOf(claim))
	}

	// create and save CloudSQL resource
	cloudsqlInstance := &gcpdbv1alpha1.CloudsqlInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gcpdbv1alpha1.APIVersion,
			Kind:       gcpdbv1alpha1.CloudsqlInstanceKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            cloudsqlInstanceName,
			OwnerReferences: []metav1.OwnerReference{claim.OwnerReference()},
		},
		Spec: *cloudsqlInstanceSpec,
	}

	err := c.Create(ctx, cloudsqlInstance)
	return cloudsqlInstance, err
}

// SetBindStatus updates resource state binding phase
// TODO: this SetBindStatus function could be refactored to 1 common implementation for all providers
func (h *CloudSQLServerHandler) SetBindStatus(name types.NamespacedName, c client.Client, bound bool) error {
	cloudsqlInstance := &gcpdbv1alpha1.CloudsqlInstance{}
	err := c.Get(ctx, name, cloudsqlInstance)
	if err != nil {
		// TODO: the CRD is not found and the binding state is supposed to be unbound. is this OK?
		if errors.IsNotFound(err) && !bound {
			return nil
		}
		return err
	}
	cloudsqlInstance.Status.SetBound(bound)
	return c.Update(ctx, cloudsqlInstance)
}

func resolveGCPClassInstanceValues(cloudsqlInstanceSpec *gcpdbv1alpha1.CloudsqlInstanceSpec, claim corev1alpha1.ResourceClaim) error {
	var engineVersion string
	var versionPrefix string

	switch claim := claim.(type) {
	case *storagev1alpha1.MySQLInstance:
		engineVersion = claim.Spec.EngineVersion
		versionPrefix = gcpdbv1alpha1.MysqlDBVersionPrefix
	case *storagev1alpha1.PostgreSQLInstance:
		engineVersion = claim.Spec.EngineVersion
		versionPrefix = gcpdbv1alpha1.PostgresqlDBVersionPrefix
	default:
		return fmt.Errorf("unexpected claim type: %+v", reflect.TypeOf(claim))
	}

	// translate and validate engine version
	translatedEngineVersion := translateVersion(engineVersion, versionPrefix)
	resolvedEngineVersion, err := corecontroller.ResolveClassClaimValues(
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
