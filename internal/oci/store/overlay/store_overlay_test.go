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

package overlay

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	ociv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/internal/oci/spec"
	"github.com/crossplane/crossplane/internal/oci/store"
)

type MockImage struct {
	ociv1.Image

	MockDigest     func() (ociv1.Hash, error)
	MockConfigFile func() (*ociv1.ConfigFile, error)
	MockLayers     func() ([]ociv1.Layer, error)
}

func (i *MockImage) Digest() (ociv1.Hash, error)            { return i.MockDigest() }
func (i *MockImage) ConfigFile() (*ociv1.ConfigFile, error) { return i.MockConfigFile() }
func (i *MockImage) Layers() ([]ociv1.Layer, error)         { return i.MockLayers() }

type MockLayer struct {
	ociv1.Layer

	MockDiffID       func() (ociv1.Hash, error)
	MockUncompressed func() (io.ReadCloser, error)
}

func (l *MockLayer) DiffID() (ociv1.Hash, error)          { return l.MockDiffID() }
func (l *MockLayer) Uncompressed() (io.ReadCloser, error) { return l.MockUncompressed() }

type MockLayerResolver struct {
	path string
	err  error
}

func (r *MockLayerResolver) Resolve(_ context.Context, _ ociv1.Layer, _ ...ociv1.Layer) (string, error) {
	return r.path, r.err
}

type MockTarballApplicator struct{ err error }

func (a *MockTarballApplicator) Apply(_ context.Context, _ io.Reader, _ string) error { return a.err }

type MockRuntimeSpecWriter struct{ err error }

func (c *MockRuntimeSpecWriter) Write(_ string, _ ...spec.Option) error { return c.err }

type MockCloser struct {
	io.Reader

	err error
}

func (c *MockCloser) Close() error { return c.err }

type MockMount struct{ err error }

func (m *MockMount) Mount() error   { return m.err }
func (m *MockMount) Unmount() error { return m.err }

