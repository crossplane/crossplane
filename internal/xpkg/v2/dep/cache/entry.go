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

package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	v1ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	v1beta1ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	xpv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/xpkg/v2"
	rxpkg "github.com/crossplane/crossplane/internal/xpkg/v2/dep/marshaler/xpkg"
)

const (
	crdNameFmt = "%s.yaml"
	delim      = "\n"

	errFailedToCreateMeta          = "failed to create meta file in entry"
	errFailedToCreateImageMeta     = "failed to create image meta entry"
	errFailedToCreateCacheEntryFmt = "failed to create a cache entry for the CRD at path %s"
	errFailedToAppendCRDFmt        = "failed to add CRD %q to package"
	errNoObjectsToFlushToDisk      = "no objects to flush"
)

// entry is the internal representation of the cache at a given directory
type entry struct {
	cacheRoot string
	fs        afero.Fs
	path      string
	pkg       *rxpkg.ParsedPackage
}

// NewEntry --
// TODO(@tnthornton) maybe pull this into cache.go
func (c *Local) newEntry(p *rxpkg.ParsedPackage) *entry {

	return &entry{
		cacheRoot: c.root,
		fs:        c.fs,
		pkg:       p,
	}
}

// CurrentEntry retrieves the current Entry at the given path.
// In addition registry and repo are provided in order to fully
// hydrate the ParsedPackage.
// TODO(@tnthornton) maybe pull this into cache.go
func (c *Local) currentEntry(path string) (*entry, error) {

	e := &entry{
		cacheRoot: c.root,
		fs:        c.fs,
		path:      path,
	}

	// grab the current entry if it exists
	pkg, err := c.pkgres.FromDir(c.fs, e.location())
	if os.IsNotExist(err) {
		return e, err
	}
	if err != nil {
		return nil, err
	}

	e.pkg = pkg

	return e, nil
}

// flush writes the package contents to disk.
// In addition to error, flush returns the number of meta, CRD, and XRD files
// written to the entry on disk.
func (e *entry) flush() (*flushstats, error) {
	stats := &flushstats{}

	if e.pkg == nil {
		return stats, errors.New(errNoObjectsToFlushToDisk)
	}

	imetaStats, err := e.writeImageMeta(e.pkg.Reg, e.pkg.DepName, e.pkg.Ver, e.pkg.Digest())
	if err != nil {
		return stats, err
	}
	stats.combine(imetaStats)

	metaStats, err := e.writeMeta(e.pkg.Meta())
	if err != nil {
		return stats, err
	}
	stats.combine(metaStats)

	objstats, err := e.writeObjects(e.pkg.Objects())
	if err != nil {
		return stats, err
	}
	stats.combine(objstats)

	// writing empty digest file
	_, err = e.fs.Create(filepath.Join(e.location(), e.pkg.Digest()))
	if err != nil {
		return stats, err
	}

	return stats, err
}

func (e *entry) writeImageMeta(registry, repo, version, digest string) (*flushstats, error) {
	stats := &flushstats{}

	b, err := json.Marshal(xpkg.ImageMeta{
		Digest:   digest,
		Repo:     repo,
		Registry: registry,
		Version:  version,
	})
	if err != nil {
		return stats, errors.Wrap(err, errFailedToCreateImageMeta)
	}

	if err := e.createPackageJSON(b); err != nil {
		return stats, errors.Wrap(err, errFailedToCreateImageMeta)
	}

	stats.incImageMeta()
	return stats, nil
}

// writeMeta writes the meta file to disk.
// If the meta file was written, we return the file count
func (e *entry) writeMeta(o runtime.Object) (*flushstats, error) {
	stats := &flushstats{}

	cf, err := e.fs.Create(filepath.Join(e.location(), xpkg.MetaFile))
	if err != nil {
		return stats, errors.Wrap(err, errFailedToCreateMeta)
	}
	defer cf.Close() //nolint:errcheck //error is checked in the happy path

	b, err := yaml.Marshal(o)
	if err != nil {
		return stats, errors.Wrap(err, errFailedToCreateMeta)
	}

	mb, err := cf.Write(b)
	if err != nil {
		return stats, errors.Wrap(err, errFailedToCreateMeta)
	}

	jb, err := json.Marshal(o)
	if err != nil {
		return stats, errors.Wrap(err, errFailedToCreateMeta)
	}

	if err := e.appendToPackageJSON(jb); err != nil {
		return stats, errors.Wrap(err, errFailedToCreateMeta)
	}

	if mb > 0 {
		stats.incMetas()
		return stats, err
	}

	return stats, cf.Close()
}

