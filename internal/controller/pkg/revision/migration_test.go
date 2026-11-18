/*
Copyright 2026 The Crossplane Authors.

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
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/controller/rbac/grant"
	"github.com/crossplane/crossplane/v2/internal/converter"
)

func TestEstablishConvertedCRDOwnership(t *testing.T) {
	provider := metav1.OwnerReference{APIVersion: "pkg.crossplane.io/v1", Kind: "Provider", Name: "provider", UID: "provider-uid", Controller: new(true)}
	revision := &v1.ProviderRevision{
		TypeMeta:   metav1.TypeMeta{APIVersion: "pkg.crossplane.io/v1", Kind: "ProviderRevision"},
		ObjectMeta: metav1.ObjectMeta{Name: "revision-new", UID: "revision-new-uid", Labels: map[string]string{v1.LabelParentPackage: provider.Name}, OwnerReferences: []metav1.OwnerReference{provider}},
		Status:     v1.ProviderRevisionStatus{PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{TLSServerSecretName: new("tls")}},
	}
	oldRevision := meta.AsController(meta.TypedReferenceTo(revision, v1.ProviderRevisionGroupVersionKind))
	oldRevision.Name = "revision-old"
	oldRevision.UID = "revision-old-uid"
	provider.Controller = new(false)
	foreignUID := provider
	foreignUID.UID = "foreign-uid"
	foreignName := provider
	foreignName.Name = "foreign-name"
	foreignKind := provider
	foreignKind.Kind = "Configuration"
	foreignGroup := provider
	foreignGroup.APIVersion = "foreign.example.org/v1"

	for name, tc := range map[string]struct {
		owners                                                                 []metav1.OwnerReference
		inactiveRevision, inactiveMRD, statusOnly, absentCRD, race, failCreate bool
		wantLink, wantError                                                    bool
	}{
		"LegacyProvider":                   {owners: []metav1.OwnerReference{provider, oldRevision}, wantLink: true},
		"CurrentRevision":                  {owners: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(revision, v1.ProviderRevisionGroupVersionKind))}, wantLink: true},
		"InactiveMRDRetainsRevisionGrants": {owners: []metav1.OwnerReference{provider, oldRevision}, inactiveMRD: true, wantLink: true},
		"InactiveRevision":                 {owners: []metav1.OwnerReference{provider, oldRevision}, inactiveRevision: true},
		"StatusOnly":                       {owners: []metav1.OwnerReference{provider, oldRevision}, statusOnly: true},
		"NoExistingCRD":                    {absentCRD: true},
		"UnownedCRD":                       {},
		"ForeignUID":                       {owners: []metav1.OwnerReference{foreignUID}},
		"ForeignName":                      {owners: []metav1.OwnerReference{foreignName}},
		"ForeignKind":                      {owners: []metav1.OwnerReference{foreignKind}},
		"ForeignGroup":                     {owners: []metav1.OwnerReference{foreignGroup}},
		"OwnershipRace":                    {owners: []metav1.OwnerReference{provider, oldRevision}, race: true, wantError: true},
		"FailedMRDCreate":                  {owners: []metav1.OwnerReference{provider, oldRevision}, failCreate: true, wantError: true},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, extv1.AddToScheme, v1alpha1.AddToScheme, v1.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			content := &extv1.CustomResourceDefinition{
				TypeMeta:   metav1.TypeMeta{APIVersion: extv1.SchemeGroupVersion.String(), Kind: "CustomResourceDefinition"},
				ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.org"},
				Spec:       extv1.CustomResourceDefinitionSpec{Group: "example.org", Scope: extv1.ClusterScoped, Names: extv1.CustomResourceDefinitionNames{Kind: "Widget", Plural: "widgets"}},
			}
			existing := content.DeepCopy()
			existing.UID = "crd-uid"
			existing.OwnerReferences = tc.owners
			objects := []client.Object{&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: "crossplane-system"}, Data: map[string][]byte{"tls.crt": []byte("test-ca")}}}
			if !tc.absentCRD {
				objects = append(objects, existing)
			}
			patches := 0
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithIndex(grant.CRDMetadata(), grant.IndexOwnerUID, grant.OwnerUIDs).WithInterceptorFuncs(interceptor.Funcs{
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					if len((&client.CreateOptions{}).ApplyOptions(opts).DryRun) == 0 {
						if tc.failCreate {
							return kerrors.NewAlreadyExists(schema.GroupResource{}, obj.GetName())
						}
						// The fake client does not allocate UIDs like the API server.
						obj.SetUID("mrd-uid")
					}
					return c.Create(ctx, obj, opts...)
				},
				Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					patches++
					if tc.race {
						changed := &extv1.CustomResourceDefinition{}
						if err := c.Get(ctx, client.ObjectKeyFromObject(obj), changed); err != nil {
							return err
						}
						changed.OwnerReferences = []metav1.OwnerReference{foreignUID}
						if err := c.Update(ctx, changed); err != nil {
							return err
						}
					}
					return c.Patch(ctx, obj, patch, opts...)
				},
			}).Build()
			pkgObjects, err := converter.CustomToManagedResourceDefinitions(!tc.inactiveMRD, content)
			if err != nil {
				t.Fatal(err)
			}
			pr := revision.DeepCopy()
			if tc.statusOnly {
				pr.Status.ObjectRefs = []xpv2.TypedReference{{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: v1alpha1.ManagedResourceDefinitionKind, Name: content.Name}}
				pkgObjects = nil
			}
			e := NewAPIEstablisher(c, "crossplane-system", func() v1.PackageRevision { return &v1.ProviderRevision{} }, 1)
			_, err = e.Establish(ctx, pkgObjects, pr, !tc.inactiveRevision)
			if (err != nil) != tc.wantError {
				t.Fatalf("Establish error = %v, want error %v", err, tc.wantError)
			}
			if tc.race && !kerrors.IsConflict(err) {
				t.Fatalf("ownership race must return conflict, got %v", err)
			}
			if tc.absentCRD {
				if patches != 0 {
					t.Fatal("patched a missing CRD")
				}
				return
			}
			got := &extv1.CustomResourceDefinition{}
			if err := c.Get(ctx, client.ObjectKeyFromObject(existing), got); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(content.Spec, got.Spec); diff != "" {
				t.Errorf("CRD schema changed during ownership handoff (-want +got):\n%s", diff)
			}
			mrdRef := metav1.OwnerReference{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: v1alpha1.ManagedResourceDefinitionKind, Name: content.Name, UID: "mrd-uid"}
			if hasOwner(got, mrdRef) != tc.wantLink {
				t.Errorf("MRD ownership = %v, want %v", got.OwnerReferences, tc.wantLink)
			}
			if tc.wantLink {
				if !hasOwner(got, meta.AsOwner(meta.TypedReferenceTo(pr, v1.ProviderRevisionGroupVersionKind))) {
					t.Fatal("revision did not acquire CRD ownership")
				}
				if diff := cmp.Diff(metav1.GetControllerOf(existing), metav1.GetControllerOf(got)); diff != "" {
					t.Errorf("handoff changed CRD control (-want +got):\n%s", diff)
				}
				owned, err := grant.OwnedCRDs(ctx, c, pr, v1.ProviderRevisionGroupVersionKind.GroupKind())
				if err != nil || len(owned) != 1 {
					t.Errorf("new revision owned CRDs = %v, %v", owned, err)
				}
				mrd := &v1alpha1.ManagedResourceDefinition{}
				if err := c.Get(ctx, client.ObjectKey{Name: content.Name}, mrd); err != nil {
					t.Fatal(err)
				}
				if err := e.establishMRDCRDOwnership(ctx, mrd, pr); err != nil {
					t.Fatal(err)
				}
				if patches != 1 {
					t.Errorf("unchanged ownership caused %d patches, want 1", patches)
				}
			} else if !tc.race && patches != 0 {
				t.Errorf("unexpected CRD writes: %d", patches)
			}
		})
	}
}

// A CRD this package can't adopt must leave only its own MRD blocked, not
// stop the rest of the package from establishing.
func TestEstablishSkipsUnlinkableCRD(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, extv1.AddToScheme, v1alpha1.AddToScheme, v1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	provider := metav1.OwnerReference{APIVersion: "pkg.crossplane.io/v1", Kind: "Provider", Name: "provider", UID: "provider-uid", Controller: new(true)}
	pr := &v1.ProviderRevision{
		TypeMeta:   metav1.TypeMeta{APIVersion: "pkg.crossplane.io/v1", Kind: "ProviderRevision"},
		ObjectMeta: metav1.ObjectMeta{Name: "revision", UID: "revision-uid", Labels: map[string]string{v1.LabelParentPackage: provider.Name}, OwnerReferences: []metav1.OwnerReference{provider}},
		Status:     v1.ProviderRevisionStatus{PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{TLSServerSecretName: new("tls")}},
	}
	crd := func(plural string) *extv1.CustomResourceDefinition {
		return &extv1.CustomResourceDefinition{
			TypeMeta:   metav1.TypeMeta{APIVersion: extv1.SchemeGroupVersion.String(), Kind: "CustomResourceDefinition"},
			ObjectMeta: metav1.ObjectMeta{Name: plural + ".example.org"},
			Spec:       extv1.CustomResourceDefinitionSpec{Group: "example.org", Scope: extv1.ClusterScoped, Names: extv1.CustomResourceDefinitionNames{Kind: plural, Plural: plural}},
		}
	}
	foreign := crd("gadgets")
	foreign.UID = "foreign-crd-uid"
	foreign.OwnerReferences = []metav1.OwnerReference{{APIVersion: "pkg.crossplane.io/v1", Kind: "Provider", Name: "other", UID: "other-uid"}}
	tls := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: "crossplane-system"}, Data: map[string][]byte{"tls.crt": []byte("test-ca")}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tls, foreign.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if len((&client.CreateOptions{}).ApplyOptions(opts).DryRun) == 0 {
				// The fake client does not allocate UIDs like the API server.
				obj.SetUID(types.UID(obj.GetName() + "-uid"))
			}
			return c.Create(ctx, obj, opts...)
		},
	}).Build()

	objs, err := converter.CustomToManagedResourceDefinitions(true, crd("widgets"), crd("gadgets"))
	if err != nil {
		t.Fatal(err)
	}
	refs, err := NewAPIEstablisher(c, "crossplane-system", func() v1.PackageRevision { return &v1.ProviderRevision{} }, 1).Establish(ctx, objs, pr, true)
	if err != nil {
		t.Fatalf("Establish failed for the whole package: %v", err)
	}
	if len(refs) != len(objs) {
		t.Errorf("Establish returned %d object references, want %d", len(refs), len(objs))
	}
	for _, name := range []string{"widgets.example.org", "gadgets.example.org"} {
		if err := c.Get(ctx, client.ObjectKey{Name: name}, &v1alpha1.ManagedResourceDefinition{}); err != nil {
			t.Errorf("MRD %q was not established: %v", name, err)
		}
	}
	got := &extv1.CustomResourceDefinition{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(foreign), got); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(foreign.OwnerReferences, got.OwnerReferences); diff != "" {
		t.Errorf("unlinkable CRD ownership changed (-want +got):\n%s", diff)
	}
}
