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

package xpkg

import (
	"os"
	"path/filepath"

	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	errAlreadyExistsFmt = "directory contains pre-existing meta file: %s"
)

// Writer defines a writer that is used for creating package meta files.
type Writer struct {
	fileBody []byte
	fs       afero.Fs
	root     string
}

// NewFileWriter returns a new Writer.
func NewFileWriter(opts ...Option) *Writer {
	w := &Writer{}

	for _, o := range opts {
		o(w)
	}

	return w
}

// Option modifies the Writer.
type Option func(*Writer)

// WithFs specifies the afero.Fs that is being used.
func WithFs(fs afero.Fs) Option {
	return func(w *Writer) {
		w.fs = fs
	}
}

// WithRoot specifies the root for the new package.
func WithRoot(root string) Option {
	return func(w *Writer) {
		w.root = root
	}
}

// WithFileBody specifies the file body that is used to populate
// the new meta file.
func WithFileBody(body []byte) Option {
	return func(w *Writer) {
		w.fileBody = body
	}
}

// NewMetaFile creates a new meta file per the given options.
func (w *Writer) NewMetaFile() error {
	targetFile := filepath.Join(w.root, MetaFile)

	// return err if file already exists
	exists, err := afero.Exists(w.fs, targetFile)
	if err != nil {
		return err
	}
	if exists {
		return errors.Errorf(errAlreadyExistsFmt, w.relativePath(targetFile))
	}

	exists, err = afero.DirExists(w.fs, w.root)
	if err != nil {
		return err
	}

	// create directory if it doesn't exist
	if !exists {
		if err := w.fs.MkdirAll(w.root, os.ModePerm); err != nil {
			return err
		}
	}

	return afero.WriteFile(w.fs, targetFile, w.fileBody, StreamFileMode)
}

func (w *Writer) relativePath(path string) string {
	if !filepath.IsAbs(path) {
		return path
	}
	rel, err := filepath.Rel(w.root, path)
	if err != nil {
		return path
	}
	return rel
}
