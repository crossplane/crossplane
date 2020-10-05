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

	"github.com/crossplane/crossplane-runtime/pkg/parser"
)

// Build compiles a Crossplane package from an on-disk package.
func Build(ctx context.Context, b parser.Backend) (v1.Image, error) {
	// Get YAML stream.
	r, err := b.Init(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()

	// Copy stream into buffer so that we know the size.
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, r)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	_, err = io.Copy(tw, buf)
	if err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	// Build image layer from tarball.
	l, err := tarball.LayerFromReader(tarBuf)
	if err != nil {
		return nil, err
	}

	// Append layer to to scratch image.
	return mutate.AppendLayers(empty.Image, l)
}
