// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package xpkg

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestFindXpkgInDir(t *testing.T) {
	match := afero.NewMemMapFs()
	_ = afero.WriteFile(match, "one.xpkg", []byte{}, StreamFileMode)

	multi := afero.NewMemMapFs()
	_ = afero.WriteFile(multi, "one.xpkg", []byte{}, StreamFileMode)
	_ = afero.WriteFile(multi, "two.xpkg", []byte{}, StreamFileMode)

	type args struct {
		root string
		fs   afero.Fs
	}

	type want struct {
		path string
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoMatch": {
			reason: "We should return an error if no matches.",
			args: args{
				root: ".",
				fs:   afero.NewMemMapFs(),
			},
			want: want{
				err: errors.New(errNoMatch),
			},
		},
		"MultiMatch": {
			reason: "We should return an error if multiple matches.",
			args: args{
				root: ".",
				fs:   multi,
			},
			want: want{
				err: errors.New(errMultiMatch),
			},
		},
		"NotExist": {
			reason: "We should return an error root does not exist.",
			args: args{
				root: "/test",
				fs:   afero.NewMemMapFs(),
			},
			want: want{
				err: &os.PathError{Op: "open", Path: "/test", Err: os.ErrNotExist},
			},
		},
		"NotDir": {
			reason: "We should return an error if root is not a directory.",
			args: args{
				root: "one.xpkg",
				fs:   multi,
			},
			want: want{
				err: &os.PathError{Op: "readdir", Path: "one.xpkg", Err: errors.New("not a dir")},
			},
		},
		"Successful": {
			reason: "We should return file path if one package exists.",
			args: args{
				root: ".",
				fs:   match,
			},
			want: want{
				path: "one.xpkg",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			path, err := FindXpkgInDir(tc.args.fs, tc.args.root)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFindXpkgInDir(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.path, path); diff != "" {
				t.Errorf("\n%s\nFindXpkgInDir(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
