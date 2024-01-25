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

package resource

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

type xrcOpt func(c *claim.Unstructured)

func withXRCRef(ref *v1.ObjectReference) xrcOpt {
	return func(c *claim.Unstructured) {
		c.SetResourceReference(ref)
	}
}

func withXRCSecretRef(ref *xpv1.LocalSecretReference) xrcOpt {
	return func(c *claim.Unstructured) {
		c.SetWriteConnectionSecretToReference(ref)
	}
}

func buildXRC(namespace string, name string, opts ...xrcOpt) *unstructured.Unstructured {
	c := claim.New()
	c.SetName(name)
	c.SetNamespace(namespace)
	for _, f := range opts {
		f(c)
	}
	return &c.Unstructured
}

type xrOpt func(c *composite.Unstructured)

func withXRRefs(refs ...v1.ObjectReference) xrOpt {
	return func(c *composite.Unstructured) {
		c.SetResourceReferences(refs)
	}
}

func withXRSecretRef(ref *xpv1.SecretReference) xrOpt {
	return func(c *composite.Unstructured) {
		c.SetWriteConnectionSecretToReference(ref)
	}
}

func buildXR(name string, opts ...xrOpt) *unstructured.Unstructured {
	c := composite.New()
	c.SetName(name)
	for _, f := range opts {
		f(c)
	}
	return &c.Unstructured
}

func TestGetResourceChildrenRefs(t *testing.T) {
	type args struct {
		resource   *Resource
		witSecrets bool
	}
	type want struct {
		refs []v1.ObjectReference
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"XRCWithChildrenXR": {
			reason: "Should return the XR child for an XRC.",
			args: args{
				resource: &Resource{
					Unstructured: *buildXRC("ns-1", "xrc", withXRCRef(&v1.ObjectReference{
						APIVersion: "example.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					})),
				},
			},
			want: want{
				refs: []v1.ObjectReference{
					{
						APIVersion: "example.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					},
				},
			},
		},
		"XRWithChildren": {
			reason: "Should return the list of children refs for an XR.",
			args: args{
				resource: &Resource{
					Unstructured: *buildXR("root-xr", withXRRefs(v1.ObjectReference{
						APIVersion: "example.com/v1",
						Kind:       "MR",
						Name:       "mr-1",
					}, v1.ObjectReference{
						APIVersion: "example2.com/v1",
						Kind:       "MR",
						Name:       "mr-2",
					}, v1.ObjectReference{
						APIVersion: "example2.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					}, v1.ObjectReference{
						APIVersion: "example2.com/v1",
						Kind:       "XRC",
						Name:       "xrc-1",
						Namespace:  "ns-1",
					},
					)),
				},
			},
			want: want{
				refs: []v1.ObjectReference{
					{
						APIVersion: "example.com/v1",
						Kind:       "MR",
						Name:       "mr-1",
					},
					{
						APIVersion: "example2.com/v1",
						Kind:       "MR",
						Name:       "mr-2",
					},
					{
						APIVersion: "example2.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					},
					{
						APIVersion: "example2.com/v1",
						Kind:       "XRC",
						Name:       "xrc-1",
						Namespace:  "ns-1",
					},
				},
			},
		},
		"XRCWithChildrenXRandConnectionSecretEnabled": {
			reason: "Should return the XR child, but no writeConnectionSecret ref for an XRC.",
			args: args{
				witSecrets: true,
				resource: &Resource{
					Unstructured: *buildXRC("ns-1", "xrc", withXRCSecretRef(&xpv1.LocalSecretReference{
						Name: "secret-1",
					}), withXRCRef(&v1.ObjectReference{
						APIVersion: "example.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					})),
				},
			},
			want: want{
				refs: []v1.ObjectReference{
					{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  "ns-1",
						Name:       "secret-1",
					},
					{
						APIVersion: "example.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					},
				},
			},
		},
		"XRCWithChildrenXRandConnectionSecretDisabled": {
			reason: "Should return the XR child, but no writeConnectionSecret, ref for an XRC.",
			args: args{
				witSecrets: false,
				resource: &Resource{
					Unstructured: *buildXRC("ns-1", "xrc", withXRCSecretRef(&xpv1.LocalSecretReference{
						Name: "secret-1",
					}), withXRCRef(&v1.ObjectReference{
						APIVersion: "example.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					})),
				},
			},
			want: want{
				refs: []v1.ObjectReference{
					{
						APIVersion: "example.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					},
				},
			},
		},
		"XRWithChildrenAndSecret": {
			reason: "Should return a list of children refs for an XR.",
			args: args{
				witSecrets: true,
				resource: &Resource{
					Unstructured: *buildXR("root-xr", withXRSecretRef(&xpv1.SecretReference{
						Name:      "secret-1",
						Namespace: "ns-1",
					}), withXRRefs(v1.ObjectReference{
						APIVersion: "example.com/v1",
						Kind:       "MR",
						Name:       "mr-1",
					},
						v1.ObjectReference{
							APIVersion: "example2.com/v1",
							Kind:       "MR",
							Name:       "mr-2",
						},
						v1.ObjectReference{
							APIVersion: "example2.com/v1",
							Kind:       "XR",
							Name:       "xr-1",
						},
						v1.ObjectReference{
							APIVersion: "example2.com/v1",
							Kind:       "XRC",
							Name:       "xrc-1",
						},
					)),
				},
			},
			want: want{
				refs: []v1.ObjectReference{
					{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  "ns-1",
						Name:       "secret-1",
					},
					{
						APIVersion: "example.com/v1",
						Kind:       "MR",
						Name:       "mr-1",
					},
					{
						APIVersion: "example2.com/v1",
						Kind:       "MR",
						Name:       "mr-2",
					},
					{
						APIVersion: "example2.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					},
					{
						APIVersion: "example2.com/v1",
						Kind:       "XRC",
						Name:       "xrc-1",
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := getResourceChildrenRefs(tc.args.resource, tc.args.witSecrets)
			if diff := cmp.Diff(tc.want.refs, got, cmpopts.SortSlices(func(r1, r2 v1.ObjectReference) bool {
				return strings.Compare(r1.String(), r2.String()) < 0
			})); diff != "" {
				t.Errorf("\n%s\ngetResourceChildrenRefs(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

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
