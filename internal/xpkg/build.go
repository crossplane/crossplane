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

package xpkg

import (
	"archive/tar"
	"bytes"
	"context"
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
)

const (
	errParserPackage = "failed to parse package"
	errLintPackage   = "failed to lint package"
	errInitBackend   = "failed to initialize package parsing backend"
	errTarFromStream = "failed to build tarball from package stream"
	errLayerFromTar  = "failed to convert tarball to image layer"
)

// annotatedTeeReadCloser is a copy of io.TeeReader that implements
// parser.AnnotatedReadCloser. It returns a Reader that writes to w what it
// reads from r. All reads from r performed through it are matched with
// corresponding writes to w. There is no internal buffering - the write must
// complete before the read completes. Any error encountered while writing is
// reported as a read error. If the underling reader is a
// parser.AnnotatedReadCloser the tee reader will invoke its Annotate function.
// Otherwise it will return nil. Closing is always a no-op.
func annotatedTeeReadCloser(r io.Reader, w io.Writer) *teeReader {
	return &teeReader{r, w}
}

type teeReader struct {
	r io.Reader
	w io.Writer
}

func (t *teeReader) Read(p []byte) (n int, err error) {
	n, err = t.r.Read(p)
	if n > 0 {
		if n, err := t.w.Write(p[:n]); err != nil {
			return n, err
		}
	}
	return
}

func (t *teeReader) Close() error {
	return nil
}

func (t *teeReader) Annotate() any {
	anno, ok := t.r.(parser.AnnotatedReadCloser)
	if !ok {
		return nil
	}
	return anno.Annotate()
}

// Build compiles a Crossplane package from an on-disk package.
func Build(ctx context.Context, b parser.Backend, p parser.Parser, l parser.Linter) (v1.Image, error) {
	// Get YAML stream.
	r, err := b.Init(ctx)
	if err != nil {
		return nil, errors.Wrap(err, errInitBackend)
	}
	defer func() { _ = r.Close() }()

	// Copy stream once to parse and once write to tarball.
	buf := new(bytes.Buffer)
	pkg, err := p.Parse(ctx, annotatedTeeReadCloser(r, buf))
	if err != nil {
		return nil, errors.Wrap(err, errParserPackage)
	}
	if err := l.Lint(pkg); err != nil {
		return nil, errors.Wrap(err, errLintPackage)
	}

	// Write on-disk package contents to tarball.
	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)

	hdr := &tar.Header{
		Name: StreamFile,
		Mode: int64(StreamFileMode),
		Size: int64(buf.Len()),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, errors.Wrap(err, errTarFromStream)
	}
	if _, err = io.Copy(tw, buf); err != nil {
		return nil, errors.Wrap(err, errTarFromStream)
	}
	if err := tw.Close(); err != nil {
		return nil, errors.Wrap(err, errTarFromStream)
	}

	// Build image layer from tarball.
	// TODO(hasheddan): we construct a new reader each time the layer is read,
	// once for calculating the digest, which is used in choosing package file
	// name if not set, and once for writing the contents to disk. This can be
	// optimized in the future, along with the fact that we are copying the full
	// package contents into memory above.
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(tarBuf.Bytes())), nil
	})
	if err != nil {
		return nil, errors.Wrap(err, errLayerFromTar)
	}

	// Append layer to to scratch image.
	return mutate.AppendLayers(empty.Image, layer)
}
