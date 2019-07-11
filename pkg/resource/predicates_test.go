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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
)

type mockObject struct{ runtime.Object }

type mockClassReferencer struct {
	runtime.Object
	ref *corev1.ObjectReference
}

func (r *mockClassReferencer) GetClassReference() *corev1.ObjectReference  { return r.ref }
func (r *mockClassReferencer) SetClassReference(_ *corev1.ObjectReference) {}

func TestObjectHasProvisioner(t *testing.T) {
	type args struct {
		c           client.Client
		provisioner string
		obj         runtime.Object
	}

	cases := map[string]struct {
		args args
		want bool
	}{
		"NotAClassReferencer": {
			args: args{
				provisioner: "cool.example.org",
				obj:         &mockObject{},
			},
			want: false,
		},
		"NoClassReference": {
			args: args{
				provisioner: "cool.example.org",
				obj:         &mockClassReferencer{},
			},
			want: false,
		},
		"GetError": {
			args: args{
				c:           &test.MockClient{MockGet: test.NewMockGetFn(errors.New("boom"))},
				provisioner: "cool.example.org",
				obj:         &mockClassReferencer{ref: &corev1.ObjectReference{Name: "cool"}},
			},
			want: false,
		},
		"DifferentProvisioner": {
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						*(obj.(*v1alpha1.ResourceClass)) = v1alpha1.ResourceClass{Provisioner: "lame.example.org"}
						return nil
					},
				},
				provisioner: "cool.example.org",
				obj:         &mockClassReferencer{ref: &corev1.ObjectReference{Name: "cool"}},
			},
			want: false,
		},
		"SameProvisionerWithDifferentCase": {
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						*(obj.(*v1alpha1.ResourceClass)) = v1alpha1.ResourceClass{Provisioner: "Cool.example.org"}
						return nil
					},
				},
				provisioner: "cool.example.org",
				obj:         &mockClassReferencer{ref: &corev1.ObjectReference{Name: "cool"}},
			},
			want: true,
		},
		"IdenticalProvisioner": {
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
						*(obj.(*v1alpha1.ResourceClass)) = v1alpha1.ResourceClass{Provisioner: "cool.example.org"}
						return nil
					},
				},
				provisioner: "cool.example.org",
				obj:         &mockClassReferencer{ref: &corev1.ObjectReference{Name: "cool"}},
			},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fn := ObjectHasProvisioner(tc.args.c, tc.args.provisioner)
			got := fn(tc.args.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ObjectHasProvisioner(...): -want, +got:\n%s", diff)
			}
		})
	}
}
