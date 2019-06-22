/*
Copyright 2018 The Crossplane Authors.

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

package resource

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
)

var (
	_ ManagedConfigurator = ManagedConfiguratorFn(ConfigureObjectMeta)
	_ ManagedConfigurator = ConfiguratorChain{}
)

func TestConfiguratorChain(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		ctx context.Context
		cm  Claim
		cs  *v1alpha1.ResourceClass
		mg  Managed
	}

	cases := map[string]struct {
		cc   ConfiguratorChain
		args args
		want error
	}{
		"EmptyChain": {
			cc: ConfiguratorChain{},
			args: args{
				ctx: context.Background(),
				cm:  &MockClaim{},
				cs:  &v1alpha1.ResourceClass{},
				mg:  &MockManaged{},
			},
			want: nil,
		},
		"SuccessulConfigurator": {
			cc: ConfiguratorChain{
				ManagedConfiguratorFn(func(_ context.Context, _ Claim, _ *v1alpha1.ResourceClass, _ Managed) error {
					return nil
				}),
			},
			args: args{
				ctx: context.Background(),
				cm:  &MockClaim{},
				cs:  &v1alpha1.ResourceClass{},
				mg:  &MockManaged{},
			},
			want: nil,
		},
		"ConfiguratorReturnsError": {
			cc: ConfiguratorChain{
				ManagedConfiguratorFn(func(_ context.Context, _ Claim, _ *v1alpha1.ResourceClass, _ Managed) error {
					return errBoom
				}),
			},
			args: args{
				ctx: context.Background(),
				cm:  &MockClaim{},
				cs:  &v1alpha1.ResourceClass{},
				mg:  &MockManaged{},
			},
			want: errBoom,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.cc.Configure(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("tc.cc.Configure(...): -want error, +got error:\n%s", diff)
			}
		})
	}
}

func TestConfigureObjectMeta(t *testing.T) {
	ns := "namespace"
	uid := types.UID("definitely-a-uuid")

	type args struct {
		ctx context.Context
		cm  Claim
		cs  *v1alpha1.ResourceClass
		mg  Managed
	}

	type want struct {
		err       error
		namespace string
		name      string
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"Successful": {
			args: args{
				ctx: context.Background(),
				cm:  &MockClaim{ObjectMeta: metav1.ObjectMeta{Namespace: ns, UID: uid}},
				cs:  &v1alpha1.ResourceClass{},
				mg:  &MockManaged{},
			},
			want: want{
				namespace: ns,
				name:      strings.ToLower(reflect.TypeOf(MockClaim{}).Name() + "-" + string(uid)),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ConfigureObjectMeta(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, got, test.EquateErrors()); diff != "" {
				t.Errorf("ConfigureObjectMeta(...): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.namespace, tc.args.mg.GetNamespace()); diff != "" {
				t.Errorf("tc.args.mg.GetNamespace(...): -want, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.name, tc.args.mg.GetName()); diff != "" {
				t.Errorf("tc.args.mg.GetName(...): -want, +got error:\n%s", diff)
			}
		})
	}
}
