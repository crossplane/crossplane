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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
)

const (
	// LabelTransactionName records which Transaction last handled this Package.
	LabelTransactionName = "pkg.crossplane.io/transaction-name"

	// LabelTransactionGeneration records which Package generation was handled
	// by the Transaction named in LabelTransactionName.
	LabelTransactionGeneration = "pkg.crossplane.io/transaction-generation"
)

// Transaction represents a complete proposed state of the package system
// and validates the entire operation before making changes.
//
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="CHANGE",type="string",JSONPath=".spec.change"
// +kubebuilder:printcolumn:name="VALIDATED",type="string",JSONPath=".status.conditions[?(@.type=='Validated')].status"
// +kubebuilder:printcolumn:name="INSTALLED",type="string",JSONPath=".status.conditions[?(@.type=='Installed')].status"
// +kubebuilder:printcolumn:name="SUCCEEDED",type="string",JSONPath=".status.conditions[?(@.type=='Succeeded')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane},shortName=tx
type Transaction struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TransactionSpec   `json:"spec,omitempty"`
	Status TransactionStatus `json:"status,omitempty"`
}

// TransactionSpec defines the desired state of a Transaction.
// +kubebuilder:validation:XValidation:rule="self.change == 'Install' ? has(self.install) : true",message="install is required when change is Install"
// +kubebuilder:validation:XValidation:rule="self.change == 'Delete' ? has(self.delete) : true",message="delete is required when change is Delete"
// +kubebuilder:validation:XValidation:rule="self.change == 'Replace' ? has(self.replace) : true",message="replace is required when change is Replace"
type TransactionSpec struct {
	// Change represents the type of change to make to the package system.
	Change ChangeType `json:"change"`

	// Install specifies parameters for installing a package.
	// Required when Change is "Install".
	// +optional
	Install *InstallSpec `json:"install,omitempty"`

	// Delete specifies parameters for deleting a package.
	// Required when Change is "Delete".
	// +optional
	Delete *DeleteSpec `json:"delete,omitempty"`

	// Replace specifies parameters for replacing one package with another.
	// Required when Change is "Replace".
	// +optional
	Replace *ReplaceSpec `json:"replace,omitempty"`

	// RetryLimit configures how many times the transaction may fail. When the
	// failure limit is exceeded, the transaction will not be retried.
	// Follows the same pattern as Operation resources for consistency.
	// +optional
	// +kubebuilder:default=5
	RetryLimit *int64 `json:"retryLimit,omitempty"`
}

// InstallSpec specifies parameters for installing a package.
type InstallSpec struct {
	// Package is a complete snapshot of the Package resource configuration
	// at the time the Transaction was created. This ensures stable inputs
	// across retries and provides all information needed to create PackageRevisions.
	Package PackageSnapshot `json:"package"`
}

// DeleteSpec specifies parameters for deleting a package.
type DeleteSpec struct {
	// Source is the OCI repository of the package to delete.
	Source string `json:"source"`
}

// ReplaceSpec specifies parameters for replacing one package with another.
type ReplaceSpec struct {
	// Source is the OCI repository of the package to be replaced.
	Source string `json:"source"`

	// Package is a complete snapshot of the new Package resource configuration.
	// The Transaction controller uses this to create the replacement package
	// and resolve its dependency tree for equivalence validation.
	Package PackageSnapshot `json:"package"`
}

// PackageSnapshot contains a complete snapshot of Package resource configuration.
type PackageSnapshot struct {
	// APIVersion of the Package resource (e.g., "pkg.crossplane.io/v1")
	APIVersion string `json:"apiVersion"`

	// Kind of the Package resource (e.g., "Provider", "Configuration", "Function")
	Kind string `json:"kind"`

	// Metadata contains essential Package metadata (name, labels)
	// Excludes fields not needed for PackageRevision creation (generation, ownerRefs, etc.)
	Metadata PackageMetadata `json:"metadata"`

	// Spec is the complete Package.Spec configuration.
	// Includes PackageSpec fields for all package types, and PackageRuntimeSpec
	// fields for Provider and Function packages.
	Spec PackageSnapshotSpec `json:"spec"`
}

// TODO(negz): Define local v1alpha1 types for PackageSpec and PackageRuntimeSpec
// and use goverter to generate conversion functions from v1 types. This avoids
// importing types across API versions.

// PackageSnapshotSpec contains the complete Package.Spec configuration.
// It embeds both PackageSpec (common to all packages) and PackageRuntimeSpec
// (used by Provider and Function packages). Configuration packages will have
// empty PackageRuntimeSpec fields.
type PackageSnapshotSpec struct {
	v1.PackageSpec        `json:",inline"`
	v1.PackageRuntimeSpec `json:",inline"`
}

// PackageMetadata contains essential Package metadata.
type PackageMetadata struct {
	// Name of the Package resource
	Name string `json:"name"`

	// UID of the Package resource, required for creating owner references
	UID types.UID `json:"uid"`

	// Labels applied to the Package resource
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// ChangeType represents the type of change to make to the package system.
type ChangeType string

const (
	// ChangeTypeInstall installs the specified package version.
	// If the package source already exists, this updates it to the new version.
	ChangeTypeInstall ChangeType = "Install"

	// ChangeTypeDelete removes the specified package source from the system.
	ChangeTypeDelete ChangeType = "Delete"

	// ChangeTypeReplace replaces one package source with another.
	// Used for migrating between package ecosystems while preserving CRDs and managed resources.
	ChangeTypeReplace ChangeType = "Replace"
)

// TransactionStatus defines the observed state of a Transaction.
type TransactionStatus struct {
	xpv1.ConditionedStatus `json:",inline"`

	// Number of transaction failures. Incremented each time the transaction
	// fails and retries. When this reaches RetryLimit, the transaction
	// will not be retried again.
	Failures int64 `json:"failures,omitempty"`

	// TransactionNumber is a monotonically increasing number assigned when
	// the Transaction acquires the Lock. Used for ordering Transaction execution.
	TransactionNumber int64 `json:"transactionNumber,omitempty"`

	// ProposedLockPackages contains the complete proposed Lock state after resolving
	// the requested change and all dependencies. This represents what the Lock
	// will contain if the Transaction succeeds.
	ProposedLockPackages []v1beta1.LockPackage `json:"proposedLockPackages,omitempty"`
}

// GetCondition of this Transaction.
func (t *Transaction) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return t.Status.GetCondition(ct)
}

// SetConditions of this Transaction.
func (t *Transaction) SetConditions(c ...xpv1.Condition) {
	t.Status.SetConditions(c...)
}

// IsComplete returns if this transaction has finished running.
func (t *Transaction) IsComplete() bool {
	c := t.GetCondition(TypeSucceeded)
	// Normally, checking observedGeneration == generation is required, but Succeeded=True/False are terminal conditions.
	return c.Status == corev1.ConditionTrue || c.Status == corev1.ConditionFalse
}

// TransactionList contains a list of Transaction resources.
//
// +kubebuilder:object:root=true
type TransactionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Transaction `json:"items"`
}
