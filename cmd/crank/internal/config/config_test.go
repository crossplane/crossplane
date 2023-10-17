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

package config

import (
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestAddOrUpdateUpboundProfile(t *testing.T) {
	name := "cool-profile"
	profOne := Profile{
		ID:      "cool-user",
		Type:    UserProfileType,
		Account: "cool-org",
	}
	profTwo := Profile{
		ID:      "cool-user",
		Type:    UserProfileType,
		Account: "other-org",
	}

	cases := map[string]struct {
		reason string
		name   string
		cfg    *Config
		add    Profile
		want   *Config
		err    error
	}{
		"AddNewProfile": {
			reason: "Adding a new profile to an empty Config should not cause an error.",
			name:   name,
			cfg:    &Config{},
			add:    profOne,
			want: &Config{
				Upbound: Upbound{
					Profiles: map[string]Profile{name: profOne},
				},
			},
		},
		"UpdateExistingProfile": {
			reason: "Updating an existing profile in the Config should not cause an error.",
			name:   name,
			cfg: &Config{
				Upbound: Upbound{
					Profiles: map[string]Profile{name: profOne},
				},
			},
			add: profTwo,
			want: &Config{
				Upbound: Upbound{
					Profiles: map[string]Profile{name: profTwo},
				},
			},
		},
		"Invalid": {
			reason: "Adding an invalid profile should cause an error.",
			name:   name,
			cfg:    &Config{},
			add:    Profile{},
			want:   &Config{},
			err:    errors.New(errInvalidProfile),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.cfg.AddOrUpdateUpboundProfile(tc.name, tc.add)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nAddOrUpdateUpboundProfile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, tc.cfg); diff != "" {
				t.Errorf("\n%s\nAddOrUpdateUpboundProfile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetDefaultUpboundProfile(t *testing.T) {
	name := "cool-profile"
	profOne := Profile{
		ID:      "cool-user",
		Type:    UserProfileType,
		Account: "cool-org",
	}

	cases := map[string]struct {
		reason string
		name   string
		cfg    *Config
		want   Profile
		err    error
	}{
		"ErrorNoDefault": {
			reason: "If no default defined an error should be returned.",
			cfg:    &Config{},
			want:   Profile{},
			err:    errors.New(errNoDefaultSpecified),
		},
		"ErrorDefaultNotExist": {
			reason: "If defined default does not exist an error should be returned.",
			cfg: &Config{
				Upbound: Upbound{
					Default: "test",
				},
			},
			want: Profile{},
			err:  errors.New(errDefaultNotExist),
		},
		"Successful": {
			reason: "If defined default exists it should be returned.",
			name:   name,
			cfg: &Config{
				Upbound: Upbound{
					Default:  "cool-profile",
					Profiles: map[string]Profile{name: profOne},
				},
			},
			want: profOne,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			name, prof, err := tc.cfg.GetDefaultUpboundProfile()
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetDefaultUpboundProfile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.name, name); diff != "" {
				t.Errorf("\n%s\nGetDefaultUpboundProfile(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, prof); diff != "" {
				t.Errorf("\n%s\nGetDefaultUpboundProfile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetUpboundProfile(t *testing.T) {
	name := "cool-profile"
	profOne := Profile{
		ID:      "cool-user",
		Type:    UserProfileType,
		Account: "cool-org",
	}

	cases := map[string]struct {
		reason string
		name   string
		cfg    *Config
		want   Profile
		err    error
	}{
		"ErrorProfileNotExist": {
			reason: "If profile does not exist an error should be returned.",
			name:   name,
			cfg:    &Config{},
			want:   Profile{},
			err:    errors.Errorf(errProfileNotFoundFmt, "cool-profile"),
		},
		"Successful": {
			reason: "If profile exists it should be returned.",
			name:   "cool-profile",
			cfg: &Config{
				Upbound: Upbound{
					Profiles: map[string]Profile{name: profOne},
				},
			},
			want: profOne,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			prof, err := tc.cfg.GetUpboundProfile(tc.name)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetUpboundProfile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, prof); diff != "" {
				t.Errorf("\n%s\nGetUpboundProfile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSetDefaultUpboundProfile(t *testing.T) {
	name := "cool-user"
	profOne := Profile{
		Type:    UserProfileType,
		Account: "cool-org",
	}

	cases := map[string]struct {
		reason string
		name   string
		cfg    *Config
		err    error
	}{
		"ErrorProfileNotExist": {
			reason: "If profile does not exist an error should be returned.",
			name:   name,
			cfg:    &Config{},
			err:    errors.Errorf(errProfileNotFoundFmt, "cool-user"),
		},
		"Successful": {
			reason: "If profile exists it should be set as default.",
			name:   "cool-user",
			cfg: &Config{
				Upbound: Upbound{
					Profiles: map[string]Profile{name: profOne},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.cfg.SetDefaultUpboundProfile(tc.name)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetUpboundProfile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetUpboundProfiles(t *testing.T) {
	nameOne := "cool-user"
	profOne := Profile{
		Type:    UserProfileType,
		Account: "cool-org",
	}
	nameTwo := "cool-user2"
	profTwo := Profile{
		Type:    UserProfileType,
		Account: "cool-org2",
	}

	type args struct {
		cfg *Config
	}
	type want struct {
		err      error
		profiles map[string]Profile
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorNoProfilesExist": {
			reason: "If no profiles exist an error should be returned.",
			args: args{
				cfg: &Config{},
			},
			want: want{
				err: errors.New(errNoProfilesFound),
			},
		},
		"Successful": {
			reason: "If profile exists it should be set as default.",
			args: args{
				cfg: &Config{
					Upbound: Upbound{
						Profiles: map[string]Profile{
							nameOne: profOne,
							nameTwo: profTwo,
						},
					},
				},
			},
			want: want{
				profiles: map[string]Profile{
					nameOne: profOne,
					nameTwo: profTwo,
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			profiles, err := tc.args.cfg.GetUpboundProfiles()

			if diff := cmp.Diff(tc.want.profiles, profiles); diff != "" {
				t.Errorf("\n%s\nGetUpboundProfiles(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetUpboundProfiles(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetBaseConfig(t *testing.T) {
	nameOne := "cool-user"
	profOne := Profile{
		Type:    UserProfileType,
		Account: "cool-org",
		BaseConfig: map[string]string{
			"key": "value",
		},
	}
	nameTwo := "cool-user2"
	profTwo := Profile{
		Type:    UserProfileType,
		Account: "cool-org2",
	}

	type args struct {
		profile string
		cfg     *Config
	}
	type want struct {
		err  error
		base map[string]string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorNoProfilesExist": {
			reason: "If no profiles exist an error should be returned.",
			args: args{
				profile: nameTwo,
				cfg:     &Config{},
			},
			want: want{
				err: errors.Errorf(errProfileNotFoundFmt, nameTwo),
			},
		},
		"Successful": {
			reason: "If profile exists, its base config should be returned.",
			args: args{
				profile: nameOne,
				cfg: &Config{
					Upbound: Upbound{
						Profiles: map[string]Profile{
							nameOne: profOne,
							nameTwo: profTwo,
						},
					},
				},
			},
			want: want{
				base: profOne.BaseConfig,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			base, err := tc.args.cfg.GetBaseConfig(tc.args.profile)

			if diff := cmp.Diff(tc.want.base, base); diff != "" {
				t.Errorf("\n%s\nGetBaseConfig(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetBaseConfig(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAddToBaseConfig(t *testing.T) {
	nameOne := "cool-user"
	profOne := Profile{
		Type:    UserProfileType,
		Account: "cool-org",
	}
	nameTwo := "cool-user2"
	profTwo := Profile{
		Type:    UserProfileType,
		Account: "cool-org2",
	}

	type args struct {
		profile string
		key     string
		value   string
		cfg     *Config
	}
	type want struct {
		err  error
		base map[string]string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorNoProfilesExist": {
			reason: "If no profiles exist an error should be returned.",
			args: args{
				profile: nameTwo,
				cfg:     &Config{},
			},
			want: want{
				err: errors.Errorf(errProfileNotFoundFmt, nameTwo),
			},
		},
		"Successful": {
			reason: "If profile exists, we should add the k,v pair to the base config.",
			args: args{
				profile: nameOne,
				key:     "k",
				value:   "v",
				cfg: &Config{
					Upbound: Upbound{
						Profiles: map[string]Profile{
							nameOne: profOne,
							nameTwo: profTwo,
						},
					},
				},
			},
			want: want{
				base: map[string]string{
					"k": "v",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.cfg.AddToBaseConfig(tc.args.profile, tc.args.key, tc.args.value)
			base, _ := tc.args.cfg.GetBaseConfig(tc.args.profile)

			if diff := cmp.Diff(tc.want.base, base); diff != "" {
				t.Errorf("\n%s\nAddToBaseConfig(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nAddToBaseConfig(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestBaseToJSON(t *testing.T) {
	dneName := "does not exist"
	exists := "exists"

	type args struct {
		profile string
		cfg     *Config
	}
	type want struct {
		err  error
		base string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorNoProfilesExist": {
			reason: "If no profiles exist an error should be returned.",
			args: args{
				profile: dneName,
				cfg:     &Config{},
			},
			want: want{
				err: errors.Errorf(errProfileNotFoundFmt, dneName),
			},
		},
		"Successful": {
			reason: "If profile exists, we should add the k,v pair to the base config.",
			args: args{
				profile: exists,
				cfg: &Config{
					Upbound: Upbound{
						Profiles: map[string]Profile{
							exists: {
								Type:    UserProfileType,
								Account: "account",
								BaseConfig: map[string]string{
									"k": "v",
								},
							},
						},
					},
				},
			},
			want: want{
				base: "{\"k\":\"v\"}\n",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r, err := tc.args.cfg.BaseToJSON(tc.args.profile)
			if r != nil {
				base, _ := io.ReadAll(r)
				if diff := cmp.Diff(tc.want.base, string(base)); diff != "" {
					t.Errorf("\n%s\nBaseToJSON(...): -want, +got:\n%s", tc.reason, diff)
				}
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBaseToJSON(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
