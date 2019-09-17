# identity.aws.crossplane.io/v1alpha2 API Reference

Package v1alpha2 contains managed resources for AWS identity services such as IAM.

This API group contains the following Crossplane resources:

* [IAMRole](#IAMRole)
* [IAMRolePolicyAttachment](#IAMRolePolicyAttachment)

## IAMRole

An IAMRole is a managed resource that represents an AWS IAM Role.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `identity.aws.crossplane.io/v1alpha2`
`kind` | string | `IAMRole`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [IAMRoleSpec](#IAMRoleSpec) | An IAMRoleSpec defines the desired state of an IAMRole.
`status` | [IAMRoleStatus](#IAMRoleStatus) | An IAMRoleStatus represents the observed state of an IAMRole.



## IAMRolePolicyAttachment

An IAMRolePolicyAttachment is a managed resource that represents an AWS IAM Role policy attachment.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `identity.aws.crossplane.io/v1alpha2`
`kind` | string | `IAMRolePolicyAttachment`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [IAMRolePolicyAttachmentSpec](#IAMRolePolicyAttachmentSpec) | An IAMRolePolicyAttachmentSpec defines the desired state of an IAMRolePolicyAttachment.
`status` | [IAMRolePolicyAttachmentStatus](#IAMRolePolicyAttachmentStatus) | An IAMRolePolicyAttachmentStatus represents the observed state of an IAMRolePolicyAttachment.



## IAMRoleExternalStatus

IAMRoleExternalStatus keeps the state for the external resource

Appears in:

* [IAMRoleStatus](#IAMRoleStatus)


Name | Type | Description
-----|------|------------
`arn` | string | ARN is the Amazon Resource Name (ARN) specifying the role. For more information about ARNs and how to use them in policies, see IAM Identifiers (http://docs.aws.amazon.com/IAM/latest/UserGuide/Using_Identifiers.html) in the IAM User Guide guide.
`roleID` | string | RoleID is the stable and unique string identifying the role. For more information about IDs, see IAM Identifiers (http://docs.aws.amazon.com/IAM/latest/UserGuide/Using_Identifiers.html) in the Using IAM guide.



## IAMRoleParameters

IAMRoleParameters define the desired state of an AWS IAM Role.

Appears in:

* [IAMRoleSpec](#IAMRoleSpec)


Name | Type | Description
-----|------|------------
`assumeRolePolicyDocument` | string | AssumeRolePolicyDocument is the the trust relationship policy document that grants an entity permission to assume the role.
`description` | Optional string | Description is a description of the role.
`roleName` | string | RoleName presents the name of the IAM role.



## IAMRolePolicyAttachmentExternalStatus

IAMRolePolicyAttachmentExternalStatus keeps the state for the external resource

Appears in:

* [IAMRolePolicyAttachmentStatus](#IAMRolePolicyAttachmentStatus)


Name | Type | Description
-----|------|------------
`attachedPolicyArn` | string | AttachedPolicyARN is the arn for the attached policy. If nil, the policy is not yet attached



## IAMRolePolicyAttachmentParameters

IAMRolePolicyAttachmentParameters define the desired state of an AWS IAM Role policy attachment.

Appears in:

* [IAMRolePolicyAttachmentSpec](#IAMRolePolicyAttachmentSpec)


Name | Type | Description
-----|------|------------
`policyArn` | string | PolicyARN is the Amazon Resource Name (ARN) of the IAM policy you want to attach.
`roleName` | string | RoleName presents the name of the IAM role.



## IAMRolePolicyAttachmentSpec

An IAMRolePolicyAttachmentSpec defines the desired state of an IAMRolePolicyAttachment.

Appears in:

* [IAMRolePolicyAttachment](#IAMRolePolicyAttachment)




IAMRolePolicyAttachmentSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [IAMRolePolicyAttachmentParameters](#IAMRolePolicyAttachmentParameters)


## IAMRolePolicyAttachmentStatus

An IAMRolePolicyAttachmentStatus represents the observed state of an IAMRolePolicyAttachment.

Appears in:

* [IAMRolePolicyAttachment](#IAMRolePolicyAttachment)




IAMRolePolicyAttachmentStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)
* [IAMRolePolicyAttachmentExternalStatus](#IAMRolePolicyAttachmentExternalStatus)


## IAMRoleSpec

An IAMRoleSpec defines the desired state of an IAMRole.

Appears in:

* [IAMRole](#IAMRole)




IAMRoleSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [IAMRoleParameters](#IAMRoleParameters)


## IAMRoleStatus

An IAMRoleStatus represents the observed state of an IAMRole.

Appears in:

* [IAMRole](#IAMRole)




IAMRoleStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)
* [IAMRoleExternalStatus](#IAMRoleExternalStatus)


This API documentation was generated by `crossdocs`.