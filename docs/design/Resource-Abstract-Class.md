# Constructs
We have been discussing 3 constructs modeled after or inspired by Kubernetes PV,PVC,StorageClass.

## [Resource]Instance
Resource could be any of cloud provider managed resources: RDSInstance, CloudSQLInstance, EKS, GKE, etc...
This construct is modeled after or inspired by the Kubernetes PersistentVolume construct.

```go
type PersistentVolumeSpec struct {
    // Resources represents the actual resources of the volume
    Capacity ResourceList
    // Source represents the location and type of a volume to mount.
    PersistentVolumeSource
    // AccessModes contains all ways the volume can be mounted
    // +optional
    AccessModes []PersistentVolumeAccessMode
    // ClaimRef is part of a bi-directional binding between PersistentVolume and PersistentVolumeClaim.
    // ClaimRef is expected to be non-nil when bound.
    // claim.VolumeName is the authoritative bind between PV and PVC.
    // When set to non-nil value, PVC.Spec.Selector of the referenced PVC is
    // ignored, i.e. labels of this PV do not need to match PVC selector.
    // +optional
    ClaimRef *ObjectReference
    // Optional: what happens to a persistent volume when released from its claim.
    // +optional
    PersistentVolumeReclaimPolicy PersistentVolumeReclaimPolicy
    // Name of StorageClass to which this persistent volume belongs. Empty value
    // means that this volume does not belong to any StorageClass.
    // +optional
    StorageClassName string
    // A list of mount options, e.g. ["ro", "soft"]. Not validated - mount will
    // simply fail if one is invalid.
    // +optional
    MountOptions []string
    // volumeMode defines if a volume is intended to be used with a formatted filesystem
    // or to remain in raw block state. Value of Filesystem is implied when not included in spec.
    // This is an alpha feature and may change in the future.
    // +optional
    VolumeMode *PersistentVolumeMode
    // NodeAffinity defines constraints that limit what nodes this volume can be accessed from.
    // This field influences the scheduling of pods that use this volume.
    // +optional
    NodeAffinity *VolumeNodeAffinity
}
```

ResourceInstance provides a concrete definition of the underlying cloud resource. For all intended purpose, the 
ResourceInstance is answering `WHAT` question, i.e. "What is the resource", or "What is the resource definition".

ResourceInstance status is intended to contain all the necessary information reflecting resource status and all other
dynamically generated properties, like: `IP Adress`, Endpoint, Password. As an implementation choice, we do not store
any sensitive information in the resource status, like `Password`, and instead leveraging Kubernetes Secrets as a storage
medium.  

ResourceInstance is a "system" resource and requires user to have elevated "system" privileges to create and manage this
resource (typically cluster administrator). 

In summary, ResourceInstance could be described by following attributes:
- Cloud resource definition
- Cloud resource state
- Kubernetes system resource


## AbstractInstance
Also formerly known as "Claim", similar to the `[Resource]Instance` it is modeled after `PersistentVolumeClaim`.
```go
// PersistentVolumeClaimSpec describes the common attributes of storage devices
// and allows a Source for provider-specific attributes
type PersistentVolumeClaimSpec struct {
	// Contains the types of access modes required
	// +optional
	AccessModes []PersistentVolumeAccessMode
	// A label query over volumes to consider for binding. This selector is
	// ignored when VolumeName is set
	// +optional
	Selector *metav1.LabelSelector
	// Resources represents the minimum resources required
	// +optional
	Resources ResourceRequirements
	// VolumeName is the binding reference to the PersistentVolume backing this
	// claim. When set to non-empty value Selector is not evaluated
	// +optional
	VolumeName string
	// Name of the StorageClass required by the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes/#class-1
	// +optional
	StorageClassName *string
	// volumeMode defines what type of volume is required by the claim.
	// Value of Filesystem is implied when not included in claim spec.
	// This is an alpha feature and may change in the future.
	// +optional
	VolumeMode *PersistentVolumeMode
	// This field requires the VolumeSnapshotDataSource alpha feature gate to be
	// enabled and currently VolumeSnapshot is the only supported data source.
	// If the provisioner can support VolumeSnapshot data source, it will create
	// a new volume and data will be restored to the volume at the same time.
	// If the provisioner does not support VolumeSnapshot data source, volume will
	// not be created and the failure will be reported as an event.
	// In the future, we plan to support more data source types and the behavior
	// of the provisioner may change.
	// +optional
	DataSource *TypedLocalObjectReference
}
```

AbstractResource intended to provide the means to consume the concrete resource from the Application domain. For that
purpose AbstractResource is defined in the same namespace as the consuming application, in contrast to the `ResourceInstance`
which is defined at the "system" level.

