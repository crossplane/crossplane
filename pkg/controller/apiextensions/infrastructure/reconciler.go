/*
Copyright 2020 The Crossplane Authors.

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

package infrastructure

import (
	"context"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/controller"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	"github.com/crossplane/crossplane/pkg/controller/apiextensions/crds"
)

const (
	shortWait      = 30 * time.Second
	longWait       = 1 * time.Minute
	timeout        = 2 * time.Minute
	maxConcurrency = 5

	errGetInfraDef         = "cannot get infrastructure definition"
	errGenerateCRD         = "cannot generate crd for given infrastructure definition"
	errApplyCRD            = "cannot apply the generated crd"
	errUpdateInfrDefStatus = "cannot update status of infrastructure definition"
)

// Setup adds a controller that reconciles ApplicationConfigurations.
func Setup(mgr ctrl.Manager, _ logging.Logger) error {
	name := "apiextensions/" + strings.ToLower(v1alpha1.InfrastructureDefinitionGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.InfrastructureDefinition{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrency}).
		Complete(NewReconciler(mgr))
}

// NewReconciler returns a new *Reconciler.
func NewReconciler(m manager.Manager) *Reconciler {
	return &Reconciler{
		client: m.GetClient(),

		// TODO(muvaf): accept these as arguments.
		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}
}

// Reconciler reconciles InfrastructureDefinition resources.
type Reconciler struct {
	client client.Client

	log    logging.Logger
	record event.Recorder
}

// Reconcile is the loop function of reconciliation.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cr := &v1alpha1.InfrastructureDefinition{}
	if err := r.client.Get(ctx, req.NamespacedName, cr); err != nil {
		log.Debug(errGetInfraDef, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetInfraDef)
	}

	log = log.WithValues(
		"uid", cr.GetUID(),
		"version", cr.GetResourceVersion(),
		"name", cr.GetName(),
	)

	generated, err := crds.GenerateInfraCRD(cr)
	if err != nil {
		log.Debug(errGenerateCRD, "error", err)
		cr.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errGenerateCRD)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateInfrDefStatus)
	}
	if err := resource.Apply(ctx, r.client, generated); err != nil {
		log.Debug(errApplyCRD, "error", err)
		cr.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errApplyCRD)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateInfrDefStatus)
	}

	// TODO(muvaf): make sure the controller of the generated type is up and
	// running.

	cr.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
	return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateInfrDefStatus)
}
