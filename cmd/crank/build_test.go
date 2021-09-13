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

package main

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/internal/xpkg"
)

func TestBuild(t *testing.T) {
	type args struct {
		child  *buildChild
		root   string
		ignore []string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"SuccessfulWithName": {
			reason: "",
			args: args{
				child: &buildChild{
					name:   "test",
					linter: parser.NewPackageLinter(nil, nil, nil),
					fs:     afero.NewMemMapFs(),
				},
				root: "/",
			},
		},
		"ErrNoNameNoMeta": {
			reason: "",
			args: args{
				child: &buildChild{
					linter: parser.NewPackageLinter(nil, nil, nil),
					fs:     afero.NewMemMapFs(),
				},
				root: "/",
			},
			want: errors.Wrap(&os.PathError{Op: "open", Path: "/crossplane.yaml", Err: os.ErrNotExist}, errGetNameFromMeta),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			b := buildCmd{
				PackageRoot: tc.args.root,
				Ignore:      tc.args.ignore,
			}
			err := b.Run(tc.args.child, logging.NewNopLogger())

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			// If we didn't encounter an error there should be a package in root.
			if tc.want == nil {
				if _, err := xpkg.FindXpkgInDir(tc.args.child.fs, tc.args.root); err != nil {
					t.Error(err)
				}
			}
		})
	}
}
