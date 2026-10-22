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
	"github.com/google/go-containerregistry/pkg/name"
	ociv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg"
	"github.com/crossplane/crossplane/internal/xpkg/fake"
)

// testDigest is the digest the fake registry resolves every tag to.
var testDigest = ociv1.Hash{
	Algorithm: "sha256",
	Hex:       "ecc4c9eff5b0c4de6dcb24b6c4c96d5e5f0f8ab0e5e58ea4d2c7d6e6bd2e5a11",
}

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
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: ":test",
					},
				})},
			},
			want: errors.Wrap(errors.New("could not parse reference: :test"), errBadReference),
		},
		"ErrMultipleAnnotatedLayers": {
			reason: "Should return error if image has multiple layers annotated as base.",
			args: args{
				f: &fake.MockFetcher{
					MockHead:  fake.NewMockHeadFn(&ociv1.Descriptor{Digest: testDigest}, nil),
					MockFetch: fake.NewMockFetchFn(randImgDup, nil),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package: "test/test:latest",
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: "test/test:latest",
					},
				})},
			},
			want: errors.New(errMultipleAnnotatedLayers),
		},
		"ErrFetchedBadPackage": {
			reason: "Should return error if image with contents does not have package.yaml.",
			args: args{
				f: &fake.MockFetcher{
					MockHead:  fake.NewMockHeadFn(&ociv1.Descriptor{Digest: testDigest}, nil),
					MockFetch: fake.NewMockFetchFn(randImg, nil),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package: "test/test:latest",
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: "test/test:latest",
					},
				})},
			},
			want: errors.Wrapf(io.EOF, errFmtNoPackageFileFound, 1, true),
		},
		"ErrEmptyImage": {
			reason: "Should return error if image is empty.",
			args: args{
				f: &fake.MockFetcher{
					MockHead:  fake.NewMockHeadFn(&ociv1.Descriptor{Digest: testDigest}, nil),
					MockFetch: fake.NewMockFetchFn(empty.Image, nil),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package: "test/test:latest",
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: "test/test:latest",
					},
				})},
			},
			want: errors.Wrapf(io.EOF, errFmtNoPackageFileFound, 0, false),
		},
		"ErrFetchPackage": {
			reason: "Should return error if package is not in cache and we fail to fetch it.",
			args: args{
				f: &fake.MockFetcher{
					MockHead:  fake.NewMockHeadFn(&ociv1.Descriptor{Digest: testDigest}, nil),
					MockFetch: fake.NewMockFetchFn(nil, errBoom),
				},
				opts: []parser.BackendOption{PackageRevision(&v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package: "test/test:latest",
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: "test/test:latest",
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
			rc, err := b.Init(t.Context(), tc.args.opts...)
			if err == nil && rc != nil {
				_, err = io.ReadAll(rc)
			}
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nb.Init(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

// refString renders a reference for comparison, tolerating the nil we get when
// the backend never made the call at all.
func refString(r name.Reference) string {
	if r == nil {
		return ""
	}

	return r.String()
}

// recordingFetcher records the reference it was asked to fetch, so we can
// assert the backend never goes back to the registry by tag once it has
// resolved a digest.
type recordingFetcher struct {
	desc     *ociv1.Descriptor
	headErr  error
	img      ociv1.Image
	fetchErr error

	headed  name.Reference
	fetched name.Reference
}

func (f *recordingFetcher) Head(_ context.Context, ref name.Reference, _ ...string) (*ociv1.Descriptor, error) {
	f.headed = ref
	return f.desc, f.headErr
}

func (f *recordingFetcher) Fetch(_ context.Context, ref name.Reference, _ ...string) (ociv1.Image, error) {
	f.fetched = ref
	return f.img, f.fetchErr
}

func (f *recordingFetcher) Tags(_ context.Context, _ name.Reference, _ ...string) ([]string, error) {
	return nil, nil
}

// recordingValidator records the reference whose signature it was asked to
// validate.
type recordingValidator struct {
	err error

	validated name.Reference
}

func (v *recordingValidator) Validate(_ context.Context, ref name.Reference, _ *v1beta1.ImageVerification, _ ...string) error {
	v.validated = ref
	return v.err
}

// TestImageBackendVerifiesAndFetchesSameDigest covers GHSA-wfqx-gjrf-g28r: a
// registry that resolves a tag to signed content while it is verified and to
// different content while it is pulled must not be able to get the unsigned
// content installed. The backend resolves the tag once and pins both the
// signature check and the pull to the resulting digest.
func TestImageBackendVerifiesAndFetchesSameDigest(t *testing.T) {
	errBoom := errors.New("boom")

	const source = "xpkg.crossplane.io/test/test:latest"

	wantRef := name.MustParseReference(source).Context().Digest(testDigest.String()).String()

	pr := PackageRevision(&v1.ProviderRevision{
		Spec: v1.ProviderRevisionSpec{
			PackageRevisionSpec: v1.PackageRevisionSpec{Package: source},
		},
		Status: v1.PackageRevisionStatus{ResolvedPackage: source},
	})

	verified := &v1beta1.ImageVerification{Cosign: &v1beta1.CosignVerificationConfig{}}

	cases := map[string]struct {
		reason        string
		fetcher       *recordingFetcher
		validator     *recordingValidator
		vc            *v1beta1.ImageVerification
		vcErr         error
		wantErr       error
		wantValidated string
		wantFetched   string
	}{
		"VerifyAndFetchByDigest": {
			reason:        "Both the signature check and the pull should target the digest the tag resolved to.",
			fetcher:       &recordingFetcher{desc: &ociv1.Descriptor{Digest: testDigest}, img: empty.Image},
			validator:     &recordingValidator{},
			vc:            verified,
			wantErr:       errors.Wrapf(io.EOF, errFmtNoPackageFileFound, 0, false),
			wantValidated: wantRef,
			wantFetched:   wantRef,
		},
		"FetchByDigestWithoutVerification": {
			reason:      "The pull should target the digest even when no verification config matches.",
			fetcher:     &recordingFetcher{desc: &ociv1.Descriptor{Digest: testDigest}, img: empty.Image},
			validator:   &recordingValidator{},
			vc:          nil,
			wantErr:     errors.Wrapf(io.EOF, errFmtNoPackageFileFound, 0, false),
			wantFetched: wantRef,
		},
		"ErrVerificationFailed": {
			reason:        "A failed signature check should stop us fetching the package at all.",
			fetcher:       &recordingFetcher{desc: &ociv1.Descriptor{Digest: testDigest}, img: empty.Image},
			validator:     &recordingValidator{err: errBoom},
			vc:            verified,
			wantErr:       errors.Wrap(errBoom, errVerifyPackage),
			wantValidated: wantRef,
		},
		"ErrResolveDigest": {
			reason:    "We should not verify or fetch anything if we cannot resolve the tag to a digest.",
			fetcher:   &recordingFetcher{headErr: errBoom},
			validator: &recordingValidator{},
			vc:        verified,
			wantErr:   errors.Wrap(errBoom, errResolveDigest),
		},
		"ErrGetVerificationConfig": {
			reason:    "We should not fetch anything if we cannot tell whether the package needs verifying.",
			fetcher:   &recordingFetcher{desc: &ociv1.Descriptor{Digest: testDigest}, img: empty.Image},
			validator: &recordingValidator{},
			vcErr:     errBoom,
			wantErr:   errors.Wrap(errBoom, errGetVerificationConfig),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &fake.MockConfigStore{
				MockImageVerificationConfigFor: fake.NewMockConfigStoreImageVerificationConfigForFn("test", tc.vc, tc.vcErr),
			}

			b := NewImageBackend(tc.fetcher, WithVerification(c, tc.validator))

			rc, err := b.Init(t.Context(), pr)
			if err == nil && rc != nil {
				_, err = io.ReadAll(rc)
			}

			if diff := cmp.Diff(tc.wantErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nb.Init(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.wantValidated, refString(tc.validator.validated)); diff != "" {
				t.Errorf("\n%s\nb.Init(...): -want validated ref, +got validated ref:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.wantFetched, refString(tc.fetcher.fetched)); diff != "" {
				t.Errorf("\n%s\nb.Init(...): -want fetched ref, +got fetched ref:\n%s", tc.reason, diff)
			}
		})
	}
}
