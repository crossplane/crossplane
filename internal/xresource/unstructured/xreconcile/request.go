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

package xreconcile

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type requestKey struct{}

// WithRequest returns a copy of parent context with the supplied reconcile request.
func WithRequest(ctx context.Context, req reconcile.Request) context.Context {
	return context.WithValue(ctx, requestKey{}, req)
}

// RequestFrom returns the reconcile request stored in context.
func RequestFrom(ctx context.Context) reconcile.Request {
	if req, ok := ctx.Value(requestKey{}).(reconcile.Request); ok {
		return req
	}
	return reconcile.Request{}
}
