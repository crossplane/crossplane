// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package composition

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func FuzzNewCompositionRevision(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		f := fuzz.NewConsumer(data)
		c := &v1.Composition{}
		f.GenerateStruct(c)
		revision, err := f.GetInt()
		if err != nil {
			return
		}

		_ = NewCompositionRevision(c, int64(revision))
	})
}
