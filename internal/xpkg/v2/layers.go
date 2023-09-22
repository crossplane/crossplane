// Copyright 2022 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package xpkg

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Layer creates a v1.Layer that represetns the layer contents for the xpkg and
// adds a corresponding label to the image Config for the layer.
func Layer(r io.Reader, fileName, annotation string, fileSize int64, mode os.FileMode, cfg *v1.Config) (v1.Layer, error) {
	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)

	exHdr := &tar.Header{
		Name: fileName,
		Mode: int64(mode),
		Size: fileSize,
	}

	if err := writeLayer(tw, exHdr, r); err != nil {
		return nil, err
	}

	// TODO(hasheddan): we currently return a new reader every time here in
	// order to calculate digest, then subsequently write contents to disk. We
	// can greatly improve performance during package build by avoiding reading
	// every layer into memory.
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(tarBuf.Bytes())), nil
	})
	if err != nil {
		return nil, errors.Wrap(err, errLayerFromTar)
	}

	d, err := layer.Digest()
	if err != nil {
		return nil, errors.Wrap(err, errDigestInvalid)
	}

	// add annotation label to config if a non-empty label is specified
	if annotation != "" {
		cfg.Labels[Label(d.String())] = annotation
	}

	return layer, nil
}

func writeLayer(tw *tar.Writer, hdr *tar.Header, buf io.Reader) error {
	if err := tw.WriteHeader(hdr); err != nil {
		return errors.Wrap(err, errTarFromStream)
	}

	if _, err := io.Copy(tw, buf); err != nil {
		return errors.Wrap(err, errTarFromStream)
	}
	if err := tw.Close(); err != nil {
		return errors.Wrap(err, errTarFromStream)
	}
	return nil
}

// Label constructs a specially formated label using the annotationKey.
func Label(annotation string) string {
	return fmt.Sprintf("%s:%s", AnnotationKey, annotation)
}
