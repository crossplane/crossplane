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
)

var _ io.ReadCloser = &gzipReadCloser{}

// gzipReadCloser reads compressed contents from a file.
type gzipReadCloser struct {
	rc   io.ReadCloser
	gzip *gzip.Reader
}

// GzipReadCloser constructs a new gzipReadCloser from the passed file.
func GzipReadCloser(rc io.ReadCloser) (io.ReadCloser, error) {
	r, err := gzip.NewReader(rc)
	if err != nil {
		return nil, err
	}
	return &gzipReadCloser{
		rc:   rc,
		gzip: r,
	}, nil
}

// Read calls the underlying gzip reader's Read method.
func (g *gzipReadCloser) Read(p []byte) (n int, err error) {
	return g.gzip.Read(p)
}

// Close first closes the gzip reader, then closes the underlying closer.
func (g *gzipReadCloser) Close() error {
	if err := g.gzip.Close(); err != nil {
		_ = g.rc.Close()
		return err
	}
	return g.rc.Close()
}

var _ io.ReadCloser = &teeReadCloser{}

// teeReadCloser is a TeeReader that also closes the underlying writer.
type teeReadCloser struct {
	w io.WriteCloser
	r io.ReadCloser
	t io.Reader
}

// TeeReadCloser constructs a teeReadCloser from the passed reader and writer.
func TeeReadCloser(r io.ReadCloser, w io.WriteCloser) io.ReadCloser {
	return &teeReadCloser{
		w: w,
		r: r,
		t: io.TeeReader(r, w),
	}
}

// Read calls the underlying TeeReader Read method.
func (t *teeReadCloser) Read(b []byte) (int, error) {
	return t.t.Read(b)
}

// Close closes the underlying ReadCloser, then the Writer for the TeeReader.
func (t *teeReadCloser) Close() error {
	if err := t.r.Close(); err != nil {
		_ = t.w.Close()
		return err
	}
	return t.w.Close()
}

var _ io.ReadCloser = &joinedReadCloser{}

// joinedReadCloser joins a reader and a closer. It is typically used in the
// context of a ReadCloser being wrapped by a Reader.
type joinedReadCloser struct {
	r io.Reader
	c io.Closer
}

// JoinedReadCloser constructs a new joinedReadCloser from the passed reader and
// closer.
func JoinedReadCloser(r io.Reader, c io.Closer) io.ReadCloser {
	return &joinedReadCloser{
		r: r,
		c: c,
	}
}

// Read calls the underlying reader Read method.
func (r *joinedReadCloser) Read(b []byte) (int, error) {
	return r.r.Read(b)
}

// Close closes the closer for the JoinedReadCloser.
func (r *joinedReadCloser) Close() error {
	return r.c.Close()
}
