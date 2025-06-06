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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	apitest "k8s.io/apiextensions-apiserver/pkg/test"
	"k8s.io/apimachinery/pkg/runtime"

	apiextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	apiextv1beta1 "github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	apiextv2alpha1 "github.com/crossplane/crossplane/apis/apiextensions/v2alpha1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
	protectionv1beta1 "github.com/crossplane/crossplane/apis/protection/v1beta1"
)

type defaultsTestCase struct {
	reason    string
	defaultFn func(any)
	obj       any
	want      any
}

func makeDefaultsTestCase[T any](t *testing.T, path string) defaultsTestCase {
	return defaultsTestCase{
		reason:    "We should have expected default fields set after default is called.",
		defaultFn: DefaultFor[T](t),
		obj:       New[T](t),
		want:      apitest.MustLoadManifest[T](t, path),
	}
}

func TestDefaults(t *testing.T) {
	cases := map[string]defaultsTestCase{
		// apiextensions.crossplane.io/v2alpha1
		"apiextensions.v2alpha1.CompositeResourceDefinition": makeDefaultsTestCase[apiextv2alpha1.CompositeResourceDefinition](t, "defaults/apiextensions_v2alpha1_compositeresourcedefinition.yaml"),

		// apiextensions.crossplane.io/v1
		"apiextensions.v1.CompositeResourceDefinition": makeDefaultsTestCase[apiextv1.CompositeResourceDefinition](t, "defaults/apiextensions_v1_compositeresourcedefinition.yaml"),
		"apiextensions.v1.CompositionRevision":         makeDefaultsTestCase[apiextv1.CompositionRevision](t, "defaults/apiextensions_v1_compositionrevision.yaml"),
		"apiextensions.v1.Composition":                 makeDefaultsTestCase[apiextv1.Composition](t, "defaults/apiextensions_v1_composition.yaml"),

		// apiextensions.crossplane.io/v1beta1
		"apiextensions.v1beta1.EnvironmentConfig": makeDefaultsTestCase[apiextv1beta1.EnvironmentConfig](t, "defaults/apiextensions_v1beta1_environmentconfig.yaml"),
		"apiextensions.v1beta1.Usage":             makeDefaultsTestCase[apiextv1beta1.Usage](t, "defaults/apiextensions_v1beta1_usage.yaml"),

		// pkg.crossplane.io/v1
		"pkg.v1.ConfigurationRevision": makeDefaultsTestCase[pkgv1.ConfigurationRevision](t, "defaults/pkg_v1_configurationrevision.yaml"),
		"pkg.v1.Configuration":         makeDefaultsTestCase[pkgv1.Configuration](t, "defaults/pkg_v1_configuration.yaml"),
		"pkg.v1.Function":              makeDefaultsTestCase[pkgv1.Function](t, "defaults/pkg_v1_function.yaml"),
		"pkg.v1.FunctionRevision":      makeDefaultsTestCase[pkgv1.FunctionRevision](t, "defaults/pkg_v1_functionrevision.yaml"),
		"pkg.v1.ProviderRevision":      makeDefaultsTestCase[pkgv1.ProviderRevision](t, "defaults/pkg_v1_providerrevision.yaml"),
		"pkg.v1.Provider":              makeDefaultsTestCase[pkgv1.Provider](t, "defaults/pkg_v1_provider.yaml"),

		// pkg.crossplane.io/v1beta1
		"pkg.v1beta1.DeploymentRuntimeConfig": makeDefaultsTestCase[pkgv1beta1.DeploymentRuntimeConfig](t, "defaults/pkg_v1beta1_deploymentruntimeconfig.yaml"),
		"pkg.v1beta1.Function":                makeDefaultsTestCase[pkgv1beta1.Function](t, "defaults/pkg_v1beta1_function.yaml"),
		"pkg.v1beta1.FunctionRevision":        makeDefaultsTestCase[pkgv1beta1.FunctionRevision](t, "defaults/pkg_v1beta1_functionrevision.yaml"),
		"pkg.v1beta1.Lock":                    makeDefaultsTestCase[pkgv1beta1.Lock](t, "defaults/pkg_v1beta1_lock.yaml"),
		"pkg.v1beta1.ImageConfig":             makeDefaultsTestCase[pkgv1beta1.ImageConfig](t, "defaults/pkg_v1beta1_imageconfig.yaml"),

		// protection.crossplane.io/v1beta1
		"protection.v1beta1.ClusterUsage": makeDefaultsTestCase[protectionv1beta1.ClusterUsage](t, "defaults/protection_v1beta1_clusterusage.yaml"),
		"protection.v1beta1.Usage":        makeDefaultsTestCase[protectionv1beta1.Usage](t, "defaults/protection_v1beta1_usage.yaml"),
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
