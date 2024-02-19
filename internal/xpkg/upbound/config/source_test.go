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
	"encoding/json"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	_ Source = &FSSource{}
	_ Source = &MockSource{}
)

// TODO(hasheddan): a mock afero.Fs could increase test coverage here with
// simulated failed file opens and writes.

func TestInitialize(t *testing.T) {
	cases := map[string]struct {
		reason    string
		modifiers []FSSourceModifier
		want      error
	}{
		"SuccessNotSuppliedNotExists": {
			reason: "If path is not supplied use default and create if not exist.",
			modifiers: []FSSourceModifier{
				func(f *FSSource) {
					f.fs = afero.NewMemMapFs()
				},
			},
		},
		"SuccessSuppliedNotExists": {
			reason: "If path is supplied but doesn't already exist create it.",
			modifiers: []FSSourceModifier{
				func(f *FSSource) {
					f.path = "/.up/config.json"
					f.fs = afero.NewMemMapFs()
				},
			},
		},
		"SuccessSuppliedExists": {
			reason: "If path is supplied and already exists do not return error.",
			modifiers: []FSSourceModifier{
				func(f *FSSource) {
					f.path = "/.up/config.json"
					fs := afero.NewMemMapFs()
					file, _ := fs.Create("/.up/config.json")
					defer file.Close()
					f.fs = fs
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src := NewFSSource(tc.modifiers...)
			err := src.Initialize()
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nInitialize(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetConfig(t *testing.T) {
	testConf := &Config{
		Upbound: Upbound{
			Default: "test",
		},
	}
	cases := map[string]struct {
		reason    string
		modifiers []FSSourceModifier
		want      *Config
		err       error
	}{
		"SuccessfulEmptyConfig": {
			reason: "An empty file should return an empty config.",
			modifiers: []FSSourceModifier{
				func(f *FSSource) {
					f.fs = afero.NewMemMapFs()
				},
			},
			want: &Config{},
		},
		"Successful": {
			reason: "If we are able to get config we should it.",
			modifiers: []FSSourceModifier{
				func(f *FSSource) {
					f.path = "/.up/config.json"
					fs := afero.NewMemMapFs()
					file, _ := fs.OpenFile("/.up/config.json", os.O_CREATE, 0o600)
					defer file.Close()
					b, _ := json.Marshal(testConf) //nolint:errchkjson // marshalling should not fail
					_, _ = file.Write(b)
					f.fs = fs
				},
			},
			want: testConf,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src := NewFSSource(tc.modifiers...)
			conf, err := src.GetConfig()
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetConfig(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, conf); diff != "" {
				t.Errorf("\n%s\nGetConfig(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestUpdateConfig(t *testing.T) {
	testConf := &Config{
		Upbound: Upbound{
			Default: "test",
		},
	}
	cases := map[string]struct {
		reason    string
		modifiers []FSSourceModifier
		conf      *Config
		err       error
	}{
		"EmptyConfig": {
			reason: "Updating with empty config should not cause an error.",
			modifiers: []FSSourceModifier{
				func(f *FSSource) {
					f.fs = afero.NewMemMapFs()
				},
			},
		},
		"PopulatedConfig": {
			reason: "Updating with populated config should not cause an error.",
			modifiers: []FSSourceModifier{
				func(f *FSSource) {
					f.fs = afero.NewMemMapFs()
				},
			},
			conf: testConf,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src := NewFSSource(tc.modifiers...)
			err := src.UpdateConfig(tc.conf)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUpdateConfig(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
