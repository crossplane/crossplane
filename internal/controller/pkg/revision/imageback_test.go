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
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/xpkg"
	"github.com/crossplane/crossplane/internal/xpkg/fake"
)

func TestImageBackend(t *testing.T) {
	errBoom := errors.New("boom")
	randLayer, _ := random.Layer(int64(1000), types.DockerLayer)
	randImg, _ := mutate.Append(empty.Image, mutate.Addendum{
		Layer: randLayer,
		Annotations: map[string]string{
			layerAnnotation: baseAnnotationValue,
		},
	})

	randImgDup, _ := mutate.Append(randImg, mutate.Addendum{
		Layer: randLayer,
		Annotations: map[string]string{
			layerAnnotation: baseAnnotationValue,
		},
	})

	// TODO(phisco): uncomment when https://github.com/google/go-containerregistry/pull/1758 is merged
	// streamCont := "somestreamofyaml"
	// tarBuf := new(bytes.Buffer)
	// tw := tar.NewWriter(tarBuf)
	// hdr := &tar.Header{
	// 	Name: xpkg.StreamFile,
	// 	Mode: int64(xpkg.StreamFileMode),
	// 	Size: int64(len(streamCont)),
	// }
	// _ = tw.WriteHeader(hdr)
	// _, _ = io.Copy(tw, strings.NewReader(streamCont))
	// _ = tw.Close()
	// packLayer, _ := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
	// 	// NOTE(hasheddan): we must construct a new reader each time as we
	// 	// ingest packImg in multiple tests below.
	// 	return io.NopCloser(bytes.NewReader(tarBuf.Bytes())), nil
	// })
	// packImg, _ := mutate.AppendLayers(empty.Image, packLayer)

	type args struct {
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
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package: ":test",
						},
					},
				})},
			},
			want: errors.Wrap(errors.New("could not parse reference: :test"), errBadReference),
		},
		"ErrMultipleAnnotatedLayers": {
			reason: "Should return error if image has multiple layers annotated as base.",
			args: args{
				f: &fake.MockFetcher{
					MockFetch: fake.NewMockFetchFn(randImgDup, nil),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package: "test/test:latest",
						},
					},
				})},
			},
			want: errors.New(errMultipleAnnotatedLayers),
		},
		"ErrFetchedBadPackage": {
			reason: "Should return error if image with contents does not have package.yaml.",
			args: args{
				f: &fake.MockFetcher{
					MockFetch: fake.NewMockFetchFn(randImg, nil),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package: "test/test:latest",
						},
					},
				})},
			},
			want: errors.Wrap(io.EOF, errOpenPackageStream),
		},
		"ErrEmptyImage": {
			reason: "Should return error if image is empty.",
			args: args{
				f: &fake.MockFetcher{
					MockFetch: fake.NewMockFetchFn(empty.Image, nil),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package: "test/test:latest",
						},
					},
				})},
			},
			want: errors.Wrap(io.EOF, errOpenPackageStream),
		},
		"ErrFetchPackage": {
			reason: "Should return error if package is not in cache and we fail to fetch it.",
			args: args{
				f: &fake.MockFetcher{
					MockFetch: fake.NewMockFetchFn(nil, errBoom),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package: "test/test:latest",
						},
					},
				})},
			},
			want: errors.Wrap(errBoom, errFetchPackage),
		},
		// TODO(phisco): uncomment when https://github.com/google/go-containerregistry/pull/1758 is merged
		// "SuccessFetchPackage": {
		// 	reason: "Should not return error is package is not in cache but is fetched successfully.",
		// 	args: args{
		// 		f: &fake.MockFetcher{
		// 			MockFetch: fake.NewMockFetchFn(packImg, nil),
		// 		},
		// 		opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
		// 			Spec: v1.PackageRevisionSpec{
		// 				Package: "test/test:latest",
		// 			},
		// 		})},
		// 	},
		// },
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			b := NewImageBackend(tc.args.f)
			rc, err := b.Init(context.TODO(), tc.args.opts...)
			if err == nil && rc != nil {
				_, err = io.ReadAll(rc)
			}
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nb.Init(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
