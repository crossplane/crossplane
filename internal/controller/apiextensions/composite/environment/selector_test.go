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
package environment

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

func TestSelect(t *testing.T) {
	errBoom := errors.New("boom")
	type args struct {
		kube client.Client
		cr   *fake.Composite
		comp *v1.Composition
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

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoopOnNilEnvironment": {
			reason: "It should be a noop if the composition does not configure an environment.",
			args: args{
				cr:   composite(),
				comp: &v1.Composition{},
			},
			want: want{
				cr: composite(),
			},
		},
		"NoopOnNilEnvironmentSources": {
			reason: "It should be a noop if the composition does not configure an environment.",
			args: args{
				cr: composite(),
				comp: &v1.Composition{
					Spec: v1.CompositionSpec{
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
				comp: &v1.Composition{
					Spec: v1.CompositionSpec{
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
				comp: &v1.Composition{
					Spec: v1.CompositionSpec{
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
				comp: &v1.Composition{
					Spec: v1.CompositionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: pointer.String("bar"),
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
				comp: &v1.Composition{
					Spec: v1.CompositionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:               v1.EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath,
												Key:                "foo",
												ValueFromFieldPath: pointer.String("objectMeta.name"),
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
			reason: "It should create a name reference for the first selected EnvironmentConfig that matches the labels.",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1alpha1.EnvironmentConfigList)
						list.Items = []v1alpha1.EnvironmentConfig{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test",
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "not-this-one",
								},
							},
						}
						return nil
					}),
				},
				cr: composite(
					withEnvironmentRefs(environmentConfigRef("test")),
				),
				comp: &v1.Composition{
					Spec: v1.CompositionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Key:   "foo",
												Value: pointer.String("bar"),
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
					withEnvironmentRefs(environmentConfigRef("test")),
				),
			},
		},
		"RefsInOrder": {
			reason: "It should create the reference list in order of the configuration.",
			args: args{
				cr: composite(),
				comp: &v1.Composition{
					Spec: v1.CompositionSpec{
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
				comp: &v1.Composition{
					Spec: v1.CompositionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: pointer.String("bar"),
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
			reason: "It should return an empty list of references if kube.List returns an empty list.",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
				},
				cr: composite(),
				comp: &v1.Composition{
					Spec: v1.CompositionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:  v1.EnvironmentSourceSelectorLabelMatcherTypeValue,
												Key:   "foo",
												Value: pointer.String("bar"),
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
		"ErrorOnInvalidLabelValueFieldPath": {
			reason: "It should return an error if the path to a label value is invalid.",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
				},
				cr: composite(),
				comp: &v1.Composition{
					Spec: v1.CompositionSpec{
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: v1.EnvironmentSourceTypeSelector,
									Selector: &v1.EnvironmentSourceSelector{
										MatchLabels: []v1.EnvironmentSourceSelectorLabelMatcher{
											{
												Type:               v1.EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath,
												Key:                "foo",
												ValueFromFieldPath: pointer.String("wrong.path"),
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
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := NewAPIEnvironmentSelector(tc.args.kube)
			err := s.SelectEnvironment(context.Background(), tc.args.cr, tc.args.comp)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cr, tc.args.cr, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
