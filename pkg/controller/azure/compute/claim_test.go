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

package compute

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
	computev1alpha1 "github.com/crossplaneio/crossplane/apis/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/azure/apis/compute/v1alpha1"
)

var _ resource.ManagedConfigurator = resource.ManagedConfiguratorFn(ConfigureAKSCluster)

func TestConfigureAKSCluster(t *testing.T) {
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
	nodeCount := 1

	cases := map[string]struct {
		args args
		want want
	}{
		"Successful": {
			args: args{
				cm: &computev1alpha1.KubernetesCluster{ObjectMeta: metav1.ObjectMeta{UID: claimUID}},
				cs: &v1alpha1.AKSClusterClass{
					SpecTemplate: v1alpha1.AKSClusterClassSpecTemplate{
						ResourceClassSpecTemplate: runtimev1alpha1.ResourceClassSpecTemplate{
							ProviderReference: &corev1.ObjectReference{Name: providerName},
							ReclaimPolicy:     runtimev1alpha1.ReclaimDelete,
						},
					},
				},
				mg: &v1alpha1.AKSCluster{},
			},
			want: want{
				mg: &v1alpha1.AKSCluster{
					Spec: v1alpha1.AKSClusterSpec{
						ResourceSpec: runtimev1alpha1.ResourceSpec{
							ReclaimPolicy:                    runtimev1alpha1.ReclaimDelete,
							WriteConnectionSecretToReference: corev1.LocalObjectReference{Name: string(claimUID)},
							ProviderReference:                &corev1.ObjectReference{Name: providerName},
						},
						AKSClusterParameters: v1alpha1.AKSClusterParameters{
							NodeCount:                     &nodeCount,
							WriteServicePrincipalSecretTo: corev1.LocalObjectReference{Name: fmt.Sprintf("principal-%s", claimUID)},
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ConfigureAKSCluster(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ConfigureAKSCluster(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("ConfigureAKSCluster(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}
