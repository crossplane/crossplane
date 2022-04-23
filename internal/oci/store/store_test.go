/*
Copyright 2022 The Crossplane Authors.

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

// Package store implements OCI container storage.
package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	ociv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

type MockImage struct {
	ociv1.Image

	MockDigest        func() (ociv1.Hash, error)
	MockRawConfigFile func() ([]byte, error)
}

func (i *MockImage) Digest() (ociv1.Hash, error)    { return i.MockDigest() }
func (i *MockImage) RawConfigFile() ([]byte, error) { return i.MockRawConfigFile() }

func TestReadConfigFile(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		i ociv1.Image
	}
	type want struct {
		cfg *ociv1.ConfigFile
		err error
	}

	cases := map[string]struct {
		reason string
		files  map[string][]byte
		args   args
		want   want
	}{
		"DigestError": {
			reason: "We should return an error if we can't get the image's digest.",
			args: args{
				i: &MockImage{
					MockDigest: func() (ociv1.Hash, error) { return ociv1.Hash{}, errBoom },
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetDigest),
			},
		},
		"UnparseableFileError": {
			reason: "We should return an error if we can't parse a cached config file.",
			files: map[string][]byte{
				filepath.Join("cool", FileConfig): []byte("wat"),
			},
			args: args{
				i: &MockImage{
					MockDigest: func() (ociv1.Hash, error) { return ociv1.Hash{Hex: "cool"}, nil },
				},
			},
			want: want{
				err: errors.Wrap(errors.New("invalid character 'w' looking for beginning of value"), errParseConfigFile),
			},
		},
		"SuccessfulCacheRead": {
			reason: "We should return a ConfigFile read successfully from our cache.",
			files: map[string][]byte{
				filepath.Join("cool", FileConfig): []byte(`{"variant":"cool"}`),
			},
			args: args{
				i: &MockImage{
					MockDigest: func() (ociv1.Hash, error) { return ociv1.Hash{Hex: "cool"}, nil },
				},
			},
			want: want{
				cfg: &ociv1.ConfigFile{Variant: "cool"},
			},
		},
		"RawConfigFileError": {
			reason: "We should return an error if we can't access the image's raw config file.",
			args: args{
				i: &MockImage{
					MockDigest:        func() (ociv1.Hash, error) { return ociv1.Hash{Hex: "cool"}, nil },
					MockRawConfigFile: func() ([]byte, error) { return nil, errBoom },
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetRawConfigFile),
			},
		},
		"SuccessfulCacheWrite": {
			reason: "We should return a ConfigFile successfully written to, then read from our cache.",
			args: args{
				i: &MockImage{
					MockDigest:        func() (ociv1.Hash, error) { return ociv1.Hash{Hex: "cool"}, nil },
					MockRawConfigFile: func() ([]byte, error) { return []byte(`{"variant":"cool"}`), nil },
				},
			},
			want: want{
				cfg: &ociv1.ConfigFile{Variant: "cool"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tmp, err := os.MkdirTemp(os.TempDir(), strings.ReplaceAll(t.Name(), string(os.PathSeparator), "_"))
			if err != nil {
				t.Fatal(err.Error())
			}
			defer os.RemoveAll(tmp)

			for name, data := range tc.files {
				path := filepath.Join(tmp, name)
				_ = os.MkdirAll(filepath.Dir(path), 0700)
				_ = os.WriteFile(path, data, 0600)
			}

			// This will call MkdirAll on the tmp dir, which should succeed
			// because it already exists.
			c, err := NewCachingImageConfigReader(tmp)
			if err != nil {
				t.Fatal(err.Error())
			}

			got, err := c.ReadConfigFile(tc.args.i)

			if diff := cmp.Diff(tc.want.cfg, got); diff != "" {
				t.Errorf("\n%s\nReadConfigFile(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nReadConfigFile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
