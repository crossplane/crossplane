// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package initializer

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestDeploymentRuntimeConfigObject(t *testing.T) {
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
		"FailedToCreate": {
			args: args{
				kube: &test.MockClient{
					MockCreate: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
						return errBoom
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errCreateDefaultRuntimeConfig),
			},
		},
		"SuccessCreated": {
			args: args{
				kube: &test.MockClient{
					MockCreate: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
						return nil
					},
				},
			},
		},
		"SuccessAlreadyExists": {
			args: args{
				kube: &test.MockClient{
					MockCreate: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
						return kerrors.NewAlreadyExists(schema.GroupResource{}, "default")
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := DefaultDeploymentRuntimeConfig(context.TODO(), tc.args.kube)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", name, diff)
			}
		})
	}
}
