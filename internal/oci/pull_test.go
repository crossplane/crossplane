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

package oci

import (
	"context"
	"crypto/x509"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	ociv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

type MockImage struct {
	ociv1.Image

	MockDigest func() (ociv1.Hash, error)
}

func (i *MockImage) Digest() (ociv1.Hash, error) { return i.MockDigest() }

type MockImageClient struct {
	MockImage func(ctx context.Context, ref name.Reference, o ...ImageClientOption) (ociv1.Image, error)
}

func (c *MockImageClient) Image(ctx context.Context, ref name.Reference, o ...ImageClientOption) (ociv1.Image, error) {
	return c.MockImage(ctx, ref, o...)
}

type MockImageCache struct {
	MockImage      func(h ociv1.Hash) (ociv1.Image, error)
	MockWriteImage func(img ociv1.Image) error
}

func (c *MockImageCache) Image(h ociv1.Hash) (ociv1.Image, error) {
	return c.MockImage(h)
}

func (c *MockImageCache) WriteImage(img ociv1.Image) error {
	return c.MockWriteImage(img)
}

type MockHashCache struct {
	MockHash      func(r name.Reference) (ociv1.Hash, error)
	MockWriteHash func(r name.Reference, h ociv1.Hash) error
}

func (c *MockHashCache) Hash(r name.Reference) (ociv1.Hash, error) {
	return c.MockHash(r)
}

func (c *MockHashCache) WriteHash(r name.Reference, h ociv1.Hash) error {
	return c.MockWriteHash(r, h)
}

func TestImage(t *testing.T) {
	errBoom := errors.New("boom")
	coolImage := &MockImage{}

	type args struct {
		ctx context.Context
		r   name.Reference
		o   []ImageClientOption
	}
	type want struct {
		i   ociv1.Image
		err error
	}

	cases := map[string]struct {
		reason string
		p      *CachingPuller
		args   args
		want   want
	}{
		"NeverPullHashError": {
			reason: "We should return an error if we must but can't read a hash from our HashStore.",
			p: NewCachingPuller(
				&MockHashCache{
					MockHash: func(r name.Reference) (ociv1.Hash, error) { return ociv1.Hash{}, errBoom },
				},
				&MockImageCache{},
				&MockImageClient{},
			),
			args: args{
				o: []ImageClientOption{WithPullPolicy(ImagePullPolicyNever)},
			},
			want: want{
				err: errors.Wrap(errBoom, errLoadHash),
			},
		},
		"NeverPullImageError": {
			reason: "We should return an error if we must but can't read our image from cache.",
			p: NewCachingPuller(
				&MockHashCache{
					MockHash: func(r name.Reference) (ociv1.Hash, error) { return ociv1.Hash{}, nil },
				},
				&MockImageCache{
					MockImage: func(h ociv1.Hash) (ociv1.Image, error) { return nil, errBoom },
				},
				&MockImageClient{},
			),
			args: args{
				o: []ImageClientOption{WithPullPolicy(ImagePullPolicyNever)},
			},
			want: want{
				err: errors.Wrap(errBoom, errLoadImage),
			},
		},
		"NeverPullSuccess": {
			reason: "We should return our image from cache.",
			p: NewCachingPuller(
				&MockHashCache{
					MockHash: func(r name.Reference) (ociv1.Hash, error) { return ociv1.Hash{}, nil },
				},
				&MockImageCache{
					MockImage: func(h ociv1.Hash) (ociv1.Image, error) { return coolImage, nil },
				},
				&MockImageClient{},
			),
			args: args{
				o: []ImageClientOption{WithPullPolicy(ImagePullPolicyNever)},
			},
			want: want{
				i: coolImage,
			},
		},
		"NeverPullSuccessExplicit": {
			reason: "We should return our image from cache without looking up its digest if the digest was specified explicitly.",
			p: NewCachingPuller(
				&MockHashCache{},
				&MockImageCache{
					MockImage: func(h ociv1.Hash) (ociv1.Image, error) {
						if h.Hex != "c34045c1a1db8d1b3fca8a692198466952daae07eaf6104b4c87ed3b55b6af1b" {
							return nil, errors.New("unexpected hash")
						}
						return coolImage, nil
					},
				},
				&MockImageClient{},
			),
			args: args{
				r: name.MustParseReference("example.org/coolimage@sha256:c34045c1a1db8d1b3fca8a692198466952daae07eaf6104b4c87ed3b55b6af1b"),
				o: []ImageClientOption{WithPullPolicy(ImagePullPolicyNever)},
			},
			want: want{
				i: coolImage,
			},
		},
		"AlwaysPullRemoteError": {
			reason: "We should return an error if we must but can't pull our image manifest from the remote.",
			p: NewCachingPuller(
				&MockHashCache{},
				&MockImageCache{},
				&MockImageClient{
					MockImage: func(ctx context.Context, ref name.Reference, o ...ImageClientOption) (ociv1.Image, error) {
						return nil, errBoom
					},
				},
			),
			args: args{
				o: []ImageClientOption{WithPullPolicy(ImagePullPolicyAlways)},
			},
			want: want{
				err: errors.Wrap(errBoom, errPullImage),
			},
		},
		"AlwaysPullWriteImageError": {
			reason: "We should return an error if we must but can't write our image to the local cache.",
			p: NewCachingPuller(
				&MockHashCache{},
				&MockImageCache{
					MockWriteImage: func(img ociv1.Image) error { return errBoom },
				},
				&MockImageClient{
					MockImage: func(ctx context.Context, ref name.Reference, o ...ImageClientOption) (ociv1.Image, error) {
						return nil, nil
					},
				},
			),
			args: args{
				o: []ImageClientOption{WithPullPolicy(ImagePullPolicyAlways)},
			},
			want: want{
				err: errors.Wrap(errBoom, errStoreImage),
			},
		},
		"AlwaysPullImageDigestError": {
			reason: "We should return an error if we can't get our image's digest.",
			p: NewCachingPuller(
				&MockHashCache{},
				&MockImageCache{
					MockWriteImage: func(img ociv1.Image) error { return nil },
				},
				&MockImageClient{
					MockImage: func(ctx context.Context, ref name.Reference, o ...ImageClientOption) (ociv1.Image, error) {
						return &MockImage{
							MockDigest: func() (ociv1.Hash, error) { return ociv1.Hash{}, errBoom },
						}, nil
					},
				},
			),
			args: args{
				o: []ImageClientOption{WithPullPolicy(ImagePullPolicyAlways)},
			},
			want: want{
				err: errors.Wrap(errBoom, errImageDigest),
			},
		},
		"AlwaysPullWriteDigestError": {
			reason: "We should return an error if we can't write our digest mapping to the cache.",
			p: NewCachingPuller(
				&MockHashCache{
					MockWriteHash: func(r name.Reference, h ociv1.Hash) error { return errBoom },
				},
				&MockImageCache{
					MockWriteImage: func(img ociv1.Image) error { return nil },
				},
				&MockImageClient{
					MockImage: func(ctx context.Context, ref name.Reference, o ...ImageClientOption) (ociv1.Image, error) {
						return &MockImage{
							MockDigest: func() (ociv1.Hash, error) { return ociv1.Hash{}, nil },
						}, nil
					},
				},
			),
			args: args{
				o: []ImageClientOption{WithPullPolicy(ImagePullPolicyAlways)},
			},
			want: want{
				err: errors.Wrap(errBoom, errStoreDigest),
			},
		},
		"AlwaysPullImageError": {
			reason: "We should return an error if we must but can't read our image back from cache.",
			p: NewCachingPuller(
				&MockHashCache{
					MockWriteHash: func(r name.Reference, h ociv1.Hash) error { return nil },
				},
				&MockImageCache{
					MockWriteImage: func(img ociv1.Image) error { return nil },
					MockImage:      func(h ociv1.Hash) (ociv1.Image, error) { return nil, errBoom },
				},
				&MockImageClient{
					MockImage: func(ctx context.Context, ref name.Reference, o ...ImageClientOption) (ociv1.Image, error) {
						return &MockImage{
							MockDigest: func() (ociv1.Hash, error) { return ociv1.Hash{}, nil },
						}, nil
					},
				},
			),
			args: args{
				o: []ImageClientOption{WithPullPolicy(ImagePullPolicyAlways)},
			},
			want: want{
				err: errors.Wrap(errBoom, errLoadImage),
			},
		},
		"AlwaysPullSuccess": {
			reason: "We should return a pulled and cached image.",
			p: NewCachingPuller(
				&MockHashCache{
					MockWriteHash: func(r name.Reference, h ociv1.Hash) error { return nil },
				},
				&MockImageCache{
					MockWriteImage: func(img ociv1.Image) error { return nil },
					MockImage:      func(h ociv1.Hash) (ociv1.Image, error) { return &MockImage{}, nil },
				},
				&MockImageClient{
					MockImage: func(ctx context.Context, ref name.Reference, o ...ImageClientOption) (ociv1.Image, error) {
						return &MockImage{
							MockDigest: func() (ociv1.Hash, error) { return ociv1.Hash{}, nil },
						}, nil
					},
				},
			),
			args: args{
				o: []ImageClientOption{WithPullPolicy(ImagePullPolicyAlways)},
			},
			want: want{
				i: &MockImage{},
			},
		},
		"PullWithCustomCA": {
			reason: "We should return a pulled and cached image.",
			p: NewCachingPuller(
				&MockHashCache{
					MockHash: func(r name.Reference) (ociv1.Hash, error) {
						return ociv1.Hash{}, errors.New("this error should not be returned")
					},
					MockWriteHash: func(r name.Reference, h ociv1.Hash) error {
						return nil
					},
				},
				&MockImageCache{
					MockWriteImage: func(img ociv1.Image) error { return nil },
					MockImage:      func(h ociv1.Hash) (ociv1.Image, error) { return &MockImage{}, nil },
				},
				&MockImageClient{
					MockImage: func(ctx context.Context, ref name.Reference, o ...ImageClientOption) (ociv1.Image, error) {
						if len(o) != 1 {
							return nil, errors.New("the number of options should be one")
						}
						c := &ImageClientOptions{}
						o[0](c)
						if c.transport == nil {
							return nil, errors.New("Transport should be set")
						}
						return &MockImage{
							MockDigest: func() (ociv1.Hash, error) { return ociv1.Hash{}, nil },
						}, nil
					},
				},
			),
			args: args{
				o: []ImageClientOption{WithCustomCA(&x509.CertPool{})},
			},
			want: want{
				i: &MockImage{},
			},
		},
		"IfNotPresentTriesCacheFirst": {
			reason: "The IfNotPresent policy should try to read from cache first.",
			p: NewCachingPuller(
				&MockHashCache{
					MockHash: func(r name.Reference) (ociv1.Hash, error) { return ociv1.Hash{}, nil },
				},
				&MockImageCache{
					MockImage: func(h ociv1.Hash) (ociv1.Image, error) { return &MockImage{}, nil },
				},
				&MockImageClient{
					// If we get here it indicates we called always.
					MockImage: func(ctx context.Context, ref name.Reference, o ...ImageClientOption) (ociv1.Image, error) {
						return nil, errors.New("this error should not be returned")
					},
				},
			),
			args: args{
				o: []ImageClientOption{WithPullPolicy(ImagePullPolicyIfNotPresent)},
			},
			want: want{
				i: &MockImage{},
			},
		},
		"IfNotPresentFallsBackToRemote": {
			reason: "The IfNotPresent policy should fall back to pulling from the remote if it can't read the image from cache.",
			p: NewCachingPuller(
				&MockHashCache{
					MockHash: func(r name.Reference) (ociv1.Hash, error) {
						// Trigger a fall-back from never to always.
						return ociv1.Hash{}, errors.New("this error should not be returned")
					},
				},
				&MockImageCache{},
				&MockImageClient{
					MockImage: func(ctx context.Context, ref name.Reference, o ...ImageClientOption) (ociv1.Image, error) {
						return nil, errBoom
					},
				},
			),
			args: args{
				o: []ImageClientOption{WithPullPolicy(ImagePullPolicyIfNotPresent)},
			},
			want: want{
				// This indicates we fell back to always.
				err: errors.Wrap(errBoom, errPullImage),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			i, err := tc.p.Image(tc.args.ctx, tc.args.r, tc.args.o...)
			if diff := cmp.Diff(tc.want.i, i); diff != "" {
				t.Errorf("\n%s\nImage(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nImage(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}

}
