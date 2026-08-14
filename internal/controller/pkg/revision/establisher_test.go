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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
)

var _ Establisher = &APIEstablisher{}

func TestAPIEstablisherPrepareApply(t *testing.T) {
	parent := &v1.ProviderRevision{
		TypeMeta: metav1.TypeMeta{APIVersion: v1.SchemeGroupVersion.String(), Kind: v1.ProviderRevisionKind},
		ObjectMeta: metav1.ObjectMeta{
			Name: "revision-name",
			UID:  "revision-uid",
			Labels: map[string]string{
				v1.LabelParentPackage: "provider-name",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       v1.ProviderKind,
				Name:       "provider-name",
				UID:        "provider-uid",
			}},
		},
	}
	pkgRef, _ := GetPackageOwnerReference(parent)
	pkgRef.Controller = ptr.To(false)
	controllerRef := meta.AsController(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind()))

	t.Run("SkipServerDefaultedControlledObject", func(t *testing.T) {
		desired := &extv1.CustomResourceDefinition{
			TypeMeta:   metav1.TypeMeta{APIVersion: extv1.SchemeGroupVersion.String(), Kind: "CustomResourceDefinition"},
			ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.org"},
			Spec:       extv1.CustomResourceDefinitionSpec{Group: "example.org"},
		}
		current := desired.DeepCopy()
		current.TypeMeta = metav1.TypeMeta{}
		current.SetOwnerReferences([]metav1.OwnerReference{pkgRef, controllerRef})
		current.Spec.Conversion = &extv1.CustomResourceConversion{Strategy: extv1.NoneConverter}

		e := newAPIEstablisher(nil)
		apply, _, err := e.prepareApply(current, desired, parent, true)
		if err != nil {
			t.Fatal(err)
		}
		current.SetAnnotations(map[string]string{
			annotationKeyEstablisherHash: apply.GetAnnotations()[annotationKeyEstablisherHash],
		})

		_, needed, err := e.prepareApply(current, desired, parent, true)
		if err != nil {
			t.Fatal(err)
		}
		if needed {
			t.Error("prepareApply(...) reported a change caused only by a server-defaulted field")
		}
	})

	t.Run("CreateWithControllerAndPackageOwners", func(t *testing.T) {
		desired := &corev1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		}
		apply, needed, err := newAPIEstablisher(nil).prepareApply(nil, desired, parent, true)
		if err != nil {
			t.Fatal(err)
		}
		if !needed {
			t.Error("prepareApply(...) did not report a missing controlled object")
		}
		if diff := cmp.Diff([]metav1.OwnerReference{controllerRef, pkgRef}, apply.GetOwnerReferences()); diff != "" {
			t.Errorf("prepareApply(...): -want owners, +got owners:\n%s", diff)
		}
	})

	t.Run("DetectControlledIntentChange", func(t *testing.T) {
		current := &corev1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{Name: "test", OwnerReferences: []metav1.OwnerReference{pkgRef, controllerRef}},
			Data:       map[string]string{"key": "old"},
		}
		desired := current.DeepCopy()
		desired.SetOwnerReferences(nil)
		desired.Data["key"] = "new"

		_, needed, err := newAPIEstablisher(nil).prepareApply(current, desired, parent, true)
		if err != nil {
			t.Fatal(err)
		}
		if !needed {
			t.Error("prepareApply(...) did not report changed package intent")
		}
	})

	t.Run("PreserveExistingOwnersWithoutControl", func(t *testing.T) {
		userRef := metav1.OwnerReference{APIVersion: "example.org/v1", Kind: "Owner", Name: "user", UID: "user-uid"}
		current := &corev1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{Name: "test", OwnerReferences: []metav1.OwnerReference{userRef}},
			Data:       map[string]string{"user": "data"},
		}
		desired := &corev1.ConfigMap{
			TypeMeta:   current.TypeMeta,
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Data:       map[string]string{"package": "data"},
		}

		apply, needed, err := newAPIEstablisher(nil).prepareApply(current, desired, parent, false)
		if err != nil {
			t.Fatal(err)
		}
		if !needed {
			t.Error("prepareApply(...) did not report missing owner references")
		}
		if _, found, _ := unstructured.NestedFieldNoCopy(apply.Object, "data"); found {
			t.Error("non-controlling apply included package data")
		}
		want := []metav1.OwnerReference{userRef, pkgRef, meta.AsOwner(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind()))}
		if diff := cmp.Diff(want, apply.GetOwnerReferences()); diff != "" {
			t.Errorf("prepareApply(...): -want owners, +got owners:\n%s", diff)
		}
	})

	t.Run("OmitExistingMRDState", func(t *testing.T) {
		current := &v1alpha1.ManagedResourceDefinition{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: v1alpha1.ManagedResourceDefinitionKind},
			ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.org", OwnerReferences: []metav1.OwnerReference{pkgRef, controllerRef}},
			Spec:       v1alpha1.ManagedResourceDefinitionSpec{State: v1alpha1.ManagedResourceDefinitionInactive},
		}
		desired := current.DeepCopy()
		desired.SetOwnerReferences(nil)
		desired.Spec.State = v1alpha1.ManagedResourceDefinitionActive

		apply, _, err := newAPIEstablisher(nil).prepareApply(current, desired, parent, true)
		if err != nil {
			t.Fatal(err)
		}
		if _, found, _ := unstructured.NestedFieldNoCopy(apply.Object, "spec", "state"); found {
			t.Error("controlled apply included spec.state for an existing MRD")
		}
	})

	t.Run("RejectForeignController", func(t *testing.T) {
		current := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "example.org/v1",
					Kind:       "Foreign",
					Name:       "foreign",
					UID:        "foreign-uid",
					Controller: ptr.To(true),
				}},
			},
		}
		desired := current.DeepCopy()
		desired.SetOwnerReferences(nil)

		if _, _, err := newAPIEstablisher(nil).prepareApply(current, desired, parent, true); err == nil {
			t.Error("prepareApply(...) accepted an object controlled by a foreign owner")
		}
	})
}

