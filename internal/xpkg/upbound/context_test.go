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

package upbound

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/internal/xpkg/upbound/config"
)

var (
	defaultConfigJSON = `{
		"upbound": {
		  "default": "default",
		  "profiles": {
			"default": {
			  "id": "someone@upbound.io",
			  "type": "user",
			  "session": "a token"
			}
		  }
		}
	  }
	`
	baseConfigJSON = `{
		"upbound": {
		  "default": "default",
		  "profiles": {
			"default": {
			  "id": "someone@upbound.io",
			  "type": "user",
			  "session": "a token",
			  "base": {
				"UP_DOMAIN": "https://local.upbound.io",
				"UP_ACCOUNT": "my-org",
				"UP_INSECURE_SKIP_TLS_VERIFY": "true"
			  }
			},
			"cool-profile": {
				"id": "someone@upbound.io",
				"type": "user",
				"session": "a token",
				"base": {
				  "UP_DOMAIN": "https://local.upbound.io",
				  "UP_ACCOUNT": "my-org",
				  "UP_INSECURE_SKIP_TLS_VERIFY": "true"
				}
			  }
		  }
		}
	  }
	`
)

func withConfig(config string) Option {
	return func(ctx *Context) {
		// establish fs and create config.json
		fs := afero.NewMemMapFs()
		fs.MkdirAll(filepath.Dir("/.up/"), 0o755)
		f, _ := fs.Create("/.up/config.json")

		f.WriteString(config)

		ctx.fs = fs
	}
}

func withFS(fs afero.Fs) Option {
	return func(ctx *Context) {
		ctx.fs = fs
	}
}

func withPath(p string) Option {
	return func(ctx *Context) {
		ctx.cfgPath = p
	}
}

func withURL(uri string) *url.URL {
	u, _ := url.Parse(uri)
	return u
}

