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
	"fmt"
	"io/fs"
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
// image should be a validate image name with the format: <registry>/<image>:<tag>.
func (c *LocalCache) Load(img string) ([]*unstructured.Unstructured, error) {
	image, err := findImageTagForVersionConstraint(img)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot resolve image tag for %s", img)
	}

	cacheImagePath := c.getCachePath(image)

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
// If the image tag is a version constraint,
// check cache for latest version that matches the constraint
func (c *LocalCache) Exists(img string) (string, error) {
	isConstraint := true
	imageBase, imageTag := separateImageTag(img)

	constraint, err := semver.NewConstraint(imageTag)
	if err != nil {
		isConstraint = false
	}

	cachePath := c.getCachePath(img)

	if isConstraint {
		// search cache-directory for tags
		cacheDir := filepath.Dir(cachePath)

		tags := []string{}
		err := filepath.WalkDir(cacheDir, func(p string, d fs.DirEntry, err error) error {
			if d.IsDir() && strings.HasPrefix(d.Name(), path.Base(imageBase)) {
				i := strings.Index(d.Name(), "@")
				if i == -1 {
					return errors.New(fmt.Sprintf("the cache entry '%s' does not contain a tag", d.Name()))
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
			}
		}

		if latestVersionInConstraint == "" {
			// no version that is valid for constraint exist
			return "", nil
		}

		// replace the constraint with latest version
		return strings.Replace(cachePath, imageTag, latestVersionInConstraint, 0), nil
	}

	_, err = os.Stat(cachePath)
	if err != nil && os.IsNotExist(err) {
		return cachePath, nil
	} else if err != nil {
		return "", errors.Wrapf(err, "cannot stat file %s", cachePath)
	}

	return "", nil
}

// getCachePath transforms an image name to a validate folder path that store schemas.
func (c *LocalCache) getCachePath(image string) string {
	cacheImagePath := strings.ReplaceAll(image, ":", "@")
	return filepath.Join(c.cacheDir, cacheImagePath)
}
