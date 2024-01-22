// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/xpkg"
	"github.com/crossplane/crossplane/internal/xpkg/fake"
)

func FuzzPackageRevision(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		pkg := &v1.Provider{}
		ff.GenerateStruct(pkg)
		fetcher := &fake.MockFetcher{
			MockHead: fake.NewMockHeadFn(nil, errors.New("boom")),
		}
		r := NewPackageRevisioner(fetcher)
		_, _ = r.Revision(context.Background(), pkg)
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
