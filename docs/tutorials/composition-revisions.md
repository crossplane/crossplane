# Composition Revisions in Action

In this tutorial we will discuss how CompositionRevisions work and how we can utilize them to manage Composite Resource
(XR) updates. We will start with a simple `Composition` and `CompositeResourceDefinition` that defines a `MyVPC`
resource, and we will change its labels and spec fields to see how XRs are updated.

To cover the different scenarios, we will create XRs with different `compositionUpdatePolicy` and `matchLabels`
configurations.

## Tutorial
1. Install an RC version of the Crossplane that includes relevant Composition Revision changes. Please note that, Crossplane
   v1.11.0 and above should already include them:
```console
kubectl create namespace crossplane-system
helm repo add crossplane-master https://charts.crossplane.io/master/
helm repo update
helm install crossplane --namespace crossplane-system crossplane-master/crossplane --devel --version 1.11.0-rc.0.108.g0521c32e
kubectl get pods -n crossplane-system
```
Expected Output:
```console
NAME                                       READY   STATUS    RESTARTS   AGE
crossplane-7f75ddcc46-f4d2z                1/1     Running   0          9s
crossplane-rbac-manager-78bd597746-sdv6w   1/1     Running   0          9s
```
2. Apply the following Composition and XRD. Note that Composition's labels will be automatically propagated to the relevant
   revision:
```console
echo 'apiVersion: apiextensions.crossplane.io/v1
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
    
---
    
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
            - id ' | kubectl apply -f -
```
Expected Output:
```console
composition.apiextensions.crossplane.io/myvpcs.aws.example.upbound.io created
compositeresourcedefinition.apiextensions.crossplane.io/myvpcs.aws.example.upbound.io created
```
3. Verify that composition revision is created:
```console
kubectl get compositionrevisions -o=custom-columns=NAME:.metadata.name,REVISION:.spec.revision,LABEL:.metadata.labels.channel
```
Expected Output:
```console
NAME                                    REVISION   LABEL
myvpcs.aws.example.upbound.io-ad265bc   1          dev
```
4. Create an XR with `compositionUpdatePolicy: Manual` and `compositionRevisionRef`:
```console
echo 'apiVersion: aws.example.upbound.io/v1alpha1
kind: MyVPC
metadata:
  name: vpc-man
spec:
  id: vpc-man
  compositionUpdatePolicy: Manual
  compositionRevisionRef:
    name: myvpcs.aws.example.upbound.io-ad265bc' | kubectl apply -f -
```
Expected Output:
```console
myvpc.aws.example.upbound.io/vpc-man created
``` 
5. Create an XR without `compositionUpdatePolicy` and selector; the update policy will be `Automatic` by default:
```console
echo 'apiVersion: aws.example.upbound.io/v1alpha1
kind: MyVPC
metadata:
  name: vpc-auto
spec:
  id: vpc-auto' | kubectl apply -f -
```
Expected Output:
```console
myvpc.aws.example.upbound.io/vpc-auto created
``` 
6. Create an XR with the `channel: dev` selector:
```console
echo 'apiVersion: aws.example.upbound.io/v1alpha1
kind:  MyVPC
metadata:
  name: vpc-dev
spec:
  id: vpc-dev
  compositionRevisionSelector:
    matchLabels:
      channel: dev' | kubectl apply -f -
```
Expected Output:
```console
myvpc.aws.example.upbound.io/vpc-dev created
``` 
7. Create an XR with the `channel: staging` selector:
```console
echo 'apiVersion: aws.example.upbound.io/v1alpha1
kind: MyVPC
metadata:
  name: vpc-staging
spec:
  id: vpc-staging
  compositionRevisionSelector:
    matchLabels:
      channel: staging' | kubectl apply -f -
```
Expected Output:
```console
myvpc.aws.example.upbound.io/vpc-staging created
``` 
8. Verify that all the XRs except with the `channel: staging` selector are bound to the revision:1 and
   there is no revision assigned for the XR with the `channel: staging`:
