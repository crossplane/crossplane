# compute.gcp.crossplane.io/v1alpha3 API Reference

Package v1alpha3 contains managed resources for GCP compute services such as GKE.

This API group contains the following Crossplane resources:

* [GKECluster](#GKECluster)
* [GKEClusterClass](#GKEClusterClass)

## GKECluster

A GKECluster is a managed resource that represents a Google Kubernetes Engine cluster.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `compute.gcp.crossplane.io/v1alpha3`
`kind` | string | `GKECluster`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [GKEClusterSpec](#GKEClusterSpec) | A GKEClusterSpec defines the desired state of a GKECluster.
`status` | [GKEClusterStatus](#GKEClusterStatus) | A GKEClusterStatus represents the observed state of a GKECluster.



## GKEClusterClass

A GKEClusterClass is a resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `compute.gcp.crossplane.io/v1alpha3`
`kind` | string | `GKEClusterClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [GKEClusterClassSpecTemplate](#GKEClusterClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned GKECluster.



## GKEClusterClassSpecTemplate

A GKEClusterClassSpecTemplate is a template for the spec of a dynamically provisioned GKECluster.

Appears in:

* [GKEClusterClass](#GKEClusterClass)




GKEClusterClassSpecTemplate supports all fields of:

* [v1alpha1.ClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#classspectemplate)
* [GKEClusterParameters](#GKEClusterParameters)


## GKEClusterParameters

GKEClusterParameters define the desired state of a Google Kubernetes Engine cluster.

Appears in:

* [GKEClusterClassSpecTemplate](#GKEClusterClassSpecTemplate)
* [GKEClusterSpec](#GKEClusterSpec)


Name | Type | Description
-----|------|------------
`clusterVersion` | Optional string | ClusterVersion is the initial Kubernetes version for this cluster. Users may specify either explicit versions offered by Kubernetes Engine or version aliases, for example &#34;latest&#34;, &#34;1.X&#34;, or &#34;1.X.Y&#34;. Leave unset to use the default version.
`labels` | Optional map[string]string | Labels for the cluster to use to annotate any related Google Compute Engine resources.
`machineType` | Optional string | MachineType is the name of a Google Compute Engine machine type (e.g. n1-standard-1). If unspecified the default machine type is n1-standard-1.
`numNodes` | int64 | NumNodes is the number of nodes to create in this cluster. You must ensure that your Compute Engine resource quota is sufficient for this number of instances. You must also have available firewall and routes quota.
`zone` | Optional string | Zone specifies the name of the Google Compute Engine zone in which this cluster resides.
`scopes` | Optional []string | Scopes are the set of Google API scopes to be made available on all of the node VMs under the &#34;default&#34; service account.
`network` | Optional string | Network is the name of the Google Compute Engine network to which the cluster is connected. If left unspecified, the default network will be used.
`networkRef` | [NetworkURIReferencerForGKECluster](#NetworkURIReferencerForGKECluster) | NetworkRef references to a Network and retrieves its URI
`subnetwork` | Optional string | Subnetwork is the name of the Google Compute Engine subnetwork to which the cluster is connected.
`subnetworkRef` | [SubnetworkURIReferencerForGKECluster](#SubnetworkURIReferencerForGKECluster) | SubnetworkRef references to a Subnetwork and retrieves its URI
`enableIPAlias` | Optional bool | EnableIPAlias determines whether Alias IPs will be used for pod IPs in the cluster.
`createSubnetwork` | Optional bool | CreateSubnetwork determines whether a new subnetwork will be created automatically for the cluster. Only applicable when EnableIPAlias is true.
`nodeIPV4CIDR` | Optional string | NodeIPV4CIDR specifies the IP address range of the instance IPs in this cluster. This is applicable only if CreateSubnetwork is true. Omit this field to have a range chosen with the default size. Set it to a netmask (e.g. /24) to have a range chosen with a specific netmask.
`clusterIPV4CIDR` | Optional string | ClusterIPV4CIDR specifies the IP address range of the pod IPs in this cluster. This is applicable only if EnableIPAlias is true. Omit this field to have a range chosen with the default size. Set it to a netmask (e.g. /24) to have a range chosen with a specific netmask.
`clusterSecondaryRangeName` | Optional string | ClusterSecondaryRangeName specifies the name of the secondary range to be used for the cluster CIDR block. The secondary range will be used for pod IP addresses. This must be an existing secondary range associated with the cluster subnetwork.
`serviceIPV4CIDR` | Optional string | ServiceIPV4CIDR specifies the IP address range of service IPs in this cluster. This is applicable only if EnableIPAlias is true. Omit this field to have a range chosen with the default size. Set it to a netmask (e.g. /24) to have a range chosen with a specific netmask.
`servicesSecondaryRangeName` | string | ServicesSecondaryRangeName specifies the name of the secondary range to be used as for the services CIDR block. The secondary range will be used for service ClusterIPs. This must be an existing secondary range associated with the cluster subnetwork.



## GKEClusterSpec

A GKEClusterSpec defines the desired state of a GKECluster.

Appears in:

* [GKECluster](#GKECluster)




GKEClusterSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [GKEClusterParameters](#GKEClusterParameters)


## GKEClusterStatus

A GKEClusterStatus represents the observed state of a GKECluster.

Appears in:

* [GKECluster](#GKECluster)


Name | Type | Description
-----|------|------------
`clusterName` | string | ClusterName is the name of this GKE cluster. The name is automatically generated by Crossplane.
`endpoint` | string | Endpoint of the GKE cluster used in connection strings.
`state` | string | State of this GKE cluster.


GKEClusterStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## NetworkURIReferencerForGKECluster

NetworkURIReferencerForGKECluster is an attribute referencer that resolves network uri from a referenced Network and assigns it to a GKECluster

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)




NetworkURIReferencerForGKECluster supports all fields of:

* github.com/crossplane/stack-gcp/apis/compute/v1beta1.NetworkURIReferencer


## SubnetworkURIReferencerForGKECluster

SubnetworkURIReferencerForGKECluster is an attribute referencer that resolves subnetwork uri from a referenced Subnetwork and assigns it to a GKECluster

Appears in:

* [GKEClusterParameters](#GKEClusterParameters)




SubnetworkURIReferencerForGKECluster supports all fields of:

* github.com/crossplane/stack-gcp/apis/compute/v1beta1.SubnetworkURIReferencer


This API documentation was generated by `crossdocs`.