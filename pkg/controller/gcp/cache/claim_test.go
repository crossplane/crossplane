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

package cache

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	cachev1alpha1 "github.com/crossplaneio/crossplane/apis/cache/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/gcp/apis/cache/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
	"github.com/crossplaneio/crossplane/pkg/test"
)

var _ resource.ManagedConfigurator = resource.ManagedConfiguratorFn(ConfigureCloudMemorystoreInstance)

func TestConfigureCloudMemorystoreInstance(t *testing.T) {
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
	region := "cool-region"
	tier := "cool-tier"

	cases := map[string]struct {
		args args
		want want
	}{
		"Successful": {
			args: args{
				cm: &cachev1alpha1.RedisCluster{
					ObjectMeta: metav1.ObjectMeta{UID: claimUID},
					Spec:       cachev1alpha1.RedisClusterSpec{EngineVersion: "3.2"},
				},
				cs: &v1alpha1.CloudMemorystoreInstanceClass{
					SpecTemplate: v1alpha1.CloudMemorystoreInstanceClassSpecTemplate{
						ResourceClassSpecTemplate: corev1alpha1.ResourceClassSpecTemplate{
							ProviderReference: &corev1.ObjectReference{Name: providerName},
							ReclaimPolicy:     corev1alpha1.ReclaimDelete,
						},
						CloudMemorystoreInstanceParameters: v1alpha1.CloudMemorystoreInstanceParameters{
							Region: region,
							Tier:   tier,
						},
					},
				},
				mg: &v1alpha1.CloudMemorystoreInstance{},
			},
			want: want{
				mg: &v1alpha1.CloudMemorystoreInstance{
					Spec: v1alpha1.CloudMemorystoreInstanceSpec{
						ResourceSpec: corev1alpha1.ResourceSpec{
							ReclaimPolicy:                    corev1alpha1.ReclaimDelete,
							WriteConnectionSecretToReference: corev1.LocalObjectReference{Name: string(claimUID)},
							ProviderReference:                &corev1.ObjectReference{Name: providerName},
						},
						CloudMemorystoreInstanceParameters: v1alpha1.CloudMemorystoreInstanceParameters{
							RedisVersion: "REDIS_3_2",
							Region:       region,
							Tier:         tier,
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ConfigureCloudMemorystoreInstance(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ConfigureCloudMemorystoreInstance(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("ConfigureCloudMemorystoreInstance(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}
