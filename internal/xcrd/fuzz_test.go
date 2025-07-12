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

package xcrd

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"

	v2 "github.com/crossplane/crossplane/apis/apiextensions/v2"
)

func FuzzForCompositeResourceXcrd(f *testing.F) {
	f.Fuzz(func(_ *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		xrd := &v2.CompositeResourceDefinition{}

		err := ff.GenerateStruct(xrd)
		if err != nil {
			return
		}

		_, _ = ForCompositeResource(xrd)
	})
}

func FuzzForCompositeResourceClaim(f *testing.F) {
	f.Fuzz(func(_ *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		xrd := &v2.CompositeResourceDefinition{}

		err := ff.GenerateStruct(xrd)
		if err != nil {
			return
		}

		_, _ = ForCompositeResourceClaim(xrd)
	})
}
