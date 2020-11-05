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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ControllerConfigSpec specifies the configuration for a packaged controller.
// Values provided will override package manager defaults. Labels and
// annotations are passed to both the controller Deployment and ServiceAccount.
type ControllerConfigSpec struct {
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// Note: If more than 1 replica is set and leader election is not enabled then
	// controllers could conflict. Environment variable "LEADER_ELECTION" can be
	// used to enable leader election process.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// ServiceAccountName is the name of the ServiceAccount to use to run this pod.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
	// +optional
	ServiceAccountName *string `json:"serviceAccountName,omitempty"`
	// NodeName is a request to schedule this pod onto a specific node. If it is non-empty,
	// the scheduler simply schedules this pod onto that node, assuming that it fits resource
	// requirements.
	// +optional
	NodeName *string `json:"nodeName,omitempty"`
	// PodSecurityContext holds pod-level security attributes and common container settings.
	// Optional: Defaults to empty.  See type description for default values of each field.
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	// SecurityContext holds container-level security attributes and common container settings.
	// Optional: Defaults to empty.  See type description for default values of each field.
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use. For example,
	// in the case of docker, only DockerConfig type secrets are honored.
	// More info: https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod
	// Setting ImagePullSecrets will replace any secrets that have been
	// propagated to a controller Deployment, typically via packagePullSecrets.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// Image pull policy.
	// One of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +optional
	ImagePullPolicy *corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// If specified, the pod's scheduling constraints
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// If specified, the pod's tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// If specified, indicates the pod's priority. "system-node-critical" and
	// "system-cluster-critical" are two special keywords which indicate the
	// highest priorities with the former being the highest priority. Any other
	// name must be defined by creating a PriorityClass object with that name.
	// If not specified, the pod priority will be default or zero if there is no
	// default.
	// +optional
	PriorityClassName *string `json:"priorityClassName,omitempty"`
	// RuntimeClassName refers to a RuntimeClass object in the node.k8s.io group, which should be used
	// to run this pod.  If no RuntimeClass resource matches the named class, the pod will not be run.
	// If unset or empty, the "legacy" RuntimeClass will be used, which is an implicit class with an
	// empty definition that uses the default runtime handler.
	// More info: https://git.k8s.io/enhancements/keps/sig-node/runtime-class.md
	// This is a beta feature as of Kubernetes v1.14.
	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`
	// Compute Resources required by this container.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	// +optional
	ResourceRequirements *corev1.ResourceRequirements `json:"resources,omitempty"`
	// Arguments to the entrypoint.
	// The docker image's CMD is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. The $(VAR_NAME) syntax
	// can be escaped with a double $$, ie: $$(VAR_NAME). Escaped references will never be expanded,
	// regardless of whether the variable exists or not.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	Args []string `json:"args,omitempty"`
	// List of sources to populate environment variables in the container.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the container is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	// Cannot be updated.
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
	// List of environment variables to set in the container.
	// Cannot be updated.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// +kubebuilder:object:root=true
// +genclient
// +genclient:nonNamespaced

// ControllerConfig is the CRD type for a packaged controller configuration.
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster
type ControllerConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ControllerConfigSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ControllerConfigList contains a list of ControllerConfig.
type ControllerConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ControllerConfig `json:"items"`
}
