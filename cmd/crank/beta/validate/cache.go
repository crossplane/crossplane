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
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Cache defines an interface for caching schemas
type Cache interface {
	Store([][]byte, string) error
	Flush() error
	Init() error
	Load() ([]*unstructured.Unstructured, error)
	Exists(string) (string, error)
}

// LocalCache implements the Cache interface
type LocalCache struct {
	fs       afero.Fs
	cacheDir string
}

// Store stores the schemas in the directory
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

// Init creates the cache directory if it doesn't exist
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

// Flush removes the cache directory
func (c *LocalCache) Flush() error {
	fmt.Println("flushing cache directory: ", c.cacheDir)
	return c.fs.RemoveAll(c.cacheDir)
}

// Load loads the schemas from the cache directory
func (c *LocalCache) Load() ([]*unstructured.Unstructured, error) {
	loader, err := NewLoader(c.cacheDir)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create loader from the path %s", c.cacheDir)
	}

	schemas, err := loader.Load()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot load schemas from the path %s", c.cacheDir)
	}

	return schemas, nil
}

// Exists checks if the cache contains the image and returns the path if it doesn't exist
func (c *LocalCache) Exists(image string) (string, error) {
	fName := strings.ReplaceAll(image, ":", "@")
	path := filepath.Join(c.cacheDir, fName)

	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return path, nil
	} else if err != nil {
		return "", errors.Wrapf(err, "cannot stat file %s", path)
	}

	return "", nil
}