func TestBundle(t *testing.T) {
	errBoom := errors.New("boom")

	type params struct {
		layer  LayerResolver
		bundle BundleBootstrapper
		spec   RuntimeSpecWriter
	}
	type args struct {
		ctx context.Context
		i   ociv1.Image
		id  string
		o   []spec.Option
	}
	type want struct {
		b   store.Bundle
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"ReadConfigFileError": {
			reason: "We should return any error encountered reading the image's config file.",
			params: params{},
			args: args{
				i: &MockImage{
					MockConfigFile: func() (*ociv1.ConfigFile, error) { return nil, errBoom },
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errReadConfigFile),
			},
		},
		"GetLayersError": {
			reason: "We should return any error encountered reading the image's layers.",
			params: params{},
			args: args{
				i: &MockImage{
					MockConfigFile: func() (*ociv1.ConfigFile, error) { return nil, nil },
					MockLayers:     func() ([]ociv1.Layer, error) { return nil, errBoom },
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetLayers),
			},
		},
		"ResolveLayerError": {
			reason: "We should return any error encountered opening an image's layers.",
			params: params{
				layer: &MockLayerResolver{err: errBoom},
			},
			args: args{
				i: &MockImage{
					MockConfigFile: func() (*ociv1.ConfigFile, error) { return nil, nil },
					MockLayers: func() ([]ociv1.Layer, error) {
						return []ociv1.Layer{&MockLayer{}}, nil
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errResolveLayer),
			},
		},
		"BootstrapBundleError": {
			reason: "We should return any error encountered bootstrapping a bundle rootfs.",
			params: params{
				layer: &MockLayerResolver{err: nil},
				bundle: BundleBootstrapperFn(func(path string, parentLayerPaths []string) (Bundle, error) {
					return Bundle{}, errBoom
				}),
			},
			args: args{
				i: &MockImage{
					MockConfigFile: func() (*ociv1.ConfigFile, error) { return nil, nil },
					MockLayers:     func() ([]ociv1.Layer, error) { return nil, nil },
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errBootstrapBundle),
			},
		},
		"WriteSpecError": {
			reason: "We should return any error encountered writing a runtime spec to the bundle.",
			params: params{
				layer: &MockLayerResolver{err: nil},
				bundle: BundleBootstrapperFn(func(path string, parentLayerPaths []string) (Bundle, error) {
					return Bundle{}, nil
				}),
				spec: &MockRuntimeSpecWriter{err: errBoom},
			},
			args: args{
				i: &MockImage{
					MockConfigFile: func() (*ociv1.ConfigFile, error) { return nil, nil },
					MockLayers:     func() ([]ociv1.Layer, error) { return nil, nil },
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errWriteRuntimeSpec),
			},
		},
		"Success": {
			reason: "We should successfully return our Bundle.",
			params: params{
				layer: &MockLayerResolver{err: nil},
				bundle: BundleBootstrapperFn(func(path string, parentLayerPaths []string) (Bundle, error) {
					return Bundle{path: "/coolbundle"}, nil
				}),
				spec: &MockRuntimeSpecWriter{err: nil},
			},
			args: args{
				i: &MockImage{
					MockConfigFile: func() (*ociv1.ConfigFile, error) { return nil, nil },
					MockLayers: func() ([]ociv1.Layer, error) {
						return []ociv1.Layer{&MockLayer{}}, nil
					},
				},
			},
			want: want{
				b: Bundle{path: "/coolbundle"},
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

			c := &CachingBundler{
				root:   tmp,
				layer:  tc.params.layer,
				bundle: tc.params.bundle,
				spec:   tc.params.spec,
			}

			got, err := c.Bundle(tc.args.ctx, tc.args.i, tc.args.id, tc.args.o...)

			if diff := cmp.Diff(tc.want.b, got, cmp.AllowUnexported(Bundle{})); diff != "" {
				t.Errorf("\n%s\nBundle(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBundle(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	errBoom := errors.New("boom")

	type params struct {
		tarball TarballApplicator
		wdopts  []NewLayerWorkdirOption
	}
	type args struct {
		ctx     context.Context
		l       ociv1.Layer
		parents []ociv1.Layer
	}
	type want struct {
		path string
		err  error
	}

	cases := map[string]struct {
		reason string
		files  map[string][]byte
		params params
		args   args
		want   want
	}{
		"DiffIDError": {
			reason: "We should return any error encountered getting the uncompressed layer's digest.",
			args: args{
				l: &MockLayer{
					MockDiffID: func() (ociv1.Hash, error) { return ociv1.Hash{}, errBoom },
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetDigest),
			},
		},
		"SuccessExistingLayer": {
			reason: "We should skip straight to returning the layer if it already exists.",
			files: map[string][]byte{
				"sha256/deadbeef": nil,
			},
			args: args{
				l: &MockLayer{
					MockDiffID: func() (ociv1.Hash, error) {
						return ociv1.Hash{Algorithm: "sha256", Hex: "deadbeef"}, nil
					},
				},
			},
			want: want{
				path: "/sha256/deadbeef",
			},
		},
		"FetchLayerError": {
			reason: "We should return any error we encounter while fetching a layer.",
			args: args{
				l: &MockLayer{
					MockDiffID: func() (ociv1.Hash, error) {
						return ociv1.Hash{Algorithm: "sha256", Hex: "deadbeef"}, nil
					},
					MockUncompressed: func() (io.ReadCloser, error) { return nil, errBoom },
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errFetchLayer),
			},
		},
		"ParentDiffIDError": {
			reason: "We should return any error we encounter while fetching a parent's uncompressed digest.",
			args: args{
				l: &MockLayer{
					MockDiffID: func() (ociv1.Hash, error) {
						return ociv1.Hash{Algorithm: "sha256", Hex: "deadbeef"}, nil
					},
					MockUncompressed: func() (io.ReadCloser, error) { return nil, nil },
				},
				parents: []ociv1.Layer{
					&MockLayer{
						MockDiffID: func() (ociv1.Hash, error) {
							return ociv1.Hash{}, errBoom
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetDigest),
			},
		},
		"NewLayerWorkDirMountOverlayError": {
			reason: "We should return any error we encounter when mounting our overlayfs",
			params: params{
				wdopts: []NewLayerWorkdirOption{
					WithNewOverlayMountFn(func(path string, parentLayerPaths []string) Mount {
						return &MockMount{err: errBoom}
					}),
				},
			},
			args: args{
				l: &MockLayer{
					MockDiffID: func() (ociv1.Hash, error) {
						return ociv1.Hash{Algorithm: "sha256", Hex: "deadbeef"}, nil
					},
					MockUncompressed: func() (io.ReadCloser, error) { return nil, nil },
				},
				parents: []ociv1.Layer{
					&MockLayer{
						MockDiffID: func() (ociv1.Hash, error) {
							return ociv1.Hash{Algorithm: "sha256", Hex: "badc0ffee"}, nil
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errMountOverlayfs), errMkWorkdir),
			},
		},
		"ApplyTarballError": {
			reason: "We should return any error we encounter while applying our layer tarball.",
			params: params{
				tarball: &MockTarballApplicator{err: errBoom},
				wdopts: []NewLayerWorkdirOption{
					WithNewOverlayMountFn(func(path string, parentLayerPaths []string) Mount {
						return &MockMount{err: nil}
					}),
				},
			},
			args: args{
				l: &MockLayer{
					MockDiffID: func() (ociv1.Hash, error) {
						return ociv1.Hash{Algorithm: "sha256", Hex: "deadbeef"}, nil
					},
					MockUncompressed: func() (io.ReadCloser, error) { return nil, nil },
				},
				parents: []ociv1.Layer{
					&MockLayer{
						MockDiffID: func() (ociv1.Hash, error) {
							return ociv1.Hash{Algorithm: "sha256", Hex: "badc0ffee"}, nil
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errApplyLayer),
			},
		},
		"SuccessNewlyCachedLayer": {
			reason: "We should return the path to our successfully cached layer.",
			params: params{
				tarball: &MockTarballApplicator{},
				wdopts: []NewLayerWorkdirOption{
					WithNewOverlayMountFn(func(path string, parentLayerPaths []string) Mount {
						return &MockMount{err: nil}
					}),
				},
			},
			args: args{
				l: &MockLayer{
					MockDiffID: func() (ociv1.Hash, error) {
						return ociv1.Hash{Algorithm: "sha256", Hex: "deadbeef"}, nil
					},
					MockUncompressed: func() (io.ReadCloser, error) { return nil, nil },
				},
				parents: []ociv1.Layer{
					&MockLayer{
						MockDiffID: func() (ociv1.Hash, error) {
							return ociv1.Hash{Algorithm: "sha256", Hex: "badc0ffee"}, nil
						},
					},
				},
			},
			want: want{
				path: "/sha256/deadbeef",
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

			c := &CachingLayerResolver{
				root:    tmp,
				tarball: tc.params.tarball,
				wdopts:  tc.params.wdopts,
			}

			// Prepend our randomly named tmp dir to our wanted layer path.
			wantPath := tc.want.path
			if tc.want.path != "" {
				wantPath = filepath.Join(tmp, tc.want.path)
			}

			path, err := c.Resolve(tc.args.ctx, tc.args.l, tc.args.parents...)

			if diff := cmp.Diff(wantPath, path); diff != "" {
				t.Errorf("\n%s\nResolve(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nResolve(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
