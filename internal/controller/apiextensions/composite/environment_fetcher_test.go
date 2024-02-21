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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

func TestFetch(t *testing.T) {
	errBoom := errors.New("boom")

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

	// Shortcut to convert a JSON map back to "regular" map.
	// This is necessary because int's get converted to float64 during
	// marshalling (JSON does have a distinct integer type).
	makeMap := func(m map[string]extv1.JSON) map[string]interface{} {
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		res := map[string]interface{}{}
		if err := json.Unmarshal(raw, &res); err != nil {
			t.Fatal(err)
		}
		return res
	}

	makeEnvironment := func(m map[string]interface{}) *Environment {
		env := &Environment{
			Unstructured: unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
		}
		if m != nil {
			env.Object = makeMap(makeJSON(m))
		}
		env.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   environmentGroup,
			Version: environmentVersion,
			Kind:    environmentKind,
		})
		return env
	}

	testData1 := map[string]interface{}{
		"int":  int(1),
		"bool": true,
		"str":  "some str",
		"array": []int{
			1, 2, 3, 4,
		},
		"test": map[string]interface{}{
			"foo": "bar",
			"complex": map[string]interface{}{
				"data": "val",
			},
		},
	}

	testData2 := map[string]interface{}{
		"int": int(2),
		"array": []int{
			1, 2, 3, 4, 5,
		},
		"test": map[string]interface{}{
			"foo":   "bar2",
			"hello": "world",
		},
	}

	testDataMerged := map[string]interface{}{
		"int":  int(2),
		"bool": true,
		"str":  "some str",
		"array": []int{
			1, 2, 3, 4, 5,
		},
		"test": map[string]interface{}{
			"foo":   "bar2",
			"hello": "world",
			"complex": map[string]interface{}{
				"data": "val",
			},
		},
		"hello": "world",
	}

	type args struct {
		kube     client.Client
		cr       *fake.Composite
		revision *v1.CompositionRevision
		required *bool
	}
	type want struct {
		env *Environment
		err error
	}

	type compositeModifier func(cr *fake.Composite)
	composite := func(mods ...compositeModifier) *fake.Composite {
		cr := &fake.Composite{}
		for _, f := range mods {
			f(cr)
		}
		return cr
	}
	withEnvironmentRefs := func(refs ...corev1.ObjectReference) compositeModifier {
		return func(cr *fake.Composite) {
			cr.SetEnvironmentConfigReferences(refs)
		}
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"DefaultOnNil": {
			reason: "It should return an empty EnvironmentConfig if environment is nil",
			args: args{
				cr:       composite(),
				revision: &v1.CompositionRevision{},
			},
			want: want{
				env: makeEnvironment(nil),
			},
		},
		"DefaultEnvironmentOnNil": {
			reason: "It should return the default environment if nothing else is selected",
			args: args{
				cr: composite(),
				revision: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							DefaultData: makeJSON(map[string]interface{}{
								"hello": "world",
							}),
						},
					},
				},
			},
			want: want{
				env: makeEnvironment(map[string]interface{}{
					"hello": "world",
				}),
			},
		},
		"DefaultEnvironmentOnEmpty": {
			reason: "It should return the init data if the ref list is empty.",
			args: args{
				cr: composite(
					withEnvironmentRefs(),
				),
				revision: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							DefaultData: makeJSON(map[string]interface{}{
								"hello": "world",
							}),
						},
					},
				},
			},
			want: want{
				env: makeEnvironment(map[string]interface{}{
					"hello": "world",
				}),
			},
		},
		"MergeMultipleSourcesInOrder": {
			reason: "It should merge the data of multiple EnvironmentConfigs in the order they are listed.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, o client.Object) error {
						cs := o.(*v1alpha1.EnvironmentConfig)
						switch key.Name {
						case "a":
							cs.Data = makeJSON(testData1)
						case "b":
							cs.Data = makeJSON(testData2)
						}
						return nil
					},
				},
				cr: composite(
					withEnvironmentRefs(
						corev1.ObjectReference{Name: "a"},
						corev1.ObjectReference{Name: "b"},
					),
				),
				revision: &v1.CompositionRevision{
					Spec: v1.CompositionRevisionSpec{
						Environment: &v1.EnvironmentConfiguration{
							DefaultData: makeJSON(map[string]interface{}{
								"hello": "world",
							}),
						},
					},
				},
			},
			want: want{
				env: makeEnvironment(testDataMerged),
			},
		},
		"ErrorOnKubeGetError": {
			reason: "It should return an error if getting a EnvironmentConfig from a reference fails",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				cr: composite(
					withEnvironmentRefs(
						corev1.ObjectReference{Name: "a"},
					),
				),
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(errBoom, errGetEnvironmentConfig), errFetchEnvironmentConfigs),
			},
		},
		"NoErrorOnKubeGetErrorIfResolutionNotRequired": {
			reason: "It should omit EnvironmentConfig if getting a EnvironmentConfig from a reference fails",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				cr: composite(
					withEnvironmentRefs(
						corev1.ObjectReference{Name: "a"},
					),
				),
				required: ptr.To(false),
			},
			want: want{
				env: makeEnvironment(map[string]interface{}{}),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := NewAPIEnvironmentFetcher(tc.args.kube)
			required := true
			if tc.args.required != nil {
				required = *tc.args.required
			}
			got, err := f.Fetch(context.Background(), EnvironmentFetcherRequest{
				Composite: tc.args.cr,
				Required:  required,
				Revision:  tc.args.revision,
			})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.env, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
