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

	appsv1 "k8s.io/api/apps/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

// DeploymentSelectorMigrator handles migration of provider deployments
// that have outdated selector labels from older Crossplane versions.
type DeploymentSelectorMigrator interface {
	MigrateDeploymentSelector(ctx context.Context, pr v1.PackageRevisionWithRuntime, b ManifestBuilder) error
}

// NopDeploymentSelectorMigrator is a no-op implementation of DeploymentSelectorMigrator.
type NopDeploymentSelectorMigrator struct{}

// NewNopDeploymentSelectorMigrator creates a new NopDeploymentSelectorMigrator.
func NewNopDeploymentSelectorMigrator() *NopDeploymentSelectorMigrator {
	return &NopDeploymentSelectorMigrator{}
}

// MigrateDeploymentSelector does nothing and always returns nil.
func (n *NopDeploymentSelectorMigrator) MigrateDeploymentSelector(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
	return nil
}

// DeletingDeploymentSelectorMigrator migrates provider deployment selectors
// by deleting deployments with outdated selectors, allowing new ones to be created.
type DeletingDeploymentSelectorMigrator struct {
	client client.Client
	log    logging.Logger
}

// NewDeletingDeploymentSelectorMigrator creates a new DeletingDeploymentSelectorMigrator.
func NewDeletingDeploymentSelectorMigrator(client client.Client, log logging.Logger) *DeletingDeploymentSelectorMigrator {
	return &DeletingDeploymentSelectorMigrator{
		client: client,
		log:    log,
	}
}

// MigrateDeploymentSelector prepares to migrate the deployment selector for a
// provider revision by deleting the existing deployment if it has an outdated
// selector.
// The pkg.crossplane.io/provider selector format changed in newer Crossplane versions:
// - Old format: used the provider name in the package metadata (e.g., "provider-nop")
// - New format: uses the parent package label value (e.g., "crossplane-provider-nop", or however the provider pkg resource is named)
// See https://github.com/crossplane/crossplane/blob/v1.20.0/internal/controller/pkg/revision/runtime_provider.go#L236-L244
// Note(turkenh): This migration could be removed in the future when we no longer
// support older Crossplane versions that use the old provider deployment
// selector labels. The latest Crossplane version using the old selector labels
// is v1.20.0.
func (m *DeletingDeploymentSelectorMigrator) MigrateDeploymentSelector(ctx context.Context, pr v1.PackageRevisionWithRuntime, builder ManifestBuilder) error {
	// Only migrate provider revisions
	providerRev, ok := pr.(*v1.ProviderRevision)
	if !ok {
		return nil
	}

	// Only perform migration for active revisions
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	// Build the expected deployment to get the correct name and selectors
	// This respects any DeploymentRuntimeConfig settings
	sa := builder.ServiceAccount()
	expectedDeploy := builder.Deployment(sa.Name)

	// Check if there's an existing deployment
	existingDeploy := &appsv1.Deployment{}

	err := m.client.Get(ctx, types.NamespacedName{
		Name:      expectedDeploy.Name,
		Namespace: expectedDeploy.Namespace,
	}, existingDeploy)
	if kerrors.IsNotFound(err) {
		// No existing deployment, no migration needed
		return nil
	}

	if err != nil {
		return errors.Wrap(err, "cannot get existing deployment")
	}

	existing := providerRev.GetLabels()[v1.LabelParentPackage]

	if existingDeploy.Spec.Selector == nil || existingDeploy.Spec.Selector.MatchLabels == nil {
		// No selector or match labels, no migration needed
		return nil
	}

	expected := existingDeploy.Spec.Selector.MatchLabels[v1.LabelProvider]

	// Check if the provider label needs migration
	if expected == existing {
		// No migration needed
		return nil
	}

	m.log.Info("Deleting provider deployment with outdated selector",
		"deployment", expectedDeploy.Name,
		"revision", pr.GetName(),
		"old-provider-label", expected,
		"new-provider-label", existing)

	// If the provider label is different, we need to delete the old deployment.
	// The new deployment will be created in the Post hook.
	if err := m.client.Delete(ctx, existingDeploy); err != nil {
		return errors.Wrap(err, "cannot delete existing deployment for selector migration")
	}

	return nil
}
