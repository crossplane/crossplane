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
	iofs "io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/crossplane/crossplane/v2/cmd/crank/common/load"
)

// Cache defines an interface for caching schemas.
type Cache interface {
	Store(schemas [][]byte, path string) error
	Flush() error
	Init() error
	Load(image string) ([]*unstructured.Unstructured, error)
	Exists(image string) (string, error)
}

// LocalCache implements the Cache interface.
type LocalCache struct {
	fs       afero.Fs
	cacheDir string
}

// Store stores the schemas in the directory.
func (c *LocalCache) Store(schemas [][]byte, path string) error {
	path = c.getCachePath(path)

	if err := c.fs.MkdirAll(path, os.ModePerm); err != nil {
		return errors.Wrapf(err, "cannot create directory %s", path)
	}

	file, err := c.fs.Create(filepath.Join(path, packageFileName))
	if err != nil {
		return errors.Wrapf(err, "cannot create file")
	}

	for _, s := range schemas {
		_, err := file.Write(s)
		if err != nil {
			return errors.Wrapf(err, "cannot write to file")
		}

		_, err = file.WriteString("---\n")
		if err != nil {
			return errors.Wrapf(err, "cannot write to file")
		}
	}

	return nil
}

// Init creates the cache directory if it doesn't exist.
func (c *LocalCache) Init() error {
	if _, err := c.fs.Stat(c.cacheDir); os.IsNotExist(err) {
		if err := c.fs.MkdirAll(c.cacheDir, os.ModePerm); err != nil {
			return errors.Wrapf(err, "cannot create cache directory %s", c.cacheDir)
		}
	} else if err != nil {
		return errors.Wrapf(err, "cannot stat cache directory %s", c.cacheDir)
	}

	return nil
}

// Flush removes the cache directory.
func (c *LocalCache) Flush() error {
	return c.fs.RemoveAll(c.cacheDir)
}

// Load loads schemas from the cache directory.
// image should be a validated image name with the format: <registry>/<image>:<tag>.
// <tag> can be a constraint, in which case the latest version of the schema that satisfies this constraint
// is loaded from the cache.
func (c *LocalCache) Load(image string) ([]*unstructured.Unstructured, error) {
	cacheImagePath := c.getCachePath(image)
	imageBase, imageTag := separateImageTag(image)

	if isRangedConstraint(imageTag) {
		var err error
		cacheImagePath, err = c.findLatestCachedVersionForConstraint(image)
		if err != nil {
			return nil, errors.Wrapf(err,
				"failed to scan cache for entries that matches image %s with the constraint %s",
				imageBase, imageTag)
		}

		if cacheImagePath == "" {
			return []*unstructured.Unstructured{}, nil
		}
	}

	loader, err := load.NewLoader(cacheImagePath)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create loader from %s", cacheImagePath)
	}

	schemas, err := loader.Load()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot load schemas from %s", cacheImagePath)
	}

	return schemas, nil
}

// Exists checks if the cache contains the image and returns the path if it doesn't exist.
// If the image contains a semantic version constraint, the returned cache-path will include it on cache miss,
// as it can not be resolved by the cache.
func (c *LocalCache) Exists(image string) (string, error) {
	path := c.getCachePath(image)

	_, imageTag := separateImageTag(image)

	// if the image-tag is a ranged constraint we need to try to find the latest cached version that satisfies that constraint
	if isRangedConstraint(imageTag) {
		v, err := c.findLatestCachedVersionForConstraint(image)
		if err != nil {
			return "", errors.Wrapf(err, "failed to scan cache for constraint")
		}

		if v == "" {
			// valid version for constraint not found
			return path, nil
		}

		// valid version for constraint was found
		return "", nil
	}

	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return path, nil
	} else if err != nil {
		return "", errors.Wrapf(err, "cannot stat file %s", path)
	}

	return "", nil
}

// getCachePath transforms an image name to a validate folder path that store schemas.
func (c *LocalCache) getCachePath(image string) string {
	cacheImagePath := strings.ReplaceAll(image, ":", "@")
	return filepath.Join(c.cacheDir, cacheImagePath)
}

// isConstraint checks if a string is a semantic version constraint, but not an exact version.
func isRangedConstraint(tag string) bool {
	if _, err := semver.NewVersion(tag); err == nil {
		return false
	}
	_, err := semver.NewConstraint(tag)
	return err == nil
}

// findLatestCachedVersionForConstraint returns the cache-path for the latest tag that matches the image version constraint.
// On cache miss, an empty string is returned.
// The image must be a valid image name with the format: <registry>/<image>:<tag>.
func (c *LocalCache) findLatestCachedVersionForConstraint(image string) (string, error) {
	imageBase, imageTag := separateImageTag(image)

	constraint, err := semver.NewConstraint(imageTag)
	if err != nil {
		return "", errors.Wrapf(err, "%s is not a valid constraint", imageTag)
	}

	cachePath := c.getCachePath(image)

	// search cache-directory for tags
	cacheDir := filepath.Dir(cachePath)

	tags := []string{}
	err = filepath.WalkDir(cacheDir, func(_ string, d iofs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, iofs.ErrNotExist) {
				// the walk will fail on first run (directories dont exist) - ignore it
				return nil
			}

			return err
		}

		if d.IsDir() && strings.HasPrefix(d.Name(), path.Base(imageBase)) {
			i := strings.Index(d.Name(), "@")
			if i == -1 {
				return errors.Errorf("the cache entry '%s' does not contain a tag", d.Name())
			}

			tags = append(tags, d.Name()[i+1:])
		}

		return nil
	})
	if err != nil {
		return "", errors.Wrapf(err, "failed to search cache-directory %s for existing tag", cacheDir)
	}

	if len(tags) == 0 {
		return "", nil
	}

	vs := convertToSemver(tags)

	sort.Sort(sort.Reverse(semver.Collection(vs)))

	var latestVersionInConstraint string
	for _, v := range vs {
		if constraint.Check(v) {
			latestVersionInConstraint = v.Original()

			break
		}
	}

	if latestVersionInConstraint == "" {
		// no version that is valid for constraint exist
		return "", nil
	}

	// return the cache-path with the latest valid version instead of the constraint
	return strings.ReplaceAll(cachePath, imageTag, latestVersionInConstraint), nil
}
