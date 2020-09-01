/*
Copyright 2020 The Crossplane Authors.

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

package parser

import (
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// FsReadCloser implements io.ReadCloser for an Afero filesystem.
type FsReadCloser struct {
	fs       afero.Fs
	dir      string
	paths    []string
	index    int
	position int
}

// A SkipFn indicates whether a file should be skipped when the FsReadCloser
// walks the filesystem. Returning true indicates the file should be skipped.
// Returning an error will cause the FsReadCloser to stop walking the filesystem and
type SkipFn func(path string, info os.FileInfo) (bool, error)

// SkipPath skips files at a certain path.
func SkipPath(pattern string) SkipFn {
	return func(path string, info os.FileInfo) (bool, error) {
		y, err := filepath.Match(pattern, path)
		return !y, err
	}
}

// SkipDirs skips directories.
func SkipDirs() SkipFn {
	return func(path string, info os.FileInfo) (bool, error) {
		if info.IsDir() {
			return false, nil
		}
		return true, nil
	}
}

// SkipNotYaml skips files that do not have yaml extension.
func SkipNotYaml() SkipFn {
	return func(path string, info os.FileInfo) (bool, error) {
		if filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml" {
			return false, nil
		}
		return true, nil
	}
}

// NewFsReadCloser returns an FsReadCloser that implements io.ReadCloser. It
// walks the filesystem ahead of time, then reads file contents when Read is
// invoked.
func NewFsReadCloser(fs afero.Fs, dir string, fns ...SkipFn) (*FsReadCloser, error) {
	paths := []string{}
	err := afero.Walk(fs, dir, func(path string, info os.FileInfo, err error) error {
		include := true
		for _, fn := range fns {
			fnInc, fnErr := fn(path, info)
			if fnErr != nil {
				return fnErr
			}
			if !fnInc {
				include = false
			}
		}
		if include {
			paths = append(paths, path)
		}
		return err
	})
	return &FsReadCloser{
		fs:       fs,
		dir:      dir,
		paths:    paths,
		index:    0,
		position: 0,
	}, err
}

func (r *FsReadCloser) Read(p []byte) (n int, err error) {
	if r.index == len(r.paths) {
		return 0, io.EOF
	}
	b, err := afero.ReadFile(r.fs, r.paths[r.index])
	n = copy(p, b[r.position:])
	r.position += n
	if err == io.EOF || n == 0 {
		r.position = 0
		r.index++
		n = copy(p, []byte("\n---\n"))
		err = nil
	}
	return
}

// Close is a no op for an FsReadCloser.
func (r *FsReadCloser) Close() error {
	return nil
}
