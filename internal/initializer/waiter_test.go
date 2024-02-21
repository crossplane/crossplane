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
	"time"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestCRDWaiter(t *testing.T) {
	type args struct {
		names   []string
		timeout time.Duration
		period  time.Duration
		kube    client.Client
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"SuccessFirstRun": {
			args: args{
				names:   []string{"arbitrary.crd.name"},
				period:  1 * time.Second,
				timeout: 2 * time.Second,
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return nil
					},
				},
			},
		},
		"Timeout": {
			args: args{
				names:   []string{"arbitrary.crd.name"},
				timeout: 2 * time.Millisecond,
				period:  1 * time.Millisecond,
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, _ client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
				},
			},
			want: want{
				err: errors.Errorf(errFmtTimeoutExceeded, 2*time.Millisecond.Seconds()),
			},
		},
		"FailGet": {
			args: args{
				names:   []string{"arbitrary.crd.name"},
				period:  1 * time.Millisecond,
				timeout: 1 * time.Second,
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return errBoom
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetCRD),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			i := NewCRDWaiter(tc.args.names, tc.args.timeout, tc.args.period, logging.NewNopLogger())
			err := i.Run(context.TODO(), tc.args.kube)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", name, diff)
			}
		})
	}
}
