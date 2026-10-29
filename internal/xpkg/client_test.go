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
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/parser"

	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
)

const (
	testDigest = "sha256:abc123def456789012345678901234567890123456789012345678901234abcd"
	testSource = "xpkg.crossplane.io/crossplane-contrib/provider-aws"
	testTag    = "v1.0.0"
)

var _ Fetcher = &MockFetcher{}

type MockFetcher struct {
	MockFetch func(context.Context, name.Reference, ...string) (v1.Image, error)
	MockHead  func(context.Context, name.Reference, ...string) (*v1.Descriptor, error)
	MockTags  func(context.Context, name.Reference, ...string) ([]string, error)
}

func (m *MockFetcher) Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error) {
	return m.MockFetch(ctx, ref, secrets...)
}

func (m *MockFetcher) Head(ctx context.Context, ref name.Reference, secrets ...string) (*v1.Descriptor, error) {
	return m.MockHead(ctx, ref, secrets...)
}

func (m *MockFetcher) Tags(ctx context.Context, ref name.Reference, secrets ...string) ([]string, error) {
	return m.MockTags(ctx, ref, secrets...)
}

var _ PackageCache = &MockCache{}

type MockCache struct {
	MockGet    func(string) (io.ReadCloser, error)
	MockStore  func(string, io.ReadCloser) error
	MockDelete func(string) error
	MockHas    func(string) bool
}

func (m *MockCache) Get(key string) (io.ReadCloser, error) {
	return m.MockGet(key)
}

func (m *MockCache) Store(key string, rc io.ReadCloser) error {
	return m.MockStore(key, rc)
}

func (m *MockCache) Delete(key string) error {
	return m.MockDelete(key)
}

func (m *MockCache) Has(key string) bool {
	return m.MockHas(key)
}

var _ ConfigStore = &MockConfigStore{}

type MockConfigStore struct {
	MockRewritePath                func(context.Context, string) (string, string, error)
	MockPullSecretFor              func(context.Context, string) (string, string, error)
	MockImageVerificationConfigFor func(context.Context, string) (string, *v1beta1.ImageVerification, error)
	MockRuntimeConfigFor           func(context.Context, string) (string, *v1beta1.ImageRuntime, error)
}

func (m *MockConfigStore) RewritePath(ctx context.Context, ref string) (string, string, error) {
	return m.MockRewritePath(ctx, ref)
}

func (m *MockConfigStore) PullSecretFor(ctx context.Context, ref string) (string, string, error) {
	return m.MockPullSecretFor(ctx, ref)
}

func (m *MockConfigStore) ImageVerificationConfigFor(ctx context.Context, ref string) (string, *v1beta1.ImageVerification, error) {
	return m.MockImageVerificationConfigFor(ctx, ref)
}

func (m *MockConfigStore) RuntimeConfigFor(ctx context.Context, ref string) (string, *v1beta1.ImageRuntime, error) {
	return m.MockRuntimeConfigFor(ctx, ref)
}

type MockValidator struct {
	MockValidate func(context.Context, name.Reference, *v1beta1.ImageVerification, ...string) error
}

func (m *MockValidator) Validate(ctx context.Context, ref name.Reference, config *v1beta1.ImageVerification, pullSecrets ...string) error {
	return m.MockValidate(ctx, ref, config, pullSecrets...)
}

type MockImage struct {
	v1.Image
	MockManifest      func() (*v1.Manifest, error)
	MockLayerByDigest func(v1.Hash) (v1.Layer, error)
	MockLayers        func() ([]v1.Layer, error)
}

func (m *MockImage) Manifest() (*v1.Manifest, error) {
	return m.MockManifest()
}

func (m *MockImage) LayerByDigest(h v1.Hash) (v1.Layer, error) {
	return m.MockLayerByDigest(h)
}

func (m *MockImage) Layers() ([]v1.Layer, error) {
	if m.MockLayers != nil {
		return m.MockLayers()
	}
	return nil, nil
}

type MockLayer struct {
	v1.Layer
	content string
}

func NewMockLayer(content string) *MockLayer {
	return &MockLayer{content: content}
}

func (m *MockLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(m.content)), nil
}

func CreateTarWithPackageYAML(packageYAML string) string {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	tw.WriteHeader(&tar.Header{
		Name: StreamFile,
		Mode: 0o644,
		Size: int64(len(packageYAML)),
	})
	tw.Write([]byte(packageYAML))
	tw.Close()

	return buf.String()
}

func NewTestParser(t *testing.T) parser.Parser {
	t.Helper()
	meta, err := BuildMetaScheme()
	if err != nil {
		t.Fatalf("failed to build meta scheme: %v", err)
	}
	obj, err := BuildObjectScheme()
	if err != nil {
		t.Fatalf("failed to build object scheme: %v", err)
	}
	return parser.New(meta, obj)
}

func NewTestPackage(t *testing.T, metaJSON string, objectsJSON ...string) *parser.Package {
	t.Helper()

	p := NewTestParser(t)

	var allJSON strings.Builder
	allJSON.WriteString("---\n")
	allJSON.WriteString(metaJSON)
	for _, objJSON := range objectsJSON {
		allJSON.WriteString("\n---\n")
		allJSON.WriteString(objJSON)
	}

	pkg, err := p.Parse(context.Background(), io.NopCloser(strings.NewReader(allJSON.String())))
	if err != nil {
		t.Fatalf("failed to parse test package: %v", err)
	}

	return pkg
}

