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

package xpkg

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/spf13/afero"
	"github.com/spf13/afero/tarfs"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/internal/xpkg/parser/examples"
)

var (
	testCRD  []byte
	testMeta []byte
	testEx1  []byte
	testEx2  []byte
	testEx3  []byte
	testEx4  []byte

	_ parser.Backend = &MockBackend{}
)

func init() {
	testCRD, _ = afero.ReadFile(afero.NewOsFs(), "testdata/providerconfigs.helm.crossplane.io.yaml")
	testMeta, _ = afero.ReadFile(afero.NewOsFs(), "testdata/provider_meta.yaml")
	testEx1, _ = afero.ReadFile(afero.NewOsFs(), "testdata/examples/ec2/instance.yaml")
	testEx2, _ = afero.ReadFile(afero.NewOsFs(), "testdata/examples/ec2/internetgateway.yaml")
	testEx3, _ = afero.ReadFile(afero.NewOsFs(), "testdata/examples/ecr/repository.yaml")
	testEx4, _ = afero.ReadFile(afero.NewOsFs(), "testdata/examples/provider.yaml")
}

type MockBackend struct {
	MockInit func() (io.ReadCloser, error)
}

func NewMockInitFn(r io.ReadCloser, err error) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) { return r, err }
}

func (m *MockBackend) Init(_ context.Context, _ ...parser.BackendOption) (io.ReadCloser, error) {
	return m.MockInit()
}

var _ parser.Parser = &MockParser{}

type MockParser struct {
	MockParse func() (*parser.Package, error)
}

func NewMockParseFn(pkg *parser.Package, err error) func() (*parser.Package, error) {
	return func() (*parser.Package, error) { return pkg, err }
}

func (m *MockParser) Parse(context.Context, io.ReadCloser) (*parser.Package, error) {
	return m.MockParse()
}

