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
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
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
	errGet       = "cannot get Composition"
	errListRevs  = "cannot list CompositionRevisions"
	errCreateRev = "cannot create CompositionRevision"
)

// Event reasons.
const (
	reasonCreateRev event.Reason = "CreateRevision"
)

// Setup adds a controller that reconciles Compositions by creating new
// CompositionRevisions for each revision of the Composition's spec.
func Setup(mgr ctrl.Manager, log logging.Logger) error {
	name := "revisions/" + strings.ToLower(v1.CompositionGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.Composition{}).
		Owns(&v1alpha1.CompositionRevision{}).
		Complete(NewReconciler(mgr,
			WithLogger(log.WithValues("controller", name)),
			WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
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
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	comp := &v1.Composition{}
	if err := r.client.Get(ctx, req.NamespacedName, comp); err != nil {
		log.Debug(errGet, "error", err)
		r.record.Event(comp, event.Warning(reasonCreateRev, err))
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGet)
	}

	if meta.WasDeleted(comp) {
		return reconcile.Result{}, nil
	}

	currentHash := hash(comp.Spec)

	log = log.WithValues(
		"uid", comp.GetUID(),
		"version", comp.GetResourceVersion(),
		"name", comp.GetName(),
		"spec-hash", currentHash,
	)

	rl := &v1alpha1.CompositionRevisionList{}
	if err := r.client.List(ctx, rl, client.MatchingLabels{v1alpha1.LabelCompositionName: comp.GetName()}); err != nil {
		log.Debug(errListRevs, "error", err)
		r.record.Event(comp, event.Warning(reasonCreateRev, err))
		return reconcile.Result{}, errors.Wrap(err, errListRevs)
	}

	var latestRev int64
	for i := range rl.Items {
		rev := &rl.Items[i]
		if !metav1.IsControlledBy(rev, comp) {
			continue
		}

		if rev.GetLabels()[v1alpha1.LabelCompositionSpecHash] == currentHash {
			log.Debug("Composition spec returned to previous state. No new revision needed.")
			return reconcile.Result{}, nil
		}

		if rev.Spec.Revision > latestRev {
			latestRev = rev.Spec.Revision
		}
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

func hash(cs v1.CompositionSpec) string {
	h := fnv.New64a()
	y, err := yaml.Marshal(cs)
	if err != nil {
		// I believe this should be impossible given we're marshalling a
		// known, strongly typed struct.
		return "unknown"
	}
	h.Write(y) //nolint:errcheck // Writing to a hash never errors.
	return fmt.Sprintf("%x", h.Sum64())
}
