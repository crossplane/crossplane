/*
Copyright 2019 The Crossplane Authors.

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

package database

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	databasev1alpha1 "github.com/crossplaneio/crossplane/apis/database/v1alpha1"
	"github.com/crossplaneio/crossplane/azure/apis/database/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
	"github.com/crossplaneio/crossplane/pkg/test"
)

var (
	_ resource.ManagedConfigurator = resource.ManagedConfiguratorFn(ConfigurePostgresqlServer)
	_ resource.ManagedConfigurator = resource.ManagedConfiguratorFn(ConfigureMysqlServer)
)

func TestConfigurePostgresqlServer(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  resource.Claim
		cs  resource.Class
		mg  resource.Managed
	}

	type want struct {
		mg  resource.Managed
		err error
	}

	claimUID := types.UID("definitely-a-uuid")
	providerName := "coolprovider"

	cases := map[string]struct {
		args args
		want want
	}{
		"Successful": {
			args: args{
				cm: &databasev1alpha1.PostgreSQLInstance{
					ObjectMeta: metav1.ObjectMeta{UID: claimUID},
					Spec:       databasev1alpha1.PostgreSQLInstanceSpec{EngineVersion: "9.6"},
				},
				cs: &v1alpha1.SQLServerClass{
					SpecTemplate: v1alpha1.SQLServerClassSpecTemplate{
						ResourceClassSpecTemplate: corev1alpha1.ResourceClassSpecTemplate{
							ProviderReference: &corev1.ObjectReference{Name: providerName},
							ReclaimPolicy:     corev1alpha1.ReclaimDelete,
						},
					},
				},
				mg: &v1alpha1.PostgresqlServer{},
			},
			want: want{
				mg: &v1alpha1.PostgresqlServer{
					Spec: v1alpha1.SQLServerSpec{
						ResourceSpec: corev1alpha1.ResourceSpec{
							ReclaimPolicy:                    corev1alpha1.ReclaimDelete,
							WriteConnectionSecretToReference: corev1.LocalObjectReference{Name: string(claimUID)},
							ProviderReference:                &corev1.ObjectReference{Name: providerName},
						},
						SQLServerParameters: v1alpha1.SQLServerParameters{
							Version: "9.6",
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ConfigurePostgresqlServer(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ConfigurePostgresqlServer(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("ConfigurePostgresqlServer(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConfigureMyPostgresqlServer(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  resource.Claim
		cs  resource.Class
		mg  resource.Managed
	}

	type want struct {
		mg  resource.Managed
		err error
	}

	claimUID := types.UID("definitely-a-uuid")
	providerName := "coolprovider"

	cases := map[string]struct {
		args args
		want want
	}{
		"Successful": {
			args: args{
				cm: &databasev1alpha1.MySQLInstance{
					ObjectMeta: metav1.ObjectMeta{UID: claimUID},
					Spec:       databasev1alpha1.MySQLInstanceSpec{EngineVersion: "5.6"},
				},
				cs: &v1alpha1.SQLServerClass{
					SpecTemplate: v1alpha1.SQLServerClassSpecTemplate{
						ResourceClassSpecTemplate: corev1alpha1.ResourceClassSpecTemplate{
							ProviderReference: &corev1.ObjectReference{Name: providerName},
							ReclaimPolicy:     corev1alpha1.ReclaimDelete,
						},
					},
				},
				mg: &v1alpha1.MysqlServer{},
			},
			want: want{
				mg: &v1alpha1.MysqlServer{
					Spec: v1alpha1.SQLServerSpec{
						ResourceSpec: corev1alpha1.ResourceSpec{
							ReclaimPolicy:                    corev1alpha1.ReclaimDelete,
							WriteConnectionSecretToReference: corev1.LocalObjectReference{Name: string(claimUID)},
							ProviderReference:                &corev1.ObjectReference{Name: providerName},
						},
						SQLServerParameters: v1alpha1.SQLServerParameters{
							Version: "5.6",
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ConfigureMysqlServer(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ConfigureMysqlServer(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("ConfigureMysqlServer(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}
