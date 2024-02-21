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

package revision

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/dag"
	dagfake "github.com/crossplane/crossplane/internal/dag/fake"
	"github.com/crossplane/crossplane/internal/xpkg"
)

var (
	metaScheme *runtime.Scheme
	objScheme  *runtime.Scheme
	linter     = xpkg.NewProviderLinter()
)

func init() {
	var err error
	metaScheme, err = xpkg.BuildMetaScheme()
	if err != nil {
		panic(err)
	}
	objScheme, err = xpkg.BuildObjectScheme()
	if err != nil {
		panic(err)
	}
}

func newFuzzDag(ff *fuzz.ConsumeFuzzer) (func() dag.DAG, error) {
	traceNodeMap := make(map[string]dag.Node)
	err := ff.FuzzMap(&traceNodeMap)
	if err != nil {
		return func() dag.DAG { return nil }, err
	}
	lp := &v1beta1.LockPackage{}
	err = ff.GenerateStruct(lp)
	if err != nil {
		return func() dag.DAG { return nil }, err
	}
	return func() dag.DAG {
		return &dagfake.MockDag{
			MockInit: func(_ []dag.Node) ([]dag.Node, error) {
				return nil, nil
			},
			MockNodeExists: func(_ string) bool {
				return true
			},
			MockTraceNode: func(_ string) (map[string]dag.Node, error) {
				return traceNodeMap, nil
			},
			MockGetNode: func(_ string) (dag.Node, error) {
				return lp, nil
			},
		}
	}, nil
}

func getFuzzMockClient(ff *fuzz.ConsumeFuzzer) (*test.MockClient, error) {
	lockPackages := make([]v1beta1.LockPackage, 0)
	ff.CreateSlice(&lockPackages)
	if len(lockPackages) == 0 {
		return nil, errors.New("No packages created")
	}
	return &test.MockClient{
		MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
			l := obj.(*v1beta1.Lock)
			l.Packages = lockPackages
			return nil
		}),
		MockUpdate: test.NewMockUpdateFn(nil),
	}, nil
}

func FuzzRevisionControllerPackageHandling(f *testing.F) {
	f.Fuzz(func(_ *testing.T, data, revisionData []byte) {
		ff := fuzz.NewConsumer(revisionData)
		p := parser.New(metaScheme, objScheme)
		r := io.NopCloser(bytes.NewReader(data))
		pkg, err := p.Parse(context.Background(), r)
		if err != nil {
			return
		}
		if len(pkg.GetMeta()) == 0 {
			return
		}
		if len(pkg.GetObjects()) == 0 {
			return
		}
		prs := &v1.PackageRevisionSpec{}
		ff.GenerateStruct(prs)
		pr := &v1.ConfigurationRevision{Spec: *prs}

		if err := linter.Lint(pkg); err != nil {
			return
		}
		pkgMeta, _ := xpkg.TryConvert(pkg.GetMeta()[0], &pkgmetav1.Provider{}, &pkgmetav1.Configuration{})
		c, err := getFuzzMockClient(ff)
		if err != nil {
			return
		}

		fd, err := newFuzzDag(ff)
		if err != nil {
			return
		}
		pm := &PackageDependencyManager{
			client: c,
			newDag: fd,
		}
		_, _, _, _ = pm.Resolve(context.Background(), pkgMeta, pr)
	})
}
