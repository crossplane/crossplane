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
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Error strings.
const (
	errLayer  = "cannot get image layers"
	errDigest = "cannot get image digest"
)

// Layer creates a v1.Layer that represents the layer contents for the xpkg and
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

	// Add annotation label to config if a non-empty label is specified. This is
	// an intermediary step. AnnotateLayers must be called on an image for it to
	// have valid layer annotations. It propagates these labels to annotations
	// on the layers.
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

// NOTE(negz): AnnotateLayers originated in upbound/up. I was confused why we
// store layer annotations as labels in the OCI config file when we build a
// package, then propagate them to OCI layer annotations when we push one. I
// believe this is because an xpkg file is really an OCI image tarball, and the
// tarball format doesn't support layer annotations (or may just lose them in
// some circumstances?), so we're using the config file to store them.
// See https://github.com/upbound/up/pull/177#discussion_r866776584.

// AnnotateLayers propagates labels from the supplied image's config file to
// annotations on its layers.
func AnnotateLayers(i v1.Image) (v1.Image, error) {
	cfgFile, err := i.ConfigFile()
	if err != nil {
		return nil, errors.Wrap(err, errConfigFile)
	}

	layers, err := i.Layers()
	if err != nil {
		return nil, errors.Wrap(err, errLayer)
	}

	addendums := make([]mutate.Addendum, 0)

	for _, l := range layers {
		d, err := l.Digest()
		if err != nil {
			return nil, errors.Wrap(err, errDigest)
		}
		if annotation, ok := cfgFile.Config.Labels[Label(d.String())]; ok {
			addendums = append(addendums, mutate.Addendum{
				Layer: l,
				Annotations: map[string]string{
					AnnotationKey: annotation,
				},
			})
			continue
		}
		addendums = append(addendums, mutate.Addendum{
			Layer: l,
		})
	}

	// we didn't find any annotations, return original image
	if len(addendums) == 0 {
		return i, nil
	}

	img := empty.Image
	for _, a := range addendums {
		img, err = mutate.Append(img, a)
		if err != nil {
			return nil, errors.Wrap(err, errBuildImage)
		}
	}

	return mutate.ConfigFile(img, cfgFile)
}
