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

package resource

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
)

const (
	controllerNameDefaultClass   = "defaultclass.crossplane.io"
	defaultClassWait             = 1 * time.Minute
	defaultClassReconcileTimeout = 1 * time.Minute
)

var logDefaultClass = logging.Logger.WithName("controller").WithValues("controller", controllerNameDefaultClass)

// Error strings
const (
	errFailedList             = "unable to list default resource classes"
	errNoDefaultClass         = "unable to locate a default resource class for claim kind"
	errMultipleDefaultClasses = "multiple default classes defined for claim kind"
)

// DefaultClassReconciler reconciles resource claims to the
// default resource class for their given kind. Predicates
// ensure that only claims with no resource class reference
// are reconciled.
type DefaultClassReconciler struct {
	client   client.Client
	newClaim func() Claim
	options  *client.ListOptions
}

// NewDefaultClassReconciler creates a new DefaultReconciler for the claim kind
func NewDefaultClassReconciler(m manager.Manager, of ClaimKind) *DefaultClassReconciler {
	nc := func() Claim { return MustCreateObject(schema.GroupVersionKind(of), m.GetScheme()).(Claim) }

	// Panic early if we've been asked to reconcile a claim that has
	// not been registered with our controller manager's scheme.
	_ = nc()

	gk := strings.ToLower(schema.GroupVersionKind(of).GroupKind().String())

	// Create list options query that will be used to search
	// for resource class that is default for claim kind.
	options := &client.ListOptions{}
	if err := options.SetLabelSelector(gk + "/default=" + "true"); err != nil {
		// Panic if unable to set label selector or else panic will occur
		// when returned reconciler is invoked.
		panic(err)
	}

	return &DefaultClassReconciler{
		client:   m.GetClient(),
		newClaim: nc,
		options:  options,
	}
}

// Reconcile reconciles a claim to the default class reference for its kind
func (r *DefaultClassReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	logDefaultClass.V(logging.Debug).Info("Reconciling", "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), defaultClassReconcileTimeout)
	defer cancel()

	claim := r.newClaim()
	if err := r.client.Get(ctx, req.NamespacedName, claim); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		return reconcile.Result{}, errors.Wrap(IgnoreNotFound(err), errGetClaim)
	}

	// Get resource classes with claim kind as default
	classes := &corev1alpha1.ResourceClassList{}
	if err := r.client.List(ctx, r.options, classes); err != nil {
		// If this is the first time we encounter no defaults we'll be
		// requeued implicitly due to the status update. If not, we don't
		// care to requeue because list parameters will not change.
		claim.SetConditions(corev1alpha1.ReconcileError(errors.New(errFailedList)))
		return reconcile.Result{}, errors.Wrap(IgnoreNotFound(r.client.Status().Update(ctx, claim)), errUpdateClaimStatus)
	}

	// Check to see if no defaults defined for claim kind.
	if len(classes.Items) == 0 {
		// If this is the first time we encounter no defaults we'll be
		// requeued implicitly due to the status update. If not, we will requeue
		// after a time to see if a default class has been created.
		claim.SetConditions(corev1alpha1.ReconcileError(errors.New(errNoDefaultClass)))
		return reconcile.Result{RequeueAfter: defaultClassWait}, errors.Wrap(IgnoreNotFound(r.client.Status().Update(ctx, claim)), errUpdateClaimStatus)
	}

	// Check to see if multiple defaults defined for claim kind.
	if len(classes.Items) > 1 {
		// If this is the first time we encounter multiple defaults we'll be
		// requeued implicitly due to the status update. If not, we will requeue
		// after a time to see if only one default class exists.
		claim.SetConditions(corev1alpha1.ReconcileError(errors.New(errMultipleDefaultClasses)))
		return reconcile.Result{RequeueAfter: defaultClassWait}, errors.Wrap(IgnoreNotFound(r.client.Status().Update(ctx, claim)), errUpdateClaimStatus)
	}

	// Set class reference on claim to default resource class
	claim.SetClassReference(meta.ReferenceTo(&classes.Items[0], corev1alpha1.ResourceClassGroupVersionKind))

	// Do not requeue, claim controller will see update and claim
	// with class reference set will pass predicates.
	return reconcile.Result{Requeue: false}, errors.Wrap(IgnoreNotFound(r.client.Update(ctx, claim)), errUpdateClaimStatus)
}
