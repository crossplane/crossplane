// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package xcrd

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func FuzzForCompositeResourceXcrd(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		xrd := &v1.CompositeResourceDefinition{}
		err := ff.GenerateStruct(xrd)
		if err != nil {
			return
		}
		_, _ = ForCompositeResource(xrd)
	})
}

func FuzzForCompositeResourceClaim(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		xrd := &v1.CompositeResourceDefinition{}
		err := ff.GenerateStruct(xrd)
		if err != nil {
			return
		}
		_, _ = ForCompositeResourceClaim(xrd)
	})
}