func TestAPIEstablisherPatchOptions(t *testing.T) {
	called := false
	e := newAPIEstablisher(&test.MockClient{
		MockPatch: func(_ context.Context, _ client.Object, p client.Patch, opts ...client.PatchOption) error {
			called = true
			if got := p.Type(); got != types.ApplyPatchType {
				t.Errorf("Patch type: want %s, got %s", types.ApplyPatchType, got)
			}
			po := (&client.PatchOptions{}).ApplyOptions(opts)
			if po.FieldManager != FieldOwnerAPIEstablisher {
				t.Errorf("Field manager: want %q, got %q", FieldOwnerAPIEstablisher, po.FieldManager)
			}
			if po.Force == nil || !*po.Force {
				t.Error("Force ownership was not enabled")
			}
			if diff := cmp.Diff([]string{metav1.DryRunAll}, po.DryRun); diff != "" {
				t.Errorf("Dry-run options: -want, +got:\n%s", diff)
			}
			return nil
		},
	})

	if err := e.patch(context.Background(), &unstructured.Unstructured{}, FieldOwnerAPIEstablisher, client.DryRunAll); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("Patch(...) was not called")
	}
}

func TestAPIEstablisherEstablishSkipsDefaultedControlledObject(t *testing.T) {
	parent := &v1.ConfigurationRevision{
		TypeMeta: metav1.TypeMeta{APIVersion: v1.SchemeGroupVersion.String(), Kind: v1.ConfigurationRevisionKind},
		ObjectMeta: metav1.ObjectMeta{
			Name: "revision-name",
			UID:  "revision-uid",
			Labels: map[string]string{
				v1.LabelParentPackage: "configuration-name",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       v1.ConfigurationKind,
				Name:       "configuration-name",
				UID:        "configuration-uid",
			}},
		},
	}
	desired := &extv1.CustomResourceDefinition{
		TypeMeta:   metav1.TypeMeta{APIVersion: extv1.SchemeGroupVersion.String(), Kind: "CustomResourceDefinition"},
		ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.org"},
		Spec:       extv1.CustomResourceDefinitionSpec{Group: "example.org"},
	}
	current := desired.DeepCopy()
	pkgRef, _ := GetPackageOwnerReference(parent)
	pkgRef.Controller = ptr.To(false)
	current.SetOwnerReferences([]metav1.OwnerReference{
		pkgRef,
		meta.AsController(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind())),
	})
	current.Spec.Conversion = &extv1.CustomResourceConversion{Strategy: extv1.NoneConverter}
	apply, _, err := newAPIEstablisher(nil).prepareApply(current, desired, parent, true)
	if err != nil {
		t.Fatal(err)
	}
	current.SetAnnotations(map[string]string{
		annotationKeyEstablisherHash: apply.GetAnnotations()[annotationKeyEstablisherHash],
	})

	e := newAPIEstablisher(&test.MockClient{
		MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
			current.DeepCopyInto(obj.(*extv1.CustomResourceDefinition))
			return nil
		},
		MockPatch: func(_ context.Context, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
			t.Error("Patch(...) called for unchanged server-defaulted object")
			return nil
		},
	})
	if _, err := e.Establish(context.Background(), []runtime.Object{desired}, parent, true); err != nil {
		t.Fatal(err)
	}
}

