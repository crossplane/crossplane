// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package namespace

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func FuzzRenderRoles(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		ns := &corev1.Namespace{}
		ff.GenerateStruct(ns)
		crs := make([]rbacv1.ClusterRole, 0)
		ff.CreateSlice(&crs)
		if len(crs) == 0 {
			return
		}
		_ = RenderRoles(ns, crs)
	})
}
