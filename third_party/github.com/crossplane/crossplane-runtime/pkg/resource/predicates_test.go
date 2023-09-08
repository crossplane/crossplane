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

package resource

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
)

func TestDesiredStateChanged(t *testing.T) {
	type args struct {
		old client.Object
		new client.Object
	}
	type want struct {
		desiredStateChanged bool
	}
	cases := map[string]struct {
		args
		want
	}{
		"NothingChanged": {
			args: args{
				old: func() client.Object {
					mg := &fake.Managed{}
					return mg
				}(),
				new: func() client.Object {
					mg := &fake.Managed{}
					return mg
				}(),
			},
			want: want{
				desiredStateChanged: false,
			},
		},
		"StatusChanged": {
			args: args{
				old: func() client.Object {
					mg := &fake.Managed{}
					return mg
				}(),
				new: func() client.Object {
					mg := &fake.Managed{}
					mg.SetConditions(runtimev1.ReconcileSuccess())
					return mg
				}(),
			},
			want: want{
				desiredStateChanged: false,
			},
		},
		"IgnoredAnnotationsChanged": {
			args: args{
				old: func() client.Object {
					mg := &fake.Managed{}
					return mg
				}(),
				new: func() client.Object {
					mg := &fake.Managed{}
					mg.SetAnnotations(map[string]string{meta.AnnotationKeyExternalCreatePending: time.Now().String()})
					return mg
				}(),
			},
			want: want{
				desiredStateChanged: false,
			},
		},
		"AnnotationsChanged": {
			args: args{
				old: func() client.Object {
					mg := &fake.Managed{}
					return mg
				}(),
				new: func() client.Object {
					mg := &fake.Managed{}
					mg.SetAnnotations(map[string]string{"foo": "bar"})
					return mg
				}(),
			},
			want: want{
				desiredStateChanged: true,
			},
		},
		"LabelsChanged": {
			args: args{
				old: func() client.Object {
					mg := &fake.Managed{}
					return mg
				}(),
				new: func() client.Object {
					mg := &fake.Managed{}
					mg.SetLabels(map[string]string{"foo": "bar"})
					return mg
				}(),
			},
			want: want{
				desiredStateChanged: true,
			},
		},
		// This happens when spec is changed.
		"GenerationChanged": {
			args: args{
				old: func() client.Object {
					mg := &fake.Managed{}
					mg.SetGeneration(1)
					return mg
				}(),
				new: func() client.Object {
					mg := &fake.Managed{}
					mg.SetGeneration(2)
					return mg
				}(),
			},
			want: want{
				desiredStateChanged: true,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := DesiredStateChanged().Update(event.UpdateEvent{
				ObjectOld: tc.args.old,
				ObjectNew: tc.args.new,
			})

			if diff := cmp.Diff(tc.want.desiredStateChanged, got); diff != "" {
				t.Errorf("DesiredStateChanged(...): -want, +got:\n%s", diff)
			}
		})
	}
}
