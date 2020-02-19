# container.gcp.crossplane.io/v1alpha1 API Reference

Package v1alpha1 contains managed resources for GCP compute services such as GKE.

This API group contains the following Crossplane resources:

* [NodePool](#NodePool)
* [NodePoolClass](#NodePoolClass)

## NodePool

A NodePool is a managed resource that represents a Google Kubernetes Engine node pool.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `container.gcp.crossplane.io/v1alpha1`
`kind` | string | `NodePool`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [NodePoolSpec](#NodePoolSpec) | A NodePoolSpec defines the desired state of a NodePool.
`status` | [NodePoolStatus](#NodePoolStatus) | A NodePoolStatus represents the observed state of a NodePool.



## NodePoolClass

A NodePoolClass is a resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `container.gcp.crossplane.io/v1alpha1`
`kind` | string | `NodePoolClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [NodePoolClassSpecTemplate](#NodePoolClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned NodePool.



## AcceleratorConfig

AcceleratorConfig represents a Hardware Accelerator request.


Name | Type | Description
-----|------|------------
`acceleratorCount,omitempty,string` | int64 | AcceleratorCount: The number of the accelerator cards exposed to an instance.
`acceleratorType` | string | AcceleratorType: The accelerator type resource name. List of supported accelerators [here](/compute/docs/gpus/#Introduction)



## AutoUpgradeOptions

AutoUpgradeOptions defines the set of options for the user to control how the Auto Upgrades will proceed.

Appears in:

* [NodeManagementStatus](#NodeManagementStatus)


Name | Type | Description
-----|------|------------
`autoUpgradeStartTime` | string | AutoUpgradeStartTime: This field is set when upgrades are about to commence with the approximate start time for the upgrades, in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) text format.
`description` | string | Description: This field is set when upgrades are about to commence with the description of the upgrade.



## GKEClusterURIReferencerForNodePool

GKEClusterURIReferencerForNodePool resolves references from a NodePool to a GKECluster by returning the referenced GKECluster&#39;s resource link, e.g. projects/projectID/locations/clusterLocation/clusters/clusterName.

Appears in:

* [NodePoolParameters](#NodePoolParameters)




GKEClusterURIReferencerForNodePool supports all fields of:

* github.com/crossplane/stack-gcp/apis/container/v1beta1.GKEClusterURIReferencer


## NodeConfig

NodeConfig is parameters that describe the nodes in a cluster.

Appears in:

* [NodePoolParameters](#NodePoolParameters)


Name | Type | Description
-----|------|------------
`accelerators` | [[]*github.com/crossplane/stack-gcp/apis/container/v1alpha1.AcceleratorConfig](#*github.com/crossplane/stack-gcp/apis/container/v1alpha1.AcceleratorConfig) | Accelerators: A list of hardware accelerators to be attached to each node. See https://cloud.google.com/compute/docs/gpus for more information about support for GPUs.
`diskSizeGb` | Optional int64 | DiskSizeGb: Size of the disk attached to each node, specified in GB. The smallest allowed disk size is 10GB.  If unspecified, the default disk size is 100GB.
`diskType` | Optional string | DiskType: Type of the disk attached to each node (e.g. &#39;pd-standard&#39; or &#39;pd-ssd&#39;)  If unspecified, the default disk type is &#39;pd-standard&#39;
`imageType` | Optional string | ImageType: The image type to use for this node. Note that for a given image type, the latest version of it will be used.
`labels` | Optional map[string]string | Labels: The map of Kubernetes labels (key/value pairs) to be applied to each node. These will added in addition to any default label(s) that Kubernetes may apply to the node. In case of conflict in label keys, the applied set may differ depending on the Kubernetes version -- it&#39;s best to assume the behavior is undefined and conflicts should be avoided. For more information, including usage and the valid values, see: https://kubernetes.io/docs/concepts/overview/working-with-objects /labels/
`localSsdCount` | Optional int64 | LocalSsdCount: The number of local SSD disks to be attached to the node.  The limit for this value is dependant upon the maximum number of disks available on a machine per zone. See: https://cloud.google.com/compute/docs/disks/local-ssd#local_ssd_l imits for more information.
`machineType` | Optional string | MachineType: The name of a Google Compute Engine [machine type](/compute/docs/machine-types) (e.g. `n1-standard-1`).  If unspecified, the default machine type is `n1-standard-1`.
`metadata` | Optional map[string]string | Metadata: The metadata key/value pairs assigned to instances in the cluster.  Keys must conform to the regexp [a-zA-Z0-9-_]&#43; and be less than 128 bytes in length. These are reflected as part of a URL in the metadata server. Additionally, to avoid ambiguity, keys must not conflict with any other metadata keys for the project or be one of the reserved keys:  &#34;cluster-location&#34;  &#34;cluster-name&#34;  &#34;cluster-uid&#34;  &#34;configure-sh&#34;  &#34;containerd-configure-sh&#34;  &#34;enable-oslogin&#34;  &#34;gci-ensure-gke-docker&#34;  &#34;gci-update-strategy&#34;  &#34;instance-template&#34;  &#34;kube-env&#34;  &#34;startup-script&#34;  &#34;user-data&#34;  &#34;disable-address-manager&#34;  &#34;windows-startup-script-ps1&#34;  &#34;common-psm1&#34;  &#34;k8s-node-setup-psm1&#34;  &#34;install-ssh-psm1&#34;  &#34;user-profile-psm1&#34;  &#34;serial-port-logging-enable&#34; Values are free-form strings, and only have meaning as interpreted by the image running in the instance. The only restriction placed on them is that each value&#39;s size must be less than or equal to 32 KB.  The total size of all keys and values must be less than 512 KB.
`minCpuPlatform` | Optional string | MinCpuPlatform: Minimum CPU platform to be used by this instance. The instance may be scheduled on the specified or newer CPU platform. Applicable values are the friendly names of CPU platforms, such as &lt;code&gt;minCpuPlatform: &amp;quot;Intel Haswell&amp;quot;&lt;/code&gt; or &lt;code&gt;minCpuPlatform: &amp;quot;Intel Sandy Bridge&amp;quot;&lt;/code&gt;. For more information, read [how to specify min CPU platform](https://cloud.google.com/compute/docs/instances/specify- min-cpu-platform)
`oauthScopes` | Optional []string | OauthScopes: The set of Google API scopes to be made available on all of the node VMs under the &#34;default&#34; service account.  The following scopes are recommended, but not required, and by default are not included:  * `https://www.googleapis.com/auth/compute` is required for mounting persistent storage on your nodes. * `https://www.googleapis.com/auth/devstorage.read_only` is required for communicating with **gcr.io** (the [Google Container Registry](/container-registry/)).  If unspecified, no scopes are added, unless Cloud Logging or Cloud Monitoring are enabled, in which case their required scopes will be added.
`preemptible` | Optional bool | Preemptible: Whether the nodes are created as preemptible VM instances. See: https://cloud.google.com/compute/docs/instances/preemptible for more inforamtion about preemptible VM instances.
`sandboxConfig` | Optional [SandboxConfig](#SandboxConfig) | SandboxConfig: Sandbox configuration for this node.
`serviceAccount` | Optional string | ServiceAccount: The Google Cloud Platform Service Account to be used by the node VMs. If no Service Account is specified, the &#34;default&#34; service account is used.
`shieldedInstanceConfig` | Optional [ShieldedInstanceConfig](#ShieldedInstanceConfig) | ShieldedInstanceConfig: Shielded Instance options.
`tags` | Optional []string | Tags: The list of instance tags applied to all nodes. Tags are used to identify valid sources or targets for network firewalls and are specified by the client during cluster or node pool creation. Each tag within the list must comply with RFC1035.
`taints` | Optional [[]*github.com/crossplane/stack-gcp/apis/container/v1alpha1.NodeTaint](#*github.com/crossplane/stack-gcp/apis/container/v1alpha1.NodeTaint) | Taints: List of kubernetes taints to be applied to each node.  For more information, including usage and the valid values, see: https://kubernetes.io/docs/concepts/configuration/taint-and-toler ation/
`workloadMetadataConfig` | Optional [WorkloadMetadataConfig](#WorkloadMetadataConfig) | WorkloadMetadataConfig: The workload metadata configuration for this node.



## NodeManagementSpec

NodeManagementSpec defines the desired set of node management services turned on for the node pool.

Appears in:

* [NodePoolParameters](#NodePoolParameters)


Name | Type | Description
-----|------|------------
`autoRepair` | Optional bool | AutoRepair: Whether the nodes will be automatically repaired.
`autoUpgrade` | Optional bool | AutoUpgrade: Whether the nodes will be automatically upgraded.



## NodeManagementStatus

NodeManagementStatus defines the observed set of node management services turned on for the node pool.

Appears in:

* [NodePoolObservation](#NodePoolObservation)


Name | Type | Description
-----|------|------------
`upgradeOptions` | [AutoUpgradeOptions](#AutoUpgradeOptions) | UpgradeOptions: Specifies the Auto Upgrade knobs for the node pool.



## NodePoolAutoscaling

NodePoolAutoscaling contains information required by cluster autoscaler to adjust the size of the node pool to the current cluster usage.

Appears in:

* [NodePoolParameters](#NodePoolParameters)


Name | Type | Description
-----|------|------------
`autoprovisioned` | Optional bool | Autoprovisioned: Can this node pool be deleted automatically.
`enabled` | Optional bool | Enabled: Is autoscaling enabled for this node pool.
`maxNodeCount` | Optional int64 | MaxNodeCount: Maximum number of nodes in the NodePool. Must be &gt;= min_node_count. There has to enough quota to scale up the cluster.
`minNodeCount` | Optional int64 | MinNodeCount: Minimum number of nodes in the NodePool. Must be &gt;= 1 and &lt;= max_node_count.



## NodePoolClassSpecTemplate

A NodePoolClassSpecTemplate is a template for the spec of a dynamically provisioned NodePool.

Appears in:

* [NodePoolClass](#NodePoolClass)




NodePoolClassSpecTemplate supports all fields of:

* [v1alpha1.ClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#classspectemplate)
* [NodePoolParameters](#NodePoolParameters)


## NodePoolObservation

NodePoolObservation is used to show the observed state of the GKE Node Pool resource on GCP.

Appears in:

* [NodePoolStatus](#NodePoolStatus)


Name | Type | Description
-----|------|------------
`conditions` | []*github.com/crossplane/stack-gcp/apis/container/v1beta1.StatusCondition | Conditions: Which conditions caused the current node pool state.
`instanceGroupUrls` | []string | InstanceGroupUrls: The resource URLs of the [managed instance groups](/compute/docs/instance-groups/creating-groups-of-mana ged-instances) associated with this node pool.
`podIpv4CidrSize` | int64 | PodIpv4CidrSize: The pod CIDR block size per node in this node pool.
`management` | [NodeManagementStatus](#NodeManagementStatus) | Management: NodeManagement configuration for this NodePool.
`selfLink` | string | SelfLink: Server-defined URL for the resource.
`status` | string | Status: The status of the nodes in this pool instance.  Possible values:   &#34;STATUS_UNSPECIFIED&#34; - Not set.   &#34;PROVISIONING&#34; - The PROVISIONING state indicates the node pool is being created.   &#34;RUNNING&#34; - The RUNNING state indicates the node pool has been created and is fully usable.   &#34;RUNNING_WITH_ERROR&#34; - The RUNNING_WITH_ERROR state indicates the node pool has been created and is partially usable. Some error state has occurred and some functionality may be impaired. Customer may need to reissue a request or trigger a new update.   &#34;RECONCILING&#34; - The RECONCILING state indicates that some work is actively being done on the node pool, such as upgrading node software. Details can be found in the `statusMessage` field.   &#34;STOPPING&#34; - The STOPPING state indicates the node pool is being deleted.   &#34;ERROR&#34; - The ERROR state indicates the node pool may be unusable. Details can be found in the `statusMessage` field.
`statusMessage` | string | StatusMessage: Additional information about the current status of this node pool instance, if available.



## NodePoolParameters

NodePoolParameters define the desired state of a Google Kubernetes Engine node pool.

Appears in:

* [NodePoolClassSpecTemplate](#NodePoolClassSpecTemplate)
* [NodePoolSpec](#NodePoolSpec)


Name | Type | Description
-----|------|------------
`cluster` | string | Cluster: The resource link for the GKE cluster to which the NodePool will attach. Must be of format projects/projectID/locations/clusterLocation/clusters/clusterName. Must be supplied if ClusterRef is not.
`clusterRef` | Optional [GKEClusterURIReferencerForNodePool](#GKEClusterURIReferencerForNodePool) | ClusterRef sets the Cluster field by resolving the resource link of the referenced Crossplane GKECluster managed resource. Must be supplied in Cluster is not.
`autoscaling` | [NodePoolAutoscaling](#NodePoolAutoscaling) | Autoscaling: Autoscaler configuration for this NodePool. Autoscaler is enabled only if a valid configuration is present.
`config` | [NodeConfig](#NodeConfig) | Config: The node configuration of the pool.
`initialNodeCount` | Optional int64 | InitialNodeCount: The initial node count for the pool. You must ensure that your Compute Engine &lt;a href=&#34;/compute/docs/resource-quotas&#34;&gt;resource quota&lt;/a&gt; is sufficient for this number of instances. You must also have available firewall and routes quota.
`locations` | Optional []string | Locations: The list of Google Compute Engine [zones](/compute/docs/zones#available) in which the NodePool&#39;s nodes should be located.
`management` | [NodeManagementSpec](#NodeManagementSpec) | Management: NodeManagement configuration for this NodePool.
`maxPodsConstraint` | github.com/crossplane/stack-gcp/apis/container/v1beta1.MaxPodsConstraint | MaxPodsConstraint: The constraint on the maximum number of pods that can be run simultaneously on a node in the node pool.
`version` | Optional string | Version: The version of the Kubernetes of this node.



## NodePoolSpec

A NodePoolSpec defines the desired state of a NodePool.

Appears in:

* [NodePool](#NodePool)


Name | Type | Description
-----|------|------------
`forProvider` | [NodePoolParameters](#NodePoolParameters) | NodePoolParameters define the desired state of a Google Kubernetes Engine node pool.


NodePoolSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## NodePoolStatus

A NodePoolStatus represents the observed state of a NodePool.

Appears in:

* [NodePool](#NodePool)


Name | Type | Description
-----|------|------------
`atProvider` | [NodePoolObservation](#NodePoolObservation) | NodePoolObservation is used to show the observed state of the GKE Node Pool resource on GCP.


NodePoolStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## NodeTaint

NodeTaint is a Kubernetes taint is comprised of three fields: key, value, and effect. Effect can only be one of three types:  NoSchedule, PreferNoSchedule or NoExecute.  For more information, including usage and the valid values, see: https://kubernetes.io/docs/concepts/configuration/taint-and-toler ation/


Name | Type | Description
-----|------|------------
`effect` | string | Effect: Effect for taint.  Possible values:   &#34;EFFECT_UNSPECIFIED&#34; - Not set   &#34;NO_SCHEDULE&#34; - NoSchedule   &#34;PREFER_NO_SCHEDULE&#34; - PreferNoSchedule   &#34;NO_EXECUTE&#34; - NoExecute
`key` | string | Key: Key for taint.
`value` | string | Value: Value for taint.



## SandboxConfig

SandboxConfig contains configurations of the sandbox to use for the node.

Appears in:

* [NodeConfig](#NodeConfig)


Name | Type | Description
-----|------|------------
`sandboxType` | string | SandboxType: Type of the sandbox to use for the node (e.g. &#39;gvisor&#39;)



## ShieldedInstanceConfig

ShieldedInstanceConfig is a set of Shielded Instance options.

Appears in:

* [NodeConfig](#NodeConfig)


Name | Type | Description
-----|------|------------
`enableIntegrityMonitoring` | Optional bool | EnableIntegrityMonitoring: Defines whether the instance has integrity monitoring enabled.  Enables monitoring and attestation of the boot integrity of the instance. The attestation is performed against the integrity policy baseline. This baseline is initially derived from the implicitly trusted boot image when the instance is created.
`enableSecureBoot` | Optional bool | EnableSecureBoot: Defines whether the instance has Secure Boot enabled.  Secure Boot helps ensure that the system only runs authentic software by verifying the digital signature of all boot components, and halting the boot process if signature verification fails.



## WorkloadMetadataConfig

WorkloadMetadataConfig defines the metadata configuration to expose to workloads on the node pool.

Appears in:

* [NodeConfig](#NodeConfig)


Name | Type | Description
-----|------|------------
`nodeMetadata` | string | NodeMetadata: NodeMetadata is the configuration for how to expose metadata to the workloads running on the node.  Possible values:   &#34;UNSPECIFIED&#34; - Not set.   &#34;SECURE&#34; - Prevent workloads not in hostGKECluster from accessing certain VM metadata, specifically kube-env, which contains Kubelet credentials, and the instance identity token.  Metadata concealment is a temporary security solution available while the bootstrapping process for cluster nodes is being redesigned with significant security improvements.  This feature is scheduled to be deprecated in the future and later removed.   &#34;EXPOSE&#34; - Expose all VM metadata to pods.   &#34;GKE_METADATA_SERVER&#34; - Run the GKE Metadata Server on this node. The GKE Metadata Server exposes a metadata API to workloads that is compatible with the V1 Compute Metadata APIs exposed by the Compute Engine and App Engine Metadata Servers. This feature can only be enabled if Workload Identity is enabled at the cluster level.



This API documentation was generated by `crossdocs`.