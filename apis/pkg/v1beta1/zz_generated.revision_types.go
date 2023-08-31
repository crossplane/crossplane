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

// Generated from pkg/v1/revision_types.go by ../hack/duplicate_api_type.sh. DO NOT EDIT.

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// PackageRevisionDesiredState is the desired state of the package revision.
type PackageRevisionDesiredState string

const (
	// PackageRevisionActive is an active package revision.
	PackageRevisionActive PackageRevisionDesiredState = "Active"

	// PackageRevisionInactive is an inactive package revision.
	PackageRevisionInactive PackageRevisionDesiredState = "Inactive"
)

// PackageRevisionSpec specifies the desired state of a PackageRevision.
type PackageRevisionSpec struct {
	// ControllerConfigRef references a ControllerConfig resource that will be
	// used to configure the packaged controller Deployment.
	// +optional
	ControllerConfigReference *ControllerConfigReference `json:"controllerConfigRef,omitempty"`

	// DesiredState of the PackageRevision. Can be either Active or Inactive.
	DesiredState PackageRevisionDesiredState `json:"desiredState"`

	// Package image used by install Pod to extract package contents.
	Package string `json:"image"`

	// PackagePullSecrets are named secrets in the same namespace that can be
	// used to fetch packages from private registries. They are also applied to
	// any images pulled for the package, such as a provider's controller image.
	// +optional
	PackagePullSecrets []corev1.LocalObjectReference `json:"packagePullSecrets,omitempty"`

	// PackagePullPolicy defines the pull policy for the package. It is also
	// applied to any images pulled for the package, such as a provider's
	// controller image.
	// Default is IfNotPresent.
	// +optional
	// +kubebuilder:default=IfNotPresent
	PackagePullPolicy *corev1.PullPolicy `json:"packagePullPolicy,omitempty"`

	// Revision number. Indicates when the revision will be garbage collected
	// based on the parent's RevisionHistoryLimit.
	Revision int64 `json:"revision"`

	// IgnoreCrossplaneConstraints indicates to the package manager whether to
	// honor Crossplane version constrains specified by the package.
	// Default is false.
	// +optional
	// +kubebuilder:default=false
	IgnoreCrossplaneConstraints *bool `json:"ignoreCrossplaneConstraints,omitempty"`

	// SkipDependencyResolution indicates to the package manager whether to skip
	// resolving dependencies for a package. Setting this value to true may have
	// unintended consequences.
	// Default is false.
	// +optional
	// +kubebuilder:default=false
	SkipDependencyResolution *bool `json:"skipDependencyResolution,omitempty"`

	// WebhookTLSSecretName is the name of the TLS Secret that will be used
	// by the provider to serve a TLS-enabled webhook server. The certificate
	// will be injected to webhook configurations as well as CRD conversion
	// webhook strategy if needed.
	// If it's not given, provider will not have a certificate mounted to its
	// filesystem, webhook configurations won't be deployed and if there is a
	// CRD with webhook conversion strategy, the installation will fail.
	// +optional
	WebhookTLSSecretName *string `json:"webhookTLSSecretName,omitempty"`

	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	CommonLabels map[string]string `json:"commonLabels,omitempty"`

	// ESSTLSSecretName is the secret name of the TLS certificates that will be used
	// by the provider for External Secret Stores.
	// +optional
	ESSTLSSecretName *string `json:"essTLSSecretName,omitempty"`

	// TLSServerSecretName is the name of the TLS Secret that stores server
	// certificates of the Provider.
	// +optional
	TLSServerSecretName *string `json:"tlsServerSecretName,omitempty"`

	// TLSClientSecretName is the name of the TLS Secret that stores client
	// certificates of the Provider.
	// +optional
	TLSClientSecretName *string `json:"tlsClientSecretName,omitempty"`
}

// PackageRevisionStatus represents the observed state of a PackageRevision.
type PackageRevisionStatus struct {
	xpv1.ConditionedStatus `json:",inline"`

	// ControllerRef references the controller (e.g. Deployment), if any, that
	// is responsible for reconciling the objects this package revision
	// installed.
	ControllerRef ControllerReference `json:"controllerRef,omitempty"`

	// References to objects owned by PackageRevision.
	ObjectRefs []xpv1.TypedReference `json:"objectRefs,omitempty"`

	// Dependency information.
	FoundDependencies     int64 `json:"foundDependencies,omitempty"`
	InstalledDependencies int64 `json:"installedDependencies,omitempty"`
	InvalidDependencies   int64 `json:"invalidDependencies,omitempty"`

	// PermissionRequests made by this package. The package declares that its
	// controller needs these permissions to run. The RBAC manager is
	// responsible for granting them.
	PermissionRequests []rbacv1.PolicyRule `json:"permissionRequests,omitempty"`
}

// A ControllerReference references the controller (e.g. Deployment), if any,
// that is responsible for reconciling the types a package revision installs.
type ControllerReference struct {
	// Name of the controller.
	Name string `json:"name"`
}

// A ControllerConfigReference to a ControllerConfig resource that will be used
// to configure the packaged controller Deployment.
type ControllerConfigReference struct {
	// Name of the ControllerConfig.
	Name string `json:"name"`
}
