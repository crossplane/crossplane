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

package pkg

import (
	"io/ioutil"
	"path/filepath"

	"github.com/ghodss/yaml"
)

// CrossplanePackage is a minimal struct to grab the name field out of the crossplane.yaml file
type CrossplanePackage struct {
	Metadata struct {
		Name string `json:"name"`
	}
}

func parseNameFromPackageFile(path string) (string, error) {
	bs, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	pkgName, err := parseNameFromPackage(bs)
	if err != nil {
		return "", err
	}
	return pkgName, nil
}

func parseNameFromPackage(bs []byte) (string, error) {
	p := &CrossplanePackage{}
	err := yaml.Unmarshal(bs, p)
	return p.Metadata.Name, err
}