The `AbstractResource` provides the utilization which could be classified by two aspectes:
- Binding - find and match or create the `ResourceInstance` and bind to it
- Usage - provide additional parameters to handle specialized usage of this resource as it pertains to a consuming application.

### Binding
#### Static
In the static binding context, the `AbstractResource` is intended to find an available `ResourceInstance` and if one is found bind to it.

The `AbstractInstance` provides two ways of finding a `ResourceInstance`:
- `VolumeName`: direct link to the `ResourceInstance` with the same name
- `Selector`: a mechanism to  narrow list of the `ResourceInstance` candidates that contain matching set of `labels/values` 

**Note**: `AbstractVolume` is expected to use one or another, but not both, i.e. if `VolumeName` is provided, the `Selector` values are ignored. 

Once the one or more `ResourceInstances` are found as a list of candidates, this list is evaluated against the `ResourceRequirements`
to match on the available resources using the "minimum" set of the required attributes.

In the end, the `AbstractResource` if bound to one of the `ResourceInstance` from the list of available candidates.

#### Dynamic
If `AbstractInstance` did not find any existing (static) `ResourceInstances` available for binding, it provides a path to 
create a new `ResourceInstance` using `ResourceClasssName` attribute, which points to an existing `ResourceClass` (see below on resource class definition).

If `ResourceClassName` is not provided, the `AbstractInstance` may use a `default` `ResourceClass` if one is defined.

In the end, the `AbstractResource` is bound to a newly created `ResourceInstance`

**Note** it is also expected that the `AbstractInstance` may be left in `Unbound` state, because it neither found
any matching instances nor was able to dynamically provision any new instances.

### Usage
In the event of `AbstractInstance` successfully bound to a `ResourceInstanc`, the usage of the resource needs to be further refined.
For that purpose the `AbstractInstance` provide additional "Usage" constructs/attributes. The intent of those attributes
is to define "HOW" the `ResourceInstance` should be used, oppositely to "WHAT" `ResourceInstance` actually is.

In the example above - case of `PVC` those are:
- `AccessMode` - mode of access the the volume by the client application
- `VolumeMode` - what file system should be exposed to the client application to be used against a given volume.
- `DataSource` - as it appears needed to further refine how this volume handles snap-shot (`VolumeSnapshotDataSource`)

In our case, the `AbstractResource` provides ability (explicit or implicit) to define secret/service name for application to
interract with the `ResourceInstance`. 

The main point here being, the "usage" attributes do not alter of "WHAT" the `ResourceInstance` is. 
This true for the `Dynamic` and even more so for `Static` provisionning.
- `Dynamic` - all `ResourceInstance` attributes are expected to be defined in the `ResourceClass` (see below)
- `Static` - the `ResourceInstance` already created (up-and-running), hence, by definition there should be no changes to this instance.


#### Usage or Definition (or Both)
Sometimes it is not easy to differentiate which category `ResourceInstance` attribute corresponds to. 
Is it a `definition`, is it a `usage`, possibly both?

Let's take a look at some of them, and hopefully we gain some clarity:

1. Example: `MySQL` engine - clearly it is a defining attribute, to the point that it dictates the client implementation (wire protocol), 
thus we promoted `MySQLIsntance` as a defined type.
2. Example: `MySQL` engine version. This is were we appears to be have difference of the opinions. I think there is a strong case to
treat `engine-version` as a defining attribute, i.e. it is closer (if not entirely in) to Database definition, than to how the client application
intended to use it.
3. Example: `Mysql` master user name. This one is not so clear cut. On one hand - `masterUsername` appears to be a part of the
Database definition. On another hand it does have a strong usage aspect - since, after all it is used by the client application to connect
to the database. 

## ResourceClass
To simply and/or to provider separation between `ResourceInstance` definition/creation and usage, the cluster "system" 
administrator may create one or many instances of the `ResourceClass`, which is modeled after the `StorageClass`   

