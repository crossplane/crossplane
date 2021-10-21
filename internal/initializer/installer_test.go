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
	"testing"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errGetProviderFmt              = "unexpected name in provider get: %s"
	errPatchProviderFmt            = "unexpected name in provider update: %s"
	errPatchProviderSourceFmt      = "unexpected source in provider update: %s"
	errGetConfigurationFmt         = "unexpected name in configuration get: %s"
	errPatchConfigurationFmt       = "unexpected name in configuration update: %s"
	errPatchConfigurationSourceFmt = "unexpected source in configuration update: %s"
)

var errBoom = errors.New("boom")

func TestInstaller(t *testing.T) {
	p1Existing := "existing-provider"
	p1 := "crossplane/provider-aws:v1.16.0"
	p1Version := "v1.16.0"
	p1Repo := "crossplane/provider-aws"
	p1Name := "crossplane-provider-aws"
	c1Existing := "existing-configuration"
	c1 := "crossplane/getting-started-aws:v0.0.1"
	c1Version := "v0.0.1"
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
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1beta1.Lock:
							*o = v1beta1.Lock{
								ObjectMeta: metav1.ObjectMeta{
									Name: "lock",
								},
								Packages: []v1beta1.LockPackage{
									{
										Name:    p1Name,
										Type:    v1beta1.ProviderPackageType,
										Source:  p1Repo,
										Version: p1Version,
									},
									{
										Name:    c1Name,
										Type:    v1beta1.ConfigurationPackageType,
										Source:  c1Repo,
										Version: c1Version,
									},
								},
							}
						case *v1.Provider:
							if key.Name != p1Name {
								t.Errorf(errGetProviderFmt, key.Name)
							}
						case *v1.Configuration:
							if key.Name != c1Name {
								t.Errorf(errGetConfigurationFmt, key.Name)
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
								t.Errorf(errPatchProviderFmt, obj.GetName())
							}
						case *v1.Configuration:
							if obj.GetName() != c1Name {
								t.Errorf(errPatchConfigurationFmt, obj.GetName())
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
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1beta1.Lock:
							*o = v1beta1.Lock{
								ObjectMeta: metav1.ObjectMeta{
									Name: "lock",
								},
								Packages: []v1beta1.LockPackage{
									{
										Name:    p1Existing,
										Type:    v1beta1.ProviderPackageType,
										Source:  p1Repo,
										Version: "v0.0.0",
									},
									{
										Name:    c1Existing,
										Type:    v1beta1.ConfigurationPackageType,
										Source:  c1Repo,
										Version: "v0.0.0",
									},
								},
							}
						case *v1.Provider:
							if key.Name != p1Existing {
								t.Errorf(errGetProviderFmt, key.Name)
							}
						case *v1.Configuration:
							if key.Name != c1Existing {
								t.Errorf(errGetConfigurationFmt, key.Name)
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
								t.Errorf(errPatchProviderFmt, o.GetName())
							}
							if o.GetSource() != p1 {
								t.Errorf(errPatchProviderSourceFmt, o.GetSource())
							}
						case *v1.Configuration:
							if o.GetName() != c1Existing {
								t.Errorf(errPatchConfigurationFmt, o.GetName())
							}
							if o.GetSource() != c1 {
								t.Errorf(errPatchConfigurationSourceFmt, o.GetSource())
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
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj.(type) {
						case *v1beta1.Lock:
							return nil
						case *v1.Provider:
							if key.Name != p1Name {
								t.Errorf(errGetProviderFmt, key.Name)
							}
						case *v1.Configuration:
							if key.Name != c1Name {
								t.Errorf(errGetConfigurationFmt, key.Name)
							}
						default:
							t.Errorf("unexpected type")
						}
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockCreate: func(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
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
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1beta1.Lock:
							*o = v1beta1.Lock{
								ObjectMeta: metav1.ObjectMeta{
									Name: "lock",
								},
								Packages: []v1beta1.LockPackage{
									{
										Name:    p1Existing,
										Type:    v1beta1.ProviderPackageType,
										Source:  "some/source",
										Version: "v0.0.0",
									},
									{
										Name:    c1Existing,
										Type:    v1beta1.ConfigurationPackageType,
										Source:  "some/othersource",
										Version: "v0.0.0",
									},
								},
							}
						case *v1.Provider:
							if key.Name != p1Name {
								t.Errorf(errGetProviderFmt, key.Name)
							}
						case *v1.Configuration:
							if key.Name != c1Name {
								t.Errorf(errGetConfigurationFmt, key.Name)
							}
						default:
							t.Errorf("unexpected type")
						}
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockCreate: func(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
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
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj.(type) {
						case *v1beta1.Lock:
							return nil
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
					MockCreate: func(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
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
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if _, ok := obj.(*v1beta1.Lock); ok {
							return nil
						}
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
