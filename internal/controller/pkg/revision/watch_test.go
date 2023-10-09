/*
Copyright 2021 The Crossplane Authors.

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

package revision

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

var (
	_ handler.EventHandler = &EnqueueRequestForReferencingProviderRevisions{}
)

type addFn func(item any)

func (fn addFn) Add(item any) {
	fn(item)
}

func TestAdd(t *testing.T) {
	errBoom := errors.New("boom")
	name := "coolname"
	prName := "coolpr"

	cases := map[string]struct {
		ctx              context.Context
		obj              runtime.Object
		client           client.Client
		controllerConfig *v1alpha1.ControllerConfig
		queue            adder
	}{
		"ObjectIsNotAControllConfig": {
			queue: addFn(func(_ any) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"ListError": {
			obj: &v1alpha1.ControllerConfig{ObjectMeta: metav1.ObjectMeta{Name: name}},
			client: &test.MockClient{
				MockList: test.NewMockListFn(errBoom),
			},
			controllerConfig: &v1alpha1.ControllerConfig{},
			queue:            addFn(func(_ any) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"SuccessfulEnqueue": {
			obj: &v1alpha1.ControllerConfig{ObjectMeta: metav1.ObjectMeta{Name: name}},
			client: &test.MockClient{
				MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
					l := obj.(*v1.ProviderRevisionList)
					l.Items = []v1.ProviderRevision{
						{
							ObjectMeta: metav1.ObjectMeta{Name: prName},
							Spec: v1.ProviderRevisionSpec{
								PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
									PackageRuntimeSpec: v1.PackageRuntimeSpec{
										ControllerConfigReference: &v1.ControllerConfigReference{},
									},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{Name: "noRef"},
						},
					}
					return nil
				}),
			},
			queue: addFn(func(got any) {
				want := reconcile.Request{NamespacedName: types.NamespacedName{Name: prName}}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("-want, +got:\n%s\n", diff)
				}
			}),
		},
	}

	for _, tc := range cases {
		e := &EnqueueRequestForReferencingProviderRevisions{client: tc.client}
		e.add(tc.ctx, tc.obj, tc.queue)
	}
}
