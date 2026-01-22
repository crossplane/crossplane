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

package customresourcesgate

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

func TestToGVKs(t *testing.T) {
	type args struct {
		crd *apiextensionsv1.CustomResourceDefinition
	}

	type want struct {
		gvks map[schema.GroupVersionKind]bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SingleVersionServed": {
			reason: "Should return single GVK for CRD with one served version",
			args: args{
				crd: &apiextensionsv1.CustomResourceDefinition{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Kind: "TestResource",
						},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{Name: "v1", Served: true},
						},
					},
				},
			},
			want: want{
				gvks: map[schema.GroupVersionKind]bool{
					{Group: "example.com", Version: "v1", Kind: "TestResource"}: true,
				},
			},
		},
		"MultipleVersionsWithServedStatus": {
			reason: "Should return GVKs with correct served status for multiple versions",
			args: args{
				crd: &apiextensionsv1.CustomResourceDefinition{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Kind: "TestResource",
						},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{Name: "v1alpha1", Served: false},
							{Name: "v1beta1", Served: true},
							{Name: "v1", Served: true},
						},
					},
				},
			},
			want: want{
				gvks: map[schema.GroupVersionKind]bool{
					{Group: "example.com", Version: "v1alpha1", Kind: "TestResource"}: false,
					{Group: "example.com", Version: "v1beta1", Kind: "TestResource"}:  true,
					{Group: "example.com", Version: "v1", Kind: "TestResource"}:       true,
				},
			},
		},
		"NoVersions": {
			reason: "Should return empty map for CRD with no versions",
			args: args{
				crd: &apiextensionsv1.CustomResourceDefinition{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Kind: "TestResource",
						},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{},
					},
				},
			},
			want: want{
				gvks: map[schema.GroupVersionKind]bool{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := toGVKs(tc.args.crd)

			if diff := cmp.Diff(tc.want.gvks, got); diff != "" {
				t.Errorf("\n%s\ntoGVKs(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestIsEstablished(t *testing.T) {
	type args struct {
		crd *apiextensionsv1.CustomResourceDefinition
	}

	type want struct {
		established bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EstablishedTrue": {
			reason: "Should return true when CRD has Established condition with True status",
			args: args{
				crd: &apiextensionsv1.CustomResourceDefinition{
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1.Established,
								Status: apiextensionsv1.ConditionTrue,
							},
						},
					},
				},
			},
			want: want{
				established: true,
			},
		},
		"EstablishedFalse": {
			reason: "Should return false when CRD has Established condition with False status",
			args: args{
				crd: &apiextensionsv1.CustomResourceDefinition{
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1.Established,
								Status: apiextensionsv1.ConditionFalse,
							},
						},
					},
				},
			},
			want: want{
				established: false,
			},
		},
		"EstablishedUnknown": {
			reason: "Should return false when CRD has Established condition with Unknown status",
			args: args{
				crd: &apiextensionsv1.CustomResourceDefinition{
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1.Established,
								Status: apiextensionsv1.ConditionUnknown,
							},
						},
					},
				},
			},
			want: want{
				established: false,
			},
		},
		"NoEstablishedCondition": {
			reason: "Should return false when CRD has no Established condition",
			args: args{
				crd: &apiextensionsv1.CustomResourceDefinition{
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1.NamesAccepted,
								Status: apiextensionsv1.ConditionTrue,
							},
						},
					},
				},
			},
			want: want{
				established: false,
			},
		},
		"NoConditions": {
			reason: "Should return false when CRD has no conditions",
			args: args{
				crd: &apiextensionsv1.CustomResourceDefinition{
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{},
					},
				},
			},
			want: want{
				established: false,
			},
		},
		"MultipleConditions": {
			reason: "Should return true when CRD has multiple conditions including Established=True",
			args: args{
				crd: &apiextensionsv1.CustomResourceDefinition{
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1.NamesAccepted,
								Status: apiextensionsv1.ConditionTrue,
							},
							{
								Type:   apiextensionsv1.Established,
								Status: apiextensionsv1.ConditionTrue,
							},
							{
								Type:   apiextensionsv1.Terminating,
								Status: apiextensionsv1.ConditionFalse,
							},
						},
					},
				},
			},
			want: want{
				established: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := isEstablished(tc.args.crd)

			if diff := cmp.Diff(tc.want.established, got); diff != "" {
				t.Errorf("\n%s\nisEstablished(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// MockGate implements the controller.Gate interface for testing.
type MockGate struct {
	TrueCalls  []schema.GroupVersionKind
	FalseCalls []schema.GroupVersionKind
}

func NewMockGate() *MockGate {
	return &MockGate{
		TrueCalls:  make([]schema.GroupVersionKind, 0),
		FalseCalls: make([]schema.GroupVersionKind, 0),
	}
}

func (m *MockGate) Set(gvk schema.GroupVersionKind, value bool) bool {
	if value {
		if m.TrueCalls == nil {
			m.TrueCalls = make([]schema.GroupVersionKind, 0)
		}

		m.TrueCalls = append(m.TrueCalls, gvk)
	} else {
		if m.FalseCalls == nil {
			m.FalseCalls = make([]schema.GroupVersionKind, 0)
		}

		m.FalseCalls = append(m.FalseCalls, gvk)
	}

	return true
}

func (m *MockGate) Register(func(), ...schema.GroupVersionKind) {}

func TestReconcile(t *testing.T) {
	now := metav1.Now()

	type fields struct {
		gate *MockGate
	}

	type args struct {
		ctx context.Context
		crd *apiextensionsv1.CustomResourceDefinition
	}

	type want struct {
		result     ctrl.Result
		err        error
		trueCalls  []schema.GroupVersionKind
		falseCalls []schema.GroupVersionKind
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"EstablishedCRDCallsGateTrue": {
			reason: "Should call gate.True for all GVKs when CRD is established",
			fields: fields{
				gate: NewMockGate(),
			},
			args: args{
				ctx: context.Background(),
				crd: &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testresources.example.com",
					},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Kind: "TestResource",
						},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{Name: "v1alpha1", Served: true},
							{Name: "v1", Served: true},
						},
					},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1.Established,
								Status: apiextensionsv1.ConditionTrue,
							},
						},
					},
				},
			},
			want: want{
				result: ctrl.Result{},
				err:    nil,
				trueCalls: []schema.GroupVersionKind{
					{Group: "example.com", Version: "v1alpha1", Kind: "TestResource"},
					{Group: "example.com", Version: "v1", Kind: "TestResource"},
				},
				falseCalls: []schema.GroupVersionKind{},
			},
		},
		"NotEstablishedCRDCallsGateFalse": {
			reason: "Should call gate.False for all GVKs when CRD is not established",
			fields: fields{
				gate: NewMockGate(),
			},
			args: args{
				ctx: context.Background(),
				crd: &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testresources.example.com",
					},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Kind: "TestResource",
						},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{Name: "v1", Served: true},
						},
					},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1.Established,
								Status: apiextensionsv1.ConditionFalse,
							},
						},
					},
				},
			},
			want: want{
				result:    ctrl.Result{},
				err:       nil,
				trueCalls: []schema.GroupVersionKind{},
				falseCalls: []schema.GroupVersionKind{
					{Group: "example.com", Version: "v1", Kind: "TestResource"},
				},
			},
		},
		"DeletingCRDCallsGateFalse": {
			reason: "Should call gate.False for all GVKs when CRD is being deleted",
			fields: fields{
				gate: NewMockGate(),
			},
			args: args{
				ctx: context.Background(),
				crd: &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "testresources.example.com",
						DeletionTimestamp: &now,
					},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Kind: "TestResource",
						},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{Name: "v1", Served: true},
						},
					},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1.Established,
								Status: apiextensionsv1.ConditionTrue,
							},
						},
					},
				},
			},
			want: want{
				result:    ctrl.Result{},
				err:       nil,
				trueCalls: []schema.GroupVersionKind{},
				falseCalls: []schema.GroupVersionKind{
					{Group: "example.com", Version: "v1", Kind: "TestResource"},
				},
			},
		},
		"MixedServedVersions": {
			reason: "Should only call gate.True for served versions",
			fields: fields{
				gate: NewMockGate(),
			},
			args: args{
				ctx: context.Background(),
				crd: &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testresources.example.com",
					},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Kind: "TestResource",
						},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{Name: "v1alpha1", Served: false},
							{Name: "v1", Served: true},
						},
					},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1.Established,
								Status: apiextensionsv1.ConditionTrue,
							},
						},
					},
				},
			},
			want: want{
				result: ctrl.Result{},
				err:    nil,
				trueCalls: []schema.GroupVersionKind{
					{Group: "example.com", Version: "v1", Kind: "TestResource"},
				},
				falseCalls: []schema.GroupVersionKind{},
			},
		},
		"NoVersionsCRD": {
			reason: "Should handle CRD with no versions gracefully",
			fields: fields{
				gate: NewMockGate(),
			},
			args: args{
				ctx: context.Background(),
				crd: &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testresources.example.com",
					},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Kind: "TestResource",
						},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{},
					},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1.Established,
								Status: apiextensionsv1.ConditionTrue,
							},
						},
					},
				},
			},
			want: want{
				result:     ctrl.Result{},
				err:        nil,
				trueCalls:  []schema.GroupVersionKind{},
				falseCalls: []schema.GroupVersionKind{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &Reconciler{
				log:  logging.NewNopLogger(),
				gate: tc.fields.gate,
			}

			got, err := r.Reconcile(tc.args.ctx, tc.args.crd)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want result, +got result:\n%s", tc.reason, diff)
			}

			// Only check gate calls if gate is not nil
			if tc.fields.gate != nil {
				sortGVK := func(a, b schema.GroupVersionKind) int {
					if c := strings.Compare(a.Group, b.Group); c != 0 {
						return c
					}
					if c := strings.Compare(a.Version, b.Version); c != 0 {
						return c
					}
					return strings.Compare(a.Kind, b.Kind)
				}

				slices.SortFunc(tc.want.trueCalls, sortGVK)
				slices.SortFunc(tc.fields.gate.TrueCalls, sortGVK)

				if diff := cmp.Diff(tc.want.trueCalls, tc.fields.gate.TrueCalls); diff != "" {
					t.Errorf("\n%s\ngate.True calls: -want, +got:\n%s", tc.reason, diff)
				}

				slices.SortFunc(tc.want.falseCalls, sortGVK)
				slices.SortFunc(tc.fields.gate.FalseCalls, sortGVK)

				if diff := cmp.Diff(tc.want.falseCalls, tc.fields.gate.FalseCalls); diff != "" {
					t.Errorf("\n%s\ngate.False calls: -want, +got:\n%s", tc.reason, diff)
				}
			}
		})
	}
}
