/*
Copyright 2023 The Crossplane Authors.

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

package meta

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	metav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep/resolver/image"
	"github.com/crossplane/crossplane/internal/xpkg/v2/scheme"
)

func TestUpsert(t *testing.T) {
	type args struct {
		metaFile runtime.Object
		dep      v1beta1.Dependency
	}

	type want struct {
		deps []v1beta1.Dependency
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"AddEntryNoPrior": {
			reason: "Should not return an error if package is created at path.",
			args: args{
				dep: dep.NewWithType(
					"crossplane/provider-gcp@v1.0.0",
					string(v1beta1.ConfigurationPackageType),
				),
				metaFile: &metav1.Configuration{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1",
						Kind:       "Configuration",
					},
					ObjectMeta: apimetav1.ObjectMeta{
						Name: "getting-started-with-aws",
					},
					Spec: metav1.ConfigurationSpec{
						MetaSpec: metav1.MetaSpec{
							Crossplane: &metav1.CrossplaneConstraints{
								Version: ">=1.0.0-0",
							},
						},
					},
				},
			},
			want: want{
				deps: []v1beta1.Dependency{
					{
						Package:     "crossplane/provider-gcp",
						Type:        v1beta1.ConfigurationPackageType,
						Constraints: "v1.0.0",
					},
				},
			},
		},
		"AddEntryNoPriorV1alpha1": {
			reason: "Should not return an error if package is created at path.",
			args: args{
				dep: dep.NewWithType(
					"crossplane/provider-gcp@v1.0.0",
					string(v1beta1.ConfigurationPackageType),
				),
				metaFile: &metav1alpha1.Configuration{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1",
						Kind:       "Configuration",
					},
					ObjectMeta: apimetav1.ObjectMeta{
						Name: "getting-started-with-aws",
					},
					Spec: metav1alpha1.ConfigurationSpec{
						MetaSpec: metav1alpha1.MetaSpec{
							Crossplane: &metav1alpha1.CrossplaneConstraints{
								Version: ">=1.0.0-0",
							},
						},
					},
				},
			},
			want: want{
				deps: []v1beta1.Dependency{
					{
						Package:     "crossplane/provider-gcp",
						Type:        v1beta1.ConfigurationPackageType,
						Constraints: "v1.0.0",
					},
				},
			},
		},
		"InsertNewEntry": {
			reason: "Should not return an error if package is created at path.",
			args: args{
				dep: dep.NewWithType(
					"crossplane/provider-gcp@v1.0.0",
					string(v1beta1.ProviderPackageType),
				),
				metaFile: &metav1.Provider{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1",
						Kind:       "Provider",
					},
					ObjectMeta: apimetav1.ObjectMeta{
						Name: "getting-started-with-aws",
					},
					Spec: metav1.ProviderSpec{
						MetaSpec: metav1.MetaSpec{
							Crossplane: &metav1.CrossplaneConstraints{
								Version: ">=1.0.0-0",
							},
							DependsOn: []metav1.Dependency{
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  ">=1.0.5",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []v1beta1.Dependency{
					{
						Package:     "crossplane/provider-aws",
						Type:        v1beta1.ProviderPackageType,
						Constraints: ">=1.0.5",
					},
					{
						Package:     "crossplane/provider-gcp",
						Type:        v1beta1.ProviderPackageType,
						Constraints: "v1.0.0",
					},
				},
			},
		},
		"InsertNewEntryV1alpha1": {
			reason: "Should not return an error if package is created at path.",
			args: args{
				dep: dep.NewWithType(
					"crossplane/provider-gcp@v1.0.0",
					string(v1beta1.ProviderPackageType),
				),
				metaFile: &metav1alpha1.Provider{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1",
						Kind:       "Provider",
					},
					ObjectMeta: apimetav1.ObjectMeta{
						Name: "getting-started-with-aws",
					},
					Spec: metav1alpha1.ProviderSpec{
						MetaSpec: metav1alpha1.MetaSpec{
							Crossplane: &metav1alpha1.CrossplaneConstraints{
								Version: ">=1.0.0-0",
							},
							DependsOn: []metav1alpha1.Dependency{
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  ">=1.0.5",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []v1beta1.Dependency{
					{
						Package:     "crossplane/provider-aws",
						Type:        v1beta1.ProviderPackageType,
						Constraints: ">=1.0.5",
					},
					{
						Package:     "crossplane/provider-gcp",
						Type:        v1beta1.ProviderPackageType,
						Constraints: "v1.0.0",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			m := New(tc.args.metaFile)

			err := m.Upsert(tc.args.dep)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUpsert(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			resultDeps, _ := m.DependsOn()

			if diff := cmp.Diff(tc.want.deps, resultDeps, cmpopts.SortSlices(func(i, j int) bool {
				return resultDeps[i].Package < resultDeps[j].Package
			})); diff != "" {
				t.Errorf("\n%s\nUpsert(...): -want err, +got err:\n%s", tc.reason, diff)
			}

		})
	}
}

func TestUpsertDeps(t *testing.T) {
	type args struct {
		dep v1beta1.Dependency
		pkg runtime.Object
	}

	type want struct {
		deps []metav1.Dependency
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyDependencyList": {
			reason: "Should return an updated deps list with the included provider.",
			args: args{
				dep: dep.NewWithType(
					"crossplane/provider-aws@v1.0.0",
					string(v1beta1.ProviderPackageType),
				),
				pkg: &metav1.Configuration{
					Spec: metav1.ConfigurationSpec{
						MetaSpec: metav1.MetaSpec{
							DependsOn: []metav1.Dependency{},
						},
					},
				},
			},
			want: want{
				deps: []metav1.Dependency{
					{
						Provider: pointer.String("crossplane/provider-aws"),
						Version:  "v1.0.0",
					},
				},
			},
		},
		"EmptyDependencyListV1alpha1": {
			reason: "Should return an updated deps list with the included provider, for v1alpha1.",
			args: args{
				dep: dep.NewWithType(
					"crossplane/provider-aws@v1.0.0",
					string(v1beta1.ProviderPackageType),
				),
				pkg: &metav1alpha1.Configuration{
					Spec: metav1alpha1.ConfigurationSpec{
						MetaSpec: metav1alpha1.MetaSpec{
							DependsOn: []metav1alpha1.Dependency{},
						},
					},
				},
			},
			want: want{
				deps: []metav1.Dependency{
					{
						Provider: pointer.String("crossplane/provider-aws"),
						Version:  "v1.0.0",
					},
				},
			},
		},
		"InsertIntoDependencyList": {
			reason: "Should return an updated deps list with 2 entries.",
			args: args{
				dep: dep.NewWithType(
					"crossplane/provider-gcp@v1.0.1",
					string(v1beta1.ProviderPackageType),
				),
				pkg: &metav1.Configuration{
					Spec: metav1.ConfigurationSpec{
						MetaSpec: metav1.MetaSpec{
							DependsOn: []metav1.Dependency{
								{
									Configuration: pointer.String("crossplane/provider-aws"),
									Version:       "v1.0.0",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []metav1.Dependency{
					{
						Configuration: pointer.String("crossplane/provider-aws"),
						Version:       "v1.0.0",
					},
					{
						Provider: pointer.String("crossplane/provider-gcp"),
						Version:  "v1.0.1",
					},
				},
			},
		},
		"InsertIntoDependencyListV1alpha1": {
			reason: "Should return an updated deps list with 2 entries, even for v1alpha packages.",
			args: args{
				dep: dep.NewWithType(
					"crossplane/provider-gcp@v1.0.1",
					string(v1beta1.ProviderPackageType),
				),
				pkg: &metav1alpha1.Configuration{
					Spec: metav1alpha1.ConfigurationSpec{
						MetaSpec: metav1alpha1.MetaSpec{
							DependsOn: []metav1alpha1.Dependency{
								{
									Configuration: pointer.String("crossplane/provider-aws"),
									Version:       "v1.0.0",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []metav1.Dependency{
					{
						Configuration: pointer.String("crossplane/provider-aws"),
						Version:       "v1.0.0",
					},
					{
						Provider: pointer.String("crossplane/provider-gcp"),
						Version:  "v1.0.1",
					},
				},
			},
		},
		"UpdateDependencyList": {
			reason: "Should return an updated deps list with the provider version updated.",
			args: args{
				dep: dep.NewWithType(
					"crossplane/provider-aws@v1.0.1",
					string(v1beta1.ConfigurationPackageType),
				),
				pkg: &metav1.Provider{
					Spec: metav1.ProviderSpec{
						MetaSpec: metav1.MetaSpec{
							DependsOn: []metav1.Dependency{
								{
									Configuration: pointer.String("crossplane/provider-aws"),
									Version:       "v1.0.0",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []metav1.Dependency{
					{
						Configuration: pointer.String("crossplane/provider-aws"),
						Version:       "v1.0.1",
					},
				},
			},
		},
		"UpdateDependencyListV1alpha1": {
			reason: "Should return an updated deps list with the provider version updated, for v1alpha1.",
			args: args{
				dep: dep.NewWithType(
					"crossplane/provider-aws@v1.0.1",
					string(v1beta1.ConfigurationPackageType),
				),
				pkg: &metav1alpha1.Provider{
					Spec: metav1alpha1.ProviderSpec{
						MetaSpec: metav1alpha1.MetaSpec{
							DependsOn: []metav1alpha1.Dependency{
								{
									Configuration: pointer.String("crossplane/provider-aws"),
									Version:       "v1.0.0",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []metav1.Dependency{
					{
						Configuration: pointer.String("crossplane/provider-aws"),
						Version:       "v1.0.1",
					},
				},
			},
		},
		"UseDefaultTag": {
			reason: "Should return an error indicating the package name is invalid.",
			args: args{
				dep: dep.NewWithType(
					"crossplane/provider-aws",
					string(v1beta1.ProviderPackageType),
				),
				pkg: &metav1.Provider{
					Spec: metav1.ProviderSpec{
						MetaSpec: metav1.MetaSpec{
							DependsOn: []metav1.Dependency{
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  "v1.0.0",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []metav1.Dependency{
					{
						Provider: pointer.String("crossplane/provider-aws"),
						Version:  image.DefaultVer,
					},
				},
			},
		},
		"UseDefaultTagV1alpha1": {
			reason: "Should return an error indicating the package name is invalid, for v1alpha1.",
			args: args{
				dep: dep.NewWithType(
					"crossplane/provider-aws",
					string(v1beta1.ProviderPackageType),
				),
				pkg: &metav1alpha1.Provider{
					Spec: metav1alpha1.ProviderSpec{
						MetaSpec: metav1alpha1.MetaSpec{
							DependsOn: []metav1alpha1.Dependency{
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  "v1.0.0",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []metav1.Dependency{
					{
						Provider: pointer.String("crossplane/provider-aws"),
						Version:  image.DefaultVer,
					},
				},
			},
		},
		"DuplicateDep": {
			reason: "Should return an error indicating duplicate dependencies detected.",
			args: args{
				dep: dep.NewWithType(
					"crossplane/provider-aws",
					string(v1beta1.ProviderPackageType),
				),
				pkg: &metav1.Provider{
					Spec: metav1.ProviderSpec{
						MetaSpec: metav1.MetaSpec{
							DependsOn: []metav1.Dependency{
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  "v1.0.0",
								},
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  "v1.0.1",
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.New(errMetaContainsDupeDep),
			},
		},
		"DuplicateDepV1alpha1": {
			reason: "Should return an error indicating duplicate dependencies detected, v1alpha1.",
			args: args{
				dep: dep.NewWithType(
					"crossplane/provider-aws",
					string(v1beta1.ProviderPackageType),
				),
				pkg: &metav1alpha1.Provider{
					Spec: metav1alpha1.ProviderSpec{
						MetaSpec: metav1alpha1.MetaSpec{
							DependsOn: []metav1alpha1.Dependency{
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  "v1.0.0",
								},
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  "v1.0.1",
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.New(errMetaContainsDupeDep),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			err := upsertDeps(tc.args.dep, tc.args.pkg)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUpsertDeps(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if tc.want.deps != nil {
				p, _ := scheme.TryConvertToPkg(tc.args.pkg, &metav1.Provider{}, &metav1.Configuration{})
				if diff := cmp.Diff(tc.want.deps, p.GetDependencies()); diff != "" {
					t.Errorf("\n%s\nUpsertDeps(...): -want err, +got err:\n%s", tc.reason, diff)
				}
			}
		})
	}
}

func TestDependsOn(t *testing.T) {
	type args struct {
		metaFile runtime.Object
	}

	type want struct {
		deps []v1beta1.Dependency
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SingleDependency": {
			reason: "Should return a slice with a single entry.",
			args: args{
				metaFile: &metav1.Configuration{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1",
						Kind:       "Configuration",
					},
					ObjectMeta: apimetav1.ObjectMeta{
						Name: "getting-started-with-aws",
					},
					Spec: metav1.ConfigurationSpec{
						MetaSpec: metav1.MetaSpec{
							Crossplane: &metav1.CrossplaneConstraints{
								Version: ">=1.0.0-0",
							},
							DependsOn: []metav1.Dependency{
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  "v1.0.0",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []v1beta1.Dependency{
					{
						Package:     "crossplane/provider-aws",
						Type:        v1beta1.ProviderPackageType,
						Constraints: "v1.0.0",
					},
				},
			},
		},
		"SingleDependencyV1alpha1": {
			reason: "Should return a slice with a single entry even for v1alpha1 files.",
			args: args{
				metaFile: &metav1alpha1.Configuration{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1",
						Kind:       "Configuration",
					},
					ObjectMeta: apimetav1.ObjectMeta{
						Name: "getting-started-with-aws",
					},
					Spec: metav1alpha1.ConfigurationSpec{
						MetaSpec: metav1alpha1.MetaSpec{
							Crossplane: &metav1alpha1.CrossplaneConstraints{
								Version: ">=1.0.0-0",
							},
							DependsOn: []metav1alpha1.Dependency{
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  "v1.0.0",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []v1beta1.Dependency{
					{
						Package:     "crossplane/provider-aws",
						Type:        v1beta1.ProviderPackageType,
						Constraints: "v1.0.0",
					},
				},
			},
		},
		"MultipleDependencies": {
			reason: "Should return a slice with multiple entries.",
			args: args{
				metaFile: &metav1.Provider{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1",
						Kind:       "Provider",
					},
					ObjectMeta: apimetav1.ObjectMeta{
						Name: "getting-started-with-aws",
					},
					Spec: metav1.ProviderSpec{
						MetaSpec: metav1.MetaSpec{
							Crossplane: &metav1.CrossplaneConstraints{
								Version: ">=1.0.0-0",
							},
							DependsOn: []metav1.Dependency{
								{
									Configuration: pointer.String("crossplane/provider-gcp"),
									Version:       ">=v1.0.1",
								},
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  "v1.0.0",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []v1beta1.Dependency{
					{
						Package:     "crossplane/provider-gcp",
						Type:        v1beta1.ConfigurationPackageType,
						Constraints: ">=v1.0.1",
					},
					{
						Package:     "crossplane/provider-aws",
						Type:        v1beta1.ProviderPackageType,
						Constraints: "v1.0.0",
					},
				},
			},
		},
		"NoDependencies": {
			reason: "Should return an empty slice.",
			args: args{
				metaFile: &metav1.Provider{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1",
						Kind:       "Provider",
					},
					ObjectMeta: apimetav1.ObjectMeta{
						Name: "getting-started-with-aws",
					},
					Spec: metav1.ProviderSpec{
						MetaSpec: metav1.MetaSpec{
							Crossplane: &metav1.CrossplaneConstraints{
								Version: ">=1.0.0-0",
							},
						},
					},
				},
			},
			want: want{
				deps: []v1beta1.Dependency{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			m := New(tc.args.metaFile)

			got, err := m.DependsOn()
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDependsOn(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.deps, got, cmpopts.SortSlices(func(i, j int) bool {
				return got[i].Package < got[j].Package
			})); diff != "" {
				t.Errorf("\n%s\nDependsOn(...): -want err, +got err:\n%s", tc.reason, diff)
			}

		})
	}
}
