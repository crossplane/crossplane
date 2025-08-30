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

package manager

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
)

func TestHasPullSecret(t *testing.T) {
	cases := map[string]struct {
		reason string
		ic     *v1beta1.ImageConfig
		want   bool
	}{
		"NilRegistry": {
			reason: "Should return false when Registry is nil",
			ic: &v1beta1.ImageConfig{
				Spec: v1beta1.ImageConfigSpec{},
			},
			want: false,
		},
		"NilAuthentication": {
			reason: "Should return false when Authentication is nil",
			ic: &v1beta1.ImageConfig{
				Spec: v1beta1.ImageConfigSpec{
					Registry: &v1beta1.RegistryConfig{},
				},
			},
			want: false,
		},
		"EmptyPullSecretName": {
			reason: "Should return false when PullSecretRef name is empty",
			ic: &v1beta1.ImageConfig{
				Spec: v1beta1.ImageConfigSpec{
					Registry: &v1beta1.RegistryConfig{
						Authentication: &v1beta1.RegistryAuthentication{
							PullSecretRef: corev1.LocalObjectReference{Name: ""},
						},
					},
				},
			},
			want: false,
		},
		"HasPullSecret": {
			reason: "Should return true when PullSecretRef name is set",
			ic: &v1beta1.ImageConfig{
				Spec: v1beta1.ImageConfigSpec{
					Registry: &v1beta1.RegistryConfig{
						Authentication: &v1beta1.RegistryAuthentication{
							PullSecretRef: corev1.LocalObjectReference{Name: "my-secret"},
						},
					},
				},
			},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := hasPullSecret(tc.ic)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nhasPullSecret(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestHasRewriteRules(t *testing.T) {
	cases := map[string]struct {
		reason string
		ic     *v1beta1.ImageConfig
		want   bool
	}{
		"NilRewriteImage": {
			reason: "Should return false when RewriteImage is nil",
			ic: &v1beta1.ImageConfig{
				Spec: v1beta1.ImageConfigSpec{},
			},
			want: false,
		},
		"HasRewriteRules": {
			reason: "Should return true when RewriteImage is set",
			ic: &v1beta1.ImageConfig{
				Spec: v1beta1.ImageConfigSpec{
					RewriteImage: &v1beta1.ImageRewrite{
						Prefix: "xpkg.crossplane.io/crossplane-contrib/",
					},
				},
			},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := hasRewriteRules(tc.ic)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nhasRewriteRules(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// testEnqueuePackagesForImageConfig tests the handler by calling it with mock events and queues
func TestEnqueuePackagesForImageConfig(t *testing.T) {
	errBoom := errors.New("boom")

	type params struct {
		kube client.Client
		l    v1.PackageList
		log  logging.Logger
		obj  client.Object
	}

	type want struct {
		reqs []reconcile.Request
	}

	cases := map[string]struct {
		reason string
		params params
		want   want
	}{
		"NotImageConfig": {
			reason: "Should not enqueue when object is not an ImageConfig",
			params: params{
				obj: &v1.Provider{},
			},
			want: want{
				reqs: nil,
			},
		},
		"NoPullSecretOrRewriteRules": {
			reason: "Should not enqueue when ImageConfig has neither pull secret nor rewrite rules",
			params: params{
				obj: &v1beta1.ImageConfig{
					Spec: v1beta1.ImageConfigSpec{},
				},
			},
			want: want{
				reqs: nil,
			},
		},
		"ErrorListingPackages": {
			reason: "Should not enqueue when listing packages fails",
			params: params{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				},
				l:   &v1.ProviderList{},
				log: logging.NewNopLogger(),
				obj: &v1beta1.ImageConfig{
					Spec: v1beta1.ImageConfigSpec{
						Registry: &v1beta1.RegistryConfig{
							Authentication: &v1beta1.RegistryAuthentication{
								PullSecretRef: corev1.LocalObjectReference{Name: "my-secret"},
							},
						},
					},
				},
			},
			want: want{
				reqs: nil,
			},
		},
		"SuccessfulWithPullSecret": {
			reason: "Should enqueue matching packages when ImageConfig has pull secret",
			params: params{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1.ProviderList)
						list.Items = []v1.Provider{
							{
								ObjectMeta: metav1.ObjectMeta{Name: "provider-aws"},
								Spec: v1.ProviderSpec{
									PackageSpec: v1.PackageSpec{
										Package: "xpkg.upbound.io/upbound/provider-aws:v1.0.0",
									},
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{Name: "provider-gcp"},
								Spec: v1.ProviderSpec{
									PackageSpec: v1.PackageSpec{
										Package: "xpkg.crossplane.io/crossplane-contrib/provider-gcp:v1.0.0",
									},
								},
							},
						}
						return nil
					}),
				},
				l:   &v1.ProviderList{},
				log: logging.NewNopLogger(),
				obj: &v1beta1.ImageConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "test-config"},
					Spec: v1beta1.ImageConfigSpec{
						MatchImages: []v1beta1.ImageMatch{
							{Prefix: "xpkg.upbound.io/upbound/"},
						},
						Registry: &v1beta1.RegistryConfig{
							Authentication: &v1beta1.RegistryAuthentication{
								PullSecretRef: corev1.LocalObjectReference{Name: "my-secret"},
							},
						},
					},
				},
			},
			want: want{
				reqs: []reconcile.Request{
					{NamespacedName: types.NamespacedName{Name: "provider-aws"}},
				},
			},
		},
		"SuccessfulWithRewriteRules": {
			reason: "Should enqueue matching packages when ImageConfig has rewrite rules",
			params: params{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1.ProviderList)
						list.Items = []v1.Provider{
							{
								ObjectMeta: metav1.ObjectMeta{Name: "provider-aws"},
								Spec: v1.ProviderSpec{
									PackageSpec: v1.PackageSpec{
										Package: "xpkg.upbound.io/upbound/provider-aws:v1.0.0",
									},
								},
								Status: v1.ProviderStatus{
									PackageStatus: v1.PackageStatus{
										ResolvedPackage: "xpkg.crossplane.io/crossplane-contrib/provider-aws:v1.0.0",
									},
								},
							},
						}
						return nil
					}),
				},
				l:   &v1.ProviderList{},
				log: logging.NewNopLogger(),
				obj: &v1beta1.ImageConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "test-config"},
					Spec: v1beta1.ImageConfigSpec{
						MatchImages: []v1beta1.ImageMatch{
							{Prefix: "xpkg.crossplane.io/crossplane-contrib/"},
						},
						RewriteImage: &v1beta1.ImageRewrite{
							Prefix: "xpkg.crossplane.io/crossplane-contrib/",
						},
					},
				},
			},
			want: want{
				reqs: []reconcile.Request{
					{NamespacedName: types.NamespacedName{Name: "provider-aws"}},
				},
			},
		},
		"NoMatchingPackages": {
			reason: "Should not enqueue when no packages match the prefix",
			params: params{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1.ProviderList)
						list.Items = []v1.Provider{
							{
								ObjectMeta: metav1.ObjectMeta{Name: "provider-aws"},
								Spec: v1.ProviderSpec{
									PackageSpec: v1.PackageSpec{
										Package: "xpkg.crossplane.io/crossplane-contrib/provider-aws:v1.0.0",
									},
								},
							},
						}
						return nil
					}),
				},
				l:   &v1.ProviderList{},
				log: logging.NewNopLogger(),
				obj: &v1beta1.ImageConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "test-config"},
					Spec: v1beta1.ImageConfigSpec{
						MatchImages: []v1beta1.ImageMatch{
							{Prefix: "xpkg.upbound.io/upbound/"},
						},
						Registry: &v1beta1.RegistryConfig{
							Authentication: &v1beta1.RegistryAuthentication{
								PullSecretRef: corev1.LocalObjectReference{Name: "my-secret"},
							},
						},
					},
				},
			},
			want: want{
				reqs: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create the handler
			handler := EnqueuePackagesForImageConfig(tc.params.kube, tc.params.l, tc.params.log)

			// Create a mock workqueue to capture enqueued requests
			mockQueue := &MockWorkQueue{}

			// Create a create event
			event := event.CreateEvent{
				Object: tc.params.obj,
			}

			// Call the handler's Create method
			ctx := context.Background()
			handler.Create(ctx, event, mockQueue)

			// Check what was enqueued
			if diff := cmp.Diff(tc.want.reqs, mockQueue.requests); diff != "" {
				t.Errorf("\n%s\nEnqueuePackagesForImageConfig(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// MockWorkQueue implements workqueue.TypedRateLimitingInterface for testing
type MockWorkQueue struct {
	workqueue.TypedRateLimitingInterface[reconcile.Request]
	requests []reconcile.Request
}

func (m *MockWorkQueue) Add(item reconcile.Request) {
	m.requests = append(m.requests, item)
}

