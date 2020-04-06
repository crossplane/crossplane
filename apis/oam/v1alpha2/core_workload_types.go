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

package v1alpha2

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// An OperatingSystem required by a containerised workload.
type OperatingSystem string

// Supported operating system types.
const (
	OperatingSystemLinux   OperatingSystem = "linux"
	OperatingSystemWindows OperatingSystem = "windows"
)

// A CPUArchitecture required by a containerised workload.
type CPUArchitecture string

// Supported architectures
const (
	CPUArchitectureI386  CPUArchitecture = "i386"
	CPUArchitectureAMD64 CPUArchitecture = "amd64"
	CPUArchitectureARM   CPUArchitecture = "arm"
	CPUArchitectureARM64 CPUArchitecture = "arm64"
)

// A SecretKeySelector is a reference to a secret key in an arbitrary namespace.
type SecretKeySelector struct {
	// The name of the secret.
	Name string `json:"name"`

	// The key to select.
	Key string `json:"key"`
}

// TODO(negz): The OAM spec calls for float64 quantities in some cases, but this
// is incompatible with controller-gen and Kubernetes API conventions. We should
// reassess whether resource.Quantity is appropriate after resolving
// https://github.com/oam-dev/spec/issues/313

// CPUResources required by a container.
type CPUResources struct {
	// Required CPU count. 1.0 represents one CPU core.
	Required resource.Quantity `json:"required"`
}

// MemoryResources required by a container.
type MemoryResources struct {
	// Required memory.
	Required resource.Quantity `json:"required"`
}

// GPUResources required by a container.
type GPUResources struct {
	// Required GPU count.
	Required resource.Quantity `json:"required"`
}

// DiskResource required by a container.
type DiskResource struct {
	// Required disk space.
	Required resource.Quantity `json:"required"`

	// Ephemeral specifies whether an external disk needs to be mounted.
	// +optional
	Ephemeral *bool `json:"ephemeral,omitempty"`
}

// A VolumeAccessMode determines how a volume may be accessed.
type VolumeAccessMode string

// Volume access modes.
const (
	VolumeAccessModeRO VolumeAccessMode = "RO"
	VolumeAccessModeRW VolumeAccessMode = "RW"
)

// A VolumeSharingPolicy determines how a volume may be shared.
type VolumeSharingPolicy string

// Volume sharing policies.
const (
	VolumeSharingPolicyExclusive VolumeSharingPolicy = "Exclusive"
	VolumeSharingPolicyShared    VolumeSharingPolicy = "Shared"
)

// VolumeResource required by a container.
type VolumeResource struct {
	// Name of this volume. Must be unique within its container.
	Name string `json:"name"`

	// MouthPath at which this volume will be mounted within its container.
	MouthPath string `json:"mountPath"`

	// TODO(negz): Use +kubebuilder:default marker to default AccessMode to RW
	// and SharingPolicy to Exclusive once we're generating v1 CRDs.

	// AccessMode of this volume; RO (read only) or RW (read and write).
	// +optional
	// +kubebuilder:validation:Enum=RO;RW
	AccessMode *VolumeAccessMode `json:"accessMode,omitempty"`

	// SharingPolicy of this volume; Exclusive or Shared.
	// +optional
	// +kubebuilder:validation:Enum=Exclusive;Shared
	SharingPolicy *VolumeSharingPolicy `json:"sharingPolicy,omitempty"`

	// Disk requirements of this volume.
	// +optional
	Disk *DiskResource `json:"disk,omitempty"`
}

// ExtendedResource required by a container.
type ExtendedResource struct {
	// Name of the external resource. Resource names are specified in
	// kind.group/version format, e.g. motionsensor.ext.example.com/v1.
	Name string `json:"name"`

	// Required extended resource(s), e.g. 8 or "very-cool-widget"
	Required intstr.IntOrString `json:"required"`
}

