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

package xpkg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/internal/input"
	"github.com/crossplane/crossplane/internal/xpkg/v2"
)

func TestInputYes(t *testing.T) {

	type args struct {
		input string
	}

	type want struct {
		output bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Yes": {
			reason: "We should see true for 'Yes'",
			args: args{
				input: "Yes",
			},
			want: want{
				output: true,
			},
		},
		"InputNo": {
			reason: "We should see false for 'No'",
			args: args{
				input: "No",
			},
			want: want{
				output: false,
			},
		},
		"InputUmm": {
			reason: "We should see false for 'umm'",
			args: args{
				input: "umm",
			},
			want: want{
				output: false,
			},
		},
		"InputEmpty": {
			reason: "We should see false for ''",
			args: args{
				input: "",
			},
			want: want{
				output: false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			y := input.Yes(tc.args.input)

			if diff := cmp.Diff(tc.want.output, y); diff != "" {
				t.Errorf("\n%s\nYes(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMetaFileInRoot(t *testing.T) {

	type args struct {
		metaExists bool
		fs         afero.Fs
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MetaFileNotInRoot": {
			args: args{
				metaExists: false,
				fs:         afero.NewMemMapFs(),
			},
			want: want{
				err: nil,
			},
		},
		"MetaFileInRoot": {
			args: args{
				metaExists: true,
				fs:         afero.NewMemMapFs(),
			},
			want: want{
				err: errors.Errorf(errAlreadyExistsFmt, xpkg.MetaFile),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			c := initCmd{
				fs:   tc.args.fs,
				root: "/tmp",
			}

			if tc.args.metaExists {
				err := afero.WriteFile(tc.args.fs, filepath.Join(c.root, xpkg.MetaFile), []byte{}, os.ModePerm)
				if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
					t.Errorf("\n%s\nMetaFileInRoot(...): -want, +got:\n%s", tc.reason, diff)
				}
			}

			got := c.metaFileInRoot()

			if diff := cmp.Diff(tc.want.err, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nMetaFileInRoot(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
