# core.crossplane.io/v1alpha1 API Reference

Package v1alpha1 contains core API types used by most Crossplane resources.

This API group contains the following Crossplane resources:


## BindingPhase

BindingPhase represents the current binding phase of a resource or claim. Alias of string.

Appears in:

* [BindingStatus](#BindingStatus)


## BindingStatus

A BindingStatus represents the bindability and binding status of a resource.

Appears in:

* [ResourceClaimStatus](#ResourceClaimStatus)
* [ResourceStatus](#ResourceStatus)


Name | Type | Description
-----|------|------------
`bindingPhase` | Optional [BindingPhase](#BindingPhase) | Phase represents the binding phase of a managed resource or claim. Unbindable resources cannot be bound, typically because they are currently unavailable, or still being created. Unbound resource are available for binding, and Bound resources have successfully bound to another resource.



## ClassSpecTemplate

A ClassSpecTemplate defines a template that will be used to create the specifications of managed resources dynamically provisioned using a resource class.


Name | Type | Description
-----|------|------------
`writeConnectionSecretsToNamespace` | string | WriteConnectionSecretsToNamespace specifies the namespace in which the connection secrets of managed resources dynamically provisioned using this claim will be created.
`providerRef` | [core/v1.ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectreference-v1-core) | ProviderReference specifies the provider that will be used to create, observe, update, and delete managed resources that are dynamically provisioned using this resource class.
`reclaimPolicy` | Optional [ReclaimPolicy](#ReclaimPolicy) | ReclaimPolicy specifies what will happen to external resources when managed resources dynamically provisioned using this resource class are deleted. &#34;Delete&#34; deletes the external resource, while &#34;Retain&#34; (the default) does not. Note this behaviour is subtly different from other uses of the ReclaimPolicy concept within the Kubernetes ecosystem per https://github.com/crossplaneio/crossplane-runtime/issues/21



## Condition

A Condition that may apply to a managed resource.

Appears in:

* [ConditionedStatus](#ConditionedStatus)


Name | Type | Description
-----|------|------------
`type` | [ConditionType](#ConditionType) | Type of this condition. At most one of each condition type may apply to a resource at any point in time.
`status` | [core/v1.ConditionStatus](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#conditionstatus-v1-core) | Status of this condition; is it currently True, False, or Unknown?
`lastTransitionTime` | [meta/v1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#time-v1-meta) | LastTransitionTime is the last time this condition transitioned from one status to another.
`reason` | [ConditionReason](#ConditionReason) | A Reason for this condition&#39;s last transition from one status to another.
`message` | Optional string | A Message containing details about this condition&#39;s last transition from one status to another, if any.



## ConditionReason

A ConditionReason represents the reason a resource is in a condition. Alias of string.

Appears in:

* [Condition](#Condition)


## ConditionType

A ConditionType represents a condition a resource could be in. Alias of string.

Appears in:

* [Condition](#Condition)


## ConditionedStatus

A ConditionedStatus reflects the observed status of a managed resource. Only one condition of each type may exist.

Appears in:

* [ResourceClaimStatus](#ResourceClaimStatus)
* [ResourceStatus](#ResourceStatus)


Name | Type | Description
-----|------|------------
`conditions` | Optional [[]Condition](#Condition) | Conditions of the resource.



## LocalSecretReference

A LocalSecretReference is a reference to a secret in the same namespace as the referencer.

Appears in:

* [ResourceClaimSpec](#ResourceClaimSpec)


Name | Type | Description
-----|------|------------
`name` | string | Name of the secret.



## ReclaimPolicy

A ReclaimPolicy determines what should happen to managed resources when their bound resource claims are deleted. Alias of string.

Appears in:

* [ClassSpecTemplate](#ClassSpecTemplate)
* [ResourceSpec](#ResourceSpec)


## ResourceClaimSpec

A ResourceClaimSpec defines the desired state of a resource claim.


Name | Type | Description
-----|------|------------
`writeConnectionSecretToRef` | Optional [LocalSecretReference](#LocalSecretReference) | WriteConnectionSecretToReference specifies the name of a Secret, in the same namespace as this resource claim, to which any connection details for this resource claim should be written. Connection details frequently include the endpoint, username, and password required to connect to the managed resource bound to this resource claim.
`classSelector` | Optional [meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#labelselector-v1-meta) | A ClassSelector specifies labels that will be used to select a resource class for this claim. If multiple classes match the labels one will be chosen at random.
`classRef` | Optional [core/v1.ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectreference-v1-core) | A ClassReference specifies a resource class that will be used to dynamically provision a managed resource when the resource claim is created.
`resourceRef` | Optional [core/v1.ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectreference-v1-core) | A ResourceReference specifies an existing managed resource, in any namespace, to which this resource claim should attempt to bind. Omit the resource reference to enable dynamic provisioning using a resource class; the resource reference will be automatically populated by Crossplane.



## ResourceClaimStatus

A ResourceClaimStatus represents the observed status of a resource claim.




ResourceClaimStatus supports all fields of:

* [ConditionedStatus](#ConditionedStatus)
* [BindingStatus](#BindingStatus)


## ResourceSpec

A ResourceSpec defines the desired state of a managed resource.


Name | Type | Description
-----|------|------------
`writeConnectionSecretToRef` | Optional [SecretReference](#SecretReference) | WriteConnectionSecretToReference specifies the namespace and name of a Secret to which any connection details for this managed resource should be written. Connection details frequently include the endpoint, username, and password required to connect to the managed resource.
`claimRef` | Optional [core/v1.ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectreference-v1-core) | ClaimReference specifies the resource claim to which this managed resource will be bound. ClaimReference is set automatically during dynamic provisioning. Crossplane does not currently support setting this field manually, per https://github.com/crossplaneio/crossplane-runtime/issues/19
`classRef` | Optional [core/v1.ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectreference-v1-core) | ClassReference specifies the resource class that was used to dynamically provision this managed resource, if any. Crossplane does not currently support setting this field manually, per https://github.com/crossplaneio/crossplane-runtime/issues/20
`providerRef` | [core/v1.ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectreference-v1-core) | ProviderReference specifies the provider that will be used to create, observe, update, and delete this managed resource.
`reclaimPolicy` | Optional [ReclaimPolicy](#ReclaimPolicy) | ReclaimPolicy specifies what will happen to the external resource this managed resource manages when the managed resource is deleted. &#34;Delete&#34; deletes the external resource, while &#34;Retain&#34; (the default) does not. Note this behaviour is subtly different from other uses of the ReclaimPolicy concept within the Kubernetes ecosystem per https://github.com/crossplaneio/crossplane-runtime/issues/21



## ResourceStatus

ResourceStatus represents the observed state of a managed resource.




ResourceStatus supports all fields of:

* [ConditionedStatus](#ConditionedStatus)
* [BindingStatus](#BindingStatus)


## SecretKeySelector

A SecretKeySelector is a reference to a secret key in an arbitrary namespace.


Name | Type | Description
-----|------|------------
`key` | string | The key to select.


SecretKeySelector supports all fields of:

* [SecretReference](#SecretReference)


## SecretReference

A SecretReference is a reference to a secret in an arbitrary namespace.

Appears in:

* [ResourceSpec](#ResourceSpec)
* [SecretKeySelector](#SecretKeySelector)


Name | Type | Description
-----|------|------------
`name` | string | Name of the secret.
`namespace` | string | Namespace of the secret.



This API documentation was generated by `crossdocs`.