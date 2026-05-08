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

package managed

import (
	"context"
	"crypto/sha256"
	"fmt"

	kmeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	protectionv1beta1 "github.com/crossplane/crossplane/apis/v2/protection/v1beta1"
)

// FieldOwnerMRDProtection is the field manager name used when applying
// ClusterUsage objects for provider deletion protection.
const FieldOwnerMRDProtection = "apiextensions.crossplane.io/managed-protection"

// ProtectionControllerName returns the name of the protection controller
// for the given MRD name.
func ProtectionControllerName(mrdName string) string {
	return "protection/" + mrdName
}

// ClusterUsageName returns a deterministic name for a ClusterUsage
// protecting a Provider due to the given MRD. It uses a SHA-256 hash
// to avoid exceeding the Kubernetes 253-character name limit.
func ClusterUsageName(mrdName string) string {
	h := sha256.Sum256([]byte(mrdName))
	return fmt.Sprintf("provider-protection-%x", h[:26])
}

// ResourceMapFunc returns a handler.MapFunc that enqueues a fixed
// reconcile request for the protection controller whenever any managed
// resource of the watched type changes.
func ResourceMapFunc(mrdName string) handler.MapFunc {
	return func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{Name: mrdName},
		}}
	}
}

// A ProtectionReconciler manages a ClusterUsage that protects a Provider from
// deletion when managed resources of the given type exist.
type ProtectionReconciler struct {
	cached  client.Client // engine's cached client, for reading MR instances
	writer  client.Client // main client, for writing ClusterUsage objects
	mrdName string
	gvk     schema.GroupVersionKind
	log     logging.Logger
}

// Reconcile checks whether any managed resources of the watched type exist.
// If they do, it ensures a ClusterUsage exists to protect the owning Provider.
// If none exist, it deletes the ClusterUsage.
func (r *ProtectionReconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("gvk", r.gvk.String())
	log.Debug("Reconciling provider deletion protection")

	list := &kunstructured.UnstructuredList{}
	list.SetGroupVersionKind(r.gvk.GroupVersion().WithKind(r.gvk.Kind + "List"))

	if err := r.cached.List(ctx, list, client.Limit(1)); err != nil {
		// If the CRD doesn't exist (yet or anymore), treat as no MRs.
		if kmeta.IsNoMatchError(err) {
			log.Debug("CRD not found, ensuring no ClusterUsage")
			return r.ensureNoClusterUsage(ctx)
		}
		return reconcile.Result{}, errors.Wrap(err, "cannot list managed resources")
	}

	if len(list.Items) > 0 {
		return r.ensureClusterUsage(ctx)
	}

	return r.ensureNoClusterUsage(ctx)
}

func (r *ProtectionReconciler) ensureClusterUsage(ctx context.Context) (reconcile.Result, error) {
	mrd := &v1alpha1.ManagedResourceDefinition{}
	if err := r.cached.Get(ctx, types.NamespacedName{Name: r.mrdName}, mrd); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot get ManagedResourceDefinition for protection")
	}

	providerName, err := resolveProviderName(ctx, r.cached, mrd)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot resolve Provider name")
	}

	mrType := fmt.Sprintf("%s.%s", mrd.Spec.Names.Kind, mrd.Spec.Group)
	cu := buildClusterUsage(r.mrdName, providerName, mrType)

	//nolint:staticcheck // TODO(adamwg): Stop using client.Apply after the v2.2 release.
	if err := r.writer.Patch(ctx, cu, client.Apply,
		client.ForceOwnership,
		client.FieldOwner(FieldOwnerMRDProtection),
	); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot apply ClusterUsage")
	}

	r.log.Debug("ClusterUsage applied for provider protection", "provider", providerName, "mrType", mrType)
	return reconcile.Result{}, nil
}

func (r *ProtectionReconciler) ensureNoClusterUsage(ctx context.Context) (reconcile.Result, error) {
	cu := &protectionv1beta1.ClusterUsage{}
	cu.SetName(ClusterUsageName(r.mrdName))
	if err := r.writer.Delete(ctx, cu); resource.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot delete ClusterUsage")
	}
	r.log.Debug("ClusterUsage removed for provider protection")
	return reconcile.Result{}, nil
}

// resolveProviderName follows the owner chain from MRD -> ProviderRevision ->
// Provider to determine the name of the Provider that owns the given MRD.
func resolveProviderName(ctx context.Context, c client.Client, mrd *v1alpha1.ManagedResourceDefinition) (string, error) {
	owner := metav1.GetControllerOf(mrd)
	if owner == nil {
		return "", errors.New("MRD has no controller owner")
	}

	rev := &pkgv1.ProviderRevision{}
	if err := c.Get(ctx, types.NamespacedName{Name: owner.Name}, rev); err != nil {
		return "", errors.Wrap(err, "cannot get ProviderRevision")
	}

	providerName := rev.GetLabels()[pkgv1.LabelParentPackage]
	if providerName == "" {
		return "", errors.Errorf("ProviderRevision %q has no %s label", rev.GetName(), pkgv1.LabelParentPackage)
	}

	return providerName, nil
}

// buildClusterUsage constructs a ClusterUsage that protects a Provider from
// deletion due to the existence of managed resources of the given type.
func buildClusterUsage(mrdName, providerName, mrTypeDescription string) *protectionv1beta1.ClusterUsage {
	return &protectionv1beta1.ClusterUsage{
		TypeMeta: metav1.TypeMeta{
			APIVersion: protectionv1beta1.SchemeGroupVersion.String(),
			Kind:       protectionv1beta1.ClusterUsageKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: ClusterUsageName(mrdName),
			Labels: map[string]string{
				"crossplane.io/provider-protection": "true",
				pkgv1.LabelParentPackage:            providerName,
				"apiextensions.crossplane.io/mrd":   mrdName,
			},
		},
		Spec: protectionv1beta1.ClusterUsageSpec{
			Of: protectionv1beta1.Resource{
				APIVersion: pkgv1.SchemeGroupVersion.String(),
				Kind:       pkgv1.ProviderKind,
				ResourceRef: &protectionv1beta1.ResourceRef{
					Name: providerName,
				},
			},
			Reason: ptr.To(fmt.Sprintf(
				"Provider has active managed resources of type %s",
				mrTypeDescription,
			)),
		},
	}
}

// storageVersion returns the storage version name from the MRD.
func storageVersion(mrd *v1alpha1.ManagedResourceDefinition) string {
	for _, v := range mrd.Spec.Versions {
		if v.Storage {
			return v.Name
		}
	}
	// This should never be reached as the MRD API requires a storage version.
	return ""
}
