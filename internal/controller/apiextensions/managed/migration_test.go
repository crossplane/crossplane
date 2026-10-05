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

package managed

import (
	"context"
	"slices"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/controller/pkg/revision"
	"github.com/crossplane/crossplane/v2/internal/converter"
	"github.com/crossplane/crossplane/v2/internal/ssa"
)

func TestPackageCRDMigration(t *testing.T) {
	c := newMRDEnvtestClient(t)
	ctx := context.Background()
	ns := "crossplane-system"
	if err := c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: ns}, Data: map[string][]byte{"tls.crt": []byte("test-ca")}}); err != nil {
		t.Fatal(err)
	}
	provider := metav1.OwnerReference{APIVersion: "pkg.crossplane.io/v1", Kind: "Provider", Name: "provider", UID: "provider-uid", Controller: new(true)}
	old := &pkgv1.ProviderRevision{
		TypeMeta:   metav1.TypeMeta{APIVersion: "pkg.crossplane.io/v1", Kind: "ProviderRevision"},
		ObjectMeta: metav1.ObjectMeta{Name: "revision-old", UID: "revision-old-uid", Labels: map[string]string{pkgv1.LabelParentPackage: provider.Name}, OwnerReferences: []metav1.OwnerReference{provider}},
		Status:     pkgv1.ProviderRevisionStatus{PackageRevisionRuntimeStatus: pkgv1.PackageRevisionRuntimeStatus{TLSServerSecretName: new("tls")}},
	}
	content := &extv1.CustomResourceDefinition{
		TypeMeta:   metav1.TypeMeta{APIVersion: extv1.SchemeGroupVersion.String(), Kind: "CustomResourceDefinition"},
		ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.org"},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: "example.org", Scope: extv1.ClusterScoped, Names: extv1.CustomResourceDefinitionNames{Plural: "widgets", Kind: "Widget"},
			Versions: []extv1.CustomResourceDefinitionVersion{{Name: "v1", Served: true, Storage: true, Schema: &extv1.CustomResourceValidation{OpenAPIV3Schema: &extv1.JSONSchemaProps{Type: "object", Properties: map[string]extv1.JSONSchemaProps{"obsolete": {Type: "string"}}}}}},
		},
	}
	e := revision.NewAPIEstablisher(c, ns, func() pkgv1.PackageRevision { return &pkgv1.ProviderRevision{} }, 1)
	// Install the CRD as older Crossplane versions did, before conversion.
	if _, err := e.Establish(ctx, []runtime.Object{content.DeepCopy()}, old, true); err != nil {
		t.Fatal(err)
	}
	crd := &extv1.CustomResourceDefinition{}
	readCRD := func() {
		t.Helper()
		if err := c.Get(ctx, client.ObjectKey{Name: content.Name}, crd); err != nil {
			t.Fatal(err)
		}
	}
	readCRD()
	originalUID := crd.UID
	// A converted package revision changes the schema and creates its MRD.
	content.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties = map[string]extv1.JSONSchemaProps{"current": {Type: "string"}}
	establish := func(pr *pkgv1.ProviderRevision) {
		t.Helper()
		if err := wait.PollUntilContextTimeout(ctx, 50*time.Millisecond, 10*time.Second, true, func(ctx context.Context) (bool, error) {
			objects, err := converter.CustomToManagedResourceDefinitions(true, content.DeepCopy())
			if err != nil {
				return false, err
			}
			refs, err := e.Establish(ctx, objects, pr, true)
			if kerrors.IsConflict(err) {
				return false, nil
			}
			pr.SetObjects(refs)
			return err == nil, err
		}); err != nil {
			t.Fatal(err)
		}
	}
	r := &Reconciler{client: c, managedFields: ssa.NewPatchingManagedFieldsUpgrader(c, ssa.ExactMatch(FieldOwnerMRD)), log: logging.NewNopLogger(), record: event.NewNopRecorder(), conditions: conditions.ObservedGenerationPropagationManager{}}
	apply := func() {
		t.Helper()
		for range 3 {
			if err := wait.PollUntilContextTimeout(ctx, 50*time.Millisecond, 10*time.Second, true, func(ctx context.Context) (bool, error) {
				result, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: content.Name}})
				return result == (reconcile.Result{}), err
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	establish(old)
	apply()
	readCRD()
	if crd.UID != originalUID {
		t.Fatal("migration replaced the existing CRD")
	}
	if _, ok := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["current"]; !ok {
		t.Fatal("converted MRD did not update the legacy CRD schema")
	}
	mrd := &v1alpha1.ManagedResourceDefinition{}
	if err := c.Get(ctx, client.ObjectKey{Name: content.Name}, mrd); err != nil {
		t.Fatal(err)
	}
	originalMRDUID := mrd.UID
	if !mayApplyTo(mrd, crd) {
		t.Fatal("establisher did not establish strict MRD ownership")
	}

	// R2 must acquire direct grants even before the MRD controller hands off control.
	if err := e.ReleaseObjects(ctx, old); err != nil {
		t.Fatal(err)
	}
	next := old.DeepCopy()
	next.Name = "revision-new"
	next.UID = "revision-new-uid"
	establish(next)
	readCRD()
	if !slices.ContainsFunc(crd.OwnerReferences, func(ref metav1.OwnerReference) bool { return ref.UID == next.UID }) {
		t.Fatal("new revision has no direct CRD ownership for RBAC")
	}
	apply()
	readCRD()
	if !metav1.IsControlledBy(crd, next) {
		t.Fatalf("CRD did not hand off to R2: %v", crd.OwnerReferences)
	}
	if slices.ContainsFunc(crd.OwnerReferences, func(ref metav1.OwnerReference) bool { return ref.UID == old.UID }) {
		t.Fatal("old revision owner was not pruned")
	}

	// Simulate GC removing the deleted MRD's reference while the Provider
	// reference keeps the CRD alive. Establishment must link the recreated MRD.
	if err := c.Delete(ctx, mrd); err != nil {
		t.Fatal(err)
	}
	crd.OwnerReferences = slices.DeleteFunc(crd.OwnerReferences, func(ref metav1.OwnerReference) bool { return ref.UID == originalMRDUID })
	if err := c.Update(ctx, crd); err != nil {
		t.Fatal(err)
	}
	establish(next)
	apply()
	readCRD()
	if err := c.Get(ctx, client.ObjectKey{Name: content.Name}, mrd); err != nil {
		t.Fatal(err)
	}
	if mrd.UID == originalMRDUID || crd.UID != originalUID || !mayApplyTo(mrd, crd) {
		t.Fatal("recreated MRD did not acquire the surviving CRD")
	}
	if !metav1.IsControlledBy(crd, next) {
		t.Fatal("MRD recreation lost the current revision's control")
	}
	if !slices.ContainsFunc(crd.OwnerReferences, func(ref metav1.OwnerReference) bool { return ref.UID == provider.UID }) {
		t.Fatal("migration lost the stable Provider owner")
	}
	if !meta.WasCreated(mrd) {
		t.Fatal("MRD has no API-server identity")
	}
}
