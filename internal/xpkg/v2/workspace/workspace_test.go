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

package workspace

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/internal/xpkg/v2"
	"github.com/crossplane/crossplane/internal/xpkg/v2/parser/yaml"
	"github.com/crossplane/crossplane/internal/xpkg/v2/workspace/meta"
)

var (
	testComposition      []byte
	testInvalidXRD       []byte
	testMultipleObject   []byte
	testMultiVersionCRD  []byte
	testSingleVersionCRD []byte
)

func init() {
	testComposition, _ = afero.ReadFile(afero.NewOsFs(), "testdata/composition.yaml")
	testInvalidXRD, _ = afero.ReadFile(afero.NewOsFs(), "testdata/invalid-xrd.yaml")
	testMultipleObject, _ = afero.ReadFile(afero.NewOsFs(), "testdata/multiple-object.yaml")
	testMultiVersionCRD, _ = afero.ReadFile(afero.NewOsFs(), "testdata/multiple-version-crd.yaml")
	testSingleVersionCRD, _ = afero.ReadFile(afero.NewOsFs(), "testdata/single-version-crd.yaml")
}

func TestParse(t *testing.T) {
	ctx := context.Background()
	cases := map[string]struct {
		reason    string
		opts      []Option
		nodes     map[NodeIdentifier]struct{}
		err       error
		errString string
	}{
		"ErrorRootNotExist": {
			reason: "Should return an error if the workspace root does not exist.",
			opts:   []Option{WithFS(afero.NewMemMapFs())},
			err:    &os.PathError{Op: "open", Path: "/ws", Err: afero.ErrFileNotFound},
		},
		"ErrorRootNotExistPermissive": {
			reason: "Should return an error if the workspace root does not exist.",
			opts:   []Option{WithFS(afero.NewMemMapFs()), WithPermissiveParser()},
			err:    &os.PathError{Op: "open", Path: "/ws", Err: afero.ErrFileNotFound},
		},
		"SuccessfulEmpty": {
			reason: "Should not return an error if the workspace is empty.",
			opts: []Option{WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()

				// NOTE(tnthornton) we're currently seeing `internal compiler error: OCALLMETH missed by typecheck`
				// without this type check.
				// TODO(tnthornton) file an issue upstream.
				mem := fs.(*afero.MemMapFs)
				_ = mem.Mkdir("/ws", os.ModePerm)

				return fs
			}())},
		},
		"SuccessfulNoKubernetesObjects": {
			// NOTE(hasheddan): this test reflects current behavior, but we
			// should be surfacing errors / diagnostics if we fail to parse
			// objects in a package unless they are specified to be ignored. We
			// likely also want skip any non-YAML files by default as we do in
			// package parsing.
			reason: "Should have no package nodes if no Kubernetes objects are present.",
			opts: []Option{WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/somerandom.yaml", []byte(`{"some non kube":"yaml"}`), os.ModePerm)
				return fs
			}())},
		},
		"ErrorInvalidYAML": {
			reason: "Should have no package nodes if no Kubernetes objects are present.",
			opts: []Option{WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/invalid.yaml", []byte(`{some invalid yaml}`), os.ModePerm)
				return fs
			}())},
			errString: "failed to parse file invalid.yaml: [1:1] unterminated flow mapping\n    >  1 | {some invalid yaml}\n           ^",
		},
		"SuccessfulInvalidYAMLPermissive": {
			reason: "Should have no package nodes if no Kubernetes objects are present.",
			opts: []Option{WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/invalid.yaml", []byte(`{some invalid yaml}`), os.ModePerm)
				return fs
			}()), WithPermissiveParser()},
		},
		"SuccessfulParseComposition": {
			reason: "Should add a package node for Composition and every embedded resource.",
			opts: []Option{WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/composition.yaml", testComposition, os.ModePerm)
				return fs
			}())},
			nodes: map[NodeIdentifier]struct{}{
				nodeID("", schema.FromAPIVersionAndKind("ec2.aws.crossplane.io/v1beta1", "VPC")):               {},
				nodeID("", schema.FromAPIVersionAndKind("ec2.aws.crossplane.io/v1beta1", "Subnet")):            {},
				nodeID("vpcpostgresqlinstances.aws.database.example.org", xpextv1.CompositionGroupVersionKind): {},
			},
		},
		"SuccessfulParseMultipleSameFile": {
			reason: "Should add a package node for every resource when multiple objects exist in single file.",
			opts: []Option{WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/multiple.yaml", testMultipleObject, os.ModePerm)
				return fs
			}())},
			nodes: map[NodeIdentifier]struct{}{
				nodeID("compositepostgresqlinstances.database.example.org", xpextv1.CompositeResourceDefinitionGroupVersionKind): {},
				nodeID("some.other.xrd", xpextv1.CompositeResourceDefinitionGroupVersionKind):                                    {},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ws, _ := New("/ws", tc.opts...)
			err := ws.Parse(ctx)

			if tc.errString != "" {
				var errString string
				if err != nil {
					errString = err.Error()
				}
				if diff := cmp.Diff(tc.errString, errString); diff != "" {
					t.Errorf("\n%s\nParse(...): -want error, +got error:\n%s", tc.reason, diff)
				}
			} else if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nParse(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if len(tc.nodes) != len(ws.view.nodes) {
				t.Errorf("\n%s\nParse(...): -want node count: %d, +got node count: %d", tc.reason, len(tc.nodes), len(ws.view.nodes))
			}
			for id := range ws.view.nodes {
				if _, ok := tc.nodes[id]; !ok {
					t.Errorf("\n%s\nParse(...): missing node:\n%v", tc.reason, id)
				}
			}
		})
	}
}

