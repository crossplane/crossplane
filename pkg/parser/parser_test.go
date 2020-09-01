/*
Copyright 2020 The Crossplane Authors.

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

package parser

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	apiextensionsv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	pkgmetav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
)

var xrd = &apiextensionsv1alpha1.CompositeResourceDefinition{
	TypeMeta: metav1.TypeMeta{
		Kind:       apiextensionsv1alpha1.CompositeResourceDefinitionKind,
		APIVersion: "apiextensions.crossplane.io/v1alpha1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "test",
	},
	Spec: apiextensionsv1alpha1.CompositeResourceDefinitionSpec{},
}

var comp = &apiextensionsv1alpha1.Composition{
	TypeMeta: metav1.TypeMeta{
		Kind:       apiextensionsv1alpha1.CompositionKind,
		APIVersion: "apiextensions.crossplane.io/v1alpha1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "test",
	},
	Spec: apiextensionsv1alpha1.CompositionSpec{},
}

var crd = &apiextensions.CustomResourceDefinition{
	TypeMeta: metav1.TypeMeta{
		Kind:       "CustomResourceDefinition",
		APIVersion: "apiextensions.k8s.io/v1beta1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "test",
	},
	Spec: apiextensions.CustomResourceDefinitionSpec{},
}

var provider = &pkgmetav1alpha1.Provider{
	TypeMeta: metav1.TypeMeta{
		Kind:       pkgmetav1alpha1.ProviderKind,
		APIVersion: "meta.pkg.crossplane.io/v1alpha1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "test",
	},
}

var configuration = &pkgmetav1alpha1.Configuration{
	TypeMeta: metav1.TypeMeta{
		Kind:       pkgmetav1alpha1.ConfigurationKind,
		APIVersion: "meta.pkg.crossplane.io/v1alpha1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "test",
	},
}

func TestParser(t *testing.T) {
	xrdBytes, _ := yaml.Marshal(xrd)
	compBytes, _ := yaml.Marshal(comp)
	crdBytes, _ := yaml.Marshal(crd)
	providerBytes, _ := yaml.Marshal(provider)
	configurationBytes, _ := yaml.Marshal(configuration)
	allBytes := bytes.Join([][]byte{providerBytes, configurationBytes, compBytes, crdBytes, xrdBytes}, []byte("\n---\n"))
	fs := afero.NewMemMapFs()
	_ = afero.WriteFile(fs, "xrd.yaml", xrdBytes, 0o644)
	_ = afero.WriteFile(fs, "comp.yaml", compBytes, 0o644)
	_ = afero.WriteFile(fs, "crd.yaml", crdBytes, 0o644)
	_ = afero.WriteFile(fs, "provider.yaml", providerBytes, 0o644)
	_ = afero.WriteFile(fs, "some/nested/dir/configuration.yaml", configurationBytes, 0o644)
	_ = afero.WriteFile(fs, ".crossplane/bad.yaml", configurationBytes, 0o644)
	allFs := afero.NewMemMapFs()
	_ = afero.WriteFile(allFs, "all.yaml", allBytes, 0o644)
	errFs := afero.NewMemMapFs()
	_ = afero.WriteFile(errFs, "bad.yaml", []byte("definitely not yaml"), 0o644)
	emptyFs := afero.NewMemMapFs()
	_ = afero.WriteFile(emptyFs, "empty.yaml", []byte(""), 0o644)
	_ = afero.WriteFile(emptyFs, "bad.yam", []byte("definitely not yaml"), 0o644)

	cases := map[string]struct {
		reason  string
		parser  Parser
		opts    []BackendOption
		pkg     *Package
		wantErr bool
	}{
		"EchoBackendEmpty": {
			reason: "should have empty output with empty input",
			parser: New(NewEchoBackend("")),
			pkg:    NewPackage(),
		},
		"EchoBackendError": {
			reason:  "should have error with invalid yaml",
			parser:  New(NewEchoBackend("definitely not yaml")),
			pkg:     NewPackage(),
			wantErr: true,
		},
		"EchoBackend": {
			reason: "should parse input stream successfully",
			parser: New(NewEchoBackend(string(allBytes))),
			pkg: &Package{
				provider:                     provider,
				configuration:                configuration,
				customResourceDefinitions:    map[string]*apiextensions.CustomResourceDefinition{crd.GetName(): crd},
				compositeResourceDefinitions: map[string]*apiextensionsv1alpha1.CompositeResourceDefinition{xrd.GetName(): xrd},
				compositions:                 map[string]*apiextensionsv1alpha1.Composition{comp.GetName(): comp},
			},
		},
		"NopBackend": {
			reason: "should never parse any objects and never return an error",
			parser: New(NewNopBackend()),
			pkg:    NewPackage(),
		},
		"FsBackend": {
			reason: "should parse filesystem successfully",
			parser: New(NewFsBackend(fs, FsDir("."), FsSkips(SkipDirs(), SkipNotYaml(), SkipPath("\\.crossplane")))),
			pkg: &Package{
				provider:                     provider,
				configuration:                configuration,
				customResourceDefinitions:    map[string]*apiextensions.CustomResourceDefinition{crd.GetName(): crd},
				compositeResourceDefinitions: map[string]*apiextensionsv1alpha1.CompositeResourceDefinition{xrd.GetName(): xrd},
				compositions:                 map[string]*apiextensionsv1alpha1.Composition{comp.GetName(): comp},
			},
		},
		"FsBackendAll": {
			reason: "should parse filesystem successfully with multiple objects in single file",
			parser: New(NewFsBackend(allFs, FsDir("."))),
			pkg: &Package{
				provider:                     provider,
				configuration:                configuration,
				customResourceDefinitions:    map[string]*apiextensions.CustomResourceDefinition{crd.GetName(): crd},
				compositeResourceDefinitions: map[string]*apiextensionsv1alpha1.CompositeResourceDefinition{xrd.GetName(): xrd},
				compositions:                 map[string]*apiextensionsv1alpha1.Composition{comp.GetName(): comp},
			},
		},
		"FsBackendError": {
			reason:  "should error if yaml file with invalid yaml",
			parser:  New(NewFsBackend(fs, FsDir("."))),
			pkg:     NewPackage(),
			wantErr: true,
		},
		"FsBackendSkip": {
			reason: "should skip empty files and files without yaml extension",
			parser: New(NewFsBackend(emptyFs, FsDir("."), FsSkips(SkipDirs(), SkipNotYaml()))),
			pkg:    NewPackage(),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pkg, err := tc.parser.Parse(context.TODO(), tc.opts...)
			if err != nil && !tc.wantErr {
				t.Errorf("Parse(...): unexpected error: %s", err)
			}
			if tc.wantErr {
				return
			}
			if diff := cmp.Diff(tc.pkg.GetProvider(), pkg.GetProvider()); diff != "" {
				t.Errorf("Provider: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.pkg.GetConfiguration(), pkg.GetConfiguration()); diff != "" {
				t.Errorf("Configuration: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.pkg.GetCompositeResourceDefinitions(), pkg.GetCompositeResourceDefinitions()); diff != "" {
				t.Errorf("XRDs: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.pkg.GetCompositions(), pkg.GetCompositions()); diff != "" {
				t.Errorf("Compositions: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.pkg.GetCustomResourceDefinitions(), pkg.GetCustomResourceDefinitions()); diff != "" {
				t.Errorf("CRDs: -want, +got:\n%s", diff)
			}
		})
	}
}
