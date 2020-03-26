# stacks.crossplane.io/v1alpha1 API Reference

Package v1alpha1 contains resources relating to Crossplane Stacks.

This API group contains the following Crossplane resources:

* [ClusterStackInstall](#ClusterStackInstall)
* [Stack](#Stack)
* [StackDefinition](#StackDefinition)
* [StackInstall](#StackInstall)

## ClusterStackInstall

ClusterStackInstall is the CRD type for a request to add a stack to Crossplane.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `stacks.crossplane.io/v1alpha1`
`kind` | string | `ClusterStackInstall`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [StackInstallSpec](#StackInstallSpec) | StackInstallSpec specifies details about a request to install a stack to Crossplane.
`status` | [StackInstallStatus](#StackInstallStatus) | StackInstallStatus represents the observed state of a StackInstall.



## Stack

A Stack that has been added to Crossplane.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `stacks.crossplane.io/v1alpha1`
`kind` | string | `Stack`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [StackSpec](#StackSpec) | StackSpec specifies the desired state of a Stack.
`status` | [StackStatus](#StackStatus) | StackStatus represents the observed state of a Stack.



## StackDefinition

StackDefinition is the Schema for the StackDefinitions API


Name | Type | Description
-----|------|------------
`apiVersion` | string | `stacks.crossplane.io/v1alpha1`
`kind` | string | `StackDefinition`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [StackDefinitionSpec](#StackDefinitionSpec) | StackDefinitionSpec defines the desired state of StackDefinition
`status` | [StackDefinitionStatus](#StackDefinitionStatus) | StackDefinitionStatus defines the observed state of StackDefinition



## StackInstall

A StackInstall requests a stack be installed to Crossplane.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `stacks.crossplane.io/v1alpha1`
`kind` | string | `StackInstall`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [StackInstallSpec](#StackInstallSpec) | StackInstallSpec specifies details about a request to install a stack to Crossplane.
`status` | [StackInstallStatus](#StackInstallStatus) | StackInstallStatus represents the observed state of a StackInstall.



## AppMetadataSpec

AppMetadataSpec defines metadata about the stack application

Appears in:

* [PackageMetadataSpec](#PackageMetadataSpec)
* [StackSpec](#StackSpec)


Name | Type | Description
-----|------|------------
`title` | string | 
`overviewShort` | string | 
`overview` | string | 
`readme` | string | 
`version` | string | 
`icons` | [[]IconSpec](#IconSpec) | 
`maintainers` | [[]ContributorSpec](#ContributorSpec) | 
`owners` | [[]ContributorSpec](#ContributorSpec) | 
`company` | string | 
`category` | string | 
`keywords` | []string | 
`website` | string | 
`source` | string | 
`license` | string | 
`dependsOn` | [[]StackInstallSpec](#StackInstallSpec) | DependsOn is the list of CRDs that this stack depends on. This data drives the RBAC generation process.
`packageType` | string | 
`permissionScope` | string | 



## Behavior

Behavior specifies the behavior for the stack, if the stack has behaviors instead of a controller

Appears in:

* [StackDefinitionSpec](#StackDefinitionSpec)


Name | Type | Description
-----|------|------------
`crd` | [BehaviorCRD](#BehaviorCRD) | BehaviorCRD represents the CRD which the stack&#39;s behavior controller will watch. When CRs of this CRD kind appear and are modified in the cluster, the behavior will execute.
`engine` | [StackResourceEngineConfiguration](#StackResourceEngineConfiguration) | StackResourceEngineConfiguration represents a configuration for a resource engine, such as helm2 or kustomize.
`source` | [StackDefinitionSource](#StackDefinitionSource) | Theoretically, source and engine could be specified at a per-hook level as well.



## BehaviorCRD

BehaviorCRD represents the CRD which the stack&#39;s behavior controller will watch. When CRs of this CRD kind appear and are modified in the cluster, the behavior will execute.

Appears in:

* [Behavior](#Behavior)


Name | Type | Description
-----|------|------------
`apiVersion` | string | 
`kind` | string | 



## ContributorSpec

ContributorSpec defines a contributor for a stack (e.g., maintainer, owner, etc.)

Appears in:

* [AppMetadataSpec](#AppMetadataSpec)


Name | Type | Description
-----|------|------------
`name` | string | 
`email` | string | 



## ControllerDeployment

ControllerDeployment defines a controller for a stack that is managed by a Deployment.

Appears in:

* [ControllerSpec](#ControllerSpec)


Name | Type | Description
-----|------|------------
`name` | string | 
`spec` | [apps/v1.DeploymentSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#deploymentspec-v1-apps) | 



## ControllerSpec

ControllerSpec defines the controller that implements the logic for a stack, which can come in different flavors.

Appears in:

* [StackSpec](#StackSpec)


Name | Type | Description
-----|------|------------
`serviceAccount` | [ServiceAccountOptions](#ServiceAccountOptions) | ServiceAccount options allow for changes to the ServiceAccount the Stack Manager creates for the Stack&#39;s controller
`deployment` | [ControllerDeployment](#ControllerDeployment) | 



## FieldBinding

FieldBinding describes a field binding of a transformation from the triggering CR to an object for configuring the resource engine. It connects a field in the source object to a field in the destination object.

Appears in:

* [KustomizeEngineOverlay](#KustomizeEngineOverlay)


Name | Type | Description
-----|------|------------
`from` | string | 
`to` | string | 



## IconSpec

IconSpec defines the icon for a stack

Appears in:

* [AppMetadataSpec](#AppMetadataSpec)


Name | Type | Description
-----|------|------------
`base64Data` | string | 
`mediatype` | string | 



## KustomizeEngineConfiguration

KustomizeEngineConfiguration provides kustomize-specific engine configuration.

Appears in:

* [StackResourceEngineConfiguration](#StackResourceEngineConfiguration)


Name | Type | Description
-----|------|------------
`overlays` | [[]KustomizeEngineOverlay](#KustomizeEngineOverlay) | 
`kustomization` | [meta/v1/unstructured.Unstructured](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#unstructured-unstructured-v1) | 



## KustomizeEngineOverlay

KustomizeEngineOverlay configures the stack behavior controller to transform the input CR into some output objects for the underlying resource engine. This is expected to be interpreted by the engine-specific logic in the controller.

Appears in:

* [KustomizeEngineConfiguration](#KustomizeEngineConfiguration)


Name | Type | Description
-----|------|------------
`apiVersion` | string | 
`kind` | string | 
`name` | string | 
`bindings` | [[]FieldBinding](#FieldBinding) | 



## PackageMetadataSpec

PackageMetadataSpec defines metadata about the stack application and package contents


Name | Type | Description
-----|------|------------
`apiVersion` | string | 


PackageMetadataSpec supports all fields of:

* [AppMetadataSpec](#AppMetadataSpec)


## PermissionsSpec

PermissionsSpec defines the permissions that a stack will require to operate.

Appears in:

* [StackSpec](#StackSpec)


Name | Type | Description
-----|------|------------
`rules` | [[]rbac/v1.PolicyRule](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#policyrule-v1-rbac) | 



## ServiceAccountOptions

ServiceAccountOptions augment the ServiceAccount created by the Stack controller

Appears in:

* [ControllerSpec](#ControllerSpec)
* [StackControllerOptions](#StackControllerOptions)


Name | Type | Description
-----|------|------------
`annotations` | map[string]string | 



## StackControllerOptions

StackControllerOptions allow for changes in the Stack extraction and deployment controllers. These can affect how images are fetched and how Stack derived resources are created.

Appears in:

* [StackInstallSpec](#StackInstallSpec)


Name | Type | Description
-----|------|------------
`imagePullSecrets` | [[]core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core) | ImagePullSecrets are named secrets in the same workspace that can be used to fetch Stacks from private repositories and to run controllers from private repositories
`imagePullPolicy` | [core/v1.PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#pullpolicy-v1-core) | ImagePullPolicy defines the pull policy for all images used during Stack extraction and when running the Stack controller. https://kubernetes.io/docs/concepts/configuration/overview/#container-images
`serviceAccount` | [ServiceAccountOptions](#ServiceAccountOptions) | ServiceAccount options allow for changes to the ServiceAccount the Stack Manager creates for the Stack&#39;s controller



## StackDefinitionSource

StackDefinitionSource is the stack image which this stack configuration is from. In the future, other source types may be supported, such as a URL.

Appears in:

* [Behavior](#Behavior)


Name | Type | Description
-----|------|------------
`image` | string | a container image id
`path` | string | The path to the files to process in the source



## StackDefinitionSpec

StackDefinitionSpec defines the desired state of StackDefinition

Appears in:

* [StackDefinition](#StackDefinition)


Name | Type | Description
-----|------|------------
`behavior` | [Behavior](#Behavior) | Behavior specifies the behavior for the stack, if the stack has behaviors instead of a controller


StackDefinitionSpec supports all fields of:

* [StackSpec](#StackSpec)


## StackDefinitionStatus

StackDefinitionStatus defines the observed state of StackDefinition

Appears in:

* [StackDefinition](#StackDefinition)


## StackInstallSpec

StackInstallSpec specifies details about a request to install a stack to Crossplane.

Appears in:

* [ClusterStackInstall](#ClusterStackInstall)
* [StackInstall](#StackInstall)
* [AppMetadataSpec](#AppMetadataSpec)


Name | Type | Description
-----|------|------------
`source` | string | Source is the domain name for the stack registry hosting the stack being requested, e.g., registry.crossplane.io
`package` | string | Package is the name of the stack package that is being requested, e.g., myapp. Either Package or CustomResourceDefinition can be specified.
`crd` | string | CustomResourceDefinition is the full name of a CRD that is owned by the stack being requested. This can be a convenient way of installing a stack when the desired CRD is known, but the package name that contains it is not known. Either Package or CustomResourceDefinition can be specified.


StackInstallSpec supports all fields of:

* [StackControllerOptions](#StackControllerOptions)


## StackInstallStatus

StackInstallStatus represents the observed state of a StackInstall.

Appears in:

* [ClusterStackInstall](#ClusterStackInstall)
* [StackInstall](#StackInstall)


Name | Type | Description
-----|------|------------
`conditionedStatus` | [v1alpha1.ConditionedStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#conditionedstatus) | 
`installJob` | [core/v1.ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectreference-v1-core) | 
`stackRecord` | [core/v1.ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectreference-v1-core) | 



## StackInstaller

StackInstaller provides a common interface for StackInstall and ClusterStackInstall to share controller and reconciler logic


## StackResourceEngineConfiguration

StackResourceEngineConfiguration represents a configuration for a resource engine, such as helm2 or kustomize.

Appears in:

* [Behavior](#Behavior)


Name | Type | Description
-----|------|------------
`controllerImage` | Optional string | ControllerImage is the image of the generic controller used to reconcile the instances of the given CRDs. If empty, it is populated by stack manager during unpack with default value.
`type` | string | Type is the engine type, such as &#34;helm2&#34; or &#34;kustomize&#34;
`kustomize` | Optional [KustomizeEngineConfiguration](#KustomizeEngineConfiguration) | Because different engine configurations could specify conflicting field names, we want to namespace the engines with engine-specific keys



## StackSpec

StackSpec specifies the desired state of a Stack.

Appears in:

* [Stack](#Stack)
* [StackDefinitionSpec](#StackDefinitionSpec)


Name | Type | Description
-----|------|------------
`customresourcedefinitions` | [CRDList](#CRDList) | CRDList is the full list of CRDs that this stack owns and depends on
`controller` | [ControllerSpec](#ControllerSpec) | ControllerSpec defines the controller that implements the logic for a stack, which can come in different flavors.
`permissions` | [PermissionsSpec](#PermissionsSpec) | PermissionsSpec defines the permissions that a stack will require to operate.


StackSpec supports all fields of:

* [AppMetadataSpec](#AppMetadataSpec)


## StackStatus

StackStatus represents the observed state of a Stack.

Appears in:

* [Stack](#Stack)


Name | Type | Description
-----|------|------------
`conditionedStatus` | [v1alpha1.ConditionedStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#conditionedstatus) | 
`controllerRef` | [core/v1.ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectreference-v1-core) | 



This API documentation was generated by `crossdocs`.