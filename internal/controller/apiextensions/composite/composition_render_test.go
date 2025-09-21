/*
Copyright 2022 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package composite

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane/crossplane/v2/internal/xcrd"
)

func TestRenderFromJSON(t *testing.T) {
	errInvalidChar := json.Unmarshal([]byte("olala"), &fake.Composed{})

	type args struct {
		o    resource.Object
		data []byte
	}

	type want struct {
		o   resource.Object
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"InvalidData": {
			reason: "We should return an error if the data can't be unmarshalled",
			args: args{
				o:    &fake.Composed{},
				data: []byte("olala"),
			},
			want: want{
				o:   &fake.Composed{},
				err: errors.Wrap(errInvalidChar, errUnmarshalJSON),
			},
		},
		"ExistingKindChanged": {
			reason: "We should return an error if unmarshalling the base template changed the composed resource's kind.",
			args: args{
				o: composed.New(composed.FromReference(corev1.ObjectReference{
					APIVersion: "example.org/v1",
					Kind:       "Potato",
				})),
				data: []byte(`{"apiVersion": "example.org/v1", "kind": "Different"}`),
			},
			want: want{
				o: composed.New(composed.FromReference(corev1.ObjectReference{
					APIVersion: "example.org/v1",
					Kind:       "Different",
				})),
				err: errors.Errorf(errFmtKindChanged, "example.org/v1, Kind=Potato", "example.org/v1, Kind=Different"),
			},
		},
		"GroupCanChange": {
			reason: "We should accept group changes in the base template.",
			args: args{
				o: composed.New(composed.FromReference(corev1.ObjectReference{
					APIVersion: "example.org/v1",
					Kind:       "Potato",
				})),
				data: []byte(`{"apiVersion": "foo.io/v1", "kind": "Potato"}`),
			},
			want: want{
				o: composed.New(composed.FromReference(corev1.ObjectReference{
					APIVersion: "foo.io/v1",
					Kind:       "Potato",
				})),
			},
		},
		"VersionCanChange": {
			reason: "We should accept version changes in the base template.",
			args: args{
				o: composed.New(composed.FromReference(corev1.ObjectReference{
					APIVersion: "example.org/v1alpha1",
					Kind:       "Potato",
				})),
				data: []byte(`{"apiVersion": "example.org/v1beta1", "kind": "Potato"}`),
			},
			want: want{
				o: composed.New(composed.FromReference(corev1.ObjectReference{
					APIVersion: "example.org/v1beta1",
					Kind:       "Potato",
				})),
			},
		},
		"NewComposedResource": {
			reason: "A valid base template should apply successfully to a new (empty) composed resource",
			args: args{
				o:    composed.New(),
				data: []byte(`{"apiVersion": "example.org/v1", "kind": "Potato", "spec": {"cool": true}}`),
			},
			want: want{
				o: &composed.Unstructured{Unstructured: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "example.org/v1",
						"kind":       "Potato",
						"spec": map[string]any{
							"cool": true,
						},
					},
				}},
			},
		},
		"ExistingComposedResource": {
			reason: "A valid base template should apply successfully to a new (empty) composed resource",
			args: args{
				o: composed.New(composed.FromReference(corev1.ObjectReference{
					APIVersion: "example.org/v1",
					Kind:       "Potato",
					Name:       "ola-superrandom",
				})),
				data: []byte(`{"apiVersion": "example.org/v1", "kind": "Potato", "spec": {"cool": true}}`),
			},
			want: want{
				o: &composed.Unstructured{Unstructured: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "example.org/v1",
						"kind":       "Potato",
						"metadata": map[string]any{
							"name": "ola-superrandom",
						},
						"spec": map[string]any{
							"cool": true,
						},
					},
				}},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := RenderFromJSON(tc.args.o, tc.data)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRenderFromJSON(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.o, tc.args.o); diff != "" {
				t.Errorf("\n%s\nRenderFromJSON(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRenderComposedResourceMetadata(t *testing.T) {
	controlled := &fake.Composed{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{{
				Controller: ptr.To(true),
				UID:        "very-random",
			}},
		},
	}
	errRef := meta.AddControllerReference(controlled, metav1.OwnerReference{UID: "not-very-random"})

	type args struct {
		xr resource.Composite
		cd resource.Composed
		rn ResourceName
	}

	type want struct {
		cd  resource.Composed
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"ConflictingControllerReference": {
			reason: "We should return an error if the composed resource has an existing (and different) controller reference",
			args: args{
				xr: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						UID: "somewhat-random",
						Labels: map[string]string{
							xcrd.LabelKeyNamePrefixForComposed: "prefix",
							xcrd.LabelKeyClaimName:             "name",
							xcrd.LabelKeyClaimNamespace:        "namespace",
						},
					},
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{{
							Controller: ptr.To(true),
							UID:        "very-random",
						}},
					},
				},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "prefix-",
						OwnerReferences: []metav1.OwnerReference{{
							Controller: ptr.To(true),
							UID:        "very-random",
						}},
						Labels: map[string]string{
							xcrd.LabelKeyNamePrefixForComposed: "prefix",
							xcrd.LabelKeyClaimName:             "name",
							xcrd.LabelKeyClaimNamespace:        "namespace",
						},
					},
				},
				err: errors.Wrap(errRef, errSetControllerRef),
			},
		},
		"CompatibleControllerReference": {
			reason: "We should not return an error if the composed resource has an existing (and matching) controller reference",
			args: args{
				xr: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						UID: "somewhat-random",
						Labels: map[string]string{
							xcrd.LabelKeyNamePrefixForComposed: "prefix",
							xcrd.LabelKeyClaimName:             "name",
							xcrd.LabelKeyClaimNamespace:        "namespace",
						},
					},
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{{
							Controller: ptr.To(true),
							UID:        "somewhat-random",
						}},
					},
				},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "prefix-",
						OwnerReferences: []metav1.OwnerReference{{
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
							UID:                "somewhat-random",
						}},
						Labels: map[string]string{
							xcrd.LabelKeyNamePrefixForComposed: "prefix",
							xcrd.LabelKeyClaimName:             "name",
							xcrd.LabelKeyClaimNamespace:        "namespace",
						},
					},
				},
			},
		},
		"NoControllerReference": {
			reason: "We should not return an error if the composed resource has no controller reference",
			args: args{
				xr: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns",
						Name:      "cool-xr",
						UID:       "somewhat-random",
						Labels: map[string]string{
							xcrd.LabelKeyNamePrefixForComposed: "prefix",
							xcrd.LabelKeyClaimName:             "name",
							xcrd.LabelKeyClaimNamespace:        "namespace",
						},
					},
				},
				cd: &fake.Composed{},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:    "ns",
						GenerateName: "prefix-",
						OwnerReferences: []metav1.OwnerReference{{
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
							UID:                "somewhat-random",
							Name:               "cool-xr",
						}},
						Labels: map[string]string{
							xcrd.LabelKeyNamePrefixForComposed: "prefix",
							xcrd.LabelKeyClaimName:             "name",
							xcrd.LabelKeyClaimNamespace:        "namespace",
						},
					},
				},
			},
		},
		"NoClaimLabels": {
			reason: "MR should not have claim labels when XR doesn't have them",
			args: args{
				xr: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns",
						Name:      "cool-xr",
						UID:       "somewhat-random",
						Labels: map[string]string{
							xcrd.LabelKeyNamePrefixForComposed: "prefix",
						},
					},
				},
				cd: &fake.Composed{},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:    "ns",
						GenerateName: "prefix-",
						OwnerReferences: []metav1.OwnerReference{{
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
							UID:                "somewhat-random",
							Name:               "cool-xr",
						}},
						Labels: map[string]string{
							xcrd.LabelKeyNamePrefixForComposed: "prefix",
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := RenderComposedResourceMetadata(tc.args.cd, tc.xr, tc.rn)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRenderComposedResourceMetadata(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.cd, tc.args.cd); diff != "" {
				t.Errorf("\n%s\nRenderComposedResourceMetadata(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
