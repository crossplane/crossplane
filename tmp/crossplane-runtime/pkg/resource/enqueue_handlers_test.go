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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
)

var _ handler.EventHandler = &EnqueueRequestForProviderConfig{}

type addFn func(item any)

func (fn addFn) Add(item reconcile.Request) {
	fn(item)
}

func TestAddProviderConfig(t *testing.T) {
	name := "coolname"

	cases := map[string]struct {
		handler *EnqueueRequestForProviderConfig
		obj     runtime.Object
		queue   adder
	}{
		"NotProviderConfigReferencer": {
			handler: &EnqueueRequestForProviderConfig{},
			queue:   addFn(func(_ any) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"IsLegacyProviderConfigReferencer": {
			handler: &EnqueueRequestForProviderConfig{},
			obj: &fake.LegacyProviderConfigUsage{
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
		"IsTypedProviderConfigReferencer": {
			handler: &EnqueueRequestForProviderConfig{},
			obj: &fake.ProviderConfigUsage{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-pcu",
					Namespace: "foo",
				},
				RequiredTypedProviderConfigReferencer: fake.RequiredTypedProviderConfigReferencer{
					Ref: xpv1.ProviderConfigReference{Name: name, Kind: "ProviderConfig"},
				},
			},
			queue: addFn(func(got any) {
				want := reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: "foo"}}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("-want, +got:\n%s", diff)
				}
			}),
		},
		"ClusterScopedProviderConfigOmitsNamespace": {
			handler: &EnqueueRequestForProviderConfig{Kind: "ClusterProviderConfig"},
			obj: &fake.ProviderConfigUsage{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-pcu",
					Namespace: "foo",
				},
				RequiredTypedProviderConfigReferencer: fake.RequiredTypedProviderConfigReferencer{
					Ref: xpv1.ProviderConfigReference{Name: name, Kind: "ClusterProviderConfig"},
				},
			},
			queue: addFn(func(got any) {
				want := reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("-want, +got:\n%s", diff)
				}
			}),
		},
		"KindFilterMatchesKind": {
			handler: &EnqueueRequestForProviderConfig{Kind: "ProviderConfig"},
			obj: &fake.ProviderConfigUsage{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-pcu",
					Namespace: "bar",
				},
				RequiredTypedProviderConfigReferencer: fake.RequiredTypedProviderConfigReferencer{
					Ref: xpv1.ProviderConfigReference{Name: name, Kind: "ProviderConfig"},
				},
			},
			queue: addFn(func(got any) {
				want := reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: "bar"}}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("-want, +got:\n%s", diff)
				}
			}),
		},
		"KindFilterSkipsNonMatchingKind": {
			handler: &EnqueueRequestForProviderConfig{Kind: "ProviderConfig"},
			obj: &fake.ProviderConfigUsage{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-pcu",
					Namespace: "bar",
				},
				RequiredTypedProviderConfigReferencer: fake.RequiredTypedProviderConfigReferencer{
					Ref: xpv1.ProviderConfigReference{Name: name, Kind: "OtherProviderConfig"},
				},
			},
			queue: addFn(func(_ any) { t.Errorf("queue.Add() called unexpectedly for non-matching kind") }),
		},
		"EmptyRefKindDefaultsToProviderConfig": {
			handler: &EnqueueRequestForProviderConfig{Kind: "ProviderConfig"},
			obj: &fake.ProviderConfigUsage{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-pcu",
					Namespace: "baz",
				},
				RequiredTypedProviderConfigReferencer: fake.RequiredTypedProviderConfigReferencer{
					Ref: xpv1.ProviderConfigReference{Name: name, Kind: ""},
				},
			},
			queue: addFn(func(got any) {
				want := reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: "baz"}}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("-want, +got:\n%s", diff)
				}
			}),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(_ *testing.T) {
			tc.handler.addProviderConfig(tc.obj, tc.queue)
		})
	}
}
