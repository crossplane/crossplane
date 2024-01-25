package xpkg

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

// TODO add more cases, fake client
// Consider testing getDependencies instead to cover more
func TestGetDependencyRef(t *testing.T) {
	type args struct {
		pkgType v1beta1.PackageType
		pkg     string
		lock    *v1beta1.Lock
	}
	type want struct {
		ref *v1.ObjectReference
		err error
	}
	cases := map[string]struct {
		reason string

		args args
		want want
	}{
		"Provider, not found in lock package": {
			reason: "Should return the provider ref for a provider dependency, even when the dep is not found.",
			args: args{
				pkgType: v1beta1.ProviderPackageType,
				pkg:     "example.com/provider-1:v1.0.0",
				lock: buildLock("lock-1", withLockPackages([]v1beta1.LockPackage{
					*buildLockPkg("configuration-1",
						withDependencies(newDependency("provider-2"), newDependency("provider-1")),
						withSource("example.com/configuration-1:v1.0.0")),
					*buildLockPkg("function-1",
						withDependencies(newDependency("provider-3"), newDependency("provider-4")),
						withSource("example.com/function-1:v1.0.0")),
				}...)),
			},
			want: want{
				ref: &v1.ObjectReference{
					APIVersion: "pkg.crossplane.io/v1",
					Kind:       "Provider",
					Name:       "provider-1",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			kc := &Client{}
			got, err := kc.getDependencyRef(context.Background(), tc.args.lock, tc.args.pkgType, tc.args.pkg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("getDependencyRef(...) error = %v, wantErr %v", err, tc.want.err)
			}
			if diff := cmp.Diff(tc.want.ref, got); diff != "" {
				t.Errorf("\n%s\ngetDependencyRef(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

type lockOpt func(c *v1beta1.Lock)

func buildLock(name string, opts ...lockOpt) *v1beta1.Lock {
	l := &v1beta1.Lock{}
	l.SetName(name)
	for _, f := range opts {
		f(l)
	}
	return l
}

func withLockPackages(pkgs ...v1beta1.LockPackage) lockOpt {
	return func(l *v1beta1.Lock) {
		l.Packages = pkgs
	}
}

type lockPkgOpt func(c *v1beta1.LockPackage)

func buildLockPkg(name string, opts ...lockPkgOpt) *v1beta1.LockPackage {
	p := &v1beta1.LockPackage{}
	p.Name = name
	for _, f := range opts {
		f(p)
	}
	return p
}

func withDependencies(deps ...v1beta1.Dependency) lockPkgOpt {
	return func(p *v1beta1.LockPackage) {
		p.Dependencies = deps
	}
}

func withSource(source string) lockPkgOpt {
	return func(p *v1beta1.LockPackage) {
		p.Source = source
	}
}

func newDependency(pkg string) v1beta1.Dependency {
	return v1beta1.Dependency{
		Package: pkg,
	}
}
