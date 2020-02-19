# container.gcp.crossplane.io/v1beta1 API Reference

Package v1beta1 contains managed resources for GCP compute services such as GKE.

This API group contains the following Crossplane resources:

* [GKECluster](#GKECluster)
* [GKEClusterClass](#GKEClusterClass)

## GKECluster

A GKECluster is a managed resource that represents a Google Kubernetes Engine cluster.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `container.gcp.crossplane.io/v1beta1`
`kind` | string | `GKECluster`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [GKEClusterSpec](#GKEClusterSpec) | A GKEClusterSpec defines the desired state of a GKECluster.
`status` | [GKEClusterStatus](#GKEClusterStatus) | A GKEClusterStatus represents the observed state of a GKECluster.



## GKEClusterClass

A GKEClusterClass is a resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `container.gcp.crossplane.io/v1beta1`
`kind` | string | `GKEClusterClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [GKEClusterClassSpecTemplate](#GKEClusterClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned GKECluster.



## AcceleratorConfigClusterStatus

AcceleratorConfigClusterStatus represents a Hardware Accelerator request.


Name | Type | Description
-----|------|------------
`acceleratorCount,omitempty,string` | int64 | AcceleratorCount: The number of the accelerator cards exposed to an instance.
`acceleratorType` | string | AcceleratorType: The accelerator type resource name. List of supported accelerators [here](/compute/docs/gpus/#Introduction)



## AddonsConfig

AddonsConfig is configuration for the addons that can be automatically spun up in the cluster, enabling additional functionality.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`cloudRunConfig` | [CloudRunConfig](#CloudRunConfig) | CloudRunConfig: Configuration for the Cloud Run addon. The `IstioConfig` addon must be enabled in order to enable Cloud Run addon. This option can only be enabled at cluster creation time.
`horizontalPodAutoscaling` | [HorizontalPodAutoscaling](#HorizontalPodAutoscaling) | HorizontalPodAutoscaling: Configuration for the horizontal pod autoscaling feature, which increases or decreases the number of replica pods a replication controller has based on the resource usage of the existing pods.
`httpLoadBalancing` | [HTTPLoadBalancing](#HTTPLoadBalancing) | HTTpLoadBalancing: Configuration for the HTTP (L7) load balancing controller addon, which makes it easy to set up HTTP load balancers for services in a cluster.
`istioConfig` | [IstioConfig](#IstioConfig) | IstioConfig: Configuration for Istio, an open platform to connect, manage, and secure microservices.
`kubernetesDashboard` | [KubernetesDashboard](#KubernetesDashboard) | KubernetesDashboard: Configuration for the Kubernetes Dashboard. This addon is deprecated, and will be disabled in 1.15. It is recommended to use the Cloud Console to manage and monitor your Kubernetes clusters, workloads and applications. For more information, see: https://cloud.google.com/kubernetes-engine/docs/concepts/dashboar ds
`networkPolicyConfig` | [NetworkPolicyConfig](#NetworkPolicyConfig) | NetworkPolicyConfig: Configuration for NetworkPolicy. This only tracks whether the addon is enabled or not on the Master, it does not track whether network policy is enabled for the nodes.



## AuthenticatorGroupsConfig

AuthenticatorGroupsConfig is configuration for returning group information from authenticators.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`enabled` | Optional bool | Enabled: Whether this cluster should return group membership lookups during authentication using a group of security groups.
`securityGroup` | Optional string | SecurityGroup: The name of the security group-of-groups to be used. Only relevant if enabled = true.



## AutoUpgradeOptionsClusterStatus

AutoUpgradeOptionsClusterStatus defines the set of options for the user to control how the Auto Upgrades will proceed.

Appears in:

* [NodeManagementClusterStatus](#NodeManagementClusterStatus)


Name | Type | Description
-----|------|------------
`autoUpgradeStartTime` | string | AutoUpgradeStartTime: This field is set when upgrades are about to commence with the approximate start time for the upgrades, in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) text format.
`description` | string | Description: This field is set when upgrades are about to commence with the description of the upgrade.



## AutoprovisioningNodePoolDefaults

AutoprovisioningNodePoolDefaults contains defaults for a node pool created by NAP.

Appears in:

* [ClusterAutoscaling](#ClusterAutoscaling)


Name | Type | Description
-----|------|------------
`oauthScopes` | []string | OauthScopes: Scopes that are used by NAP when creating node pools. If oauth_scopes are specified, service_account should be empty.
`serviceAccount` | Optional string | ServiceAccount: The Google Cloud Platform Service Account to be used by the node VMs. If service_account is specified, scopes should be empty.



## BigQueryDestination

BigQueryDestination is parameters for using BigQuery as the destination of resource usage export.

Appears in:

* [ResourceUsageExportConfig](#ResourceUsageExportConfig)


Name | Type | Description
-----|------|------------
`datasetId` | string | DatasetId: The ID of a BigQuery Dataset.



## BinaryAuthorization

BinaryAuthorization is configuration for Binary Authorization.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`enabled` | bool | Enabled: Enable Binary Authorization for this cluster. If enabled, all container images will be validated by Google Binauthz.



## CidrBlock

CidrBlock contains an optional name and one CIDR block.


Name | Type | Description
-----|------|------------
`cidrBlock` | string | CidrBlock: cidr_block must be specified in CIDR notation.
`displayName` | Optional string | DisplayName: display_name is an optional field for users to identify CIDR blocks.



## ClientCertificateConfig

ClientCertificateConfig is configuration for client certificates on the cluster.

Appears in:

* [MasterAuth](#MasterAuth)


Name | Type | Description
-----|------|------------
`issueClientCertificate` | bool | IssueClientCertificate: Issue a client certificate.



## CloudRunConfig

CloudRunConfig is configuration options for the Cloud Run feature.

Appears in:

* [AddonsConfig](#AddonsConfig)


Name | Type | Description
-----|------|------------
`disabled` | bool | Disabled: Whether Cloud Run addon is enabled for this cluster.



## ClusterAutoscaling

ClusterAutoscaling contains global, per-cluster information required by Cluster Autoscaler to automatically adjust the size of the cluster and create/delete node pools based on the current needs.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`autoprovisioningLocations` | []string | AutoprovisioningLocations: The list of Google Compute Engine [zones](/compute/docs/zones#available) in which the NodePool&#39;s nodes can be created by NAP.
`autoprovisioningNodePoolDefaults` | [AutoprovisioningNodePoolDefaults](#AutoprovisioningNodePoolDefaults) | AutoprovisioningNodePoolDefaults: AutoprovisioningNodePoolDefaults contains defaults for a node pool created by NAP.
`enableNodeAutoprovisioning` | Optional bool | EnableNodeAutoprovisioning: Enables automatic node pool creation and deletion.
`resourceLimits` | [[]*github.com/crossplane/stack-gcp/apis/container/v1beta1.ResourceLimit](#*github.com/crossplane/stack-gcp/apis/container/v1beta1.ResourceLimit) | ResourceLimits: Contains global constraints regarding minimum and maximum amount of resources in the cluster.



## ConsumptionMeteringConfig

ConsumptionMeteringConfig is parameters for controlling consumption metering.

Appears in:

* [ResourceUsageExportConfig](#ResourceUsageExportConfig)


Name | Type | Description
-----|------|------------
`enabled` | bool | Enabled: Whether to enable consumption metering for this cluster. If enabled, a second BigQuery table will be created to hold resource consumption records.



## DailyMaintenanceWindowSpec

DailyMaintenanceWindowSpec is the time window specified for daily maintenance operations.

Appears in:

* [MaintenanceWindowSpec](#MaintenanceWindowSpec)


Name | Type | Description
-----|------|------------
`startTime` | string | StartTime: Time within the maintenance window to start the maintenance operations. Time format should be in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) format &#34;HH:MM&#34;, where HH : [00-23] and MM : [00-59] GMT.



## DailyMaintenanceWindowStatus

DailyMaintenanceWindowStatus is the observed time window for daily maintenance operations.

Appears in:

* [MaintenanceWindowStatus](#MaintenanceWindowStatus)


Name | Type | Description
-----|------|------------
`duration` | string | Duration: Duration of the time window, automatically chosen to be smallest possible in the given scenario. Duration will be in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) format &#34;PTnHnMnS&#34;.



## DatabaseEncryption

DatabaseEncryption is configuration of etcd encryption.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`keyName` | Optional string | KeyName: Name of CloudKMS key to use for the encryption of secrets in etcd. Ex. projects/my-project/locations/global/keyRings/my-ring/cryptoKeys/my-ke y
`state` | Optional string | State: Denotes the state of etcd encryption.  Possible values:   &#34;UNKNOWN&#34; - Should never be set   &#34;ENCRYPTED&#34; - Secrets in etcd are encrypted.   &#34;DECRYPTED&#34; - Secrets in etcd are stored in plain text (at etcd level) - this is unrelated to Google Compute Engine level full disk encryption.



## GKEClusterClassSpecTemplate

A GKEClusterClassSpecTemplate is a template for the spec of a dynamically provisioned GKECluster.

Appears in:

* [GKEClusterClass](#GKEClusterClass)


Name | Type | Description
-----|------|------------
`forProvider` | [GKEClusterParameters](#GKEClusterParameters) | GKEClusterParameters define the desired state of a Google Kubernetes Engine cluster. Most of its fields are direct mirror of GCP Cluster object. See https://cloud.google.com/kubernetes-engine/docs/reference/rest/v1/projects.locations.clusters#Cluster


GKEClusterClassSpecTemplate supports all fields of:

* [v1alpha1.ClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#classspectemplate)


## GKEClusterObservation

GKEClusterObservation is used to show the observed state of the GKE cluster resource on GCP.

Appears in:

* [GKEClusterStatus](#GKEClusterStatus)


Name | Type | Description
-----|------|------------
`conditions` | [[]*github.com/crossplane/stack-gcp/apis/container/v1beta1.StatusCondition](#*github.com/crossplane/stack-gcp/apis/container/v1beta1.StatusCondition) | Conditions: Which conditions caused the current cluster state.
`createTime` | string | CreateTime: The time the cluster was created, in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) text format.
`currentMasterVersion` | string | CurrentMasterVersion: The current software version of the master endpoint.
`currentNodeCount` | int64 | CurrentNodeCount:  The number of nodes currently in the cluster. Deprecated. Call Kubernetes API directly to retrieve node information.
`currentNodeVersion` | string | CurrentNodeVersion: Deprecated, use [NodePools.version](/kubernetes-engine/docs/reference/rest/v1/proj ects.zones.clusters.nodePools) instead. The current version of the node software components. If they are currently at multiple versions because they&#39;re in the process of being upgraded, this reflects the minimum version of all nodes.
`endpoint` | string | Endpoint: The IP address of this cluster&#39;s master endpoint. The endpoint can be accessed from the internet at `https://username:password@endpoint/`.  See the `masterAuth` property of this resource for username and password information.
`expireTime` | string | ExpireTime: The time the cluster will be automatically deleted in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) text format.
`location` | string | Location: The name of the Google Compute Engine [zone](/compute/docs/regions-zones/regions-zones#available) or [region](/compute/docs/regions-zones/regions-zones#available) in which the cluster resides.
`maintenancePolicy` | [MaintenancePolicyStatus](#MaintenancePolicyStatus) | MaintenancePolicy: Configure the maintenance policy for this cluster.
`networkConfig` | [NetworkConfigStatus](#NetworkConfigStatus) | NetworkConfig: Configuration for cluster networking.
`nodeIpv4CidrSize` | int64 | NodeIpv4CidrSize: The size of the address space on each node for hosting containers. This is provisioned from within the `container_ipv4_cidr` range. This field will only be set when cluster is in route-based network mode.
`privateClusterConfig` | [PrivateClusterConfigStatus](#PrivateClusterConfigStatus) | PrivateClusterConfig: Configuration for private cluster.
`nodePools` | [[]*github.com/crossplane/stack-gcp/apis/container/v1beta1.NodePoolClusterStatus](#*github.com/crossplane/stack-gcp/apis/container/v1beta1.NodePoolClusterStatus) | NodePools: The node pools associated with this cluster. This field should not be set if &#34;node_config&#34; or &#34;initial_node_count&#34; are specified.
`selfLink` | string | SelfLink: Server-defined URL for the resource.
`servicesIpv4Cidr` | string | ServicesIpv4Cidr: The IP address range of the Kubernetes services in this cluster, in [CIDR](http://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing)  notation (e.g. `1.2.3.4/29`). Service addresses are typically put in the last `/16` from the container CIDR.
`status` | string | Status: The current status of this cluster.  Possible values:   &#34;STATUS_UNSPECIFIED&#34; - Not set.   &#34;PROVISIONING&#34; - The PROVISIONING state indicates the cluster is being created.   &#34;RUNNING&#34; - The RUNNING state indicates the cluster has been created and is fully usable.   &#34;RECONCILING&#34; - The RECONCILING state indicates that some work is actively being done on the cluster, such as upgrading the master or node software. Details can be found in the `statusMessage` field.   &#34;STOPPING&#34; - The STOPPING state indicates the cluster is being deleted.   &#34;ERROR&#34; - The ERROR state indicates the cluster may be unusable. Details can be found in the `statusMessage` field.   &#34;DEGRADED&#34; - The DEGRADED state indicates the cluster requires user action to restore full functionality. Details can be found in the `statusMessage` field.
`statusMessage` | string | StatusMessage: Additional information about the current status of this cluster, if available.
`tpuIpv4CidrBlock` | string | TpuIpv4CidrBlock: The IP address range of the Cloud TPUs in this cluster, in [CIDR](http://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing)  notation (e.g. `1.2.3.4/29`).
`zone` | string | Zone: The name of the Google Compute Engine [zone](/compute/docs/zones#available) in which the cluster resides. This field is deprecated, use location instead.



## GKEClusterParameters

GKEClusterParameters define the desired state of a Google Kubernetes Engine cluster. Most of its fields are direct mirror of GCP Cluster object. See https://cloud.google.com/kubernetes-engine/docs/reference/rest/v1/projects.locations.clusters#Cluster

Appears in:

* [GKEClusterClassSpecTemplate](#GKEClusterClassSpecTemplate)
* [GKEClusterSpec](#GKEClusterSpec)


Name | Type | Description
-----|------|------------
`location` | string | Location: The name of the Google Compute Engine [zone](/compute/docs/regions-zones/regions-zones#available) or [region](/compute/docs/regions-zones/regions-zones#available) in which the cluster resides.
`addonsConfig` | Optional [AddonsConfig](#AddonsConfig) | AddonsConfig: Configurations for the various addons available to run in the cluster.
`authenticatorGroupsConfig` | Optional [AuthenticatorGroupsConfig](#AuthenticatorGroupsConfig) | AuthenticatorGroupsConfig: Configuration controlling RBAC group membership information.
`autoscaling` | Optional [ClusterAutoscaling](#ClusterAutoscaling) | Autoscaling: Cluster-level autoscaling configuration.
`binaryAuthorization` | Optional [BinaryAuthorization](#BinaryAuthorization) | BinaryAuthorization: Configuration for Binary Authorization.
`clusterIpv4Cidr` | Optional string | ClusterIpv4Cidr: The IP address range of the container pods in this cluster, in [CIDR](http://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing)  notation (e.g. `10.96.0.0/14`). Leave blank to have one automatically chosen or specify a `/14` block in `10.0.0.0/8`.
`databaseEncryption` | Optional [DatabaseEncryption](#DatabaseEncryption) | DatabaseEncryption: Configuration of etcd encryption.
`defaultMaxPodsConstraint` | Optional [MaxPodsConstraint](#MaxPodsConstraint) | DefaultMaxPodsConstraint: The default constraint on the maximum number of pods that can be run simultaneously on a node in the node pool of this cluster. Only honored if cluster created with IP Alias support.
`description` | Optional string | Description: An optional description of this cluster.
`enableKubernetesAlpha` | Optional bool | EnableKubernetesAlpha: Kubernetes alpha features are enabled on this cluster. This includes alpha API groups (e.g. v1alpha1) and features that may not be production ready in the kubernetes version of the master and nodes. The cluster has no SLA for uptime and master/node upgrades are disabled. Alpha enabled clusters are automatically deleted thirty days after creation.
`enableTpu` | Optional bool | EnableTpu: Enable the ability to use Cloud TPUs in this cluster.
`initialClusterVersion` | Optional string | InitialClusterVersion: The initial Kubernetes version for this cluster.  Valid versions are those found in validMasterVersions returned by getServerConfig.  The version can be upgraded over time; such upgrades are reflected in currentMasterVersion and currentNodeVersion.  Users may specify either explicit versions offered by Kubernetes Engine or version aliases, which have the following behavior:  - &#34;latest&#34;: picks the highest valid Kubernetes version - &#34;1.X&#34;: picks the highest valid patch&#43;gke.N patch in the 1.X version - &#34;1.X.Y&#34;: picks the highest valid gke.N patch in the 1.X.Y version - &#34;1.X.Y-gke.N&#34;: picks an explicit Kubernetes version - &#34;&#34;,&#34;-&#34;: picks the default Kubernetes version
`ipAllocationPolicy` | Optional [IPAllocationPolicy](#IPAllocationPolicy) | IPAllocationPolicy: Configuration for cluster IP allocation.
`labelFingerprint` | Optional string | LabelFingerprint: The fingerprint of the set of labels for this cluster.
`legacyAbac` | Optional [LegacyAbac](#LegacyAbac) | LegacyAbac: Configuration for the legacy ABAC authorization mode.
`locations` | Optional []string | Locations: The list of Google Compute Engine [zones](/compute/docs/zones#available) in which the cluster&#39;s nodes should be located.
`loggingService` | Optional string | LoggingService: The logging service the cluster should use to write logs. Currently available options:  * &#34;logging.googleapis.com/kubernetes&#34; - the Google Cloud Logging service with Kubernetes-native resource model in Stackdriver * `logging.googleapis.com` - the Google Cloud Logging service. * `none` - no logs will be exported from the cluster. * if left as an empty string,`logging.googleapis.com` will be used.
`maintenancePolicy` | Optional [MaintenancePolicySpec](#MaintenancePolicySpec) | MaintenancePolicy: Configure the maintenance policy for this cluster.
`masterAuth` | Optional [MasterAuth](#MasterAuth) | MasterAuth: The authentication information for accessing the master endpoint. If unspecified, the defaults are used: For clusters before v1.12, if master_auth is unspecified, `username` will be set to &#34;admin&#34;, a random password will be generated, and a client certificate will be issued.
`masterAuthorizedNetworksConfig` | Optional [MasterAuthorizedNetworksConfig](#MasterAuthorizedNetworksConfig) | MasterAuthorizedNetworksConfig: The configuration options for master authorized networks feature.
`monitoringService` | Optional string | MonitoringService: The monitoring service the cluster should use to write metrics. Currently available options:  * `monitoring.googleapis.com` - the Google Cloud Monitoring service. * `none` - no metrics will be exported from the cluster. * if left as an empty string, `monitoring.googleapis.com` will be used.
`network` | Optional string | Network: The name of the Google Compute Engine [network](/compute/docs/networks-and-firewalls#networks) to which the cluster is connected. If left unspecified, the `default` network will be used.
`networkRef` | Optional [NetworkURIReferencerForGKECluster](#NetworkURIReferencerForGKECluster) | NetworkRef references to a Network and retrieves its URI
`networkConfig` | Optional [NetworkConfigSpec](#NetworkConfigSpec) | NetworkConfig: Configuration for cluster networking.
`networkPolicy` | Optional [NetworkPolicy](#NetworkPolicy) | NetworkPolicy: Configuration options for the NetworkPolicy feature.
`podSecurityPolicyConfig` | Optional [PodSecurityPolicyConfig](#PodSecurityPolicyConfig) | PodSecurityPolicyConfig: Configuration for the PodSecurityPolicy feature.
`privateClusterConfig` | Optional [PrivateClusterConfigSpec](#PrivateClusterConfigSpec) | PrivateClusterConfig: Configuration for private cluster.
`resourceLabels` | Optional map[string]string | ResourceLabels: The resource labels for the cluster to use to annotate any related Google Compute Engine resources.
`resourceUsageExportConfig` | Optional [ResourceUsageExportConfig](#ResourceUsageExportConfig) | ResourceUsageExportConfig: Configuration for exporting resource usages. Resource usage export is disabled when this config is unspecified.
`subnetwork` | Optional string | Subnetwork: The name of the Google Compute Engine [subnetwork](/compute/docs/subnetworks) to which the cluster is connected.
`subnetworkRef` | Optional [SubnetworkURIReferencerForGKECluster](#SubnetworkURIReferencerForGKECluster) | SubnetworkRef references to a Subnetwork and retrieves its URI
`tierSettings` | Optional [TierSettings](#TierSettings) | TierSettings: Cluster tier settings.
`verticalPodAutoscaling` | Optional [VerticalPodAutoscaling](#VerticalPodAutoscaling) | VerticalPodAutoscaling: Cluster-level Vertical Pod Autoscaling configuration.
`workloadIdentityConfig` | Optional [WorkloadIdentityConfig](#WorkloadIdentityConfig) | WorkloadIdentityConfig: Configuration for the use of Kubernetes Service Accounts in GCP IAM policies.



## GKEClusterSpec

A GKEClusterSpec defines the desired state of a GKECluster.

Appears in:

* [GKECluster](#GKECluster)


Name | Type | Description
-----|------|------------
`forProvider` | [GKEClusterParameters](#GKEClusterParameters) | GKEClusterParameters define the desired state of a Google Kubernetes Engine cluster. Most of its fields are direct mirror of GCP Cluster object. See https://cloud.google.com/kubernetes-engine/docs/reference/rest/v1/projects.locations.clusters#Cluster


GKEClusterSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## GKEClusterStatus

A GKEClusterStatus represents the observed state of a GKECluster.

Appears in:

* [GKECluster](#GKECluster)


Name | Type | Description
-----|------|------------
`atProvider` | [GKEClusterObservation](#GKEClusterObservation) | GKEClusterObservation is used to show the observed state of the GKE cluster resource on GCP.


GKEClusterStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## GKEClusterURIReferencer

GKEClusterURIReferencer retrieves a GKEClusterURI from a referenced GKECluster object




GKEClusterURIReferencer supports all fields of:

* [core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core)


## HTTPLoadBalancing

HTTPLoadBalancing is configuration options for the HTTP (L7) load balancing controller addon, which makes it easy to set up HTTP load balancers for services in a cluster.

Appears in:

* [AddonsConfig](#AddonsConfig)


Name | Type | Description
-----|------|------------
`disabled` | bool | Disabled: Whether the HTTP Load Balancing controller is enabled in the cluster. When enabled, it runs a small pod in the cluster that manages the load balancers.



## HorizontalPodAutoscaling

HorizontalPodAutoscaling is configuration options for the horizontal pod autoscaling feature, which increases or decreases the number of replica pods a replication controller has based on the resource usage of the existing pods.

Appears in:

* [AddonsConfig](#AddonsConfig)


Name | Type | Description
-----|------|------------
`disabled` | bool | Disabled: Whether the Horizontal Pod Autoscaling feature is enabled in the cluster. When enabled, it ensures that a Heapster pod is running in the cluster, which is also used by the Cloud Monitoring service.



## IPAllocationPolicy

IPAllocationPolicy is configuration for controlling how IPs are allocated in the cluster.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`allowRouteOverlap` | bool | AllowRouteOverlap: If true, allow allocation of cluster CIDR ranges that overlap with certain kinds of network routes. By default we do not allow cluster CIDR ranges to intersect with any user declared routes. With allow_route_overlap == true, we allow overlapping with CIDR ranges that are larger than the cluster CIDR range.  If this field is set to true, then cluster and services CIDRs must be fully-specified (e.g. `10.96.0.0/14`, but not `/14`), which means: 1) When `use_ip_aliases` is true, `cluster_ipv4_cidr_block` and    `services_ipv4_cidr_block` must be fully-specified. 2) When `use_ip_aliases` is false, `cluster.cluster_ipv4_cidr` muse be    fully-specified.
`clusterIpv4CidrBlock` | Optional string | ClusterIpv4CidrBlock: The IP address range for the cluster pod IPs. If this field is set, then `cluster.cluster_ipv4_cidr` must be left blank.  This field is only applicable when `use_ip_aliases` is true.  Set to blank to have a range chosen with the default size.  Set to /netmask (e.g. `/14`) to have a range chosen with a specific netmask.  Set to a [CIDR](http://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing)  notation (e.g. `10.96.0.0/14`) from the RFC-1918 private networks (e.g. `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`) to pick a specific range to use.
`clusterSecondaryRangeName` | Optional string | ClusterSecondaryRangeName: The name of the secondary range to be used for the cluster CIDR block.  The secondary range will be used for pod IP addresses. This must be an existing secondary range associated with the cluster subnetwork.  This field is only applicable with use_ip_aliases is true and create_subnetwork is false.
`createSubnetwork` | Optional bool | CreateSubnetwork: Whether a new subnetwork will be created automatically for the cluster.  This field is only applicable when `use_ip_aliases` is true.
`nodeIpv4CidrBlock` | Optional string | NodeIpv4CidrBlock: The IP address range of the instance IPs in this cluster.  This is applicable only if `create_subnetwork` is true.  Set to blank to have a range chosen with the default size.  Set to /netmask (e.g. `/14`) to have a range chosen with a specific netmask.  Set to a [CIDR](http://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing)  notation (e.g. `10.96.0.0/14`) from the RFC-1918 private networks (e.g. `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`) to pick a specific range to use.
`servicesIpv4CidrBlock` | Optional string | ServicesIpv4CidrBlock: The IP address range of the services IPs in this cluster. If blank, a range will be automatically chosen with the default size.  This field is only applicable when `use_ip_aliases` is true.  Set to blank to have a range chosen with the default size.  Set to /netmask (e.g. `/14`) to have a range chosen with a specific netmask.  Set to a [CIDR](http://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing)  notation (e.g. `10.96.0.0/14`) from the RFC-1918 private networks (e.g. `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`) to pick a specific range to use.
`servicesSecondaryRangeName` | Optional string | ServicesSecondaryRangeName: The name of the secondary range to be used as for the services CIDR block.  The secondary range will be used for service ClusterIPs. This must be an existing secondary range associated with the cluster subnetwork.  This field is only applicable with use_ip_aliases is true and create_subnetwork is false.
`subnetworkName` | Optional string | SubnetworkName: A custom subnetwork name to be used if `create_subnetwork` is true.  If this field is empty, then an automatic name will be chosen for the new subnetwork.
`tpuIpv4CidrBlock` | Optional string | TpuIpv4CidrBlock: The IP address range of the Cloud TPUs in this cluster. If unspecified, a range will be automatically chosen with the default size.  This field is only applicable when `use_ip_aliases` is true.  If unspecified, the range will use the default size.  Set to /netmask (e.g. `/14`) to have a range chosen with a specific netmask.  Set to a [CIDR](http://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing)  notation (e.g. `10.96.0.0/14`) from the RFC-1918 private networks (e.g. `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`) to pick a specific range to use.
`useIpAliases` | Optional bool | UseIPAliases: Whether alias IPs will be used for pod IPs in the cluster.



## IstioConfig

IstioConfig is configuration options for Istio addon.

Appears in:

* [AddonsConfig](#AddonsConfig)


Name | Type | Description
-----|------|------------
`auth` | Optional string | Auth: The specified Istio auth mode, either none, or mutual TLS.  Possible values:   &#34;AUTH_NONE&#34; - auth not enabled   &#34;AUTH_MUTUAL_TLS&#34; - auth mutual TLS enabled
`disabled` | Optional bool | Disabled: Whether Istio is enabled for this cluster.



## KubernetesDashboard

KubernetesDashboard is configuration for the Kubernetes Dashboard.

Appears in:

* [AddonsConfig](#AddonsConfig)


Name | Type | Description
-----|------|------------
`disabled` | bool | Disabled: Whether the Kubernetes Dashboard is enabled for this cluster.



## LegacyAbac

LegacyAbac is configuration for the legacy Attribute Based Access Control authorization mode.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`enabled` | bool | Enabled: Whether the ABAC authorizer is enabled for this cluster. When enabled, identities in the system, including service accounts, nodes, and controllers, will have statically granted permissions beyond those provided by the RBAC configuration or IAM.



## MaintenancePolicySpec

MaintenancePolicySpec defines the maintenance policy to be used for the cluster.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`window` | [MaintenanceWindowSpec](#MaintenanceWindowSpec) | Window: Specifies the maintenance window in which maintenance may be performed.



## MaintenancePolicyStatus

MaintenancePolicyStatus defines the maintenance policy to be used for the cluster.

Appears in:

* [GKEClusterObservation](#GKEClusterObservation)


Name | Type | Description
-----|------|------------
`window` | [MaintenanceWindowStatus](#MaintenanceWindowStatus) | Window: Specifies the maintenance window in which maintenance may be performed.



## MaintenanceWindowSpec

MaintenanceWindowSpec defines the maintenance window to be used for the cluster.

Appears in:

* [MaintenancePolicySpec](#MaintenancePolicySpec)


Name | Type | Description
-----|------|------------
`dailyMaintenanceWindow` | [DailyMaintenanceWindowSpec](#DailyMaintenanceWindowSpec) | DailyMaintenanceWindow: DailyMaintenanceWindow specifies a daily maintenance operation window.



## MaintenanceWindowStatus

MaintenanceWindowStatus defines the maintenance window to be used for the cluster.

Appears in:

* [MaintenancePolicyStatus](#MaintenancePolicyStatus)


Name | Type | Description
-----|------|------------
`dailyMaintenanceWindow` | [DailyMaintenanceWindowStatus](#DailyMaintenanceWindowStatus) | DailyMaintenanceWindow: DailyMaintenanceWindow specifies a daily maintenance operation window.



## MasterAuth

MasterAuth is the authentication information for accessing the master endpoint. Authentication can be done using HTTP basic auth or using client certificates.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`clientCertificateConfig` | Optional [ClientCertificateConfig](#ClientCertificateConfig) | ClientCertificateConfig: Configuration for client certificate authentication on the cluster. For clusters before v1.12, if no configuration is specified, a client certificate is issued.
`username` | Optional string | Username: The username to use for HTTP basic authentication to the master endpoint. For clusters v1.6.0 and later, basic authentication can be disabled by leaving username unspecified (or setting it to the empty string).



## MasterAuthorizedNetworksConfig

MasterAuthorizedNetworksConfig is configuration options for the master authorized networks feature. Enabled master authorized networks will disallow all external traffic to access Kubernetes master through HTTPS except traffic from the given CIDR blocks, Google Compute Engine Public IPs and Google Prod IPs.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`cidrBlocks` | Optional [[]*github.com/crossplane/stack-gcp/apis/container/v1beta1.CidrBlock](#*github.com/crossplane/stack-gcp/apis/container/v1beta1.CidrBlock) | CidrBlocks: cidr_blocks define up to 50 external networks that could access Kubernetes master through HTTPS.
`enabled` | Optional bool | Enabled: Whether or not master authorized networks is enabled.



## MaxPodsConstraint

MaxPodsConstraint defines constraints applied to pods.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)
* [NodePoolClusterStatus](#NodePoolClusterStatus)


Name | Type | Description
-----|------|------------
`maxPodsPerNode` | int64 | MaxPodsPerNode: Constraint enforced on the max num of pods per node.



## NetworkConfigSpec

NetworkConfigSpec reports the relative names of network &amp; subnetwork.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`enableIntraNodeVisibility` | bool | EnableIntraNodeVisibility: Whether Intra-node visibility is enabled for this cluster. This makes same node pod to pod traffic visible for VPC network.



## NetworkConfigStatus

NetworkConfigStatus reports the relative names of network &amp; subnetwork.

Appears in:

* [GKEClusterObservation](#GKEClusterObservation)


Name | Type | Description
-----|------|------------
`network` | string | Network: The relative name of the Google Compute Engine network(/compute/docs/networks-and-firewalls#networks) to which the cluster is connected. Example: projects/my-project/global/networks/my-network
`subnetwork` | string | Subnetwork: The relative name of the Google Compute Engine [subnetwork](/compute/docs/vpc) to which the cluster is connected. Example: projects/my-project/regions/us-central1/subnetworks/my-subnet



## NetworkPolicy

NetworkPolicy is configuration options for the NetworkPolicy feature. https://kubernetes.io/docs/concepts/services-networking/netwo rkpolicies/

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`enabled` | Optional bool | Enabled: Whether network policy is enabled on the cluster.
`provider` | Optional string | Provider: The selected network policy provider.  Possible values:   &#34;PROVIDER_UNSPECIFIED&#34; - Not set   &#34;CALICO&#34; - Tigera (Calico Felix).



## NetworkPolicyConfig

NetworkPolicyConfig is configuration for NetworkPolicy. This only tracks whether the addon is enabled or not on the Master, it does not track whether network policy is enabled for the nodes.

Appears in:

* [AddonsConfig](#AddonsConfig)


Name | Type | Description
-----|------|------------
`disabled` | bool | Disabled: Whether NetworkPolicy is enabled for this cluster.



## NetworkURIReferencerForGKECluster

NetworkURIReferencerForGKECluster is an attribute referencer that resolves network uri from a referenced Network and assigns it to a GKECluster

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)




NetworkURIReferencerForGKECluster supports all fields of:

* github.com/crossplane/stack-gcp/apis/compute/v1beta1.NetworkURIReferencer


## NodeConfigClusterStatus

NodeConfigClusterStatus is the configuration of the node pool.

Appears in:

* [NodePoolClusterStatus](#NodePoolClusterStatus)


Name | Type | Description
-----|------|------------
`accelerators` | [[]*github.com/crossplane/stack-gcp/apis/container/v1beta1.AcceleratorConfigClusterStatus](#*github.com/crossplane/stack-gcp/apis/container/v1beta1.AcceleratorConfigClusterStatus) | Accelerators: A list of hardware accelerators to be attached to each node. See https://cloud.google.com/compute/docs/gpus for more information about support for GPUs.
`diskSizeGb` | int64 | DiskSizeGb: Size of the disk attached to each node, specified in GB. The smallest allowed disk size is 10GB.  If unspecified, the default disk size is 100GB.
`diskType` | string | DiskType: Type of the disk attached to each node (e.g. &#39;pd-standard&#39; or &#39;pd-ssd&#39;)  If unspecified, the default disk type is &#39;pd-standard&#39;
`imageType` | string | ImageType: The image type to use for this node. Note that for a given image type, the latest version of it will be used.
`labels` | map[string]string | Labels: The map of Kubernetes labels (key/value pairs) to be applied to each node. These will added in addition to any default label(s) that Kubernetes may apply to the node. In case of conflict in label keys, the applied set may differ depending on the Kubernetes version -- it&#39;s best to assume the behavior is undefined and conflicts should be avoided. For more information, including usage and the valid values, see: https://kubernetes.io/docs/concepts/overview/working-with-objects /labels/
`localSsdCount` | int64 | LocalSsdCount: The number of local SSD disks to be attached to the node.  The limit for this value is dependant upon the maximum number of disks available on a machine per zone. See: https://cloud.google.com/compute/docs/disks/local-ssd#local_ssd_l imits for more information.
`machineType` | string | MachineType: The name of a Google Compute Engine [machine type](/compute/docs/machine-types) (e.g. `n1-standard-1`).  If unspecified, the default machine type is `n1-standard-1`.
`metadata` | map[string]string | Metadata: The metadata key/value pairs assigned to instances in the cluster.  Keys must conform to the regexp [a-zA-Z0-9-_]&#43; and be less than 128 bytes in length. These are reflected as part of a URL in the metadata server. Additionally, to avoid ambiguity, keys must not conflict with any other metadata keys for the project or be one of the reserved keys:  &#34;cluster-location&#34;  &#34;cluster-name&#34;  &#34;cluster-uid&#34;  &#34;configure-sh&#34;  &#34;containerd-configure-sh&#34;  &#34;enable-oslogin&#34;  &#34;gci-ensure-gke-docker&#34;  &#34;gci-update-strategy&#34;  &#34;instance-template&#34;  &#34;kube-env&#34;  &#34;startup-script&#34;  &#34;user-data&#34;  &#34;disable-address-manager&#34;  &#34;windows-startup-script-ps1&#34;  &#34;common-psm1&#34;  &#34;k8s-node-setup-psm1&#34;  &#34;install-ssh-psm1&#34;  &#34;user-profile-psm1&#34;  &#34;serial-port-logging-enable&#34; Values are free-form strings, and only have meaning as interpreted by the image running in the instance. The only restriction placed on them is that each value&#39;s size must be less than or equal to 32 KB.  The total size of all keys and values must be less than 512 KB.
`minCpuPlatform` | string | MinCpuPlatform: Minimum CPU platform to be used by this instance. The instance may be scheduled on the specified or newer CPU platform. Applicable values are the friendly names of CPU platforms, such as &lt;code&gt;minCpuPlatform: &amp;quot;Intel Haswell&amp;quot;&lt;/code&gt; or &lt;code&gt;minCpuPlatform: &amp;quot;Intel Sandy Bridge&amp;quot;&lt;/code&gt;. For more information, read [how to specify min CPU platform](https://cloud.google.com/compute/docs/instances/specify- min-cpu-platform)
`oauthScopes` | []string | OauthScopes: The set of Google API scopes to be made available on all of the node VMs under the &#34;default&#34; service account.  The following scopes are recommended, but not required, and by default are not included:  * `https://www.googleapis.com/auth/compute` is required for mounting persistent storage on your nodes. * `https://www.googleapis.com/auth/devstorage.read_only` is required for communicating with **gcr.io** (the [Google Container Registry](/container-registry/)).  If unspecified, no scopes are added, unless Cloud Logging or Cloud Monitoring are enabled, in which case their required scopes will be added.
`preemptible` | bool | Preemptible: Whether the nodes are created as preemptible VM instances. See: https://cloud.google.com/compute/docs/instances/preemptible for more inforamtion about preemptible VM instances.
`sandboxConfig` | [SandboxConfigClusterStatus](#SandboxConfigClusterStatus) | SandboxConfig: Sandbox configuration for this node.
`serviceAccount` | string | ServiceAccount: The Google Cloud Platform Service Account to be used by the node VMs. If no Service Account is specified, the &#34;default&#34; service account is used.
`shieldedInstanceConfig` | [ShieldedInstanceConfigClusterStatus](#ShieldedInstanceConfigClusterStatus) | ShieldedInstanceConfig: Shielded Instance options.
`tags` | []string | Tags: The list of instance tags applied to all nodes. Tags are used to identify valid sources or targets for network firewalls and are specified by the client during cluster or node pool creation. Each tag within the list must comply with RFC1035.
`taints` | [[]*github.com/crossplane/stack-gcp/apis/container/v1beta1.NodeTaintClusterStatus](#*github.com/crossplane/stack-gcp/apis/container/v1beta1.NodeTaintClusterStatus) | Taints: List of kubernetes taints to be applied to each node.  For more information, including usage and the valid values, see: https://kubernetes.io/docs/concepts/configuration/taint-and-toler ation/
`workloadMetadataConfig` | [WorkloadMetadataConfigClusterStatus](#WorkloadMetadataConfigClusterStatus) | WorkloadMetadataConfig: The workload metadata configuration for this node.



## NodeManagementClusterStatus

NodeManagementClusterStatus defines the set of node management services turned on for the node pool.

Appears in:

* [NodePoolClusterStatus](#NodePoolClusterStatus)


Name | Type | Description
-----|------|------------
`autoRepair` | bool | AutoRepair: Whether the nodes will be automatically repaired.
`autoUpgrade` | bool | AutoUpgrade: Whether the nodes will be automatically upgraded.
`upgradeOptions` | [AutoUpgradeOptionsClusterStatus](#AutoUpgradeOptionsClusterStatus) | UpgradeOptions: Specifies the Auto Upgrade knobs for the node pool.



## NodePoolAutoscalingClusterStatus

NodePoolAutoscalingClusterStatus contains information required by cluster autoscaler to adjust the size of the node pool to the current cluster usage.

Appears in:

* [NodePoolClusterStatus](#NodePoolClusterStatus)


Name | Type | Description
-----|------|------------
`autoprovisioned` | bool | Autoprovisioned: Can this node pool be deleted automatically.
`enabled` | bool | Enabled: Is autoscaling enabled for this node pool.
`maxNodeCount` | int64 | MaxNodeCount: Maximum number of nodes in the NodePool. Must be &gt;= min_node_count. There has to enough quota to scale up the cluster.
`minNodeCount` | int64 | MinNodeCount: Minimum number of nodes in the NodePool. Must be &gt;= 1 and &lt;= max_node_count.



## NodePoolClusterStatus

NodePoolClusterStatus is a subset of information about NodePools associated with a GKE cluster.


Name | Type | Description
-----|------|------------
`autoscaling` | [NodePoolAutoscalingClusterStatus](#NodePoolAutoscalingClusterStatus) | Autoscaling: Autoscaler configuration for this NodePool. Autoscaler is enabled only if a valid configuration is present.
`conditions` | [[]*github.com/crossplane/stack-gcp/apis/container/v1beta1.StatusCondition](#*github.com/crossplane/stack-gcp/apis/container/v1beta1.StatusCondition) | Conditions: Which conditions caused the current node pool state.
`config` | [NodeConfigClusterStatus](#NodeConfigClusterStatus) | Config: The node configuration of the pool.
`initialNodeCount` | int64 | InitialNodeCount: The initial node count for the pool. You must ensure that your Compute Engine &lt;a href=&#34;/compute/docs/resource-quotas&#34;&gt;resource quota&lt;/a&gt; is sufficient for this number of instances. You must also have available firewall and routes quota.
`instanceGroupUrls` | []string | InstanceGroupUrls: The resource URLs of the [managed instance groups](/compute/docs/instance-groups/creating-groups-of-mana ged-instances) associated with this node pool.
`locations` | []string | Locations: The list of Google Compute Engine [zones](/compute/docs/zones#available) in which the NodePool&#39;s nodes should be located.
`management` | [NodeManagementClusterStatus](#NodeManagementClusterStatus) | Management: NodeManagement configuration for this NodePool.
`maxPodsConstraint` | [MaxPodsConstraint](#MaxPodsConstraint) | MaxPodsConstraint: The constraint on the maximum number of pods that can be run simultaneously on a node in the node pool.
`name` | string | Name: The name of the node pool.
`podIpv4CidrSize` | int64 | PodIpv4CidrSize: The pod CIDR block size per node in this node pool.
`selfLink` | string | SelfLink: Server-defined URL for the resource.
`status` | string | Status: The status of the nodes in this pool instance.  Possible values:   &#34;STATUS_UNSPECIFIED&#34; - Not set.   &#34;PROVISIONING&#34; - The PROVISIONING state indicates the node pool is being created.   &#34;RUNNING&#34; - The RUNNING state indicates the node pool has been created and is fully usable.   &#34;RUNNING_WITH_ERROR&#34; - The RUNNING_WITH_ERROR state indicates the node pool has been created and is partially usable. Some error state has occurred and some functionality may be impaired. Customer may need to reissue a request or trigger a new update.   &#34;RECONCILING&#34; - The RECONCILING state indicates that some work is actively being done on the node pool, such as upgrading node software. Details can be found in the `statusMessage` field.   &#34;STOPPING&#34; - The STOPPING state indicates the node pool is being deleted.   &#34;ERROR&#34; - The ERROR state indicates the node pool may be unusable. Details can be found in the `statusMessage` field.
`statusMessage` | string | StatusMessage: Additional information about the current status of this node pool instance, if available.
`version` | string | Version: The version of the Kubernetes of this node.



## NodeTaintClusterStatus

NodeTaintClusterStatus is a Kubernetes taint is comprised of three fields: key, value, and effect. Effect can only be one of three types:  NoSchedule, PreferNoSchedule or NoExecute.


Name | Type | Description
-----|------|------------
`effect` | string | Effect: Effect for taint.  Possible values:   &#34;EFFECT_UNSPECIFIED&#34; - Not set   &#34;NO_SCHEDULE&#34; - NoSchedule   &#34;PREFER_NO_SCHEDULE&#34; - PreferNoSchedule   &#34;NO_EXECUTE&#34; - NoExecute
`key` | string | Key: Key for taint.
`value` | string | Value: Value for taint.



## PodSecurityPolicyConfig

PodSecurityPolicyConfig is configuration for the PodSecurityPolicy feature.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`enabled` | bool | Enabled: Enable the PodSecurityPolicy controller for this cluster. If enabled, pods must be valid under a PodSecurityPolicy to be created.



## PrivateClusterConfigSpec

PrivateClusterConfigSpec is configuration options for private clusters.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`enablePeeringRouteSharing` | bool | EnablePeeringRouteSharing: Whether to enable route sharing over the network peering.
`enablePrivateEndpoint` | Optional bool | EnablePrivateEndpoint: Whether the master&#39;s internal IP address is used as the cluster endpoint.
`enablePrivateNodes` | Optional bool | EnablePrivateNodes: Whether nodes have internal IP addresses only. If enabled, all nodes are given only RFC 1918 private addresses and communicate with the master via private networking.
`masterIpv4CidrBlock` | Optional string | MasterIpv4CidrBlock: The IP range in CIDR notation to use for the hosted master network. This range will be used for assigning internal IP addresses to the master or set of masters, as well as the ILB VIP. This range must not overlap with any other ranges in use within the cluster&#39;s network.



## PrivateClusterConfigStatus

PrivateClusterConfigStatus is configuration options for private clusters.

Appears in:

* [GKEClusterObservation](#GKEClusterObservation)


Name | Type | Description
-----|------|------------
`privateEndpoint` | string | PrivateEndpoint: The internal IP address of this cluster&#39;s master endpoint.
`publicEndpoint` | string | PublicEndpoint: The external IP address of this cluster&#39;s master endpoint.



## ResourceLimit

ResourceLimit contains information about amount of some resource in the cluster. For memory, value should be in GB.


Name | Type | Description
-----|------|------------
`maximum` | int64 | Maximum: Maximum amount of the resource in the cluster.
`minimum` | int64 | Minimum: Minimum amount of the resource in the cluster.
`resourceType` | string | ResourceType: Resource name &#34;cpu&#34;, &#34;memory&#34; or gpu-specific string.



## ResourceUsageExportConfig

ResourceUsageExportConfig is configuration for exporting cluster resource usages.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`bigqueryDestination` | Optional [BigQueryDestination](#BigQueryDestination) | BigqueryDestination: Configuration to use BigQuery as usage export destination.
`consumptionMeteringConfig` | Optional [ConsumptionMeteringConfig](#ConsumptionMeteringConfig) | ConsumptionMeteringConfig: Configuration to enable resource consumption metering.
`enableNetworkEgressMetering` | Optional bool | EnableNetworkEgressMetering: Whether to enable network egress metering for this cluster. If enabled, a daemonset will be created in the cluster to meter network egress traffic.



## SandboxConfigClusterStatus

SandboxConfigClusterStatus contains configurations of the sandbox to use for the node.

Appears in:

* [NodeConfigClusterStatus](#NodeConfigClusterStatus)


Name | Type | Description
-----|------|------------
`sandboxType` | string | SandboxType: Type of the sandbox to use for the node (e.g. &#39;gvisor&#39;)



## ShieldedInstanceConfigClusterStatus

ShieldedInstanceConfigClusterStatus is a set of Shielded Instance options.

Appears in:

* [NodeConfigClusterStatus](#NodeConfigClusterStatus)


Name | Type | Description
-----|------|------------
`enableIntegrityMonitoring` | bool | EnableIntegrityMonitoring: Defines whether the instance has integrity monitoring enabled.  Enables monitoring and attestation of the boot integrity of the instance. The attestation is performed against the integrity policy baseline. This baseline is initially derived from the implicitly trusted boot image when the instance is created.
`enableSecureBoot` | bool | EnableSecureBoot: Defines whether the instance has Secure Boot enabled.  Secure Boot helps ensure that the system only runs authentic software by verifying the digital signature of all boot components, and halting the boot process if signature verification fails.



## StatusCondition

StatusCondition describes why a cluster or a node pool has a certain status (e.g., ERROR or DEGRADED).


Name | Type | Description
-----|------|------------
`code` | string | Code: Machine-friendly representation of the condition  Possible values:   &#34;UNKNOWN&#34; - UNKNOWN indicates a generic condition.   &#34;GCE_STOCKOUT&#34; - GCE_STOCKOUT indicates a Google Compute Engine stockout.   &#34;GKE_SERVICE_ACCOUNT_DELETED&#34; - GKE_SERVICE_ACCOUNT_DELETED indicates that the user deleted their robot service account.   &#34;GCE_QUOTA_EXCEEDED&#34; - Google Compute Engine quota was exceeded.   &#34;SET_BY_OPERATOR&#34; - Cluster state was manually changed by an SRE due to a system logic error. More codes TBA
`message` | string | Message: Human-friendly representation of the condition



## SubnetworkURIReferencerForGKECluster

SubnetworkURIReferencerForGKECluster is an attribute referencer that resolves subnetwork uri from a referenced Subnetwork and assigns it to a GKECluster

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)




SubnetworkURIReferencerForGKECluster supports all fields of:

* github.com/crossplane/stack-gcp/apis/compute/v1beta1.SubnetworkURIReferencer


## TierSettings

TierSettings is cluster tier settings.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`tier` | string | Tier: Cluster tier.  Possible values:   &#34;UNSPECIFIED&#34; - UNSPECIFIED is the default value. If this value is set during create or update, it defaults to the project level tier setting.   &#34;STANDARD&#34; - Represents the standard tier or base Google Kubernetes Engine offering.   &#34;ADVANCED&#34; - Represents the advanced tier.



## VerticalPodAutoscaling

VerticalPodAutoscaling contains global, per-cluster information required by Vertical Pod Autoscaler to automatically adjust the resources of pods controlled by it.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`enabled` | bool | Enabled: Enables vertical pod autoscaling.



## WorkloadIdentityConfig

WorkloadIdentityConfig is configuration for the use of Kubernetes Service Accounts in GCP IAM policies.

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)


Name | Type | Description
-----|------|------------
`identityNamespace` | string | IdentityNamespace: IAM Identity Namespace to attach all Kubernetes Service Accounts to.



## WorkloadMetadataConfigClusterStatus

WorkloadMetadataConfigClusterStatus defines the metadata configuration to expose to workloads on the node pool.

Appears in:

* [NodeConfigClusterStatus](#NodeConfigClusterStatus)


Name | Type | Description
-----|------|------------
`nodeMetadata` | string | NodeMetadata: NodeMetadata is the configuration for how to expose metadata to the workloads running on the node.  Possible values:   &#34;UNSPECIFIED&#34; - Not set.   &#34;SECURE&#34; - Prevent workloads not in hostNetwork from accessing certain VM metadata, specifically kube-env, which contains Kubelet credentials, and the instance identity token.  Metadata concealment is a temporary security solution available while the bootstrapping process for cluster nodes is being redesigned with significant security improvements.  This feature is scheduled to be deprecated in the future and later removed.   &#34;EXPOSE&#34; - Expose all VM metadata to pods.   &#34;GKE_METADATA_SERVER&#34; - Run the GKE Metadata Server on this node. The GKE Metadata Server exposes a metadata API to workloads that is compatible with the V1 Compute Metadata APIs exposed by the Compute Engine and App Engine Metadata Servers. This feature can only be enabled if Workload Identity is enabled at the cluster level.



This API documentation was generated by `crossdocs`.