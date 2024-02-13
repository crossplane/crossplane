/*
Copyright 2024 The Crossplane Authors.

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

package claim

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/internal/names"
	"github.com/crossplane/crossplane/internal/xcrd"
)

func TestClientSideSync(t *testing.T) {
	errBoom := errors.New("boom")

	type params struct {
		c  client.Client
		ng names.NameGenerator
	}
	type args struct {
		ctx context.Context
		cm  *claim.Unstructured
		xr  *composite.Unstructured
	}
	type want struct {
		cm  *claim.Unstructured
		xr  *composite.Unstructured
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"WeirdClaimSpec": {
			reason: "We should return an error if the claim spec is not an object.",
			args: args{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					cm.Object["spec"] = 42
				}),
				xr: NewComposite(),
			},
			want: want{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					cm.Object["spec"] = 42
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyClaimNamespace: "default",
						xcrd.LabelKeyClaimName:      "cool-claim",
					})
				}),
				err: errors.New(errUnsupportedClaimSpec),
			},
		},
		"GenerateXRNameError": {
			reason: "We should return an error if we can't generate an XR name.",
			params: params{
				ng: names.NameGeneratorFn(func(_ context.Context, _ resource.Object) error {
					return errBoom
				}),
			},
			args: args{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")

					// To make sure the claim spec is an object.
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "some-composition",
					})
				}),
				xr: NewComposite(),
			},
			want: want{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "some-composition",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetGenerateName("cool-claim-")
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyClaimNamespace: "default",
						xcrd.LabelKeyClaimName:      "cool-claim",
					})
					xr.SetClaimReference(&claim.Reference{
						Namespace: "default",
						Name:      "cool-claim",
					})
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "some-composition",
					})
				}),
				err: errors.Wrap(errBoom, errGenerateName),
			},
		},
		"UpdateClaimResourceRefError": {
			reason: "We should return an error if we can't update the claim to persist its resourceRef.",
			params: params{
				ng: names.NameGeneratorFn(func(_ context.Context, cd resource.Object) error {
					cd.SetName("cool-claim-random")
					return nil
				}),
				c: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(errBoom),
				},
			},
			args: args{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")

					// To make sure the claim spec is an object.
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "some-composition",
					})
				}),
				xr: NewComposite(),
			},
			want: want{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "some-composition",
					})
					cm.SetResourceReference(&corev1.ObjectReference{
						Name: "cool-claim-random",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetGenerateName("cool-claim-")
					xr.SetName("cool-claim-random")
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyClaimNamespace: "default",
						xcrd.LabelKeyClaimName:      "cool-claim",
					})
					xr.SetClaimReference(&claim.Reference{
						Namespace: "default",
						Name:      "cool-claim",
					})
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "some-composition",
					})
				}),
				err: errors.Wrap(errBoom, errUpdateClaim),
			},
		},
		"ApplyXRError": {
			reason: "We should return an error if we can't apply the XR.",
			params: params{
				ng: names.NameGeneratorFn(func(_ context.Context, cd resource.Object) error {
					cd.SetName("cool-claim-random")
					return nil
				}),
				c: &test.MockClient{
					// Updating the claim should succeed.
					MockUpdate: test.NewMockUpdateFn(nil),
					// Applying the XR will first call Get, which should fail.
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			args: args{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")

					// To make sure the claim spec is an object.
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "some-composition",
					})
				}),
				xr: NewComposite(),
			},
			want: want{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "some-composition",
					})
					cm.SetResourceReference(&corev1.ObjectReference{
						Name: "cool-claim-random",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetGenerateName("cool-claim-")
					xr.SetName("cool-claim-random")
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyClaimNamespace: "default",
						xcrd.LabelKeyClaimName:      "cool-claim",
					})
					xr.SetClaimReference(&claim.Reference{
						Namespace: "default",
						Name:      "cool-claim",
					})
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "some-composition",
					})
				}),
				err: errors.Wrap(errors.Wrap(errBoom, "cannot get object"), errApplyComposite),
			},
		},
		"UpdateClaimStatusError": {
			reason: "We should return an error if we can't update the claim's status.",
			params: params{
				ng: names.NameGeneratorFn(func(_ context.Context, cd resource.Object) error {
					cd.SetName("cool-claim-random")
					return nil
				}),
				c: &test.MockClient{
					// Updating the claim should succeed.
					MockUpdate: test.NewMockUpdateFn(nil),
					// Applying the XR will call get.
					MockGet: test.NewMockGetFn(nil),
					// Updating the claim's status should fail.
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
				},
			},
			args: args{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")

					// To make sure the claim spec is an object.
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "some-composition",
					})
				}),
				xr: NewComposite(),
			},
			want: want{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "some-composition",
					})
					cm.SetResourceReference(&corev1.ObjectReference{
						Name: "cool-claim-random",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetGenerateName("cool-claim-")
					xr.SetName("cool-claim-random")
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyClaimNamespace: "default",
						xcrd.LabelKeyClaimName:      "cool-claim",
					})
					xr.SetClaimReference(&claim.Reference{
						Namespace: "default",
						Name:      "cool-claim",
					})
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "some-composition",
					})
				}),
				err: errors.Wrap(errBoom, errUpdateClaimStatus),
			},
		},
		"XRDoesNotExist": {
			reason: "We should create, bind, and sync with an XR when none exists.",
			params: params{
				ng: names.NameGeneratorFn(func(_ context.Context, cd resource.Object) error {
					cd.SetName("cool-claim-random")
					return nil
				}),
				c: &test.MockClient{
					// Updating the claim the first time to set the resourceRef
					// should succeed. The second time we update it should fail.
					MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
						if obj.(*claim.Unstructured).GetCompositionSelector() != nil {
							return errBoom
						}
						return nil
					}),
					// Applying the XR will call get.
					MockGet: test.NewMockGetFn(nil),
					// Updating the claim's status should succeed.
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
			},
			args: args{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					meta.SetExternalName(cm, "external-name")

					// Kube stuff should not be propagated to the XR.
					cm.SetLabels(map[string]string{
						"k8s.io/some-label": "filter-me-out",
					})
					cm.SetAnnotations(map[string]string{
						"kubernetes.io/some-anno":  "filter-me-out",
						"example.org/propagate-me": "true",
					})

					// To make sure the claim spec is an object.
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "some-composition",
					})

					// A user-defined spec field we should propagate.
					cm.Object["spec"].(map[string]any)["propagateMe"] = "true"
				}),
				// A non-existent, empty XR.
				xr: NewComposite(),
			},
			want: want{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					meta.SetExternalName(cm, "external-name")

					cm.SetLabels(map[string]string{
						"k8s.io/some-label": "filter-me-out",
					})
					cm.SetAnnotations(map[string]string{
						"kubernetes.io/some-anno":  "filter-me-out",
						"example.org/propagate-me": "true",
					})

					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "some-composition",
					})
					cm.SetResourceReference(&corev1.ObjectReference{
						Name: "cool-claim-random",
					})

					cm.Object["spec"].(map[string]any)["propagateMe"] = "true"
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetGenerateName("cool-claim-")
					xr.SetName("cool-claim-random")
					meta.SetExternalName(xr, "external-name")

					xr.SetLabels(map[string]string{
						xcrd.LabelKeyClaimNamespace: "default",
						xcrd.LabelKeyClaimName:      "cool-claim",
					})
					xr.SetAnnotations(map[string]string{
						"example.org/propagate-me": "true",
					})

					xr.SetClaimReference(&claim.Reference{
						Namespace: "default",
						Name:      "cool-claim",
					})
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "some-composition",
					})

					xr.Object["spec"].(map[string]any)["propagateMe"] = "true"
				}),
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := NewClientSideCompositeSyncer(tc.params.c, tc.params.ng)
			err := s.Sync(tc.args.ctx, tc.args.cm, tc.args.xr)

			if diff := cmp.Diff(tc.want.cm, tc.args.cm); diff != "" {
				t.Errorf("\n%s\ns.Sync(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.xr, tc.args.xr); diff != "" {
				t.Errorf("\n%s\ns.Sync(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Sync(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

type CompositeModifier func(xr *composite.Unstructured)

func NewComposite(m ...CompositeModifier) *composite.Unstructured {
	xr := composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{}))
	for _, fn := range m {
		fn(xr)
	}
	return xr
}
