package v1alpha2

import "k8s.io/apimachinery/pkg/types"

// A WorkloadStatus represents the current state of a workload.
type WorkloadStatus string

// Workload statuses.
const (
	WorkloadStatusUnknown = "Unknown"
	WorkloadStatusError   = "Error"
	WorkloadStatusCreated = "Created"
)

// A TraitStatus represents the state of a trait.
type TraitStatus string

// Trait statuses.
const (
	TraitStatusUnknown = "Unknown"
	TraitStatusError   = "Error"
	TraitStatusCreated = "Created"
)

// A DefinitionStatus represents the state of a definition.
type DefinitionStatus string

// Trait statuses.
const (
	DefinitionStatusUnknown    = "Unknown"
	DefinitionStatusError      = "Error"
	DefinitionStatusAssociated = "Associated"
)

// A DefinitionReference refers to a CustomResourceDefinition by name.
type DefinitionReference struct {
	// Name of the referenced CustomResourceDefinition.
	Name string `json:"name"`
}

// A OAMReference refers to an OAM resource.
type OAMReference struct {
	// APIVersion of the referenced workload.
	APIVersion string `json:"apiVersion"`

	// Kind of the referenced workload.
	Kind string `json:"kind"`

	// Name of the referenced workload.
	Name string `json:"name"`

	// UID of the referenced workload.
	// +optional
	UID *types.UID `json:"uid,omitempty"`
}
