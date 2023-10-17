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

package credhelper

import (
	"testing"

	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/internal/xpkg/upbound/config"
)

// TODO(hasheddan): these tests are testing through to the underlying config
// package more than we would like. We should consider refactoring the config
// package to make it more mockable.

var _ credentials.Helper = &Helper{}

func TestGet(t *testing.T) {
	testServer := "xpkg.upbound.io"
	testProfile := "test"
	testSecret := "supersecretvalue"
	errBoom := errors.New("boom")
	type args struct {
		server string
	}
	type want struct {
		user   string
		secret string
		err    error
	}
	cases := map[string]struct {
		reason string
		args   args
		opts   []Opt
		want   want
	}{
		"ErrorUnsupportedDomain": {
			reason: "Should return error if domain is not supported.",
			args: args{
				server: testServer,
			},
			opts: []Opt{
				WithDomain("registry.upbound.io"),
			},
			want: want{
				err: errors.New(errUnsupportedDomain),
			},
		},
		"ErrorInitializeSource": {
			reason: "Should return error if we fail to initialize source.",
			args: args{
				server: testServer,
			},
			opts: []Opt{
				WithSource(&config.MockSource{
					InitializeFn: func() error {
						return errBoom
					},
				}),
			},
			want: want{
				err: errors.Wrap(errBoom, errInitializeSource),
			},
		},
		"ErrorExtractConfig": {
			reason: "Should return error if we fail to extract config.",
			args: args{
				server: testServer,
			},
			opts: []Opt{
				WithSource(&config.MockSource{
					InitializeFn: func() error {
						return nil
					},
					GetConfigFn: func() (*config.Config, error) {
						return nil, errBoom
					},
				}),
			},
			want: want{
				err: errors.Wrap(errBoom, errExtractConfig),
			},
		},
		"ErrorGetDefault": {
			reason: "If no profile is specified and we fail to get default return error.",
			args: args{
				server: testServer,
			},
			opts: []Opt{
				WithSource(&config.MockSource{
					InitializeFn: func() error {
						return nil
					},
					GetConfigFn: func() (*config.Config, error) {
						return &config.Config{}, nil
					},
				}),
			},
			want: want{
				err: errors.Wrap(errors.New("no default profile specified"), errGetDefaultProfile),
			},
		},
		"ErrorGetProfile": {
			reason: "If we fail to get the specified profile return error.",
			args: args{
				server: testServer,
			},
			opts: []Opt{
				WithProfile(testProfile),
				WithSource(&config.MockSource{
					InitializeFn: func() error {
						return nil
					},
					GetConfigFn: func() (*config.Config, error) {
						return &config.Config{}, nil
					},
				}),
			},
			want: want{
				err: errors.Wrap(errors.Errorf("profile not found with identifier: %s", testProfile), errGetProfile),
			},
		},
		"Success": {
			reason: "If we successfully get profile return credentials.",
			args: args{
				server: testServer,
			},
			opts: []Opt{
				WithProfile(testProfile),
				WithSource(&config.MockSource{
					InitializeFn: func() error {
						return nil
					},
					GetConfigFn: func() (*config.Config, error) {
						return &config.Config{
							Upbound: config.Upbound{
								Profiles: map[string]config.Profile{
									testProfile: {
										Session: testSecret,
									},
								},
							},
						}, nil
					},
				}),
			},
			want: want{
				user:   defaultDockerUser,
				secret: testSecret,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			user, secret, err := New(tc.opts...).Get(tc.args.server)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGet(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.user, user); diff != "" {
				t.Errorf("\n%s\nGet(...): -want user, +got user:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.secret, secret); diff != "" {
				t.Errorf("\n%s\nGet(...): -want secret, +got secret:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	err := New().Add(nil)
	if diff := cmp.Diff(errors.New(errUnimplemented), err, test.EquateErrors()); diff != "" {
		t.Errorf("\nAdd(...): -want error, +got error:\n%s", diff)
	}
}

func TestDelete(t *testing.T) {
	err := New().Delete("")
	if diff := cmp.Diff(errors.New(errUnimplemented), err, test.EquateErrors()); diff != "" {
		t.Errorf("\nDelete(...): -want error, +got error:\n%s", diff)
	}
}

func TestList(t *testing.T) {
	_, err := New().List()
	if diff := cmp.Diff(errors.New(errUnimplemented), err, test.EquateErrors()); diff != "" {
		t.Errorf("\nList(...): -want error, +got error:\n%s", diff)
	}
}
