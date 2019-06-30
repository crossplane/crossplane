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

package database

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp/database/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
	"github.com/crossplaneio/crossplane/pkg/test"
)

var (
	_ resource.ManagedConfigurator = resource.ManagedConfiguratorFn(ConfigurePostgreCloudsqlInstance)
	_ resource.ManagedConfigurator = resource.ManagedConfiguratorFn(ConfigureMyCloudsqlInstance)
)

func TestConfigurePostgreCloudsqlInstance(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  resource.Claim
		cs  *corev1alpha1.ResourceClass
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
				cm: &storagev1alpha1.PostgreSQLInstance{
					ObjectMeta: metav1.ObjectMeta{UID: claimUID},
					Spec:       storagev1alpha1.PostgreSQLInstanceSpec{EngineVersion: "9.6"},
				},
				cs: &corev1alpha1.ResourceClass{
					ProviderReference: &corev1.ObjectReference{Name: providerName},
					ReclaimPolicy:     corev1alpha1.ReclaimDelete,
				},
				mg: &v1alpha1.CloudsqlInstance{},
			},
			want: want{
				mg: &v1alpha1.CloudsqlInstance{
					Spec: v1alpha1.CloudsqlInstanceSpec{
						ResourceSpec: corev1alpha1.ResourceSpec{
							ReclaimPolicy:                    corev1alpha1.ReclaimDelete,
							WriteConnectionSecretToReference: corev1.LocalObjectReference{Name: string(claimUID)},
							ProviderReference:                &corev1.ObjectReference{Name: providerName},
						},
						DatabaseVersion: "POSTGRES_9_6",
						Labels:          map[string]string{},
						StorageGB:       v1alpha1.DefaultStorageGB,
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ConfigurePostgreCloudsqlInstance(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ConfigurePostgreCloudsqlInstance(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("ConfigurePostgreCloudsqlInstance(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConfigureMyCloudsqlInstance(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  resource.Claim
		cs  *corev1alpha1.ResourceClass
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
				cm: &storagev1alpha1.MySQLInstance{
					ObjectMeta: metav1.ObjectMeta{UID: claimUID},
					Spec:       storagev1alpha1.MySQLInstanceSpec{EngineVersion: "5.6"},
				},
				cs: &corev1alpha1.ResourceClass{
					ProviderReference: &corev1.ObjectReference{Name: providerName},
					ReclaimPolicy:     corev1alpha1.ReclaimDelete,
				},
				mg: &v1alpha1.CloudsqlInstance{},
			},
			want: want{
				mg: &v1alpha1.CloudsqlInstance{
					Spec: v1alpha1.CloudsqlInstanceSpec{
						ResourceSpec: corev1alpha1.ResourceSpec{
							ReclaimPolicy:                    corev1alpha1.ReclaimDelete,
							WriteConnectionSecretToReference: corev1.LocalObjectReference{Name: string(claimUID)},
							ProviderReference:                &corev1.ObjectReference{Name: providerName},
						},
						DatabaseVersion: "MYSQL_5_6",
						Labels:          map[string]string{},
						StorageGB:       v1alpha1.DefaultStorageGB,
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ConfigureMyCloudsqlInstance(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ConfigureMyCloudsqlInstance(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("ConfigureMyCloudsqlInstance(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}
