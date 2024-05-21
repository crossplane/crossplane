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
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"github.com/spf13/afero"
)

func FuzzFindXpkgInDir(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		noOfFiles, err := ff.GetInt()
		if err != nil {
			t.Skip()
		}
		fs := afero.NewMemMapFs()
		createdFiles := make([]string, 0)

		defer func() {
			for _, createdFile := range createdFiles {
				fs.Remove(createdFile)
			}
		}()
		for range noOfFiles % 500 {
			fname, err := ff.GetString()
			if err != nil {
				t.Skip()
			}
			fcontents, err := ff.GetBytes()
			if err != nil {
				t.Skip()
			}

			if err = afero.WriteFile(fs, fname, fcontents, 0o777); err != nil {
				t.Skip()
			}
		}

		_, _ = FindXpkgInDir(fs, "/")
		_, _ = ParseNameFromMeta(fs, "/")
	})
}
