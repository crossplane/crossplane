/*
Copyright 2023 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICEE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIO OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package namespace

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func FuzzRenderRoles(f *testing.F) {
	f.Fuzz(func(_ *testing.T, data []byte) {
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
