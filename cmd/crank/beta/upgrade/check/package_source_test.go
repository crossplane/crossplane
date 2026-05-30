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

package check

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

// pkgData is the declarative input for one package-sources check Run. Zero-value
// fields mean the corresponding List returns nothing; a single non-nil error
// lets a case exercise one List failure in isolation.
type pkgData struct {
	provs []pkgv1.Provider
	cfgs  []pkgv1.Configuration
	fns   []pkgv1.Function

	provErr error
	cfgErr  error
	fnErr   error
}

// pkgClient builds a MockClient serving the three package-type List calls, from
// the given data.
func pkgClient(d pkgData) *test.MockClient {
	return &test.MockClient{
		MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
			switch l := list.(type) {
			case *pkgv1.ProviderList:
				if d.provErr != nil {
					return d.provErr
				}
				l.Items = d.provs
			case *pkgv1.ConfigurationList:
				if d.cfgErr != nil {
					return d.cfgErr
				}
				l.Items = d.cfgs
			case *pkgv1.FunctionList:
				if d.fnErr != nil {
					return d.fnErr
				}
				l.Items = d.fns
			}
			return nil
		},
	}
}

func TestUnqualifiedPackageSourcesRun(t *testing.T) {
	type want struct {
		findings []Finding
		err      error
	}
	cases := map[string]struct {
		reason string
		client client.Client
		want   want
	}{
		"ListProvidersError": {
			reason: "A failure listing Providers surfaces as an error.",
			client: pkgClient(pkgData{provErr: errBoom}),
			want:   want{err: cmpopts.AnyError},
		},
		"ListConfigurationsError": {
			reason: "A failure listing Configurations returns the Provider findings gathered so far plus the error.",
			client: pkgClient(pkgData{
				provs: []pkgv1.Provider{{
					ObjectMeta: metav1.ObjectMeta{Name: "p"},
					Spec: pkgv1.ProviderSpec{
						PackageSpec: pkgv1.PackageSpec{Package: "crossplane-contrib/provider-nop:v0.4.0"},
					},
				}},
				cfgErr: errBoom,
			}),
			want: want{
				findings: []Finding{{Resource: ResourceRef{Group: pkgv1.Group, Kind: pkgv1.ProviderKind, Name: "p"}, FieldPath: ".spec.package"}},
				err:      cmpopts.AnyError,
			},
		},
		"ListFunctionsError": {
			reason: "A failure listing Functions returns the Provider and Configuration findings gathered so far plus the error.",
			client: pkgClient(pkgData{
				provs: []pkgv1.Provider{{
					ObjectMeta: metav1.ObjectMeta{Name: "p"},
					Spec: pkgv1.ProviderSpec{
						PackageSpec: pkgv1.PackageSpec{Package: "crossplane-contrib/provider-nop:v0.4.0"},
					},
				}},
				cfgs: []pkgv1.Configuration{{
					ObjectMeta: metav1.ObjectMeta{Name: "c"},
					Spec: pkgv1.ConfigurationSpec{
						PackageSpec: pkgv1.PackageSpec{Package: "crossplane-contrib/configuration-foo:v0.1.0"},
					},
				}},
				fnErr: errBoom,
			}),
			want: want{
				findings: []Finding{
					{Resource: ResourceRef{Group: pkgv1.Group, Kind: pkgv1.ProviderKind, Name: "p"}, FieldPath: ".spec.package"},
					{Resource: ResourceRef{Group: pkgv1.Group, Kind: pkgv1.ConfigurationKind, Name: "c"}, FieldPath: ".spec.package"},
				},
				err: cmpopts.AnyError,
			},
		},
		"AllQualified": {
			reason: "Fully qualified references, produce no findings.",
			client: pkgClient(pkgData{
				provs: []pkgv1.Provider{{
					ObjectMeta: metav1.ObjectMeta{Name: "p"},
					Spec: pkgv1.ProviderSpec{
						PackageSpec: pkgv1.PackageSpec{Package: "xpkg.crossplane.io/crossplane-contrib/provider-nop:v0.4.0"},
					},
				}},
				cfgs: []pkgv1.Configuration{{
					ObjectMeta: metav1.ObjectMeta{Name: "c"},
					Spec: pkgv1.ConfigurationSpec{
						PackageSpec: pkgv1.PackageSpec{Package: "docker.io/crossplane/configuration-foo:v0.1.0"},
					},
				}},
				fns: []pkgv1.Function{{
					ObjectMeta: metav1.ObjectMeta{Name: "f"},
					Spec: pkgv1.FunctionSpec{
						PackageSpec: pkgv1.PackageSpec{Package: "xpkg.crossplane.io/crossplane-contrib/function-cool:v0.5.0"},
					},
				}},
			}),
			want: want{findings: nil},
		},
		"UnqualifiedAndUnparseable": {
			reason: "An unqualified reference is flagged at .spec.package; one that can't be parsed at all is flagged as unparseable. The qualified Function is left alone.",
			client: pkgClient(pkgData{
				provs: []pkgv1.Provider{{
					ObjectMeta: metav1.ObjectMeta{Name: "p"},
					Spec: pkgv1.ProviderSpec{
						PackageSpec: pkgv1.PackageSpec{Package: "crossplane-contrib/provider-nop:v0.4.0"}, // no registry host
					},
				}},
				cfgs: []pkgv1.Configuration{{
					ObjectMeta: metav1.ObjectMeta{Name: "c"},
					Spec: pkgv1.ConfigurationSpec{
						PackageSpec: pkgv1.PackageSpec{Package: "NOT A VALID REF!!"}, // unparseable
					},
				}},
				fns: []pkgv1.Function{{
					ObjectMeta: metav1.ObjectMeta{Name: "f"},
					Spec: pkgv1.FunctionSpec{
						PackageSpec: pkgv1.PackageSpec{Package: "xpkg.crossplane.io/crossplane-contrib/function-cool:v0.5.0"},
					},
				}},
			}),
			want: want{findings: []Finding{
				{Resource: ResourceRef{Group: pkgv1.Group, Kind: pkgv1.ProviderKind, Name: "p"}, FieldPath: ".spec.package"},
				{Resource: ResourceRef{Group: pkgv1.Group, Kind: pkgv1.ConfigurationKind, Name: "c"}, FieldPath: ".spec.package (unparseable)"},
			}},
		},
		"BareName": {
			reason: "A bare image name with no registry host is flagged at .spec.package.",
			client: pkgClient(pkgData{
				provs: []pkgv1.Provider{{
					ObjectMeta: metav1.ObjectMeta{Name: "p"},
					Spec: pkgv1.ProviderSpec{
						PackageSpec: pkgv1.PackageSpec{Package: "provider-nop:v0.4.0"},
					},
				}},
			}),
			want: want{findings: []Finding{
				{Resource: ResourceRef{Group: pkgv1.Group, Kind: pkgv1.ProviderKind, Name: "p"}, FieldPath: ".spec.package"},
			}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &UnqualifiedPackageSources{Client: tc.client}
			got, err := c.Run(context.Background())
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.findings, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nRun(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
