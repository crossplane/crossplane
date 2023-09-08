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
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

var _ AnnotatedReadCloser = &FsReadCloser{}

// FsReadCloserAnnotation annotates data for an FsReadCloser.
type FsReadCloserAnnotation struct {
	path     string
	position int
}

// FsReadCloser implements io.ReadCloser for an Afero filesystem.
type FsReadCloser struct {
	fs         afero.Fs
	dir        string
	paths      []string
	index      int
	position   int
	writeBreak bool
	wroteBreak bool
}

// A FilterFn filters files when the FsReadCloser walks the filesystem.
// Returning true indicates the file should be skipped. Returning an error will
// cause the FsReadCloser to stop walking the filesystem and return.
type FilterFn func(path string, info os.FileInfo) (bool, error)

// SkipPath skips files at a certain path.
func SkipPath(pattern string) FilterFn {
	return func(path string, info os.FileInfo) (bool, error) {
		return filepath.Match(pattern, path)
	}
}

// SkipDirs skips directories.
func SkipDirs() FilterFn {
	return func(path string, info os.FileInfo) (bool, error) {
		if info.IsDir() {
			return true, nil
		}
		return false, nil
	}
}

// SkipEmpty skips empty files.
func SkipEmpty() FilterFn {
	return func(path string, info os.FileInfo) (bool, error) {
		return info.Size() == 0, nil
	}
}

// SkipNotYAML skips files that do not have YAML extension.
func SkipNotYAML() FilterFn {
	return func(path string, info os.FileInfo) (bool, error) {
		if filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml" {
			return true, nil
		}
		return false, nil
	}
}

// NewFsReadCloser returns an FsReadCloser that implements io.ReadCloser. It
// walks the filesystem ahead of time, then reads file contents when Read is
// invoked. It does not follow symbolic links.
func NewFsReadCloser(fs afero.Fs, dir string, fns ...FilterFn) (*FsReadCloser, error) {
	paths := []string{}
	err := afero.Walk(fs, dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		for _, fn := range fns {
			filter, err := fn(path, info)
			if err != nil {
				return err
			}
			if filter {
				return nil
			}
		}
		paths = append(paths, path)
		return nil
	})
	return &FsReadCloser{
		fs:         fs,
		dir:        dir,
		paths:      paths,
		index:      0,
		position:   0,
		writeBreak: false,
		wroteBreak: false,
	}, err
}

func (r *FsReadCloser) Read(p []byte) (n int, err error) {
	if r.wroteBreak {
		r.index++
		r.position = 0
		r.wroteBreak = false
		n = copy(p, "\n---\n")
		return n, nil
	}
	if r.index == len(r.paths) {
		return 0, io.EOF
	}
	if r.writeBreak {
		n = copy(p, "\n...\n")
		r.writeBreak = false
		r.wroteBreak = true
		return n, nil
	}
	b, err := afero.ReadFile(r.fs, r.paths[r.index])
	n = copy(p, b[r.position:])
	r.position += n
	if errors.Is(err, io.EOF) || n == 0 {
		r.writeBreak = true
		err = nil
	}
	return n, err
}

// Close is a no op for an FsReadCloser.
func (r *FsReadCloser) Close() error {
	return nil
}

// Annotate returns additional about the data currently being read.
func (r *FsReadCloser) Annotate() any {
	// Index will be out of bounds if we error after the final file has been
	// read.
	index := r.index
	if index == len(r.paths) {
		index--
	}
	return FsReadCloserAnnotation{
		path:     r.paths[index],
		position: r.position,
	}
}
