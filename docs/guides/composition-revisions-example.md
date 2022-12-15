---
title: Composition Revision Example
weight: 101
---
This tutorial discusses how CompositionRevisions work and how they manage Composite Resource
(XR) updates. This start with a `Composition` and `CompositeResourceDefinition` that defines a `MyVPC`
resource.

This tutorial creates multiple XRs and modifies Composition to apply different Composition Revisions to the XRs.

## Install Crossplane
Install Crossplane v1.11.0 or later and wait until the Crossplane pods are running.
```shell
kubectl create namespace crossplane-system
helm repo add crossplane-master https://charts.crossplane.io/master/
helm repo update
helm install crossplane --namespace crossplane-system crossplane-master/crossplane --devel --version 1.11.0-rc.0.108.g0521c32e
kubectl get pods -n crossplane-system
```
Expected Output:
```shell
NAME                                       READY   STATUS    RESTARTS   AGE
crossplane-7f75ddcc46-f4d2z                1/1     Running   0          9s
crossplane-rbac-manager-78bd597746-sdv6w   1/1     Running   0          9s
```

## Apply a Composition
Apply the example Composition.

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  labels:
    channel: dev
  name: myvpcs.aws.example.upbound.io
spec:
  writeConnectionSecretsToNamespace: crossplane-system
  compositeTypeRef:
    apiVersion: aws.example.upbound.io/v1alpha1
    kind: MyVPC
  resources:
  - base:
      apiVersion: ec2.aws.upbound.io/v1beta1
      kind: VPC
      spec:
        forProvider:
          region: us-west-1
          cidrBlock: 192.168.0.0/16
          enableDnsSupport: true
          enableDnsHostnames: true
    name: my-vcp
```


## Apply a Composite Resource Definition

Apply the example XRD.
```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: myvpcs.aws.example.upbound.io
spec:
  group: aws.example.upbound.io
  names:
    kind: MyVPC
    plural: myvpcs
  versions:
  - name: v1alpha1
    served: true 
    referenceable: true 
    schema:
      openAPIV3Schema:
        type: object 
        properties:
          spec:
            type: object 
            properties:
              id:
                type: string 
                description: ID of this VPC that other objects will use to refer to it. 
            required:
            - id
```

## Verify the Composition revision
Verify that Crossplane created the Composition revision
```shell
kubectl get compositionrevisions -o="custom-columns=NAME:.metadata.name,REVISION:.spec.revision,CHANNEL:.metadata.labels.channel"
```
Expected Output:
```shell
NAME                                    REVISION   CHANNEL
myvpcs.aws.example.upbound.io-ad265bc   1          dev
```

The label `dev` is automatically created from the Composition.

## Create Composite Resources

### Default update policy
Create an XR without a `compositionUpdatePolicy` defined. The update policy is `Automatic` by default:
```yaml
apiVersion: aws.example.upbound.io/v1alpha1
kind: MyVPC
metadata:
  name: vpc-auto
spec:
  id: vpc-auto
```
Expected Output:
```shell
myvpc.aws.example.upbound.io/vpc-auto created
``` 

### Manual update policy
Create an Composite Resource with `compositionUpdatePolicy: Manual` and `compositionRevisionRef`.
```yaml
apiVersion: aws.example.upbound.io/v1alpha1
kind: MyVPC
metadata:
  name: vpc-man
spec:
  id: vpc-man
  compositionUpdatePolicy: Manual
  compositionRevisionRef:
    name: myvpcs.aws.example.upbound.io-ad265bc
```

Expected Output:
```shell
myvpc.aws.example.upbound.io/vpc-man created
``` 


### Using a selector
Create an XR with a `compositionRevisionSelector` of `channel: dev`:
```yaml
apiVersion: aws.example.upbound.io/v1alpha1
kind:  MyVPC
metadata:
  name: vpc-dev
spec:
  id: vpc-dev
  compositionRevisionSelector:
    matchLabels:
      channel: dev
```
Expected Output:
```shell
myvpc.aws.example.upbound.io/vpc-dev created
``` 

Create an XR with a `compositionRevisionSelector` of `channel: staging`:
```yaml
apiVersion: aws.example.upbound.io/v1alpha1
kind: MyVPC
metadata:
  name: vpc-staging
spec:
  id: vpc-staging
  compositionRevisionSelector:
    matchLabels:
      channel: staging
