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
func (h *RDSInstanceHandler) Find(name types.NamespacedName, c client.Client) (corev1alpha1.ConcreteResource, error) {
	rdsInstance := &awsdbv1alpha1.RDSInstance{}
	err := c.Get(ctx, name, rdsInstance)
	return rdsInstance, err
}

// Provision create new RDSInstance
func (h *RDSInstanceHandler) Provision(class *corev1alpha1.ResourceClass, instance corev1alpha1.AbstractResource, c client.Client) (corev1alpha1.ConcreteResource, error) {
	// construct RDSInstance Spec from class definition
	rdsInstanceSpec := awsdbv1alpha1.NewRDSInstanceSpec(class.Parameters)

	// resolve the resource class params and the abstract instance values
	if err := resolveAWSClassInstanceValues(rdsInstanceSpec, instance); err != nil {
		return nil, err
	}

	rdsInstanceName := fmt.Sprintf("%s-%s", rdsInstanceSpec.Engine, instance.GetObjectMeta().UID)

	// assign provider reference and reclaim policy from the resource class
	rdsInstanceSpec.ProviderRef = class.ProviderRef
	rdsInstanceSpec.ReclaimPolicy = class.ReclaimPolicy

	// set class and claim references
	rdsInstanceSpec.ClassRef = class.ObjectReference()
	rdsInstanceSpec.ClaimRef = instance.ObjectReference()

	// create and save RDSInstance
	rdsInstance := &awsdbv1alpha1.RDSInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: awsdbv1alpha1.APIVersion,
			Kind:       awsdbv1alpha1.RDSInstanceKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            rdsInstanceName,
			OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
		},
		Spec: *rdsInstanceSpec,
	}

	err := c.Create(ctx, rdsInstance)
	return rdsInstance, err
}

// Bind updates resource state binding phase
// - state = true: bound
// - state = false: unbound
// TODO: this setBindStatus function could be refactored to 1 common implementation for all providers
func (h RDSInstanceHandler) SetBindStatus(name types.NamespacedName, c client.Client, state bool) error {
	rdsInstance := &awsdbv1alpha1.RDSInstance{}
	err := c.Get(ctx, name, rdsInstance)
	if err != nil {
		// TODO: the CRD is not found and the binding state is supposed to be unbound. is this OK?
		if errors.IsNotFound(err) && !state {
			return nil
		}
		return err
	}
	if state {
		rdsInstance.Status.SetBound()
	} else {
		rdsInstance.Status.SetUnbound()
	}
	return c.Update(ctx, rdsInstance)
}

func resolveAWSClassInstanceValues(rdsInstanceSpec *awsdbv1alpha1.RDSInstanceSpec, instance corev1alpha1.AbstractResource) error {
	var engineVersion string

	switch instance.(type) {
	case *storagev1alpha1.MySQLInstance:
		// translate mysql spec fields to RDSInstance instance spec
		rdsInstanceSpec.Engine = awsdbv1alpha1.MysqlEngine
		engineVersion = instance.(*storagev1alpha1.MySQLInstance).Spec.EngineVersion
	case *storagev1alpha1.PostgreSQLInstance:
		// translate postgres spec fields to RDSInstance instance spec
		rdsInstanceSpec.Engine = awsdbv1alpha1.PostgresqlEngine
		engineVersion = instance.(*storagev1alpha1.PostgreSQLInstance).Spec.EngineVersion
	default:
		return fmt.Errorf("unexpected instance type: %+v", reflect.TypeOf(instance))
	}

	resolvedEngineVersion, err := validateEngineVersion(rdsInstanceSpec.EngineVersion, engineVersion)
	if err != nil {
		return err
	}

	rdsInstanceSpec.EngineVersion = resolvedEngineVersion
	return nil
}

// validateEngineVersion compares class and instance engine values and returns an engine value or error
// if class values is empty - instance value returned (could be an empty string),
// otherwise if instance value is not a prefix of the class value - return an error
// else return class value
// Examples:
// class: "", instance: "" - result: ""
// class: 5.6, instance: "" - result: 5.6
// class: "", instance: 5.7 - result: 5.7
// class: 5.6.45, instance 5.6 - result: 5.6.45
// class: 5.6, instance 5.7 - result error
func validateEngineVersion(class, instance string) (string, error) {
	if class == "" {
		return instance, nil
	}
	if strings.HasPrefix(class, instance) {
		return class, nil
	}
	return "", fmt.Errorf("invalid class: [%s], instance: [%s] values combination", class, instance)
}
