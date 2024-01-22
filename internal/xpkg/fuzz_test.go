// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

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
		for i := 0; i < noOfFiles%500; i++ {
			fname, err := ff.GetString()
			if err != nil {
				t.Skip()
			}
			fcontents, err := ff.GetBytes()
			if err != nil {
				t.Skip()
			}

			if err = afero.WriteFile(fs, fname, fcontents, 0777); err != nil {
				t.Skip()
			}
		}

		_, _ = FindXpkgInDir(fs, "/")
		_, _ = ParseNameFromMeta(fs, "/")
	})

}
