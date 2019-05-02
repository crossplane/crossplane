/*
Copyright 2018 The Crossplane Authors.

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
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
)

// A KubernetesApplicationSpec specifies the resources of a Kubernetes
// application.
type KubernetesApplicationSpec struct {
	// TODO(negz): Use a validation webhook to ensure the below selectors cannot
	// be updated - only set at creation time.

	// TODO(negz): Use a validation webhook to ensure ResourceSelector matches
	// the labels of all templated KubernetesApplicationResources.

	// ResourceSelector selects the KubernetesApplicationResources that are
	// managed by this KubernetesApplication. Note that a KubernetesApplication
	// will never adopt orphaned KubernetesApplicationResources, and thus this
	// selector serves only to help match a KubernetesApplication to its
	// KubernetesApplicationResources.
	ResourceSelector *metav1.LabelSelector `json:"resourceSelector"`

	// ClusterSelector selects the clusters to which this application may be
	// scheduled.
	ClusterSelector *metav1.LabelSelector `json:"clusterSelector,omitempty"`

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

// KubernetesApplicationStatus represents the status of a Kubernetes
// application.
type KubernetesApplicationStatus struct {
	corev1alpha1.ConditionedStatus

	// State of the application.
	State KubernetesApplicationState `json:"state,omitempty"`

	// Cluster to which this application has been scheduled.
	Cluster *corev1.ObjectReference `json:"clusterRef,omitempty"`

	// Desired resources of this application, i.e. the number of resources
	// that match this application's resource selector.
	DesiredResources int `json:"desiredResources,omitempty"`

	// Submitted resources of this workload, i.e. the subset of desired
	// resources that have been successfully submitted to their scheduled
	// Kubernetes cluster.
	SubmittedResources int `json:"submittedResources,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// A KubernetesApplication defines an application deployed by Crossplane to a
// Kubernetes cluster that is managed by Crossplane.
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="CLUSTER",type="string",JSONPath=".status.clusterRef.name"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="DESIRED",type="integer",JSONPath=".status.desiredResources"
// +kubebuilder:printcolumn:name="SUBMITTED",type="integer",JSONPath=".status.submittedResources"
type KubernetesApplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesApplicationSpec   `json:"spec,omitempty"`
	Status KubernetesApplicationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubernetesApplicationList contains a list of KubernetesApplications.
type KubernetesApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesApplication `json:"items"`
}

// KubernetesApplicationResourceState represents the state of a Kubernetes application.
type KubernetesApplicationResourceState string

// KubernetesApplicationResource states.
const (
	KubernetesApplicationResourceStateUnknown   KubernetesApplicationResourceState = ""
	KubernetesApplicationResourceStatePending   KubernetesApplicationResourceState = "Pending"
	KubernetesApplicationResourceStateScheduled KubernetesApplicationResourceState = "Scheduled"
	KubernetesApplicationResourceStateSubmitted KubernetesApplicationResourceState = "Submitted"
	KubernetesApplicationResourceStateFailed    KubernetesApplicationResourceState = "Failed"
)

// KubernetesApplicationResourceSpec specifies the configuration of a
// Kubernetes application resource.
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
	Template *unstructured.Unstructured `json:"template"`

	// Secrets upon which this application resource depends. These secrets will
	// be propagated to the Kubernetes cluster to which this application is
	// scheduled.
	Secrets []corev1.LocalObjectReference `json:"secrets,omitempty"`
}

// RemoteStatus represents the status of a resource in a remote Kubernetes
// cluster. Its content is opaque to Crossplane. We wrap json.RawMessage in this
// type in order to trick controller-tools into generating an OpenAPI spec that
// expects RemoteStatus to be a JSON object rather than a byte array. It is not
// currently possible to override controller-tools' type detection per
// https://github.com/kubernetes-sigs/controller-tools/issues/155
type RemoteStatus struct {
	// Raw JSON representation of the remote status as a byte array.
	Raw json.RawMessage
}

// MarshalJSON returns the JSON encoding of the RemoteStatus.
func (s RemoteStatus) MarshalJSON() ([]byte, error) {
	return s.Raw.MarshalJSON()
}

// UnmarshalJSON sets the RemoteStatus to a copy of data.
func (s *RemoteStatus) UnmarshalJSON(data []byte) error {
	return s.Raw.UnmarshalJSON(data)
}

// KubernetesApplicationResourceStatus represents the status of a Kubernetes
// application resource.
type KubernetesApplicationResourceStatus struct {
	corev1alpha1.ConditionedStatus

	// State of the application.
	State KubernetesApplicationResourceState `json:"state,omitempty"`

	// Cluster to which this application has been scheduled.
	Cluster *corev1.ObjectReference `json:"clusterRef,omitempty"`

	// Remote status of the resource templated by this application resource.
	Remote *RemoteStatus `json:"remote,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// A KubernetesApplicationResource is a resource of a Kubernetes application.
// Each resource templates a single Kubernetes resource to be deployed to its
// scheduled KubernetesCluster.
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="TEMPLATE-KIND",type="string",JSONPath=".spec.template.kind"
// +kubebuilder:printcolumn:name="TEMPLATE-NAME",type="string",JSONPath=".spec.template.metadata.name"
// +kubebuilder:printcolumn:name="CLUSTER",type="string",JSONPath=".status.clusterRef.name"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.state"
type KubernetesApplicationResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesApplicationResourceSpec   `json:"spec,omitempty"`
	Status KubernetesApplicationResourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubernetesApplicationResourceList contains a list of
// KubernetesApplicationResources.
type KubernetesApplicationResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesApplicationResource `json:"items"`
}
