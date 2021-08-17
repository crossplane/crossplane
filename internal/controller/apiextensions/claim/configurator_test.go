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

package claim

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/internal/xcrd"
)

func TestCompositeConfigure(t *testing.T) {
	ns := "spacename"
	name := "cool"
	apiVersion := "v"
	kind := "k"
	now := metav1.Now()
	errBoom := errors.New("boom")

	type args struct {
		ctx context.Context
		cm  resource.CompositeClaim
		cp  resource.Composite
	}

	type want struct {
		cp  resource.Composite
		err error
	}

	cases := map[string]struct {
		reason string
		c      client.Client
		args   args
		want   want
	}{
		"ClaimNotUnstructured": {
			reason: "We should return early if the claim is not unstructured",
			args: args{
				cm: &fake.CompositeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ns,
						Name:      name,
					},
				},
				cp: &fake.Composite{},
			},
			want: want{
				cp: &fake.Composite{},
			},
		},
		"CompositeNotUnstructured": {
			reason: "We should return early if the composite is not unstructured",
			args: args{
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"namespace": ns,
								"name":      name,
							},
						},
					},
				},
				cp: &fake.Composite{},
			},
			want: want{
				cp: &fake.Composite{},
			},
		},
		"UnsupportedSpecError": {
			reason: "We should return early if the claim's spec is not an unstructured object",
			args: args{
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"namespace": ns,
								"name":      name,
							},
							"spec": "wat",
						},
					},
				},
				cp: &composite.Unstructured{},
			},
			want: want{
				cp:  &composite.Unstructured{},
				err: errors.New(errUnsupportedClaimSpec),
			},
		},
		"AlreadyClaimedError": {
			reason: "We should return an error if we appear to be configuring a composite resource claimed by a different... claim.",
			args: args{
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"namespace": ns,
								"name":      name,
							},
							"spec": map[string]interface{}{},
						},
					},
				},
				cp: &composite.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"spec": map[string]interface{}{
								"claimRef": map[string]interface{}{
									"apiVersion": apiVersion,
									"kind":       kind,
									"namespace":  ns,
									"name":       "some-other-claim",
								},
							},
						},
					},
				},
			},
			want: want{
				cp: &composite.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"spec": map[string]interface{}{
								"claimRef": map[string]interface{}{
									"apiVersion": apiVersion,
									"kind":       kind,
									"namespace":  ns,
									"name":       "some-other-claim",
								},
							},
						},
					},
				},
				err: errors.New(errBindCompositeConflict),
			},
		},
		"DryRunError": {
			reason: "We should return any error we encounter while dry-run creating a dynamically provisioned composite",
			c: &test.MockClient{
				MockCreate: test.NewMockCreateFn(errBoom),
			},
			args: args{
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": apiVersion,
							"kind":       kind,
							"metadata": map[string]interface{}{
								"namespace": ns,
								"name":      name,
							},
							"spec": map[string]interface{}{
								"coolness": 23,

								// These should be preserved.
								"compositionRef":      "ref",
								"compositionSelector": "ref",

								// These should be filtered out.
								"resourceRef":                "ref",
								"writeConnectionSecretToRef": "ref",
							},
						},
					},
				},
				cp: &composite.Unstructured{},
			},
			want: want{
				cp: &composite.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"generateName": name + "-",
								"labels": map[string]interface{}{
									xcrd.LabelKeyClaimNamespace: ns,
									xcrd.LabelKeyClaimName:      name,
								},
							},
							"spec": map[string]interface{}{
								"coolness":            23,
								"compositionRef":      "ref",
								"compositionSelector": "ref",
								"claimRef": map[string]interface{}{
									"apiVersion": apiVersion,
									"kind":       kind,
									"namespace":  ns,
									"name":       name,
								},
							},
						},
					},
				},
				err: errors.Wrap(errBoom, errName),
			},
		},
		"ConfiguredNewXR": {
			reason: "A dynamically provisioned composite resource should be configured according to the claim",
			c: &test.MockClient{
				MockCreate: test.NewMockCreateFn(nil),
			},
			args: args{
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": apiVersion,
							"kind":       kind,
							"metadata": map[string]interface{}{
								"namespace": ns,
								"name":      name,
							},
							"spec": map[string]interface{}{
								"coolness": 23,

								// These should be preserved.
								"compositionRef":      "ref",
								"compositionSelector": "ref",

								// These should be filtered out.
								"resourceRef":                "ref",
								"writeConnectionSecretToRef": "ref",
							},
						},
					},
				},
				cp: &composite.Unstructured{},
			},
			want: want{
				cp: &composite.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"generateName": name + "-",
								"labels": map[string]interface{}{
									xcrd.LabelKeyClaimNamespace: ns,
									xcrd.LabelKeyClaimName:      name,
								},
							},
							"spec": map[string]interface{}{
								"coolness":            23,
								"compositionRef":      "ref",
								"compositionSelector": "ref",
								"claimRef": map[string]interface{}{
									"apiVersion": apiVersion,
									"kind":       kind,
									"namespace":  ns,
									"name":       name,
								},
							},
						},
					},
				},
			},
		},
		"ConfiguredExistingXR": {
			reason: "A statically provisioned composite resource should be configured according to the claim",
			args: args{
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": apiVersion,
							"kind":       kind,
							"metadata": map[string]interface{}{
								"namespace": ns,
								"name":      name,
								"annotations": map[string]interface{}{
									// This should be reset to the equivalent
									// composite resource value, since it has
									// most likely already taken effect and
									// cannot be updated retroactively.
									meta.AnnotationKeyExternalName: "wat",
									"xrc":                          "annotation",
								},
							},
							"spec": map[string]interface{}{
								"coolness": 23,

								// These should be filtered out.
								"resourceRef":                "ref",
								"writeConnectionSecretToRef": "ref",
							},
						},
					},
				},
				cp: &composite.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": name,
								"creationTimestamp": func() string {
									b, _ := now.MarshalJSON()
									return strings.Trim(string(b), "\"")
								}(),
								"labels": map[string]interface{}{
									xcrd.LabelKeyClaimNamespace: ns,
									xcrd.LabelKeyClaimName:      name,
								},
								"annotations": map[string]interface{}{
									meta.AnnotationKeyExternalName: name,
									"xr":                           "annotation",
								},
							},
							"spec": map[string]interface{}{
								// This should be overridden with the value of
								// the equivalent claim field.
								"coolness": 42,
							},
						},
					},
				},
			},
			want: want{
				cp: &composite.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": name,
								"creationTimestamp": func() string {
									b, _ := now.MarshalJSON()
									return strings.Trim(string(b), "\"")
								}(),
								"labels": map[string]interface{}{
									xcrd.LabelKeyClaimNamespace: ns,
									xcrd.LabelKeyClaimName:      name,
								},
								"annotations": map[string]interface{}{
									meta.AnnotationKeyExternalName: name,
									"xr":                           "annotation",
									"xrc":                          "annotation",
								},
							},
							"spec": map[string]interface{}{
								"coolness": 23,
								"claimRef": map[string]interface{}{
									"apiVersion": apiVersion,
									"kind":       kind,
									"namespace":  ns,
									"name":       name,
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewAPIDryRunCompositeConfigurator(tc.c)
			got := c.Configure(tc.args.ctx, tc.args.cm, tc.args.cp)
			if diff := cmp.Diff(tc.want.err, got, test.EquateErrors()); diff != "" {
				t.Errorf("Configure(...): %s\n-want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cp, tc.args.cp); diff != "" {
				t.Errorf("Configure(...): %s\n-want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}

}

func TestClaimConfigure(t *testing.T) {
	errBoom := errors.New("boom")
	ns := "spacename"
	name := "cool"

	type args struct {
		cm     resource.CompositeClaim
		cp     resource.Composite
		client client.Client
	}

	type want struct {
		cm  resource.CompositeClaim
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MergeStatusClaimError": {
			reason: "Should return an error if unable to merge status of claim",
			args: args{
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"status": "notStatus",
						},
					},
				},
				cp: &composite.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"status": "notStatus",
						},
					},
				},
			},
			want: want{
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"status": "notStatus",
						},
					},
				},
				err: errors.Wrap(errors.New(errUnsupportedDstObject), errMergeClaimStatus),
			},
		},
		"MergeStatusCompositeError": {
			reason: "Should return an error if unable to merge status from composite",
			args: args{
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"status": map[string]interface{}{},
						},
					},
				},
				cp: &composite.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"status": "notStatus",
						},
					},
				},
			},
			want: want{
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"status": map[string]interface{}{},
						},
					},
				},
				err: errors.Wrap(errors.New(errUnsupportedSrcObject), errMergeClaimStatus),
			},
		},
		"UpdateStatusError": {
			reason: "Should return an error if unable to update status",
			args: args{
				client: &test.MockClient{
					MockStatusUpdate: test.NewMockStatusUpdateFn(errBoom),
				},
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"spec":   map[string]interface{}{},
							"status": map[string]interface{}{},
						},
					},
				},
				cp: &composite.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"spec":   map[string]interface{}{},
							"status": map[string]interface{}{},
						},
					},
				},
			},
			want: want{
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"spec":   map[string]interface{}{},
							"status": map[string]interface{}{},
						},
					},
				},
				err: errors.Wrap(errBoom, errUpdateClaimStatus),
			},
		},
		"MergeSpecError": {
			reason: "Should return an error if unable to merge spec",
			args: args{
				client: &test.MockClient{
					MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
				},
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"spec":   "notSpec",
							"status": map[string]interface{}{},
						},
					},
				},
				cp: &composite.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"spec":   "notSpec",
							"status": map[string]interface{}{},
						},
					},
				},
			},
			want: want{
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"spec":   "notSpec",
							"status": map[string]interface{}{},
						},
					},
				},
				err: errors.Wrap(errors.New(errUnsupportedDstObject), errMergeClaimSpec),
			},
		},
		"UpdateClaimError": {
			reason: "Should return an error if unable to update claim",
			args: args{
				client: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(errBoom),
					MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
				},
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"spec":   map[string]interface{}{},
							"status": map[string]interface{}{},
						},
					},
				},
				cp: &composite.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"spec":   map[string]interface{}{},
							"status": map[string]interface{}{},
						},
					},
				},
			},
			want: want{
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"spec":   map[string]interface{}{},
							"status": map[string]interface{}{},
						},
					},
				},
				err: errors.Wrap(errBoom, errUpdateClaim),
			},
		},
		"LateInitializeClaim": {
			reason: "Empty fields in claim should be late initialized from the composite",
			args: args{
				client: test.NewMockClient(),
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"namespace": ns,
								"name":      name,
								"annotations": map[string]interface{}{
									meta.AnnotationKeyExternalName: "nope",
								},
							},
							"spec": map[string]interface{}{
								"someField":                  "someValue",
								"resourceRef":                "ref",
								"writeConnectionSecretToRef": "ref",
							},
							"status": map[string]interface{}{},
						},
					},
				},
				cp: &composite.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"namespace": ns,
								"name":      name + "-12345",
								"annotations": map[string]interface{}{
									meta.AnnotationKeyExternalName: name,
								},
							},
							"spec": map[string]interface{}{
								"coolness": 23,

								// These should be filtered out.
								"resourceRefs": "ref",
								"claimRef":     "ref",
							},
							"status": map[string]interface{}{},
						},
					},
				},
			},
			want: want{
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"namespace": ns,
								"name":      name,
								"annotations": map[string]interface{}{
									meta.AnnotationKeyExternalName: name,
								},
							},
							"spec": map[string]interface{}{
								"someField":                  "someValue",
								"coolness":                   23,
								"resourceRef":                "ref",
								"writeConnectionSecretToRef": "ref",
							},
							"status": map[string]interface{}{},
						},
					},
				},
			},
		},
		"ConfigureStatus": {
			reason: "Status of claim should be overwritten by the composite",
			args: args{
				client: test.NewMockClient(),
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"namespace": ns,
								"name":      name,
							},
							"spec": map[string]interface{}{
								"resourceRef":                "ref",
								"writeConnectionSecretToRef": "ref",
							},
							"status": map[string]interface{}{
								"previousCoolness": 23,
								"conditions": []map[string]interface{}{
									{
										"type": "someCondition",
									},
								},
							},
						},
					},
				},
				cp: &composite.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"namespace": ns,
								"name":      name + "-12345",
							},
							"spec": map[string]interface{}{
								"resourceRefs": "ref",
								"claimRef":     "ref",
							},
							"status": map[string]interface{}{
								"previousCoolness": 28,
								"conditions": []map[string]interface{}{
									{
										"type": "otherCondition",
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cm: &claim.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"namespace": ns,
								"name":      name,
							},
							"spec": map[string]interface{}{
								"resourceRef":                "ref",
								"writeConnectionSecretToRef": "ref",
							},
							"status": map[string]interface{}{
								"previousCoolness": 28,
								"conditions": []map[string]interface{}{
									{
										"type": "someCondition",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewAPIClaimConfigurator(tc.args.client)
			got := c.Configure(context.Background(), tc.args.cm, tc.args.cp)
			if diff := cmp.Diff(tc.want.err, got, test.EquateErrors()); diff != "" {
				t.Errorf("c.Configure(...): %s\n-want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cm, tc.args.cm); diff != "" {
				t.Errorf("c.Configure(...): %s\n-want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}

}
