/*
Copyright 2025 The Crossplane Authors.

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

package render

import (
	"encoding/json"
	iofs "io/fs"
	"path/filepath"

	"github.com/spf13/afero"
	"k8s.io/kube-openapi/pkg/spec3"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// LoadRequiredSchemas loads OpenAPI v3 schema documents from a directory,
// recursively. Each file should contain a single OpenAPI v3 document in JSON
// format (as returned by /openapi/v3/<group-version>).
func LoadRequiredSchemas(fs afero.Fs, dir string) ([]spec3.OpenAPI, error) {
	var files []string

	err := iofs.WalkDir(afero.NewIOFS(fs), dir, func(path string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".json" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "cannot walk directory")
	}

	if len(files) == 0 {
		return nil, errors.Errorf("no JSON files found in %q", dir)
	}

	schemas := make([]spec3.OpenAPI, 0, len(files))
	for _, file := range files {
		data, err := afero.ReadFile(fs, file)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot read file %q", file)
		}

		s := spec3.OpenAPI{}
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, errors.Wrapf(err, "cannot parse OpenAPI JSON from %q", file)
		}

		schemas = append(schemas, s)
	}

	return schemas, nil
}
