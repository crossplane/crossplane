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
	"context"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver"
	ociname "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	corev1 "k8s.io/api/core/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/parser"

	pkgmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/v2/internal/xpkg/signature"
)

// Client is a client for fetching and parsing Crossplane packages.
type Client interface {
	// Get fetches and parses a complete package from the given reference.
	// The ref parameter is a package reference (e.g.,
	// "registry.io/org/package:v1.0.0" or "registry.io/org/package@sha256:...").
	//
	// Caching and ImageConfig path rewriting are handled transparently.
	Get(ctx context.Context, ref string, opts ...GetOption) (*Package, error)

	// ListVersions returns available versions for a package source.
	// The source parameter is the package path without tag/digest
	// (e.g., "registry.io/org/package").
	//
	// Honors ImageConfig path rewriting when listing versions.
	ListVersions(ctx context.Context, source string, opts ...GetOption) ([]string, error)
}

// ImageConfig represents an ImageConfig that was applied during package fetch.
type ImageConfig struct {
	Name   string
	Reason ImageConfigReason
}

// ImageConfigReason describes why an ImageConfig was applied.
type ImageConfigReason string

const (
	// ImageConfigReasonRewrite indicates the ImageConfig rewrote the image
	// path.
	ImageConfigReasonRewrite ImageConfigReason = "RewriteImage"

	// ImageConfigReasonSetPullSecret indicates the ImageConfig provided a
	// pull secret.
	ImageConfigReasonSetPullSecret ImageConfigReason = "SetImagePullSecret"

	// ImageConfigReasonVerify indicates the ImageConfig provided signature
	// verification config.
	ImageConfigReasonVerify ImageConfigReason = "VerifyImage"
)

// SupportedImageConfigs returns the ImageConfigReasons that Client may return.
// Callers tracking applied ImageConfigs should clear all of these before
// setting the ones returned by Client.Get, to handle the case where a
// previously-applied ImageConfig no longer matches.
func SupportedImageConfigs() []ImageConfigReason {
	return []ImageConfigReason{
		ImageConfigReasonRewrite,
		ImageConfigReasonSetPullSecret,
		ImageConfigReasonVerify,
	}
}

// maxPackageSize is the maximum size of a package.yaml file that can be parsed.
// This limit prevents denial of service attacks via maliciously large packages.
const maxPackageSize = 200 << 20 // 200 MB

// Package represents a successfully fetched package with all its content.
type Package struct {
	*parser.Package

	// Digest is the immutable content identifier (sha256 from OCI image).
	Digest string

	// Version is the package version, either a semver tag (v1.0.0) or digest
	// (sha256:abc123). This is extracted from the original reference used to
	// fetch the package.
	Version string

	// Source is the package source without tag/digest, normalized.
	// This is the ORIGINAL source before any ImageConfig rewriting.
	Source string

	// ResolvedVersion is the package version after ImageConfig rewriting.
	// May differ from Version if an ImageConfig rewrote the tag/digest.
	ResolvedVersion string

	// ResolvedSource is the source after ImageConfig path rewriting.
	// May be the same as Source if no rewriting occurred.
	ResolvedSource string

	// AppliedImageConfigs tracks which ImageConfigs were applied during fetch.
	AppliedImageConfigs []ImageConfig
}

// DigestHex returns the hex string of the digest without the algorithm prefix.
// Returns empty string if the digest cannot be parsed.
func (p *Package) DigestHex() string {
	hash, err := v1.NewHash(p.Digest)
	if err != nil {
		return ""
	}
	return hash.Hex
}

// GetMeta returns the package metadata object.
// Returns nil if the package doesn't contain exactly one metadata object.
func (p *Package) GetMeta() pkgmetav1.Pkg {
	meta := p.Package.GetMeta()
	if len(meta) != 1 {
		return nil
	}

	pkg, _ := TryConvertToPkg(meta[0], &pkgmetav1.Provider{}, &pkgmetav1.Configuration{}, &pkgmetav1.Function{})
	return pkg
}

// GetDependencies returns the package dependencies from metadata.
// Returns nil if metadata cannot be extracted.
func (p *Package) GetDependencies() []pkgmetav1.Dependency {
	meta := p.GetMeta()
	if meta == nil {
		return nil
	}
	return meta.GetDependencies()
}

// Ref returns the full original package reference (Source + Version).
func (p *Package) Ref() string {
	return BuildPackageRef(p.Source, p.Version)
}

// ResolvedRef returns the full resolved package reference after ImageConfig
// rewriting (ResolvedSource + ResolvedVersion).
func (p *Package) ResolvedRef() string {
	return BuildPackageRef(p.ResolvedSource, p.ResolvedVersion)
}

