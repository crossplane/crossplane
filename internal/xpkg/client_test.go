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
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
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
			reason: "Should bypass cache when PullAlways is specified",
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
						return nil, errors.New("cache should not be checked with PullAlways")
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
