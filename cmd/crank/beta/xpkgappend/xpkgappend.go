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

// Package xpkgappend contains the xpkg-append command.
package xpkgappend

import (
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errParseReference          = "error parsing remote reference"
	errCreateExtensionsTarball = "error creating package extensions tarball"
	errAppendExtensions        = "error appending package extensions to image"
	errReadIndex               = "error reading remote index"
	errWriteIndex              = "error writing image index to remote ref"
	errGetIndexDigest          = "error getting index digests"
)

// AfterApply constructs and binds context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply() error {
	// TODO(jastang): consider prompting about re-signing if already signed
	c.fs = afero.NewOsFs()
	paths := []string{}

	err := os.Chdir(c.ExtensionsRoot)
	if err != nil {
		return err
	}
	err = afero.Walk(c.fs, c.ExtensionsRoot, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relpath, _ := filepath.Rel(c.ExtensionsRoot, path)
		paths = append(paths, relpath)
		return nil
	})
	if err != nil {
		return err
	}
	c.relPaths = paths

	// Get default docker auth.
	c.keychain = remote.WithAuthFromKeychain(authn.NewMultiKeychain(authn.DefaultKeychain))

	// Make sure the ref parses properly
	ref, err := name.ParseReference(c.RemoteRef)
	if err != nil {
		return errors.Wrap(err, errParseReference)
	}

	c.indexRef = ref

	c.appender = xpkg.NewAppender(
		c.keychain,
		c.indexRef,
	)

	return nil
}

// Cmd appends an additional manifest of package extensions to a crossplane package.
type Cmd struct {
	// Arguments
	RemoteRef string `arg:"" help:"The fully qualified remote image reference" required:""`

	// Flags. Keep sorted alphabetically.
	ExtensionsRoot string `default:"./extensions" help:"An optional directory of arbitrary files for additional consumers of the package." placeholder:"PATH" type:"path"`

	// Internal state. These aren't part of the user-exposed CLI structure.
	fs       afero.Fs
	relPaths []string
	indexRef name.Reference
	keychain remote.Option
	appender *xpkg.Appender
}

// Help returns the help message for the xpkg-append command.
func (c *Cmd) Help() string {
	return `
This command creates a tarball from a local directory of additional package
assets, such as images or documentation, and appends them to a remote image.

If your remote image is already signed, this command will invalidate current signatures and the updated image will need to be re-signed.

Examples:

  # Add all files under an "/extensions" folder to a remote image.
  crossplane beta xpkg-append --extensions-root=./extensions my-registry/my-organization/my-repo@sha256:<digest>

`
}

// Run executes the append command.
func (c *Cmd) Run(logger logging.Logger) error {
	logger = logger.WithValues("cmd", "xpkg-append")

	extLayer, err := xpkg.LayerFromFiles(c.relPaths, c.fs)
	if err != nil {
		return errors.Wrap(err, errCreateExtensionsTarball)
	}

	logger.Debug("Appending package extensions for image", "ref", c.indexRef.String())
	// Ensure we are working with an image index, for now.
	// We do not currently support converting a single manifest into an index, which could create unintentional side effects.
	index, err := remote.Index(c.indexRef, c.keychain)
	if err != nil {
		return errors.Wrap(err, errReadIndex)
	}
	// Construct a new image index with the extensions manifest appended.
	// Passing a different extensions directory overwrites the previous manifest if one exists.
	newIndex, err := c.appender.Append(index, extLayer, xpkg.WithAuth(c.keychain))
	if err != nil {
		return errors.Wrap(err, errAppendExtensions)
	}
	// No-op if the index digest has not changed
	noop, err := indexDigestsEqual(index, newIndex)
	if err != nil {
		return errors.Wrap(err, errGetIndexDigest)
	}
	if noop {
		return nil
	}
	err = remote.WriteIndex(c.indexRef, newIndex, c.keychain)
	if err != nil {
		return errors.Wrap(err, errWriteIndex)
	}
	return nil
}

// indexDigestsEqual checks if two v1.ImageIndex have the same digest.
func indexDigestsEqual(oi, ni v1.ImageIndex) (bool, error) {
	oldDigest, err := oi.Digest()
	if err != nil {
		return false, err
	}
	newDigest, err := ni.Digest()
	if err != nil {
		return false, err
	}
	return oldDigest.String() == newDigest.String(), nil
}
