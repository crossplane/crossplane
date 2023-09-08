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

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
)

var (
	_ handler.EventHandler = &EnqueueRequestForProviderConfig{}
)

type addFn func(item any)

func (fn addFn) Add(item any) {
	fn(item)
}

func TestAddProviderConfig(t *testing.T) {
	name := "coolname"

	cases := map[string]struct {
		obj   runtime.Object
		queue adder
	}{
		"NotProviderConfigReferencer": {
			queue: addFn(func(_ any) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"IsProviderConfigReferencer": {
			obj: &fake.ProviderConfigUsage{
				RequiredProviderConfigReferencer: fake.RequiredProviderConfigReferencer{
					Ref: xpv1.Reference{Name: name},
				},
			},
			queue: addFn(func(got any) {
				want := reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("-want, +got:\n%s", diff)
				}
			}),
		},
	}

	for _, tc := range cases {
		addProviderConfig(tc.obj, tc.queue)
	}
}
