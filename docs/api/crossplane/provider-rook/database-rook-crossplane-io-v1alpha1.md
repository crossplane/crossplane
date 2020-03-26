# database.rook.crossplane.io/v1alpha1 API Reference

Package v1alpha1 contains database service resources for Rook

This API group contains the following Crossplane resources:

* [CockroachCluster](#CockroachCluster)
* [CockroachClusterClass](#CockroachClusterClass)
* [YugabyteCluster](#YugabyteCluster)
* [YugabyteClusterClass](#YugabyteClusterClass)

## CockroachCluster

A CockroachCluster configures a Rook &#39;clusters.cockroachdb.rook.io&#39;


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.rook.crossplane.io/v1alpha1`
`kind` | string | `CockroachCluster`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [CockroachClusterSpec](#CockroachClusterSpec) | A CockroachClusterSpec defines the desired state of a CockroachCluster.
`status` | [CockroachClusterStatus](#CockroachClusterStatus) | A CockroachClusterStatus defines the current state of a CockroachCluster.



## CockroachClusterClass

A CockroachClusterClass is a resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.rook.crossplane.io/v1alpha1`
`kind` | string | `CockroachClusterClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [CockroachClusterClassSpecTemplate](#CockroachClusterClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned CockroachCluster.



## YugabyteCluster

A YugabyteCluster configures a Rook &#39;ybclusters.yugabytedb.rook.io&#39;


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.rook.crossplane.io/v1alpha1`
`kind` | string | `YugabyteCluster`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [YugabyteClusterSpec](#YugabyteClusterSpec) | A YugabyteClusterSpec defines the desired state of a YugabyteCluster.
`status` | [YugabyteClusterStatus](#YugabyteClusterStatus) | A YugabyteClusterStatus defines the current state of a YugabyteCluster.



## YugabyteClusterClass

A YugabyteClusterClass is a resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.rook.crossplane.io/v1alpha1`
`kind` | string | `YugabyteClusterClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [YugabyteClusterClassSpecTemplate](#YugabyteClusterClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned YugabyteCluster.



## CockroachClusterClassSpecTemplate

A CockroachClusterClassSpecTemplate is a template for the spec of a dynamically provisioned CockroachCluster.

Appears in:

* [CockroachClusterClass](#CockroachClusterClass)


Name | Type | Description
-----|------|------------
`forProvider` | [CockroachClusterParameters](#CockroachClusterParameters) | A CockroachClusterParameters defines the desired state of a CockroachCluster.


CockroachClusterClassSpecTemplate supports all fields of:

* [v1alpha1.ClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#classspectemplate)


## CockroachClusterParameters

A CockroachClusterParameters defines the desired state of a CockroachCluster.

Appears in:

* [CockroachClusterClassSpecTemplate](#CockroachClusterClassSpecTemplate)
* [CockroachClusterSpec](#CockroachClusterSpec)


Name | Type | Description
-----|------|------------
`name` | string | 
`namespace` | string | 
`annotations` | github.com/crossplane/provider-rook/apis/v1alpha1.Annotations | The annotations-related configuration to add/set on each Pod related object.
`scope` | github.com/crossplane/provider-rook/apis/v1alpha1.StorageScopeSpec | 
`network` | [NetworkSpec](#NetworkSpec) | NetworkSpec describes network related settings of the cluster
`secure` | bool | 
`cachePercent` | int | 
`maxSQLMemoryPercent` | int | 



## CockroachClusterSpec

A CockroachClusterSpec defines the desired state of a CockroachCluster.

Appears in:

* [CockroachCluster](#CockroachCluster)


Name | Type | Description
-----|------|------------
`forProvider` | [CockroachClusterParameters](#CockroachClusterParameters) | A CockroachClusterParameters defines the desired state of a CockroachCluster.


CockroachClusterSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## CockroachClusterStatus

A CockroachClusterStatus defines the current state of a CockroachCluster.

Appears in:

* [CockroachCluster](#CockroachCluster)




CockroachClusterStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## NetworkSpec

NetworkSpec describes network related settings of the cluster

Appears in:

* [CockroachClusterParameters](#CockroachClusterParameters)
* [ServerSpec](#ServerSpec)


Name | Type | Description
-----|------|------------
`ports` | [[]PortSpec](#PortSpec) | Set of named ports that can be configured for this resource



## PortSpec

PortSpec is named port

Appears in:

* [NetworkSpec](#NetworkSpec)


Name | Type | Description
-----|------|------------
`name` | string | Name of port
`port` | int32 | Port number



## ServerSpec

ServerSpec describes server related settings of the cluster

Appears in:

* [YugabyteClusterParameters](#YugabyteClusterParameters)


Name | Type | Description
-----|------|------------
`replicas` | int32 | 
`network` | [NetworkSpec](#NetworkSpec) | NetworkSpec describes network related settings of the cluster
`volumeClaimTemplate` | [core/v1.PersistentVolumeClaim](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#persistentvolumeclaim-v1-core) | 



## YugabyteClusterClassSpecTemplate

A YugabyteClusterClassSpecTemplate is a template for the spec of a dynamically provisioned YugabyteCluster.

Appears in:

* [YugabyteClusterClass](#YugabyteClusterClass)


Name | Type | Description
-----|------|------------
`forProvider` | [YugabyteClusterParameters](#YugabyteClusterParameters) | A YugabyteClusterParameters defines the desired state of a YugabyteCluster.


YugabyteClusterClassSpecTemplate supports all fields of:

* [v1alpha1.ClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#classspectemplate)


## YugabyteClusterParameters

A YugabyteClusterParameters defines the desired state of a YugabyteCluster.

Appears in:

* [YugabyteClusterClassSpecTemplate](#YugabyteClusterClassSpecTemplate)
* [YugabyteClusterSpec](#YugabyteClusterSpec)


Name | Type | Description
-----|------|------------
`name` | string | 
`namespace` | string | 
`annotations` | github.com/crossplane/provider-rook/apis/v1alpha1.Annotations | 
`master` | [ServerSpec](#ServerSpec) | ServerSpec describes server related settings of the cluster
`tserver` | [ServerSpec](#ServerSpec) | ServerSpec describes server related settings of the cluster



## YugabyteClusterSpec

A YugabyteClusterSpec defines the desired state of a YugabyteCluster.

Appears in:

* [YugabyteCluster](#YugabyteCluster)


Name | Type | Description
-----|------|------------
`forProvider` | [YugabyteClusterParameters](#YugabyteClusterParameters) | A YugabyteClusterParameters defines the desired state of a YugabyteCluster.


YugabyteClusterSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## YugabyteClusterStatus

A YugabyteClusterStatus defines the current state of a YugabyteCluster.

Appears in:

* [YugabyteCluster](#YugabyteCluster)




YugabyteClusterStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


This API documentation was generated by `crossdocs`.