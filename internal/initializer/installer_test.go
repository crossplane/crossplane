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
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errBoom = errors.New("boom")

func TestInstaller(t *testing.T) {
	p1 := "crossplane/provider-aws:v1.16.0"
	c1 := "crossplane/getting-started-aws:v0.0.1"
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
		"SuccessAlreadyExists": {
			args: args{
				p: []string{p1},
				c: []string{c1},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj.(type) {
						case *v1.Provider:
							if key.Name != cleanUpName(p1) {
								t.Errorf("unexpected name in provider get")
							}
						case *v1.Configuration:
							if key.Name != cleanUpName(c1) {
								t.Errorf("unexpected name in configuration get")
							}
						default:
							t.Errorf("unexpected type")
						}
						return nil
					},
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						switch obj.(type) {
						case *v1.Provider:
							if obj.GetName() != cleanUpName(p1) {
								t.Errorf("unexpected name in provider update")
							}
						case *v1.Configuration:
							if obj.GetName() != cleanUpName(c1) {
								t.Errorf("unexpected name in configuration update")
							}
						default:
							t.Errorf("unexpected type")
						}
						return nil
					},
				},
			},
		},
		"SuccessCreate": {
			args: args{
				p: []string{p1},
				c: []string{c1},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj.(type) {
						case *v1.Provider:
							if key.Name != cleanUpName(p1) {
								t.Errorf("unexpected name in provider apply")
							}
						case *v1.Configuration:
							if key.Name != cleanUpName(c1) {
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
