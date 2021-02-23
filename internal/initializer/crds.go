/*
Copyright 2021 The Crossplane Authors.

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

package initializer

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/pkg/errors"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// NewCoreCRDs returns a new *CoreCRDs.
func NewCoreCRDs(path string) *CoreCRDs {
	return &CoreCRDs{Path: path}
}

// CoreCRDs makes sure the CRDs are installed.
type CoreCRDs struct {
	Path string
}

// Run applies all CRDs in the given directory.
func (c *CoreCRDs) Run(ctx context.Context, kube resource.ClientApplicator) error { // nolint:gocyclo
	var crds []*v1.CustomResourceDefinition
	err := filepath.Walk(c.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(filepath.Clean(path))
		if err != nil {
			return errors.Wrapf(err, "cannot read file %s", path)
		}
		yr := yaml.NewYAMLReader(bufio.NewReader(file))
		for {
			bytes, err := yr.Read()
			if err != nil && err != io.EOF {
				return errors.Wrap(err, "cannot read YAML")
			}
			if err == io.EOF {
				break
			}
			if len(bytes) < 5 {
				continue
			}
			crd := &v1.CustomResourceDefinition{}
			if err := yaml.Unmarshal(bytes, crd); err != nil {
				return errors.Wrap(err, "cannot unmarshal YAML file into CustomResourceDefinition struct")
			}
			crds = append(crds, crd)
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "cannot walk the crds directory")
	}
	for _, crd := range crds {
		if err := kube.Apply(ctx, crd); err != nil {
			return errors.Wrap(err, "cannot create crd")
		}
	}
	return nil
}
