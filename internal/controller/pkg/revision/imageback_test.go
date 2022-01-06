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
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	corev1 "k8s.io/api/core/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/xpkg"
	"github.com/crossplane/crossplane/internal/xpkg/fake"
)

func TestImageBackend(t *testing.T) {
	errBoom := errors.New("boom")
	pullPolicy := corev1.PullNever
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
		f    xpkg.Fetcher
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
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						Package: ":test",
					},
				})},
			},
			want: errors.Wrap(errors.New("could not parse reference: :test"), errBadReference),
		},
		"ErrFetchedBadPackage": {
			reason: "Should return error if image with contents does not have package.yaml.",
			args: args{
				c: xpkg.NewNopCache(),
				f: &fake.MockFetcher{
					MockFetch: fake.NewMockFetchFn(randImg, nil),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						Package: "test/test:latest",
					},
				})},
			},
			want: errors.Wrap(&os.PathError{Op: "open", Path: xpkg.StreamFile, Err: syscall.ENOENT}, errOpenPackageStream),
		},
		"ErrEmptyImage": {
			reason: "Should return error if image is empty.",
			args: args{
				c: xpkg.NewNopCache(),
				f: &fake.MockFetcher{
					MockFetch: fake.NewMockFetchFn(empty.Image, nil),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						Package: "test/test:latest",
					},
				})},
			},
			want: errors.Wrap(&os.PathError{Op: "open", Path: "package.yaml", Err: syscall.ENOENT}, errOpenPackageStream),
		},
		"ErrFetchPackage": {
			reason: "Should return error if package is not in cache and we fail to fetch it.",
			args: args{
				c: xpkg.NewNopCache(),
				f: &fake.MockFetcher{
					MockFetch: fake.NewMockFetchFn(nil, errBoom),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						Package: "test/test:latest",
					},
				})},
			},
			want: errors.Wrap(errBoom, errFetchPackage),
		},
		"ErrStorePackage": {
			reason: "Should return error if package is not in cache, we fetch successfully, but we fail to store it in cache.",
			args: args{
				c: &fake.MockCache{
					MockGet:   fake.NewMockCacheGetFn(nil, errBoom),
					MockStore: fake.NewMockCacheStoreFn(errBoom),
				},
				f: &fake.MockFetcher{
					MockFetch: fake.NewMockFetchFn(packImg, nil),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						Package: "test/test:latest",
					},
				})},
			},
			want: errors.Wrap(errBoom, errCachePackage),
		},
		"SuccessFetchPackage": {
			reason: "Should not return error is package is not in cache but is fetched successfully.",
			args: args{
				c: xpkg.NewNopCache(),
				f: &fake.MockFetcher{
					MockFetch: fake.NewMockFetchFn(packImg, nil),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						Package: "test/test:latest",
					},
				})},
			},
		},
		"SuccessCachedPackage": {
			reason: "Should not return error is package is in cache and is gotten successfully.",
			args: args{
				c: &fake.MockCache{
					MockGet: fake.NewMockCacheGetFn(packImg, nil),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						Package: "test/test:latest",
					},
				})},
			},
		},
		"ErrorCachedPackageNoPull": {
			reason: "Should return error if package is pre-cached and is not gotten successfully.",
			args: args{
				c: &fake.MockCache{
					MockGet: fake.NewMockCacheGetFn(nil, errBoom),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						Package:           "test/test:latest",
						PackagePullPolicy: &pullPolicy,
					},
				})},
			},
			want: errors.Wrap(errBoom, errPullPolicyNever),
		},
		"SuccessCachedPackageNoPull": {
			reason: "Should not return error if package is pre-cached and is gotten successfully.",
			args: args{
				c: &fake.MockCache{
					MockGet: fake.NewMockCacheGetFn(packImg, nil),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						Package:           "test/test:latest",
						PackagePullPolicy: &pullPolicy,
					},
				})},
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