func TestAPIEstablisherEstablishUpdatesOnlyChangedObjects(t *testing.T) {
	parent := &v1.ProviderRevision{
		TypeMeta: metav1.TypeMeta{APIVersion: v1.SchemeGroupVersion.String(), Kind: v1.ProviderRevisionKind},
		ObjectMeta: metav1.ObjectMeta{
			Name: "revision-name",
			UID:  "revision-uid",
		},
	}

	cases := map[string]struct {
		reason       string
		currentOwner bool
		wantPatches  int
	}{
		"SkipUnchanged": {
			reason:       "Dry-run validation and real establishment should both skip an unchanged object.",
			currentOwner: true,
			wantPatches:  0,
		},
		"UpdateChanged": {
			reason:      "Dry-run validation must not hide an owner reference change from real establishment.",
			wantPatches: 2,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			desired := &corev1.ConfigMap{
				TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			}
			current := desired.DeepCopy()
			current.SetResourceVersion("42")
			if tc.currentOwner {
				current.SetOwnerReferences([]metav1.OwnerReference{
					meta.AsOwner(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind())),
				})
			}

			patches := 0
			e := newAPIEstablisher(&test.MockClient{
				MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
					current.DeepCopyInto(obj.(*corev1.ConfigMap))
					return nil
				},
				MockPatch: func(_ context.Context, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
					patches++
					return nil
				},
			})
			if _, err := e.Establish(context.Background(), []runtime.Object{desired}, parent, false); err != nil {
				t.Fatalf("\n%s\ne.Establish(...): unexpected error: %v", tc.reason, err)
			}
			if diff := cmp.Diff(tc.wantPatches, patches); diff != "" {
				t.Errorf("\n%s\ne.Establish(...): -want patches, +got patches:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAPIEstablisherEstablish(t *testing.T) {
	errBoom := errors.New("boom")
	tlsServerSecretName := "tls-server-secret"
	caBundle := []byte("CABUNDLE")

	type args struct {
		est     *APIEstablisher
		objs    []runtime.Object
		parent  v1.PackageRevision
		control bool
	}

	type want struct {
		err  error
		refs []xpv2.TypedReference
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulExistsEstablishControl": {
			reason: "Establishment should be successful if we can establish control for a parent of existing objects.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if s, ok := obj.(*corev1.Secret); ok {
							(&corev1.Secret{
								Data: map[string][]byte{
									"tls.crt": caBundle,
								},
							}).DeepCopyInto(s)
							return nil
						}
						return nil
					},
					MockPatch: test.NewMockPatchFn(nil),
				}),
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
					},
				},
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "provider-name",
								UID:  "some-unique-uid-2312",
							},
						},
						Labels: map[string]string{
							v1.LabelParentPackage: "provider-name",
						},
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
							TLSServerSecretName: &tlsServerSecretName,
						},
					},
				},
				control: true,
			},
			want: want{
				refs: []xpv2.TypedReference{{Name: "ref-me"}},
			},
		},
		"SuccessfulNotExistsEstablishControl": {
			reason: "Establishment should be successful if we can establish control for a parent of new objects.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if s, ok := obj.(*corev1.Secret); ok {
							(&corev1.Secret{
								Data: map[string][]byte{
									"tls.crt": caBundle,
								},
							}).DeepCopyInto(s)
							return nil
						}
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
					MockPatch: test.NewMockPatchFn(nil),
				}),
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
					},
				},
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "provider-name",
								UID:  "some-unique-uid-2312",
							},
						},
						Labels: map[string]string{
							v1.LabelParentPackage: "provider-name",
						},
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
							TLSServerSecretName: &tlsServerSecretName,
						},
					},
				},
				control: true,
			},
			want: want{
				refs: []xpv2.TypedReference{{Name: "ref-me"}},
			},
		},
		"SuccessfulNotExistsEstablishControlWebhookEnabledActiveRevision": {
			reason: "Establishment should be successful if we can establish control for a parent of new objects in case webhooks are enabled.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if s, ok := obj.(*corev1.Secret); ok {
							(&corev1.Secret{
								Data: map[string][]byte{
									"tls.crt": caBundle,
								},
							}).DeepCopyInto(s)
							return nil
						}
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
					MockPatch: test.NewMockPatchFn(nil),
				}),
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
						Spec: extv1.CustomResourceDefinitionSpec{
							Conversion: &extv1.CustomResourceConversion{
								Strategy: extv1.WebhookConverter,
							},
						},
					},
					&admv1.MutatingWebhookConfiguration{
						ObjectMeta: metav1.ObjectMeta{
							Name: "crossplane-providerrevision-provider-name",
						},
						Webhooks: []admv1.MutatingWebhook{
							{
								Name: "some-webhook",
							},
						},
					},
					&admv1.ValidatingWebhookConfiguration{
						ObjectMeta: metav1.ObjectMeta{
							Name: "crossplane-providerrevision-provider-name",
						},
						Webhooks: []admv1.ValidatingWebhook{
							{
								Name: "some-webhook",
							},
						},
					},
				},
				parent: &v1.ProviderRevision{
					TypeMeta: metav1.TypeMeta{
						Kind: "ProviderRevision",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "provider-name-1234",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "Provider",
								Name: "provider-name",
								UID:  "some-unique-uid-2312",
							},
						},
						Labels: map[string]string{
							v1.LabelParentPackage: "provider-name",
						},
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
							TLSServerSecretName: &tlsServerSecretName,
						},
					},
				},
				control: true,
			},
			want: want{
				refs: []xpv2.TypedReference{
					{Name: "ref-me"},
					{Name: "crossplane-provider-provider-name"},
					{Name: "crossplane-provider-provider-name"},
				},
			},
		},
		"SuccessfulExistsEstablishOwnership": {
			reason: "Establishment should be successful if we can establish ownership for a parent of existing objects.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet:   test.NewMockGetFn(nil),
					MockPatch: test.NewMockPatchFn(nil),
				}),
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
					},
				},
				parent:  &v1.ProviderRevision{},
				control: false,
			},
			want: want{
				refs: []xpv2.TypedReference{{Name: "ref-me"}},
			},
		},
		"SuccessfulNotExistsDoNotCreate": {
			reason: "Establishment should be successful if we skip creating a resource we do not want to control.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet:   test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockPatch: test.NewMockPatchFn(errBoom),
				}),
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
					},
				},
				parent:  &v1.ProviderRevision{},
				control: false,
			},
			want: want{
				refs: []xpv2.TypedReference{{Name: "ref-me"}},
			},
		},
		"FailedTLSSecretNotPresent": {
			reason: "Establishment should fail if TLS server secret is not present when trying to establish control.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet:   test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockPatch: test.NewMockPatchFn(nil),
				}),
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
					},
				},
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "provider-name",
								UID:  "some-unique-uid-2312",
							},
						},
						Labels: map[string]string{
							v1.LabelParentPackage: "provider-name",
						},
					},
				},
				control: true,
			},
			want: want{
				err: errors.New(errWebhookSecretNotPresent),
			},
		},
		"FailedCreationWebhookDisabledConversionRequested": {
			reason: "Establishment should fail if the CRD requires conversion webhook and Crossplane does not have the webhooks enabled.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if s, ok := obj.(*corev1.Secret); ok {
							// Return empty secret (no CA bundle)
							s.Data = map[string][]byte{}
							return nil
						}
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
					MockPatch: test.NewMockPatchFn(nil),
				}),
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
						Spec: extv1.CustomResourceDefinitionSpec{
							Conversion: &extv1.CustomResourceConversion{
								Strategy: extv1.WebhookConverter,
							},
						},
					},
				},
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "provider-name",
								UID:  "some-unique-uid-2312",
							},
						},
						Labels: map[string]string{
							v1.LabelParentPackage: "provider-name",
						},
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
							TLSServerSecretName: &tlsServerSecretName,
						},
					},
				},
				control: true,
			},
			want: want{
				err: errors.New(errWebhookSecretWithoutCABundle),
			},
		},
		"FailedGettingWebhookTLSSecretControl": {
			reason: "Establishment of a controlling revision should fail if a webhook TLS secret is given but cannot be fetched",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				}),
				parent: &v1.ProviderRevision{
					Status: v1.ProviderRevisionStatus{
						PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
							TLSServerSecretName: &tlsServerSecretName,
						},
					},
				},
				control: true,
			},
			want: want{
				err: errors.Wrap(errBoom, errGetWebhookTLSSecret),
			},
		},
		"NoErrGettingWebhookTLSSecretNoControl": {
			reason: "Establishment of a revision should not fail if a webhook TLS secret is given but cannot be fetched if we don't want to control resources",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				}),
				parent: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
							TLSServerSecretName: &tlsServerSecretName,
						},
					},
				},
				control: false,
			},
			want: want{
				err: nil,
			},
		},
		"FailedEmptyWebhookTLSSecretControl": {
			reason: "Establishment should fail for a controlling revision if a webhook TLS secret is given but empty if we want to control resources",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						s := &corev1.Secret{}
						s.DeepCopyInto(obj.(*corev1.Secret))
						return nil
					},
				}),
				parent: &v1.ProviderRevision{
					Status: v1.ProviderRevisionStatus{
						PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
							TLSServerSecretName: &tlsServerSecretName,
						},
					},
				},
				control: true,
			},
			want: want{
				err: errors.New(errWebhookSecretWithoutCABundle),
			},
		},
		"NoErrEmptyWebhookTLSSecretNoControl": {
			reason: "Establishment should not fail for an revision if a webhook TLS secret is given but empty if we don't want to control resources",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						s := &corev1.Secret{}
						s.DeepCopyInto(obj.(*corev1.Secret))
						return nil
					},
				}),
				parent: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
							TLSServerSecretName: &tlsServerSecretName,
						},
					},
				},
				control: false,
			},
			want: want{
				err: nil,
			},
		},
		"FailedCreateApply": {
			reason: "Cannot establish control of object if we cannot apply it during creation.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if s, ok := obj.(*corev1.Secret); ok {
							(&corev1.Secret{
								Data: map[string][]byte{
									"tls.crt": caBundle,
								},
							}).DeepCopyInto(s)
							return nil
						}
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
					MockPatch: test.NewMockPatchFn(errBoom),
				}),
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
					},
				},
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
							TLSServerSecretName: &tlsServerSecretName,
						},
					},
				},
				control: true,
			},
			want: want{
				err: errBoom,
			},
		},
		"FailedUpdateApply": {
			reason: "Cannot establish control of an existing object if we cannot apply it.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if s, ok := obj.(*corev1.Secret); ok {
							(&corev1.Secret{
								Data: map[string][]byte{
									"tls.crt": caBundle,
								},
							}).DeepCopyInto(s)
							return nil
						}
						return nil
					},
					MockPatch: test.NewMockPatchFn(errBoom),
				}),
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
					},
				},
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
							TLSServerSecretName: &tlsServerSecretName,
						},
					},
				},
				control: true,
			},
			want: want{
				err: errBoom,
			},
		},
		"SuccessfulManagedResourceDefinitionUnsetState": {
			reason: "Establishment should be successful for ManagedResourceDefinitions with various spec.state values.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if s, ok := obj.(*corev1.Secret); ok {
							(&corev1.Secret{
								Data: map[string][]byte{
									"tls.crt": caBundle,
								},
							}).DeepCopyInto(s)
							return nil
						}
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
					MockPatch: test.NewMockPatchFn(nil),
				}),
				objs: []runtime.Object{
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-mrd-unset",
						},
						Spec: v1alpha1.ManagedResourceDefinitionSpec{
							// spec.state field is intentionally unset (zero value)
						},
					},
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-mrd-active",
						},
						Spec: v1alpha1.ManagedResourceDefinitionSpec{
							State: v1alpha1.ManagedResourceDefinitionActive,
						},
					},
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-mrd-inactive",
						},
						Spec: v1alpha1.ManagedResourceDefinitionSpec{
							State: v1alpha1.ManagedResourceDefinitionInactive,
						},
					},
				},
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "provider-name",
								UID:  "some-unique-uid-2312",
							},
						},
						Labels: map[string]string{
							v1.LabelParentPackage: "provider-name",
						},
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
							TLSServerSecretName: &tlsServerSecretName,
						},
					},
				},
				control: true,
			},
			want: want{
				refs: []xpv2.TypedReference{
					{Name: "test-mrd-unset"},
					{Name: "test-mrd-active"},
					{Name: "test-mrd-inactive"},
				},
			},
		},
		"SuccessfulManagedResourceDefinitionAllStateCombinations": {
			reason: "Establishment should omit state for existing ManagedResourceDefinitions regardless of current and desired state.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if s, ok := obj.(*corev1.Secret); ok {
							(&corev1.Secret{
								Data: map[string][]byte{
									"tls.crt": caBundle,
								},
							}).DeepCopyInto(s)
							return nil
						}
						if mrd, ok := obj.(*v1alpha1.ManagedResourceDefinition); ok {
							switch mrd.GetName() {
							case "active-to-unset":
								// Existing: Active, Desired: Unset -> Expected: Active (preserve existing)
								(&v1alpha1.ManagedResourceDefinition{
									ObjectMeta: metav1.ObjectMeta{
										Name:            "active-to-unset",
										ResourceVersion: "100",
									},
									Spec: v1alpha1.ManagedResourceDefinitionSpec{
										State: v1alpha1.ManagedResourceDefinitionActive,
									},
								}).DeepCopyInto(mrd)
							case "active-to-active":
								// Existing: Active, Desired: Active -> Expected: Active (use desired)
								(&v1alpha1.ManagedResourceDefinition{
									ObjectMeta: metav1.ObjectMeta{
										Name:            "active-to-active",
										ResourceVersion: "101",
									},
									Spec: v1alpha1.ManagedResourceDefinitionSpec{
										State: v1alpha1.ManagedResourceDefinitionActive,
									},
								}).DeepCopyInto(mrd)
							case "active-to-inactive":
								// Existing: Active, Desired: Inactive -> Expected: Active (preserve existing)
								(&v1alpha1.ManagedResourceDefinition{
									ObjectMeta: metav1.ObjectMeta{
										Name:            "active-to-inactive",
										ResourceVersion: "102",
									},
									Spec: v1alpha1.ManagedResourceDefinitionSpec{
										State: v1alpha1.ManagedResourceDefinitionActive,
									},
								}).DeepCopyInto(mrd)
							case "inactive-to-unset":
								// Existing: Inactive, Desired: Unset -> Expected: Inactive (preserve existing)
								(&v1alpha1.ManagedResourceDefinition{
									ObjectMeta: metav1.ObjectMeta{
										Name:            "inactive-to-unset",
										ResourceVersion: "103",
									},
									Spec: v1alpha1.ManagedResourceDefinitionSpec{
										State: v1alpha1.ManagedResourceDefinitionInactive,
									},
								}).DeepCopyInto(mrd)
							case "inactive-to-active":
								// Existing: Inactive, Desired: Active -> Expected: Active (use desired)
								(&v1alpha1.ManagedResourceDefinition{
									ObjectMeta: metav1.ObjectMeta{
										Name:            "inactive-to-active",
										ResourceVersion: "104",
									},
									Spec: v1alpha1.ManagedResourceDefinitionSpec{
										State: v1alpha1.ManagedResourceDefinitionInactive,
									},
								}).DeepCopyInto(mrd)
							case "inactive-to-inactive":
								// Existing: Inactive, Desired: Inactive -> Expected: Inactive (preserve existing)
								(&v1alpha1.ManagedResourceDefinition{
									ObjectMeta: metav1.ObjectMeta{
										Name:            "inactive-to-inactive",
										ResourceVersion: "105",
									},
									Spec: v1alpha1.ManagedResourceDefinitionSpec{
										State: v1alpha1.ManagedResourceDefinitionInactive,
									},
								}).DeepCopyInto(mrd)
							}
							return nil
						}
						return nil
					},
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						u, ok := obj.(*unstructured.Unstructured)
						if !ok {
							return errors.Errorf("expected unstructured apply object, got %T", obj)
						}
						if state, found, _ := unstructured.NestedFieldNoCopy(u.Object, "spec", "state"); found {
							return errors.Errorf("expected spec.state to be omitted for %s, got %v", u.GetName(), state)
						}
						return nil
					},
				}),
				objs: []runtime.Object{
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "active-to-unset",
						},
						Spec: v1alpha1.ManagedResourceDefinitionSpec{
							// spec.state field is intentionally unset (zero value)
						},
					},
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "active-to-active",
						},
						Spec: v1alpha1.ManagedResourceDefinitionSpec{
							State: v1alpha1.ManagedResourceDefinitionActive,
						},
					},
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "active-to-inactive",
						},
						Spec: v1alpha1.ManagedResourceDefinitionSpec{
							State: v1alpha1.ManagedResourceDefinitionInactive,
						},
					},
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "inactive-to-unset",
						},
						Spec: v1alpha1.ManagedResourceDefinitionSpec{
							// spec.state field is intentionally unset (zero value)
						},
					},
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "inactive-to-active",
						},
						Spec: v1alpha1.ManagedResourceDefinitionSpec{
							State: v1alpha1.ManagedResourceDefinitionActive,
						},
					},
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "inactive-to-inactive",
						},
						Spec: v1alpha1.ManagedResourceDefinitionSpec{
							State: v1alpha1.ManagedResourceDefinitionInactive,
						},
					},
				},
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "provider-name",
								UID:  "some-unique-uid-2312",
							},
						},
						Labels: map[string]string{
							v1.LabelParentPackage: "provider-name",
						},
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
							TLSServerSecretName: &tlsServerSecretName,
						},
					},
				},
				control: true,
			},
			want: want{
				refs: []xpv2.TypedReference{
					{Name: "active-to-unset"},
					{Name: "active-to-active"},
					{Name: "active-to-inactive"},
					{Name: "inactive-to-unset"},
					{Name: "inactive-to-active"},
					{Name: "inactive-to-inactive"},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			refs, err := tc.args.est.Establish(context.TODO(), tc.args.objs, tc.args.parent, tc.args.control)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors(), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\ne.Check(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			sort := cmpopts.SortSlices(func(x, y xpv2.TypedReference) bool {
				return x.Name < y.Name
			})
			if diff := cmp.Diff(tc.want.refs, refs, test.EquateErrors(), sort, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\ne.Check(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAPIEstablisherReleaseObjects(t *testing.T) {
	errBoom := errors.New("boom")
	controls := true
	noControl := false

	type args struct {
		est    *APIEstablisher
		parent v1.PackageRevision
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"CannotGetObject": {
			reason: "Should return an error if we cannot get the owned object.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return errBoom
					},
				}),
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						UID: "some-unique-uid-2312",
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ObjectRefs: []xpv2.TypedReference{
								{
									APIVersion: "apiextensions.k8s.io/v1",
									Kind:       "CustomResourceDefinition",
									Name:       "releases.helm.crossplane.io",
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtGetOwnedObject, "CustomResourceDefinition", "releases.helm.crossplane.io"),
			},
		},
		"IgnoreOwnedObjectNotFound": {
			reason: "Should ignore if we the owned object does not exist.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
				}),
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						UID: "some-unique-uid-2312",
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ObjectRefs: []xpv2.TypedReference{
								{
									APIVersion: "apiextensions.k8s.io/v1",
									Kind:       "CustomResourceDefinition",
									Name:       "releases.helm.crossplane.io",
								},
							},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"CannotUpdate": {
			reason: "Should return an error if we cannot update the owned object.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						o := obj.(*unstructured.Unstructured)
						o.SetOwnerReferences([]metav1.OwnerReference{
							{
								APIVersion: "pkg.crossplane.io/v1",
								Kind:       "Provider",
								Name:       "provider-helm",
								UID:        "some-other-uid-1234",
								Controller: &noControl,
							},
							{
								APIVersion: "pkg.crossplane.io/v1",
								Kind:       "ProviderRevision",
								Name:       "provider-helm-ce18dd03e6e4",
								UID:        "some-unique-uid-2312",
								Controller: &controls,
							},
						})
						return nil
					},
					MockUpdate: func(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
						return errBoom
					},
				}),
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						UID: "some-unique-uid-2312",
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ObjectRefs: []xpv2.TypedReference{
								{
									APIVersion: "apiextensions.k8s.io/v1",
									Kind:       "CustomResourceDefinition",
									Name:       "releases.helm.crossplane.io",
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtUpdateOwnedObject, "CustomResourceDefinition", "releases.helm.crossplane.io"),
			},
		},
		"NoObjectsInStatus": {
			reason: "Should not return an error if there are no objects in the status.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return nil
					},
					MockUpdate: func(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
						return nil
					},
				}),
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						UID: "some-unique-uid-2312",
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"AlreadyReleased": {
			reason: "ReleaseObjects should make no updates if the object is already released.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						o := obj.(*unstructured.Unstructured)
						o.SetOwnerReferences([]metav1.OwnerReference{
							{
								APIVersion: "pkg.crossplane.io/v1",
								Kind:       "Provider",
								Name:       "provider-helm",
								UID:        "some-other-uid-1234",
								Controller: &noControl,
							},
							{
								APIVersion: "pkg.crossplane.io/v1",
								Kind:       "ProviderRevision",
								Name:       "provider-helm-ce18dd03e6e4",
								UID:        "some-unique-uid-2312",
								Controller: &noControl,
							},
						})
						return nil
					},
					MockUpdate: func(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
						t.Errorf("should not have called update")
						return nil
					},
				}),
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						UID: "some-unique-uid-2312",
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ObjectRefs: []xpv2.TypedReference{
								{
									APIVersion: "apiextensions.k8s.io/v1",
									Kind:       "CustomResourceDefinition",
									Name:       "releases.helm.crossplane.io",
								},
							},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"OwnedIfNotAlready": {
			reason: "ReleaseObjects should put owner reference back if we are not already the owner.",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						o := obj.(*unstructured.Unstructured)
						o.SetOwnerReferences([]metav1.OwnerReference{
							{
								APIVersion: "pkg.crossplane.io/v1",
								Kind:       "Provider",
								Name:       "provider-helm",
								UID:        "some-other-uid-1234",
								Controller: &noControl,
							},
						})
						return nil
					},
					MockUpdate: func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
						o := obj.(*unstructured.Unstructured)
						if len(o.GetOwnerReferences()) != 2 {
							t.Errorf("expected 2 owner references, got %d", len(o.GetOwnerReferences()))
						}
						found := false
						for _, ref := range o.GetOwnerReferences() {
							if ref.Kind == "ProviderRevision" && ref.UID == "some-unique-uid-2312" {
								found = true
								if ptr.Deref(ref.Controller, false) {
									t.Errorf("expected controller to be false, got %t", *ref.Controller)
								}
							}
						}
						if !found {
							t.Errorf("expected to find owner reference for revision with uid some-unique-uid-2312")
						}
						return nil
					},
				}),
				parent: &v1.ProviderRevision{
					TypeMeta: metav1.TypeMeta{
						Kind: "ProviderRevision",
					},
					ObjectMeta: metav1.ObjectMeta{
						UID: "some-unique-uid-2312",
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ObjectRefs: []xpv2.TypedReference{
								{
									APIVersion: "apiextensions.k8s.io/v1",
									Kind:       "CustomResourceDefinition",
									Name:       "releases.helm.crossplane.io",
								},
							},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulRelease": {
			reason: "ReleaseObjects should be successful if we can release control of existing objects",
			args: args{
				est: newAPIEstablisher(&test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						o := obj.(*unstructured.Unstructured)
						o.SetOwnerReferences([]metav1.OwnerReference{
							{
								APIVersion: "pkg.crossplane.io/v1",
								Kind:       "Provider",
								Name:       "provider-helm",
								UID:        "some-other-uid-1234",
								Controller: &noControl,
							},
							{
								APIVersion: "pkg.crossplane.io/v1",
								Kind:       "ProviderRevision",
								Name:       "provider-helm-ce18dd03e6e4",
								UID:        "some-unique-uid-2312",
								Controller: &controls,
							},
						})
						return nil
					},
					MockUpdate: func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
						o := obj.(*unstructured.Unstructured)
						if len(o.GetOwnerReferences()) != 2 {
							t.Errorf("expected 2 owner references, got %d", len(o.GetOwnerReferences()))
						}
						for _, ref := range o.GetOwnerReferences() {
							if ref.UID == "some-unique-uid-2312" && *ref.Controller {
								t.Errorf("expected controller to be false, got %t", *ref.Controller)
							}
						}
						return nil
					},
				}),
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						UID: "some-unique-uid-2312",
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ObjectRefs: []xpv2.TypedReference{
								{
									APIVersion: "apiextensions.k8s.io/v1",
									Kind:       "CustomResourceDefinition",
									Name:       "releases.helm.crossplane.io",
								},
							},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.est.ReleaseObjects(context.TODO(), tc.args.parent)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Check(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetPackageOwnerReference(t *testing.T) {
	type args struct {
		revision resource.Object
	}

	type want struct {
		ref metav1.OwnerReference
		ok  bool
	}

	ref := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Provider",
		Name:       "provider-name",
		UID:        types.UID("some-random-uid"),
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Found": {
			reason: "We need to correctly find the owner reference of the parent package",
			args: args{
				revision: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{},
							ref,
							{
								Name: "something-else",
							},
						},
						Labels: map[string]string{
							v1.LabelParentPackage: "provider-name",
						},
					},
				},
			},
			want: want{
				ref: ref,
				ok:  true,
			},
		},
		"NotFound": {
			args: args{
				revision: &v1.ProviderRevision{},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result, ok := GetPackageOwnerReference(tc.args.revision)

			if diff := cmp.Diff(tc.want.ref, result); diff != "" {
				t.Errorf("\n%s\ne.GetPackageOwnerReference(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.ok, ok); diff != "" {
				t.Errorf("\n%s\ne.GetPackageOwnerReference(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAddLabels(t *testing.T) {
	type args struct {
		objs   []runtime.Object
		parent v1.PackageRevision
	}

	type want struct {
		labels map[string]string
		err    error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoCommonLabels": {
			reason: "Objects should not be modified when no common labels are set.",
			args: args{
				objs: []runtime.Object{
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{Name: "mrd-a"},
					},
				},
				parent: &v1.ProviderRevision{},
			},
			want: want{
				labels: nil,
			},
		},
		"SetsLabelsOnObjectWithoutLabels": {
			reason: "Common labels should be set on an object that has no existing labels.",
			args: args{
				objs: []runtime.Object{
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{Name: "mrd-a"},
					},
				},
				parent: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							CommonLabels: map[string]string{"env": "prod"},
						},
					},
				},
			},
			want: want{
				labels: map[string]string{"env": "prod"},
			},
		},
		"MergesLabelsOnObjectWithExistingLabels": {
			reason: "Common labels should be merged with existing labels on an object.",
			args: args{
				objs: []runtime.Object{
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "mrd-a",
							Labels: map[string]string{"existing": "value"},
						},
					},
				},
				parent: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							CommonLabels: map[string]string{"env": "prod"},
						},
					},
				},
			},
			want: want{
				labels: map[string]string{"existing": "value", "env": "prod"},
			},
		},
		"OverwritesExistingLabelWithCommonLabel": {
			reason: "A common label should overwrite an existing label with the same key.",
			args: args{
				objs: []runtime.Object{
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "mrd-a",
							Labels: map[string]string{"env": "staging"},
						},
					},
				},
				parent: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							CommonLabels: map[string]string{"env": "prod"},
						},
					},
				},
			},
			want: want{
				labels: map[string]string{"env": "prod"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newAPIEstablisher(nil)
			err := e.addLabels(tc.args.objs, tc.args.parent)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\naddLabels(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if err != nil {
				return
			}
			obj := tc.args.objs[0].(metav1.Object)
			if diff := cmp.Diff(tc.want.labels, obj.GetLabels()); diff != "" {
				t.Errorf("\n%s\naddLabels(...): -want labels, +got labels:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAddAnnotations(t *testing.T) {
	type args struct {
		objs   []runtime.Object
		parent v1.PackageRevision
	}

	type want struct {
		annotations map[string]string
		err         error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoCommonAnnotations": {
			reason: "Objects should not be modified when no common annotations are set.",
			args: args{
				objs: []runtime.Object{
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{Name: "mrd-a"},
					},
				},
				parent: &v1.ProviderRevision{},
			},
			want: want{
				annotations: nil,
			},
		},
		"SetsAnnotationsOnObjectWithoutAnnotations": {
			reason: "Common annotations should be set on an object that has no existing annotations.",
			args: args{
				objs: []runtime.Object{
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{Name: "mrd-a"},
					},
				},
				parent: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							CommonAnnotations: map[string]string{"owner": "team-a"},
						},
					},
				},
			},
			want: want{
				annotations: map[string]string{"owner": "team-a"},
			},
		},
		"MergesAnnotationsOnObjectWithExistingAnnotations": {
			reason: "Common annotations should be merged with existing annotations on an object.",
			args: args{
				objs: []runtime.Object{
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "mrd-a",
							Annotations: map[string]string{"existing": "value"},
						},
					},
				},
				parent: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							CommonAnnotations: map[string]string{"owner": "team-a"},
						},
					},
				},
			},
			want: want{
				annotations: map[string]string{"existing": "value", "owner": "team-a"},
			},
		},
		"OverwritesExistingAnnotationWithCommonAnnotation": {
			reason: "A common annotation should overwrite an existing annotation with the same key.",
			args: args{
				objs: []runtime.Object{
					&v1alpha1.ManagedResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "mrd-a",
							Annotations: map[string]string{"owner": "team-b"},
						},
					},
				},
				parent: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							CommonAnnotations: map[string]string{"owner": "team-a"},
						},
					},
				},
			},
			want: want{
				annotations: map[string]string{"owner": "team-a"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newAPIEstablisher(nil)
			err := e.addAnnotations(tc.args.objs, tc.args.parent)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\naddAnnotations(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if err != nil {
				return
			}
			obj := tc.args.objs[0].(metav1.Object)
			if diff := cmp.Diff(tc.want.annotations, obj.GetAnnotations()); diff != "" {
				t.Errorf("\n%s\naddAnnotations(...): -want annotations, +got annotations:\n%s", tc.reason, diff)
			}
		})
	}
}

