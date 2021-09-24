/*
Copyright 2021 The Crossplane Authors.

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

package composition

import (
	"context"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

const (
	timeout = 2 * time.Minute
)

// Error strings
const (
	errGet             = "cannot get Composition"
	errListRevs        = "cannot list CompositionRevisions"
	errCreateRev       = "cannot create CompositionRevision"
	errUpdateRevStatus = "cannot update CompositionRevision status"
)

// Event reasons.
const (
	reasonCreateRev event.Reason = "CreateRevision"
	reasonUpdateRev event.Reason = "UpdateRevision"
)

// Setup adds a controller that reconciles Compositions by creating new
// CompositionRevisions for each revision of the Composition's spec.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := "revisions/" + strings.ToLower(v1.CompositionGroupKind)

	r := NewReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.Composition{}).
		Owns(&v1alpha1.CompositionRevision{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
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

// NewReconciler returns a Reconciler of Compositions.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	kube := unstructured.NewClient(mgr.GetClient())

	r := &Reconciler{
		client: kube,
		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}
	return r
}

// A Reconciler reconciles Compositions by creating new CompositionRevisions for
// each revision of the Composition's spec.
type Reconciler struct {
	client client.Client

	log    logging.Logger
	record event.Recorder
}

// Reconcile a Composition.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocyclo
	// NOTE(negz): This method is a little over our complexity goal. Be wary
	// of making it more complex.
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	comp := &v1.Composition{}
	if err := r.client.Get(ctx, req.NamespacedName, comp); err != nil {
		log.Debug(errGet, "error", err)
		r.record.Event(comp, event.Warning(reasonCreateRev, errors.Wrap(err, errGet)))
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGet)
	}

	if meta.WasDeleted(comp) {
		return reconcile.Result{}, nil
	}

	currentHash := comp.Spec.Hash()

	log = log.WithValues(
		"uid", comp.GetUID(),
		"version", comp.GetResourceVersion(),
		"name", comp.GetName(),
		"spec-hash", currentHash,
	)

	rl := &v1alpha1.CompositionRevisionList{}
	if err := r.client.List(ctx, rl, client.MatchingLabels{v1alpha1.LabelCompositionName: comp.GetName()}); err != nil {
		log.Debug(errListRevs, "error", err)
		r.record.Event(comp, event.Warning(reasonCreateRev, errors.Wrap(err, errListRevs)))
		return reconcile.Result{}, errors.Wrap(err, errListRevs)
	}

	var latestRev, existingRev int64
	for i := range rl.Items {
		rev := &rl.Items[i]
		if !metav1.IsControlledBy(rev, comp) {
			continue
		}

		if rev.Spec.Revision > latestRev {
			latestRev = rev.Spec.Revision
		}

		want := v1alpha1.CompositionSpecDiffers()
		if rev.GetLabels()[v1alpha1.LabelCompositionSpecHash] == currentHash {
			existingRev = rev.Spec.Revision
			want = v1alpha1.CompositionSpecMatches()
		}

		// No need to update this revision's status; it already has the
		// appropriate 'Current' condition.
		if got := rev.Status.GetCondition(v1alpha1.TypeCurrent); got.Status == want.Status {
			continue
		}

		// Toggle the 'Current' condition of this revision.
		rev.Status.SetConditions(want)
		if err := r.client.Status().Update(ctx, rev); err != nil {
			log.Debug(errUpdateRevStatus, "error", err)
			r.record.Event(comp, event.Warning(reasonUpdateRev, err))
			return reconcile.Result{}, errors.Wrap(err, errUpdateRevStatus)
		}
	}

	// We start from revision 1, so 0 indicates we didn't find one.
	if existingRev > 0 {
		log.Debug("No new revision needed.", "current-revision", existingRev)
		return reconcile.Result{}, nil
	}

	if err := r.client.Create(ctx, NewCompositionRevision(comp, latestRev+1, currentHash)); err != nil {
		log.Debug(errCreateRev, "error", err)
		r.record.Event(comp, event.Warning(reasonCreateRev, err))
		return reconcile.Result{}, errors.Wrap(err, errCreateRev)
	}

	log.Debug("Created new revision", "revision", latestRev+1)
	r.record.Event(comp, event.Normal(reasonCreateRev, "Created new revision", "revision", strconv.FormatInt(latestRev+1, 10)))
	return reconcile.Result{}, nil
}
