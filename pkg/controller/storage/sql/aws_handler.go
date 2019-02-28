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

	awsdbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/database/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	// construct RDSInstance Spec from class definition
	rdsInstanceSpec := awsdbv1alpha1.NewRDSInstanceSpec(class.Parameters)

	// resolve the resource class params and the resource claim values
	if err := resolveAWSClassInstanceValues(rdsInstanceSpec, claim); err != nil {
		return nil, err
	}

	rdsInstanceName := fmt.Sprintf("%s-%s", rdsInstanceSpec.Engine, claim.GetUID())

	// assign provider reference and reclaim policy from the resource class
	rdsInstanceSpec.ProviderRef = class.ProviderRef
	rdsInstanceSpec.ReclaimPolicy = class.ReclaimPolicy

	// set class and claim references
	rdsInstanceSpec.ClassRef = class.ObjectReference()
	rdsInstanceSpec.ClaimRef = claim.ObjectReference()

	// create and save RDSInstance
	rdsInstance := &awsdbv1alpha1.RDSInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: awsdbv1alpha1.APIVersion,
			Kind:       awsdbv1alpha1.RDSInstanceKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            rdsInstanceName,
			OwnerReferences: []metav1.OwnerReference{claim.OwnerReference()},
		},
		Spec: *rdsInstanceSpec,
	}
	rdsInstance.Status.SetUnbound()

	err := c.Create(ctx, rdsInstance)
	return rdsInstance, err
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
	if bound {
		rdsInstance.Status.SetBound()
	} else {
		rdsInstance.Status.SetUnbound()
	}
	return c.Update(ctx, rdsInstance)
}

func resolveAWSClassInstanceValues(rdsInstanceSpec *awsdbv1alpha1.RDSInstanceSpec, claim corev1alpha1.ResourceClaim) error {
	var engineVersion string

	switch claim.(type) {
	case *storagev1alpha1.MySQLInstance:
		// translate mysql spec fields to RDSInstance spec
		rdsInstanceSpec.Engine = awsdbv1alpha1.MysqlEngine
		engineVersion = claim.(*storagev1alpha1.MySQLInstance).Spec.EngineVersion
	case *storagev1alpha1.PostgreSQLInstance:
		// translate postgres spec fields to RDSInstance spec
		rdsInstanceSpec.Engine = awsdbv1alpha1.PostgresqlEngine
		engineVersion = claim.(*storagev1alpha1.PostgreSQLInstance).Spec.EngineVersion
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
