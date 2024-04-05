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
	"encoding/json"
	"testing"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/internal/names"
	"github.com/crossplane/crossplane/internal/xcrd"
)

func TestServerSideSync(t *testing.T) {
	errBoom := errors.New("boom")
	now := metav1.Now()

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
					cm.SetResourceReference(&corev1.ObjectReference{
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
					cm.SetResourceReference(&corev1.ObjectReference{
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
					cm.SetResourceReference(&corev1.ObjectReference{
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
					cm.SetResourceReference(&corev1.ObjectReference{
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
					cm.SetResourceReference(&corev1.ObjectReference{
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
					cm.SetResourceReference(&corev1.ObjectReference{
						Name: "existing-composite",
					})
				}),
				xr: NewComposite(func(xr *composite.Unstructured) {
					xr.SetName("existing-composite")
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyClaimNamespace: "default",
						xcrd.LabelKeyClaimName:      "cool-claim",
					})
					xr.SetClaimReference(&claim.Reference{
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
					cm.SetResourceReference(&corev1.ObjectReference{
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
					cm.SetResourceReference(&corev1.ObjectReference{
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
					xr.SetClaimReference(&claim.Reference{
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
					cm.SetResourceReference(&corev1.ObjectReference{
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
					xr.SetClaimReference(&claim.Reference{
						Namespace: "default",
						Name:      "cool-claim",
					})
					xr.Object["status"] = map[string]any{
						"userDefinedField": "status",
					}
				}),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := NewServerSideCompositeSyncer(tc.params.c, tc.params.ng)
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

func TestStripManagedFields(t *testing.T) {
	cases := map[string]struct {
		reason        string
		originalPatch []byte
		expectedPatch []byte
		err           error
	}{
		"StripManagedFields": {
			reason: "should strip spec.resourceRefs field from ssa claim manager managed fields entries",
			originalPatch: []byte(`
[
	{
		"op":    "replace",
		"path":  "/metadata/managedFields",
		"value": [{"apiVersion":"nop.example.org/v1alpha1","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:finalizers":{".":{},"v:\"composite.apiextensions.crossplane.io\"":{}},"f:generateName":{},"f:labels":{".":{},"f:crossplane.io/claim-name":{},"f:crossplane.io/claim-namespace":{},"f:crossplane.io/composite":{}}},"f:spec":{".":{},"f:claimRef":{".":{},"f:apiVersion":{},"f:kind":{},"f:name":{},"f:namespace":{}},"f:compositionRef":{".":{},"f:name":{}},"f:compositionRevisionRef":{".":{},"f:name":{}},"f:compositionUpdatePolicy":{},"f:coolField":{},"f:resourceRefs":{}}},"manager":"apiextensions.crossplane.io/claim","operation":"Apply","time":"2024-03-21T22:13:23Z"}]
	},
	{
		"op":    "replace",
		"path":  "/metadata/resourceVersion",
		"value": "28726594"
	}
]
`),
			expectedPatch: []byte(`
[
	{
		"op":    "replace",
		"path":  "/metadata/managedFields",
		"value": [{"apiVersion":"nop.example.org/v1alpha1","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:finalizers":{".":{},"v:\"composite.apiextensions.crossplane.io\"":{}},"f:generateName":{},"f:labels":{".":{},"f:crossplane.io/claim-name":{},"f:crossplane.io/claim-namespace":{},"f:crossplane.io/composite":{}}},"f:spec":{".":{},"f:claimRef":{".":{},"f:apiVersion":{},"f:kind":{},"f:name":{},"f:namespace":{}},"f:compositionRef":{".":{},"f:name":{}},"f:compositionRevisionRef":{".":{},"f:name":{}},"f:compositionUpdatePolicy":{},"f:coolField":{}}},"manager":"apiextensions.crossplane.io/claim","operation":"Apply","time":"2024-03-21T22:13:23Z"}]
	},
	{
		"op":    "replace",
		"path":  "/metadata/resourceVersion",
		"value": "28726594"
	}
]
`),
		},
		"StripManagedFieldsNoop": {
			reason: "should not strip spec.resourceRefs field from ssa composite manager managed fields entries",
			originalPatch: []byte(`
[
	{
		"op":    "replace",
		"path":  "/metadata/managedFields",
		"value": [{"apiVersion":"nop.example.org/v1alpha1","fieldsType":"FieldsV1","fieldsV1":{"f:spec":{"f:resourceRefs":{}}},"manager":"apiextensions.crossplane.io/composite","operation":"Apply","time":"2024-04-03T00:28:10Z"},{"apiVersion":"nop.example.org/v1alpha1","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:labels":{"f:crossplane.io/claim-name":{},"f:crossplane.io/claim-namespace":{}}},"f:spec":{"f:claimRef":{"f:apiVersion":{},"f:kind":{},"f:name":{},"f:namespace":{}},"f:compositionRef":{"f:name":{}},"f:compositionSelector":{"f:matchLabels":{"f:xr-template-source":{}}},"f:compositionUpdatePolicy":{},"f:coolField":{},"f:testConfig":{"f:cilium":{".":{},"f:enabled":{},"f:mode":{}},"f:eks_version":{},"f:environment":{},"f:identity_provider":{".":{},"f:enabled":{},"f:groups":{}},"f:region":{}}}},"manager":"apiextensions.crossplane.io/claim","operation":"Apply","time":"2024-04-03T00:37:30Z"}]
	},
	{
		"op":    "replace",
		"path":  "/metadata/resourceVersion",
		"value": "28726594"
	}
]
`),
			expectedPatch: []byte(`
[
	{
		"op":    "replace",
		"path":  "/metadata/managedFields",
		"value": [{"apiVersion":"nop.example.org/v1alpha1","fieldsType":"FieldsV1","fieldsV1":{"f:spec":{"f:resourceRefs":{}}},"manager":"apiextensions.crossplane.io/composite","operation":"Apply","time":"2024-04-03T00:28:10Z"},{"apiVersion":"nop.example.org/v1alpha1","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:labels":{"f:crossplane.io/claim-name":{},"f:crossplane.io/claim-namespace":{}}},"f:spec":{"f:claimRef":{"f:apiVersion":{},"f:kind":{},"f:name":{},"f:namespace":{}},"f:compositionRef":{"f:name":{}},"f:compositionSelector":{"f:matchLabels":{"f:xr-template-source":{}}},"f:compositionUpdatePolicy":{},"f:coolField":{},"f:testConfig":{"f:cilium":{".":{},"f:enabled":{},"f:mode":{}},"f:eks_version":{},"f:environment":{},"f:identity_provider":{".":{},"f:enabled":{},"f:groups":{}},"f:region":{}}}},"manager":"apiextensions.crossplane.io/claim","operation":"Apply","time":"2024-04-03T00:37:30Z"}]
	},
	{
		"op":    "replace",
		"path":  "/metadata/resourceVersion",
		"value": "28726594"
	}
]
`),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := stripManagedFields(tc.originalPatch)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nstripManagedFields: -want error, +got error:\n%s", tc.reason, diff)
			}

			jp, err := jsonpatch.DecodePatch(p)
			if err != nil {
				t.Error(err)
			}
			v, err := jp[0].ValueInterface()
			if err != nil {
				t.Error(err)
			}
			es := v.([]interface{})
			managedFields := make([]metav1.ManagedFieldsEntry, len(es))
			for i := range es {
				e, err := json.Marshal(es[i])
				if err != nil {
					t.Error(err)
				}
				err = json.Unmarshal(e, &managedFields[i])
				if err != nil {
					t.Error(err)
				}
			}

			jp, err = jsonpatch.DecodePatch(tc.expectedPatch)
			if err != nil {
				t.Error(err)
			}
			v, err = jp[0].ValueInterface()
			if err != nil {
				t.Error(err)
			}
			es = v.([]interface{})
			expectedManagedFields := make([]metav1.ManagedFieldsEntry, len(es))
			for i := range es {
				e, err := json.Marshal(es[i])
				if err != nil {
					t.Error(err)
				}
				err = json.Unmarshal(e, &expectedManagedFields[i])
				if err != nil {
					t.Error(err)
				}
			}

			if diff := cmp.Diff(expectedManagedFields, managedFields); diff != "" {
				t.Errorf("\n%s\nstripManagedFields: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
