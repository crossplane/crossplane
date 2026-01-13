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

package manager

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
)

func TestTransactionName(t *testing.T) {
	type args struct {
		pkg v1.Package
	}
	type want struct {
		name string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"DeterministicName": {
			reason: "Should generate a deterministic name based on package GVK, name, and generation",
			args: args{
				pkg: &v1.Provider{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "pkg.crossplane.io/v1",
						Kind:       "Provider",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:       "provider-aws",
						Generation: 1,
					},
				},
			},
			want: want{
				name: "tx-provider-aws-9eca9e58",
			},
		},
		"DifferentGeneration": {
			reason: "Should generate different name for different generation",
			args: args{
				pkg: &v1.Provider{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "pkg.crossplane.io/v1",
						Kind:       "Provider",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:       "provider-aws",
						Generation: 2,
					},
				},
			},
			want: want{
				name: "tx-provider-aws-1ce06184",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := TransactionName(tc.args.pkg)
			if diff := cmp.Diff(tc.want.name, got); diff != "" {
				t.Errorf("%s\nTransactionName(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestNewTransaction(t *testing.T) {
	type args struct {
		pkg        v1.Package
		changeType v1alpha1.ChangeType
	}
	type want struct {
		tx *v1alpha1.Transaction
	}

	uid := types.UID("test-uid")

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"InstallTransaction": {
			reason: "Should create a Transaction with Install spec",
			args: args{
				pkg: &v1.Provider{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "pkg.crossplane.io/v1",
						Kind:       "Provider",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:       "provider-aws",
						UID:        uid,
						Generation: 1,
						Labels: map[string]string{
							"foo": "bar",
						},
					},
					Spec: v1.ProviderSpec{
						PackageSpec: v1.PackageSpec{
							Package: "xpkg.upbound.io/crossplane-contrib/provider-aws:v1.0.0",
						},
					},
				},
				changeType: v1alpha1.ChangeTypeInstall,
			},
			want: want{
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-provider-aws-9eca9e58",
						Labels: map[string]string{
							"pkg.crossplane.io/package":      "provider-aws",
							"pkg.crossplane.io/package-type": "Provider",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "pkg.crossplane.io/v1",
								Kind:               "Provider",
								Name:               "provider-aws",
								UID:                uid,
								Controller:         ptr.To(false),
								BlockOwnerDeletion: ptr.To(true),
							},
						},
					},
					Spec: v1alpha1.TransactionSpec{
						Change: v1alpha1.ChangeTypeInstall,
						Install: &v1alpha1.InstallSpec{
							Package: v1alpha1.PackageSnapshot{
								APIVersion: "pkg.crossplane.io/v1",
								Kind:       "Provider",
								Metadata: v1alpha1.PackageMetadata{
									Name: "provider-aws",
									UID:  uid,
									Labels: map[string]string{
										"foo": "bar",
									},
								},
								Spec: v1alpha1.PackageSnapshotSpec{
									PackageSpec: v1.PackageSpec{
										Package: "xpkg.upbound.io/crossplane-contrib/provider-aws:v1.0.0",
									},
								},
							},
						},
					},
				},
			},
		},
		"DeleteTransaction": {
			reason: "Should create a Transaction with Delete spec",
			args: args{
				pkg: &v1.Provider{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "pkg.crossplane.io/v1",
						Kind:       "Provider",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:       "provider-aws",
						UID:        uid,
						Generation: 1,
					},
					Spec: v1.ProviderSpec{
						PackageSpec: v1.PackageSpec{
							Package: "xpkg.upbound.io/crossplane-contrib/provider-aws:v1.0.0",
						},
					},
				},
				changeType: v1alpha1.ChangeTypeDelete,
			},
			want: want{
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-provider-aws-9eca9e58",
						Labels: map[string]string{
							"pkg.crossplane.io/package":      "provider-aws",
							"pkg.crossplane.io/package-type": "Provider",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "pkg.crossplane.io/v1",
								Kind:               "Provider",
								Name:               "provider-aws",
								UID:                uid,
								Controller:         ptr.To(false),
								BlockOwnerDeletion: ptr.To(true),
							},
						},
					},
					Spec: v1alpha1.TransactionSpec{
						Change: v1alpha1.ChangeTypeDelete,
						Delete: &v1alpha1.DeleteSpec{
							Source: "xpkg.upbound.io/crossplane-contrib/provider-aws:v1.0.0",
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := NewTransaction(tc.args.pkg, tc.args.changeType)
			if diff := cmp.Diff(tc.want.tx, got, cmpopts.IgnoreTypes(schema.GroupVersionKind{})); diff != "" {
				t.Errorf("%s\nNewTransaction(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPackageSnapshot(t *testing.T) {
	type args struct {
		pkg v1.Package
	}
	type want struct {
		snapshot v1alpha1.PackageSnapshot
	}

	uid := types.UID("test-uid")

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ProviderSnapshot": {
			reason: "Should create a complete snapshot of a Provider",
			args: args{
				pkg: &v1.Provider{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "pkg.crossplane.io/v1",
						Kind:       "Provider",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "provider-aws",
						UID:  uid,
						Labels: map[string]string{
							"foo": "bar",
						},
					},
					Spec: v1.ProviderSpec{
						PackageSpec: v1.PackageSpec{
							Package:                     "xpkg.upbound.io/crossplane-contrib/provider-aws:v1.0.0",
							RevisionActivationPolicy:    ptr.To(v1.AutomaticActivation),
							RevisionHistoryLimit:        ptr.To(int64(5)),
							IgnoreCrossplaneConstraints: ptr.To(true),
							SkipDependencyResolution:    ptr.To(false),
							CommonLabels: map[string]string{
								"common": "label",
							},
						},
						PackageRuntimeSpec: v1.PackageRuntimeSpec{
							RuntimeConfigReference: ptr.To(v1.RuntimeConfigReference{
								Name: "default",
							}),
						},
					},
				},
			},
			want: want{
				snapshot: v1alpha1.PackageSnapshot{
					APIVersion: "pkg.crossplane.io/v1",
					Kind:       "Provider",
					Metadata: v1alpha1.PackageMetadata{
						Name: "provider-aws",
						UID:  uid,
						Labels: map[string]string{
							"foo": "bar",
						},
					},
					Spec: v1alpha1.PackageSnapshotSpec{
						PackageSpec: v1.PackageSpec{
							Package:                     "xpkg.upbound.io/crossplane-contrib/provider-aws:v1.0.0",
							RevisionActivationPolicy:    ptr.To(v1.AutomaticActivation),
							RevisionHistoryLimit:        ptr.To(int64(5)),
							IgnoreCrossplaneConstraints: ptr.To(true),
							SkipDependencyResolution:    ptr.To(false),
							CommonLabels: map[string]string{
								"common": "label",
							},
						},
						PackageRuntimeSpec: v1.PackageRuntimeSpec{
							RuntimeConfigReference: ptr.To(v1.RuntimeConfigReference{
								Name: "default",
							}),
						},
					},
				},
			},
		},
		"ConfigurationSnapshot": {
			reason: "Should create a snapshot of a Configuration without runtime spec",
			args: args{
				pkg: &v1.Configuration{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "pkg.crossplane.io/v1",
						Kind:       "Configuration",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-config",
						UID:  uid,
					},
					Spec: v1.ConfigurationSpec{
						PackageSpec: v1.PackageSpec{
							Package: "xpkg.upbound.io/my-org/my-config:v1.0.0",
						},
					},
				},
			},
			want: want{
				snapshot: v1alpha1.PackageSnapshot{
					APIVersion: "pkg.crossplane.io/v1",
					Kind:       "Configuration",
					Metadata: v1alpha1.PackageMetadata{
						Name: "my-config",
						UID:  uid,
					},
					Spec: v1alpha1.PackageSnapshotSpec{
						PackageSpec: v1.PackageSpec{
							Package: "xpkg.upbound.io/my-org/my-config:v1.0.0",
						},
					},
				},
			},
		},
		"FunctionSnapshot": {
			reason: "Should create a snapshot of a Function with runtime spec",
			args: args{
				pkg: &v1.Function{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "pkg.crossplane.io/v1",
						Kind:       "Function",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "function-auto-ready",
						UID:  uid,
					},
					Spec: v1.FunctionSpec{
						PackageSpec: v1.PackageSpec{
							Package: "xpkg.upbound.io/crossplane-contrib/function-auto-ready:v1.0.0",
						},
						PackageRuntimeSpec: v1.PackageRuntimeSpec{
							RuntimeConfigReference: ptr.To(v1.RuntimeConfigReference{
								Name: "function-runtime",
							}),
						},
					},
				},
			},
			want: want{
				snapshot: v1alpha1.PackageSnapshot{
					APIVersion: "pkg.crossplane.io/v1",
					Kind:       "Function",
					Metadata: v1alpha1.PackageMetadata{
						Name: "function-auto-ready",
						UID:  uid,
					},
					Spec: v1alpha1.PackageSnapshotSpec{
						PackageSpec: v1.PackageSpec{
							Package: "xpkg.upbound.io/crossplane-contrib/function-auto-ready:v1.0.0",
						},
						PackageRuntimeSpec: v1.PackageRuntimeSpec{
							RuntimeConfigReference: ptr.To(v1.RuntimeConfigReference{
								Name: "function-runtime",
							}),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := PackageSnapshot(tc.args.pkg)
			if diff := cmp.Diff(tc.want.snapshot, got, cmpopts.IgnoreTypes(schema.GroupVersionKind{})); diff != "" {
				t.Errorf("%s\nPackageSnapshot(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
