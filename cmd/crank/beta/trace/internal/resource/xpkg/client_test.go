package xpkg

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	xpv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	xpkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
)

// TODO add more cases, fake client
// Consider testing getPackageDeps instead to cover more
func TestGetDependencyRef(t *testing.T) {
	type args struct {
		pkgType v1beta1.PackageType
		pkg     string
		client  client.Client
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
		"PkgNotInLock": {
			reason: "Should return the provider ref for a provider dependency, even when the dep is not found.",
			args: args{
				pkgType: v1beta1.ProviderPackageType,
				pkg:     "example.com/provider-1:v1.0.0",
				client:  &test.MockClient{},
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
		"PKGInLock": {
			reason: "Should return the provider ref for a provider dependency.",
			args: args{
				pkgType: v1beta1.ProviderPackageType,
				pkg:     "example.com/provider-1:v1.0.0",
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						pr, ok := obj.(*xpkgv1.ProviderRevision)
						if ok {
							pr.SetName("provider-1")
							pr.SetOwnerReferences([]xpv1.OwnerReference{
								{
									APIVersion: "pkg.crossplane.io/v1",
									Kind:       "Provider",
									Name:       "my-awesome-provider",
									Controller: ptr.To(true),
								},
							})
							return nil
						}

						return errors.New("boom")
					}),
				},
				lock: buildLock("lock-1", withLockPackages([]v1beta1.LockPackage{
					*buildLockPkg("provider-3",
						withDependencies(newDependency("provider-2"), newDependency("provider-1")),
						withSource("example.com/provider-1:v1.0.0")),
					*buildLockPkg("function-1",
						withDependencies(newDependency("provider-3"), newDependency("provider-4")),
						withSource("example.com/function-1:v1.0.0")),
				}...)),
			},
			want: want{
				ref: &v1.ObjectReference{
					APIVersion: "pkg.crossplane.io/v1",
					Kind:       "Provider",
					Name:       "my-awesome-provider",
				},
			},
		},
		"PKGTypeWrong": {
			reason: "Should return an error for a provider dependency when the package type is wrong.",
			args: args{
				pkgType: v1beta1.PackageType("wrong"),
				pkg:     "example.com/provider-1:v1.0.0",
				client:  test.NewMockClient(),
				lock:    buildLock("lock-1"),
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorGettingPKGRevision": {
			reason: "Should return an error for a provider dependency when the package revision cannot be retrieved.",
			args: args{
				pkgType: v1beta1.ConfigurationPackageType,
				pkg:     "example.com/configuration-1:v1.0.0",
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(errors.New("boom")),
				},
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
				err: cmpopts.AnyError,
			},
		},
		"PKGRevisionNotFound": {
			reason: "Should return no error for a provider dependency when the package revision is not found.",
			args: args{
				pkgType: v1beta1.FunctionPackageType,
				pkg:     "example.com/function-1:v1.0.0",
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, "whatever")
					}),
				},
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
				err: nil,
				ref: &v1.ObjectReference{
					APIVersion: "pkg.crossplane.io/v1beta1",
					Kind:       "Function",
					Name:       "function-1",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			kc := &Client{
				client: tc.args.client,
			}
			got, err := kc.getDependencyRef(context.Background(), tc.args.lock, tc.args.pkgType, tc.args.pkg)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("getDependencyRef(...) error = %v, wantErr %v", err, tc.want.err)
			}
			if diff := cmp.Diff(tc.want.ref, got); diff != "" {
				t.Errorf("\n%s\ngetDependencyRef(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetPackageDeps(t *testing.T) {
	type args struct {
		client           client.Client
		dependencyOutput DependencyOutput

		node       *resource.Resource
		lock       *v1beta1.Lock
		uniqueDeps map[string]struct{}
	}
	type want struct {
		deps []v1.ObjectReference
		err  error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoCurrentRevision": {
			reason: "Should return no error when the current revision cannot be retrieved.",
			args: args{
				client: &test.MockClient{},
				node: &resource.Resource{
					Unstructured: unstructured.Unstructured{
						Object: nil,
					},
				},
			},
			want: want{
				err:  nil,
				deps: nil,
			},
		},
		"NotInLockYet": {
			reason: "Should return no error when the current revision is not in the lock yet.",
			args: args{
				client: &test.MockClient{},
				node: &resource.Resource{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"status": map[string]interface{}{
								"currentRevision": "provider-revision-1",
							},
						},
					},
				},
				lock: buildLock("lock-1"),
			},
			want: want{
				err:  nil,
				deps: nil,
			},
		},
		"WantUniqueAndPresent": {
			reason: "Should return no error when the unique dependencies are already present.",
			args: args{
				client:           &test.MockClient{},
				dependencyOutput: DependencyOutputUnique,
				node: &resource.Resource{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"status": map[string]interface{}{
								"currentRevision": "provider-revision-1",
							},
						},
					},
				},
				lock: buildLock("lock-1", withLockPackages([]v1beta1.LockPackage{
					*buildLockPkg("provider-revision-1",
						withDependencies(newDependency("provider-2"), newDependency("provider-1")),
						withSource("example.com/provider-1:v1.0.0")),
					*buildLockPkg("provider-2",
						withDependencies(newDependency("provider-3"), newDependency("provider-4")),
						withSource("example.com/provider-2:v1.0.0")),
				}...)),
				uniqueDeps: map[string]struct{}{
					"provider-1": {},
					"provider-2": {},
				},
			},
			want: want{
				err:  nil,
				deps: nil,
			},
		},
		"WantUniqueNotPresent": {
			reason: "Should return the right dependencies when the unique dependencies are not already present.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						pr, ok := obj.(*xpkgv1.ProviderRevision)
						if ok {
							pr.SetOwnerReferences([]xpv1.OwnerReference{
								{
									APIVersion: xpkgv1.ProviderGroupVersionKind.GroupVersion().String(),
									Kind:       xpkgv1.ProviderKind,
									Name:       "my-awesome-provider",
									Controller: ptr.To(true),
								},
							})
						}
						return nil
					}),
				},
				dependencyOutput: DependencyOutputUnique,
				node: &resource.Resource{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"status": map[string]interface{}{
								"currentRevision": "provider-revision-0",
							},
						},
					},
				},
				lock: buildLock("lock-1", withLockPackages([]v1beta1.LockPackage{
					*buildLockPkg("provider-revision-0",
						withDependencies(newDependency("example.com/provider-1:v1.0.0", withPackageType(v1beta1.ProviderPackageType))),
						withSource("example.com/provider-0:v1.0.0")),
					*buildLockPkg("provider-revision-1",
						withDependencies(newDependency("example.com/provider-2:v1.0.0", withPackageType(v1beta1.ProviderPackageType)), newDependency("example.com/provider-3:v1.0.0", withPackageType(v1beta1.ProviderPackageType))),
						withSource("example.com/provider-1:v1.0.0")),
					*buildLockPkg("provider-2",
						withDependencies(newDependency("example.com/provider-3:v1.0.0"), newDependency("example.com/provider-4:v1.0.0")),
						withSource("example.com/provider-2:v1.0.0")),
					*buildLockPkg("provider-3",
						withDependencies(newDependency("example.com/provider-4:v1.0.0"), newDependency("example.com/provider-5:v1.0.0")),
						withSource("example.com/provider-3:v1.0.0")),
				}...)),
				uniqueDeps: map[string]struct{}{},
			},
			want: want{
				err: nil,
				deps: []v1.ObjectReference{
					{
						APIVersion: xpkgv1.ProviderGroupVersionKind.GroupVersion().String(),
						Kind:       xpkgv1.ProviderKind,
						Name:       "my-awesome-provider",
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			kc := &Client{
				client:           tc.args.client,
				dependencyOutput: tc.args.dependencyOutput,
			}
			got, err := kc.getPackageDeps(context.Background(), tc.args.node, tc.args.lock, tc.args.uniqueDeps)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("getPackageDeps(...) error = %v, wantErr %v", err, tc.want.err)
			}
			if diff := cmp.Diff(tc.want.deps, got, cmpopts.SortSlices(func(r1, r2 v1.ObjectReference) bool {
				return strings.Compare(r1.String(), r2.String()) < 0
			}), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\ngetPackageDeps(...): -want, +got:\n%s", tc.reason, diff)
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

type dependencyOpts func(d *v1beta1.Dependency)

func withPackageType(pkgType v1beta1.PackageType) dependencyOpts {
	return func(d *v1beta1.Dependency) {
		d.Type = pkgType
	}
}

func newDependency(pkg string, opts ...dependencyOpts) v1beta1.Dependency {
	d := v1beta1.Dependency{
		Package: pkg,
	}
	for _, f := range opts {
		f(&d)
	}
	return d
}
