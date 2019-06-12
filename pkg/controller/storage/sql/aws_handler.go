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

	awsdbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/database/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/meta"
)

// RDSInstanceHandler handles RDS Instance functionality
type RDSInstanceHandler struct{}

// Find RDSInstance
func (h *RDSInstanceHandler) Find(name types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	rdsInstance := &awsdbv1alpha1.RDSInstance{}
	err := c.Get(ctx, name, rdsInstance)
	return rdsInstance, err
}

// Provision create new RDSInstance
func (h *RDSInstanceHandler) Provision(class *corev1alpha1.ResourceClass, claim corev1alpha1.ResourceClaim, c client.Client) (corev1alpha1.Resource, error) {
	spec := awsdbv1alpha1.NewRDSInstanceSpec(class.Parameters)

	if err := resolveAWSClassInstanceValues(spec, claim); err != nil {
		return nil, err
	}

	spec.ProviderRef = class.ProviderRef
	spec.ReclaimPolicy = class.ReclaimPolicy

	spec.ClassRef = meta.ReferenceTo(class, corev1alpha1.ResourceClassGroupVersionKind)
	switch claim.(type) {
	case *storagev1alpha1.MySQLInstance:
		spec.ClaimRef = meta.ReferenceTo(claim, storagev1alpha1.MySQLInstanceGroupVersionKind)
	case *storagev1alpha1.PostgreSQLInstance:
		spec.ClaimRef = meta.ReferenceTo(claim, storagev1alpha1.PostgreSQLInstanceGroupVersionKind)
	}

	i := &awsdbv1alpha1.RDSInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: awsdbv1alpha1.APIVersion,
			Kind:       awsdbv1alpha1.RDSInstanceKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.GetNamespace(),
			Name:            fmt.Sprintf("%s-%s", spec.Engine, claim.GetUID()),
			OwnerReferences: []metav1.OwnerReference{meta.AsOwner(spec.ClaimRef)},
		},
		Spec: *spec,
	}

	return i, c.Create(ctx, i)
}

// SetBindStatus updates resource state binding phase
// TODO: this SetBindStatus function could be refactored to 1 common implementation for all providers
func (h RDSInstanceHandler) SetBindStatus(name types.NamespacedName, c client.Client, bound bool) error {
	rdsInstance := &awsdbv1alpha1.RDSInstance{}
	err := c.Get(ctx, name, rdsInstance)
	if err != nil {
		// TODO: the CRD is not found and the binding state is supposed to be unbound. is this OK?
		if errors.IsNotFound(err) && !bound {
			return nil
		}
		return err
	}
	rdsInstance.Status.SetBound(bound)
	return c.Update(ctx, rdsInstance)
}

func resolveAWSClassInstanceValues(rdsInstanceSpec *awsdbv1alpha1.RDSInstanceSpec, claim corev1alpha1.ResourceClaim) error {
	var engineVersion string

	switch claim := claim.(type) {
	case *storagev1alpha1.MySQLInstance:
		// translate mysql spec fields to RDSInstance spec
		rdsInstanceSpec.Engine = awsdbv1alpha1.MysqlEngine
		engineVersion = claim.Spec.EngineVersion
	case *storagev1alpha1.PostgreSQLInstance:
		// translate postgres spec fields to RDSInstance spec
		rdsInstanceSpec.Engine = awsdbv1alpha1.PostgresqlEngine
		engineVersion = claim.Spec.EngineVersion
	default:
		return fmt.Errorf("unexpected claim type: %+v", reflect.TypeOf(claim))
	}

	resolvedEngineVersion, err := validateEngineVersion(rdsInstanceSpec.EngineVersion, engineVersion)
	if err != nil {
		return err
	}

	rdsInstanceSpec.EngineVersion = resolvedEngineVersion
	return nil
}

// validateEngineVersion compares class and claim engine values and returns an engine value or error
// if class values is empty - claim value returned (could be an empty string),
// otherwise if claim value is not a prefix of the class value - return an error
// else return class value
// Examples:
// class: "", claim: "" - result: ""
// class: 5.6, claim: "" - result: 5.6
// class: "", claim: 5.7 - result: 5.7
// class: 5.6.45, claim 5.6 - result: 5.6.45
// class: 5.6, claim 5.7 - result error
func validateEngineVersion(class, claim string) (string, error) {
	if class == "" {
		return claim, nil
	}
	if strings.HasPrefix(class, claim) {
		return class, nil
	}
	return "", fmt.Errorf("invalid class: [%s], claim: [%s] values combination", class, claim)
}
