package v1

// PackageRuntimeSpec specifies configuration for the runtime of a package.
// Only used by packages that uses a runtime, i.e. by providers and functions
// but not for configurations.
type PackageRuntimeSpec struct {
	// ControllerConfigRef references a ControllerConfig resource that will be
	// used to configure the packaged controller Deployment.
	// Deprecated: Use RuntimeConfigReference instead.
	// +optional
	ControllerConfigReference *ControllerConfigReference `json:"controllerConfigRef,omitempty"`
}

// PackageRevisionRuntimeSpec specifies configuration for the runtime of a
// package revision. Only used by packages that uses a runtime, i.e. by
// providers and functions but not for configurations.
type PackageRevisionRuntimeSpec struct {
	PackageRuntimeSpec `json:",inline"`
	// TLSServerSecretName is the name of the TLS Secret that stores server
	// certificates of the Provider.
	// +optional
	TLSServerSecretName *string `json:"tlsServerSecretName,omitempty"`

	// TLSClientSecretName is the name of the TLS Secret that stores client
	// certificates of the Provider.
	// +optional
	TLSClientSecretName *string `json:"tlsClientSecretName,omitempty"`
}

// A ControllerConfigReference to a ControllerConfig resource that will be used
// to configure the packaged controller Deployment.
type ControllerConfigReference struct {
	// Name of the ControllerConfig.
	Name string `json:"name"`
}
