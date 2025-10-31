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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/reference"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane/crossplane/v2/internal/names"
	"github.com/crossplane/crossplane/v2/internal/xcrd"
)

func TestServerSideSync(t *testing.T) {
	errBoom := errors.New("boom")
	now := metav1.Now()

	type params struct {
		c  client.Client
		ng names.NameGenerator
	}

	type args struct {
		ctx                    context.Context
		cm                     *claim.Unstructured
		xr                     *composite.Unstructured
		hasEnforcedComposition bool
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
				}),
				xr: NewComposite(),
			},
			want: want{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
				}),
				xr:  NewComposite(),
				err: errors.Wrap(errBoom, errGenerateName),
			},
		},
		"WeirdClaimSpec": {
			reason: "We should return an error if the claim spec is not an object.",
			params: params{
				ng: names.NameGeneratorFn(func(_ context.Context, _ resource.Object) error {
					return nil
				}),
			},
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
				xr:  NewComposite(),
				err: errors.New(errUnsupportedClaimSpec),
			},
		},
		"UpdateClaimError": {
			reason: "We should return an error if we can't update the claim.",
			params: params{
				c: &test.MockClient{
					// Fail to update the claim.
					MockUpdate: test.NewMockUpdateFn(errBoom),
				},
				ng: names.NameGeneratorFn(func(_ context.Context, _ resource.Object) error {
					return nil
				}),
			},
			args: args{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")

					// To make sure the claim spec is an object.
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "cool-composition",
					})
				}),
			},
			want: want{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})

					// Back-propagated from the XR.
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "cool-composition",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "cool-composition",
					})
				}),
				err: errors.Wrap(errBoom, errUpdateClaim),
			},
		},
		"ApplyXRError": {
			reason: "We should return an error if we can't apply (i.e. patch) the XR.",
			params: params{
				c: &test.MockClient{
					// Update the claim.
					MockUpdate: test.NewMockUpdateFn(nil),

					// Fail to patch the XR.
					MockPatch: test.NewMockPatchFn(errBoom),
				},
				ng: names.NameGeneratorFn(func(_ context.Context, _ resource.Object) error {
					return nil
				}),
			},
			args: args{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")

					// To make sure the claim spec is an object.
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "cool-composition",
					})
				}),
			},
			want: want{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})

					// Back-propagated from the XR.
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "cool-composition",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "cool-composition",
					})
				}),
				err: errors.Wrap(errBoom, errApplyComposite),
			},
		},
		"WeirdXRStatus": {
			reason: "We should return an error if the XR status is not an object.",
			params: params{
				c: &test.MockClient{
					// Update the claim.
					MockUpdate: test.NewMockUpdateFn(nil),

					// Patch the XR. We reset the XR passed to Sync to the
					// result of this patch, so we need to make its status
					// something other than an object here.
					MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
						obj.(*composite.Unstructured).Object["status"] = 42
						return nil
					}),
				},
				ng: names.NameGeneratorFn(func(_ context.Context, _ resource.Object) error {
					return nil
				}),
			},
			args: args{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")

					// To make sure the claim spec is an object.
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
				}),
			},
			want: want{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyClaimNamespace: "default",
						xcrd.LabelKeyClaimName:      "cool-claim",
					})
					xr.SetClaimReference(&reference.Claim{
						Namespace: "default",
						Name:      "cool-claim",
					})
					xr.Object["status"] = 42
				}),
				err: errors.New(errUnsupportedCompositeStatus),
			},
		},
		"UpdateClaimStatusError": {
			reason: "We should return an error if we can't update the claim's status.",
			params: params{
				c: &test.MockClient{
					// Update the claim.
					MockUpdate: test.NewMockUpdateFn(nil),

					// Patch the XR. Make sure it has a valid status.
					MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
						obj.(*composite.Unstructured).SetConditions(xpv1.Creating())
						return nil
					}),

					// Fail to update the claim's status.
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
				},
				ng: names.NameGeneratorFn(func(_ context.Context, _ resource.Object) error {
					return nil
				}),
			},
			args: args{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")

					// To make sure the claim spec is an object.
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
				}),
			},
			want: want{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})
					cm.Object["status"] = map[string]any{}
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyClaimNamespace: "default",
						xcrd.LabelKeyClaimName:      "cool-claim",
					})
					xr.SetClaimReference(&reference.Claim{
						Namespace: "default",
						Name:      "cool-claim",
					})
					xr.SetConditions(xpv1.Creating())
				}),
				err: errors.Wrap(errBoom, errUpdateClaimStatus),
			},
		},
		"XRDoesNotExist": {
			reason: "We should create, bind, and sync with an XR when none exists.",
			params: params{
				c: &test.MockClient{
					// Update the claim.
					MockUpdate: test.NewMockUpdateFn(nil),

					// Patch the XR. Make sure it has a valid status.
					MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
						obj.(*composite.Unstructured).Object["status"] = map[string]any{
							"userDefinedField": "status",
						}
						return nil
					}),

					// Update the claim's status.
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				ng: names.NameGeneratorFn(func(_ context.Context, cd resource.Object) error {
					// Generate a name for the XR.
					cd.SetName("cool-claim-random")
					return nil
				}),
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

					// Make sure user-defined fields are propagated to the XR.
					cm.Object["spec"] = map[string]any{
						"userDefinedField": "spec",
					}

					// Make sure these don't get lost when we propagate status
					// from the XR.
					cm.SetConditions(xpv1.ReconcileSuccess(), Waiting())
					cm.SetConnectionDetailsLastPublishedTime(&now)
				}),
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
					cm.Object["spec"] = map[string]any{
						"userDefinedField": "spec",
					}
					cm.SetResourceReference(&reference.Composite{
						Name: "cool-claim-random",
					})
					cm.Object["status"] = map[string]any{
						"userDefinedField": "status",
					}
					cm.SetConditions(xpv1.ReconcileSuccess(), Waiting())
					cm.SetConnectionDetailsLastPublishedTime(&now)
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
					xr.Object["spec"] = map[string]any{
						"userDefinedField": "spec",
					}
					xr.SetClaimReference(&reference.Claim{
						Namespace: "default",
						Name:      "cool-claim",
					})
					xr.Object["status"] = map[string]any{
						"userDefinedField": "status",
					}
				}),
			},
		},
		"XRExists": {
			reason: "When the XR already exists, we should ensure we preserve any custom status conditions.",
			params: params{
				c: &test.MockClient{
					// Update the claim.
					MockUpdate: test.NewMockUpdateFn(nil),

					// The XR already exists and has custom conditions.
					MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
						*obj.(*composite.Unstructured) = *NewComposite(func(xr *composite.Unstructured) {
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
							xr.Object["spec"] = map[string]any{
								"userDefinedField": "spec",
							}
							xr.SetClaimReference(&reference.Claim{
								Namespace: "default",
								Name:      "cool-claim",
							})
							xr.Object["status"] = map[string]any{
								"userDefinedField": "status",
								// Types of custom conditions that were copied from the
								// Composite to the Claim.
								"claimConditionTypes": []string{"ExampleCustomStatus"},
								"conditions": []xpv1.Condition{
									{
										Type:               "ExampleCustomStatus",
										Status:             "True",
										Reason:             "SomeReason",
										Message:            "Example message.",
										ObservedGeneration: 20,
									},
								},
							}
						})
						return nil
					}),

					// Update the claim's status.
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				ng: names.NameGeneratorFn(func(_ context.Context, cd resource.Object) error {
					// Generate a name for the XR.
					cd.SetName("cool-claim-random")
					return nil
				}),
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

					// Make sure user-defined fields are propagated to the XR.
					cm.Object["spec"] = map[string]any{
						"userDefinedField": "spec",
					}

					// Make sure these don't get lost when we propagate status
					// from the XR.
					cm.SetConditions(
						// Crossplane system conditions.
						xpv1.ReconcileSuccess(),
						Waiting(),
						// User custom conditions from the Composite.
						xpv1.Condition{
							Type:               "ExampleCustomStatus",
							Status:             "True",
							Reason:             "SomeReason",
							Message:            "Example message.",
							ObservedGeneration: 20,
						},
					)
					cm.SetConnectionDetailsLastPublishedTime(&now)
				}),
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
					cm.Object["spec"] = map[string]any{
						"userDefinedField": "spec",
					}
					cm.SetResourceReference(&reference.Composite{
						Name: "cool-claim-random",
					})
					cm.Object["status"] = map[string]any{
						"userDefinedField": "status",
					}
					cm.SetConditions(
						xpv1.ReconcileSuccess(),
						Waiting(),
						xpv1.Condition{
							Type:               "ExampleCustomStatus",
							Status:             "True",
							Reason:             "SomeReason",
							Message:            "Example message.",
							ObservedGeneration: 20,
						},
					)
					cm.SetConnectionDetailsLastPublishedTime(&now)
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
					xr.Object["spec"] = map[string]any{
						"userDefinedField": "spec",
					}
					xr.SetClaimReference(&reference.Claim{
						Namespace: "default",
						Name:      "cool-claim",
					})
					xr.Object["status"] = map[string]any{
						"userDefinedField":    "status",
						"claimConditionTypes": []string{"ExampleCustomStatus"},
						"conditions": []xpv1.Condition{
							{
								Type:               "ExampleCustomStatus",
								Status:             "True",
								Reason:             "SomeReason",
								Message:            "Example message.",
								ObservedGeneration: 20,
							},
						},
					}
				}),
			},
		},
		"EnforcedCompositionSkipsClaimToXRPropagation": {
			reason: "When hasEnforcedComposition is true, compositionRef should NOT be propagated from claim to XR.",
			params: params{
				c: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
					MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
						// The XR patch should NOT have compositionRef
						xr := obj.(*composite.Unstructured)
						if ref := xr.GetCompositionReference(); ref != nil {
							t.Errorf("XR patch should not have compositionRef when enforced, got: %v", ref)
						}
						// Mock: preserve the XR's existing compositionRef (simulating SSA behavior)
						xr.SetCompositionReference(&corev1.ObjectReference{Name: "enforced-composition"})
						return nil
					}),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				ng: names.NameGeneratorFn(func(_ context.Context, _ resource.Object) error {
					return nil
				}),
			},
			args: args{
				hasEnforcedComposition: true,
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					// Claim has a compositionRef that should NOT be propagated
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "claim-composition",
					})
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
					// XR has the enforced composition
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "enforced-composition",
					})
				}),
			},
			want: want{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					// Claim's compositionRef should be updated to match XR's enforced value
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "enforced-composition",
					})
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyClaimNamespace: "default",
						xcrd.LabelKeyClaimName:      "cool-claim",
					})
					xr.SetClaimReference(&reference.Claim{
						Namespace: "default",
						Name:      "cool-claim",
					})
					// XR's compositionRef should remain the enforced value
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "enforced-composition",
					})
				}),
			},
		},
		"EnforcedCompositionForcesXRToClaimPropagation": {
			reason: "When hasEnforcedComposition is true, XR's compositionRef should ALWAYS overwrite claim's compositionRef.",
			params: params{
				c: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
					MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
						// Mock: preserve the XR's enforced compositionRef
						xr := obj.(*composite.Unstructured)
						xr.SetCompositionReference(&corev1.ObjectReference{Name: "enforced-composition"})
						return nil
					}),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				ng: names.NameGeneratorFn(func(_ context.Context, _ resource.Object) error {
					return nil
				}),
			},
			args: args{
				hasEnforcedComposition: true,
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					// Claim has WRONG compositionRef
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "wrong-composition",
					})
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
					// XR has the CORRECT enforced composition
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "enforced-composition",
					})
				}),
			},
			want: want{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					// Claim's compositionRef should be OVERWRITTEN with XR's value
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "enforced-composition",
					})
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyClaimNamespace: "default",
						xcrd.LabelKeyClaimName:      "cool-claim",
					})
					xr.SetClaimReference(&reference.Claim{
						Namespace: "default",
						Name:      "cool-claim",
					})
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "enforced-composition",
					})
				}),
			},
		},
		"NoEnforcedCompositionUsesNormalBehavior": {
			reason: "When hasEnforcedComposition is false, compositionRef should be propagated from claim to XR normally.",
			params: params{
				c: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
					MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
						// The XR patch SHOULD have compositionRef from claim
						xr := obj.(*composite.Unstructured)
						ref := xr.GetCompositionReference()
						if ref == nil || ref.Name != "claim-composition" {
							t.Errorf("XR patch should have claim's compositionRef when not enforced, got: %v", ref)
						}
						// Keep the compositionRef in the patch
						return nil
					}),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				ng: names.NameGeneratorFn(func(_ context.Context, _ resource.Object) error {
					return nil
				}),
			},
			args: args{
				hasEnforcedComposition: false,
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					// Claim has a compositionRef that SHOULD be propagated
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "claim-composition",
					})
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
				}),
			},
			want: want{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					// Claim's compositionRef should remain unchanged
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "claim-composition",
					})
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyClaimNamespace: "default",
						xcrd.LabelKeyClaimName:      "cool-claim",
					})
					xr.SetClaimReference(&reference.Claim{
						Namespace: "default",
						Name:      "cool-claim",
					})
					// XR should have received claim's compositionRef
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "claim-composition",
					})
				}),
			},
		},
		"NoEnforcedCompositionDoesNotOverwriteClaimCompositionRef": {
			reason: "When hasEnforcedComposition is false and claim already has compositionRef, it should NOT be overwritten by XR's compositionRef.",
			params: params{
				c: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockPatch:        test.NewMockPatchFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				ng: names.NameGeneratorFn(func(_ context.Context, _ resource.Object) error {
					return nil
				}),
			},
			args: args{
				hasEnforcedComposition: false,
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					// Claim already has a compositionRef
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "claim-composition",
					})
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
					// XR has a different compositionRef
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "xr-composition",
					})
				}),
			},
			want: want{
				cm: NewClaim(func(cm *claim.Unstructured) {
					cm.SetNamespace("default")
					cm.SetName("cool-claim")
					// Claim's compositionRef should NOT be overwritten
					cm.SetCompositionReference(&corev1.ObjectReference{
						Name: "claim-composition",
					})
					cm.SetResourceReference(&reference.Composite{
						Name: "existing-composite",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyClaimNamespace: "default",
						xcrd.LabelKeyClaimName:      "cool-claim",
					})
					xr.SetClaimReference(&reference.Claim{
						Namespace: "default",
						Name:      "cool-claim",
					})
					xr.SetCompositionReference(&corev1.ObjectReference{
						Name: "claim-composition",
					})
				}),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := NewServerSideCompositeSyncer(tc.params.c, tc.params.ng)
			err := s.Sync(tc.args.ctx, tc.args.cm, tc.args.xr, tc.args.hasEnforcedComposition)

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
