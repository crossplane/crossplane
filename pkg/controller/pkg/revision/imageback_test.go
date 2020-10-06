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

package revision

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"

	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/pkg/xpkg"
)

type MockFetcher struct {
	MockFetch func() (v1.Image, error)
}

func (m *MockFetcher) Fetch(ctx context.Context, ref name.Reference, secrets []string) (v1.Image, error) {
	return m.MockFetch()
}

func TestImageBackend(t *testing.T) {
	errBoom := errors.New("boom")
	randLayer, _ := random.Layer(int64(1000), types.DockerLayer)
	randImg, _ := mutate.AppendLayers(empty.Image, randLayer)

	streamCont := "somestreamofyaml"
	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)
	hdr := &tar.Header{
		Name: xpkg.StreamFile,
		Mode: int64(xpkg.StreamFileMode),
		Size: int64(len(streamCont)),
	}
	_ = tw.WriteHeader(hdr)
	_, _ = io.Copy(tw, strings.NewReader(streamCont))
	_ = tw.Close()
	packLayer, _ := tarball.LayerFromReader(tarBuf)
	packImg, _ := mutate.AppendLayers(empty.Image, packLayer)

	type args struct {
		c    xpkg.Cache
		f    Fetcher
		opts []parser.BackendOption
	}

	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"ErrBadReference": {
			reason: "Should return error if package tag is not a valid image reference.",
			args: args{
				opts: []parser.BackendOption{Package(":test")},
			},
			want: errors.Wrap(name.NewErrBadName("could not parse reference: :test"), errBadReference),
		},
		"ErrFetchedBadPackage": {
			reason: "Should return error if image with contents does not have package.yaml.",
			args: args{
				c: xpkg.NewNopCache(),
				f: &MockFetcher{
					MockFetch: func() (v1.Image, error) {
						return randImg, nil
					},
				},
				opts: []parser.BackendOption{Package("test/test:latest")},
			},
			want: errors.Wrap(&os.PathError{Op: "open", Path: xpkg.StreamFile, Err: syscall.ENOENT}, errOpenPackageStream),
		},
		// TODO(hasheddan): re-enable after https://github.com/spf13/afero/pull/268 is merged.
		// "ErrEmptyImage": {
		// 	reason: "Should return error if image is empty.",
		// 	args: args{
		// 		c: xpkg.NewNopCache(),
		// 		f: &MockFetcher{
		// 			MockFetch: func() (v1.Image, error) {
		// 				return empty.Image, nil
		// 			},
		// 		},
		// 		opts: []parser.BackendOption{Package("test/test:latest")},
		// 	},
		// 	want: errors.Wrap(&os.PathError{Op: "open", Path: "package.yaml", Err: syscall.ENOENT}, errOpenPackageStream),
		// },
		"ErrFetchPackage": {
			reason: "Should return error if package is not in cache and we fail to fetch it.",
			args: args{
				c: xpkg.NewNopCache(),
				f: &MockFetcher{
					MockFetch: func() (v1.Image, error) {
						return nil, errBoom
					},
				},
				opts: []parser.BackendOption{Package("test/test:latest")},
			},
			want: errors.Wrap(errBoom, errFetchPackage),
		},
		"ErrStorePackage": {
			reason: "Should return error if package is not in cache, we fetch successfully, but we fail to store it in cache.",
			args: args{
				c: &MockCache{
					MockGet:   NewMockCacheGetFn(nil, errBoom),
					MockStore: NewMockCacheStoreFn(errBoom),
				},
				f: &MockFetcher{
					MockFetch: func() (v1.Image, error) {
						return packImg, nil
					},
				},
				opts: []parser.BackendOption{Package("test/test:latest")},
			},
			want: errors.Wrap(errBoom, errCachePackage),
		},
		"SuccessFetchPackage": {
			reason: "Should not return error is package is not in cache but is fetched successfully.",
			args: args{
				c: xpkg.NewNopCache(),
				f: &MockFetcher{
					MockFetch: func() (v1.Image, error) {
						return packImg, nil
					},
				},
				opts: []parser.BackendOption{Package("test/test:latest")},
			},
		},
		"SuccessCachedPackage": {
			reason: "Should not return error is package is in cached and is gotten successfully.",
			args: args{
				c: &MockCache{
					MockGet: NewMockCacheGetFn(packImg, nil),
				},
				opts: []parser.BackendOption{Package("test/test:latest")},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			b := NewImageBackend(tc.args.c, tc.args.f)
			_, err := b.Init(context.TODO(), tc.args.opts...)

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nb.Init(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
