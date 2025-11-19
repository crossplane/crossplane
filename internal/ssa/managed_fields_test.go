/*
Copyright 2025 The Crossplane Authors.

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

package ssa

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

func TestExactMatch(t *testing.T) {
	type args struct {
		name    string
		manager string
	}
	type want struct {
		matches bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Matches": {
			reason: "Should match exact manager name",
			args: args{
				name:    "apiextensions.crossplane.io/claim",
				manager: "apiextensions.crossplane.io/claim",
			},
			want: want{
				matches: true,
			},
		},
		"DoesNotMatch": {
			reason: "Should not match different manager name",
			args: args{
				name:    "apiextensions.crossplane.io/claim",
				manager: "apiextensions.crossplane.io/composite",
			},
			want: want{
				matches: false,
			},
		},
		"DoesNotMatchPrefix": {
			reason: "Should not match if manager is only a prefix",
			args: args{
				name:    "apiextensions.crossplane.io/composed",
				manager: "apiextensions.crossplane.io/composed/abc123",
			},
			want: want{
				matches: false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			matcher := ExactMatch(tc.args.name)
			got := matcher(tc.args.manager)

			if diff := cmp.Diff(tc.want.matches, got); diff != "" {
				t.Errorf("\n%s\nExactMatch(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPrefixMatch(t *testing.T) {
	type args struct {
		prefix  string
		manager string
	}
	type want struct {
		matches bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MatchesExact": {
			reason: "Should match exact manager name",
			args: args{
				prefix:  "apiextensions.crossplane.io/composed",
				manager: "apiextensions.crossplane.io/composed",
			},
			want: want{
				matches: true,
			},
		},
		"MatchesWithSuffix": {
			reason: "Should match manager with suffix",
			args: args{
				prefix:  "apiextensions.crossplane.io/composed",
				manager: "apiextensions.crossplane.io/composed/abc123",
			},
			want: want{
				matches: true,
			},
		},
		"DoesNotMatch": {
			reason: "Should not match different prefix",
			args: args{
				prefix:  "apiextensions.crossplane.io/composed",
				manager: "apiextensions.crossplane.io/claim",
			},
			want: want{
				matches: false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			matcher := PrefixMatch(tc.args.prefix)
			got := matcher(tc.args.manager)

			if diff := cmp.Diff(tc.want.matches, got); diff != "" {
				t.Errorf("\n%s\nPrefixMatch(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPatchingManagedFieldsUpgrader(t *testing.T) {
	errBoom := errors.New("boom")
	fieldManagerSSA := "apiextensions.crossplane.io/test"
	fieldManagerOld := "crossplane"

	type fields struct {
		client  client.Writer
		matcher FieldManagerMatcher
	}
	type args struct {
		ctx context.Context
		obj client.Object
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"ObjectNotCreated": {
			reason: "Should return nil if object doesn't exist (no CreationTimestamp)",
			fields: fields{
				client:  &test.MockClient{},
				matcher: ExactMatch(fieldManagerSSA),
			},
			args: args{
				ctx: context.Background(),
				obj: &corev1.ConfigMap{
					// No CreationTimestamp = not created
				},
			},
			want: want{
				err: nil,
			},
		},
		"AlreadyUpgraded": {
			reason: "Should return nil if SSA manager exists and no before-first-apply",
			fields: fields{
				client:  &test.MockClient{},
				matcher: ExactMatch(fieldManagerSSA),
			},
			args: args{
				ctx: context.Background(),
				obj: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						UID:               "test-uid",
						CreationTimestamp: metav1.Now(),
						ResourceVersion:   "1",
						ManagedFields: []metav1.ManagedFieldsEntry{
							{
								Manager:    fieldManagerSSA,
								Operation:  metav1.ManagedFieldsOperationApply,
								APIVersion: "v1",
							},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"FirstUpgradeStep": {
			reason: "Should clear all managed fields on first upgrade",
			fields: fields{
				client: &test.MockClient{
					MockPatch: func(_ context.Context, obj client.Object, patch client.Patch, _ ...client.PatchOption) error {
						// Validate it's a JSON patch
						data, err := patch.Data(obj)
						if err != nil {
							return err
						}

						// Should clear managed fields with resourceVersion
						wantPatch := `[
			{"op":"replace","path": "/metadata/managedFields","value": [{}]},
			{"op":"replace","path":"/metadata/resourceVersion","value":"1"}
		]`

						if diff := cmp.Diff(wantPatch, string(data)); diff != "" {
							return errors.Errorf("unexpected patch: -want, +got:\n%s", diff)
						}
						return nil
					},
				},
				matcher: ExactMatch(fieldManagerSSA),
			},
			args: args{
				ctx: context.Background(),
				obj: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						UID:               "test-uid",
						CreationTimestamp: metav1.Now(),
						ResourceVersion:   "1",
						ManagedFields: []metav1.ManagedFieldsEntry{
							{
								Manager:    fieldManagerOld,
								Operation:  metav1.ManagedFieldsOperationUpdate,
								APIVersion: "v1",
							},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"FirstUpgradeStepError": {
			reason: "Should return error if patch fails during first upgrade",
			fields: fields{
				client: &test.MockClient{
					MockPatch: func(_ context.Context, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return errBoom
					},
				},
				matcher: ExactMatch(fieldManagerSSA),
			},
			args: args{
				ctx: context.Background(),
				obj: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						UID:               "test-uid",
						CreationTimestamp: metav1.Now(),
						ResourceVersion:   "1",
						ManagedFields: []metav1.ManagedFieldsEntry{
							{
								Manager:    fieldManagerOld,
								Operation:  metav1.ManagedFieldsOperationUpdate,
								APIVersion: "v1",
							},
						},
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"SecondUpgradeStep": {
			reason: "Should remove before-first-apply manager",
			fields: fields{
				client: &test.MockClient{
					MockPatch: func(_ context.Context, obj client.Object, patch client.Patch, _ ...client.PatchOption) error {
						// Validate it's a JSON patch
						data, err := patch.Data(obj)
						if err != nil {
							return err
						}

						// Should remove before-first-apply at index 1 with resourceVersion
						// Note: before-first-apply is at index 1 in the managedFields array
						wantPatch := `[
			{"op":"remove","path":"/metadata/managedFields/1"},
			{"op":"replace","path":"/metadata/resourceVersion","value":"2"}
		]`

						if diff := cmp.Diff(wantPatch, string(data)); diff != "" {
							return errors.Errorf("unexpected patch: -want, +got:\n%s", diff)
						}
						return nil
					},
				},
				matcher: ExactMatch(fieldManagerSSA),
			},
			args: args{
				ctx: context.Background(),
				obj: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						UID:               "test-uid",
						CreationTimestamp: metav1.Now(),
						ResourceVersion:   "2",
						ManagedFields: []metav1.ManagedFieldsEntry{
							{
								Manager:    fieldManagerSSA,
								Operation:  metav1.ManagedFieldsOperationApply,
								APIVersion: "v1",
							},
							{
								Manager:    "before-first-apply",
								Operation:  metav1.ManagedFieldsOperationUpdate,
								APIVersion: "v1",
							},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"SecondUpgradeStepError": {
			reason: "Should return error if patch fails during second upgrade",
			fields: fields{
				client: &test.MockClient{
					MockPatch: func(_ context.Context, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return errBoom
					},
				},
				matcher: ExactMatch(fieldManagerSSA),
			},
			args: args{
				ctx: context.Background(),
				obj: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						UID:               "test-uid",
						CreationTimestamp: metav1.Now(),
						ResourceVersion:   "2",
						ManagedFields: []metav1.ManagedFieldsEntry{
							{
								Manager:    fieldManagerSSA,
								Operation:  metav1.ManagedFieldsOperationApply,
								APIVersion: "v1",
							},
							{
								Manager:    "before-first-apply",
								Operation:  metav1.ManagedFieldsOperationUpdate,
								APIVersion: "v1",
							},
						},
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"PrefixMatcher": {
			reason: "Should work with prefix matcher for dynamic field managers",
			fields: fields{
				client:  &test.MockClient{},
				matcher: PrefixMatch("apiextensions.crossplane.io/composed"),
			},
			args: args{
				ctx: context.Background(),
				obj: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						UID:               "test-uid",
						CreationTimestamp: metav1.Now(),
						ResourceVersion:   "1",
						ManagedFields: []metav1.ManagedFieldsEntry{
							{
								Manager:    "apiextensions.crossplane.io/composed/abc123",
								Operation:  metav1.ManagedFieldsOperationApply,
								APIVersion: "v1",
							},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"MultipleOldManagers": {
			reason: "Should clear all old managers on first upgrade",
			fields: fields{
				client: &test.MockClient{
					MockPatch: func(_ context.Context, obj client.Object, patch client.Patch, _ ...client.PatchOption) error {
						// Validate it's a JSON patch
						data, err := patch.Data(obj)
						if err != nil {
							return err
						}

						// Should clear all managed fields regardless of how many there are
						wantPatch := `[
			{"op":"replace","path": "/metadata/managedFields","value": [{}]},
			{"op":"replace","path":"/metadata/resourceVersion","value":"1"}
		]`

						if diff := cmp.Diff(wantPatch, string(data)); diff != "" {
							return errors.Errorf("unexpected patch: -want, +got:\n%s", diff)
						}
						return nil
					},
				},
				matcher: ExactMatch(fieldManagerSSA),
			},
			args: args{
				ctx: context.Background(),
				obj: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						UID:               "test-uid",
						CreationTimestamp: metav1.Now(),
						ResourceVersion:   "1",
						ManagedFields: []metav1.ManagedFieldsEntry{
							{
								Manager:    "crossplane",
								Operation:  metav1.ManagedFieldsOperationUpdate,
								APIVersion: "v1",
							},
							{
								Manager:    "kubectl",
								Operation:  metav1.ManagedFieldsOperationUpdate,
								APIVersion: "v1",
							},
							{
								Manager:    "some-other-controller",
								Operation:  metav1.ManagedFieldsOperationUpdate,
								APIVersion: "v1",
							},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			u := NewPatchingManagedFieldsUpgrader(tc.fields.client, tc.fields.matcher)
			err := u.Upgrade(tc.args.ctx, tc.args.obj)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUpgrade(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