// ContainerResources specifies a container's required compute resources.
type ContainerResources struct {
	// CPU required by this container.
	CPU CPUResources `json:"cpu"`

	// Memory required by this container.
	Memory MemoryResources `json:"memory"`

	// GPU required by this container.
	// +optional
	GPU *GPUResources `json:"gpu,omitempty"`

	// Volumes required by this container.
	// +optional
	Volumes []VolumeResource `json:"volumes,omitempty"`

	// Extended resources required by this container.
	// +optional
	Extended []ExtendedResource `json:"extended,omitempty"`
}

// A ContainerEnvVar specifies an environment variable that should be set within
// a container.
type ContainerEnvVar struct {
	// Name of the environment variable. Must be composed of valid Unicode
	// letter and number characters, as well as _ and -.
	// +kubebuilder:validation:Pattern=^[-_a-zA-Z0-9]+$
	Name string `json:"name"`

	// Value of the environment variable.
	// +optional
	Value *string `json:"value,omitempty"`

	// FromSecret is a secret key reference which can be used to assign a value
	// to the environment variable.
	// +optional
	FromSecret *SecretKeySelector `json:"fromSecret,omitempty"`
}

// A ContainerConfigFile specifies a configuration file that should be written
// within a container.
type ContainerConfigFile struct {
	// Path within the container at which the configuration file should be
	// written.
	Path string `json:"path"`

	// Value that should be written to the configuration file.
	// +optional
	Value *string `json:"value,omitempty"`

	// FromSecret is a secret key reference which can be used to assign a value
	// to be written to the configuration file at the given path in the
	// container.
	// +optional
	FromSecret *SecretKeySelector `json:"fromSecret,omitempty"`
}

// A TransportProtocol represents a transport layer protocol.
type TransportProtocol string

// Transport protocols.
const (
	TransportProtocolTCP TransportProtocol = "TCP"
	TransportProtocolUDP TransportProtocol = "UDP"
)

// A ContainerPort specifies a port that is exposed by a container.
type ContainerPort struct {
	// Name of this port. Must be unique within its container. Must be lowercase
	// alphabetical characters.
	// +kubebuilder:validation:Pattern=^[a-z]+$
	Name string `json:"name"`

	// Port number. Must be unique within its container.
	Port int32 `json:"containerPort"`

	// TODO(negz): Use +kubebuilder:default marker to default Protocol to TCP
	// once we're generating v1 CRDs.

	// Protocol used by the server listening on this port.
	// +kubebuilder:validation:Enum=TCP;UDP
	// +optional
	Protocol *TransportProtocol `json:"protocol,omitempty"`
}

// An ExecProbe probes a container's health by executing a command.
type ExecProbe struct {
	// Command to be run by this probe.
	Command []string `json:"command"`
}

// A HTTPHeader to be passed when probing a container.
type HTTPHeader struct {
	// Name of this HTTP header. Must be unique per probe.
	Name string `json:"name"`

	// Value of this HTTP header.
	Value string `json:"value"`
}

// A HTTPGetProbe probes a container's health by sending an HTTP GET request.
type HTTPGetProbe struct {
	// Path to probe, e.g. '/healthz'.
	Path string `json:"path"`

	// Port to probe.
	Port int32 `json:"port"`

	// HTTPHeaders to send with the GET request.
	// +optional
	HTTPHeaders []HTTPHeader `json:"httpHeaders,omitempty"`
}

// A TCPSocketProbe probes a container's health by connecting to a TCP socket.
type TCPSocketProbe struct {
	// Port this probe should connect to.
	Port int32 `json:"port"`
}

