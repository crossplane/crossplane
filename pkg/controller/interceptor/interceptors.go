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

// Package interceptor offers reconciler interceptors
package interceptor

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// LoggingInterceptor wraps a Reconciler and logs messages
// before and after the reconciliation,
// reporting if the operation was successful together with its duration.
func LoggingInterceptor(log logging.Logger, r reconcile.Reconciler) reconcile.Func {
	return func(ctx context.Context, req reconcile.Request) (res reconcile.Result, err error) {
		l := log.WithValues("request", req)
		l.Info("Reconciling")
		start := time.Now()
		defer func() {
			d := time.Since(start).String()
			if err == nil {
				l.Info("Reconciliation done", "result", res, "duration", d)
			} else {
				l.Info("Reconciliation failed", "error", err, "duration", d)
			}
		}()
		return r.Reconcile(ctx, req)
	}
}