func (e *entry) createPackageJSON(data []byte) error {
	pf, err := e.fs.Create(filepath.Join(e.location(), xpkg.JSONStreamFile))
	if err != nil {
		return err
	}
	defer pf.Close() //nolint:errcheck // not much we could do here

	return writeToFile(data, pf)
}

func (e *entry) appendToPackageJSON(data []byte) error {
	pf, err := e.fs.OpenFile(filepath.Join(e.location(), xpkg.JSONStreamFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer pf.Close() //nolint:errcheck // not much we could do here

	return writeToFile(data, pf)
}

func writeToFile(data []byte, f afero.File) error {
	// add delimiter
	data = append(data, []byte(delim)...)
	_, err := f.Write(data)
	return err
}

// writeObjects writes out the CRDs and XRDs that came from the package.yaml
func (e *entry) writeObjects(objs []runtime.Object) (*flushstats, error) {
	stats := &flushstats{}

	for _, o := range objs {
		var inc statsIncrementer

		yb, err := yaml.Marshal(o)
		if err != nil {
			return stats, err
		}

		jb, err := json.Marshal(o)
		if err != nil {
			return stats, err
		}

		name := ""
		switch crd := o.(type) {
		case *v1beta1ext.CustomResourceDefinition:
			name = crd.GetName()
			inc = stats.incCRDs
		case *v1ext.CustomResourceDefinition:
			name = crd.GetName()
			inc = stats.incCRDs
		case *xpv1.CompositeResourceDefinition:
			name = crd.GetName()
			inc = stats.incXRDs
		case *xpv1.Composition:
			name = crd.GetName()
			inc = stats.incComps
		default:
			// not a CRD, XRD, nor a Composition, skip
			continue
		}

		entryLocation := filepath.Join(e.location(), fmt.Sprintf(crdNameFmt, name))
		if err := afero.WriteFile(e.fs, entryLocation, yb, 0o600); err != nil {
			return stats, errors.Wrapf(err, errFailedToCreateCacheEntryFmt, entryLocation)
		}

		if err := e.appendToPackageJSON(jb); err != nil {
			return stats, errors.Wrapf(err, errFailedToAppendCRDFmt, name)
		}

		inc()
	}

	return stats, nil
}

// Path returns the path this entry represents.
func (e *entry) Path() string {
	return e.path
}

// SetPath sets the Entry path to the supplied path.
func (e *entry) setPath(path string) {
	e.path = path
}

// Clean cleans all files from the entry without deleting the parent directory
// where the Entry is located.
func (e *entry) Clean() error {
	files, err := afero.ReadDir(e.fs, e.location())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, f := range files {
		if err := e.fs.RemoveAll(filepath.Join(e.location(), f.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (e *entry) location() string {
	return filepath.Join(e.cacheRoot, e.path)
}

type flushstats struct {
	imageMeta int
	comps     int
	crds      int
	metas     int
	xrds      int
}

type statsIncrementer func()

func (s *flushstats) incImageMeta() {
	s.imageMeta++
}

func (s *flushstats) incComps() {
	s.comps++
}

func (s *flushstats) incCRDs() {
	s.crds++
}

func (s *flushstats) incMetas() {
	s.metas++
}

func (s *flushstats) incXRDs() {
	s.xrds++
}

func (s *flushstats) combine(src *flushstats) {
	s.imageMeta += src.imageMeta
	s.comps += src.comps
	s.crds += src.crds
	s.metas += src.metas
	s.xrds += src.xrds
}