func TestNewFromFlags(t *testing.T) {
	type args struct {
		flags []string
		opts  []Option
	}
	type want struct {
		err error
		c   *Context
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoPreExistingProfile": {
			reason: "We should successfully return a Context if a pre-existing profile does not exist.",
			args: args{
				flags: []string{},
				opts: []Option{
					withFS(afero.NewMemMapFs()),
				},
			},
			want: want{
				c: &Context{
					Account:          "",
					APIEndpoint:      withURL("https://api.upbound.io"),
					Cfg:              &config.Config{},
					Domain:           withURL("https://upbound.io"),
					Profile:          config.Profile{},
					RegistryEndpoint: withURL("https://xpkg.upbound.io"),
				},
			},
		},
		"ErrorSuppliedNotExist": {
			reason: "We should return an error if profile is supplied and it does not exist.",
			args: args{
				flags: []string{
					"--profile=not-here",
				},
			},
			want: want{
				err: errors.Errorf(errProfileNotFoundFmt, "not-here"),
			},
		},
		"SuppliedNotExistAllowEmpty": {
			reason: "We should successfully return a Context if a supplied profile does not exist and.",
			args: args{
				flags: []string{
					"--profile=not-here",
				},
				opts: []Option{
					withFS(afero.NewMemMapFs()),
					AllowMissingProfile(),
				},
			},
			want: want{
				c: &Context{
					ProfileName:      "not-here",
					Account:          "",
					APIEndpoint:      withURL("https://api.upbound.io"),
					Cfg:              &config.Config{},
					Domain:           withURL("https://upbound.io"),
					Profile:          config.Profile{},
					RegistryEndpoint: withURL("https://xpkg.upbound.io"),
				},
			},
		},
		"PreExistingProfileNoBaseConfig": {
			reason: "We should successfully return a Context if a pre-existing profile exists, but does not have a base config",
			args: args{
				flags: []string{},
				opts: []Option{
					withConfig(defaultConfigJSON),
					withPath("/.up/config.json"),
				},
			},
			want: want{
				c: &Context{
					ProfileName:           "default",
					Account:               "",
					APIEndpoint:           withURL("https://api.upbound.io"),
					Domain:                withURL("https://upbound.io"),
					InsecureSkipTLSVerify: false,
					Profile: config.Profile{
						ID:      "someone@upbound.io",
						Type:    config.UserProfileType,
						Session: "a token",
						Account: "",
					},
					RegistryEndpoint: withURL("https://xpkg.upbound.io"),
					Token:            "",
				},
			},
		},
		"PreExistingProfileBaseConfigSetProfile": {
			reason: "We should return a Context that includes the persisted Profile from base config",
			args: args{
				flags: []string{},
				opts: []Option{
					withConfig(baseConfigJSON),
					withPath("/.up/config.json"),
				},
			},
			want: want{
				c: &Context{
					ProfileName:           "default",
					Account:               "my-org",
					APIEndpoint:           withURL("https://api.local.upbound.io"),
					Domain:                withURL("https://local.upbound.io"),
					InsecureSkipTLSVerify: true,
					Profile: config.Profile{
						ID:      "someone@upbound.io",
						Type:    config.UserProfileType,
						Session: "a token",
						Account: "",
						BaseConfig: map[string]string{
							"UP_ACCOUNT":                  "my-org",
							"UP_DOMAIN":                   "https://local.upbound.io",
							"UP_INSECURE_SKIP_TLS_VERIFY": "true",
						},
					},
					RegistryEndpoint: withURL("https://xpkg.local.upbound.io"),
					Token:            "",
				},
			},
		},
		"PreExistingBaseConfigOverrideThroughFlags": {
			reason: "We should return a Context that includes the persisted Profile from base config overridden based on flags",
			args: args{
				flags: []string{
					"--profile=cool-profile",
					"--account=not-my-org",
					fmt.Sprintf("--domain=%s", withURL("http://a.domain.org")),
					fmt.Sprintf("--override-api-endpoint=%s", withURL("http://not.a.url")),
				},
				opts: []Option{
					withConfig(baseConfigJSON),
					withPath("/.up/config.json"),
				},
			},
			want: want{
				c: &Context{
					ProfileName:           "cool-profile",
					Account:               "not-my-org",
					APIEndpoint:           withURL("http://not.a.url"),
					Domain:                withURL("http://a.domain.org"),
					InsecureSkipTLSVerify: true,
					Profile: config.Profile{
						ID:      "someone@upbound.io",
						Type:    config.UserProfileType,
						Session: "a token",
						Account: "",
						BaseConfig: map[string]string{
							"UP_ACCOUNT":                  "my-org",
							"UP_DOMAIN":                   "https://local.upbound.io",
							"UP_INSECURE_SKIP_TLS_VERIFY": "true",
						},
					},
					RegistryEndpoint: withURL("http://xpkg.a.domain.org"),
					Token:            "",
				},
			},
		},
	}

	for name, tc := range cases {
		// Unset common UP env vars used by the test to avoid unexpect behaviours describe in #5721
		os.Unsetenv("UP_ACCOUNT")
		os.Unsetenv("UP_DOMAIN")
		os.Unsetenv("UP_PROFILE")
		os.Unsetenv("UP_INSECURE_SKIP_TLS_VERIFY")
		t.Run(name, func(t *testing.T) {
			flags := Flags{}
			parser, _ := kong.New(&flags)
			parser.Parse(tc.args.flags)

			c, err := NewFromFlags(flags, tc.args.opts...)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nNewFromFlags(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.c, c,
				cmpopts.IgnoreUnexported(Context{}),
				// NOTE(tnthornton): we're not concerned about the FSSource's
				// internal components.
				cmpopts.IgnoreFields(Context{}, "CfgSrc"),
				// NOTE(tnthornton) we're not concerned about the Cfg's
				// internal components.
				cmpopts.IgnoreFields(Context{}, "Cfg"),
			); diff != "" {
				t.Errorf("\n%s\nNewFromFlags(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
