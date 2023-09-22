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

package image

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

func TestResolveTag(t *testing.T) {

	type args struct {
		dep     v1beta1.Dependency
		fetcher Fetcher
	}

	type want struct {
		tag string
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessTagFound": {
			reason: "Should return tag.",
			args: args{
				dep: v1beta1.Dependency{
					Package:     "crossplane/provider-aws",
					Constraints: ">=v0.1.1",
				},
				fetcher: NewMockFetcher(
					WithTags(
						[]string{
							"v0.2.0",
							"alpha",
						},
					),
				),
			},
			want: want{
				tag: "v0.2.0",
			},
		},
		"SuccessNoVersionSupplied": {
			reason: "Should return tag.",
			args: args{
				dep: v1beta1.Dependency{
					Package:     "crossplane/provider-aws",
					Constraints: "",
				},
				fetcher: NewMockFetcher(
					WithTags(
						[]string{
							"v0.2.0",
							"alpha",
						},
					),
				),
			},
			want: want{
				tag: "v0.2.0",
			},
		},
		"ErrorInvalidTag": {
			reason: "Should return an error if dep has invalid constraint.",
			args: args{
				dep: v1beta1.Dependency{
					Package:     "crossplane/provider-aws",
					Constraints: "alpha",
				},
				fetcher: NewMockFetcher(
					WithError(
						errors.New(errInvalidConstraint),
					),
				),
			},
			want: want{
				err: errors.Wrap(errors.New("improper constraint: alpha"), errInvalidConstraint),
			},
		},
		"ErrorInvalidReference": {
			reason: "Should return an error if dep has invalid provider.",
			args: args{
				dep: v1beta1.Dependency{
					Package:     "",
					Constraints: "v1.0.0",
				},
				fetcher: NewMockFetcher(
					WithError(
						errors.New(errInvalidProviderRef),
					),
				),
			},
			want: want{
				err: errors.Wrap(errors.New("could not parse reference: "), errInvalidProviderRef),
			},
		},
		"ErrorFailedToFetchTags": {
			reason: "Should return an error if we could not fetch tags.",
			args: args{
				dep: v1beta1.Dependency{
					Package:     "crossplane/provider-aws",
					Constraints: ">=v1.0.0",
				},
				fetcher: NewMockFetcher(
					WithError(
						errors.New(errFailedToFetchTags),
					),
				),
			},
			want: want{
				err: errors.Wrap(errors.New(errFailedToFetchTags), errFailedToFetchTags),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			r := NewResolver(WithFetcher(tc.args.fetcher))

			got, err := r.ResolveTag(context.Background(), tc.args.dep)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUpsertDeps(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.tag, got); diff != "" {
				t.Errorf("\n%s\nUpsertDeps(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
