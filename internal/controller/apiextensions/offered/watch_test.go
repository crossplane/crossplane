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
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/reference"

	v2 "github.com/crossplane/crossplane/apis/apiextensions/v2"
)

var _ handler.EventHandler = &EnqueueRequestForClaim{}

func TestOffersClaim(t *testing.T) {
	cases := map[string]struct {
		obj  runtime.Object
		want bool
	}{
		"NotAnXRD": {
			want: false,
		},
		"CRD": {
			obj:  &extv1.CustomResourceDefinition{},
			want: false,
		},
		"DoesNotOfferClaim": {
			obj:  &v2.CompositeResourceDefinition{},
			want: false,
		},
		"OffersClaim": {
			obj: &v2.CompositeResourceDefinition{
				Spec: v2.CompositeResourceDefinitionSpec{
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
				t.Errorf("\n%s\nOffersClaim(...): -want, +got:\n%s", name, diff)
			}
		})
	}
}

func TestIsClaimCRD(t *testing.T) {
	cases := map[string]struct {
		obj  runtime.Object
		want bool
	}{
		"NotCRD": {
			want: false,
		},
		"XRD": {
			obj:  &v2.CompositeResourceDefinition{},
			want: false,
		},
		"ClaimCRD": {
			obj: &extv1.CustomResourceDefinition{
				Spec: extv1.CustomResourceDefinitionSpec{
					Names: extv1.CustomResourceDefinitionNames{
						Categories: []string{
							"claim",
						},
					},
				},
			},
			want: true,
		},
		"CompositeCRD": {
			obj: &extv1.CustomResourceDefinition{
				Spec: extv1.CustomResourceDefinitionSpec{
					Names: extv1.CustomResourceDefinitionNames{
						Categories: []string{
							"composite",
						},
					},
				},
			},
			want: false,
		},
		"OtherCRD": {
			obj:  &extv1.CustomResourceDefinition{},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsClaimCRD()(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nIsClaimCRD(...): -want, +got:\n%s", name, diff)
			}
		})
	}
}

type addFn func(item reconcile.Request)

func (fn addFn) Add(item reconcile.Request) {
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
			queue: addFn(func(_ reconcile.Request) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"ObjectHasNilClaimReference": {
			obj:   composite.New(),
			queue: addFn(func(_ reconcile.Request) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"ObjectHasClaimReference": {
			obj: func() runtime.Object {
				cp := composite.New()
				cp.SetClaimReference(&reference.Claim{Namespace: ns, Name: name})
				return &cp.Unstructured
			}(),
			queue: addFn(func(got reconcile.Request) {
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
