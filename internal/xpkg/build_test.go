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

package xpkg

import (
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var _ parser.Backend = &MockBackend{}

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

var _ parser.Linter = &MockLinter{}

type MockLinter struct {
	MockLint func() error
}

func NewMockLintFn(err error) func() error {
	return func() error { return err }
}

func (m *MockLinter) Lint(*parser.Package) error {
	return m.MockLint()
}

func TestBuild(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		be parser.Backend
		p  parser.Parser
		l  parser.Linter
	}
	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"Success": {
			reason: "Should not return an error if package building is successful.",
			args: args{
				be: parser.NewEchoBackend(""),
				p:  p,
				l:  parser.NewPackageLinter(nil, nil, nil),
			},
		},
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
				p: &MockParser{
					MockParse: NewMockParseFn(nil, errBoom),
				},
			},
			want: errors.Wrap(errBoom, errParserPackage),
		},
		"ErrLint": {
			reason: "Should return an error if we fail to lint package.",
			args: args{
				be: parser.NewEchoBackend(""),
				p:  p,
				l: &MockLinter{
					MockLint: NewMockLintFn(errBoom),
				},
			},
			want: errors.Wrap(errBoom, errLintPackage),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Build(context.TODO(), tc.args.be, tc.args.p, tc.args.l)

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBuild(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
