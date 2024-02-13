/*
Copyright 2021 The Crossplane Authors.

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

package initializer

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

const (
	errFmtGetProvider              = "unexpected name in provider get: %s"
	errFmtPatchProvider            = "unexpected name in provider update: %s"
	errFmtPatchProviderSource      = "unexpected source in provider update: %s"
	errFmtGetConfiguration         = "unexpected name in configuration get: %s"
	errFmtPatchConfiguration       = "unexpected name in configuration update: %s"
	errFmtPatchConfigurationSource = "unexpected source in configuration update: %s"
)

var errBoom = errors.New("boom")

func TestInstaller(t *testing.T) {
	p1Existing := "existing-provider"
	p1 := "crossplane/provider-aws:v1.16.0"
	p1Repo := "crossplane/provider-aws"
	p1Name := "crossplane-provider-aws"
	c1Existing := "existing-configuration"
	c1 := "crossplane/getting-started-aws:v0.0.1"
	c1Repo := "crossplane/getting-started-aws"
	c1Name := "crossplane-getting-started-aws"
	type args struct {
		p    []string
		c    []string
		kube client.Client
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"SuccessAlreadyExistsSameVersion": {
			args: args{
				p: []string{p1},
				c: []string{c1},
				kube: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						switch l := list.(type) {
						case *v1.ProviderList:
							*l = v1.ProviderList{
								Items: []v1.Provider{
									{
										ObjectMeta: metav1.ObjectMeta{
											Name: p1Name,
										},
										Spec: v1.ProviderSpec{
											PackageSpec: v1.PackageSpec{
												Package: p1,
											},
										},
									},
								},
							}
						case *v1.ConfigurationList:
							*l = v1.ConfigurationList{
								Items: []v1.Configuration{
									{
										ObjectMeta: metav1.ObjectMeta{
											Name: c1Name,
										},
										Spec: v1.ConfigurationSpec{
											PackageSpec: v1.PackageSpec{
												Package: c1,
											},
										},
									},
								},
							}
						default:
							t.Errorf("unexpected type")
						}
						return nil
					},
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj.(type) {
						case *v1.Provider:
							if key.Name != p1Name {
								t.Errorf(errFmtGetProvider, key.Name)
							}
						case *v1.Configuration:
							if key.Name != c1Name {
								t.Errorf(errFmtGetConfiguration, key.Name)
							}
						default:
							t.Errorf("unexpected type")
						}
						return nil
					},
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						switch obj.(type) {
						case *v1.Provider:
							if obj.GetName() != p1Name {
								t.Errorf(errFmtPatchProvider, obj.GetName())
							}
						case *v1.Configuration:
							if obj.GetName() != c1Name {
								t.Errorf(errFmtPatchConfiguration, obj.GetName())
							}
						default:
							t.Errorf("unexpected type")
						}
						return nil
					},
				},
			},
		},
		"SuccessAlreadyExistsDifferentNameDifferentVersion": {
			args: args{
				p: []string{p1},
				c: []string{c1},
				kube: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						switch l := list.(type) {
						case *v1.ProviderList:
							*l = v1.ProviderList{
								Items: []v1.Provider{
									{
										ObjectMeta: metav1.ObjectMeta{
											Name: p1Existing,
										},
										Spec: v1.ProviderSpec{
											PackageSpec: v1.PackageSpec{
												Package: fmt.Sprintf("%s:%s", p1Repo, "v100.100.100"),
											},
										},
									},
								},
							}
						case *v1.ConfigurationList:
							*l = v1.ConfigurationList{
								Items: []v1.Configuration{
									{
										ObjectMeta: metav1.ObjectMeta{
											Name: c1Existing,
										},
										Spec: v1.ConfigurationSpec{
											PackageSpec: v1.PackageSpec{
												Package: fmt.Sprintf("%s:%s", c1Repo, "v100.100.100"),
											},
										},
									},
								},
							}
						default:
							t.Errorf("unexpected type")
						}
						return nil
					},
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj.(type) {
						case *v1.Provider:
							if key.Name != p1Existing {
								t.Errorf(errFmtGetProvider, key.Name)
							}
						case *v1.Configuration:
							if key.Name != c1Existing {
								t.Errorf(errFmtGetConfiguration, key.Name)
							}
						default:
							t.Errorf("unexpected type")
						}
						return nil
					},
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						switch o := obj.(type) {
						case *v1.Provider:
							if o.GetName() != p1Existing {
								t.Errorf(errFmtPatchProvider, o.GetName())
							}
							if o.GetSource() != p1 {
								t.Errorf(errFmtPatchProviderSource, o.GetSource())
							}
						case *v1.Configuration:
							if o.GetName() != c1Existing {
								t.Errorf(errFmtPatchConfiguration, o.GetName())
							}
							if o.GetSource() != c1 {
								t.Errorf(errFmtPatchConfigurationSource, o.GetSource())
							}
						default:
							t.Errorf("unexpected type")
						}
						return nil
					},
				},
			},
		},
		"SuccessCreateNoneExist": {
			args: args{
				p: []string{p1},
				c: []string{c1},
				kube: &test.MockClient{
					MockList: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
						return nil
					},
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj.(type) {
						case *v1.Provider:
							if key.Name != p1Name {
								t.Errorf(errFmtGetProvider, key.Name)
							}
						case *v1.Configuration:
							if key.Name != c1Name {
								t.Errorf(errFmtGetConfiguration, key.Name)
							}
						default:
							t.Errorf("unexpected type")
						}
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockCreate: func(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
						return nil
					},
				},
			},
		},
		"SuccessCreateSomeExist": {
			args: args{
				p: []string{p1},
				c: []string{c1},
				kube: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						switch l := list.(type) {
						case *v1.ProviderList:
							*l = v1.ProviderList{
								Items: []v1.Provider{
									{
										ObjectMeta: metav1.ObjectMeta{
											Name: "other-package",
										},
										Spec: v1.ProviderSpec{
											PackageSpec: v1.PackageSpec{
												Package: fmt.Sprintf("%s:%s", "other-repo", "v100.100.100"),
											},
										},
									},
									{
										ObjectMeta: metav1.ObjectMeta{
											Name: "another-package",
										},
										Spec: v1.ProviderSpec{
											PackageSpec: v1.PackageSpec{
												Package: "preloaded-source",
											},
										},
									},
								},
							}
						case *v1.ConfigurationList:
							return nil
						default:
							t.Errorf("unexpected type")
						}
						return nil
					},
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj.(type) {
						case *v1.Provider:
							if key.Name != p1Name {
								t.Errorf(errFmtGetProvider, key.Name)
							}
						case *v1.Configuration:
							if key.Name != c1Name {
								t.Errorf(errFmtGetConfiguration, key.Name)
							}
						default:
							t.Errorf("unexpected type")
						}
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockCreate: func(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
						return nil
					},
				},
			},
		},
		"SuccessOneConfiguration": {
			// NOTE(hasheddan): test case added due to
			// https://github.com/crossplane/crossplane/issues/2635
			args: args{
				c: []string{c1},
				kube: &test.MockClient{
					MockList: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
						return nil
					},
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj.(type) {
						case *v1.Provider:
							t.Errorf("no providers specified")
						case *v1.Configuration:
							if key.Name != c1Name {
								t.Errorf("unexpected name in configuration apply")
							}
						default:
							t.Errorf("unexpected type")
						}
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockCreate: func(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
						return nil
					},
				},
			},
		},
		"FailApply": {
			args: args{
				p: []string{p1},
				c: []string{c1},
				kube: &test.MockClient{
					MockList: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
						return nil
					},
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return errBoom
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, "cannot get object"), errApplyPackage),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			i := NewPackageInstaller(tc.args.p, tc.args.c)
			err := i.Run(context.TODO(), tc.args.kube)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", name, diff)
			}
		})
	}
}
