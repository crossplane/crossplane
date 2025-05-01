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
	"io"
	"io/fs"
	"path/filepath"

	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	errCreatePrefixDir = "failed to create prefix directory in tar archive"
	errPopulateTar     = "failed to populate tar archive"
	errCloseTar        = "failed to close tar archive"
)

// FSToTar produces a tarball of all the files in a filesystem.
// NOTE(jastang): this is inspired by how github.com/upbound/up builds embedded function images.
func FSToTar(f afero.Fs, prefix string) ([]byte, error) {
	// TODO(jastang): we could consider capping memory and report short writes.
	// this would be a per-layer cap and files should be small, so we'll let this get some real-world usage before imposing limtis.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	prefixHdr := &tar.Header{
		Name:     prefix,
		Typeflag: tar.TypeDir,
		Mode:     0o777,
	}

	err := tw.WriteHeader(prefixHdr)
	if err != nil {
		return nil, errors.Wrap(err, errCreatePrefixDir)
	}
	err = afero.Walk(f, ".", func(name string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		return addToTar(tw, prefix, f, name, info)
	})
	if err != nil {
		return nil, errors.Wrap(err, errPopulateTar)
	}
	err = tw.Close()
	if err != nil {
		return nil, errors.Wrap(err, errCloseTar)
	}

	return buf.Bytes(), nil
}

func addToTar(tw *tar.Writer, prefix string, f afero.Fs, filename string, info fs.FileInfo) error {
	// Compute the full path in the tar archive
	fullPath := filepath.Join(prefix, filename)

	if info.IsDir() {
		// Skip the root directory as it was already added
		if fullPath == prefix {
			return nil
		}

		h, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		h.Name = fullPath
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		return nil
	}

	if !info.Mode().IsRegular() {
		return errors.Errorf("unhandled file mode %v", info.Mode())
	}

	h, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	h.Name = fullPath

	if err := tw.WriteHeader(h); err != nil {
		return err
	}

	file, err := f.Open(filename)
	if err != nil {
		return err
	}

	_, err = io.Copy(tw, file)
	return err
}
