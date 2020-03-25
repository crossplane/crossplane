# workload.crossplane.io/v1alpha1 API Reference

Package v1alpha1 contains resources relating to Crossplane Workloads.

This API group contains the following Crossplane resources:

* [KubernetesApplication](#KubernetesApplication)
* [KubernetesApplicationResource](#KubernetesApplicationResource)
* [KubernetesTarget](#KubernetesTarget)

## KubernetesApplication

A KubernetesApplication defines an application deployed by Crossplane to a Kubernetes cluster, i.e. a portable KubernetesCluster resource claim.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `workload.crossplane.io/v1alpha1`
`kind` | string | `KubernetesApplication`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [KubernetesApplicationSpec](#KubernetesApplicationSpec) | A KubernetesApplicationSpec specifies the resources of a Kubernetes application.
`status` | [KubernetesApplicationStatus](#KubernetesApplicationStatus) | KubernetesApplicationStatus represents the observed state of a KubernetesApplication.



## KubernetesApplicationResource

A KubernetesApplicationResource is a resource of a Kubernetes application. Each resource templates a single Kubernetes resource to be deployed to its scheduled KubernetesCluster.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `workload.crossplane.io/v1alpha1`
`kind` | string | `KubernetesApplicationResource`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [KubernetesApplicationResourceSpec](#KubernetesApplicationResourceSpec) | KubernetesApplicationResourceSpec specifies the desired state of a KubernetesApplicationResource.
`status` | [KubernetesApplicationResourceStatus](#KubernetesApplicationResourceStatus) | KubernetesApplicationResourceStatus represents the observed state of a KubernetesApplicationResource.



## KubernetesTarget

A KubernetesTarget is a scheduling target for a Kubernetes Application.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `workload.crossplane.io/v1alpha1`
`kind` | string | `KubernetesTarget`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [v1alpha1.TargetSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#targetspec) | 
`status` | [v1alpha1.TargetStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#targetstatus) | 



## KubernetesApplicationResourceSpec

KubernetesApplicationResourceSpec specifies the desired state of a KubernetesApplicationResource.

Appears in:

* [KubernetesApplicationResource](#KubernetesApplicationResource)
* [KubernetesApplicationResourceTemplate](#KubernetesApplicationResourceTemplate)


Name | Type | Description
-----|------|------------
`template` | k8s.io/apimachinery/pkg/runtime.RawExtension | A Template for a Kubernetes resource to be submitted to the KubernetesCluster to which this application resource is scheduled. The resource must be understood by the KubernetesCluster. Crossplane requires only that the resource contains standard Kubernetes type and object metadata.
`targetRef` | Optional [KubernetesTargetReference](#KubernetesTargetReference) | Target to which this application has been scheduled.
`secrets` | [[]core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core) | Secrets upon which this application resource depends. These secrets will be propagated to the Kubernetes cluster to which this application is scheduled.



## KubernetesApplicationResourceState

KubernetesApplicationResourceState represents the state of a KubernetesApplicationResource. Alias of string.

Appears in:

* [KubernetesApplicationResourceStatus](#KubernetesApplicationResourceStatus)


## KubernetesApplicationResourceStatus

KubernetesApplicationResourceStatus represents the observed state of a KubernetesApplicationResource.

Appears in:

* [KubernetesApplicationResource](#KubernetesApplicationResource)


Name | Type | Description
-----|------|------------
`conditionedStatus` | [v1alpha1.ConditionedStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#conditionedstatus) | 
`state` | [KubernetesApplicationResourceState](#KubernetesApplicationResourceState) | State of the application.
`remote` | [RemoteStatus](#RemoteStatus) | Remote status of the resource templated by this application resource.



## KubernetesApplicationResourceTemplate

A KubernetesApplicationResourceTemplate is used to instantiate new KubernetesApplicationResources.

Appears in:

* [KubernetesApplicationSpec](#KubernetesApplicationSpec)


Name | Type | Description
-----|------|------------
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [KubernetesApplicationResourceSpec](#KubernetesApplicationResourceSpec) | KubernetesApplicationResourceSpec specifies the desired state of a KubernetesApplicationResource.



## KubernetesApplicationSpec

A KubernetesApplicationSpec specifies the resources of a Kubernetes application.

Appears in:

* [KubernetesApplication](#KubernetesApplication)


Name | Type | Description
-----|------|------------
`resourceSelector` | [meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#labelselector-v1-meta) | ResourceSelector selects the KubernetesApplicationResources that are managed by this KubernetesApplication. Note that a KubernetesApplication will never adopt orphaned KubernetesApplicationResources, and thus this selector serves only to help match a KubernetesApplication to its KubernetesApplicationResources.
`targetSelector` | Optional [meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#labelselector-v1-meta) | TargetSelector selects the targets to which this application may be scheduled. Leave both match labels and expressions empty to match any target.
`targetRef` | Optional [KubernetesTargetReference](#KubernetesTargetReference) | Target to which this application has been scheduled.
`resourceTemplates` | [[]KubernetesApplicationResourceTemplate](#KubernetesApplicationResourceTemplate) | ResourceTemplates specifies a set of Kubernetes application resources managed by this application.



## KubernetesApplicationState

KubernetesApplicationState represents the state of a Kubernetes application. Alias of string.

Appears in:

* [KubernetesApplicationStatus](#KubernetesApplicationStatus)


## KubernetesApplicationStatus

KubernetesApplicationStatus represents the observed state of a KubernetesApplication.

Appears in:

* [KubernetesApplication](#KubernetesApplication)


Name | Type | Description
-----|------|------------
`conditionedStatus` | [v1alpha1.ConditionedStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#conditionedstatus) | 
`state` | [KubernetesApplicationState](#KubernetesApplicationState) | State of the application.
`desiredResources` | int | Desired resources of this application, i.e. the number of resources that match this application&#39;s resource selector.
`submittedResources` | int | Submitted resources of this workload, i.e. the subset of desired resources that have been successfully submitted to their scheduled Kubernetes cluster.



## KubernetesTargetReference

A KubernetesTargetReference is a reference to a KubernetesTarget resource claim in the same namespace as the referrer.

Appears in:

* [KubernetesApplicationResourceSpec](#KubernetesApplicationResourceSpec)
* [KubernetesApplicationSpec](#KubernetesApplicationSpec)


Name | Type | Description
-----|------|------------
`name` | string | Name of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names



## RemoteStatus

RemoteStatus represents the observed state of a remote cluster.

Appears in:

* [KubernetesApplicationResourceStatus](#KubernetesApplicationResourceStatus)


Name | Type | Description
-----|------|------------
`raw` | [encoding/json.RawMessage](https://golang.org/pkg/encoding/json#RawMessage) | Raw JSON representation of the remote status as a byte array.



This API documentation was generated by `crossdocs`.