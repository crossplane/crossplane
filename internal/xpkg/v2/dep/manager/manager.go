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

// Package manager contains utilities for managing Crossplane dependencies.
package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Masterminds/semver"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	ixpkg "github.com/crossplane/crossplane/internal/xpkg/v2"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep/cache"
	xpkg "github.com/crossplane/crossplane/internal/xpkg/v2/dep/marshaler/xpkg"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep/resolver/image"
)

var (
	defaultCacheRoot = ".up/cache"
)

const (
	defaultWatchInterval = "100ms"

	errInvalidSemVerConstraintFmt = "invalid semver constraint %v: %w"
)

// Manager defines a dependency Manager
type Manager struct {
	c             Cache
	i             ImageResolver
	x             XpkgMarshaler
	log           logging.Logger
	cacheRoot     string
	watchInterval *time.Duration

	acc []*xpkg.ParsedPackage
}

// Cache defines the API contract for working with a Cache.
type Cache interface {
	Get(v1beta1.Dependency) (*xpkg.ParsedPackage, error)
	Store(v1beta1.Dependency, *xpkg.ParsedPackage) error
	Versions(v1beta1.Dependency) ([]string, error)
	Watch() <-chan cache.Event
}

// ImageResolver defines the API contract for working with an
// ImageResolver.
type ImageResolver interface {
	ResolveDigest(context.Context, v1beta1.Dependency) (string, error)
	ResolveImage(context.Context, v1beta1.Dependency) (string, v1.Image, error)
	ResolveTag(context.Context, v1beta1.Dependency) (string, error)
}

// XpkgMarshaler defines the API contract for working with an
// xpkg.ParsedPackage marshaler.
type XpkgMarshaler interface {
	FromImage(ixpkg.Image) (*xpkg.ParsedPackage, error)
	FromDir(afero.Fs, string) (*xpkg.ParsedPackage, error)
}

// New returns a new Manager
func New(opts ...Option) (*Manager, error) {
	interval, err := time.ParseDuration(defaultWatchInterval)
	if err != nil {
		return nil, err
	}

	m := &Manager{
		log:           logging.NewNopLogger(),
		cacheRoot:     defaultCacheRoot,
		watchInterval: &interval,
	}

	// TODO(@tnthornton) move this resolution to the config.
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	c, err := cache.NewLocal(
		filepath.Join(
			filepath.Clean(home),
			m.cacheRoot,
		),
		cache.WithLogger(m.log),
		cache.WithWatchInterval(m.watchInterval),
	)
	if err != nil {
		return nil, err
	}

	x, err := xpkg.NewMarshaler()
	if err != nil {
		return nil, err
	}

	m.i = image.NewResolver()
	m.c = c
	m.x = x
	m.acc = make([]*xpkg.ParsedPackage, 0)

	for _, o := range opts {
		o(m)
	}

	return m, nil
}

// Option modifies the Manager.
type Option func(*Manager)

// WithCache sets the supplied cache.Local on the Manager.
func WithCache(c Cache) Option {
	return func(m *Manager) {
		m.c = c
	}
}

// WithLogger overrides the default logger with the supplied logger.
func WithLogger(l logging.Logger) Option {
	return func(m *Manager) {
		m.log = l
	}
}

// WithResolver sets the supplied dep.Resolver on the Manager.
func WithResolver(r ImageResolver) Option {
	return func(m *Manager) {
		m.i = r
	}
}

// WithWatchInterval overrides the default watch interval for the Manager.
func WithWatchInterval(i *time.Duration) Option {
	return func(m *Manager) {
		m.watchInterval = i
	}
}

// View returns a View corresponding to the supplied dependency slice
// (both defined and transitive).
func (m *Manager) View(ctx context.Context, deps []v1beta1.Dependency) (*View, error) {
	packages := make(map[string]*xpkg.ParsedPackage)

	for _, d := range deps {
		_, acc, err := m.Resolve(ctx, d)
		if err != nil && errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, p := range acc {
			packages[p.Name()] = p
		}
	}

	return &View{
		packages: packages,
	}, nil
}

// Versions returns the dependency versions corresponding to the supplied
// v1beta1.Dependency that currently exist locally.
func (m *Manager) Versions(_ context.Context, d v1beta1.Dependency) ([]string, error) {
	return m.c.Versions(d)
}

// Watch provides a hook for watching changes coming from the cache.
func (m *Manager) Watch() <-chan cache.Event {
	return m.c.Watch()
}

// Resolve resolves the given package as well as it's transitive dependencies. If dependencies
// are not included in the current cache, an error is returned.
func (m *Manager) Resolve(ctx context.Context, d v1beta1.Dependency) (v1beta1.Dependency, []*xpkg.ParsedPackage, error) {
	ud := v1beta1.Dependency{}

	e, err := m.retrievePkg(ctx, d)
	if err != nil {
		return ud, m.acc, err
	}

	m.acc = append(m.acc, e)
	if err := m.retrieveAllDeps(ctx, e); err != nil {
		return ud, m.acc, err
	}

	ud.Type = e.Type()
	ud.Package = d.Package
	ud.Constraints = e.Version()

	return ud, m.acc, nil
}