func PackageComparer() cmp.Option {
	return cmp.Comparer(func(a, b *parser.Package) bool {
		if a == nil && b == nil {
			return true
		}
		if a == nil || b == nil {
			return false
		}

		if !cmp.Equal(a.GetMeta(), b.GetMeta()) {
			return false
		}

		return cmp.Equal(a.GetObjects(), b.GetObjects())
	})
}

func TestClientGet(t *testing.T) {
	providerMeta := `{"apiVersion":"meta.pkg.crossplane.io/v1","kind":"Provider","metadata":{"name":"provider-aws"}}`
	tarContent := CreateTarWithPackageYAML(providerMeta)

	type args struct {
		ref  string
		opts []GetOption
	}
	type want struct {
		pkg *Package
		err error
	}

	cases := map[string]struct {
		reason string
		client *CachedClient
		args   args
		want   want
	}{
		"SuccessWithTag": {
			reason: "Should successfully fetch and parse a package with a tag reference",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
						return &v1.Descriptor{
							Digest: v1.Hash{
								Algorithm: "sha256",
								Hex:       "abc123def456789012345678901234567890123456789012345678901234abcd",
							},
						}, nil
					},
					MockFetch: func(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
						return &MockImage{
							MockManifest: func() (*v1.Manifest, error) {
								return &v1.Manifest{
									Layers: []v1.Descriptor{
										{
											Annotations: map[string]string{
												AnnotationKey: PackageAnnotation,
											},
											Digest: v1.Hash{Algorithm: "sha256", Hex: "layer123"},
										},
									},
								}, nil
							},
							MockLayerByDigest: func(_ v1.Hash) (v1.Layer, error) {
								return NewMockLayer(tarContent), nil
							},
						}, nil
					},
				},
				parser: NewTestParser(t),
				cache: &MockCache{
					MockGet: func(_ string) (io.ReadCloser, error) {
						return nil, errors.New("not in cache")
					},
					MockStore: func(_ string, rc io.ReadCloser) error {
						_, _ = io.Copy(io.Discard, rc)
						return nil
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, nil
					},
				},
			},
			args: args{
				ref: testSource + ":" + testTag,
			},
			want: want{
				pkg: &Package{
					Package:         NewTestPackage(t, providerMeta),
					Digest:          testDigest,
					Version:         testTag,
					Source:          testSource,
					ResolvedVersion: testTag,
					ResolvedSource:  testSource,
				},
			},
		},
		"SuccessWithDigest": {
			reason: "Should successfully fetch a package with a digest reference without calling Head",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
						return nil, errors.New("Head should not be called for digest refs")
					},
					MockFetch: func(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
						return &MockImage{
							MockManifest: func() (*v1.Manifest, error) {
								return &v1.Manifest{
									Layers: []v1.Descriptor{
										{
											Annotations: map[string]string{
												AnnotationKey: PackageAnnotation,
											},
											Digest: v1.Hash{Algorithm: "sha256", Hex: "layer123"},
										},
									},
								}, nil
							},
							MockLayerByDigest: func(_ v1.Hash) (v1.Layer, error) {
								return NewMockLayer(tarContent), nil
							},
						}, nil
					},
				},
				parser: NewTestParser(t),
				cache: &MockCache{
					MockGet: func(_ string) (io.ReadCloser, error) {
						return nil, errors.New("not in cache")
					},
					MockStore: func(_ string, rc io.ReadCloser) error {
						_, _ = io.Copy(io.Discard, rc)
						return nil
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, nil
					},
				},
			},
			args: args{
				ref: testSource + "@" + testDigest,
			},
			want: want{
				pkg: &Package{
					Package:         NewTestPackage(t, providerMeta),
					Digest:          testDigest,
					Version:         testDigest,
					Source:          testSource,
					ResolvedVersion: testDigest,
					ResolvedSource:  testSource,
				},
			},
		},
		"SuccessFromCache": {
			reason: "Should return cached package without fetching from registry",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
						return &v1.Descriptor{
							Digest: v1.Hash{
								Algorithm: "sha256",
								Hex:       "abc123def456789012345678901234567890123456789012345678901234abcd",
							},
						}, nil
					},
					MockFetch: func(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
						return nil, errors.New("Fetch should not be called when cached")
					},
				},
				parser: NewTestParser(t),
				cache: &MockCache{
					MockGet: func(_ string) (io.ReadCloser, error) {
						return io.NopCloser(strings.NewReader(providerMeta)), nil
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, nil
					},
				},
			},
			args: args{
				ref:  testSource + ":" + testTag,
				opts: []GetOption{WithPullPolicy(corev1.PullIfNotPresent)},
			},
			want: want{
				pkg: &Package{
					Package:         NewTestPackage(t, providerMeta),
					Digest:          testDigest,
					Version:         testTag,
					Source:          testSource,
					ResolvedVersion: testTag,
					ResolvedSource:  testSource,
				},
			},
		},
		"SuccessFromCacheWithPullAlways": {
			reason: "Should return cached package without fetching when PullAlways resolves to a cached digest",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
						return &v1.Descriptor{
							Digest: v1.Hash{
								Algorithm: "sha256",
								Hex:       "abc123def456789012345678901234567890123456789012345678901234abcd",
							},
						}, nil
					},
					MockFetch: func(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
						return nil, errors.New("Fetch should not be called when the resolved digest is cached")
					},
				},
				parser: NewTestParser(t),
				cache: &MockCache{
					MockGet: func(_ string) (io.ReadCloser, error) {
						return io.NopCloser(strings.NewReader(providerMeta)), nil
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "test-verification-config", &v1beta1.ImageVerification{
							Provider: v1beta1.ImageVerificationProviderCosign,
						}, nil
					},
				},
				validator: &MockValidator{
					MockValidate: func(_ context.Context, _ name.Reference, _ *v1beta1.ImageVerification, _ ...string) error {
						return errors.New("signature verification should not run when the resolved digest is cached")
					},
				},
			},
			args: args{
				ref:  testSource + ":" + testTag,
				opts: []GetOption{WithPullPolicy(corev1.PullAlways)},
			},
			want: want{
				pkg: &Package{
					Package:         NewTestPackage(t, providerMeta),
					Digest:          testDigest,
					Version:         testTag,
					Source:          testSource,
					ResolvedVersion: testTag,
					ResolvedSource:  testSource,
				},
			},
		},
		"SuccessWithImageConfigRewrite": {
			reason: "Should use rewritten path from ImageConfig and track which config was applied",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
						return &v1.Descriptor{
							Digest: v1.Hash{
								Algorithm: "sha256",
								Hex:       "abc123def456789012345678901234567890123456789012345678901234abcd",
							},
						}, nil
					},
					MockFetch: func(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
						return &MockImage{
							MockManifest: func() (*v1.Manifest, error) {
								return &v1.Manifest{
									Layers: []v1.Descriptor{
										{
											Annotations: map[string]string{
												AnnotationKey: PackageAnnotation,
											},
											Digest: v1.Hash{Algorithm: "sha256", Hex: "layer123"},
										},
									},
								}, nil
							},
							MockLayerByDigest: func(_ v1.Hash) (v1.Layer, error) {
								return NewMockLayer(tarContent), nil
							},
						}, nil
					},
				},
				parser: NewTestParser(t),
				cache: &MockCache{
					MockGet: func(_ string) (io.ReadCloser, error) {
						return nil, errors.New("not in cache")
					},
					MockStore: func(_ string, rc io.ReadCloser) error {
						_, _ = io.Copy(io.Discard, rc)
						return nil
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "mirror-config", "private-registry.io/mirror/provider-aws:v1.0.0", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, nil
					},
				},
			},
			args: args{
				ref: testSource + ":" + testTag,
			},
			want: want{
				pkg: &Package{
					Package:         NewTestPackage(t, providerMeta),
					Digest:          testDigest,
					Version:         testTag,
					Source:          testSource,
					ResolvedVersion: testTag,
					ResolvedSource:  "private-registry.io/mirror/provider-aws",
					AppliedImageConfigs: []ImageConfig{
						{Name: "mirror-config", Reason: ImageConfigReasonRewrite},
					},
				},
			},
		},
		"SuccessWithImageConfigRewriteAndPullSecret": {
			reason: "Should track both rewrite and pull secret ImageConfigs when both are applied",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
						return &v1.Descriptor{
							Digest: v1.Hash{
								Algorithm: "sha256",
								Hex:       "abc123def456789012345678901234567890123456789012345678901234abcd",
							},
						}, nil
					},
					MockFetch: func(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
						return &MockImage{
							MockManifest: func() (*v1.Manifest, error) {
								return &v1.Manifest{
									Layers: []v1.Descriptor{
										{
											Annotations: map[string]string{
												AnnotationKey: PackageAnnotation,
											},
											Digest: v1.Hash{Algorithm: "sha256", Hex: "layer123"},
										},
									},
								}, nil
							},
							MockLayerByDigest: func(_ v1.Hash) (v1.Layer, error) {
								return NewMockLayer(tarContent), nil
							},
						}, nil
					},
				},
				parser: NewTestParser(t),
				cache: &MockCache{
					MockGet: func(_ string) (io.ReadCloser, error) {
						return nil, errors.New("not in cache")
					},
					MockStore: func(_ string, rc io.ReadCloser) error {
						_, _ = io.Copy(io.Discard, rc)
						return nil
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "mirror-config", "private-registry.io/mirror/provider-aws:v1.0.0", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "secret-config", "registry-secret", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, nil
					},
				},
			},
			args: args{
				ref: testSource + ":" + testTag,
			},
			want: want{
				pkg: &Package{
					Package:         NewTestPackage(t, providerMeta),
					Digest:          testDigest,
					Version:         testTag,
					Source:          testSource,
					ResolvedVersion: testTag,
					ResolvedSource:  "private-registry.io/mirror/provider-aws",
					AppliedImageConfigs: []ImageConfig{
						{Name: "mirror-config", Reason: ImageConfigReasonRewrite},
						{Name: "secret-config", Reason: ImageConfigReasonSetPullSecret},
					},
				},
			},
		},
		"SuccessWithPullAlways": {
			reason: "Should fetch when PullAlways resolves to a digest that isn't cached",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
						return &v1.Descriptor{
							Digest: v1.Hash{
								Algorithm: "sha256",
								Hex:       "abc123def456789012345678901234567890123456789012345678901234abcd",
							},
						}, nil
					},
					MockFetch: func(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
						return &MockImage{
							MockManifest: func() (*v1.Manifest, error) {
								return &v1.Manifest{
									Layers: []v1.Descriptor{
										{
											Annotations: map[string]string{
												AnnotationKey: PackageAnnotation,
											},
											Digest: v1.Hash{Algorithm: "sha256", Hex: "layer123"},
										},
									},
								}, nil
							},
							MockLayerByDigest: func(_ v1.Hash) (v1.Layer, error) {
								return NewMockLayer(tarContent), nil
							},
						}, nil
					},
				},
				parser: NewTestParser(t),
				cache: &MockCache{
					MockGet: func(_ string) (io.ReadCloser, error) {
						return nil, errors.New("not in cache")
					},
					MockStore: func(_ string, rc io.ReadCloser) error {
						_, _ = io.Copy(io.Discard, rc)
						return nil
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, nil
					},
				},
			},
			args: args{
				ref:  testSource + ":" + testTag,
				opts: []GetOption{WithPullPolicy(corev1.PullAlways)},
			},
			want: want{
				pkg: &Package{
					Package:         NewTestPackage(t, providerMeta),
					Digest:          testDigest,
					Version:         testTag,
					Source:          testSource,
					ResolvedVersion: testTag,
					ResolvedSource:  testSource,
				},
			},
		},
		"ErrorPullNeverNotInCache": {
			reason: "Should return error when PullNever is specified and package not in cache",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
						return &v1.Descriptor{
							Digest: v1.Hash{
								Algorithm: "sha256",
								Hex:       "abc123def456789012345678901234567890123456789012345678901234abcd",
							},
						}, nil
					},
					MockFetch: func(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
						return nil, errors.New("Fetch should not be called with PullNever")
					},
				},
				parser: NewTestParser(t),
				cache: &MockCache{
					MockGet: func(_ string) (io.ReadCloser, error) {
						return nil, errors.New("not in cache")
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, nil
					},
				},
			},
			args: args{
				ref:  testSource + ":" + testTag,
				opts: []GetOption{WithPullPolicy(corev1.PullNever)},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorInvalidReference": {
			reason: "Should return error for invalid package reference",
			client: &CachedClient{
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, nil
					},
				},
			},
			args: args{
				ref: "invalid::reference",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorHeadFails": {
			reason: "Should return error when Head request fails",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
						return nil, errors.New("network error")
					},
				},
				cache: &MockCache{
					MockGet: func(_ string) (io.ReadCloser, error) {
						return nil, errors.New("not in cache")
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, nil
					},
				},
			},
			args: args{
				ref: testSource + ":" + testTag,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorFetchFails": {
			reason: "Should return error when Fetch fails",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
						return &v1.Descriptor{
							Digest: v1.Hash{
								Algorithm: "sha256",
								Hex:       "abc123def456789012345678901234567890123456789012345678901234abcd",
							},
						}, nil
					},
					MockFetch: func(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
						return nil, errors.New("fetch failed")
					},
				},
				cache: &MockCache{
					MockGet: func(_ string) (io.ReadCloser, error) {
						return nil, errors.New("not in cache")
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, nil
					},
				},
			},
			args: args{
				ref: testSource + ":" + testTag,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorParseFails": {
			reason: "Should return error when package parsing fails",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
						return &v1.Descriptor{
							Digest: v1.Hash{
								Algorithm: "sha256",
								Hex:       "abc123def456789012345678901234567890123456789012345678901234abcd",
							},
						}, nil
					},
					MockFetch: func(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
						invalidYAML := CreateTarWithPackageYAML("invalid yaml content {{{")
						return &MockImage{
							MockManifest: func() (*v1.Manifest, error) {
								return &v1.Manifest{
									Layers: []v1.Descriptor{
										{
											Annotations: map[string]string{
												AnnotationKey: PackageAnnotation,
											},
											Digest: v1.Hash{Algorithm: "sha256", Hex: "layer123"},
										},
									},
								}, nil
							},
							MockLayerByDigest: func(_ v1.Hash) (v1.Layer, error) {
								return NewMockLayer(invalidYAML), nil
							},
						}, nil
					},
				},
				parser: NewTestParser(t),
				cache: &MockCache{
					MockGet: func(_ string) (io.ReadCloser, error) {
						return nil, errors.New("not in cache")
					},
					MockStore: func(_ string, rc io.ReadCloser) error {
						_, _ = io.Copy(io.Discard, rc)
						return nil
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, nil
					},
				},
			},
			args: args{
				ref: testSource + ":" + testTag,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"SuccessWithVerification": {
			reason: "Should successfully verify and fetch a package when verification config exists",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
						return &v1.Descriptor{
							Digest: v1.Hash{
								Algorithm: "sha256",
								Hex:       "abc123def456789012345678901234567890123456789012345678901234abcd",
							},
						}, nil
					},
					MockFetch: func(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
						return &MockImage{
							MockManifest: func() (*v1.Manifest, error) {
								return &v1.Manifest{
									Layers: []v1.Descriptor{
										{
											Annotations: map[string]string{
												AnnotationKey: PackageAnnotation,
											},
											Digest: v1.Hash{Algorithm: "sha256", Hex: "layer123"},
										},
									},
								}, nil
							},
							MockLayerByDigest: func(_ v1.Hash) (v1.Layer, error) {
								return NewMockLayer(tarContent), nil
							},
						}, nil
					},
				},
				parser: NewTestParser(t),
				cache: &MockCache{
					MockGet: func(_ string) (io.ReadCloser, error) {
						return nil, errors.New("not in cache")
					},
					MockStore: func(_ string, rc io.ReadCloser) error {
						_, _ = io.Copy(io.Discard, rc)
						return nil
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "test-verification-config", &v1beta1.ImageVerification{
							Provider: v1beta1.ImageVerificationProviderCosign,
						}, nil
					},
				},
				validator: &MockValidator{
					MockValidate: func(_ context.Context, _ name.Reference, _ *v1beta1.ImageVerification, _ ...string) error {
						return nil
					},
				},
			},
			args: args{
				ref: testSource + ":" + testTag,
			},
			want: want{
				pkg: &Package{
					Package:         NewTestPackage(t, providerMeta),
					Digest:          testDigest,
					Version:         testTag,
					Source:          testSource,
					ResolvedVersion: testTag,
					ResolvedSource:  testSource,
					AppliedImageConfigs: []ImageConfig{
						{Name: "test-verification-config", Reason: ImageConfigReasonVerify},
					},
				},
			},
		},
		"ErrorVerificationFails": {
			reason: "Should return error when signature verification fails",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
						return &v1.Descriptor{
							Digest: v1.Hash{
								Algorithm: "sha256",
								Hex:       "abc123def456789012345678901234567890123456789012345678901234abcd",
							},
						}, nil
					},
					MockFetch: func(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
						return nil, errors.New("fetch should not be called")
					},
				},
				parser: NewTestParser(t),
				cache: &MockCache{
					MockGet: func(_ string) (io.ReadCloser, error) {
						return nil, errors.New("not in cache")
					},
					MockStore: func(_ string, _ io.ReadCloser) error {
						return nil
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "test-verification-config", &v1beta1.ImageVerification{
							Provider: v1beta1.ImageVerificationProviderCosign,
						}, nil
					},
				},
				validator: &MockValidator{
					MockValidate: func(_ context.Context, _ name.Reference, _ *v1beta1.ImageVerification, _ ...string) error {
						return errors.New("signature verification failed")
					},
				},
			},
			args: args{
				ref: testSource + ":" + testTag,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorRewritePathFails": {
			reason: "Should return error when RewritePath config lookup fails",
			client: &CachedClient{
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", errors.New("cannot list ImageConfigs")
					},
				},
			},
			args: args{
				ref: testSource + ":" + testTag,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorPullSecretForFails": {
			reason: "Should return error when PullSecretFor config lookup fails",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
						return &v1.Descriptor{
							Digest: v1.Hash{
								Algorithm: "sha256",
								Hex:       "abc123def456789012345678901234567890123456789012345678901234abcd",
							},
						}, nil
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", errors.New("cannot list ImageConfigs")
					},
				},
			},
			args: args{
				ref: testSource + ":" + testTag,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorImageVerificationConfigForFails": {
			reason: "Should return error when ImageVerificationConfigFor lookup fails",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
						return &v1.Descriptor{
							Digest: v1.Hash{
								Algorithm: "sha256",
								Hex:       "abc123def456789012345678901234567890123456789012345678901234abcd",
							},
						}, nil
					},
				},
				cache: &MockCache{
					MockGet: func(_ string) (io.ReadCloser, error) {
						return nil, errors.New("not in cache")
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, errors.New("cannot list ImageConfigs")
					},
				},
			},
			args: args{
				ref: testSource + ":" + testTag,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.client.Get(context.Background(), tc.args.ref, tc.args.opts...)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGet(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.pkg, got, PackageComparer()); diff != "" {
				t.Errorf("\n%s\nGet(...): -want Package, +got Package:\n%s", tc.reason, diff)
			}
		})
	}
}

