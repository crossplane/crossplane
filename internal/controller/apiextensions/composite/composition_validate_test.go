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
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/pointer"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestRejectMixedTemplates(t *testing.T) {
	cases := map[string]struct {
		comp *v1.Composition
		want error
	}{
		"Mixed": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Resources: []v1.ComposedTemplate{
						{
							// Unnamed.
						},
						{
							Name: pointer.String("cool"),
						},
					},
				},
			},
			want: errors.New(errMixed),
		},
		"Anonymous": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Resources: []v1.ComposedTemplate{
						{
							// Unnamed.
						},
						{
							// Unnamed.
						},
					},
				},
			},
			want: nil,
		},
		"Named": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Resources: []v1.ComposedTemplate{
						{
							Name: pointer.String("cool"),
						},
						{
							Name: pointer.String("cooler"),
						},
					},
				},
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RejectMixedTemplates(tc.comp)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nRejectMixedTemplates(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestRejectDuplicateNames(t *testing.T) {
	cases := map[string]struct {
		comp *v1.Composition
		want error
	}{
		"Unique": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Resources: []v1.ComposedTemplate{
						{
							Name: pointer.String("cool"),
						},
						{
							Name: pointer.String("cooler"),
						},
					},
				},
			},
			want: nil,
		},
		"Anonymous": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Resources: []v1.ComposedTemplate{
						{
							// Unnamed.
						},
						{
							// Unnamed.
						},
					},
				},
			},
			want: nil,
		},
		"Duplicates": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Resources: []v1.ComposedTemplate{
						{
							Name: pointer.String("cool"),
						},
						{
							Name: pointer.String("cool"),
						},
					},
				},
			},
			want: errors.New(errDuplicate),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RejectDuplicateNames(tc.comp)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nRejectDuplicateNames(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestRejectAnonymousTemplatesWithFunctions(t *testing.T) {
	cases := map[string]struct {
		comp *v1.Composition
		want error
	}{
		"AnonymousAndCompFnsNotInUse": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Resources: []v1.ComposedTemplate{
						{
							// Anonymous
						},
						{
							// Anonymous
						},
					},
					// Functions array is empty.
				},
			},
			want: nil,
		},
		"AnonymousAndCompFnsInUse": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Resources: []v1.ComposedTemplate{
						{
							// Anonymous
						},
						{
							// Anonymous
						},
					},
					Functions: []v1.Function{{
						Name: "cool-fn",
					}},
				},
			},
			want: errors.New(errFnsRequireNames),
		},
		"NamedAndCompFnsInUse": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Resources: []v1.ComposedTemplate{
						{
							Name: pointer.String("cool"),
						},
						{
							Name: pointer.String("cooler"),
						},
					},
					Functions: []v1.Function{{
						Name: "cool-fn",
					}},
				},
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RejectAnonymousTemplatesWithFunctions(tc.comp)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nRejectAnonymousTemplatesWithFunctions(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestRejectFunctionsWithoutRequiredConfig(t *testing.T) {
	cases := map[string]struct {
		comp *v1.Composition
		want error
	}{
		"UnknownType": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Functions: []v1.Function{{
						Type: "wat",
					}},
				},
			},
			want: errors.Errorf(errFmtUnknownFnType, "wat"),
		},
		"MissingContainerConfig": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Functions: []v1.Function{{
						Type: v1.FunctionTypeContainer,
					}},
				},
			},
			want: errors.New(errFnMissingContainerConfig),
		},
		"HasContainerConfig": {
			comp: &v1.Composition{
				Spec: v1.CompositionSpec{
					Functions: []v1.Function{{
						Type: v1.FunctionTypeContainer,
						Container: &v1.ContainerFunction{
							Image: "example.org/coolimg",
						},
					}},
				},
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RejectFunctionsWithoutRequiredConfig(tc.comp)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nRejectFunctionsWithoutRequiredConfig(...): -want, +got:\n%s", diff)
			}
		})
	}
}
