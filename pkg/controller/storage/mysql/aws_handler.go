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
	"strings"

	awsdbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/database/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	mysqlv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RDSInstanceHandler handles RDS Instance functionality
type RDSInstanceHandler struct{}

// find RDSInstance
func (h *RDSInstanceHandler) find(name types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	rdsInstance := &awsdbv1alpha1.RDSInstance{}
	err := c.Get(ctx, name, rdsInstance)
	return rdsInstance, err
}

// provision create new RDSInstance
func (h *RDSInstanceHandler) provision(class *corev1alpha1.ResourceClass, instance *mysqlv1alpha1.MySQLInstance, c client.Client) (corev1alpha1.Resource, error) {
	// construct RDSInstance Spec from class definition
	rdsInstanceSpec := awsdbv1alpha1.NewRDSInstanceSpec(class.Parameters)

	// TODO: it is not clear if all concrete resource use the same constant value for database engine
	// if they do - we will need to refactor this value into constant.
	rdsInstanceSpec.Engine = "mysql"
	rdsInstanceName := fmt.Sprintf("%s-%s", rdsInstanceSpec.Engine, instance.UID)

	// validate engine version value (if needed)
	var err error
	if rdsInstanceSpec.EngineVersion, err = validateEngineVersion(instance.Spec.EngineVersion, rdsInstanceSpec.EngineVersion); err != nil {
		return nil, err
	}

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

	err = c.Create(ctx, rdsInstance)
	return rdsInstance, err
}

// bind updates resource state binding phase
// - state = true: bound
// - state = false: unbound
// TODO: this setBindStatus function could be refactored to 1 common implementation for all providers
func (h RDSInstanceHandler) setBindStatus(name types.NamespacedName, c client.Client, state bool) error {
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