func TestRWMetaFile(t *testing.T) {

	cfgMetaFile := &metav1.Configuration{
		TypeMeta: apimetav1.TypeMeta{
			APIVersion: "meta.pkg.crossplane.io/v1",
			Kind:       "Configuration",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name: "getting-started-with-aws",
		},
		Spec: metav1.ConfigurationSpec{
			MetaSpec: metav1.MetaSpec{
				Crossplane: &metav1.CrossplaneConstraints{
					Version: ">=1.0.0-0",
				},
				DependsOn: []metav1.Dependency{
					{
						Configuration: pointer.String("crossplane/provider-aws"),
						Version:       "v1.0.0",
					},
				},
			},
		},
	}

	providerMetaFile := &metav1.Provider{
		TypeMeta: apimetav1.TypeMeta{
			APIVersion: "meta.pkg.crossplane.io/v1",
			Kind:       "Provider",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name: "getting-started-with-aws",
		},
		Spec: metav1.ProviderSpec{
			Controller: metav1.ControllerSpec{
				Image: pointer.String("crossplane/provider-aws"),
			},
			MetaSpec: metav1.MetaSpec{
				Crossplane: &metav1.CrossplaneConstraints{
					Version: ">=1.0.0-0",
				},
				DependsOn: []metav1.Dependency{
					{
						Provider: pointer.String("crossplane/provider-aws"),
						Version:  "v1.0.0",
					},
				},
			},
		},
	}

	type args struct {
		opt      Option
		metaFile runtime.Object
	}

	type want struct {
		metaFile  metav1.Pkg
		readErr   error
		writerErr error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoPriorCfgFile": {
			reason: "Should create file and read it back in without modification.",
			args: args{
				opt:      WithFS(afero.NewMemMapFs()),
				metaFile: cfgMetaFile,
			},
			want: want{
				metaFile: cfgMetaFile,
			},
		},
		"NoPriorProviderFile": {
			reason: "Should create file and read it back in without modification.",
			args: args{
				opt:      WithFS(afero.NewMemMapFs()),
				metaFile: providerMetaFile,
			},
			want: want{
				metaFile: providerMetaFile,
			},
		},
		"ErrReturnedWhenCannotWrite": {
			reason: "Should return an error if we cannot write to the fs.",
			args: args{
				opt:      WithFS(afero.NewReadOnlyFs(afero.NewMemMapFs())),
				metaFile: providerMetaFile,
			},
			want: want{
				writerErr: syscall.EPERM,
				readErr:   &os.PathError{Op: "open", Path: "/tmp/crossplane.yaml", Err: afero.ErrFileNotFound},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// init workspace that is specific to this test
			ws, _ := New("/tmp", tc.args.opt)

			err := ws.Write(meta.New(tc.args.metaFile))

			if diff := cmp.Diff(tc.want.writerErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRWMetaFile(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			// read meta file
			pkgBytes, err := afero.ReadFile(ws.fs, filepath.Join(ws.root, xpkg.MetaFile))
			if diff := cmp.Diff(tc.want.readErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRWMetaFile(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			// parse meta file
			parser, _ := yaml.New()

			p, _ := parser.Parse(context.Background(), io.NopCloser(bytes.NewReader(pkgBytes)))
			if len(p.GetMeta()) == 1 {
				meta := p.GetMeta()[0]

				pkg := meta.(metav1.Pkg)

				if diff := cmp.Diff(tc.want.metaFile, pkg); diff != "" {
					t.Errorf("\n%s\nRWMetaFile(...): -want err, +got err:\n%s", tc.reason, diff)
				}
			}
		})
	}
}
