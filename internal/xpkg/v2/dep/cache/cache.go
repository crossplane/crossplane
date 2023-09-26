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

// Package cache contains utilities for caching packages.
package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/radovskyb/watcher"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep/marshaler/xpkg"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep/resolver/image"
)

const (
	defaultWatchInterval = "100ms"

	errFailedToAddEntry     = "failed to add entry to cache"
	errFailedToFindEntry    = "failed to find entry"
	errInvalidValueSupplied = "invalid value supplied"
	errInvalidVersion       = "invalid version found"

	errFailedToWatchCache = "failed to setup cache watch"
)

// Local stores and retrieves xpkg.ParsedPackages in a filesystem-backed cache
// in a thread-safe manner.
type Local struct {
	fs     afero.Fs
	log    logging.Logger
	mu     sync.RWMutex
	pkgres XpkgMarshaler
	root   string

	closed        bool
	subs          []chan Event
	watchInterval *time.Duration
	watching      bool
}

// XpkgMarshaler defines the API contract for working marshaling
// xpkg.ParsedPackage's from a directory.
type XpkgMarshaler interface {
	FromDir(afero.Fs, string) (*xpkg.ParsedPackage, error)
}

// NewLocal creates a new LocalCache.
func NewLocal(root string, opts ...Option) (*Local, error) {
	interval, err := time.ParseDuration(defaultWatchInterval)
	if err != nil {
		return nil, err
	}

	l := &Local{
		fs:            afero.NewOsFs(),
		log:           logging.NewNopLogger(),
		watchInterval: &interval,
	}

	for _, o := range opts {
		o(l)
	}

	l.root = filepath.Clean(root)

	r, err := xpkg.NewMarshaler()
	if err != nil {
		return nil, err
	}

	l.pkgres = r

	return l, nil
}

// Option represents an option that can be applied to Local.
type Option func(*Local)

// WithFS defines the filesystem that is configured for Local.
func WithFS(fs afero.Fs) Option {
	return func(l *Local) {
		l.fs = fs
	}
}

// WithLogger defines the logger that is configured for Local.
func WithLogger(logger logging.Logger) Option {
	return func(l *Local) {
		l.log = logger
	}
}

// WithWatchInterval overrides the default watchInterval for Local.
func WithWatchInterval(i *time.Duration) Option {
	return func(l *Local) {
		l.watchInterval = i
	}
}

// Get retrieves an image from the LocalCache.
func (c *Local) Get(k v1beta1.Dependency) (*xpkg.ParsedPackage, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, err := name.NewTag(image.FullTag(k))
	if err != nil {
		return nil, err
	}

	e, err := c.currentEntry(calculatePath(&t))
	if err != nil {
		return nil, err
	}

	return e.pkg, nil
}

// Store saves an image to the LocalCache. If a file currently
// exists at that location, we overwrite the current file.
func (c *Local) Store(k v1beta1.Dependency, v *xpkg.ParsedPackage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if v == nil {
		return errors.New(errInvalidValueSupplied)
	}

	t, err := name.NewTag(image.FullTag(k))
	if err != nil {
		return err
	}

	path := calculatePath(&t)

	curr, err := c.currentEntry(path)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, errFailedToFindEntry)
	}

	e := c.newEntry(v)

	// clean the current entry
	if err := curr.Clean(); err != nil {
		return err
	}

	return c.add(e, path)
}

// Versions returns a slice of versions that exist in the cache for the given
// package.
func (c *Local) Versions(k v1beta1.Dependency) ([]string, error) {
	t, err := name.NewTag(k.Package)
	if err != nil {
		return nil, err
	}

	glob := calculateVersionsGlob(&t)

	matches, err := afero.Glob(c.fs, filepath.Join(c.root, glob))
	if err != nil {
		return nil, err
	}
	vers := make([]string, 0)
	for _, m := range matches {
		ver := strings.Split(m, "@")
		if len(ver) != 2 {
			return nil, errors.New(errInvalidVersion)
		}
		vers = append(vers, ver[1])
	}

	return vers, nil
}