func newAPIEstablisher(client client.Client) *APIEstablisher {
	return &APIEstablisher{
		client:                           client,
		MaxConcurrentPackageEstablishers: 10, // Use the current default
	}
}

func TestFilteringEstablisherEstablish(t *testing.T) {
	errBoom := errors.New("boom")

	crd := &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: extv1.SchemeGroupVersion.String(),
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-crd",
		},
	}

	sa := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sa",
		},
	}

	type args struct {
		wrap Establisher
		gks  []schema.GroupKind
		objs []runtime.Object
	}

	type want struct {
		refs []xpv2.TypedReference
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"FilterPartialMatch": {
			reason: "Should only pass objects matching the filter to the wrapped establisher",
			args: args{
				wrap: &MockEstablisher{
					MockEstablish: func(_ context.Context, objects []runtime.Object, _ v1.PackageRevision, _ bool) ([]xpv2.TypedReference, error) {
						if diff := cmp.Diff([]runtime.Object{crd}, objects); diff != "" {
							t.Errorf("\n%s\nMockEstablish(...): -want error, +got error:\n%s", "incorrect objects passed to wrapped establisher", diff)
							return nil, errBoom
						}

						return []xpv2.TypedReference{{Name: "test-crd"}}, nil
					},
				},
				gks:  []schema.GroupKind{crd.GroupVersionKind().GroupKind()},
				objs: []runtime.Object{crd, sa},
			},
			want: want{
				refs: []xpv2.TypedReference{{Name: "test-crd"}},
			},
		},
		"FilterFullMatch": {
			reason: "Should pass all objects matching any of the filters to the wrapped establisher",
			args: args{
				wrap: &MockEstablisher{
					MockEstablish: func(_ context.Context, objects []runtime.Object, _ v1.PackageRevision, _ bool) ([]xpv2.TypedReference, error) {
						if diff := cmp.Diff([]runtime.Object{crd, sa}, objects); diff != "" {
							t.Errorf("\n%s\nMockEstablish(...): -want error, +got error:\n%s", "incorrect objects passed to wrapped establisher", diff)
							return nil, errBoom
						}

						return []xpv2.TypedReference{{Name: "test-crd"}, {Name: "test-sa"}}, nil
					},
				},
				gks:  []schema.GroupKind{crd.GroupVersionKind().GroupKind(), sa.GroupVersionKind().GroupKind()},
				objs: []runtime.Object{crd, sa},
			},
			want: want{
				refs: []xpv2.TypedReference{{Name: "test-crd"}, {Name: "test-sa"}},
			},
		},
		"FilterNoMatches": {
			reason: "Should pass no objects to the wrapped establisher if none match the filter",
			args: args{
				wrap: &MockEstablisher{
					MockEstablish: func(_ context.Context, objects []runtime.Object, _ v1.PackageRevision, _ bool) ([]xpv2.TypedReference, error) {
						if diff := cmp.Diff([]runtime.Object{}, objects); diff != "" {
							t.Errorf("\n%s\nMockEstablish(...): -want error, +got error:\n%s", "incorrect objects passed to wrapped establisher", diff)
							return nil, errBoom
						}

						return []xpv2.TypedReference{}, nil
					},
				},
				gks:  []schema.GroupKind{{Group: "example.com", Kind: "CustomKind"}},
				objs: []runtime.Object{crd, sa},
			},
			want: want{
				refs: []xpv2.TypedReference{},
			},
		},
		"FilterEmpty": {
			reason: "Should pass no objects to the wrapped establisher if empty filter is specified",
			args: args{
				wrap: &MockEstablisher{
					MockEstablish: func(_ context.Context, objects []runtime.Object, _ v1.PackageRevision, _ bool) ([]xpv2.TypedReference, error) {
						if diff := cmp.Diff([]runtime.Object{}, objects); diff != "" {
							t.Errorf("\n%s\nMockEstablish(...): -want error, +got error:\n%s", "incorrect objects passed to wrapped establisher", diff)
							return nil, errBoom
						}

						return []xpv2.TypedReference{}, nil
					},
				},
				gks:  []schema.GroupKind{},
				objs: []runtime.Object{crd, sa},
			},
			want: want{
				refs: []xpv2.TypedReference{},
			},
		},
		"ErrorFromWrappedEstablisher": {
			reason: "Should propagate errors from the wrapped establisher",
			args: args{
				wrap: &MockEstablisher{
					MockEstablish: func(_ context.Context, _ []runtime.Object, _ v1.PackageRevision, _ bool) ([]xpv2.TypedReference, error) {
						return nil, errBoom
					},
				},
				gks:  []schema.GroupKind{crd.GroupVersionKind().GroupKind()},
				objs: []runtime.Object{crd},
			},
			want: want{
				err: errBoom,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			est := NewFilteringEstablisher(tc.args.wrap, tc.args.gks...)
			refs, err := est.Establish(context.Background(), tc.args.objs, &v1.ProviderRevision{}, true)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nest.Establish(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.refs, refs); diff != "" {
				t.Errorf("\n%s\nest.Establish(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
