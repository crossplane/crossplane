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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

func TestSilentlyRequeueOnConflict(t *testing.T) {
	type args struct {
		result reconcile.Result
		err    error
	}

	type want struct {
		result reconcile.Result
		err    error
	}

	tests := []struct {
		reason string
		args   args
		want   want
	}{
		{
			reason: "nil error",
			args: args{
				result: reconcile.Result{RequeueAfter: time.Second},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: time.Second},
			},
		},
		{
			reason: "other error",
			args: args{
				result: reconcile.Result{RequeueAfter: time.Second},
				err:    New("some other error"),
			},
			want: want{
				result: reconcile.Result{RequeueAfter: time.Second},
				err:    New("some other error"),
			},
		},
		{
			reason: "conflict error",
			args: args{
				result: reconcile.Result{RequeueAfter: time.Second},
				err:    kerrors.NewConflict(schema.GroupResource{Group: "nature", Resource: "stones"}, "foo", New("nested error")),
			},
			want: want{
				result: reconcile.Result{Requeue: true},
			},
		},
		{
			reason: "nested conflict error",
			args: args{
				result: reconcile.Result{RequeueAfter: time.Second},
				err: Wrap(
					kerrors.NewConflict(schema.GroupResource{Group: "nature", Resource: "stones"}, "foo", New("nested error")),
					"outer error"),
			},
			want: want{
				result: reconcile.Result{Requeue: true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			got, err := SilentlyRequeueOnConflict(tt.args.result, tt.args.err)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nIgnoreConflict(...): -want error, +got error:\n%s", tt.reason, diff)
			}

			if diff := cmp.Diff(tt.want.result, got); diff != "" {
				t.Errorf("\n%s\nIgnoreConflict(...): -want result, +got result:\n%s", tt.reason, diff)
			}
		})
	}
}
