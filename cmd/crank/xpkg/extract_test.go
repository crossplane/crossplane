/*
Copyright 2025 The Crossplane Authors.

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

package xpkg

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/internal/xpkg"
)

func TestExtractRun(t *testing.T) {
	errBoom := errors.New("boom")
	validTag := name.MustParseReference("crossplane/provider-aws:v0.24.1")
	randLayer, _ := random.Layer(int64(1000), types.DockerLayer)
	randImg, _ := mutate.Append(empty.Image, mutate.Addendum{
		Layer: randLayer,
		Annotations: map[string]string{
			xpkg.AnnotationKey: xpkg.PackageAnnotation,
		},
	})

	randImgDup, _ := mutate.Append(randImg, mutate.Addendum{
		Layer: randLayer,
		Annotations: map[string]string{
			xpkg.AnnotationKey: xpkg.PackageAnnotation,
		},
	})

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

	packLayer, _ := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		// NOTE(hasheddan): we must construct a new reader each time as we
		// ingest packImg in multiple tests below.
		return io.NopCloser(bytes.NewReader(tarBuf.Bytes())), nil
	})
	packImg, _ := mutate.AppendLayers(empty.Image, packLayer)

	cases := map[string]struct {
		reason string
		fs     afero.Fs
		name   name.Reference
		fetch  fetchFn
		out    string
		want   error
	}{
		"ErrorFetchPackage": {
			reason: "Should return error if we fail to fetch package.",
			name:   validTag,
			fetch: func(_ context.Context, _ name.Reference) (v1.Image, error) {
				return nil, errBoom
			},
			want: errors.Wrap(errBoom, errFetchPackage),
		},
		"ErrorMultipleAnnotatedLayers": {
			reason: "Should return error if manifest contains multiple annotated layers.",
			name:   validTag,
			fetch: func(_ context.Context, _ name.Reference) (v1.Image, error) {
				return randImgDup, nil
			},
			want: errors.New(errMultipleAnnotatedLayers),
		},
		"ErrorFetchBadPackage": {
			reason: "Should return error if image with contents does not have package.yaml.",
			name:   validTag,
			fetch: func(_ context.Context, _ name.Reference) (v1.Image, error) {
				return randImg, nil
			},
			want: errors.Wrap(io.EOF, errOpenPackageStream),
		},
		"Success": {
			reason: "Should not return error if we successfully fetch package and extract contents.",
			name:   validTag,
			fetch: func(_ context.Context, _ name.Reference) (v1.Image, error) {
				return packImg, nil
			},
			fs:  afero.NewMemMapFs(),
			out: "out.gz",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			logger := logging.NewNopLogger()

			err := (&extractCmd{
				fs:     tc.fs,
				fetch:  tc.fetch,
				name:   tc.name,
				Output: tc.out,
			}).Run(logger)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
