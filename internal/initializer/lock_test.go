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

package initializer

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestLockObject(t *testing.T) {
	type args struct {
		kube client.Client
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"SuccessAlreadyExists": {
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return nil
					},
					MockPatch: func(_ context.Context, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return nil
					},
				},
			},
		},
		"FailApply": {
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return errBoom
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, "cannot get object"), errApplyLock),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := NewLockObject().Run(context.TODO(), tc.args.kube)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", name, diff)
			}
		})
	}
}
