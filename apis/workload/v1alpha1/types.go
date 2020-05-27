/*
Copyright 2019 The Crossplane Authors.

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
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// KubernetesApplicationState represents the state of a Kubernetes application.
type KubernetesApplicationState string

// KubernetesApplication states.
const (
	KubernetesApplicationStateUnknown   KubernetesApplicationState = ""
	KubernetesApplicationStatePending   KubernetesApplicationState = "Pending"
	KubernetesApplicationStateScheduled KubernetesApplicationState = "Scheduled"
	KubernetesApplicationStatePartial   KubernetesApplicationState = "PartiallySubmitted"
	KubernetesApplicationStateSubmitted KubernetesApplicationState = "Submitted"
	KubernetesApplicationStateFailed    KubernetesApplicationState = "Failed"
	KubernetesApplicationStateDeleting  KubernetesApplicationState = "Deleting"
)

// A KubernetesApplicationSpec specifies the resources of a Kubernetes
// application.
type KubernetesApplicationSpec struct {
	// TODO(negz): Ensure the below selectors cannot be updated - only set at
	// creation time per https://github.com/crossplane/crossplane/issues/727

	// TODO(negz): Ensure ResourceSelector matches the labels of all templated
	// KubernetesApplicationResources.

	// ResourceSelector selects the KubernetesApplicationResources that are
	// managed by this KubernetesApplication. Note that a KubernetesApplication
	// will never adopt orphaned KubernetesApplicationResources, and thus this
	// selector serves only to help match a KubernetesApplication to its
	// KubernetesApplicationResources.
	ResourceSelector *metav1.LabelSelector `json:"resourceSelector"`

	// TODO(muvaf): Only MatchLabels field of LabelSelector is used. Incorporate
	// MatchExpressions as well when it's available as controller-runtime ListOption

	// TargetSelector selects the targets to which this application may be
	// scheduled. Leave both match labels and expressions empty to match any
	// target.
	// +optional
	TargetSelector *metav1.LabelSelector `json:"targetSelector,omitempty"`

	// Target to which this application has been scheduled.
	// +optional
	Target *KubernetesTargetReference `json:"targetRef,omitempty"`

	// TODO(negz): Use a validation webhook to ensure the below templates have
	// unique names.

	// ResourceTemplates specifies a set of Kubernetes application resources
	// managed by this application.
	ResourceTemplates []KubernetesApplicationResourceTemplate `json:"resourceTemplates"`
}

// A KubernetesApplicationResourceTemplate is used to instantiate new
// KubernetesApplicationResources.
type KubernetesApplicationResourceTemplate struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec KubernetesApplicationResourceSpec `json:"spec,omitempty"`
}

// A KubernetesTargetReference is a reference to a KubernetesTarget resource
// claim in the same namespace as the referrer.
type KubernetesTargetReference struct {
	// Name of the referent. More info:
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	Name string `json:"name"`
}

// KubernetesApplicationStatus represents the observed state of a
// KubernetesApplication.
type KubernetesApplicationStatus struct {
	runtimev1alpha1.ConditionedStatus `json:"conditionedStatus,omitempty"`

	// State of the application.
	State KubernetesApplicationState `json:"state,omitempty"`

	// Desired resources of this application, i.e. the number of resources
	// that match this application's resource selector.
	DesiredResources int `json:"desiredResources,omitempty"`

	// Submitted resources of this workload, i.e. the subset of desired
	// resources that have been successfully submitted to their scheduled
	// Kubernetes cluster.
	SubmittedResources int `json:"submittedResources,omitempty"`
}

// +kubebuilder:object:root=true

// A KubernetesApplication defines an application deployed by Crossplane to a
// Kubernetes cluster, i.e. a portable KubernetesCluster resource claim.
// +kubebuilder:resource:categories=crossplane
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="CLUSTER",type="string",JSONPath=".spec.targetRef.name"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="DESIRED",type="integer",JSONPath=".status.desiredResources"
// +kubebuilder:printcolumn:name="SUBMITTED",type="integer",JSONPath=".status.submittedResources"
type KubernetesApplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesApplicationSpec   `json:"spec,omitempty"`
	Status KubernetesApplicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KubernetesApplicationList contains a list of KubernetesApplication.
type KubernetesApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesApplication `json:"items"`
}

// KubernetesApplicationResourceState represents the state of a
// KubernetesApplicationResource.
type KubernetesApplicationResourceState string

// KubernetesApplicationResource states.
const (
	KubernetesApplicationResourceStateUnknown   KubernetesApplicationResourceState = ""
	KubernetesApplicationResourceStatePending   KubernetesApplicationResourceState = "Pending"
	KubernetesApplicationResourceStateScheduled KubernetesApplicationResourceState = "Scheduled"
	KubernetesApplicationResourceStateSubmitted KubernetesApplicationResourceState = "Submitted"
	KubernetesApplicationResourceStateFailed    KubernetesApplicationResourceState = "Failed"
)

// KubernetesApplicationResourceSpec specifies the desired state of a
// KubernetesApplicationResource.
type KubernetesApplicationResourceSpec struct {
	// TODO(negz): Use a validation webhook to reject updates to the template's
	// group, version, kind, namespace, and name. Changing any of these fields
	// would cause our controller to orphan any existing resource and create a
	// new one.

	// A Template for a Kubernetes resource to be submitted to the
	// KubernetesCluster to which this application resource is scheduled. The
	// resource must be understood by the KubernetesCluster. Crossplane requires
	// only that the resource contains standard Kubernetes type and object
	// metadata.
	Template runtime.RawExtension `json:"template"`

	// Target to which this application has been scheduled.
	// +optional
	Target *KubernetesTargetReference `json:"targetRef,omitempty"`

	// Secrets upon which this application resource depends. These secrets will
	// be propagated to the Kubernetes cluster to which this application is
	// scheduled.
	Secrets []corev1.LocalObjectReference `json:"secrets,omitempty"`
}

// NOTE(negz): This content of a RemoteStatus is opaque to Crossplane. We wrap
// json.RawMessage in this type in order to trick controller-tools into
// generating an OpenAPI spec that expects RemoteStatus to be a JSON object
// rather than a byte array. It is not currently possible to override
// controller-tools' type detection per
// https://github.com/kubernetes-sigs/controller-tools/issues/155

// RemoteStatus represents the observed state of a remote cluster.
type RemoteStatus struct {
	// Raw JSON representation of the remote status as a byte array.
	Raw json.RawMessage `json:"raw,omitempty"`
}

// MarshalJSON returns the JSON encoding of the RemoteStatus.
func (s RemoteStatus) MarshalJSON() ([]byte, error) {
	return s.Raw.MarshalJSON()
}

// UnmarshalJSON sets the RemoteStatus to a copy of data.
func (s *RemoteStatus) UnmarshalJSON(data []byte) error {
	return s.Raw.UnmarshalJSON(data)
}

// KubernetesApplicationResourceStatus represents the observed state of a
// KubernetesApplicationResource.
type KubernetesApplicationResourceStatus struct {
	runtimev1alpha1.ConditionedStatus `json:"conditionedStatus,omitempty"`

	// State of the application.
	State KubernetesApplicationResourceState `json:"state,omitempty"`

	// Remote status of the resource templated by this application resource.
	Remote *RemoteStatus `json:"remote,omitempty"`
}

// +kubebuilder:object:root=true

// A KubernetesApplicationResource is a resource of a Kubernetes application.
// Each resource templates a single Kubernetes resource to be deployed to its
// scheduled KubernetesCluster.
// +kubebuilder:resource:categories=crossplane
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="TEMPLATE-KIND",type="string",JSONPath=".spec.template.kind"
// +kubebuilder:printcolumn:name="TEMPLATE-NAME",type="string",JSONPath=".spec.template.metadata.name"
// +kubebuilder:printcolumn:name="CLUSTER",type="string",JSONPath=".spec.targetRef.name"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.state"
type KubernetesApplicationResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesApplicationResourceSpec   `json:"spec,omitempty"`
	Status KubernetesApplicationResourceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KubernetesApplicationResourceList contains a list of
// KubernetesApplicationResource.
type KubernetesApplicationResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesApplicationResource `json:"items"`
}

// +kubebuilder:object:root=true

// A KubernetesTarget is a scheduling target for a Kubernetes Application.
// +kubebuilder:resource:categories=crossplane
// +kubebuilder:printcolumn:name="CLUSTER",type="string",JSONPath=".spec.clusterRef.name"
type KubernetesTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   runtimev1alpha1.TargetSpec   `json:"spec"`
	Status runtimev1alpha1.TargetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KubernetesTargetList contains a list of KubernetesTarget.
type KubernetesTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesTarget `json:"items"`
}
