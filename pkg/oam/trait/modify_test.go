/*
Copyright 2020 The Crossplane Authors.

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

package trait

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/oam/trait"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	workloadv1alpha1 "github.com/crossplane/crossplane/apis/workload/v1alpha1"
)

var (
	workloadName = "test-workload"
)

var _ trait.ModifyAccessor = DeploymentFromKubeAppAccessor

func TestDeploymentFromKubeAppAccessor(t *testing.T) {
	type args struct {
		o runtime.Object
		t resource.Trait
		m trait.ModifyFn
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorObjectIsNotKubeApp": {
			reason: "Object passed to accessor that is not a KubernetesApplication should return error.",
			args: args{
				o: &workloadv1alpha1.KubernetesApplicationResource{},
			},
			want: want{err: errors.New(errNotKubeApp)},
		},
		"ErrorNoMatchingDeployment": {
			reason: "Object passed to accessor that is not a KubernetesApplication should return error.",
			args: args{
				o: &workloadv1alpha1.KubernetesApplication{},
				t: &fake.Trait{},
			},
			want: want{err: errors.New(errNoDeploymentForTrait)},
		},
		"SuccessfulNoopModifier": {
			reason: "KubernetesApplication has matching Deployment and is modified successfully.",
			args: args{
				o: &workloadv1alpha1.KubernetesApplication{
					ObjectMeta: metav1.ObjectMeta{},
					Spec: workloadv1alpha1.KubernetesApplicationSpec{
						ResourceTemplates: []workloadv1alpha1.KubernetesApplicationResourceTemplate{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: workloadName,
								},
								Spec: workloadv1alpha1.KubernetesApplicationResourceSpec{
									Template: runtime.RawExtension{Raw: []byte(`{
										"kind":"Deployment",
										"apiVersion":"apps/v1"
									}`)},
								},
							},
						},
					},
				},
				t: &fake.Trait{
					WorkloadReferencer: fake.WorkloadReferencer{
						Ref: runtimev1alpha1.TypedReference{
							Name: workloadName,
						},
					},
				},
				m: trait.NoopModifier,
			},
			want: want{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := DeploymentFromKubeAppAccessor(context.Background(), tc.args.o, tc.args.t, tc.args.m)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nDeploymentFromKubeAppAccessor(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
