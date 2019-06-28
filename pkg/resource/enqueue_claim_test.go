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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	_ handler.EventHandler = &EnqueueRequestForClaim{}
)

type addFn func(item interface{})

func (fn addFn) Add(item interface{}) {
	fn(item)
}

func TestAddClaim(t *testing.T) {
	ns := "coolns"
	name := "coolname"

	cases := map[string]struct {
		obj   runtime.Object
		queue adder
	}{
		"ObjectIsNotAClaimReferencer": {
			obj:   &corev1.Pod{},
			queue: addFn(func(_ interface{}) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"ObjectHasNilClaimReference": {
			obj:   &MockManaged{},
			queue: addFn(func(_ interface{}) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"ObjectHasClaimReference": {
			obj: &MockManaged{MockClaimReferencer: MockClaimReferencer{Ref: &corev1.ObjectReference{
				Namespace: ns,
				Name:      name,
			}}},
			queue: addFn(func(got interface{}) {
				want := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("-want, +got:\n%s", diff)
				}

			}),
		},
	}

	for _, tc := range cases {
		addClaim(tc.obj, tc.queue)
	}
}
