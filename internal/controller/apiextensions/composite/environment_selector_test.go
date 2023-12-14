/*
Copyright 2022 The Crossplane Authors.

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
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

func TestSelect(t *testing.T) {
	errBoom := errors.New("boom")
	type args struct {
		kube client.Client
		cr   *fake.Composite
		rev  *v1.CompositionRevision
	}
	type want struct {
		cr  *fake.Composite
		err error
	}

	now := metav1.Now()

	type compositeModifier func(cr *fake.Composite)
	composite := func(mods ...compositeModifier) *fake.Composite {
		cr := &fake.Composite{}
		cr.SetCreationTimestamp(now)
		cr.SetConnectionDetailsLastPublishedTime(&now)
		for _, f := range mods {
			f(cr)
		}
		return cr
	}
	withName := func(name string) compositeModifier {
		return func(cr *fake.Composite) {
			cr.SetName(name)
		}
	}
	withEnvironmentRefs := func(refs ...corev1.ObjectReference) compositeModifier {
		return func(cr *fake.Composite) {
			cr.SetEnvironmentConfigReferences(refs)
		}
	}
	environmentConfigRef := func(name string) corev1.ObjectReference {
		return corev1.ObjectReference{
			Name:       name,
			Kind:       v1alpha1.EnvironmentConfigKind,
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
		}
	}

	makeJSON := func(m map[string]interface{}) map[string]extv1.JSON {
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		res := map[string]extv1.JSON{}
		if err := json.Unmarshal(raw, &res); err != nil {
			t.Fatal(err)
		}
		return res
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoopOnNilEnvironment": {
			reason: "It should be a noop if the composition does not configure an environment.",
			args: args{
				cr:  composite(),
				rev: &v1.CompositionRevision{},
			},
			want: want{
				cr: composite(),
			},
		},
		"NoopOnNilEnvironmentSources": {
			reason: "It should be a noop if the composition does not configure an environment.",
			args: args{
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{},
					},
				},
			},
			want: want{
				cr: composite(),
			},
		},
		"EmptyRefsOnEmptyConfigRefs": {
			reason: "It should create an empty list of references if the config reference list is empty.",
			args: args{
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs(),
				),
			},
		},
		"RefForRef": {
			reason: "It should create a name reference for a named reference in the config.",
			args: args{
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeReference,
									Ref: &v1.EnvironmentSourceReference{
										Name: "test",
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs(environmentConfigRef("test")),
				),
			},
		},
		"RefForLabelSelectedObjects": {
			reason: "It should create a name reference for selected EnvironmentConfigs that match the labels.",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1alpha1.EnvironmentConfigList)
						list.Items = []v1alpha1.EnvironmentConfig{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-1",
									Labels: map[string]string{
										"foo": "bar",
									},
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-2",
									Labels: map[string]string{
										"foo": "bar",
									},
								},
							},
						}
						return nil
					}),
				},
				cr: composite(
					withName("test-composite"),
				),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										Mode:            v1.EnvironmentSourceSelectorMultiMode,
										SortByFieldPath: "metadata.name",
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: ptr.To("bar"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withName("test-composite"),
					withEnvironmentRefs(environmentConfigRef("test-1"), environmentConfigRef("test-2")),
				),
			},
		},
		"RefForLabelSelectedObjectWithLabelValueFromFieldPath": {
			reason: "It should create a name reference for the first selected EnvironmentConfig that matches the labels.",
			args: args{
				kube: &test.MockClient{
					MockList: func(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error {
						list := obj.(*v1alpha1.EnvironmentConfigList)
						match := opts[0].(client.MatchingLabels)
						if match["foo"] != "test-composite" {
							return errors.Errorf("Expected label selector value to be 'foo', but was '%s'", match["foo"])
						}
						list.Items = []v1alpha1.EnvironmentConfig{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test",
								},
							},
						}
						return nil
					},
				},
				cr: composite(
					withName("test-composite"),
				),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										Mode: v1.EnvironmentSourceSelectorSingleMode,
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:               v1.EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath,
												Key:                "foo",
												ValueFromFieldPath: ptr.To("objectMeta.name"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withName("test-composite"),
					withEnvironmentRefs(environmentConfigRef("test")),
				),
			},
		},
		"RefForFirstLabelSelectedObject": {
			reason: "It should create a name reference for the single selected EnvironmentConfig that matches the labels.",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1alpha1.EnvironmentConfigList)
						list.Items = []v1alpha1.EnvironmentConfig{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-1",
								},
							},
						}
						return nil
					}),
				},
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										Mode: v1.EnvironmentSourceSelectorSingleMode,
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: ptr.To("bar"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs(environmentConfigRef("test-1")),
				),
			},
		},
		"ErrorOnMultipleObjectsInSingleMode": {
			reason: "It should return an error if more than 1 EnvironmentConfigs match the labels.",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1alpha1.EnvironmentConfigList)
						list.Items = []v1alpha1.EnvironmentConfig{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-1",
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-not-this-one",
								},
							},
						}
						return nil
					}),
				},
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										Mode: v1.EnvironmentSourceSelectorSingleMode,
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: ptr.To("bar"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr:  composite(),
				err: errors.Wrap(fmt.Errorf(errFmtFoundMultipleInSingleMode, 2), "failed to build reference at index 0"),
			},
		},
		"RefsInOrder": {
			reason: "It should create the reference list in order of the configuration.",
			args: args{
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeReference,
									Ref: &v1.EnvironmentSourceReference{
										Name: "test1",
									},
								},
								{
									Type: v1.EnvironmentSourceTypeReference,
									Ref: &v1.EnvironmentSourceReference{
										Name: "test2",
									},
								},
								{
									Type: v1.EnvironmentSourceTypeReference,
									Ref: &v1.EnvironmentSourceReference{
										Name: "test3",
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs(
						environmentConfigRef("test1"),
						environmentConfigRef("test2"),
						environmentConfigRef("test3"),
					),
				),
			},
		},
		"ErrorOnKubeListError": {
			reason: "It should return an error if kube.List returns an error.",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				},
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: ptr.To("bar"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr:  composite(),
				err: errors.Wrapf(errors.Wrap(errBoom, errListEnvironmentConfigs), errFmtReferenceEnvironmentConfig, 0),
			},
		},
		"NoReferenceOnKubeListEmpty": {
			reason: "It should return an empty list of references if kube.List returns an empty list and Config is optional.",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
				},
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							Policy: &xpv1.Policy{
								Resolution: ptr.To(xpv1.ResolutionPolicyOptional),
							},
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										Mode:     v1.EnvironmentSourceSelectorMultiMode,
										MinMatch: ptr.To[uint64](0),
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: ptr.To("bar"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs([]corev1.ObjectReference{}...),
				),
			},
		},
		"ErrSelectNotFoundRequiredConfig": {
			reason: "It should return error if not found Config is mandatory.",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1alpha1.EnvironmentConfigList)
						list.Items = []v1alpha1.EnvironmentConfig{}
						return nil
					}),
				},
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										Mode:            v1.EnvironmentSourceSelectorMultiMode,
										SortByFieldPath: "metadata.annotations[int/weight]",
										MaxMatch:        ptr.To[uint64](3),
										MinMatch:        ptr.To[uint64](1),
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: ptr.To("bar"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs(),
				),
				err: errors.Wrap(fmt.Errorf("expected at least 1 EnvironmentConfig(s) with matching labels, found: 0"), "failed to build reference at index 0"),
			},
		},
		"ErrorOnInvalidLabelValueFieldPath": {
			reason: "It should return an error if the path to a label value is invalid.",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
				},
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:               v1.EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath,
												Key:                "foo",
												ValueFromFieldPath: ptr.To("wrong.path"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs(),
				),
				err: errors.Wrapf(errors.Wrapf(errors.New("wrong: no such field"), errFmtResolveLabelValue, 0), errFmtReferenceEnvironmentConfig, 0),
			},
		},
		"NoErrorOnInvalidOptionalLabelValueFieldPath": {
			reason: "It should not return an error if the path to a label value is invalid, but was set as optional.",
			args: args{
				kube: &test.MockClient{},
				cr:   composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										Mode:     v1.EnvironmentSourceSelectorMultiMode,
										MinMatch: ptr.To[uint64](0),
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:                v1.EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath,
												Key:                 "foo",
												ValueFromFieldPath:  ptr.To("wrong.path"),
												FromFieldPathPolicy: &[]v1.FromFieldPathPolicy{v1.FromFieldPathPolicyOptional}[0],
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs(),
				),
			},
		},
		"ErrorOnInvalidRequiredLabelValueFieldPath": {
			reason: "It should return an error if the path to a label value is invalid and set as required.",
			args: args{
				kube: &test.MockClient{},
				cr:   composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:                v1.EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath,
												Key:                 "foo",
												ValueFromFieldPath:  ptr.To("wrong.path"),
												FromFieldPathPolicy: &[]v1.FromFieldPathPolicy{v1.FromFieldPathPolicyRequired}[0],
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs(),
				),
				err: errors.Wrapf(errors.Wrapf(errors.New("wrong: no such field"), errFmtResolveLabelValue, 0), errFmtReferenceEnvironmentConfig, 0),
			},
		},
		"AllRefsSortedInMultiMode": {
			reason: "It should return complete list of references sorted by metadata.name",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1alpha1.EnvironmentConfigList)
						list.Items = []v1alpha1.EnvironmentConfig{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-2",
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-1",
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-4",
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-3",
								},
							},
						}
						return nil
					}),
				},
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										Mode:            v1.EnvironmentSourceSelectorMultiMode,
										SortByFieldPath: "metadata.name",
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: ptr.To("bar"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs([]corev1.ObjectReference{
						{
							Name:       "test-1",
							Kind:       v1alpha1.EnvironmentConfigKind,
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
						},
						{

							Name:       "test-2",
							Kind:       v1alpha1.EnvironmentConfigKind,
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
						},
						{

							Name:       "test-3",
							Kind:       v1alpha1.EnvironmentConfigKind,
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
						},
						{
							Name:       "test-4",
							Kind:       v1alpha1.EnvironmentConfigKind,
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
						},
					}...),
				),
			},
		},
		"MaxMatchRefsSortedInMultiMode": {
			reason: "It should return limited list of references sorted by specified annotation",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1alpha1.EnvironmentConfigList)
						list.Items = []v1alpha1.EnvironmentConfig{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-2",
									Annotations: map[string]string{
										"sort.by/weight": "2",
									},
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-1",
									Annotations: map[string]string{
										"sort.by/weight": "1",
									},
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-4",
									Annotations: map[string]string{
										"sort.by/weight": "4",
									},
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-3",
									Annotations: map[string]string{
										"sort.by/weight": "3",
									},
								},
							},
						}
						return nil
					}),
				},
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										Mode:            v1.EnvironmentSourceSelectorMultiMode,
										SortByFieldPath: "metadata.annotations[sort.by/weight]",
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: ptr.To("bar"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs([]corev1.ObjectReference{
						{
							Name:       "test-1",
							Kind:       v1alpha1.EnvironmentConfigKind,
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
						},
						{
							Name:       "test-2",
							Kind:       v1alpha1.EnvironmentConfigKind,
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
						},
						{
							Name:       "test-3",
							Kind:       v1alpha1.EnvironmentConfigKind,
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
						},
						{
							Name:       "test-4",
							Kind:       v1alpha1.EnvironmentConfigKind,
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
						},
					}...),
				),
			},
		},
		"MaxMatchRefsSortedByFloatInMultiMode": {
			reason: "It should return limited list of references sorted by float values",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1alpha1.EnvironmentConfigList)
						list.Items = []v1alpha1.EnvironmentConfig{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-2",
								},
								Data: makeJSON(
									map[string]interface{}{
										"float/weight": float64(1.2),
									},
								),
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-1",
								},
								Data: makeJSON(
									map[string]interface{}{
										"float/weight": float64(1.1),
									},
								),
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-4",
								},
								Data: makeJSON(
									map[string]interface{}{
										"float/weight": float64(1.4),
									},
								),
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-3",
								},
								Data: makeJSON(
									map[string]interface{}{
										"float/weight": float64(1.3),
									},
								),
							},
						}
						return nil
					}),
				},
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										Mode:            v1.EnvironmentSourceSelectorMultiMode,
										SortByFieldPath: "data[float/weight]",
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: ptr.To("bar"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs([]corev1.ObjectReference{
						{
							Name:       "test-1",
							Kind:       v1alpha1.EnvironmentConfigKind,
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
						},
						{
							Name:       "test-2",
							Kind:       v1alpha1.EnvironmentConfigKind,
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
						},
						{
							Name:       "test-3",
							Kind:       v1alpha1.EnvironmentConfigKind,
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
						},
						{
							Name:       "test-4",
							Kind:       v1alpha1.EnvironmentConfigKind,
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
						},
					}...),
				),
			},
		},
		"MaxMatchRefsSortedByIntInMultiMode": {
			reason: "It should return limited list of references sorted by int values",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1alpha1.EnvironmentConfigList)
						list.Items = []v1alpha1.EnvironmentConfig{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-2",
								},
								Data: makeJSON(
									map[string]interface{}{
										"int/weight": int64(2),
									},
								),
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-1",
								},
								Data: makeJSON(
									map[string]interface{}{
										"int/weight": int64(1),
									},
								),
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-3",
								},
								Data: makeJSON(
									map[string]interface{}{
										"int/weight": int64(3),
									},
								),
							},
						}
						return nil
					}),
				},
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										Mode:            v1.EnvironmentSourceSelectorMultiMode,
										MaxMatch:        ptr.To[uint64](4),
										SortByFieldPath: "data[int/weight]",
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: ptr.To("bar"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs([]corev1.ObjectReference{
						{
							Name:       "test-1",
							Kind:       v1alpha1.EnvironmentConfigKind,
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
						},
						{

							Name:       "test-2",
							Kind:       v1alpha1.EnvironmentConfigKind,
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
						},
						{

							Name:       "test-3",
							Kind:       v1alpha1.EnvironmentConfigKind,
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
						},
					}...),
				),
			},
		},
		"ErrSelectOnNotMatchingType": {
			reason: "It should return when types of copared values dont match",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1alpha1.EnvironmentConfigList)
						list.Items = []v1alpha1.EnvironmentConfig{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-2",
								},
								Data: makeJSON(
									map[string]interface{}{
										"int/weight": float64(2.1),
									},
								),
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-1",
								},
								Data: makeJSON(
									map[string]interface{}{
										"int/weight": int64(1),
									},
								),
							},
						}
						return nil
					}),
				},
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										Mode:            v1.EnvironmentSourceSelectorMultiMode,
										SortByFieldPath: "data[int/weight]",
										MaxMatch:        ptr.To[uint64](3),
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: ptr.To("bar"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs(),
				),
				err: errors.Wrap(fmt.Errorf(errFmtSortNotMatchingTypes, int64(1), reflect.Float64), "failed to build reference at index 0"),
			},
		},
		"ErrSelectOnUnexpectedType": {
			reason: "It should return error when compared values have unexpected types",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1alpha1.EnvironmentConfigList)
						list.Items = []v1alpha1.EnvironmentConfig{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-2",
								},
								Data: makeJSON(
									map[string]interface{}{
										"int/weight": true,
									},
								),
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-1",
								},
								Data: makeJSON(
									map[string]interface{}{
										"int/weight": true,
									},
								),
							},
						}
						return nil
					}),
				},
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										Mode:            v1.EnvironmentSourceSelectorMultiMode,
										SortByFieldPath: "data[int/weight]",
										MaxMatch:        ptr.To[uint64](3),
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: ptr.To("bar"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs(),
				),
				err: errors.Wrap(fmt.Errorf("unexpected type bool"), "failed to build reference at index 0"),
			},
		},
		"ErrSelectOnInvalidFieldPath": {
			reason: "It should return error on invalid field path",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1alpha1.EnvironmentConfigList)
						list.Items = []v1alpha1.EnvironmentConfig{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-2",
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-1",
								},
							},
						}
						return nil
					}),
				},
				cr: composite(),
				rev: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										Mode:            v1.EnvironmentSourceSelectorMultiMode,
										SortByFieldPath: "metadata.annotations[int/weight]",
										MaxMatch:        ptr.To[uint64](3),
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: ptr.To("bar"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				cr: composite(
					withEnvironmentRefs(),
				),
				err: errors.Wrap(fmt.Errorf("metadata.annotations: no such field"), "failed to build reference at index 0"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := NewAPIEnvironmentSelector(tc.args.kube)
			err := s.SelectEnvironment(context.Background(), tc.args.cr, tc.args.rev)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cr, tc.args.cr, test.EquateErrors(), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