// truncatedLayer is a package layer whose stream fails with
// io.ErrUnexpectedEOF after its content, like a download that dies partway.
type truncatedLayer struct {
	v1.Layer
	content string
}

func (l *truncatedLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(io.MultiReader(strings.NewReader(l.content), iotest.ErrReader(io.ErrUnexpectedEOF))), nil
}

func TestClientGetCachesOnlyCompletePackages(t *testing.T) {
	providerMeta := `{"apiVersion":"meta.pkg.crossplane.io/v1","kind":"Provider","metadata":{"name":"provider-aws"}}`
	crd := `{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinition","metadata":{"name":"%s.example.org"}}`
	names := []string{"aaa", "bbb", "ccc", "ddd"}

	docs := make([]string, 0, 1+len(names))
	docs = append(docs, providerMeta)
	for _, n := range names {
		docs = append(docs, fmt.Sprintf(crd, n))
	}
	packageYAML := "---\n" + strings.Join(docs, "\n---\n") + "\n"
	tarContent := CreateTarWithPackageYAML(packageYAML)

	// The package.yaml body starts after the 512 byte tar header.
	const tarHeaderSize = 512
	docBoundary := tarHeaderSize + strings.Index(packageYAML, "\n---\n"+fmt.Sprintf(crd, "ccc")) + len("\n---\n")
	midString := tarHeaderSize + strings.Index(packageYAML, `"ccc.exa`) + len(`"ccc.exa`)

	image := func(layer v1.Layer) func(context.Context, name.Reference, ...string) (v1.Image, error) {
		return func(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
			return &MockImage{
				MockManifest: func() (*v1.Manifest, error) {
					return &v1.Manifest{
						Layers: []v1.Descriptor{{
							Annotations: map[string]string{AnnotationKey: PackageAnnotation},
							Digest:      v1.Hash{Algorithm: "sha256", Hex: "layer123"},
						}},
					}, nil
				},
				MockLayerByDigest: func(_ v1.Hash) (v1.Layer, error) {
					return layer, nil
				},
			}, nil
		}
	}

	newClient := func(cache PackageCache, fetch func(context.Context, name.Reference, ...string) (v1.Image, error)) *CachedClient {
		return &CachedClient{
			fetcher: &MockFetcher{MockFetch: fetch},
			parser:  NewTestParser(t),
			cache:   cache,
			config: &MockConfigStore{
				MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
					return "", "", nil
				},
				MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
					return "", "", nil
				},
				MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
					return "", nil, nil
				},
			},
		}
	}

	type args struct {
		layer v1.Layer
	}
	type want struct {
		fetchErr     error
		cachedFiles  int
		cacheHitErr  error
		cacheObjects int
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"CompletePackage": {
			reason: "A package that downloads and parses successfully should be cached in full.",
			args:   args{layer: NewMockLayer(tarContent)},
			want:   want{cachedFiles: 1, cacheObjects: len(names)},
		},
		"TruncatedAtDocumentBoundary": {
			reason: "A download that fails between two YAML documents must not be cached, because the prefix is itself a valid YAML stream.",
			args:   args{layer: &truncatedLayer{content: tarContent[:docBoundary]}},
			want:   want{fetchErr: cmpopts.AnyError, cacheHitErr: cmpopts.AnyError},
		},
		"TruncatedMidString": {
			reason: "A download that fails in the middle of a value must not be cached, because the prefix parses as a partial object.",
			args:   args{layer: &truncatedLayer{content: tarContent[:midString]}},
			want:   want{fetchErr: cmpopts.AnyError, cacheHitErr: cmpopts.AnyError},
		},
	}

	for caseName, tc := range cases {
		t.Run(caseName, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			cache := NewFsPackageCache("/cache", fs)
			ref := testSource + "@" + testDigest

			_, err := newClient(cache, image(tc.args.layer)).Get(context.Background(), ref)
			if diff := cmp.Diff(tc.want.fetchErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGet(...) with fetch: -want error, +got error:\n%s", tc.reason, diff)
			}

			files, _ := afero.ReadDir(fs, "/cache")
			if diff := cmp.Diff(tc.want.cachedFiles, len(files)); diff != "" {
				t.Errorf("\n%s\nfiles in cache: -want, +got:\n%s", tc.reason, diff)
			}

			// With the registry unavailable, Get can only succeed from the cache.
			unavailable := func(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
				return nil, errors.New("registry unavailable")
			}
			got, err := newClient(cache, unavailable).Get(context.Background(), ref)
			if diff := cmp.Diff(tc.want.cacheHitErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGet(...) from cache: -want error, +got error:\n%s", tc.reason, diff)
			}

			objects := 0
			if got != nil {
				objects = len(got.GetObjects())
			}
			if diff := cmp.Diff(tc.want.cacheObjects, objects); diff != "" {
				t.Errorf("\n%s\nGet(...) from cache: -want objects, +got objects:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestClientGetPullAlwaysTagMove(t *testing.T) {
	meta := `{"apiVersion":"meta.pkg.crossplane.io/v1","kind":"Provider","metadata":{"name":"%s"}}`
	digestA := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("a", 64)}
	digestB := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("b", 64)}

	// The tag resolves to whatever digest it currently points at, and each
	// digest serves a package with a different name.
	tagDigest := digestA
	packages := map[v1.Hash]string{
		digestA: fmt.Sprintf(meta, "provider-a"),
		digestB: fmt.Sprintf(meta, "provider-b"),
	}
	var fetched []v1.Hash

	c := &CachedClient{
		fetcher: &MockFetcher{
			MockHead: func(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
				return &v1.Descriptor{Digest: tagDigest}, nil
			},
			MockFetch: func(_ context.Context, ref name.Reference, _ ...string) (v1.Image, error) {
				d, err := v1.NewHash(ref.Identifier())
				if err != nil {
					return nil, errors.Wrapf(err, "expected a fetch by digest, got %q", ref)
				}
				fetched = append(fetched, d)
				content := CreateTarWithPackageYAML(packages[d])
				return &MockImage{
					MockManifest: func() (*v1.Manifest, error) {
						return &v1.Manifest{
							Layers: []v1.Descriptor{{
								Annotations: map[string]string{AnnotationKey: PackageAnnotation},
								Digest:      v1.Hash{Algorithm: "sha256", Hex: "layer123"},
							}},
						}, nil
					},
					MockLayerByDigest: func(_ v1.Hash) (v1.Layer, error) {
						return NewMockLayer(content), nil
					},
				}, nil
			},
		},
		parser: NewTestParser(t),
		cache:  NewFsPackageCache("/cache", afero.NewMemMapFs()),
		config: &MockConfigStore{
			MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
				return "", "", nil
			},
			MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
				return "", "", nil
			},
			MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
				return "", nil, nil
			},
		},
	}

	type args struct {
		tagDigest v1.Hash
	}
	type want struct {
		name    string
		digest  v1.Hash
		fetched []v1.Hash
	}

	steps := []struct {
		reason string
		args   args
		want   want
	}{
		{
			reason: "The first Get should fetch the package the tag points at.",
			args:   args{tagDigest: digestA},
			want: want{
				name:    "provider-a",
				digest:  digestA,
				fetched: []v1.Hash{digestA},
			},
		},
		{
			reason: "A second Get with an unchanged digest should be served from the cache.",
			args:   args{tagDigest: digestA},
			want: want{
				name:    "provider-a",
				digest:  digestA,
				fetched: []v1.Hash{digestA},
			},
		},
		{
			reason: "After the tag moves, Get should fetch the new digest rather than return the cached package.",
			args:   args{tagDigest: digestB},
			want: want{
				name:    "provider-b",
				digest:  digestB,
				fetched: []v1.Hash{digestA, digestB},
			},
		},
		{
			reason: "Moving the tag back should hit the cache entry for the original digest.",
			args:   args{tagDigest: digestA},
			want: want{
				name:    "provider-a",
				digest:  digestA,
				fetched: []v1.Hash{digestA, digestB},
			},
		},
	}

	for i, step := range steps {
		tagDigest = step.args.tagDigest

		got, err := c.Get(context.Background(), testSource+":"+testTag, WithPullPolicy(corev1.PullAlways))
		if err != nil {
			t.Fatalf("step %d: %s\nGet(...): unexpected error: %v", i, step.reason, err)
		}

		gotName := ""
		if m := got.GetMeta(); m != nil {
			gotName = m.GetName()
		}
		if diff := cmp.Diff(step.want.name, gotName); diff != "" {
			t.Errorf("step %d: %s\nGet(...): -want package name, +got package name:\n%s", i, step.reason, diff)
		}
		if diff := cmp.Diff(step.want.digest.String(), got.Digest); diff != "" {
			t.Errorf("step %d: %s\nGet(...): -want digest, +got digest:\n%s", i, step.reason, diff)
		}
		if diff := cmp.Diff(step.want.fetched, fetched); diff != "" {
			t.Errorf("step %d: %s\nfetched digests: -want, +got:\n%s", i, step.reason, diff)
		}
	}
}

