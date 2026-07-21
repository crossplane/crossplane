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

package definition

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

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	protectionv1beta1 "github.com/crossplane/crossplane/apis/v2/protection/v1beta1"
)

// FieldOwnerXRDProtection is the field manager name used when applying
// ClusterUsage objects for XRD and Configuration deletion protection.
const FieldOwnerXRDProtection = "apiextensions.crossplane.io/definition-protection"

// XRDProtectionControllerName returns the name of the protection controller
// for the given XRD name.
func XRDProtectionControllerName(xrdName string) string {
	return "xrd-protection/" + xrdName
}

// XRDClusterUsageName returns a deterministic name for a ClusterUsage
// protecting a CompositeResourceDefinition due to the given XRD. It uses a
// SHA-256 hash to avoid exceeding the Kubernetes 253-character name limit.
func XRDClusterUsageName(xrdName string) string {
	h := sha256.Sum256([]byte(xrdName))
	return fmt.Sprintf("xrd-protection-%x", h[:26])
}

// ConfigClusterUsageName returns a deterministic name for a ClusterUsage
// protecting a Configuration due to the given XRD. It uses a SHA-256 hash
// to avoid exceeding the Kubernetes 253-character name limit.
func ConfigClusterUsageName(xrdName string) string {
	h := sha256.Sum256([]byte(xrdName))
	return fmt.Sprintf("config-protection-%x", h[:26])
}

// XRDResourceMapFunc returns a handler.MapFunc that enqueues a fixed
// reconcile request for the protection controller whenever any composite
// resource of the watched type changes.
func XRDResourceMapFunc(xrdName string) handler.MapFunc {
	return func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{Name: xrdName},
		}}
	}
}

// An XRDProtectionReconciler manages ClusterUsage objects that protect a
// CompositeResourceDefinition (and optionally its owning Configuration) from
// deletion when composite resources of the given type exist.
type XRDProtectionReconciler struct {
	cached  client.Client // engine's cached client, for reading XR instances
	writer  client.Client // main client, for writing ClusterUsage objects
	xrdName string
	gvk     schema.GroupVersionKind
	log     logging.Logger
}

// Reconcile checks whether any composite resources of the watched type exist.
// If they do, it ensures ClusterUsage objects exist to protect the owning XRD
// (and its Configuration, if applicable). If none exist, it deletes the
// ClusterUsages.
func (r *XRDProtectionReconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("gvk", r.gvk.String())
	log.Debug("Reconciling XRD deletion protection")

	list := &kunstructured.UnstructuredList{}
	list.SetGroupVersionKind(r.gvk.GroupVersion().WithKind(r.gvk.Kind + "List"))

	if err := r.cached.List(ctx, list, client.Limit(1)); err != nil {
		// If the CRD doesn't exist (yet or anymore), treat as no XRs.
		if kmeta.IsNoMatchError(err) {
			log.Debug("CRD not found, ensuring no ClusterUsages")
			return r.ensureNoClusterUsages(ctx)
		}
		return reconcile.Result{}, errors.Wrap(err, "cannot list composite resources")
	}

	if len(list.Items) > 0 {
		return r.ensureClusterUsages(ctx)
	}

	return r.ensureNoClusterUsages(ctx)
}

func (r *XRDProtectionReconciler) ensureClusterUsages(ctx context.Context) (reconcile.Result, error) {
	xrType := fmt.Sprintf("%s.%s", r.gvk.Kind, r.gvk.Group)

	// Always apply a ClusterUsage protecting the XRD itself.
	cu := buildXRDClusterUsage(r.xrdName, xrType)
	//nolint:staticcheck // TODO(adamwg): Stop using client.Apply after the v2.2 release.
	if err := r.writer.Patch(ctx, cu, client.Apply,
		client.ForceOwnership,
		client.FieldOwner(FieldOwnerXRDProtection),
	); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot apply XRD ClusterUsage")
	}
	r.log.Debug("ClusterUsage applied for XRD protection", "xrd", r.xrdName, "xrType", xrType)

	// Try to resolve the owning Configuration. If the XRD is standalone
	// (no ConfigurationRevision owner), we skip the Configuration ClusterUsage.
	xrd := &v1.CompositeResourceDefinition{}
	if err := r.cached.Get(ctx, types.NamespacedName{Name: r.xrdName}, xrd); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot get CompositeResourceDefinition for protection")
	}

	configName, err := resolveConfigurationName(ctx, r.cached, xrd)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot resolve Configuration name")
	}

	if configName != "" {
		ccu := buildConfigClusterUsage(r.xrdName, configName, xrType)
		//nolint:staticcheck // TODO(adamwg): Stop using client.Apply after the v2.2 release.
		if err := r.writer.Patch(ctx, ccu, client.Apply,
			client.ForceOwnership,
			client.FieldOwner(FieldOwnerXRDProtection),
		); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "cannot apply Configuration ClusterUsage")
		}
		r.log.Debug("ClusterUsage applied for Configuration protection", "configuration", configName, "xrType", xrType)
	}

	return reconcile.Result{}, nil
}

