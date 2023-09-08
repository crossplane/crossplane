/*
Copyright 2022 The Crossplane Authors.

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

package webhook

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

// Mutator has to satisfy CustomDefaulter interface so that it can be used by
// controller-runtime Manager.
var _ webhook.CustomDefaulter = &Mutator{}

func TestDefault(t *testing.T) {
	type args struct {
		obj runtime.Object
		fns []MutateFn
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"Success": {
			reason: "Functions without errors should be executed successfully",
			args: args{
				fns: []MutateFn{
					func(_ context.Context, _ runtime.Object) error {
						return nil
					},
				},
			},
		},
		"Failure": {
			reason: "Functions with errors should return with error",
			args: args{
				fns: []MutateFn{
					func(_ context.Context, _ runtime.Object) error {
						return errBoom
					},
				},
			},
			want: want{
				err: errBoom,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			v := NewMutator(WithMutationFns(tc.fns...))
			err := v.Default(context.TODO(), tc.args.obj)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDefault(...): -want, +got\n%s\n", tc.reason, diff)
			}
		})
	}
}
