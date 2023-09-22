/*
Copyright 2023 The Crossplane Authors.

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

package image

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/Masterminds/semver"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

const (
	// DefaultVer effectively defines latest for the semver constraints
	DefaultVer = ">=v0.0.0"

	packageTagFmt = "%s:%s"

	errInvalidConstraint  = "invalid dependency constraint"
	errInvalidProviderRef = "invalid package reference"
	errFailedToFetchTags  = "failed to fetch tags"
	errNoMatchingVersion  = "supplied version does not match an existing version"
	errTagDoesNotExist    = "supplied tag does not exist in the registry"
)

// Resolver --
type Resolver struct {
	f Fetcher
}

// Fetcher defines how we expect to intract with the Image repository.
type Fetcher interface {
	Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error)
	Head(ctx context.Context, ref name.Reference, secrets ...string) (*v1.Descriptor, error)
	Tags(ctx context.Context, ref name.Reference, secrets ...string) ([]string, error)
}

// NewResolver returns a new Resolver.
func NewResolver(opts ...ResolverOption) *Resolver {
	r := &Resolver{
		f: NewLocalFetcher(),
	}

	for _, o := range opts {
		o(r)
	}
	return r
}

// ResolverOption modifies the image resolver.
type ResolverOption func(*Resolver)

// WithFetcher modifies the Resolver and adds the given fetcher.
func WithFetcher(f Fetcher) ResolverOption {
	return func(r *Resolver) {
		r.f = f
	}
}

// ResolveImage resolves the image corresponding to the given v1beta1.Dependency.
func (r *Resolver) ResolveImage(ctx context.Context, dep v1beta1.Dependency) (string, v1.Image, error) {

	tag, err := r.ResolveTag(ctx, dep)
	if err != nil {
		return "", nil, errors.Errorf("failed to resolve %s:%s: %w", dep.Package, dep.Constraints, err)
	}

	remoteImageRef, err := name.ParseReference(FullTag(v1beta1.Dependency{
		Package:     dep.Package,
		Type:        dep.Type,
		Constraints: tag,
	}))
	if err != nil {
		return "", nil, err
	}

	i, err := r.f.Fetch(ctx, remoteImageRef)
	return tag, i, err
}

// ResolveTag resolves the tag corresponding to the given v1beta1.Dependency.
// TODO(@tnthornton) add a test that flexes resolving constraint versions to the expected target version
func (r *Resolver) ResolveTag(ctx context.Context, dep v1beta1.Dependency) (string, error) { //nolint:gocyclo
	// if the passed in version was blank use the default to pass
	// constraint checks and grab latest semver
	if dep.Constraints == "" {
		dep.Constraints = DefaultVer
	}

	// check up front if we already have a valid semantic version
	v, err := semver.NewVersion(dep.Constraints)
	if err != nil && !errors.Is(err, semver.ErrInvalidSemVer) {
		return "", err
	}

	if v != nil {
		// version is a valid semantic version, check if it's a real tag
		_, err := r.ResolveDigest(ctx, dep)
		if err != nil {
			return "", err
		}
		return dep.Constraints, nil
	}

	// supplied version may be a semantic version constraint
	c, err := semver.NewConstraint(dep.Constraints)
	if err != nil {
		return "", errors.Wrap(err, errInvalidConstraint)
	}

	ref, err := name.ParseReference(dep.Identifier())
	if err != nil {
		return "", errors.Wrap(err, errInvalidProviderRef)
	}

	tags, err := r.f.Tags(ctx, ref)
	if err != nil {
		return "", errors.Wrap(err, errFailedToFetchTags)
	}

	vs := []*semver.Version{}
	for _, r := range tags {
		v, err := semver.NewVersion(r)
		if err != nil {
			// We skip any tags that are not valid semantic versions.
			//
			// TODO @(tnthornton) we should verify this is the behavior we
			// want long term - i.e. should we care if an end user chooses
			// not to tag their packages with semver?
			continue
		}
		vs = append(vs, v)
	}

	sort.Sort(semver.Collection(vs))
	var ver string
	for _, v := range vs {
		if c.Check(v) {
			ver = v.Original()
		}
	}

	if ver == "" {
		return "", errors.New(errNoMatchingVersion)
	}

	return ver, nil
}

// ResolveDigest performs a head request to the configured registry in order to determine
// if the provided version corresponds to a real tag and what the digest of that tag is.
func (r *Resolver) ResolveDigest(ctx context.Context, d v1beta1.Dependency) (string, error) {
	ref, err := name.ParseReference(d.Identifier(), name.WithDefaultTag(d.Constraints))
	if err != nil {
		return "", errors.Wrap(err, errInvalidProviderRef)
	}

	desc, err := r.f.Head(ctx, ref)
	if err != nil {
		var e *transport.Error
		if errors.As(err, &e) {
			if e.StatusCode == http.StatusNotFound {
				// couldn't find the specified tag, it appears to be invalid
				return "", errors.New(errTagDoesNotExist)
			}
		}
		return "", err
	}
	return desc.Digest.String(), nil
}

// FullTag returns the full image tag "source:version" of the given dependency
func FullTag(d v1beta1.Dependency) string {
	// NOTE(@tnthornton) this should ONLY be used after the version constraint
	// has been resolved for the given dependency. Using a semver range is not
	// a valid tag format and will cause lookups to this string to fail.
	return fmt.Sprintf(packageTagFmt, d.Package, d.Constraints)
}