func TestClientListVersions(t *testing.T) {
	type args struct {
		source string
		opts   []GetOption
	}
	type want struct {
		versions []string
		err      error
	}

	cases := map[string]struct {
		reason string
		client *CachedClient
		args   args
		want   want
	}{
		"Success": {
			reason: "Should successfully list and filter versions",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockTags: func(_ context.Context, _ name.Reference, _ ...string) ([]string, error) {
						return []string{"v1.0.0", "v1.1.0", "v2.0.0", "latest", "main"}, nil
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, nil
					},
				},
			},
			args: args{
				source: testSource,
			},
			want: want{
				versions: []string{"v1.0.0", "v1.1.0", "v2.0.0"},
			},
		},
		"SuccessWithImageConfigRewrite": {
			reason: "Should use rewritten path from ImageConfig",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockTags: func(_ context.Context, _ name.Reference, _ ...string) ([]string, error) {
						return []string{"v1.0.0"}, nil
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "private-registry.io/mirror/provider-aws", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, nil
					},
				},
			},
			args: args{
				source: testSource,
			},
			want: want{
				versions: []string{"v1.0.0"},
			},
		},
		"ErrorInvalidSource": {
			reason: "Should return error for invalid source",
			client: &CachedClient{
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, nil
					},
				},
			},
			args: args{
				source: "invalid::source",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorTagsFails": {
			reason: "Should return error when Tags request fails",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockTags: func(_ context.Context, _ name.Reference, _ ...string) ([]string, error) {
						return nil, errors.New("network error")
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockImageVerificationConfigFor: func(_ context.Context, _ string) (string, *v1beta1.ImageVerification, error) {
						return "", nil, nil
					},
				},
			},
			args: args{
				source: testSource,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorRewritePathFails": {
			reason: "Should return error when RewritePath config lookup fails",
			client: &CachedClient{
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", errors.New("cannot list ImageConfigs")
					},
				},
			},
			args: args{
				source: testSource,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorPullSecretForFails": {
			reason: "Should return error when PullSecretFor config lookup fails",
			client: &CachedClient{
				fetcher: &MockFetcher{
					MockTags: func(_ context.Context, _ name.Reference, _ ...string) ([]string, error) {
						return []string{"v1.0.0"}, nil
					},
				},
				config: &MockConfigStore{
					MockRewritePath: func(_ context.Context, _ string) (string, string, error) {
						return "", "", nil
					},
					MockPullSecretFor: func(_ context.Context, _ string) (string, string, error) {
						return "", "", errors.New("cannot list ImageConfigs")
					},
				},
			},
			args: args{
				source: testSource,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.client.ListVersions(context.Background(), tc.args.source, tc.args.opts...)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nListVersions(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.versions, got); diff != "" {
				t.Errorf("\n%s\nListVersions(...): -want versions, +got versions:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestExtractPackageYAML(t *testing.T) {
	providerMeta := `{"apiVersion":"meta.pkg.crossplane.io/v1","kind":"Provider","metadata":{"name":"provider-aws"}}`
	tarContent := CreateTarWithPackageYAML(providerMeta)

	type want struct {
		content string
		err     error
	}

	cases := map[string]struct {
		reason string
		img    v1.Image
		want   want
	}{
		"SuccessWithAnnotatedLayer": {
			reason: "Should extract package.yaml from annotated layer",
			img: &MockImage{
				MockManifest: func() (*v1.Manifest, error) {
					return &v1.Manifest{
						Layers: []v1.Descriptor{
							{
								Annotations: map[string]string{
									AnnotationKey: PackageAnnotation,
								},
								Digest: v1.Hash{Algorithm: "sha256", Hex: "layer123"},
							},
						},
					}, nil
				},
				MockLayerByDigest: func(_ v1.Hash) (v1.Layer, error) {
					return NewMockLayer(tarContent), nil
				},
			},
			want: want{
				content: providerMeta,
			},
		},
		"SuccessWithoutAnnotatedLayer": {
			reason: "Should fall back to flattened extraction when no annotated layer",
			img: &MockImage{
				MockManifest: func() (*v1.Manifest, error) {
					return &v1.Manifest{
						Layers: []v1.Descriptor{
							{
								Annotations: nil,
								Digest:      v1.Hash{Algorithm: "sha256", Hex: "layer123"},
							},
						},
					}, nil
				},
				MockLayers: func() ([]v1.Layer, error) {
					return []v1.Layer{NewMockLayer(tarContent)}, nil
				},
			},
			want: want{
				content: providerMeta,
			},
		},
		"ErrorManifestFails": {
			reason: "Should return error when manifest retrieval fails",
			img: &MockImage{
				MockManifest: func() (*v1.Manifest, error) {
					return nil, errors.New("manifest error")
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorLayerByDigestFails": {
			reason: "Should return error when layer retrieval fails",
			img: &MockImage{
				MockManifest: func() (*v1.Manifest, error) {
					return &v1.Manifest{
						Layers: []v1.Descriptor{
							{
								Annotations: map[string]string{
									AnnotationKey: PackageAnnotation,
								},
								Digest: v1.Hash{Algorithm: "sha256", Hex: "layer123"},
							},
						},
					}, nil
				},
				MockLayerByDigest: func(_ v1.Hash) (v1.Layer, error) {
					return nil, errors.New("layer error")
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorPackageYAMLNotFound": {
			reason: "Should return error when package.yaml is not in tar",
			img: &MockImage{
				MockManifest: func() (*v1.Manifest, error) {
					return &v1.Manifest{
						Layers: []v1.Descriptor{
							{
								Annotations: map[string]string{
									AnnotationKey: PackageAnnotation,
								},
								Digest: v1.Hash{Algorithm: "sha256", Hex: "layer123"},
							},
						},
					}, nil
				},
				MockLayerByDigest: func(_ v1.Hash) (v1.Layer, error) {
					var buf bytes.Buffer
					tw := tar.NewWriter(&buf)
					tw.WriteHeader(&tar.Header{
						Name: "other-file.txt",
						Mode: 0o644,
						Size: 5,
					})
					tw.Write([]byte("hello"))
					tw.Close()
					return NewMockLayer(buf.String()), nil
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ExtractPackageYAML(tc.img)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nExtractPackageYAML(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if err != nil {
				return
			}

			defer got.Close()
			content, err := io.ReadAll(got)
			if err != nil {
				t.Fatalf("\n%s\nExtractPackageYAML(...): failed to read content: %v", tc.reason, err)
			}

			if diff := cmp.Diff(tc.want.content, string(content)); diff != "" {
				t.Errorf("\n%s\nExtractPackageYAML(...): -want content, +got content:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFilterAndSortVersions(t *testing.T) {
	type args struct {
		tags []string
	}
	type want struct {
		versions []string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should filter non-semver tags and sort ascending",
			args: args{
				tags: []string{"v2.0.0", "latest", "v1.0.0", "main", "v1.1.0"},
			},
			want: want{
				versions: []string{"v1.0.0", "v1.1.0", "v2.0.0"},
			},
		},
		"EmptyInput": {
			reason: "Should handle empty input",
			args: args{
				tags: []string{},
			},
			want: want{
				versions: []string{},
			},
		},
		"NoValidVersions": {
			reason: "Should return empty slice when no valid semver tags",
			args: args{
				tags: []string{"latest", "main", "dev"},
			},
			want: want{
				versions: []string{},
			},
		},
		"PreReleaseVersions": {
			reason: "Should include pre-release versions",
			args: args{
				tags: []string{"v1.0.0", "v1.1.0-alpha", "v1.1.0-beta", "v1.1.0"},
			},
			want: want{
				versions: []string{"v1.0.0", "v1.1.0-alpha", "v1.1.0-beta", "v1.1.0"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := FilterAndSortVersions(tc.args.tags)

			if diff := cmp.Diff(tc.want.versions, got); diff != "" {
				t.Errorf("\n%s\nFilterAndSortVersions(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPackageDigestHex(t *testing.T) {
	const testHex = "abc123def456789012345678901234567890123456789012345678901234abcd"

	cases := map[string]struct {
		reason string
		digest string
		want   string
	}{
		"ValidDigest": {
			reason: "Should return hex part of valid SHA256 digest",
			digest: "sha256:" + testHex,
			want:   testHex,
		},
		"InvalidDigest": {
			reason: "Should return empty string for invalid digest",
			digest: "invalid-digest",
			want:   "",
		},
		"EmptyDigest": {
			reason: "Should return empty string for empty digest",
			digest: "",
			want:   "",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pkg := &Package{Digest: tc.digest}
			got := pkg.DigestHex()

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nPackage.DigestHex(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
