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
	pkgv1alpha1 "github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

// ccData is the declarative input for one ControllerConfig check Run. Zero-value
// fields mean the corresponding List returns nothing; a single non-nil error
// lets a case exercise one List failure in isolation.
type ccData struct {
	ccs   []pkgv1alpha1.ControllerConfig
	provs []pkgv1.Provider
	fns   []pkgv1.Function

	ccErr   error
	provErr error
	fnErr   error
}

// ccClient builds a MockClient serving the three List calls the ControllerConfig
// check makes, from the given data.
func ccClient(d ccData) *test.MockClient {
	return &test.MockClient{
		MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
			switch l := list.(type) {
			case *pkgv1alpha1.ControllerConfigList:
				if d.ccErr != nil {
					return d.ccErr
				}
				l.Items = d.ccs
			case *pkgv1.ProviderList:
				if d.provErr != nil {
					return d.provErr
				}
				l.Items = d.provs
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

func TestControllerConfigRun(t *testing.T) {
	group := pkgv1.Group

	type want struct {
		findings []Finding
		err      error
	}
	cases := map[string]struct {
		reason string
		client client.Client
		want   want
	}{
		"ListControllerConfigsError": {
			reason: "A failure listing ControllerConfigs surfaces as an error with no findings.",
			client: ccClient(ccData{ccErr: errBoom}),
			want:   want{err: cmpopts.AnyError},
		},
		"ListProvidersError": {
			reason: "A failure listing Providers returns the ControllerConfig findings gathered so far plus the error.",
			client: ccClient(ccData{
				ccs:     []pkgv1alpha1.ControllerConfig{{ObjectMeta: metav1.ObjectMeta{Name: "cc1"}}},
				provErr: errBoom,
			}),
			want: want{
				findings: []Finding{{Resource: ResourceRef{Group: group, Kind: pkgv1alpha1.ControllerConfigKind, Name: "cc1"}}},
				err:      cmpopts.AnyError,
			},
		},
		"ListFunctionsError": {
			reason: "A failure listing Functions returns earlier findings plus the error.",
			client: ccClient(ccData{
				provs: []pkgv1.Provider{{
					ObjectMeta: metav1.ObjectMeta{Name: "p"},
					Spec: pkgv1.ProviderSpec{
						PackageRuntimeSpec: pkgv1.PackageRuntimeSpec{
							ControllerConfigReference: &pkgv1.ControllerConfigReference{Name: "cc"},
						},
					},
				}},
				fnErr: errBoom,
			}),
			want: want{
				findings: []Finding{{Resource: ResourceRef{Group: group, Kind: pkgv1.ProviderKind, Name: "p"}, FieldPath: ".spec.controllerConfigRef"}},
				err:      cmpopts.AnyError,
			},
		},
		"NoUsage": {
			reason: "No ControllerConfigs and no references means no findings.",
			client: ccClient(ccData{
				provs: []pkgv1.Provider{{ObjectMeta: metav1.ObjectMeta{Name: "p"}}},
				fns:   []pkgv1.Function{{ObjectMeta: metav1.ObjectMeta{Name: "f"}}},
			}),
			want: want{findings: nil},
		},
		"AllSources": {
			reason: "Each ControllerConfig CR, and each Provider/Function referencing one, produces a finding.",
			client: ccClient(ccData{
				ccs: []pkgv1alpha1.ControllerConfig{{ObjectMeta: metav1.ObjectMeta{Name: "cc1"}}},
				provs: []pkgv1.Provider{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "p-ref"},
						Spec: pkgv1.ProviderSpec{
							PackageRuntimeSpec: pkgv1.PackageRuntimeSpec{
								ControllerConfigReference: &pkgv1.ControllerConfigReference{Name: "cc"},
							},
						},
					},
					{ObjectMeta: metav1.ObjectMeta{Name: "p-noref"}},
				},
				fns: []pkgv1.Function{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "f-ref"},
						Spec: pkgv1.FunctionSpec{
							PackageRuntimeSpec: pkgv1.PackageRuntimeSpec{
								ControllerConfigReference: &pkgv1.ControllerConfigReference{Name: "cc"},
							},
						},
					},
				},
			}),
			want: want{findings: []Finding{
				{Resource: ResourceRef{Group: group, Kind: pkgv1alpha1.ControllerConfigKind, Name: "cc1"}},
				{Resource: ResourceRef{Group: group, Kind: pkgv1.ProviderKind, Name: "p-ref"}, FieldPath: ".spec.controllerConfigRef"},
				{Resource: ResourceRef{Group: group, Kind: pkgv1.FunctionKind, Name: "f-ref"}, FieldPath: ".spec.controllerConfigRef"},
			}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &ControllerConfigCheck{Client: tc.client}
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
