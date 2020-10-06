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

package manager

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/pkg/xpkg"
	"github.com/crossplane/crossplane/pkg/xpkg/fake"
)

func TestPackageDigester(t *testing.T) {
	errBoom := errors.New("boom")
	pullNever := corev1.PullNever

	type args struct {
		f   xpkg.Fetcher
		pkg v1alpha1.Package
	}

	type want struct {
		err    error
		digest string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulPullNever": {
			reason: "Should return the package source directly if pull policy is Never.",
			args: args{
				pkg: &v1alpha1.Provider{
					Spec: v1alpha1.ProviderSpec{
						PackageSpec: v1alpha1.PackageSpec{
							Package:           "provider-aws-1234567",
							PackagePullPolicy: &pullNever,
						},
					},
				},
			},
			want: want{
				digest: "provider-aws-1234567",
			},
		},
		"ErrParseRef": {
			reason: "Should return an error if we cannot parse reference from package source image.",
			args: args{
				pkg: &v1alpha1.Provider{
					Spec: v1alpha1.ProviderSpec{
						PackageSpec: v1alpha1.PackageSpec{
							Package: "*THISISNOTVALID",
						},
					},
				},
			},
			want: want{
				err: name.NewErrBadName("could not parse reference: " + "*THISISNOTVALID"),
			},
		},
		"ErrBadFetch": {
			reason: "Should return an error if we fail to fetch package image.",
			args: args{
				f: &fake.MockFetcher{
					MockFetch: fake.NewMockFetchFn(nil, errBoom),
				},
				pkg: &v1alpha1.Provider{
					Spec: v1alpha1.ProviderSpec{
						PackageSpec: v1alpha1.PackageSpec{
							Package: "test/test:test",
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errFetchPackage),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := NewPackageDigester(tc.args.f)
			h, err := d.Digest(context.TODO(), tc.args.pkg)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Check(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.digest, h, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Check(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
