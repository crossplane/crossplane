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
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	"github.com/crossplane/crossplane/v2/internal/ssa"
)

// newMRDEnvtestClient starts an isolated API server for opt-in local regressions.
// CI does not currently provide KUBEBUILDER_ASSETS.
func newMRDEnvtestClient(t *testing.T) client.Client {
	t.Helper()
	assets := os.Getenv("KUBEBUILDER_ASSETS")
	if assets == "" {
		t.Skip("set KUBEBUILDER_ASSETS to run the API-server regression")
	}
	env := &envtest.Environment{
		UseExistingCluster: new(false), BinaryAssetsDirectory: assets,
		CRDDirectoryPaths: []string{"../../../../cluster/crds/apiextensions.crossplane.io_managedresourcedefinitions.yaml"}, ErrorIfCRDPathMissing: true,
	}
	cfg, err := env.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := env.Stop(); err != nil {
			t.Error(err)
		}
	})
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, extv1.AddToScheme, v1alpha1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestCRDPrunesFieldsAfterCreate needs real managedFields and CRD merge semantics.
func TestCRDPrunesFieldsAfterCreate(t *testing.T) {
	c := newMRDEnvtestClient(t)
	ctx := context.Background()
	oldSchema := &v1alpha1.CustomResourceValidation{OpenAPIV3Schema: runtime.RawExtension{Raw: []byte(`{"type":"object","properties":{"spec":{"type":"object","properties":{"keep":{"type":"string"},"obsolete":{"type":"string"}}}}}`)}}
	mrd := &v1alpha1.ManagedResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.org", OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "pkg.crossplane.io/v1", Kind: "ProviderRevision", Name: "provider-old", UID: "provider-old-uid", Controller: new(true),
		}}},
		Spec: v1alpha1.ManagedResourceDefinitionSpec{
			State: v1alpha1.ManagedResourceDefinitionActive,
			CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
				Group: "example.org", Scope: extv1.ClusterScoped,
				Names: extv1.CustomResourceDefinitionNames{Kind: "Widget", Plural: "widgets"},
				Versions: []v1alpha1.CustomResourceDefinitionVersion{
					{Name: "v1beta1", Served: true, Schema: oldSchema},
					{Name: "v1", Served: true, Storage: true, Schema: oldSchema, AdditionalPrinterColumns: []extv1.CustomResourceColumnDefinition{{Name: "Obsolete", Type: "string", JSONPath: ".spec.obsolete"}}},
				},
			},
		},
	}
	if err := c.Create(ctx, mrd); err != nil {
		t.Fatal(err)
	}
	r := &Reconciler{
		client: c, managedFields: ssa.NewPatchingManagedFieldsUpgrader(c, ssa.ExactMatch(FieldOwnerMRD)),
		log: logging.NewNopLogger(), record: event.NewNopRecorder(), conditions: conditions.ObservedGenerationPropagationManager{},
	}
	reconcileOnce := func() {
		t.Helper()
		if err := wait.PollUntilContextTimeout(ctx, 50*time.Millisecond, 10*time.Second, true, func(ctx context.Context) (bool, error) {
			result, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(mrd)})
			if kerrors.IsConflict(err) {
				return false, nil
			}
			return result == (reconcile.Result{}), err
		}); err != nil {
			t.Fatal(err)
		}
	}
	// Create, transfer fields to apply, and finish the managed-fields upgrade.
	for range 3 {
		reconcileOnce()
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(mrd), mrd); err != nil {
		t.Fatal(err)
	}
	mrd.OwnerReferences[0].Name = "provider-new"
	mrd.OwnerReferences[0].UID = "provider-new-uid"
	mrd.Spec.Versions = []v1alpha1.CustomResourceDefinitionVersion{{
		Name: "v1", Served: true, Storage: true,
		Schema: &v1alpha1.CustomResourceValidation{OpenAPIV3Schema: runtime.RawExtension{Raw: []byte(`{"type":"object","properties":{"spec":{"type":"object","properties":{"keep":{"type":"string"}}}}}`)}},
	}}
	if err := c.Update(ctx, mrd); err != nil {
		t.Fatal(err)
	}
	// More reconciles must not be needed to remove fields, but also must not
	// reintroduce fields still owned by a create-time Update manager.
	for range 3 {
		reconcileOnce()
	}
	crd := &extv1.CustomResourceDefinition{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(mrd), crd); err != nil {
		t.Fatal(err)
	}
	if len(crd.Spec.Versions) != 1 || crd.Spec.Versions[0].Name != "v1" {
		t.Errorf("obsolete version remains: %v", crd.Spec.Versions)
	}

	for _, version := range crd.Spec.Versions {
		if len(version.AdditionalPrinterColumns) != 0 {
			t.Errorf("obsolete printer columns remain in %s", version.Name)
		}
		if _, ok := version.Schema.OpenAPIV3Schema.Properties["spec"].Properties["obsolete"]; ok {
			t.Errorf("obsolete schema field remains in %s", version.Name)
		}
	}
	if controller := metav1.GetControllerOf(crd); controller == nil || controller.UID != "provider-new-uid" {
		t.Errorf("CRD control did not transfer to the new revision: %v", controller)
	}
	for _, manager := range crd.ManagedFields {
		if manager.Subresource == "" && manager.Operation != metav1.ManagedFieldsOperationApply {
			t.Errorf("create-time field manager remains: %s (%s) %s", manager.Manager, manager.Operation, manager.FieldsV1.GetRawString())
		}
	}
}
