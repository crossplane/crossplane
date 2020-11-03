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

package offered

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

var (
	_ handler.EventHandler = &EnqueueRequestForClaim{}
)

func TestOffersClaim(t *testing.T) {
	cases := map[string]struct {
		obj  runtime.Object
		want bool
	}{
		"NotAnXRD": {
			want: false,
		},
		"DoesNotOfferClaim": {
			obj:  &v1alpha1.CompositeResourceDefinition{},
			want: false,
		},
		"OffersClaim": {
			obj: &v1alpha1.CompositeResourceDefinition{
				Spec: v1alpha1.CompositeResourceDefinitionSpec{
					// An XRD with non-nil claim names offers a claim.
					ClaimNames: &extv1.CustomResourceDefinitionNames{},
				},
			},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := OffersClaim()(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("OffersClaim(...): -want, +got:\n%s", diff)
			}
		})
	}
}

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
		"ObjectIsNotAComposite": {
			queue: addFn(func(_ interface{}) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"ObjectHasNilClaimReference": {
			obj:   composite.New(),
			queue: addFn(func(_ interface{}) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"ObjectHasClaimReference": {
			obj: func() runtime.Object {
				cp := composite.New()
				cp.SetClaimReference(&corev1.ObjectReference{Namespace: ns, Name: name})
				return &cp.Unstructured
			}(),
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

func TestAddControllersClaim(t *testing.T) {
	errBoom := errors.New("boom")
	ctrl := true
	ns := "coolns"
	name := "coolname"

	cases := map[string]struct {
		client client.Reader
		obj    runtime.Object
		queue  adder
	}{
		"ObjectIsNotASecret": {
			queue: addFn(func(_ interface{}) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"ObjectIsNotAConnectioNSecret": {
			queue: addFn(func(_ interface{}) { t.Errorf("queue.Add() called unexpectedly") }),
			obj:   &corev1.Secret{},
		},
		"ObjectHasNoController": {
			queue: addFn(func(_ interface{}) { t.Errorf("queue.Add() called unexpectedly") }),
			obj:   &corev1.Secret{Type: resource.SecretTypeConnection},
		},
		"GetControllerError": {
			queue: addFn(func(_ interface{}) { t.Errorf("queue.Add() called unexpectedly") }),
			client: &test.MockClient{
				MockGet: test.NewMockGetFn(errBoom),
			},
			obj: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "v",
					Kind:       "k",
					Name:       "n",
					Controller: &ctrl,
				}}},
				Type: resource.SecretTypeConnection,
			},
		},
		"ControllerHasNilClaimReference": {
			queue: addFn(func(_ interface{}) { t.Errorf("queue.Add() called unexpectedly") }),
			client: &test.MockClient{
				MockGet: test.NewMockGetFn(nil),
			},
			obj: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "v",
					Kind:       "k",
					Name:       "n",
					Controller: &ctrl,
				}}},
				Type: resource.SecretTypeConnection,
			},
		},
		"ControllerHasClaimReference": {
			queue: addFn(func(got interface{}) {
				want := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("-want, +got:\n%s", diff)
				}
			}),
			client: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
					cp := composite.New()
					cp.SetClaimReference(&corev1.ObjectReference{Namespace: ns, Name: name})
					*obj.(*composite.Unstructured) = *cp
					return nil
				}),
			},
			obj: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "v",
					Kind:       "k",
					Name:       "n",
					Controller: &ctrl,
				}}},
				Type: resource.SecretTypeConnection,
			},
		},
	}

	for _, tc := range cases {
		e := &EnqueueRequestForControllersClaim{client: tc.client}
		e.addClaim(tc.obj, tc.queue)
	}
}
