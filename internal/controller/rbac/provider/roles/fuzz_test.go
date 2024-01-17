// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package roles

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

func FuzzRenderClusterRoles(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		pr := &v1.ProviderRevision{}
		ff.GenerateStruct(pr)
		rs := make([]Resource, 0)
		ff.CreateSlice(&rs)
		if len(rs) == 0 {
			return
		}

		_ = RenderClusterRoles(pr, rs)
	})
}
