/*
Copyright 2019 The Crossplane Authors.

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

package rds

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/source"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	databasev1alpha1 "github.com/crossplaneio/crossplane/apis/database/v1alpha1"
	"github.com/crossplaneio/crossplane/aws/apis/database/v1alpha1"
)

// PostgreSQLInstanceClaimController is responsible for adding the PostgreSQLInstance
// claim controller and its corresponding reconciler to the manager with any runtime configuration.
type PostgreSQLInstanceClaimController struct{}

// SetupWithManager adds a controller that reconciles PostgreSQLInstance instance claims.
func (c *PostgreSQLInstanceClaimController) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(databasev1alpha1.PostgreSQLInstanceGroupVersionKind),
		resource.ClassKind(v1alpha1.RDSInstanceClassGroupVersionKind),
		resource.ManagedKind(v1alpha1.RDSInstanceGroupVersionKind),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigurePostgreRDSInstance),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	name := strings.ToLower(fmt.Sprintf("%s.%s", databasev1alpha1.PostgreSQLInstanceKind, controllerName))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		Watches(&source.Kind{Type: &v1alpha1.RDSInstance{}}, &resource.EnqueueRequestForClaim{}).
		For(&databasev1alpha1.PostgreSQLInstance{}).
		WithEventFilter(resource.NewPredicates(resource.HasClassReferenceKind(resource.ClassKind(v1alpha1.RDSInstanceClassGroupVersionKind)))).
		Complete(r)
}

// MySQLInstanceClaimController is responsible for adding the MySQLInstance
// claim controller and its corresponding reconciler to the manager with any runtime configuration.
type MySQLInstanceClaimController struct{}

// SetupWithManager adds a controller that reconciles MySQLInstance instance claims.
func (c *MySQLInstanceClaimController) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(databasev1alpha1.MySQLInstanceGroupVersionKind),
		resource.ClassKind(v1alpha1.RDSInstanceClassGroupVersionKind),
		resource.ManagedKind(v1alpha1.RDSInstanceGroupVersionKind),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigureMyRDSInstance),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	name := strings.ToLower(fmt.Sprintf("%s.%s", databasev1alpha1.MySQLInstanceKind, controllerName))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		Watches(&source.Kind{Type: &v1alpha1.RDSInstance{}}, &resource.EnqueueRequestForClaim{}).
		For(&databasev1alpha1.MySQLInstance{}).
		WithEventFilter(resource.NewPredicates(resource.HasClassReferenceKind(resource.ClassKind(v1alpha1.RDSInstanceClassGroupVersionKind)))).
		Complete(r)
}

// ConfigurePostgreRDSInstance configures the supplied resource (presumed
// to be a RDSInstance) using the supplied resource claim (presumed to be a
// PostgreSQLInstance) and resource class.
func ConfigurePostgreRDSInstance(_ context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	pg, cmok := cm.(*databasev1alpha1.PostgreSQLInstance)
	if !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), databasev1alpha1.PostgreSQLInstanceGroupVersionKind)
	}

	rs, csok := cs.(*v1alpha1.RDSInstanceClass)
	if !csok {
		return errors.Errorf("expected resource class %s to be %s", cs.GetName(), v1alpha1.RDSInstanceClassGroupVersionKind)
	}

	i, mgok := mg.(*v1alpha1.RDSInstance)
	if !mgok {
		return errors.Errorf("expected managed resource %s to be %s", mg.GetName(), v1alpha1.RDSInstanceGroupVersionKind)
	}

	spec := &v1alpha1.RDSInstanceSpec{
		ResourceSpec: runtimev1alpha1.ResourceSpec{
			ReclaimPolicy: runtimev1alpha1.ReclaimRetain,
		},
		RDSInstanceParameters: rs.SpecTemplate.RDSInstanceParameters,
	}
	spec.Engine = v1alpha1.PostgresqlEngine
	v, err := validateEngineVersion(spec.EngineVersion, pg.Spec.EngineVersion)
	if err != nil {
		return err
	}
	spec.EngineVersion = v

	spec.WriteConnectionSecretToReference = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	spec.ProviderReference = rs.SpecTemplate.ProviderReference
	spec.ReclaimPolicy = rs.SpecTemplate.ReclaimPolicy

	i.Spec = *spec

	return nil
}

// ConfigureMyRDSInstance configures the supplied resource (presumed to be
// a RDSInstance) using the supplied resource claim (presumed to be a
// MySQLInstance) and resource class.
func ConfigureMyRDSInstance(_ context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	my, cmok := cm.(*databasev1alpha1.MySQLInstance)
	if !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), databasev1alpha1.MySQLInstanceGroupVersionKind)
	}

	rs, csok := cs.(*v1alpha1.RDSInstanceClass)
	if !csok {
		return errors.Errorf("expected resource class %s to be %s", cs.GetName(), v1alpha1.RDSInstanceClassGroupVersionKind)
	}

	i, mgok := mg.(*v1alpha1.RDSInstance)
	if !mgok {
		return errors.Errorf("expected managed resource %s to be %s", mg.GetName(), v1alpha1.RDSInstanceGroupVersionKind)
	}

	spec := &v1alpha1.RDSInstanceSpec{
		ResourceSpec: runtimev1alpha1.ResourceSpec{
			ReclaimPolicy: runtimev1alpha1.ReclaimRetain,
		},
		RDSInstanceParameters: rs.SpecTemplate.RDSInstanceParameters,
	}
	spec.Engine = v1alpha1.MysqlEngine
	v, err := validateEngineVersion(spec.EngineVersion, my.Spec.EngineVersion)
	if err != nil {
		return err
	}
	spec.EngineVersion = v

	spec.WriteConnectionSecretToReference = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	spec.ProviderReference = rs.SpecTemplate.ProviderReference
	spec.ReclaimPolicy = rs.SpecTemplate.ReclaimPolicy

	i.Spec = *spec

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
	return "", errors.Errorf("claim value [%s] does not match class value [%s]", claim, class)
}
