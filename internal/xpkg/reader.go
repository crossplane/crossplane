/*
Copyright 2022 The Crossplane Authors.

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
	"compress/gzip"
	"io"

	"github.com/spf13/afero"
)

var _ io.ReadCloser = &gzipFileReader{}

// GzipFileReader reads compressed contents from a file.
type gzipFileReader struct {
	f    afero.File
	gzip *gzip.Reader
}

// newGzipFileReader builds a gzipFileReader with the provided file.
func newGzipFileReader(f afero.File) (*gzipFileReader, error) {
	r, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	return &gzipFileReader{
		f:    f,
		gzip: r,
	}, nil
}

// Read calls the underlying gzip reader's Read method.
func (g *gzipFileReader) Read(p []byte) (n int, err error) {
	return g.gzip.Read(p)
}

// Close first closes the gzip reader, then closes the underlying file.
func (g *gzipFileReader) Close() error {
	if err := g.gzip.Close(); err != nil {
		return err
	}
	return g.f.Close()
}

// TeeReadCloser is a TeeReader that also closes the underlying writer.
type TeeReadCloser struct {
	w io.WriteCloser
	t io.Reader
}

var _ io.ReadCloser = &TeeReadCloser{}

// NewTeeReadCloser constructs a TeeReadCloser from the passed reader and
// writer.
func NewTeeReadCloser(r io.ReadCloser, w io.WriteCloser) *TeeReadCloser {
	return &TeeReadCloser{
		w: w,
		t: io.TeeReader(r, w),
	}
}

// Read calls the underlying TeeReader Read method.
func (t *TeeReadCloser) Read(b []byte) (int, error) {
	return t.t.Read(b)
}

// Close closes the writer for the TeeReader.
func (t *TeeReadCloser) Close() error {
	return t.w.Close()
}