func TestBuild(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		be parser.Backend
		ex parser.Backend
		p  parser.Parser
		e  *examples.Parser
	}
	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"ErrInitBackend": {
			reason: "Should return an error if we fail to initialize backend.",
			args: args{
				be: &MockBackend{
					MockInit: NewMockInitFn(nil, errBoom),
				},
			},
			want: errors.Wrap(errBoom, errInitBackend),
		},
		"ErrParse": {
			reason: "Should return an error if we fail to parse package.",
			args: args{
				be: parser.NewEchoBackend(""),
				ex: parser.NewEchoBackend(""),
				p: &MockParser{
					MockParse: NewMockParseFn(nil, errBoom),
				},
			},
			want: errors.Wrap(errBoom, errParserPackage),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			builder := New(tc.args.be, tc.args.ex, tc.args.p, tc.args.e)

			_, _, err := builder.Build(context.TODO())

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBuild(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestBuildExamples(t *testing.T) {
	pkgp, _ := yamlParser()

	defaultFilters := []parser.FilterFn{
		parser.SkipDirs(),
		parser.SkipNotYAML(),
		parser.SkipEmpty(),
	}

	type withFsFn func() afero.Fs

	type args struct {
		rootDir     string
		examplesDir string
		fs          withFsFn
	}
	type want struct {
		pkgExists bool
		exExists  bool
		labels    []string
		err       error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessNoExamples": {
			args: args{
				rootDir:     "/ws",
				examplesDir: "/ws/examples",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("/ws", os.ModePerm)
					_ = fs.Mkdir("/ws/crds", os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crossplane.yaml", testMeta, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/crd.yaml", testCRD, os.ModePerm)
					return fs
				},
			},
			want: want{
				pkgExists: true,
				labels: []string{
					PackageAnnotation,
				},
			},
		},
		"SuccessExamplesAtRoot": {
			args: args{
				rootDir:     "/ws",
				examplesDir: "/ws/examples",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("/ws", os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crossplane.yaml", testMeta, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/crd.yaml", testCRD, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/examples/ec2/instance.yaml", testEx1, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/examples/ec2/internetgateway.yaml", testEx2, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/examples/ecr/repository.yaml", testEx3, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/examples/provider.yaml", testEx4, os.ModePerm)
					return fs
				},
			},
			want: want{
				pkgExists: true,
				exExists:  true,
				labels: []string{
					PackageAnnotation,
					ExamplesAnnotation,
				},
			},
		},
		"SuccessExamplesAtCustomDir": {
			args: args{
				rootDir:     "/ws",
				examplesDir: "/other_directory/examples",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("/ws", os.ModePerm)
					_ = fs.Mkdir("/other_directory", os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crossplane.yaml", testMeta, os.ModePerm)
					_ = afero.WriteFile(fs, "/ws/crds/crd.yaml", testCRD, os.ModePerm)
					_ = afero.WriteFile(fs, "/other_directory/examples/ec2/instance.yaml", testEx1, os.ModePerm)
					_ = afero.WriteFile(fs, "/other_directory/examples/ec2/internetgateway.yaml", testEx2, os.ModePerm)
					_ = afero.WriteFile(fs, "/other_directory/examples/ecr/repository.yaml", testEx3, os.ModePerm)
					_ = afero.WriteFile(fs, "/other_directory/examples/provider.yaml", testEx4, os.ModePerm)
					return fs
				},
			},
			want: want{
				pkgExists: true,
				exExists:  true,
				labels: []string{
					PackageAnnotation,
					ExamplesAnnotation,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pkgBe := parser.NewFsBackend(
				tc.args.fs(),
				parser.FsDir(tc.args.rootDir),
				parser.FsFilters([]parser.FilterFn{
					parser.SkipDirs(),
					parser.SkipNotYAML(),
					parser.SkipEmpty(),
					SkipContains("examples/"), // don't try to parse the examples in the package
				}...),
			)
			pkgEx := parser.NewFsBackend(
				tc.args.fs(),
				parser.FsDir(tc.args.examplesDir),
				parser.FsFilters(defaultFilters...),
			)

			builder := New(pkgBe, pkgEx, pkgp, examples.New())

			img, _, err := builder.Build(context.TODO())

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBuildExamples(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			// validate the xpkg img has the correct annotations, etc
			contents, err := readImg(img)
			// sort the contents slice for test comparison
			sort.Strings(contents.labels)

			if diff := cmp.Diff(tc.want.pkgExists, len(contents.pkgBytes) != 0); diff != "" {
				t.Errorf("\n%s\nBuildExamples(...): -want err, +got err:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.exExists, len(contents.exBytes) != 0); diff != "" {
				t.Errorf("\n%s\nBuildExamples(...): -want err, +got err:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.labels, contents.labels, cmpopts.SortSlices(func(i, j int) bool {
				return contents.labels[i] < contents.labels[j]
			})); diff != "" {
				t.Errorf("\n%s\nBuildExamples(...): -want err, +got err:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBuildExamples(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

type xpkgContents struct {
	labels   []string
	pkgBytes []byte
	exBytes  []byte
}

func readImg(i v1.Image) (xpkgContents, error) {
	contents := xpkgContents{
		labels: make([]string, 0),
	}

	reader := mutate.Extract(i)
	fs := tarfs.New(tar.NewReader(reader))
	pkgYaml, err := fs.Open(StreamFile)
	if err != nil {
		return contents, err
	}

	pkgBytes, err := io.ReadAll(pkgYaml)
	if err != nil {
		return contents, err
	}
	contents.pkgBytes = pkgBytes

	exYaml, err := fs.Open(XpkgExamplesFile)
	if err != nil && !os.IsNotExist(err) {
		return contents, err
	}

	if exYaml != nil {
		exBytes, err := io.ReadAll(exYaml)
		if err != nil {
			return contents, err
		}
		contents.exBytes = exBytes
	}

	labels, err := allLabels(i)
	if err != nil {
		return contents, err
	}

	contents.labels = labels

	return contents, nil
}

func allLabels(i partial.WithConfigFile) ([]string, error) {
	labels := []string{}

	cfgFile, err := i.ConfigFile()
	if err != nil {
		return labels, err
	}

	cfg := cfgFile.Config

	for _, label := range cfg.Labels {
		labels = append(labels, label)
	}

	return labels, nil
}

// This is equivalent to yaml.New. Duplicated here to avoid an import cycle.
func yamlParser() (*parser.PackageParser, error) {
	metaScheme, err := BuildMetaScheme()
	if err != nil {
		panic(err)
	}
	objScheme, err := BuildObjectScheme()
	if err != nil {
		panic(err)
	}

	return parser.New(metaScheme, objScheme), nil
}
