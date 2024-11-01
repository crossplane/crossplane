/*
Copyright 2024 The Crossplane Authors.

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

package validate

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/google/go-containerregistry/pkg/crane"
	conregv1 "github.com/google/go-containerregistry/pkg/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const maxDecompressedSize = 200 * 1024 * 1024 // 200 MB

// ImageFetcher defines an interface for fetching images.
type ImageFetcher interface {
	FetchBaseLayer(image string) (*conregv1.Layer, error)
	FetchImage(image string) ([]conregv1.Layer, error)
}

// Fetcher implements the ImageFetcher interface.
type Fetcher struct{}

// FetchImage pulls the full image and extracts the CRDs folder to fetch .yaml files.
func (f *Fetcher) FetchImage(image string) ([]conregv1.Layer, error) {
	image, err := prepareImageReference(image)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare image reference")
	}

	// Pull the image
	img, err := crane.Pull(image)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to pull image")
	}

	// Extract the layers of the image into the temporary directory
	layers, err := img.Layers()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get image layers")
	}

	return layers, nil
}

// BaseLayerNotFoundError is returned when the base layer of the image could not be found.
type BaseLayerNotFoundError struct {
	image string
}

// Error implements the error interface.
func (e *BaseLayerNotFoundError) Error() string {
	return fmt.Sprintf("base layer not found for the image %s", e.image)
}

// NewBaseLayerNotFoundError returns a new BaseLayerNotFoundError error.
func NewBaseLayerNotFoundError(image string) error {
	return &BaseLayerNotFoundError{image: image}
}

// IsErrBaseLayerNotFound checks if the error is of type BaseLayerNotFoundError.
func IsErrBaseLayerNotFound(err error) bool {
	var e *BaseLayerNotFoundError
	return errors.As(err, &e)
}

// FetchBaseLayer fetches the base layer of the image which contains the 'package.yaml' file.
func (f *Fetcher) FetchBaseLayer(image string) (*conregv1.Layer, error) {
	image, err := prepareImageReference(image)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare image reference")
	}

	cBytes, err := crane.Config(image)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get config")
	}

	cfg := &conregv1.ConfigFile{}
	if err := yaml.Unmarshal(cBytes, cfg); err != nil {
		return nil, errors.Wrapf(err, "cannot unmarshal image config")
	}

	// TODO(ezgidemirel): consider using the annotations instead of labels to find out the base layer like package managed
	if cfg.Config.Labels == nil {
		return nil, errors.New("cannot get image labels")
	}

	var label string
	ls := cfg.Config.Labels
	for v, k := range ls {
		if k == baseLayerLabel {
			label = v // e.g.: io.crossplane.xpkg:sha256:0158764f65dc2a68728fdffa6ee6f2c9ef158f2dfed35abbd4f5bef8973e4b59
		}
	}
	if label == "" {
		return nil, NewBaseLayerNotFoundError(image)
	}

	lDigest := strings.SplitN(label, ":", 2)[1] // e.g.: sha256:0158764f65dc2a68728fdffa6ee6f2c9ef158f2dfed35abbd4f5bef8973e4b59

	ll, err := crane.PullLayer(fmt.Sprintf(refFmt, image, lDigest))
	if err != nil {
		return nil, errors.Wrapf(err, "cannot pull base layer %s", lDigest)
	}

	return &ll, nil
}

func findImageTagForVersionConstraint(image string) (string, error) {
	// Separate the image base and the image tag
	parts := strings.Split(image, ":")
	lastPart := len(parts) - 1
	imageBase := strings.Join(parts[0:lastPart], ":")
	imageTag := parts[lastPart]

	// Check if the tag is a constraint
	isConstraint := true
	c, err := semver.NewConstraint(imageTag)
	if err != nil {
		isConstraint = false
	}

	// Return original image if no constraint was detected
	if !isConstraint {
		return image, nil
	}

	// Fetch all image tags
	tags, err := crane.ListTags(imageBase)
	if err != nil {
		return "", errors.Wrapf(err, "cannot fetch tags for the image %s", imageBase)
	}

	// Convert tags to semver versions
	vs := []*semver.Version{}
	for _, r := range tags {
		v, err := semver.NewVersion(r)
		if err != nil {
			// We skip any tags that are not valid semantic versions
			continue
		}
		vs = append(vs, v)
	}

	// Sort all versions and find the last version complient with the constraint
	sort.Sort(sort.Reverse(semver.Collection(vs)))
	var addVer string
	for _, v := range vs {
		if c.Check(v) {
			addVer = v.Original()

			break
		}
	}

	if addVer == "" {
		return "", errors.Errorf("cannot find any tag complient with the constraint %s", imageTag)
	}

	// Compose new complete image string if any complient version was found
	image = fmt.Sprintf("%s:%s", imageBase, addVer)

	return image, nil
}

func extractPackageContent(layer conregv1.Layer) ([][]byte, []byte, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "cannot get uncompressed layer")
	}

	objs, err := load(rc)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "cannot read from layer")
	}

	// we need the meta object for identifying the dependencies
	var metaObj []byte
	if len(objs) > 0 {
		metaObj = objs[0]
	}

	// the first line of the layer is not part of the meta object, so we need to remove it
	metaStr := string(metaObj)
	metaLines := strings.Split(metaStr, "\n")
	metaStr = strings.Join(metaLines[1:], "\n")

	// the last obj is not yaml, so we need to remove it
	return objs[1 : len(objs)-1], []byte(metaStr), nil
}

func extractPackageCRDs(layers []conregv1.Layer) ([][]byte, error) {
	// Create a temporary directory to extract the files
	tmpDir, err := os.MkdirTemp("", "image-extract")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create temporary directory")
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("Failed to remove temporary directory: %v", err)
		}
	}()

	for _, layer := range layers {
		if err := extractLayer(layer, tmpDir); err != nil {
			return nil, errors.Wrapf(err, "failed to extract layer")
		}
	}

	// Search for .yaml files in the "crds" directory
	var yamlFiles [][]byte
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if the file is in the "crds" directory and has a .yaml extension
		if strings.Contains(path, "/crds/") && strings.HasSuffix(info.Name(), ".yaml") {
			content, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				return errors.Wrapf(err, "failed to read file: %s", path)
			}
			yamlFiles = append(yamlFiles, content)
		}

		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to walk through extracted files")
	}

	return yamlFiles, nil
}

// extractLayer extracts the contents of a layer to the specified directory.
func extractLayer(layer conregv1.Layer, destDir string) error { //nolint:gocognit // no extra func
	r, err := layer.Uncompressed()
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			log.Printf("Failed to close reader: %v", err)
		}
	}()

	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break // End of tar archive
		}
		if err != nil {
			return err
		}

		// Resolve the target path
		target := filepath.Join(destDir, filepath.Clean(hdr.Name))
		targetPath, err := filepath.Abs(target)
		if err != nil {
			return errors.Wrap(err, "failed to get absolute path")
		}

		// Skip entries that are the same as the destination directory or just "./"
		if targetPath == filepath.Clean(destDir) || hdr.Name == "./" {
			continue
		}

		// Ensure the target path is within the destination directory
		if !strings.HasPrefix(targetPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return errors.Errorf("invalid file path: %s", targetPath)
		}

		// Create the file or directory
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o750); err != nil {
				return errors.Wrapf(err, "cannot create directory: %s", targetPath)
			}
		case tar.TypeReg:
			dir := filepath.Dir(targetPath)
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return errors.Wrapf(err, "cannot create directory: %s", dir)
			}
			file, err := os.Create(filepath.Clean(targetPath))
			if err != nil {
				return errors.Wrapf(err, "cannot create file: %s", targetPath)
			}
			defer func() {
				if err := file.Close(); err != nil {
					log.Printf("Failed to close file: %v", err)
				}
			}()

			// Limit the decompression size to avoid DoS attacks
			limitedReader := io.LimitReader(tr, maxDecompressedSize)
			if _, err := io.Copy(file, limitedReader); err != nil {
				return errors.Wrapf(err, "cannot decompress file: %s", targetPath)
			}
		}
	}

	return nil
}

// prepareImageReference prepares the image reference by stripping the digest or resolving the tag if necessary.
func prepareImageReference(image string) (string, error) {
	if strings.Contains(image, "@") {
		return strings.SplitN(image, "@", 2)[0], nil
	}
	if strings.Contains(image, ":") {
		return findImageTagForVersionConstraint(image)
	}
	return image, nil
}
