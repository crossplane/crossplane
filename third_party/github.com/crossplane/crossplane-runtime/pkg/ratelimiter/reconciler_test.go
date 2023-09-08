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

package ratelimiter

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var _ ratelimiter.RateLimiter = &predictableRateLimiter{}

type predictableRateLimiter struct{ d time.Duration }

func (r *predictableRateLimiter) When(_ any) time.Duration { return r.d }
func (r *predictableRateLimiter) Forget(_ any)             {}
func (r *predictableRateLimiter) NumRequeues(_ any) int    { return 0 }

func TestReconcile(t *testing.T) {
	type args struct {
		ctx context.Context
		req reconcile.Request
	}
	type want struct {
		res reconcile.Result
		err error
	}

	cases := map[string]struct {
		reason string
		r      reconcile.Reconciler
		args   args
		want   want
	}{
		"NotRateLimited": {
			reason: "Requests that are not rate limited should be passed to the inner Reconciler.",
			r: NewReconciler("test",
				reconcile.Func(func(c context.Context, r reconcile.Request) (reconcile.Result, error) {
					return reconcile.Result{Requeue: true}, nil
				}),
				&predictableRateLimiter{}),
			want: want{
				res: reconcile.Result{Requeue: true},
				err: nil,
			},
		},
		"RateLimited": {
			reason: "Requests that are rate limited should be requeued after the duration specified by the RateLimiter.",
			r:      NewReconciler("test", nil, &predictableRateLimiter{d: 8 * time.Second}),
			want: want{
				res: reconcile.Result{RequeueAfter: 8 * time.Second},
				err: nil,
			},
		},
		"Returning": {
			reason: "Returning requests that were previously rate limited should be allowed through without further rate limiting.",
			r: func() reconcile.Reconciler {
				inner := reconcile.Func(func(c context.Context, r reconcile.Request) (reconcile.Result, error) {
					return reconcile.Result{Requeue: true}, nil
				})

				// Rate limit the request once.
				r := NewReconciler("test", inner, &predictableRateLimiter{d: 8 * time.Second})
				r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "limited"}})
				return r

			}(),
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "limited"}},
			},
			want: want{
				res: reconcile.Result{Requeue: true},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.r.Reconcile(tc.args.ctx, tc.args.req)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nr.Reconcile(...): -want, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.res, got); diff != "" {
				t.Errorf("%s\nr.Reconcile(...): -want, +got result:\n%s", tc.reason, diff)
			}
		})
	}
}
