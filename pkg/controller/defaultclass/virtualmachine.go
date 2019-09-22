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

package defaultclass

import (
	"fmt"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	computev1alpha1 "github.com/crossplaneio/crossplane/apis/compute/v1alpha1"
)

// VirtualMachineController is responsible for adding the default class controller
// for VirtualMachineInstance and its corresponding reconciler to the manager with any runtime configuration.
type VirtualMachineController struct{}

// SetupWithManager adds a default class controller that reconciles claims
// of kind VirtualMachine to a resource class that declares it as the VirtualMachine
// default
func (c *VirtualMachineController) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewDefaultClassReconciler(mgr,
		resource.ClaimKind(computev1alpha1.VirtualMachineGroupVersionKind),
		resource.PortableClassKind{Singular: computev1alpha1.VirtualMachineClassGroupVersionKind, Plural: computev1alpha1.VirtualMachineClassListGroupVersionKind},
	)

	name := strings.ToLower(fmt.Sprintf("%s.%s", computev1alpha1.VirtualMachineKind, controllerBaseName))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&computev1alpha1.VirtualMachine{}).
		WithEventFilter(resource.NewPredicates(resource.HasNoPortableClassReference())).
		WithEventFilter(resource.NewPredicates(resource.HasNoManagedResourceReference())).
		Complete(r)
}
