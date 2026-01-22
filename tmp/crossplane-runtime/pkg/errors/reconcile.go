/*
Copyright 2023 The Crossplane Authors.

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

package errors

import (
	"context"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SilentlyRequeueOnConflict returns a requeue result and silently drops the
// error if it is a Kubernetes conflict error from the optimistic concurrency
// protocol.
func SilentlyRequeueOnConflict(result reconcile.Result, err error) (reconcile.Result, error) {
	if kerrors.IsConflict(Cause(err)) {
		return reconcile.Result{Requeue: true}, nil
	}

	return result, err
}

// WithSilentRequeueOnConflict wraps a Reconciler and silently drops conflict
// errors and requeues instead.
func WithSilentRequeueOnConflict(r reconcile.Reconciler) reconcile.Reconciler {
	return &silentlyRequeueOnConflict{Reconciler: r}
}

type silentlyRequeueOnConflict struct {
	reconcile.Reconciler
}

func (r *silentlyRequeueOnConflict) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	result, err := r.Reconciler.Reconcile(ctx, req)
	return SilentlyRequeueOnConflict(result, err)
}
