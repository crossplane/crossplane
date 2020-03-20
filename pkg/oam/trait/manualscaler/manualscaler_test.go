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

package manualscaler

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/oam/trait"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	oamv1alpha2 "github.com/crossplane/crossplane/apis/oam/v1alpha2"
)

var (
	startingReplicas int32 = 2
)

var _ trait.Modifier = trait.ModifyFn(DeploymentModifier)

func TestDeploymentModifier(t *testing.T) {
	type args struct {
		o runtime.Object
		t resource.Trait
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorObjectNotDeployment": {
			reason: "Object passed to modifier that is not a Deployment should return error.",
			args: args{
				o: &appsv1.DaemonSet{},
			},
			want: want{err: errors.New(errNotDeployment)},
		},
		"ErrorTraitNotManualScaler": {
			reason: "Trait passed to modifier that is not a ManualScalerTrait should return error.",
			args: args{
				o: &appsv1.Deployment{},
				t: &fake.Trait{},
			},
			want: want{err: errors.New(errNotManualScalerTrait)},
		},
		"Success": {
			reason: "A Deployment should have its replicas field changed on successful modification.",
			args: args{
				o: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Replicas: &startingReplicas,
					},
				},
				t: &oamv1alpha2.ManualScalerTrait{
					Spec: oamv1alpha2.ManualScalerTraitSpec{
						ReplicaCount: 3,
					},
				},
			},
			want: want{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := DeploymentModifier(context.Background(), tc.args.o, tc.args.t)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nDeploymentModifier(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
