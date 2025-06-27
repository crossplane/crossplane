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
	conregv1 "github.com/google/go-containerregistry/pkg/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/xpkg"
	"github.com/crossplane/crossplane/internal/xpkg/fake"
)

func TestPackageRevisioner(t *testing.T) {
	errBoom := errors.New("boom")
	pullNever := corev1.PullNever
	pullIfNotPresent := corev1.PullIfNotPresent

	type args struct {
		f                    xpkg.Fetcher
		pkg                  v1.Package
		pullSecretFromConfig string
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
			reason: "Should return friendly identifier if pull policy is Never.",
			args: args{
				pkg: &v1.Provider{
					ObjectMeta: metav1.ObjectMeta{
						Name: "provider-aws",
					},
					Spec: v1.ProviderSpec{
						PackageSpec: v1.PackageSpec{
							Package:           "my-revision",
							PackagePullPolicy: &pullNever,
						},
					},
					Status: v1.ProviderStatus{
						PackageStatus: v1.PackageStatus{
							ResolvedPackage: "my-revision",
						},
					},
				},
			},
			want: want{
				digest: "provider-aws-my-revision",
			},
		},
		"SuccessfulPullIfNotPresentSameSource": {
			reason: "Should return the existing package revision if identifier did not change.",
			args: args{
				pkg: &v1.Provider{
					ObjectMeta: metav1.ObjectMeta{
						Name: "provider-aws",
					},
					Spec: v1.ProviderSpec{
						PackageSpec: v1.PackageSpec{
							Package:           "crossplane/provider-aws:latest",
							PackagePullPolicy: &pullIfNotPresent,
						},
					},
					Status: v1.ProviderStatus{
						PackageStatus: v1.PackageStatus{
							ResolvedPackage:   "crossplane/provider-aws:latest",
							CurrentRevision:   "return-me",
							CurrentIdentifier: "crossplane/provider-aws:latest",
						},
					},
				},
			},
			want: want{
				digest: "return-me",
			},
		},
		"SuccessfulPullRewrittenImage": {
			reason: "Should resolve the image correctly when it has been rewritten by an image config.",
			args: args{
				f: &fake.MockFetcher{
					MockHead: func(ref name.Reference) (*conregv1.Descriptor, error) {
						if ref.String() != "registry.acme.co/crossplane/provider-aws:latest" {
							return nil, errors.Errorf("incorrect ref %q", ref)
						}
						return &conregv1.Descriptor{
							Digest: conregv1.Hash{
								Algorithm: "sha256",
								Hex:       "ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
							},
						}, nil
					},
				},
				pkg: &v1.Provider{
					ObjectMeta: metav1.ObjectMeta{
						Name: "provider-aws",
					},
					Spec: v1.ProviderSpec{
						PackageSpec: v1.PackageSpec{
							Package:           "xpkg.crossplane.io/crossplane/provider-aws:latest",
							PackagePullPolicy: &pullIfNotPresent,
						},
					},
					Status: v1.ProviderStatus{
						PackageStatus: v1.PackageStatus{
							ResolvedPackage: "registry.acme.co/crossplane/provider-aws:latest",
						},
					},
				},
			},
			want: want{
				digest: "provider-aws-ecc25c121431",
			},
		},
		"SuccessfulDigest": {
			reason: "Should return the digest of the package source image.",
			args: args{
				pkg: &v1.Provider{
					ObjectMeta: metav1.ObjectMeta{
						Name: "provider-nop",
					},
					Spec: v1.ProviderSpec{
						PackageSpec: v1.PackageSpec{
							Package:           "crossplane-contrib/provider-nop@sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
							PackagePullPolicy: &pullIfNotPresent,
						},
					},
					Status: v1.ProviderStatus{
						PackageStatus: v1.PackageStatus{
							ResolvedPackage: "crossplane-contrib/provider-nop@sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
						},
					},
				},
				f: &fake.MockFetcher{
					MockHead: fake.NewMockHeadFn(&conregv1.Descriptor{
						Digest: conregv1.Hash{
							Algorithm: "sha256",
							Hex:       "ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
						},
					}, nil),
				},
			},
			want: want{
				digest: "provider-nop-ecc25c121431",
			},
		},
		"ErrParseRef": {
			reason: "Should return an error if we cannot parse reference from package source image.",
			args: args{
				pkg: &v1.Provider{
					Spec: v1.ProviderSpec{
						PackageSpec: v1.PackageSpec{
							Package: "*THISISNOTVALID",
						},
					},
					Status: v1.ProviderStatus{
						PackageStatus: v1.PackageStatus{
							ResolvedPackage: "*THISISNOTVALID",
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.New("could not parse reference: *THISISNOTVALID"), errBadReference),
			},
		},
		"ErrBadFetch": {
			reason: "Should return an error if we fail to fetch package image.",
			args: args{
				f: &fake.MockFetcher{
					MockHead: fake.NewMockHeadFn(nil, errBoom),
				},
				pkg: &v1.Provider{
					Spec: v1.ProviderSpec{
						PackageSpec: v1.PackageSpec{
							Package: "test/test:test",
						},
					},
					Status: v1.ProviderStatus{
						PackageStatus: v1.PackageStatus{
							ResolvedPackage: "test/test:test",
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
			r := NewPackageRevisioner(tc.args.f)
			h, err := r.Revision(context.TODO(), tc.args.pkg, tc.args.pullSecretFromConfig)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Name(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.digest, h, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Name(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
