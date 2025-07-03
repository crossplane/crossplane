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

package runtime

import (
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

const (
	testDeploymentName = "test-deployment"
	testNamespaceName  = "test-namespace"
)

// MockDeploymentSelectorMigrator is a mock implementation of DeploymentSelectorMigrator.
type MockDeploymentSelectorMigrator struct {
	MockMigrateDeploymentSelector func(ctx context.Context, pr v1.PackageRevisionWithRuntime, b ManifestBuilder) error
}

// MigrateDeploymentSelector calls MockMigrateDeploymentSelector if set, otherwise returns nil.
func (m *MockDeploymentSelectorMigrator) MigrateDeploymentSelector(ctx context.Context, pr v1.PackageRevisionWithRuntime, b ManifestBuilder) error {
	if m.MockMigrateDeploymentSelector != nil {
		return m.MockMigrateDeploymentSelector(ctx, pr, b)
	}

	return nil
}

func TestDeletingDeploymentSelectorMigrator_MigrateDeploymentSelector(t *testing.T) {
	errBoom := errors.New("boom")
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))

	type args struct {
		client  client.Client
		pr      v1.PackageRevisionWithRuntime
		builder ManifestBuilder
	}

	type want struct {
		err error
	}

	mockBuilder := &MockManifestBuilder{
		DeploymentFn: func(_ string, _ ...DeploymentOverride) *appsv1.Deployment {
			return &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testDeploymentName,
					Namespace: testNamespaceName,
				},
			}
		},
		ServiceAccountFn: func(_ ...ServiceAccountOverride) *corev1.ServiceAccount {
			return &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: testNamespaceName,
				},
			}
		},
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NotProviderRevision": {
			reason: "Should return nil for non-provider revisions (like function revisions).",
			args: args{
				client:  &test.MockClient{},
				pr:      &v1.FunctionRevision{},
				builder: mockBuilder,
			},
			want: want{
				err: nil,
			},
		},
		"InactiveProviderRevision": {
			reason: "Should return nil for inactive provider revisions.",
			args: args{
				client: &test.MockClient{},
				pr: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionInactive,
						},
					},
				},
				builder: mockBuilder,
			},
			want: want{
				err: nil,
			},
		},
		"NoExistingDeployment": {
			reason: "Should return nil when no existing deployment is found.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				},
				pr: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.LabelParentPackage: "crossplane-provider-nop",
						},
					},
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				builder: mockBuilder,
			},
			want: want{
				err: nil,
			},
		},
		"ErrorGettingDeployment": {
			reason: "Should return error when getting existing deployment fails.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				pr: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.LabelParentPackage: "crossplane-provider-nop",
						},
					},
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				builder: mockBuilder,
			},
			want: want{
				err: errors.Wrap(errBoom, "cannot get existing deployment"),
			},
		},
		"NoSelectorInDeployment": {
			reason: "Should return nil when deployment has no selector.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						deploy := o.(*appsv1.Deployment)
						deploy.Name = testDeploymentName
						deploy.Namespace = testNamespaceName
						deploy.Spec.Selector = nil
						return nil
					}),
				},
				pr: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.LabelParentPackage: "crossplane-provider-nop",
						},
					},
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				builder: mockBuilder,
			},
			want: want{
				err: nil,
			},
		},
		"NoMatchLabelsInSelector": {
			reason: "Should return nil when deployment selector has no match labels.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						deploy := o.(*appsv1.Deployment)
						deploy.Name = testDeploymentName
						deploy.Namespace = testNamespaceName
						deploy.Spec.Selector = &metav1.LabelSelector{
							MatchLabels: nil,
						}
						return nil
					}),
				},
				pr: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.LabelParentPackage: "crossplane-provider-nop",
						},
					},
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				builder: mockBuilder,
			},
			want: want{
				err: nil,
			},
		},
		"NoMigrationNeeded": {
			reason: "Should return nil when provider labels match (no migration needed).",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						deploy := o.(*appsv1.Deployment)
						deploy.Name = testDeploymentName
						deploy.Namespace = testNamespaceName
						deploy.Spec.Selector = &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"pkg.crossplane.io/provider": "crossplane-provider-nop",
							},
						}
						return nil
					}),
				},
				pr: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.LabelParentPackage: "crossplane-provider-nop",
						},
					},
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				builder: mockBuilder,
			},
			want: want{
				err: nil,
			},
		},
		"MigrationRequired": {
			reason: "Should delete deployment when migration is required.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						deploy := o.(*appsv1.Deployment)
						deploy.Name = testDeploymentName
						deploy.Namespace = testNamespaceName
						deploy.Spec.Selector = &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"pkg.crossplane.io/provider": "provider-nop", // Old format
							},
						}
						return nil
					}),
					MockDelete: test.NewMockDeleteFn(nil, func(o client.Object) error {
						want := &appsv1.Deployment{
							ObjectMeta: metav1.ObjectMeta{
								Name:      testDeploymentName,
								Namespace: testNamespaceName,
							},
							Spec: appsv1.DeploymentSpec{
								Selector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"pkg.crossplane.io/provider": "provider-nop",
									},
								},
							},
						}
						if diff := cmp.Diff(want, o); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				pr: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "provider-nop-v1.0.0",
						Labels: map[string]string{
							v1.LabelParentPackage: "crossplane-provider-nop", // New format
						},
					},
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				builder: mockBuilder,
			},
			want: want{
				err: nil,
			},
		},
		"ErrorDeletingDeployment": {
			reason: "Should return error when deleting deployment fails.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						deploy := o.(*appsv1.Deployment)
						deploy.Name = testDeploymentName
						deploy.Namespace = testNamespaceName
						deploy.Spec.Selector = &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"pkg.crossplane.io/provider": "provider-nop", // Old format
							},
						}
						return nil
					}),
					MockDelete: test.NewMockDeleteFn(errBoom),
				},
				pr: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.LabelParentPackage: "crossplane-provider-nop", // New format
						},
					},
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				builder: mockBuilder,
			},
			want: want{
				err: errors.Wrap(errBoom, "cannot delete existing deployment for selector migration"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			migrator := NewDeletingDeploymentSelectorMigrator(tc.args.client, testLog)

			err := migrator.MigrateDeploymentSelector(context.Background(), tc.args.pr, tc.args.builder)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nMigrateDeploymentSelector(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestNopDeploymentSelectorMigrator_MigrateDeploymentSelector(t *testing.T) {
	mockBuilder := &MockManifestBuilder{
		DeploymentFn: func(_ string, _ ...DeploymentOverride) *appsv1.Deployment {
			return &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testDeploymentName,
					Namespace: testNamespaceName,
				},
			}
		},
		ServiceAccountFn: func(_ ...ServiceAccountOverride) *corev1.ServiceAccount {
			return &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: testNamespaceName,
				},
			}
		},
	}

	testCases := []struct {
		name string
		pr   v1.PackageRevisionWithRuntime
	}{
		{
			name: "ProviderRevision",
			pr: &v1.ProviderRevision{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.LabelParentPackage: "crossplane-provider-test",
					},
				},
				Spec: v1.ProviderRevisionSpec{
					PackageRevisionSpec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
		},
		{
			name: "FunctionRevision",
			pr: &v1.FunctionRevision{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.LabelParentPackage: "crossplane-function-test",
					},
				},
				Spec: v1.FunctionRevisionSpec{
					PackageRevisionSpec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
		},
	}

	migrator := NewNopDeploymentSelectorMigrator()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := migrator.MigrateDeploymentSelector(context.Background(), tc.pr, mockBuilder)
			if err != nil {
				t.Errorf("NopDeploymentSelectorMigrator should never return an error, got: %v", err)
			}
		})
	}
}

func TestNewNopDeploymentSelectorMigrator(t *testing.T) {
	migrator := NewNopDeploymentSelectorMigrator()

	if migrator == nil {
		t.Error("Expected non-nil migrator")
	}
}
