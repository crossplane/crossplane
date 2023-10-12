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

	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
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
			obj:  &v1.CompositeResourceDefinition{},
			want: false,
		},
		"OffersClaim": {
			obj: &v1.CompositeResourceDefinition{
				Spec: v1.CompositeResourceDefinitionSpec{
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

type addFn func(item any)

func (fn addFn) Add(item any) {
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
			queue: addFn(func(_ any) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"ObjectHasNilClaimReference": {
			obj:   composite.New(),
			queue: addFn(func(_ any) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"ObjectHasClaimReference": {
			obj: func() runtime.Object {
				cp := composite.New()
				cp.SetClaimReference(&claim.Reference{Namespace: ns, Name: name})
				return &cp.Unstructured
			}(),
			queue: addFn(func(got any) {
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
