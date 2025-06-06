/*
Copyright 2025 The Crossplane Authors.

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

package v1beta1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	apitest "k8s.io/apiextensions-apiserver/pkg/test"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/test/resources"
)

func TestImageConfigDefaults(t *testing.T) {
	cases := map[string]struct {
		reason    string
		defaultFn func(any)
		obj       any
		want      any
	}{
		"FromEmpty": {
			reason:    "We should have expected default fields set after default is called.",
			defaultFn: resources.DefaultFor[v1beta1.ImageConfig](t),
			obj:       resources.New[v1beta1.ImageConfig](t),
			want:      apitest.MustLoadManifest[v1beta1.ImageConfig](t, "imageconfig.yaml"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := tc.obj.(runtime.Object)
			if !ok {
				t.Fatalf("could not convert test case object to runtime.Object")
			}
			got = got.DeepCopyObject()
			tc.defaultFn(got)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Default(): -want, +got:\n%s", diff)
			}
		})
	}
}