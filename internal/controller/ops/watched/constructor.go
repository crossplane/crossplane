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

package watched

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
	"github.com/crossplane/crossplane/v2/internal/ops/watchcondition"
)

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = log
	}
}

// WithRecorder specifies how the Reconciler should record events.
func WithRecorder(er event.Recorder) ReconcilerOption {
	return func(r *Reconciler) {
		r.record = er
	}
}

// WithProgram specifies a compiled watch condition program. Intended for tests.
func WithProgram(p *watchcondition.Program) ReconcilerOption {
	return func(r *Reconciler) {
		r.program = p
	}
}

// NewReconciler returns a Reconciler that watches resources on behalf of
// a WatchOperation.
func NewReconciler(c client.Client, wo *v1alpha1.WatchOperation, opts ...ReconcilerOption) (*Reconciler, error) {
	r := &Reconciler{
		client:       c,
		watchOpName:  wo.GetName(),
		watchedGVK:   schema.FromAPIVersionAndKind(wo.Spec.Watch.APIVersion, wo.Spec.Watch.Kind),
		log:          logging.NewNopLogger(),
		record:       event.NewNopRecorder(),
		fingerprints: sync.Map{},
	}

	for _, f := range opts {
		f(r)
	}

	if r.program == nil {
		p, err := compileWatchProgram(wo.Spec.Watch)
		if err != nil {
			return nil, err
		}
		r.program = p
	}

	return r, nil
}

func compileWatchProgram(watch v1alpha1.WatchSpec) (*watchcondition.Program, error) {
	p, err := watchcondition.Compile(watch)
	if err != nil {
		return nil, errors.Wrap(err, "cannot compile watch conditions")
	}

	return p, nil
}

// UpdateProgram recompiles and replaces the watch condition program. Call this
// when spec.watch.onChange or spec.watch.when changes after the watched
// controller has already been started.
func (r *Reconciler) UpdateProgram(watch v1alpha1.WatchSpec) error {
	p, err := compileWatchProgram(watch)
	if err != nil {
		return err
	}

	r.program = p
	r.fingerprints = sync.Map{}

	return nil
}