// AddAll resolves the given package as well as it's transitive dependencies.
// If storage is successful, the resolved dependency is returned, errors
// otherwise.
func (m *Manager) AddAll(ctx context.Context, d v1beta1.Dependency) (v1beta1.Dependency, []*xpkg.ParsedPackage, error) {
	ud := v1beta1.Dependency{}

	e, err := m.retrieveAndStorePkg(ctx, d)
	if err != nil {
		return ud, m.acc, err
	}
	m.acc = append(m.acc, e)

	// recursively resolve all transitive dependencies
	// currently assumes we have something from
	if err := m.addAllDeps(ctx, e); err != nil {
		return ud, m.acc, err
	}

	ud.Type = e.Type()
	ud.Package = d.Package
	ud.Constraints = e.Version()

	return ud, m.acc, nil
}

func (m *Manager) retrieveAllDeps(ctx context.Context, p *xpkg.ParsedPackage) error {
	if len(p.Dependencies()) == 0 {
		// no remaining dependencies to resolve
		return nil
	}

	for _, d := range p.Dependencies() {
		e, err := m.retrievePkg(ctx, d)
		if err != nil {
			return err
		}
		m.acc = append(m.acc, e)

		if err := m.retrieveAllDeps(ctx, e); err != nil {
			return err
		}
	}

	return nil
}

// addAllDeps recursively resolves the transitive dependencies for a
// given xpkg.ParsedPackage.
func (m *Manager) addAllDeps(ctx context.Context, p *xpkg.ParsedPackage) error {

	if len(p.Dependencies()) == 0 {
		// no remaining dependencies to resolve
		return nil
	}

	for _, d := range p.Dependencies() {
		e, err := m.retrieveAndStorePkg(ctx, d)
		if err != nil {
			return err
		}
		m.acc = append(m.acc, e)

		if err := m.addAllDeps(ctx, e); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) addPkg(ctx context.Context, d v1beta1.Dependency) (*xpkg.ParsedPackage, error) {
	// this is expensive
	t, i, err := m.i.ResolveImage(ctx, d)
	if err != nil {
		return nil, err
	}

	tag, err := name.NewTag(d.Package)
	if err != nil {
		return nil, err
	}

	digest, err := i.Digest()
	if err != nil {
		return nil, err
	}

	p, err := m.x.FromImage(ixpkg.Image{
		Meta: ixpkg.ImageMeta{
			Repo:     deriveRepoName(tag),
			Registry: tag.RegistryStr(),
			Version:  t,
			Digest:   digest.String(),
		},
		Image: i,
	})
	if err != nil {
		return nil, err
	}

	// add xpkg to cache
	err = m.c.Store(d, p)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func deriveRepoName(t name.Tag) string {
	if t.Registry.Name() == name.DefaultRegistry {
		return t.RepositoryStr()
	}
	return t.Repository.Name()
}

func (m *Manager) retrievePkg(ctx context.Context, d v1beta1.Dependency) (*xpkg.ParsedPackage, error) {
	// resolve version prior to Get
	if err := m.finalizeLocalDepVersion(ctx, &d); err != nil {
		return nil, err
	}

	return m.c.Get(d)
}

func (m *Manager) retrieveAndStorePkg(ctx context.Context, d v1beta1.Dependency) (*xpkg.ParsedPackage, error) {
	// resolve version prior to Get
	if err := m.finalizeExtDepVersion(ctx, &d); err != nil {
		return nil, fmt.Errorf("failed to resolve %s:%s: %w", d.Package, d.Constraints, err)
	}

	p, err := m.c.Get(d)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if os.IsNotExist(err) {
		// root dependency does not yet exist in cache, store it
		p, err = m.addPkg(ctx, d)
		if err != nil {
			return nil, err
		}
	} else {
		// check if digest is different from what we have locally
		digest, err := m.i.ResolveDigest(ctx, d)
		if err != nil {
			return nil, err
		}

		if p.Digest() != digest {
			// digest is different, update what we have
			p, err = m.addPkg(ctx, d)
			if err != nil {
				return nil, err
			}
		}
	}

	return p, nil
}

// finalizeExtDepVersion sets the resolved tag version on the supplied v1beta1.Dependency.
func (m *Manager) finalizeExtDepVersion(ctx context.Context, d *v1beta1.Dependency) error {
	// determine the version (using resolver) to use based on the supplied constraints
	v, err := m.i.ResolveTag(ctx, *d)
	if err != nil {
		return err
	}

	d.Constraints = v
	return nil
}

// finalizeLocalDepVersion sets the resolve tag version on the supplied v1beta1.Dependency
// based on versions currently located in the cache.
func (m *Manager) finalizeLocalDepVersion(_ context.Context, d *v1beta1.Dependency) error {
	// check up front if we already have a semver constraint
	c, err := semver.NewConstraint(d.Constraints)
	if err != nil {
		// Constraints is not a semver constraint, we're not going to
		// find it locally.
		return fmt.Errorf(errInvalidSemVerConstraintFmt, err, os.ErrNotExist)
	}

	// we're working with a semver constraint, let's try to resolve
	// it based on the versions we have locally
	vers, err := m.c.Versions(*d)
	if err != nil {
		return err
	}

	vs := []*semver.Version{}
	for _, r := range vers {
		v, err := semver.NewVersion(r)
		if err != nil {
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
		return os.ErrNotExist
	}

	d.Constraints = ver

	return nil
}

// View represents the processed View corresponding to some dependencies.
type View struct {
	packages map[string]*xpkg.ParsedPackage
}

// Packages returns the packages map for the view.
func (v *View) Packages() map[string]*xpkg.ParsedPackage {
	return v.packages
}
