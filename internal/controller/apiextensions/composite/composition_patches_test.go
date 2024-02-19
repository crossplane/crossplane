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

package composite

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestPatchApply(t *testing.T) {
	now := metav1.NewTime(time.Unix(0, 0))
	lpt := fake.ConnectionDetailsLastPublishedTimer{
		Time: &now,
	}

	errNotFound := func(path string) error {
		p := &fieldpath.Paved{}
		_, err := p.GetValue(path)
		return err
	}

	type args struct {
		patch v1.Patch
		cp    *fake.Composite
		cd    *fake.Composed
		only  []v1.PatchType
	}
	type want struct {
		cp  *fake.Composite
		cd  *fake.Composed
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"InvalidCompositeFieldPathPatch": {
			reason: "Should return error when required fields not passed to applyFromFieldPathPatch",
			args: args{
				patch: v1.Patch{
					Type: v1.PatchTypeFromCompositeFieldPath,
				},
				cp: &fake.Composite{
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "cd"}},
			},
			want: want{
				err: errors.Errorf(errFmtRequiredField, "FromFieldPath", v1.PatchTypeFromCompositeFieldPath),
			},
		},
		"Invalidv1.PatchType": {
			reason: "Should return an error if an invalid patch type is specified",
			args: args{
				patch: v1.Patch{
					Type: "invalid-patchtype",
				},
				cp: &fake.Composite{
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "cd"}},
			},
			want: want{
				err: errors.Errorf(errFmtInvalidPatchType, "invalid-patchtype"),
			},
		},
		"ValidCompositeFieldPathPatch": {
			reason: "Should correctly apply a CompositeFieldPathPatch with valid settings",
			args: args{
				patch: v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: ptr.To("objectMeta.labels"),
					ToFieldPath:   ptr.To("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{Name: "cd"},
				},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				err: nil,
			},
		},
		"ValidCompositeFieldPathPatchWithNilLastPublishTime": {
			reason: "Should correctly apply a CompositeFieldPathPatch with valid settings",
			args: args{
				patch: v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: ptr.To("objectMeta.labels"),
					ToFieldPath:   ptr.To("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{Name: "cd"},
				},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				err: nil,
			},
		},
		"ValidCompositeFieldPathPatchWithWildcards": {
			reason: "When passed a wildcarded path, adds a field to each element of an array",
			args: args{
				patch: v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: ptr.To("objectMeta.name"),
					ToFieldPath:   ptr.To("objectMeta.ownerReferences[*].name"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "",
								APIVersion: "v1",
							},
							{
								Name:       "",
								APIVersion: "v1alpha1",
							},
						},
					},
				},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test",
								APIVersion: "v1",
							},
							{
								Name:       "test",
								APIVersion: "v1alpha1",
							},
						},
					},
				},
			},
		},
		"InvalidCompositeFieldPathPatchWithWildcards": {
			reason: "When passed a wildcarded path, throws an error if ToFieldPath cannot be expanded",
			args: args{
				patch: v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: ptr.To("objectMeta.name"),
					ToFieldPath:   ptr.To("objectMeta.ownerReferences[*].badField"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test",
								APIVersion: "v1",
							},
						},
					},
				},
			},
			want: want{
				err: errors.Errorf(errFmtExpandingArrayFieldPaths, "objectMeta.ownerReferences[*].badField"),
			},
		},
		"MissingOptionalFieldPath": {
			reason: "A FromFieldPath patch should be a no-op when an optional fromFieldPath doesn't exist",
			args: args{
				patch: v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: ptr.To("objectMeta.labels"),
					ToFieldPath:   ptr.To("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{Name: "cd"},
				},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
					},
				},
				err: nil,
			},
		},
		"MissingRequiredFieldPath": {
			reason: "A FromFieldPath patch should return an error when a required fromFieldPath doesn't exist",
			args: args{
				patch: v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: ptr.To("wat"),
					Policy: &v1.PatchPolicy{
						FromFieldPath: func() *v1.FromFieldPathPolicy {
							s := v1.FromFieldPathPolicyRequired
							return &s
						}(),
					},
					ToFieldPath: ptr.To("wat"),
				},
				cp: &fake.Composite{
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{Name: "cd"},
				},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
					},
				},
				err: errNotFound("wat"),
			},
		},
		"ValidFromEnvironmentFieldPathPatch": {
			reason: "Should correctly apply a FromEnvironmentFieldPathPatch with valid settings",
			args: args{
				patch: v1.Patch{
					Type:          v1.PatchTypeFromEnvironmentFieldPath,
					FromFieldPath: ptr.To("objectMeta.labels"),
					ToFieldPath:   ptr.To("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{Name: "cd"},
				},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				err: nil,
			},
		},
		"MissingFromEnvironmentRequiredFieldPath": {
			reason: "A FromFieldPath patch should return an error when a required fromFieldPath doesn't exist",
			args: args{
				patch: v1.Patch{
					Type:          v1.PatchTypeFromEnvironmentFieldPath,
					FromFieldPath: ptr.To("wat"),
					Policy: &v1.PatchPolicy{
						FromFieldPath: func() *v1.FromFieldPathPolicy {
							s := v1.FromFieldPathPolicyRequired
							return &s
						}(),
					},
					ToFieldPath: ptr.To("wat"),
				},
				cp: &fake.Composite{
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{Name: "cd"},
				},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
					},
				},
				err: errNotFound("wat"),
			},
		},
		"MergeOptionsKeepMapValues": {
			reason: "Setting mergeOptions.keepMapValues = true adds new map values to existing ones",
			args: args{
				patch: v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: ptr.To("objectMeta.labels"),
					Policy: &v1.PatchPolicy{
						MergeOptions: &xpv1.MergeOptions{
							KeepMapValues: ptr.To(true),
						},
					},
					ToFieldPath: ptr.To("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"labelone": "foo",
							"labeltwo": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"labelthree": "baz",
						},
					},
				},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"labelone":   "foo",
							"labeltwo":   "bar",
							"labelthree": "baz",
						},
					},
				},
				err: nil,
			},
		},
		"FilterExcludeCompositeFieldPathPatch": {
			reason: "Should not apply the patch as the v1.PatchType is not present in filter.",
			args: args{
				patch: v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: ptr.To("objectMeta.labels"),
					ToFieldPath:   ptr.To("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{Name: "cd"},
				},
				only: []v1.PatchType{v1.PatchTypePatchSet},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
					},
				},
				err: nil,
			},
		},
		"FilterIncludeCompositeFieldPathPatch": {
			reason: "Should apply the patch as the v1.PatchType is present in filter.",
			args: args{
				patch: v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: ptr.To("objectMeta.labels"),
					ToFieldPath:   ptr.To("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{Name: "cd"},
				},
				only: []v1.PatchType{v1.PatchTypeFromCompositeFieldPath},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				err: nil,
			},
		},
		"DefaultToFieldCompositeFieldPathPatch": {
			reason: "Should correctly default the ToFieldPath value if not specified.",
			args: args{
				patch: v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: ptr.To("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				err: nil,
			},
		},
		"ValidToCompositeFieldPathPatch": {
			reason: "Should correctly apply a ToCompositeFieldPath patch with valid settings",
			args: args{
				patch: v1.Patch{
					Type:          v1.PatchTypeToCompositeFieldPath,
					FromFieldPath: ptr.To("objectMeta.labels"),
					ToFieldPath:   ptr.To("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				err: nil,
			},
		},
		"ValidToCompositeFieldPathPatchWithWildcards": {
			reason: "When passed a wildcarded path, adds a field to each element of an array",
			args: args{
				patch: v1.Patch{
					Type:          v1.PatchTypeToCompositeFieldPath,
					FromFieldPath: ptr.To("objectMeta.name"),
					ToFieldPath:   ptr.To("objectMeta.ownerReferences[*].name"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "",
								APIVersion: "v1",
							},
							{
								Name:       "",
								APIVersion: "v1alpha1",
							},
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test",
								APIVersion: "v1",
							},
							{
								Name:       "test",
								APIVersion: "v1alpha1",
							},
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
			},
		},
		"ValidToEnvironmentFieldPathPatch": {
			reason: "Should correctly apply a ToEnvironmentFieldPath patch with valid settings",
			args: args{
				patch: v1.Patch{
					Type:          v1.PatchTypeToEnvironmentFieldPath,
					FromFieldPath: ptr.To("objectMeta.labels"),
					ToFieldPath:   ptr.To("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				err: nil,
			},
		},
		"MissingCombineFromCompositeConfig": {
			reason: "Should return an error if Combine config is not passed",
			args: args{
				patch: v1.Patch{
					Type:        v1.PatchTypeCombineFromComposite,
					ToFieldPath: ptr.To("objectMeta.labels.destination"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				err: errors.Errorf(errFmtRequiredField, "Combine", v1.PatchTypeCombineFromComposite),
			},
		},
		"MissingCombineFromEnvironmentConfig": {
			reason: "Should return an error if Combine config is not passed",
			args: args{
				patch: v1.Patch{
					Type:        v1.PatchTypeCombineFromEnvironment,
					ToFieldPath: ptr.To("objectMeta.labels.destination"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				err: errors.Errorf(errFmtRequiredField, "Combine", v1.PatchTypeCombineFromEnvironment),
			},
		},
		"MissingCombineStrategyFromCompositeConfig": {
			reason: "Should return an error if Combine strategy config is not passed",
			args: args{
				patch: v1.Patch{
					Type: v1.PatchTypeCombineFromComposite,
					Combine: &v1.Combine{
						Variables: []v1.CombineVariable{
							{FromFieldPath: "objectMeta.labels.source1"},
							{FromFieldPath: "objectMeta.labels.source2"},
						},
						Strategy: v1.CombineStrategyString,
					},
					ToFieldPath: ptr.To("objectMeta.labels.destination"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				err: errors.Errorf(errFmtCombineConfigMissing, v1.CombineStrategyString),
			},
		},
		"MissingCombineVariablesFromCompositeConfig": {
			reason: "Should return an error if no variables have been passed",
			args: args{
				patch: v1.Patch{
					Type: v1.PatchTypeCombineFromComposite,
					Combine: &v1.Combine{
						Variables: []v1.CombineVariable{},
						Strategy:  v1.CombineStrategyString,
						String:    &v1.StringCombine{Format: "%s-%s"},
					},
					ToFieldPath: ptr.To("objectMeta.labels.destination"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				err: errors.New(errCombineRequiresVariables),
			},
		},
		"NoOpOptionalInputFieldFromCompositeConfig": {
			// Note: OptionalFieldPathNotFound is tested below, but we want to
			// test that we abort the patch if _any_ of our source fields are
			// not available.
			reason: "Should return no error and not apply patch if an optional variable is missing",
			args: args{
				patch: v1.Patch{
					Type: v1.PatchTypeCombineFromComposite,
					Combine: &v1.Combine{
						Variables: []v1.CombineVariable{
							{FromFieldPath: "objectMeta.labels.source1"},
							{FromFieldPath: "objectMeta.labels.source2"},
							{FromFieldPath: "objectMeta.labels.source3"},
						},
						Strategy: v1.CombineStrategyString,
						String:   &v1.StringCombine{Format: "%s-%s"},
					},
					ToFieldPath: ptr.To("objectMeta.labels.destination"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source3": "baz",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source3": "baz",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				err: nil,
			},
		},
		"ValidCombineFromComposite": {
			reason: "Should correctly apply a CombineFromComposite patch with valid settings",
			args: args{
				patch: v1.Patch{
					Type: v1.PatchTypeCombineFromComposite,
					Combine: &v1.Combine{
						Variables: []v1.CombineVariable{
							{FromFieldPath: "objectMeta.labels.source1"},
							{FromFieldPath: "objectMeta.labels.source2"},
						},
						Strategy: v1.CombineStrategyString,
						String:   &v1.StringCombine{Format: "%s-%s"},
					},
					ToFieldPath: ptr.To("objectMeta.labels.destination"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test":        "blah",
							"destination": "foo-bar",
						},
					},
				},
				err: nil,
			},
		},
		"ValidCombineToComposite": {
			reason: "Should correctly apply a CombineToComposite patch with valid settings",
			args: args{
				patch: v1.Patch{
					Type: v1.PatchTypeCombineToComposite,
					Combine: &v1.Combine{
						Variables: []v1.CombineVariable{
							{FromFieldPath: "objectMeta.labels.source1"},
							{FromFieldPath: "objectMeta.labels.source2"},
						},
						Strategy: v1.CombineStrategyString,
						String:   &v1.StringCombine{Format: "%s-%s"},
					},
					ToFieldPath: ptr.To("objectMeta.labels.destination"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test":        "blah",
							"destination": "foo-bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
				},
				err: nil,
			},
		},
		"ValidCombineToEnvironment": {
			reason: "Should correctly apply a CombineToEnvironment patch with valid settings",
			args: args{
				patch: v1.Patch{
					Type: v1.PatchTypeCombineToEnvironment,
					Combine: &v1.Combine{
						Variables: []v1.CombineVariable{
							{FromFieldPath: "objectMeta.labels.source1"},
							{FromFieldPath: "objectMeta.labels.source2"},
						},
						Strategy: v1.CombineStrategyString,
						String:   &v1.StringCombine{Format: "%s-%s"},
					},
					ToFieldPath: ptr.To("objectMeta.labels.destination"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test":        "blah",
							"destination": "foo-bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
				},
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ncp := tc.args.cp.DeepCopyObject().(resource.Composite)
			err := Apply(tc.args.patch, ncp, tc.args.cd, tc.args.only...)

			if tc.want.cp != nil {
				if diff := cmp.Diff(tc.want.cp, ncp); diff != "" {
					t.Errorf("\n%s\nApply(cp): -want, +got:\n%s", tc.reason, diff)
				}
			}
			if tc.want.cd != nil {
				if diff := cmp.Diff(tc.want.cd, tc.args.cd); diff != "" {
					t.Errorf("\n%s\nApply(cd): -want, +got:\n%s", tc.reason, diff)
				}
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nApply(err): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestOptionalFieldPathNotFound(t *testing.T) {
	errBoom := errors.New("boom")
	errNotFound := func() error {
		p := &fieldpath.Paved{}
		_, err := p.GetValue("boom")
		return err
	}
	required := v1.FromFieldPathPolicyRequired
	optional := v1.FromFieldPathPolicyOptional
	type args struct {
		err error
		p   *v1.PatchPolicy
	}

	cases := map[string]struct {
		reason string
		args
		want bool
	}{
		"NotAnError": {
			reason: "Should perform patch if no error finding field.",
			args:   args{},
			want:   false,
		},
		"NotFieldNotFoundError": {
			reason: "Should return error if something other than field not found.",
			args: args{
				err: errBoom,
			},
			want: false,
		},
		"DefaultOptionalNoPolicy": {
			reason: "Should return no-op if field not found and no patch policy specified.",
			args: args{
				err: errNotFound(),
			},
			want: true,
		},
		"DefaultOptionalNoPathPolicy": {
			reason: "Should return no-op if field not found and empty patch policy specified.",
			args: args{
				p:   &v1.PatchPolicy{},
				err: errNotFound(),
			},
			want: true,
		},
		"OptionalNotFound": {
			reason: "Should return no-op if field not found and optional patch policy explicitly specified.",
			args: args{
				p: &v1.PatchPolicy{
					FromFieldPath: &optional,
				},
				err: errNotFound(),
			},
			want: true,
		},
		"RequiredNotFound": {
			reason: "Should return error if field not found and required patch policy explicitly specified.",
			args: args{
				p: &v1.PatchPolicy{
					FromFieldPath: &required,
				},
				err: errNotFound(),
			},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsOptionalFieldPathNotFound(tc.args.err, tc.args.p)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("IsOptionalFieldPathNotFound(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestComposedTemplates(t *testing.T) {
	asJSON := func(val interface{}) extv1.JSON {
		raw, err := json.Marshal(val)
		if err != nil {
			t.Fatal(err)
		}
		res := extv1.JSON{}
		if err := json.Unmarshal(raw, &res); err != nil {
			t.Fatal(err)
		}
		return res
	}

	type args struct {
		pss []v1.PatchSet
		cts []v1.ComposedTemplate
	}

	type want struct {
		ct  []v1.ComposedTemplate
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"NoCompositionPatchSets": {
			reason: "Patches defined on a composite resource should be applied correctly if no PatchSets are defined on the composition",
			args: args{
				cts: []v1.ComposedTemplate{
					{
						Patches: []v1.Patch{
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("metadata.name"),
							},
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("metadata.namespace"),
							},
						},
					},
				},
			},
			want: want{
				ct: []v1.ComposedTemplate{
					{
						Patches: []v1.Patch{
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("metadata.name"),
							},
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("metadata.namespace"),
							},
						},
					},
				},
			},
		},
		"UndefinedPatchSet": {
			reason: "Should return error and not modify the patches field when referring to an undefined PatchSet",
			args: args{
				cts: []v1.ComposedTemplate{{
					Patches: []v1.Patch{
						{
							Type:         v1.PatchTypePatchSet,
							PatchSetName: ptr.To("patch-set-1"),
						},
					},
				}},
			},
			want: want{
				err: errors.Errorf(errFmtUndefinedPatchSet, "patch-set-1"),
			},
		},
		"DefinedPatchSets": {
			reason: "Should de-reference PatchSets defined on the Composition when referenced in a composed resource",
			args: args{
				// PatchSets, existing patches and references
				// should output in the correct order.
				pss: []v1.PatchSet{
					{
						Name: "patch-set-1",
						Patches: []v1.Patch{
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("metadata.namespace"),
							},
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("spec.parameters.test"),
							},
						},
					},
					{
						Name: "patch-set-2",
						Patches: []v1.Patch{
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("metadata.annotations.patch-test-1"),
							},
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("metadata.annotations.patch-test-2"),
								Transforms: []v1.Transform{{
									Type: v1.TransformTypeMap,
									Map: &v1.MapTransform{
										Pairs: map[string]extv1.JSON{
											"k-1": asJSON("v-1"),
											"k-2": asJSON("v-2"),
										},
									},
								}},
							},
						},
					},
				},
				cts: []v1.ComposedTemplate{
					{
						Patches: []v1.Patch{
							{
								Type:         v1.PatchTypePatchSet,
								PatchSetName: ptr.To("patch-set-2"),
							},
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("metadata.name"),
							},
							{
								Type:         v1.PatchTypePatchSet,
								PatchSetName: ptr.To("patch-set-1"),
							},
						},
					},
					{
						Patches: []v1.Patch{
							{
								Type:         v1.PatchTypePatchSet,
								PatchSetName: ptr.To("patch-set-1"),
							},
						},
					},
				},
			},
			want: want{
				err: nil,
				ct: []v1.ComposedTemplate{
					{
						Patches: []v1.Patch{
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("metadata.annotations.patch-test-1"),
							},
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("metadata.annotations.patch-test-2"),
								Transforms: []v1.Transform{{
									Type: v1.TransformTypeMap,
									Map: &v1.MapTransform{
										Pairs: map[string]extv1.JSON{
											"k-1": asJSON("v-1"),
											"k-2": asJSON("v-2"),
										},
									},
								}},
							},
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("metadata.name"),
							},
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("metadata.namespace"),
							},
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("spec.parameters.test"),
							},
						},
					},
					{
						Patches: []v1.Patch{
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("metadata.namespace"),
							},
							{
								Type:          v1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: ptr.To("spec.parameters.test"),
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ComposedTemplates(tc.args.pss, tc.args.cts)

			if diff := cmp.Diff(tc.want.ct, got); diff != "" {
				t.Errorf("\n%s\nrs.ComposedTemplates(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nrs.ComposedTemplates(...)): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestResolveTransforms(t *testing.T) {
	type args struct {
		ts    []v1.Transform
		input any
	}
	type want struct {
		output any
		err    error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "NoTransforms",
			args: args{
				ts: nil,
				input: map[string]interface{}{
					"spec": map[string]interface{}{
						"parameters": map[string]interface{}{
							"test": "test",
						},
					},
				},
			},
			want: want{
				output: map[string]interface{}{
					"spec": map[string]interface{}{
						"parameters": map[string]interface{}{
							"test": "test",
						},
					},
				},
			},
		},
		{
			name: "MathTransformWithConversionToFloat64",
			args: args{
				ts: []v1.Transform{{
					Type: v1.TransformTypeConvert,
					Convert: &v1.ConvertTransform{
						ToType: v1.TransformIOTypeFloat64,
					},
				}, {
					Type: v1.TransformTypeMath,
					Math: &v1.MathTransform{
						Multiply: ptr.To[int64](2),
					},
				}},
				input: int64(2),
			},
			want: want{
				output: float64(4),
			},
		},
		{
			name: "MathTransformWithConversionToInt64",
			args: args{
				ts: []v1.Transform{{
					Type: v1.TransformTypeConvert,
					Convert: &v1.ConvertTransform{
						ToType: v1.TransformIOTypeInt64,
					},
				}, {
					Type: v1.TransformTypeMath,
					Math: &v1.MathTransform{
						Multiply: ptr.To[int64](2),
					},
				}},
				input: int64(2),
			},
			want: want{
				output: int64(4),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveTransforms(v1.Patch{Transforms: tt.args.ts}, tt.args.input)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ResolveTransforms(...): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.output, got); diff != "" {
				t.Errorf("ResolveTransforms(...): -want, +got:\n%s", diff)
			}
		})
	}
}
