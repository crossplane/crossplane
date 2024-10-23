/*
Copyright 2024 The Crossplane Authors.

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

package resolver

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

func TestHasPullSecret(t *testing.T) {
	cases := map[string]struct {
		reason string
		o      client.Object
		want   bool
	}{
		"NotAnImageConfig": {
			reason: "Objects that aren't an ImageConfig should be filtered.",
			o:      &fake.Object{},
			want:   true,
		},
		"ImageConfigWithoutPullSecret": {
			reason: "An ImageConfig without a pull secret should be filtered.",
			o:      &v1beta1.ImageConfig{},
			want:   true,
		},
		"ImageConfigWithPullSecret": {
			reason: "An ImageConfig with a pull secret shouldn't be filtered.",
			o: &v1beta1.ImageConfig{
				Spec: v1beta1.ImageConfigSpec{
					Registry: &v1beta1.RegistryConfig{
						Authentication: &v1beta1.RegistryAuthentication{
							PullSecretRef: v1.LocalObjectReference{
								Name: "supersecret",
							},
						},
					},
				},
			},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := HasPullSecret()(tc.o)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nHasPullsecret(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestForName(t *testing.T) {
	type args struct {
		name string
		fns  []FilterFn
	}
	cases := map[string]struct {
		reason string
		args   args
		want   []reconcile.Request
	}{
		"NoFilters": {
			reason: "A request should be enqueued for the named object if there are no filter functions.",
			args: args{
				name: "cool-object",
			},
			want: []reconcile.Request{{NamespacedName: client.ObjectKey{Name: "cool-object"}}},
		},
		"AllFiltersReturnFalse": {
			reason: "A request should be enqueued for the named object if all filter functions return false.",
			args: args{
				name: "cool-object",
				fns:  []FilterFn{func(_ client.Object) bool { return false }},
			},
			want: []reconcile.Request{{NamespacedName: client.ObjectKey{Name: "cool-object"}}},
		},
		"OneFilterReturnsTrue": {
			reason: "A request shouldn't be enqueued for the named object if any filter functions return true.",
			args: args{
				name: "cool-object",
				fns: []FilterFn{
					func(_ client.Object) bool { return false },
					func(_ client.Object) bool { return true },
					func(_ client.Object) bool { return false },
				},
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ForName(tc.args.name, tc.args.fns...)(context.Background(), &fake.Object{})

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nForName(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