// Watch returns a channel that can be used to subscribe to events
// from the cache.
func (c *Local) Watch() <-chan Event {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Start watching if we haven't already.
	if !c.watching {
		if err := c.ensureDirExists(c.root); err != nil {
			c.log.Debug(errFailedToWatchCache, "error", err)
		}

		// Kick off cache watching in a separate routine so we don't
		// block upstream operations on an infrequently used operation.
		go c.watchCache()
	}

	// use an buffered channel so that we don't block incoming watch events.
	ch := make(chan Event, 1)
	c.subs = append(c.subs, ch)
	return ch
}

// Clean removes all entries from the cache. Returns nil if the directory DNE.
func (c *Local) Clean() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	files, err := afero.ReadDir(c.fs, c.root)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, f := range files {
		if err := c.fs.RemoveAll(filepath.Join(c.root, f.Name())); err != nil {
			return err
		}
	}
	return nil
}

// Publish publishes the given Event to the subscription channels defined
// for the cache.
func (c *Local) publish(e Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// subscriptions are closed, skip publish.
	if c.closed {
		return
	}

	for _, ch := range c.subs {
		ch <- e
	}
}

// Close closes the subscription channels for the cache.
func (c *Local) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.closed {
		c.closed = true
		for _, ch := range c.subs {
			close(ch)
		}
	}
}

// Event contains change information about the cache.
type Event any

// add the given entry to the supplied path (to)
func (c *Local) add(e *entry, to string) error {
	if err := c.ensureDirExists(filepath.Join(c.root, to)); err != nil {
		return err
	}

	e.setPath(to)

	if _, err := e.flush(); err != nil {
		return errors.Wrap(err, errFailedToAddEntry)
	}

	return nil
}

// ensureDirExists ensures the target directory corresponding to the given path exists.
func (c *Local) ensureDirExists(path string) error {
	return c.fs.MkdirAll(path, os.ModePerm)
}

// calculatePath calculates the directory path from the given name.Tag following
// our convention.
// example:
//
//	tag: crossplane/provider-aws:v0.20.1-alpha
//	path: index.docker.io/crossplane/provider-aws@v0.20.1-alpha
func calculatePath(tag *name.Tag) string {
	return filepath.Join(
		tag.RegistryStr(),
		fmt.Sprintf("%s@%s", tag.RepositoryStr(), tag.TagStr()),
	)
}

func calculateVersionsGlob(tag *name.Tag) string {
	return filepath.Join(
		tag.RegistryStr(),
		fmt.Sprintf("%s@*", tag.RepositoryStr()),
	)
}

// watchCache watches the cache for changes. As changes are detected by the
// underlying watcher package, events are published to upstream cache watchers.
func (c *Local) watchCache() {
	watch := watcher.New()
	watch.SetMaxEvents(1)

	go func() {
		for {
			select {
			case event := <-watch.Event:
				// send the event to the subscribers
				c.publish(event)
			case err := <-watch.Error:
				c.log.Debug(err.Error())
			case <-watch.Closed:
				return
			}
		}
	}()

	c.log.Debug(fmt.Sprintf("setting up watch at cache root: %s", c.root))
	// Watch cache root directory recursively for changes.
	if err := watch.AddRecursive(c.root); err != nil {
		c.log.Debug(errFailedToWatchCache, "error", err)
	}

	// Print a list of all the files and folders currently
	// being watched and their paths.
	for path, f := range watch.WatchedFiles() {
		c.log.Debug(fmt.Sprintf("%s: %s\n", path, f.Name()))
	}

	// Start the watching process - it'll check for changes at the given watchInterval.
	go func() {
		if err := watch.Start(*c.watchInterval); err != nil {
			c.log.Debug(errFailedToWatchCache, "error", err)
		}
	}()
}
