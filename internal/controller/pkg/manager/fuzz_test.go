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

package manager

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"

	"github.com/crossplane/crossplane/v2/internal/xpkg"
)

func FuzzFriendlyID(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)

		n, err := ff.GetString()
		if err != nil {
			t.Skip()
		}

		h, err := ff.GetString()
		if err != nil {
			t.Skip()
		}

		_ = xpkg.FriendlyID(n, h)
	})
}
