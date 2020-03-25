# core.oam.dev/v1alpha2 API Reference

Package v1alpha2 contains resources relating to the Open Application Model. See https://github.com/oam-dev/spec for more details.

This API group contains the following Crossplane resources:

* [ApplicationConfiguration](#ApplicationConfiguration)
* [Component](#Component)
* [ContainerizedWorkload](#ContainerizedWorkload)
* [ManualScalerTrait](#ManualScalerTrait)
* [ScopeDefinition](#ScopeDefinition)
* [TraitDefinition](#TraitDefinition)
* [WorkloadDefinition](#WorkloadDefinition)

## ApplicationConfiguration

An ApplicationConfiguration represents an OAM application.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `core.oam.dev/v1alpha2`
`kind` | string | `ApplicationConfiguration`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [ApplicationConfigurationSpec](#ApplicationConfigurationSpec) | An ApplicationConfigurationSpec defines the desired state of a ApplicationConfiguration.
`status` | [ApplicationConfigurationStatus](#ApplicationConfigurationStatus) | An ApplicationConfigurationStatus represents the observed state of a ApplicationConfiguration.



## Component

A Component describes how an OAM workload kind may be instantiated.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `core.oam.dev/v1alpha2`
`kind` | string | `Component`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [ComponentSpec](#ComponentSpec) | A ComponentSpec defines the desired state of a Component.
`status` | [ComponentStatus](#ComponentStatus) | A ComponentStatus represents the observed state of a Component.



## ContainerizedWorkload

A ContainerizedWorkload is a workload that runs OCI containers.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `core.oam.dev/v1alpha2`
`kind` | string | `ContainerizedWorkload`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [ContainerizedWorkloadSpec](#ContainerizedWorkloadSpec) | A ContainerizedWorkloadSpec defines the desired state of a ContainerizedWorkload.
`status` | [ContainerizedWorkloadStatus](#ContainerizedWorkloadStatus) | A ContainerizedWorkloadStatus represents the observed state of a ContainerizedWorkload.



## ManualScalerTrait

A ManualScalerTrait determines how many replicas a workload should have.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `core.oam.dev/v1alpha2`
`kind` | string | `ManualScalerTrait`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [ManualScalerTraitSpec](#ManualScalerTraitSpec) | A ManualScalerTraitSpec defines the desired state of a ManualScalerTrait.
`status` | [ManualScalerTraitStatus](#ManualScalerTraitStatus) | A ManualScalerTraitStatus represents the observed state of a ManualScalerTrait.



## ScopeDefinition

A ScopeDefinition registers a kind of Kubernetes custom resource as a valid OAM scope kind by referencing its CustomResourceDefinition. The CRD is used to validate the schema of the scope when it is embedded in an OAM ApplicationConfiguration.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `core.oam.dev/v1alpha2`
`kind` | string | `ScopeDefinition`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [ScopeDefinitionSpec](#ScopeDefinitionSpec) | A ScopeDefinitionSpec defines the desired state of a ScopeDefinition.



## TraitDefinition

A TraitDefinition registers a kind of Kubernetes custom resource as a valid OAM trait kind by referencing its CustomResourceDefinition. The CRD is used to validate the schema of the trait when it is embedded in an OAM ApplicationConfiguration.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `core.oam.dev/v1alpha2`
`kind` | string | `TraitDefinition`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [TraitDefinitionSpec](#TraitDefinitionSpec) | A TraitDefinitionSpec defines the desired state of a TraitDefinition.



## WorkloadDefinition

A WorkloadDefinition registers a kind of Kubernetes custom resource as a valid OAM workload kind by referencing its CustomResourceDefinition. The CRD is used to validate the schema of the workload when it is embedded in an OAM Component.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `core.oam.dev/v1alpha2`
`kind` | string | `WorkloadDefinition`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [WorkloadDefinitionSpec](#WorkloadDefinitionSpec) | A WorkloadDefinitionSpec defines the desired state of a WorkloadDefinition.



## ApplicationConfigurationComponent

An ApplicationConfigurationComponent specifies a component of an ApplicationConfiguration. Each component is used to instantiate a workload.

Appears in:

* [ApplicationConfigurationSpec](#ApplicationConfigurationSpec)


Name | Type | Description
-----|------|------------
`componentName` | string | ComponentName specifies a component of which an ApplicationConfiguration should consist. The named component must exist.
`parameterValues` | Optional [[]ComponentParameterValue](#ComponentParameterValue) | ParameterValues specify values for the the specified component&#39;s parameters. Any parameter required by the component must be specified.
`traits` | Optional [[]ComponentTrait](#ComponentTrait) | Traits of the specified component.
`scopes` | Optional [[]ComponentScope](#ComponentScope) | Scopes in which the specified component should exist.



## ApplicationConfigurationSpec

An ApplicationConfigurationSpec defines the desired state of a ApplicationConfiguration.

Appears in:

* [ApplicationConfiguration](#ApplicationConfiguration)


Name | Type | Description
-----|------|------------
`components` | [[]ApplicationConfigurationComponent](#ApplicationConfigurationComponent) | Components of which this ApplicationConfiguration consists. Each component will be used to instantiate a workload.



## ApplicationConfigurationStatus

An ApplicationConfigurationStatus represents the observed state of a ApplicationConfiguration.

Appears in:

* [ApplicationConfiguration](#ApplicationConfiguration)


Name | Type | Description
-----|------|------------
`workloads` | [[]WorkloadStatus](#WorkloadStatus) | Workloads created by this ApplicationConfiguration.


ApplicationConfigurationStatus supports all fields of:

* [v1alpha1.ConditionedStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#conditionedstatus)


## CPUArchitecture

A CPUArchitecture required by a containerised workload. Alias of string.

Appears in:

* [ContainerizedWorkloadSpec](#ContainerizedWorkloadSpec)


## CPUResources

CPUResources required by a container.

Appears in:

* [ContainerResources](#ContainerResources)


Name | Type | Description
-----|------|------------
`required` | k8s.io/apimachinery/pkg/api/resource.Quantity | Required CPU count. 1.0 represents one CPU core.



## ComponentParameter

A ComponentParameter defines a configurable parameter of a component.

Appears in:

* [ComponentSpec](#ComponentSpec)


Name | Type | Description
-----|------|------------
`name` | string | Name of this parameter. OAM ApplicationConfigurations will specify parameter values using this name.
`fieldPaths` | []string | FieldPaths specifies an array of fields within this Component&#39;s workload that will be overwritten by the value of this parameter. The type of the parameter (e.g. int, string) is inferred from the type of these fields; All fields must be of the same type. Fields are specified as JSON field paths without a leading dot, for example &#39;spec.replicas&#39;.
`required` | Optional bool | Required specifies whether or not a value for this parameter must be supplied when authoring an ApplicationConfiguration.
`description` | Optional string | Description of this parameter.



## ComponentParameterValue

A ComponentParameterValue specifies a value for a named parameter. The associated component must publish a parameter with this name.

Appears in:

* [ApplicationConfigurationComponent](#ApplicationConfigurationComponent)


Name | Type | Description
-----|------|------------
`name` | string | Name of the component parameter to set.
`value` | k8s.io/apimachinery/pkg/util/intstr.IntOrString | Value to set.



## ComponentScope

A ComponentScope specifies a scope in which a component should exist.

Appears in:

* [ApplicationConfigurationComponent](#ApplicationConfigurationComponent)


Name | Type | Description
-----|------|------------
`scopeRef` | [v1alpha1.TypedReference](../crossplane-runtime/core-crossplane-io-v1alpha1.md#typedreference) | A ScopeReference must refer to an OAM scope resource.



## ComponentSpec

A ComponentSpec defines the desired state of a Component.

Appears in:

* [Component](#Component)


Name | Type | Description
-----|------|------------
`workload` | k8s.io/apimachinery/pkg/runtime.RawExtension | A Workload that will be created for each ApplicationConfiguration that includes this Component. Workloads must be defined by a WorkloadDefinition.
`parameters` | Optional [[]ComponentParameter](#ComponentParameter) | Parameters exposed by this component. ApplicationConfigurations that reference this component may specify values for these parameters, which will in turn be injected into the embedded workload.



## ComponentStatus

A ComponentStatus represents the observed state of a Component.

Appears in:

* [Component](#Component)




ComponentStatus supports all fields of:

* [v1alpha1.ConditionedStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#conditionedstatus)


## ComponentTrait

A ComponentTrait specifies a trait that should be applied to a component.

Appears in:

* [ApplicationConfigurationComponent](#ApplicationConfigurationComponent)


Name | Type | Description
-----|------|------------
`trait` | k8s.io/apimachinery/pkg/runtime.RawExtension | A Trait that will be created for the component



## Container

A Container represents an Open Containers Initiative (OCI) container.

Appears in:

* [ContainerizedWorkloadSpec](#ContainerizedWorkloadSpec)


Name | Type | Description
-----|------|------------
`name` | string | Name of this container. Must be unique within its workload.
`image` | string | Image this container should run. Must be a path-like or URI-like representation of an OCI image. May be prefixed with a registry address and should be suffixed with a tag.
`resources` | Optional [ContainerResources](#ContainerResources) | Resources required by this container
`command` | Optional []string | Command to be run by this container.
`args` | Optional []string | Arguments to be passed to the command run by this container.
`env` | Optional [[]ContainerEnvVar](#ContainerEnvVar) | Environment variables that should be set within this container.
`config` | Optional [[]ContainerConfigFile](#ContainerConfigFile) | ConfigFiles that should be written within this container.
`ports` | Optional [[]ContainerPort](#ContainerPort) | Ports exposed by this container.
`livenessProbe` | Optional [ContainerHealthProbe](#ContainerHealthProbe) | A LivenessProbe assesses whether this container is alive. Containers that fail liveness probes will be restarted.
`readiessProbe` | Optional [ContainerHealthProbe](#ContainerHealthProbe) | A ReadinessProbe assesses whether this container is ready to serve requests. Containers that fail readiness probes will be withdrawn from service.
`imagePullSecret` | Optional string | ImagePullSecret specifies the name of a Secret from which the credentials required to pull this container&#39;s image can be loaded.



## ContainerConfigFile

A ContainerConfigFile specifies a configuration file that should be written within a container.

Appears in:

* [Container](#Container)


Name | Type | Description
-----|------|------------
`path` | string | Path within the container at which the configuration file should be written.
`value` | Optional string | Value that should be written to the configuration file.
`fromSecret` | Optional [SecretKeySelector](#SecretKeySelector) | FromSecret is a secret key reference which can be used to assign a value to be written to the configuration file at the given path in the container.



## ContainerEnvVar

A ContainerEnvVar specifies an environment variable that should be set within a container.

Appears in:

* [Container](#Container)


Name | Type | Description
-----|------|------------
`name` | string | Name of the environment variable. Must be composed of valid Unicode letter and number characters, as well as _ and -.
`value` | Optional string | Value of the environment variable.
`fromSecret` | Optional [SecretKeySelector](#SecretKeySelector) | FromSecret is a secret key reference which can be used to assign a value to the environment variable.



## ContainerHealthProbe

A ContainerHealthProbe specifies how to probe the health of a container. Exactly one of Exec, HTTPGet, or TCPSocket must be specified.

Appears in:

* [Container](#Container)


Name | Type | Description
-----|------|------------
`exec` | Optional [ExecProbe](#ExecProbe) | Exec probes a container&#39;s health by executing a command.
`httpGet` | Optional [HTTPGetProbe](#HTTPGetProbe) | HTTPGet probes a container&#39;s health by sending an HTTP GET request.
`tcpSocket` | Optional [TCPSocketProbe](#TCPSocketProbe) | TCPSocketProbe probes a container&#39;s health by connecting to a TCP socket.
`initialDelaySeconds` | Optional int32 | InitialDelaySeconds after a container starts before the first probe.
`periodSeconds` | Optional int32 | PeriodSeconds between probes.
`timeoutSeconds` | Optional int32 | TimeoutSeconds after which the probe times out.
`successThreshold` | Optional int32 | SuccessThreshold specifies how many consecutive probes must success in order for the container to be considered healthy.
`failureThreshold` | Optional int32 | FailureThreshold specifies how many consecutive probes must fail in order for the container to be considered healthy.



## ContainerPort

A ContainerPort specifies a port that is exposed by a container.

Appears in:

* [Container](#Container)


Name | Type | Description
-----|------|------------
`name` | string | Name of this port. Must be unique within its container. Must be lowercase alphabetical characters.
`containerPort` | int32 | Port number. Must be unique within its container.
`protocol` | Optional [TransportProtocol](#TransportProtocol) | Protocol used by the server listening on this port.



## ContainerResources

ContainerResources specifies a container&#39;s required compute resources.

Appears in:

* [Container](#Container)


Name | Type | Description
-----|------|------------
`cpu` | [CPUResources](#CPUResources) | CPU required by this container.
`memory` | [MemoryResources](#MemoryResources) | Memory required by this container.
`gpu` | Optional [GPUResources](#GPUResources) | GPU required by this container.
`volumes` | Optional [[]VolumeResource](#VolumeResource) | Volumes required by this container.
`extended` | Optional [[]ExtendedResource](#ExtendedResource) | Extended resources required by this container.



## ContainerizedWorkloadSpec

A ContainerizedWorkloadSpec defines the desired state of a ContainerizedWorkload.

Appears in:

* [ContainerizedWorkload](#ContainerizedWorkload)


Name | Type | Description
-----|------|------------
`osType` | Optional [OperatingSystem](#OperatingSystem) | OperatingSystem required by this workload.
`arch` | Optional [CPUArchitecture](#CPUArchitecture) | CPUArchitecture required by this workload.
`containers` | [[]Container](#Container) | Containers of which this workload consists.



## ContainerizedWorkloadStatus

A ContainerizedWorkloadStatus represents the observed state of a ContainerizedWorkload.

Appears in:

* [ContainerizedWorkload](#ContainerizedWorkload)


Name | Type | Description
-----|------|------------
`resources` | [[]v1alpha1.TypedReference](../crossplane-runtime/core-crossplane-io-v1alpha1.md#typedreference) | Resources managed by this containerised workload.


ContainerizedWorkloadStatus supports all fields of:

* [v1alpha1.ConditionedStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#conditionedstatus)


## DefinitionReference

A DefinitionReference refers to a CustomResourceDefinition by name.

Appears in:

* [ScopeDefinitionSpec](#ScopeDefinitionSpec)
* [TraitDefinitionSpec](#TraitDefinitionSpec)
* [WorkloadDefinitionSpec](#WorkloadDefinitionSpec)


Name | Type | Description
-----|------|------------
`name` | string | Name of the referenced CustomResourceDefinition.



## DiskResource

DiskResource required by a container.

Appears in:

* [VolumeResource](#VolumeResource)


Name | Type | Description
-----|------|------------
`required` | k8s.io/apimachinery/pkg/api/resource.Quantity | Required disk space.
`ephemeral` | Optional bool | Ephemeral specifies whether an external disk needs to be mounted.



## ExecProbe

An ExecProbe probes a container&#39;s health by executing a command.

Appears in:

* [ContainerHealthProbe](#ContainerHealthProbe)


Name | Type | Description
-----|------|------------
`command` | []string | Command to be run by this probe.



## ExtendedResource

ExtendedResource required by a container.

Appears in:

* [ContainerResources](#ContainerResources)


Name | Type | Description
-----|------|------------
`name` | string | Name of the external resource. Resource names are specified in kind.group/version format, e.g. motionsensor.ext.example.com/v1.
`required` | k8s.io/apimachinery/pkg/util/intstr.IntOrString | Required extended resource(s), e.g. 8 or &#34;very-cool-widget&#34;



## GPUResources

GPUResources required by a container.

Appears in:

* [ContainerResources](#ContainerResources)


Name | Type | Description
-----|------|------------
`required` | k8s.io/apimachinery/pkg/api/resource.Quantity | Required GPU count.



## HTTPGetProbe

A HTTPGetProbe probes a container&#39;s health by sending an HTTP GET request.

Appears in:

* [ContainerHealthProbe](#ContainerHealthProbe)


Name | Type | Description
-----|------|------------
`path` | string | Path to probe, e.g. &#39;/healthz&#39;.
`port` | int32 | Port to probe.
`httpHeaders` | Optional [[]HTTPHeader](#HTTPHeader) | HTTPHeaders to send with the GET request.



## HTTPHeader

A HTTPHeader to be passed when probing a container.

Appears in:

* [HTTPGetProbe](#HTTPGetProbe)


Name | Type | Description
-----|------|------------
`name` | string | Name of this HTTP header. Must be unique per probe.
`value` | string | Value of this HTTP header.



## ManualScalerTraitSpec

A ManualScalerTraitSpec defines the desired state of a ManualScalerTrait.

Appears in:

* [ManualScalerTrait](#ManualScalerTrait)


Name | Type | Description
-----|------|------------
`replicaCount` | int32 | ReplicaCount of the workload this trait applies to.
`workloadRef` | [v1alpha1.TypedReference](../crossplane-runtime/core-crossplane-io-v1alpha1.md#typedreference) | WorkloadReference to the workload this trait applies to.



## ManualScalerTraitStatus

A ManualScalerTraitStatus represents the observed state of a ManualScalerTrait.

Appears in:

* [ManualScalerTrait](#ManualScalerTrait)




ManualScalerTraitStatus supports all fields of:

* [v1alpha1.ConditionedStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#conditionedstatus)


## MemoryResources

MemoryResources required by a container.

Appears in:

* [ContainerResources](#ContainerResources)


Name | Type | Description
-----|------|------------
`required` | k8s.io/apimachinery/pkg/api/resource.Quantity | Required memory.



## OperatingSystem

An OperatingSystem required by a containerised workload. Alias of string.

Appears in:

* [ContainerizedWorkloadSpec](#ContainerizedWorkloadSpec)


## ScopeDefinitionSpec

A ScopeDefinitionSpec defines the desired state of a ScopeDefinition.

Appears in:

* [ScopeDefinition](#ScopeDefinition)


Name | Type | Description
-----|------|------------
`definitionRef` | [DefinitionReference](#DefinitionReference) | Reference to the CustomResourceDefinition that defines this scope kind.
`allowComponentOverlap` | bool | AllowComponentOverlap specifies whether an OAM component may exist in multiple instances of this kind of scope.



## SecretKeySelector

A SecretKeySelector is a reference to a secret key in an arbitrary namespace.

Appears in:

* [ContainerConfigFile](#ContainerConfigFile)
* [ContainerEnvVar](#ContainerEnvVar)


Name | Type | Description
-----|------|------------
`name` | string | The name of the secret.
`key` | string | The key to select.



## TCPSocketProbe

A TCPSocketProbe probes a container&#39;s health by connecting to a TCP socket.

Appears in:

* [ContainerHealthProbe](#ContainerHealthProbe)


Name | Type | Description
-----|------|------------
`port` | int32 | Port this probe should connect to.



## TraitDefinitionSpec

A TraitDefinitionSpec defines the desired state of a TraitDefinition.

Appears in:

* [TraitDefinition](#TraitDefinition)


Name | Type | Description
-----|------|------------
`definitionRef` | [DefinitionReference](#DefinitionReference) | Reference to the CustomResourceDefinition that defines this trait kind.
`appliesToWorkloads` | Optional []string | AppliesToWorkloads specifies the list of workload kinds this trait applies to. Workload kinds are specified in kind.group/version format, e.g. server.core.oam.dev/v1alpha2. Traits that omit this field apply to all workload kinds.



## TraitStatus

A TraitStatus represents the state of a trait. Alias of string.


## TransportProtocol

A TransportProtocol represents a transport layer protocol. Alias of string.

Appears in:

* [ContainerPort](#ContainerPort)


## VolumeAccessMode

A VolumeAccessMode determines how a volume may be accessed. Alias of string.

Appears in:

* [VolumeResource](#VolumeResource)


## VolumeResource

VolumeResource required by a container.

Appears in:

* [ContainerResources](#ContainerResources)


Name | Type | Description
-----|------|------------
`name` | string | Name of this volume. Must be unique within its container.
`mountPath` | string | MouthPath at which this volume will be mounted within its container.
`accessMode` | Optional [VolumeAccessMode](#VolumeAccessMode) | AccessMode of this volume; RO (read only) or RW (read and write).
`sharingPolicy` | Optional [VolumeSharingPolicy](#VolumeSharingPolicy) | SharingPolicy of this volume; Exclusive or Shared.
`disk` | Optional [DiskResource](#DiskResource) | Disk requirements of this volume.



## VolumeSharingPolicy

A VolumeSharingPolicy determines how a volume may be shared. Alias of string.

Appears in:

* [VolumeResource](#VolumeResource)


## WorkloadDefinitionSpec

A WorkloadDefinitionSpec defines the desired state of a WorkloadDefinition.

Appears in:

* [WorkloadDefinition](#WorkloadDefinition)


Name | Type | Description
-----|------|------------
`definitionRef` | [DefinitionReference](#DefinitionReference) | Reference to the CustomResourceDefinition that defines this workload kind.



## WorkloadStatus

A WorkloadStatus represents the status of a workload.

Appears in:

* [ApplicationConfigurationStatus](#ApplicationConfigurationStatus)


Name | Type | Description
-----|------|------------
`componentName` | string | ComponentName that produced this workload.
`workloadRef` | [v1alpha1.TypedReference](../crossplane-runtime/core-crossplane-io-v1alpha1.md#typedreference) | Reference to a workload created by an ApplicationConfiguration.
`traits` | [[]WorkloadTrait](#WorkloadTrait) | Traits associated with this workload.



## WorkloadTrait

A WorkloadTrait represents a trait associated with a workload.

Appears in:

* [WorkloadStatus](#WorkloadStatus)


Name | Type | Description
-----|------|------------
`traitRef` | [v1alpha1.TypedReference](../crossplane-runtime/core-crossplane-io-v1alpha1.md#typedreference) | Reference to a trait created by an ApplicationConfiguration.



This API documentation was generated by `crossdocs`.