// BuildPackageRef combines a source and version into a full package reference.
// Uses "@" for digests (version contains ":") and ":" for tags.
func BuildPackageRef(source, version string) string {
	if strings.Contains(version, ":") {
		return source + "@" + version
	}
	return source + ":" + version
}

// GetOption configures per-request package fetching behavior.
type GetOption func(*GetConfig)

// WithPullSecrets specifies secrets for authenticating to private registries.
// These are combined with any pull secrets from ImageConfig.
func WithPullSecrets(secrets ...string) GetOption {
	return func(c *GetConfig) {
		c.pullSecrets = secrets
	}
}

// WithPullPolicy specifies when to fetch from the registry vs use cache.
// Default is IfNotPresent.
func WithPullPolicy(policy corev1.PullPolicy) GetOption {
	return func(c *GetConfig) {
		c.pullPolicy = policy
	}
}

// GetConfig configures the client's Get method.
type GetConfig struct {
	pullSecrets []string
	pullPolicy  corev1.PullPolicy
}

// CachedClient implements Client with caching support.
type CachedClient struct {
	fetcher   Fetcher
	parser    parser.Parser
	cache     PackageCache
	config    ConfigStore
	validator signature.Validator
}

// NewCachedClient creates a new package client.
func NewCachedClient(f Fetcher, p parser.Parser, c PackageCache, s ConfigStore, v signature.Validator) *CachedClient {
	return &CachedClient{
		fetcher:   f,
		parser:    p,
		cache:     c,
		config:    s,
		validator: v,
	}
}

// Get fetches and parses a complete package.
func (c *CachedClient) Get(ctx context.Context, ref string, opts ...GetOption) (*Package, error) {
	cfg := &GetConfig{
		pullPolicy: corev1.PullIfNotPresent,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	originalRef := ref
	resolvedRef := ref

	var applied []ImageConfig

	name, rewritten, err := c.config.RewritePath(ctx, ref)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get image rewrite config")
	}
	if rewritten != "" {
		resolvedRef = rewritten
		applied = append(applied, ImageConfig{Name: name, Reason: ImageConfigReasonRewrite})
	}

	parsedOriginalRef, err := ociname.ParseReference(originalRef)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse package reference %s", originalRef)
	}

	parsedResolvedRef, err := ociname.ParseReference(resolvedRef)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse resolved package reference %s", resolvedRef)
	}

	secrets := cfg.pullSecrets

	name, secret, err := c.config.PullSecretFor(ctx, ref)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get image pull secret config")
	}
	if secret != "" {
		secrets = append(secrets, secret)
		applied = append(applied, ImageConfig{Name: name, Reason: ImageConfigReasonSetPullSecret})
	}

	var digest string
	if d, ok := parsedResolvedRef.(ociname.Digest); ok {
		digest = d.Identifier()
	} else {
		desc, err := c.fetcher.Head(ctx, parsedResolvedRef, secrets...)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot resolve %s to digest", parsedResolvedRef.String())
		}
		digest = desc.Digest.String()
	}

	cacheKey := FriendlyID(ParsePackageSourceFromReference(parsedOriginalRef), digest)

	if cfg.pullPolicy != corev1.PullAlways {
		rc, err := c.cache.Get(cacheKey)
		if err == nil {
			pkg, err := c.parser.Parse(ctx, struct {
				io.Reader
				io.Closer
			}{
				Reader: io.LimitReader(rc, maxPackageSize),
				Closer: rc,
			})
			rc.Close() //nolint:errcheck // Only open for reading.
			if err == nil {
				return &Package{
					Package:             pkg,
					Digest:              digest,
					Version:             parsedOriginalRef.Identifier(),
					Source:              ParsePackageSourceFromReference(parsedOriginalRef),
					ResolvedVersion:     parsedResolvedRef.Identifier(),
					ResolvedSource:      ParsePackageSourceFromReference(parsedResolvedRef),
					AppliedImageConfigs: applied,
				}, nil
			}
		}
	}

	if cfg.pullPolicy == corev1.PullNever {
		return nil, errors.New("package not in cache and pull policy is Never")
	}

	// Verification only happens if we don't get a cache hit. This means we
	// won't verify packages if verification is enabled _after_ the package
	// was fetched and cached.
	//
	// We cache gzipped package.yaml files from the OCI image. We can't
	// cryptographically guarantee the cached files are the ones from the
	// verified package - e.g. if someone manually populated the cache. So
	// we're making a trade-off here. The only way to guarantee we're using
	// a verified OCI image on every reconcile would be to disable caching,
	// or cache the entire OCI image.
	name, vc, err := c.config.ImageVerificationConfigFor(ctx, resolvedRef)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get image verification config")
	}
	if vc != nil {
		ref := parsedResolvedRef.Context().Digest(digest)
		if err := c.validator.Validate(ctx, ref, vc, secrets...); err != nil {
			return nil, errors.Wrap(err, "signature verification failed")
		}
		applied = append(applied, ImageConfig{Name: name, Reason: ImageConfigReasonVerify})
	}

	img, err := c.fetcher.Fetch(ctx, parsedResolvedRef, secrets...)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot fetch package %s", resolvedRef)
	}

	rc, err := ExtractPackageYAML(img)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot extract package content from %s", resolvedRef)
	}

	pipeR, pipeW := io.Pipe()
	teeRC := TeeReadCloser(rc, pipeW)
	defer teeRC.Close() //nolint:errcheck // Would only error if we called pipeW.CloseWithError()

	go func() {
		defer pipeR.Close() //nolint:errcheck // Only open for reading.
		_ = c.cache.Store(cacheKey, pipeR)
	}()

	pkg, err := c.parser.Parse(ctx, struct {
		io.Reader
		io.Closer
	}{
		Reader: io.LimitReader(teeRC, maxPackageSize),
		Closer: teeRC,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse package %s", resolvedRef)
	}

	return &Package{
		Package:             pkg,
		Digest:              digest,
		Version:             parsedOriginalRef.Identifier(),
		Source:              ParsePackageSourceFromReference(parsedOriginalRef),
		ResolvedVersion:     parsedResolvedRef.Identifier(),
		ResolvedSource:      ParsePackageSourceFromReference(parsedResolvedRef),
		AppliedImageConfigs: applied,
	}, nil
}