```

Expected Output:
```shell
myvpc.aws.example.upbound.io/vpc-staging created
``` 

## Verify the Composite Resources
Verify the Composite Resource with the label `channel: staging` doesn't have a `REVISION`.  
All other XRs have a `REVISION` matching the created Composition Revision.
```shell
kubectl get composite -o="custom-columns=NAME:.metadata.name,SYNCED:.status.conditions[0].status,REVISION:.spec.compositionRevisionRef.name,POLICY:.spec.compositionUpdatePolicy,MATCHLABEL:.spec.compositionRevisionSelector.matchLabels"
```
Expected Output:
```shell
NAME          SYNCED   REVISION                                POLICY      MATCHLABEL
vpc-auto      True     myvpcs.aws.example.upbound.io-ad265bc   Automatic   <none>
vpc-dev       True     myvpcs.aws.example.upbound.io-ad265bc   Automatic   map[channel:dev]
vpc-man       True     myvpcs.aws.example.upbound.io-ad265bc   Manual      <none>
vpc-staging   False    <none>                                  Automatic   map[channel:staging]
``` 

{{< hint "note" >}}
The `vpc-staging` XR label doesn't match any existing Composition Revisions.
{{< /hint >}}

## Create new Composition revisions

### Update the Composition label
Update the `Composition` label to `channel: staging`:
```shell
kubectl label composition myvpcs.aws.example.upbound.io channel=staging --overwrite
```
Expected Output:
```shell
composition.apiextensions.crossplane.io/myvpcs.aws.example.upbound.io labeled
``` 

#### Verify the updated Composition revisions
Verify that Crossplane creates a new Composition revision:
```shell
kubectl get compositionrevisions -o="custom-columns=NAME:.metadata.name,REVISION:.spec.revision,CHANNEL:.metadata.labels.channel"
```
Expected Output:
```shell
NAME                                    REVISION   CHANNEL
myvpcs.aws.example.upbound.io-727b3c8   2          staging
myvpcs.aws.example.upbound.io-ad265bc   1          dev
``` 

Crossplane assigns the Composite Resources `vpc-auto` and `vpc-staging` to Composite revision:2.  
XRs `vpc-man` and `vpc-dev` are still assigned to the original revision:1:

```shell
kubectl get composite -o="custom-columns=NAME:.metadata.name,SYNCED:.status.conditions[0].status,REVISION:.spec.compositionRevisionRef.name,POLICY:.spec.compositionUpdatePolicy,MATCHLABEL:.spec.compositionRevisionSelector.matchLabels"
```
Expected Output:
```shell
NAME          SYNCED   REVISION                                POLICY      MATCHLABEL
vpc-auto      True     myvpcs.aws.example.upbound.io-727b3c8   Automatic   <none>
vpc-dev       True     myvpcs.aws.example.upbound.io-ad265bc   Automatic   map[channel:dev]
vpc-man       True     myvpcs.aws.example.upbound.io-ad265bc   Manual      <none>
vpc-staging   True     myvpcs.aws.example.upbound.io-727b3c8   Automatic   map[channel:staging]
``` 

{{< hint "note" >}}
`vpc-auto` always use the latest Revision.  
`vpc-staging` now matches the label applied to Revision revision:2.
{{< /hint >}}

### Update Composition Spec
Update the Composition to disable DNS support in the VPC and change the label from `staging` back to `dev`.

Apply the following changes to update the `Composition` spec and label:
```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  labels:
    channel: dev
  name: myvpcs.aws.example.upbound.io
spec:
  writeConnectionSecretsToNamespace: crossplane-system
  compositeTypeRef:
    apiVersion: aws.example.upbound.io/v1alpha1
    kind: MyVPC
  resources:
  - base:
      apiVersion: ec2.aws.upbound.io/v1beta1
      kind: VPC
      spec:
        forProvider:
          region: us-west-1
          cidrBlock: 192.168.0.0/16
          enableDnsSupport: false
          enableDnsHostnames: true
    name: my-vcp
```

Expected Output:
```shell
composition.apiextensions.crossplane.io/myvpcs.aws.example.upbound.io configured
``` 

#### Verify the updated Composition revisions
Verify that Crossplane creates a new Composition revision:

```shell
kubectl get compositionrevisions -o="custom-columns=NAME:.metadata.name,REVISION:.spec.revision,CHANNEL:.metadata.labels.channel"
```
Expected Output:
```shell
NAME                                    REVISION   CHANNEL
myvpcs.aws.example.upbound.io-727b3c8   2          staging
myvpcs.aws.example.upbound.io-ad265bc   1          dev
myvpcs.aws.example.upbound.io-f81c553   3          dev
``` 

Crossplane assigns the Composite Resources `vpc-auto` and `vpc-dev` to Composite revision:3.  
`vpc-staging` is assigned to revision:2, and `vpc-man` is still assigned to the origina revision:1:

```shell
kubectl get composite -o="custom-columns=NAME:.metadata.name,SYNCED:.status.conditions[0].status,REVISION:.spec.compositionRevisionRef.name,POLICY:.spec.compositionUpdatePolicy,MATCHLABEL:.spec.compositionRevisionSelector.matchLabels"
```
Expected Output:
```shell
NAME          SYNCED   REVISION                                POLICY      MATCHLABEL
vpc-auto      True     myvpcs.aws.example.upbound.io-f81c553   Automatic   <none>
vpc-dev       True     myvpcs.aws.example.upbound.io-f81c553   Automatic   map[channel:dev]
vpc-man       True     myvpcs.aws.example.upbound.io-ad265bc   Manual      <none>
vpc-staging   True     myvpcs.aws.example.upbound.io-727b3c8   Automatic   map[channel:staging]
``` 

{{< hint "note" >}}
`vpc-dev` matches the updated label applied to Revision revision:3.
`vpc-staging` matches the label applied to Revision revision:2.
{{< /hint >}}