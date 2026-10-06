/*
Copyright 2026 The Crossplane Authors.

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

package composition

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
)

func TestGarbageCollect(t *testing.T) {
	errBoom := errors.New("boom")
	ctrl := true

	newComp := func(limit *int64) *v1.Composition {
		return &v1.Composition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cool-composition",
				UID:  types.UID("no-you-uid"),
			},
			Spec: v1.CompositionSpec{
				CompositeTypeRef: v1.TypeReference{
					APIVersion: "example.org/v1",
					Kind:       "XCool",
				},
				RevisionHistoryLimit: limit,
			},
		}
	}

	// newRev returns a revision named cool-composition-<n>, controlled by the
	// above composition.
	newRev := func(n int64, labels map[string]string) v1.CompositionRevision {
		return v1.CompositionRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("cool-composition-%d", n),
				Labels: labels,
				OwnerReferences: []metav1.OwnerReference{{
					UID:        "no-you-uid",
					Controller: &ctrl,
				}},
			},
			Spec: v1.CompositionRevisionSpec{Revision: n},
		}
	}

	// revs returns revisions 1 through n, oldest first.
	revs := func(n int64) []v1.CompositionRevision {
		out := make([]v1.CompositionRevision, 0, n)
		for i := int64(1); i <= n; i++ {
			out = append(out, newRev(i, nil))
		}

		return out
	}

	// newXR returns an XR with the supplied fields set under spec.
	newXR := func(spec map[string]any) kunstructured.Unstructured {
		return kunstructured.Unstructured{Object: map[string]any{
			"apiVersion": "example.org/v1",
			"kind":       "XCool",
			"spec":       spec,
		}}
	}

	// listXRs returns a mock List function that returns the supplied XRs.
	listXRs := func(xrs ...kunstructured.Unstructured) test.MockListFn {
		return func(_ context.Context, obj client.ObjectList, _ ...client.ListOption) error {
			l, ok := obj.(*kunstructured.UnstructuredList)
			if !ok {
				return errors.Errorf("unexpected list type %T", obj)
			}

			if got, want := l.GroupVersionKind(), (schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "XCoolList"}); got != want {
				return errors.Errorf("unexpected GVK %s, want %s", got, want)
			}

			l.Items = xrs

			return nil
		}
	}

	// listXRDs returns a mock List function that returns the supplied XRDs.
	listXRDs := func(xrds ...v1.CompositeResourceDefinition) test.MockListFn {
		return func(_ context.Context, obj client.ObjectList, _ ...client.ListOption) error {
			l, ok := obj.(*v1.CompositeResourceDefinitionList)
			if !ok {
				return errors.Errorf("unexpected list type %T", obj)
			}

			l.Items = xrds

			return nil
		}
	}

	// listShouldNotBeCalled fails the test if called.
	listShouldNotBeCalled := func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
		t.Errorf("List() should not be called")
		return nil
	}

	type params struct {
		client *test.MockClient
		xrs    *test.MockClient
	}

	type args struct {
		comp *v1.Composition
		revs []v1.CompositionRevision
	}

	type want struct {
		deleted []string
		n       int
		err     error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"FastPathDefaultLimit": {
			reason: "We shouldn't list XRs if there are no more than limit+1 revisions.",
			params: params{
				client: &test.MockClient{MockList: listShouldNotBeCalled},
				xrs:    &test.MockClient{MockList: listShouldNotBeCalled},
			},
			args: args{
				comp: newComp(nil),
				revs: revs(2),
			},
			want: want{},
		},
		"FastPathCustomLimit": {
			reason: "We shouldn't list XRs if there are no more than limit+1 revisions.",
			params: params{
				client: &test.MockClient{MockList: listShouldNotBeCalled},
				xrs:    &test.MockClient{MockList: listShouldNotBeCalled},
			},
			args: args{
				comp: newComp(new(int64(4))),
				revs: revs(5),
			},
			want: want{},
		},
		"FastPathIgnoresUncontrolledRevisions": {
			reason: "We should only count revisions controlled by the Composition.",
			params: params{
				client: &test.MockClient{MockList: listShouldNotBeCalled},
				xrs:    &test.MockClient{MockList: listShouldNotBeCalled},
			},
			args: args{
				comp: newComp(nil),
				revs: append(revs(2), v1.CompositionRevision{
					ObjectMeta: metav1.ObjectMeta{Name: "someone-elses"},
					Spec:       v1.CompositionRevisionSpec{Revision: 3},
				}),
			},
			want: want{},
		},
		"LimitZeroDisablesGC": {
			reason: "A revision history limit of 0 should disable garbage collection.",
			params: params{
				client: &test.MockClient{MockList: listShouldNotBeCalled},
				xrs:    &test.MockClient{MockList: listShouldNotBeCalled},
			},
			args: args{
				comp: newComp(new(int64(0))),
				revs: revs(10),
			},
			want: want{},
		},
		"DeleteBeyondDefaultLimit": {
			reason: "With the default limit we should keep the latest revision plus one more, and delete the rest oldest first.",
			params: params{
				client: &test.MockClient{
					MockList:   listXRDs(),
					MockDelete: test.NewMockDeleteFn(nil),
				},
				xrs: &test.MockClient{MockList: listXRs()},
			},
			args: args{
				comp: newComp(nil),
				revs: revs(5),
			},
			want: want{
				deleted: []string{"cool-composition-1", "cool-composition-2", "cool-composition-3"},
				n:       3,
			},
		},
		"DeleteBeyondCustomLimit": {
			reason: "We should keep the latest revision plus limit more, regardless of the order revisions are supplied in.",
			params: params{
				client: &test.MockClient{
					MockList:   listXRDs(),
					MockDelete: test.NewMockDeleteFn(nil),
				},
				xrs: &test.MockClient{MockList: listXRs()},
			},
			args: args{
				comp: newComp(new(int64(2))),
				revs: []v1.CompositionRevision{newRev(3, nil), newRev(5, nil), newRev(1, nil), newRev(4, nil), newRev(2, nil)},
			},
			want: want{
				deleted: []string{"cool-composition-1", "cool-composition-2"},
				n:       2,
			},
		},
		"KeepOldInUseRevision": {
			reason: "We should keep an old revision that's in use, but still delete newer unused revisions beyond the limit.",
			params: params{
				client: &test.MockClient{
					MockList:   listXRDs(),
					MockDelete: test.NewMockDeleteFn(nil),
				},
				xrs: &test.MockClient{MockList: listXRs(
					newXR(map[string]any{"crossplane": map[string]any{
						"compositionUpdatePolicy": "Manual",
						"compositionRevisionRef":  map[string]any{"name": "cool-composition-1"},
					}}),
				)},
			},
			args: args{
				comp: newComp(nil),
				revs: revs(5),
			},
			want: want{
				deleted: []string{"cool-composition-2", "cool-composition-3", "cool-composition-4"},
				n:       3,
			},
		},
		"KeepRevisionsReferencedByModernAndLegacyXRs": {
			reason: "We should find revision references in both the v2 and legacy XR schemas.",
			params: params{
				client: &test.MockClient{
					MockList:   listXRDs(),
					MockDelete: test.NewMockDeleteFn(nil),
				},
				xrs: &test.MockClient{MockList: listXRs(
					newXR(map[string]any{"crossplane": map[string]any{
						"compositionRevisionRef": map[string]any{"name": "cool-composition-1"},
					}}),
					newXR(map[string]any{
						"compositionRevisionRef": map[string]any{"name": "cool-composition-2"},
					}),
				)},
			},
			args: args{
				comp: newComp(nil),
				revs: revs(5),
			},
			want: want{
				deleted: []string{"cool-composition-3", "cool-composition-4"},
				n:       2,
			},
		},
		"KeepNewestRevisionMatchingXRSelector": {
			reason: "We should keep the newest revision that matches an XR's revision selector, but not older matching revisions.",
			params: params{
				client: &test.MockClient{
					MockList:   listXRDs(),
					MockDelete: test.NewMockDeleteFn(nil),
				},
				xrs: &test.MockClient{MockList: listXRs(
					newXR(map[string]any{"crossplane": map[string]any{
						"compositionRef":              map[string]any{"name": "cool-composition"},
						"compositionRevisionSelector": map[string]any{"matchLabels": map[string]any{"channel": "stable"}},
					}}),
				)},
			},
			args: args{
				comp: newComp(nil),
				revs: []v1.CompositionRevision{
					newRev(1, map[string]string{"channel": "stable"}),
					newRev(2, map[string]string{"channel": "stable"}),
					newRev(3, map[string]string{"channel": "dev"}),
					newRev(4, map[string]string{"channel": "dev"}),
					newRev(5, map[string]string{"channel": "dev"}),
				},
			},
			want: want{
				deleted: []string{"cool-composition-1", "cool-composition-3", "cool-composition-4"},
				n:       3,
			},
		},
		"KeepNewestRevisionMatchingLegacyXRSelector": {
			reason: "We should find revision selectors in the legacy XR schema.",
			params: params{
				client: &test.MockClient{
					MockList:   listXRDs(),
					MockDelete: test.NewMockDeleteFn(nil),
				},
				xrs: &test.MockClient{MockList: listXRs(
					newXR(map[string]any{
						"compositionRevisionSelector": map[string]any{"matchLabels": map[string]any{"channel": "stable"}},
					}),
				)},
			},
			args: args{
				comp: newComp(nil),
				revs: []v1.CompositionRevision{
					newRev(1, map[string]string{"channel": "stable"}),
					newRev(2, map[string]string{"channel": "dev"}),
					newRev(3, map[string]string{"channel": "dev"}),
					newRev(4, map[string]string{"channel": "dev"}),
				},
			},
			want: want{
				deleted: []string{"cool-composition-2", "cool-composition-3"},
				n:       2,
			},
		},
		"IgnoreSelectorOfXRUsingAnotherComposition": {
			reason: "We should ignore the revision selector of an XR that uses a different Composition.",
			params: params{
				client: &test.MockClient{
					MockList:   listXRDs(),
					MockDelete: test.NewMockDeleteFn(nil),
				},
				xrs: &test.MockClient{MockList: listXRs(
					newXR(map[string]any{"crossplane": map[string]any{
						"compositionRef":              map[string]any{"name": "other-composition"},
						"compositionRevisionSelector": map[string]any{"matchLabels": map[string]any{"channel": "stable"}},
					}}),
				)},
			},
			args: args{
				comp: newComp(nil),
				revs: []v1.CompositionRevision{
					newRev(1, map[string]string{"channel": "stable"}),
					newRev(2, map[string]string{"channel": "dev"}),
					newRev(3, map[string]string{"channel": "dev"}),
				},
			},
			want: want{
				deleted: []string{"cool-composition-1"},
				n:       1,
			},
		},
		"KeepNewestRevisionMatchingXRDDefaultSelector": {
			reason: "We should keep the newest revision that matches the XRD's default revision selector.",
			params: params{
				client: &test.MockClient{
					MockList: listXRDs(
						// A different XRD with the same kind in another group.
						v1.CompositeResourceDefinition{Spec: v1.CompositeResourceDefinitionSpec{
							Group:                              "other.org",
							Names:                              extv1.CustomResourceDefinitionNames{Kind: "XCool"},
							DefaultCompositionRevisionSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"channel": "dev"}},
						}},
						v1.CompositeResourceDefinition{Spec: v1.CompositeResourceDefinitionSpec{
							Group:                              "example.org",
							Names:                              extv1.CustomResourceDefinitionNames{Kind: "XCool"},
							DefaultCompositionRevisionSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"channel": "stable"}},
						}},
					),
					MockDelete: test.NewMockDeleteFn(nil),
				},
				xrs: &test.MockClient{MockList: listXRs()},
			},
			args: args{
				comp: newComp(nil),
				revs: []v1.CompositionRevision{
					newRev(1, map[string]string{"channel": "stable"}),
					newRev(2, map[string]string{"channel": "dev"}),
					newRev(3, map[string]string{"channel": "dev"}),
					newRev(4, map[string]string{"channel": "dev"}),
				},
			},
			want: want{
				deleted: []string{"cool-composition-2", "cool-composition-3"},
				n:       2,
			},
		},
		"NeverDeleteLatestRevision": {
			reason: "We should never delete the latest revision, even when it's unused and everything else is in use.",
			params: params{
				client: &test.MockClient{
					MockList: listXRDs(),
					MockDelete: func(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
						if obj.GetName() == "cool-composition-4" {
							t.Errorf("Delete(): deleted the latest revision")
						}
						return nil
					},
				},
				xrs: &test.MockClient{MockList: listXRs(
					newXR(map[string]any{"crossplane": map[string]any{"compositionRevisionRef": map[string]any{"name": "cool-composition-1"}}}),
					newXR(map[string]any{"crossplane": map[string]any{"compositionRevisionRef": map[string]any{"name": "cool-composition-2"}}}),
					newXR(map[string]any{"crossplane": map[string]any{"compositionRevisionRef": map[string]any{"name": "cool-composition-3"}}}),
				)},
			},
			args: args{
				comp: newComp(nil),
				revs: revs(4),
			},
			want: want{},
		},
		"ListXRsError": {
			reason: "We should delete nothing and return an error if we can't list XRs.",
			params: params{
				client: &test.MockClient{
					MockList:   listXRDs(),
					MockDelete: test.NewMockDeleteFn(errors.New("should not be called")),
				},
				xrs: &test.MockClient{MockList: test.NewMockListFn(errBoom)},
			},
			args: args{
				comp: newComp(nil),
				revs: revs(5),
			},
			want: want{
				err: errBoom,
			},
		},
		"ListXRDsError": {
			reason: "We should delete nothing and return an error if we can't list XRDs.",
			params: params{
				client: &test.MockClient{
					MockList:   test.NewMockListFn(errBoom),
					MockDelete: test.NewMockDeleteFn(errors.New("should not be called")),
				},
				xrs: &test.MockClient{MockList: listXRs()},
			},
			args: args{
				comp: newComp(nil),
				revs: revs(5),
			},
			want: want{
				err: errBoom,
			},
		},
		"IgnoreNotFoundOnDelete": {
			reason: "We should ignore revisions that are already gone.",
			params: params{
				client: &test.MockClient{
					MockList:   listXRDs(),
					MockDelete: test.NewMockDeleteFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				},
				xrs: &test.MockClient{MockList: listXRs()},
			},
			args: args{
				comp: newComp(nil),
				revs: revs(4),
			},
			want: want{
				deleted: []string{"cool-composition-1", "cool-composition-2"},
				n:       2,
			},
		},
		"DeleteError": {
			reason: "We should return the number of revisions deleted before an error.",
			params: params{
				client: &test.MockClient{
					MockList: listXRDs(),
					MockDelete: func(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
						if obj.GetName() == "cool-composition-2" {
							return errBoom
						}
						return nil
					},
				},
				xrs: &test.MockClient{MockList: listXRs()},
			},
			args: args{
				comp: newComp(nil),
				revs: revs(4),
			},
			want: want{
				deleted: []string{"cool-composition-1", "cool-composition-2"},
				n:       1,
				err:     errBoom,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Record every revision we try to delete.
			var deleted []string

			if del := tc.params.client.MockDelete; del != nil {
				tc.params.client.MockDelete = func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
					deleted = append(deleted, obj.GetName())
					return del(ctx, obj, opts...)
				}
			}

			gc := NewAPIRevisionGarbageCollector(tc.params.client, tc.params.xrs)

			n, err := gc.GarbageCollect(t.Context(), tc.args.comp, tc.args.revs)
			if !errors.Is(err, tc.want.err) {
				diff := cmp.Diff(tc.want.err, err, test.EquateErrors())
				t.Errorf("\n%s\ngc.GarbageCollect(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.n, n); diff != "" {
				t.Errorf("\n%s\ngc.GarbageCollect(...): -want count, +got count:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.deleted, deleted, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\ngc.GarbageCollect(...): -want deleted, +got deleted:\n%s", tc.reason, diff)
			}
		})
	}
}