```console
kubectl get composite -o=custom-columns=NAME:.metadata.name,SYNCED:.status.conditions[0].status,REVISION:.spec.compositionRevisionRef.name,POLICY:.spec.compositionUpdatePolicy,MATCHLABEL:.spec.compositionRevisionSelector.matchLabels
```
Expected Output:
```console
NAME          SYNCED   REVISION                                POLICY      MATCHLABEL
vpc-auto      True     myvpcs.aws.example.upbound.io-ad265bc   Automatic   <none>
vpc-dev       True     myvpcs.aws.example.upbound.io-ad265bc   Automatic   map[channel:dev]
vpc-man       True     myvpcs.aws.example.upbound.io-ad265bc   Manual      <none>
vpc-staging   False    <none>                                  Automatic   map[channel:staging]
``` 
9. Update the `Composition` label to mark it as `channel: staging`:
```console
kubectl label composition myvpcs.aws.example.upbound.io channel=staging --overwrite
```
Expected Output:
```console
composition.apiextensions.crossplane.io/myvpcs.aws.example.upbound.io labeled
``` 
10. Verify that a new revision is created:
```console
kubectl get compositionrevisions -o=custom-columns=NAME:.metadata.name,REVISION:.spec.revision,LABEL:.metadata.labels.channel
```
Expected Output:
```console
NAME                                    REVISION   LABEL
myvpcs.aws.example.upbound.io-727b3c8   2          staging
myvpcs.aws.example.upbound.io-ad265bc   1          dev
``` 
11. Verify that `vpc-auto` and `vpc-staging` are assigned to the revision:2, and `vpc-man` and `vpc-dev` are still assigned to the revision:1:
```console
kubectl get composite -o=custom-columns=NAME:.metadata.name,SYNCED:.status.conditions[0].status,REVISION:.spec.compositionRevisionRef.name,POLICY:.spec.compositionUpdatePolicy,MATCHLABEL:.spec.compositionRevisionSelector.matchLabels
```
Expected Output:
```console
NAME          SYNCED   REVISION                                POLICY      MATCHLABEL
vpc-auto      True     myvpcs.aws.example.upbound.io-727b3c8   Automatic   <none>
vpc-dev       True     myvpcs.aws.example.upbound.io-ad265bc   Automatic   map[channel:dev]
vpc-man       True     myvpcs.aws.example.upbound.io-ad265bc   Manual      <none>
vpc-staging   True     myvpcs.aws.example.upbound.io-727b3c8   Automatic   map[channel:staging]
``` 
12. Apply the following changes to update the `Composition` spec and label:
```console
echo 'apiVersion: apiextensions.crossplane.io/v1
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
    name: my-vcp' | kubectl apply -f -
```
Expected Output:
```console
composition.apiextensions.crossplane.io/myvpcs.aws.example.upbound.io configured
``` 
13. Verify that a new revision is created:
```console 
kubectl get compositionrevisions -o=custom-columns=NAME:.metadata.name,REVISION:.spec.revision,LABEL:.metadata.labels.channel
```
Expected Output:
```console
NAME                                    REVISION   LABEL
myvpcs.aws.example.upbound.io-727b3c8   2          staging
myvpcs.aws.example.upbound.io-ad265bc   1          dev
myvpcs.aws.example.upbound.io-f81c553   3          dev
``` 
14. Verify that `vpc-auto` and `vpc-dev` are assigned to revision:3, `vpc-staging` is referring to revision:2, and `vpc-man`is still referring to revision:1:
```console
kubectl get composite -o=custom-columns=NAME:.metadata.name,SYNCED:.status.conditions[0].status,REVISION:.spec.compositionRevisionRef.name,POLICY:.spec.compositionUpdatePolicy,MATCHLABEL:.spec.compositionRevisionSelector.matchLabels
```
Expected Output:
```console
NAME          SYNCED   REVISION                                POLICY      MATCHLABEL
vpc-auto      True     myvpcs.aws.example.upbound.io-f81c553   Automatic   <none>
vpc-dev       True     myvpcs.aws.example.upbound.io-f81c553   Automatic   map[channel:dev]
vpc-man       True     myvpcs.aws.example.upbound.io-ad265bc   Manual      <none>
vpc-staging   True     myvpcs.aws.example.upbound.io-727b3c8   Automatic   map[channel:staging]
``` 