```go
// StorageClass describes a named "class" of storage offered in a cluster.
// Different classes might map to quality-of-service levels, or to backup policies,
// or to arbitrary policies determined by the cluster administrators.  Kubernetes
// itself is unopinionated about what classes represent.  This concept is sometimes
// called "profiles" in other storage systems.
// The name of a StorageClass object is significant, and is how users can request a particular class.
type StorageClass struct {
	metav1.TypeMeta
	// +optional
	metav1.ObjectMeta

	// provisioner is the driver expected to handle this StorageClass.
	// This is an optionally-prefixed name, like a label key.
	// For example: "kubernetes.io/gce-pd" or "kubernetes.io/aws-ebs".
	// This value may not be empty.
	Provisioner string

	// parameters holds parameters for the provisioner.
	// These values are opaque to the  system and are passed directly
	// to the provisioner.  The only validation done on keys is that they are
	// not empty.  The maximum number of parameters is
	// 512, with a cumulative max size of 256K
	// +optional
	Parameters map[string]string

	// reclaimPolicy is the reclaim policy that dynamically provisioned
	// PersistentVolumes of this storage class are created with
	// +optional
	ReclaimPolicy *api.PersistentVolumeReclaimPolicy

	// mountOptions are the mount options that dynamically provisioned
	// PersistentVolumes of this storage class are created with
	// +optional
	MountOptions []string

	// AllowVolumeExpansion shows whether the storage class allow volume expand
	// If the field is nil or not set, it would amount to expansion disabled
	// for all PVs created from this storageclass.
	// +optional
	AllowVolumeExpansion *bool

	// VolumeBindingMode indicates how PersistentVolumeClaims should be
	// provisioned and bound.  When unset, VolumeBindingImmediate is used.
	// This field is only honored by servers that enable the VolumeScheduling feature.
	// +optional
	VolumeBindingMode *VolumeBindingMode

	// Restrict the node topologies where volumes can be dynamically provisioned.
	// Each volume plugin defines its own supported topology specifications.
	// An empty TopologySelectorTerm list means there is no topology restriction.
	// This field is only honored by servers that enable the VolumeScheduling feature.
	// +optional
	AllowedTopologies []api.TopologySelectorTerm
}
```

The expectation (in my understanding) from the `ResourceClass` is to provide a `complete` or "nearly" `complete` resource definition.

# Challenges
It appears from our discussions the most (if not entire) disagreement is about how we classify some of the  `ResourceInstance`, i.e.
weather or not the given attribute signifies "definition", "usage", or both. 

In that spirit, base on our last discussion, it appears we have proposed that the `ResourceClass` may not contain a **complete** set of 
attributes. Even more so, the union of `ResourceClass` and `AbstractInstance` may not produce the complete set of the attributes, and
ultimately it is up to the provisioner to deal with the `attributes` interpretation. 

1. I think we can make a strong case for the `ResourceClass` to provide a complete definition for a given `ResourceInstance`.

Pros:
- it provides the cleanest separation between the `AbstractResource` and `ResourceClass`, where:
    - `ResourceClass` defines all (every single one) attributes
    - `AbstractClass` deals with the usage (secret/services) + binding/matching for static defined `ResourceInstances`
- it provides the cleanest application integration model: as application developer all I need to know/provide is the
`className` that corresponds to the resources that satisfies my dependency. This is true in cases with 
    - `PVC`+`StorageClass`: give me the `storage volume` of the `standard` class - and I don't even care what "standard" may be.
    - `MySQLInstance`: give me the `MySQL` instance of the `standard` class - and I don't even care ... (same as above)
    
    In both cases, I know how to consume or how to customize consumption, but I do not provide any additional information as pertains to 
    resource definition
    
Cons:
- it could be **too** restrictive, i.e. the cluster administrator bears the full burden of defining each and every `ResourceClass`, 
whereas application owner has no say on how to further customize the resource. 
    - Note: I could (and did) see use cases where this "con" was the exact goal/motive behind the separation of concerns. 

1. I think we can make a case for loose attribute completeness in both `ResourceClass` and `AbstractInstance`, however, as long as we 
clearly define the authority of the values. From our last discussion, it appears there is the option that the ultimate authority 
should lie with the provisioner implementation

Pros: 
- Administrator does not have to provide an exhaustive definition for the `ResourceInstance` and instead has to fill in only the "important" attributes.
- Application owner can chose to provide some attributes he/she thinks are important, granted that any collisions with attributes defined in the  
`ResourceClass` will be resolved in favor of the `ResourceClass` 
    - **Note** this last point wasn't entirely clear after yesterdays meeting, and would be nice to confirm.

Cons:
- There is no definition way to say what to expect in term of the resulting `ResourceInstance` from the "incomplete" definition 
of both `ResourceCalss` and `AbstractInstance`.
- There is "strong" reliance on "documentation" as for the behaviour on what to expect in terms of:
    - incomplete definition 
    - collided values 
    - default behaviour provided by the `provisioner` implementation.
    
Separate Cons on delegating properties interpreting logic to the provisioner implementation:
- Opaque: the application owner expected to know the implementation details of a given provider via documentation or worse, code.
- Unclear authority and inconsistency with provided definition: For example, if the application owner request the `MySQLInstance` for a `ResourceClass` (mysql, 5.7), but
due to the logic inside the provisioner it may get back `MySQLInstance` version 5.8, or even worse `PostgresInstance` - after all, 
it is entirely up to provisioner implementation to decide!