// A ContainerHealthProbe specifies how to probe the health of a container.
// Exactly one of Exec, HTTPGet, or TCPSocket must be specified.
type ContainerHealthProbe struct {
	// Exec probes a container's health by executing a command.
	// +optional
	Exec *ExecProbe `json:"exec,omitempty"`

	// HTTPGet probes a container's health by sending an HTTP GET request.
	// +optional
	HTTPGet *HTTPGetProbe `json:"httpGet,omitempty"`

	// TCPSocketProbe probes a container's health by connecting to a TCP socket.
	// +optional
	TCPSocket *TCPSocketProbe `json:"tcpSocket,omitempty"`

	// InitialDelaySeconds after a container starts before the first probe.
	// +optional
	InitialDelaySeconds *int32 `json:"initialDelaySeconds,omitempty"`

	// TODO(negz): Use +kubebuilder:default marker to default PeriodSeconds,
	// TimeoutSeconds, SuccessThreshold, and FailureThreshold to 10, 1, 1, and 3
	// respectively once we're generating v1 CRDs.

	// PeriodSeconds between probes.
	// +optional
	PeriodSeconds *int32 `json:"periodSeconds,omitempty"`

	// TimeoutSeconds after which the probe times out.
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`

	// SuccessThreshold specifies how many consecutive probes must success in
	// order for the container to be considered healthy.
	// +optional
	SuccessThreshold *int32 `json:"successThreshold,omitempty"`

	// FailureThreshold specifies how many consecutive probes must fail in order
	// for the container to be considered healthy.
	// +optional
	FailureThreshold *int32 `json:"failureThreshold,omitempty"`
}

// A Container represents an Open Containers Initiative (OCI) container.
type Container struct {
	// Name of this container. Must be unique within its workload.
	Name string `json:"name"`

	// Image this container should run. Must be a path-like or URI-like
	// representation of an OCI image. May be prefixed with a registry address
	// and should be suffixed with a tag.
	Image string `json:"image"`

	// Resources required by this container
	// +optional
	Resources *ContainerResources `json:"resources,omitempty"`

	// Command to be run by this container.
	// +optional
	Command []string `json:"command,omitempty"`

	// Arguments to be passed to the command run by this container.
	// +optional
	Arguments []string `json:"args,omitempty"`

	// Environment variables that should be set within this container.
	// +optional
	Environment []ContainerEnvVar `json:"env,omitempty"`

	// ConfigFiles that should be written within this container.
	// +optional
	ConfigFiles []ContainerConfigFile `json:"config,omitempty"`

	// Ports exposed by this container.
	// +optional
	Ports []ContainerPort `json:"ports,omitempty"`

	// A LivenessProbe assesses whether this container is alive. Containers that
	// fail liveness probes will be restarted.
	// +optional
	LivenessProbe *ContainerHealthProbe `json:"livenessProbe,omitempty"`

	// A ReadinessProbe assesses whether this container is ready to serve
	// requests. Containers that fail readiness probes will be withdrawn from
	// service.
	// +optional
	ReadinessProbe *ContainerHealthProbe `json:"readinessProbe,omitempty"`

	// TODO(negz): Ideally the key within this secret would be configurable, but
	// the current OAM spec allows only a secret name.

	// ImagePullSecret specifies the name of a Secret from which the
	// credentials required to pull this container's image can be loaded.
	// +optional
	ImagePullSecret *string `json:"imagePullSecret,omitempty"`
}

// A ContainerizedWorkloadSpec defines the desired state of a
// ContainerizedWorkload.
type ContainerizedWorkloadSpec struct {
	// OperatingSystem required by this workload.
	// +kubebuilder:validation:Enum=linux;windows
	// +optional
	OperatingSystem *OperatingSystem `json:"osType,omitempty"`

	// CPUArchitecture required by this workload.
	// +kubebuilder:validation:Enum=i386;amd64;arm;arm64
	// +optional
	CPUArchitecture *CPUArchitecture `json:"arch,omitempty"`

	// Containers of which this workload consists.
	Containers []Container `json:"containers"`
}

// A ContainerizedWorkloadStatus represents the observed state of a
// ContainerizedWorkload.
type ContainerizedWorkloadStatus struct {
	runtimev1alpha1.ConditionedStatus `json:",inline"`

	// Resources managed by this containerised workload.
	Resources []runtimev1alpha1.TypedReference `json:"resources,omitempty"`
}

// +kubebuilder:object:root=true

// A ContainerizedWorkload is a workload that runs OCI containers.
// +kubebuilder:resource:categories={crossplane,oam}
// +kubebuilder:subresource:status
type ContainerizedWorkload struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContainerizedWorkloadSpec   `json:"spec,omitempty"`
	Status ContainerizedWorkloadStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ContainerizedWorkloadList contains a list of ContainerizedWorkload.
type ContainerizedWorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContainerizedWorkload `json:"items"`
}
