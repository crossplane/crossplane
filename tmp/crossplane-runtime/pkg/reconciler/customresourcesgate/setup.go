/*
Copyright 2025 The Crossplane Authors.

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

package customresourcesgate

import (
	"errors"
	"reflect"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
)

// Setup adds a controller that reconciles CustomResourceDefinitions to support delayed start of controllers.
// o.Gate is expected to be something like *gate.Gate[schema.GroupVersionKind].
func Setup(mgr ctrl.Manager, o controller.Options) error {
	if o.Gate == nil || reflect.ValueOf(o.Gate).IsNil() {
		return errors.New("gate is required")
	}

	r := &Reconciler{
		log:  o.Logger,
		gate: o.Gate,
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&apiextensionsv1.CustomResourceDefinition{}).
		Named("crd-gate").
		Complete(reconcile.AsReconciler[*apiextensionsv1.CustomResourceDefinition](mgr.GetClient(), r))
}