func (r *XRDProtectionReconciler) ensureNoClusterUsages(ctx context.Context) (reconcile.Result, error) {
	// Delete both ClusterUsages (XRD + Configuration). If they don't exist,
	// that's fine.
	for _, name := range []string{XRDClusterUsageName(r.xrdName), ConfigClusterUsageName(r.xrdName)} {
		cu := &protectionv1beta1.ClusterUsage{}
		cu.SetName(name)
		if err := r.writer.Delete(ctx, cu); resource.IgnoreNotFound(err) != nil {
			return reconcile.Result{}, errors.Wrapf(err, "cannot delete ClusterUsage %q", name)
		}
	}
	r.log.Debug("ClusterUsages removed for XRD protection")
	return reconcile.Result{}, nil
}

// resolveConfigurationName follows the owner chain from XRD ->
// ConfigurationRevision -> Configuration to determine the name of the
// Configuration that owns the given XRD. It returns an empty string if the
// XRD is not owned by a ConfigurationRevision (i.e. it's a standalone XRD).
func resolveConfigurationName(ctx context.Context, c client.Client, xrd *v1.CompositeResourceDefinition) (string, error) {
	owner := metav1.GetControllerOf(xrd)
	if owner == nil {
		// Standalone XRD, not owned by any ConfigurationRevision.
		return "", nil
	}

	// Check if the owner is a ConfigurationRevision.
	if owner.Kind != pkgv1.ConfigurationRevisionKind {
		return "", nil
	}

	rev := &pkgv1.ConfigurationRevision{}
	if err := c.Get(ctx, types.NamespacedName{Name: owner.Name}, rev); err != nil {
		return "", errors.Wrap(err, "cannot get ConfigurationRevision")
	}

	configName := rev.GetLabels()[pkgv1.LabelParentPackage]
	if configName == "" {
		return "", errors.Errorf("ConfigurationRevision %q has no %s label", rev.GetName(), pkgv1.LabelParentPackage)
	}

	return configName, nil
}

// buildXRDClusterUsage constructs a ClusterUsage that protects a
// CompositeResourceDefinition from deletion due to the existence of composite
// resources of the given type.
func buildXRDClusterUsage(xrdName, xrTypeDescription string) *protectionv1beta1.ClusterUsage {
	return &protectionv1beta1.ClusterUsage{
		TypeMeta: metav1.TypeMeta{
			APIVersion: protectionv1beta1.SchemeGroupVersion.String(),
			Kind:       protectionv1beta1.ClusterUsageKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: XRDClusterUsageName(xrdName),
			Labels: map[string]string{
				"crossplane.io/xrd-protection":    "true",
				"apiextensions.crossplane.io/xrd": xrdName,
			},
		},
		Spec: protectionv1beta1.ClusterUsageSpec{
			Of: protectionv1beta1.Resource{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       v1.CompositeResourceDefinitionKind,
				ResourceRef: &protectionv1beta1.ResourceRef{
					Name: xrdName,
				},
			},
			Reason: ptr.To(fmt.Sprintf(
				"CompositeResourceDefinition has active composite resources of type %s",
				xrTypeDescription,
			)),
		},
	}
}

// buildConfigClusterUsage constructs a ClusterUsage that protects a
// Configuration from deletion due to the existence of composite resources
// of the given type.
func buildConfigClusterUsage(xrdName, configName, xrTypeDescription string) *protectionv1beta1.ClusterUsage {
	return &protectionv1beta1.ClusterUsage{
		TypeMeta: metav1.TypeMeta{
			APIVersion: protectionv1beta1.SchemeGroupVersion.String(),
			Kind:       protectionv1beta1.ClusterUsageKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: ConfigClusterUsageName(xrdName),
			Labels: map[string]string{
				"crossplane.io/configuration-protection": "true",
				pkgv1.LabelParentPackage:                 configName,
				"apiextensions.crossplane.io/xrd":        xrdName,
			},
		},
		Spec: protectionv1beta1.ClusterUsageSpec{
			Of: protectionv1beta1.Resource{
				APIVersion: pkgv1.SchemeGroupVersion.String(),
				Kind:       pkgv1.ConfigurationKind,
				ResourceRef: &protectionv1beta1.ResourceRef{
					Name: configName,
				},
			},
			Reason: ptr.To(fmt.Sprintf(
				"Configuration has active composite resources of type %s",
				xrTypeDescription,
			)),
		},
	}
}
