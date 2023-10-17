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
	"encoding/json"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

// withTokenFile sets the filesystem with a creds file.
func withTokenFile(b []byte) TokenOption {
	return func(conf *tokenConf) {
		fs := afero.NewMemMapFs()
		f, _ := fs.Create("creds.json")
		f.Write(b)
		conf.fs = fs
	}
}

func TestTokenFromPath(t *testing.T) {
	tf := TokenFile{
		AccessID: "cool-access",
		Token:    "cool-token",
	}
	validTF, err := json.Marshal(tf)
	if err != nil {
		t.Fatalf("Failed to marshal token file: %s", err)
	}
	type args struct {
		path string
		opts []TokenOption
	}
	type want struct {
		tf  TokenFile
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NotExist": {
			reason: "We should return error if token file does not exist.",
			args: args{
				path: "creds.json",
			},
			want: want{
				err: &os.PathError{Op: "open", Path: "creds.json", Err: errors.New("no such file or directory")},
			},
		},
		"InvalidTokenFile": {
			reason: "We should return error if token file is invalid.",
			args: args{
				path: "creds.json",
				opts: []TokenOption{
					withTokenFile([]byte("{}")),
				},
			},
			want: want{
				err: errors.New(errInvalidTokenFile),
			},
		},
		"ValidTokenFile": {
			reason: "We should return token information if exists and is valid.",
			args: args{
				path: "creds.json",
				opts: []TokenOption{
					withTokenFile(validTF),
				},
			},
			want: want{
				tf: tf,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			token, err := TokenFromPath(tc.args.path, tc.args.opts...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nTokenFromPath(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.tf, token); diff != "" {
				t.Errorf("\n%s\nTokenFromPath(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