// ListVersions returns available versions for a package source.
func (c *CachedClient) ListVersions(ctx context.Context, source string, opts ...GetOption) ([]string, error) {
	cfg := &GetConfig{
		pullPolicy: corev1.PullIfNotPresent,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	resolvedSource := source

	_, rewritten, err := c.config.RewritePath(ctx, source)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get image rewrite config")
	}
	if rewritten != "" {
		resolvedSource = rewritten
	}

	ref, err := ociname.ParseReference(resolvedSource)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse package source %s", resolvedSource)
	}

	secrets := cfg.pullSecrets
	_, secret, err := c.config.PullSecretFor(ctx, source)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get image pull secret config")
	}
	if secret != "" {
		secrets = append(secrets, secret)
	}

	tags, err := c.fetcher.Tags(ctx, ref, secrets...)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot list tags for %s", resolvedSource)
	}

	return FilterAndSortVersions(tags), nil
}

// ExtractPackageYAML extracts the package.yaml file from an OCI image.
// It looks for the annotated package layer (io.crossplane.xpkg: base) and
// falls back to the flattened filesystem from all layers if no annotation
// is found, per the xpkg specification.
func ExtractPackageYAML(img v1.Image) (io.ReadCloser, error) {
	manifest, err := img.Manifest()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get image manifest")
	}

	var tarc io.ReadCloser
	for _, l := range manifest.Layers {
		if l.Annotations[AnnotationKey] != PackageAnnotation {
			continue
		}

		layer, err := img.LayerByDigest(l.Digest)
		if err != nil {
			return nil, errors.Wrap(err, "cannot get annotated layer")
		}

		tarc, err = layer.Uncompressed()
		if err != nil {
			return nil, errors.Wrap(err, "cannot uncompress layer")
		}
		break // Only one annotated layer expected per xpkg spec.
	}

	if tarc == nil {
		tarc = mutate.Extract(img)
	}

	t := tar.NewReader(tarc)

	for {
		h, err := t.Next()
		if err == io.EOF {
			return nil, errors.New("package.yaml not found in package")
		}
		if err != nil {
			return nil, errors.Wrap(err, "cannot read package contents")
		}

		if filepath.Base(h.Name) == StreamFile {
			break
		}
	}

	return JoinedReadCloser(t, tarc), nil
}

// FilterAndSortVersions filters tags to valid semver versions and sorts them
// in ascending order (oldest first).
func FilterAndSortVersions(tags []string) []string {
	versions := make([]*semver.Version, 0, len(tags))
	for _, tag := range tags {
		v, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}
		versions = append(versions, v)
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].LessThan(versions[j])
	})

	result := make([]string, len(versions))
	for i, v := range versions {
		result[i] = v.Original()
	}

	return result
}
