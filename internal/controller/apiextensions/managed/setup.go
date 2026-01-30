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

package managed

import (
	"strings"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/apis/apiextensions/v1alpha1"
	apiextensionscontroller "github.com/crossplane/crossplane/v2/internal/controller/apiextensions/controller"
	"github.com/crossplane/crossplane/v2/internal/ssa"
)

// Setup adds a controller that reconciles CompositeResourceDefinitions by
// defining a composite resource and starting a controller to reconcile it.
func Setup(mgr ctrl.Manager, o apiextensionscontroller.Options) error {
	name := "mrd/" + strings.ToLower(v1alpha1.ManagedResourceDefinitionKind)

	r := NewReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		WithManagedFieldsUpgrader(ssa.NewPatchingManagedFieldsUpgrader(
			mgr.GetClient(),
			ssa.ExactMatch(FieldOwnerMRD),
		)),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.ManagedResourceDefinition{}).
		Owns(&extv1.CustomResourceDefinition{}, builder.MatchEveryOwner).
		WithOptions(o.ForControllerRuntime()).
		Complete(errors.WithSilentRequeueOnConflict(r))
}

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = log
	}
}

// WithRecorder specifies how the Reconciler should record Kubernetes events.
func WithRecorder(er event.Recorder) ReconcilerOption {
	return func(r *Reconciler) {
		r.record = er
	}
}

// WithManagedFieldsUpgrader specifies how the Reconciler should upgrade CRD
// managed fields from client-side apply to server-side apply.
func WithManagedFieldsUpgrader(u ssa.ManagedFieldsUpgrader) ReconcilerOption {
	return func(r *Reconciler) {
		r.managedFields = u
	}
}

// NewReconciler returns a Reconciler of ManagedResourceDefinitions.
func NewReconciler(mgr ctrl.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client: mgr.GetClient(),

		managedFields: &ssa.NopManagedFieldsUpgrader{},

		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
	}

	for _, f := range opts {
		f(r)
	}

	return r
}
