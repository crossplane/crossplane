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

package roles

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	_ handler.EventHandler = &EnqueueRequestIfNamed{}
)

type addFn func(item interface{})

func (fn addFn) Add(item interface{}) {
	fn(item)
}

func TestAdd(t *testing.T) {
	name := "coolname"

	cases := map[string]struct {
		obj   runtime.Object
		name  string
		queue adder
	}{
		"ObjectIsNotAMetaObject": {
			queue: addFn(func(_ interface{}) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"WrongName": {
			obj:   &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "wat"}},
			name:  name,
			queue: addFn(func(_ interface{}) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"SuccessfulEnqueue": {
			obj:  &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name}},
			name: name,
			queue: addFn(func(got interface{}) {
				want := reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("-want, +got:\n%s\n", diff)
				}
			}),
		},
	}

	for _, tc := range cases {
		e := &EnqueueRequestIfNamed{Name: tc.name}
		e.add(tc.obj, tc.queue)
	}
}
