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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-containerregistry/pkg/name"
	ociv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

type MockImage struct {
	ociv1.Image

	MockDigest        func() (ociv1.Hash, error)
	MockRawConfigFile func() ([]byte, error)
	MockLayers        func() ([]ociv1.Layer, error)
}

func (i *MockImage) Digest() (ociv1.Hash, error)    { return i.MockDigest() }
func (i *MockImage) RawConfigFile() ([]byte, error) { return i.MockRawConfigFile() }
func (i *MockImage) Layers() ([]ociv1.Layer, error) { return i.MockLayers() }

type MockLayer struct {
	ociv1.Layer

	MockDiffID       func() (ociv1.Hash, error)
	MockUncompressed func() (io.ReadCloser, error)
}

func (l *MockLayer) DiffID() (ociv1.Hash, error)          { return l.MockDiffID() }
func (l *MockLayer) Uncompressed() (io.ReadCloser, error) { return l.MockUncompressed() }

func TestHash(t *testing.T) {
	type args struct {
		r name.Reference
	}
	type want struct {
		h   ociv1.Hash
		err error
	}

	cases := map[string]struct {
		reason string
		files  map[string][]byte
		args   args
		want   want
	}{
		"ReadError": {
			reason: "We should return any error encountered reading the stored hash.",
			args: args{
				r: name.MustParseReference("example.org/image"),
			},
			want: want{
				// Note we're matching with cmpopts.EquateErrors, which only
				// cares that the returned error errors.Is() this one.
				err: os.ErrNotExist,
			},
		},
		"ParseError": {
			reason: "We should return any error encountered reading the stored hash.",
			files: map[string][]byte{
				"276640b463239572f62edd97253f05e0de082e9888f57dac0b83d2149efa59e0": []byte("wat"),
			},
			args: args{
				r: name.MustParseReference("example.org/image"),
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"SuccessfulRead": {
			reason: "We should return the stored hash.",
			files: map[string][]byte{
				"276640b463239572f62edd97253f05e0de082e9888f57dac0b83d2149efa59e0": []byte("sha256:c34045c1a1db8d1b3fca8a692198466952daae07eaf6104b4c87ed3b55b6af1b"),
			},
			args: args{
				r: name.MustParseReference("example.org/image"),
			},
			want: want{
				h: ociv1.Hash{
					Algorithm: "sha256",
					Hex:       "c34045c1a1db8d1b3fca8a692198466952daae07eaf6104b4c87ed3b55b6af1b",
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tmp, err := os.MkdirTemp(os.TempDir(), strings.ReplaceAll(t.Name(), string(os.PathSeparator), "_"))
			if err != nil {
				t.Fatal(err.Error())
			}
			t.Cleanup(func() {
				os.RemoveAll(tmp)
			})

			for name, data := range tc.files {
				path := filepath.Join(tmp, DirDigests, "sha256", name)
				_ = os.MkdirAll(filepath.Dir(path), 0700)
				_ = os.WriteFile(path, data, 0600)
			}

			c, err := NewDigest(tmp)
			if err != nil {
				t.Fatal(err)
			}

			h, err := c.Hash(tc.args.r)
			if diff := cmp.Diff(tc.want.h, h); diff != "" {
				t.Errorf("\n%s\nHash(...): -want, +got:\n%s", tc.reason, diff)
			}
			// Note cmpopts.EquateErrors, not the usual testing.EquateErrors
			// from crossplane-runtime. We need this to support cmpopts.AnyError.
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nHash(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWriteImage(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		i ociv1.Image
	}
	type want struct {
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
		"WriteLayerError": {
			reason: "We should return an error if we can't write a layer to the store.",
			args: args{
				i: &MockImage{
					MockDigest:        func() (ociv1.Hash, error) { return ociv1.Hash{Hex: "cool"}, nil },
					MockRawConfigFile: func() ([]byte, error) { return nil, nil },
					MockLayers: func() ([]ociv1.Layer, error) {
						return []ociv1.Layer{
							&MockLayer{
								// To cause WriteLayer to fail.
								MockDiffID: func() (ociv1.Hash, error) { return ociv1.Hash{}, errBoom },
							},
						}, nil
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errGetDigest), errWriteLayers),
			},
		},
		"SuccessfulWrite": {
			reason: "We should not return an error if we successfully wrote an image to the store.",
			args: args{
				i: &MockImage{
					MockDigest:        func() (ociv1.Hash, error) { return ociv1.Hash{Hex: "cool"}, nil },
					MockRawConfigFile: func() ([]byte, error) { return []byte(`{"variant":"cool"}`), nil },
					MockLayers:        func() ([]ociv1.Layer, error) { return nil, nil },
				},
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulNoOp": {
			reason: "We should return early if the supplied image is already stored.",
			files: map[string][]byte{
				// The minimum valid config file required by validate.Image.
				"cool": []byte(`{"rootfs":{"type":"layers"}}`),
			},
			args: args{
				i: &MockImage{
					MockDigest: func() (ociv1.Hash, error) { return ociv1.Hash{Hex: "cool"}, nil },
				},
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tmp, err := os.MkdirTemp(os.TempDir(), strings.ReplaceAll(t.Name(), string(os.PathSeparator), "_"))
			if err != nil {
				t.Fatal(err.Error())
			}
			t.Cleanup(func() {
				os.RemoveAll(tmp)
			})

			for name, data := range tc.files {
				path := filepath.Join(tmp, DirImages, name)
				_ = os.MkdirAll(filepath.Dir(path), 0700)
				_ = os.WriteFile(path, data, 0600)
			}

			c := NewImage(tmp)
			err = c.WriteImage(tc.args.i)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nWriteImage(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWriteLayer(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		l ociv1.Layer
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		files  map[string][]byte
		args   args
		want   want
	}{
		"DiffIDError": {
			reason: "We should return an error if we can't get the layer's (diff) digest.",
			args: args{
				l: &MockLayer{
					MockDiffID: func() (ociv1.Hash, error) { return ociv1.Hash{}, errBoom },
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetDigest),
			},
		},
		"Uncompressed": {
			reason: "We should return an error if we can't get the layer's uncompressed tarball reader.",
			args: args{
				l: &MockLayer{
					MockDiffID:       func() (ociv1.Hash, error) { return ociv1.Hash{}, nil },
					MockUncompressed: func() (io.ReadCloser, error) { return nil, errBoom },
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errReadLayer),
			},
		},
		"SuccessfulWrite": {
			reason: "We should not return an error if we successfully wrote a layer to the store.",
			args: args{
				l: &MockLayer{
					MockDiffID:       func() (ociv1.Hash, error) { return ociv1.Hash{Hex: "cool"}, nil },
					MockUncompressed: func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("")), nil },
				},
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulNoOp": {
			reason: "We should return early if the supplied layer is already stored.",
			files: map[string][]byte{
				"cool": nil, // This file just has to exist.
			},
			args: args{
				l: &MockLayer{
					MockDiffID:       func() (ociv1.Hash, error) { return ociv1.Hash{Hex: "cool"}, nil },
					MockUncompressed: func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("")), nil },
				},
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tmp, err := os.MkdirTemp(os.TempDir(), strings.ReplaceAll(t.Name(), string(os.PathSeparator), "_"))
			if err != nil {
				t.Fatal(err.Error())
			}
			t.Cleanup(func() {
				os.RemoveAll(tmp)
			})

			for name, data := range tc.files {
				path := filepath.Join(tmp, DirImages, name)
				_ = os.MkdirAll(filepath.Dir(path), 0700)
				_ = os.WriteFile(path, data, 0600)
			}

			c := NewImage(tmp)
			err = c.WriteLayer(tc.args.l)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nWriteLayer(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
