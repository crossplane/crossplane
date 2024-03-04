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

package namespace

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var _ handler.EventHandler = &EnqueueRequestForNamespaces{}

type addFn func(item any)

func (fn addFn) Add(item any) {
	fn(item)
}

func TestAdd(t *testing.T) {
	name := "coolname"

	cases := map[string]struct {
		client client.Reader
		ctx    context.Context
		obj    runtime.Object
		queue  adder
	}{
		"ObjectIsNotAClusterRole": {
			queue: addFn(func(_ any) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"ClusterRoleIsNotAggregated": {
			obj:   &rbacv1.ClusterRole{},
			queue: addFn(func(_ any) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"ListNamespacesError": {
			client: &test.MockClient{
				MockList: test.NewMockListFn(errors.New("boom")),
			},
			obj:   &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{keyAggToAdmin: valTrue}}},
			queue: addFn(func(_ any) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"SuccessfulEnqueue": {
			client: &test.MockClient{
				MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
					nsl := o.(*corev1.NamespaceList)
					*nsl = corev1.NamespaceList{Items: []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: name}}}}
					return nil
				}),
			},
			obj: &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{keyAggToAdmin: valTrue}}},
			queue: addFn(func(got any) {
				want := reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("-want, +got:\n%s\n", diff)
				}
			}),
		},
	}

	for _, tc := range cases {
		e := &EnqueueRequestForNamespaces{client: tc.client}
		e.add(tc.ctx, tc.obj, tc.queue)
	}
}
