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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	ociv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/internal/oci/store"
)

type MockImage struct {
	ociv1.Image

	MockDigest func() (ociv1.Hash, error)
	MockLayers func() ([]ociv1.Layer, error)
}

func (i *MockImage) Digest() (ociv1.Hash, error)    { return i.MockDigest() }
func (i *MockImage) Layers() ([]ociv1.Layer, error) { return i.MockLayers() }

type MockLayer struct {
	ociv1.Layer

	MockDigest       func() (ociv1.Hash, error)
	MockUncompressed func() (io.ReadCloser, error)
}

func (l *MockLayer) Digest() (ociv1.Hash, error)          { return l.MockDigest() }
func (l *MockLayer) Uncompressed() (io.ReadCloser, error) { return l.MockUncompressed() }

type MockImageConfigReader struct {
	cfg *ociv1.ConfigFile
	err error
}

func (r *MockImageConfigReader) ReadConfigFile(_ ociv1.Image) (*ociv1.ConfigFile, error) {
	return r.cfg, r.err
}

type MockLayerResolver struct {
	path string
	err  error
}

func (r *MockLayerResolver) Resolve(_ context.Context, _ ociv1.Layer, _ ...ociv1.Layer) (string, error) {
	return r.path, r.err
}

type MockTarballApplicator struct{ err error }

func (a *MockTarballApplicator) Apply(_ context.Context, _ io.Reader, _ string) error { return a.err }

type MockRuntimeSpecCreator struct{ err error }

func (c *MockRuntimeSpecCreator) Create(_ store.Bundle, _ *ociv1.ConfigFile) error { return c.err }

type MockCloser struct {
	io.Reader

	err error
}

func (c *MockCloser) Close() error { return c.err }

func TestBundle(t *testing.T) {
	errBoom := errors.New("boom")

	type params struct {
		image ImageConfigReader
		layer LayerResolver
		spec  RuntimeSpecCreator
	}
	type args struct {
		ctx context.Context
		i   ociv1.Image
		id  string
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
			params: params{
				image: &MockImageConfigReader{err: errBoom},
			},
			want: want{
				err: errors.Wrap(errBoom, errReadConfigFile),
			},
		},
		"GetLayersError": {
			reason: "We should return any error encountered reading the image's layers.",
			params: params{
				image: &MockImageConfigReader{},
			},
			args: args{
				i: &MockImage{
					MockLayers: func() ([]ociv1.Layer, error) { return nil, errBoom },
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetLayers),
			},
		},
		"ResolveLayerError": {
			reason: "We should return any error encountered opening an image's layers.",
			params: params{
				image: &MockImageConfigReader{},
				layer: &MockLayerResolver{err: errBoom},
			},
			args: args{
				i: &MockImage{
					MockLayers: func() ([]ociv1.Layer, error) {
						return []ociv1.Layer{&MockLayer{}}, nil
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errResolveLayer),
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
				root:  tmp,
				image: tc.params.image,
				layer: tc.params.layer,
				spec:  tc.params.spec,
			}

			got, err := c.Bundle(tc.args.ctx, tc.args.i, tc.args.id)

			if diff := cmp.Diff(tc.want.b, got, cmpopts.IgnoreUnexported(Bundle{})); diff != "" {
				t.Errorf("\n%s\nBundle(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBundle(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
