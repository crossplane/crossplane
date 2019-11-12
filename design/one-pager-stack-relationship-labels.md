# Stack Parent/Child Relationship Label Spec

- Owner: Steven Rathbauer (@rathpc)
- Reviewers: Crossplane Maintainers
- Status: Draft

## Proposal

[crossplaneio/crossplane#752](https://github.com/crossplaneio/crossplane/issues/752)

## Problem

There is currently no easy way to get a list of all resources that are owned by a parent resource.
We can leverage CRD labels to make this easier. As long as we have a standard format for doing so.

Per the proposal, it seems that the closest example of what I am referring to is shown on 
[this page](https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels) 
within the `app.kubernetes.io/part-of` label. However since we are not exactly using these labels 
for the intentions defined within that page, we should use something following the patterns instead.

As long as we define this label on all children CRD's, we should then be able to query all children
of a given parent object using those labels.

## Design

> _**Important Note**: These files are post processed yaml outputs, **NOT** static package files. The purpose of showing these is only to better outline and explain the relationships with respect to this specific example._

Wordpress example file tree structure of the example files below:

```text
stack.yaml
|
`-- crd.yaml
    |
    `-- resource.yaml
        |-- kubernetesapplication.yaml
        |-- kubernetescluster.yaml
        |-- mysqlinstance.yaml
        |-- ...
```

### Example wordpress `stack.yaml`

```yaml
apiVersion: stacks.crossplane.io/v1alpha1
kind: Stack
metadata:
  creationTimestamp: "2019-10-23T23:47:38Z"
  generation: 1
  name: sample-stack-wordpress
  namespace: app-project1-dev
  ownerReferences:
  - apiVersion: stacks.crossplane.io/v1alpha1
    kind: StackInstall
    name: sample-stack-wordpress
    uid: 86c89c94-cfc0-474e-b3b3-beffc09e7793
  resourceVersion: "6431"
  selfLink: /apis/stacks.crossplane.io/v1alpha1/namespaces/app-project1-dev/stacks/sample-stack-wordpress
  uid: ec52c8c2-379f-45ec-9458-e40f070f8d2e
  labels:
    app.kubernetes.io/managed-by: stack-manager
...
```

### Example wordpress `crd.yaml`

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  creationTimestamp: null
  name: wordpressinstances.wordpress.samples.stacks.crossplane.io
  labels:
    core.crossplane.io/parent-kind: "stack.stacks.crossplane.io"
    core.crossplane.io/parent-name: "sample-stack-wordpress"
    core.crossplane.io/parent-namespace: "app-project1-dev"
    app.kubernetes.io/managed-by: stack-manager
...
```

### Example wordpress `resource.yaml`

```yaml
apiVersion: wordpress.samples.stacks.crossplane.io/v1alpha1
kind: WordpressInstance
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {
        "apiVersion":"wordpress.samples.stacks.crossplane.io/v1alpha1",
        "kind":"WordpressInstance",
        "metadata":{
          "annotations":{},
          "name":"my-wordpressinstance",
          "namespace":"app-project1-dev"
        }
      }
  creationTimestamp: "2019-10-23T23:47:40Z"
  generation: 1
  name: my-wordpressinstance
  namespace: app-project1-dev
  resourceVersion: "6445"
  selfLink: /apis/wordpress.samples.stacks.crossplane.io/v1alpha1/namespaces/app-project1-dev/wordpressinstances/my-wordpressinstance
  uid: f2d13a15-1f9b-40a7-a173-a40abefa61bf
  labels:
    core.crossplane.io/parent-kind: "customresourcedefinition.apiextensions.k8s.io"
    core.crossplane.io/parent-name: "sample-stack-wordpress"
    core.crossplane.io/parent-namespace: "app-project1-dev"
    app.kubernetes.io/managed-by: stack-manager
...
```

### Example wordpress `kubernetesapplication.yaml`

```yaml
apiVersion: workload.crossplane.io/v1alpha1
kind: KubernetesApplication
metadata:
  name: wordpress-app-wordpress
  labels:
    stack: sample-stack-wordpress
    core.crossplane.io/parent-kind: "wordpressinstances.wordpress.samples.stacks.crossplane.io"
    core.crossplane.io/parent-name: "my-wordpressinstance"
    core.crossplane.io/parent-namespace: "app-project1-dev"
    app.kubernetes.io/managed-by: stack-manager
...
```

### Example wordpress `kubernetescluster.yaml`

```yaml
apiVersion: compute.crossplane.io/v1alpha1
kind: KubernetesCluster
metadata:
  name: wordpress-cluster-wordpress
  labels:
    stack: sample-stack-wordpress
    core.crossplane.io/parent-kind: "wordpressinstances.wordpress.samples.stacks.crossplane.io"
    core.crossplane.io/parent-name: "my-wordpressinstance"
    core.crossplane.io/parent-namespace: "app-project1-dev"
    app.kubernetes.io/managed-by: stack-manager
...
```

### Example wordpress `mysqlinstance.yaml`

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: wordpress-mysql-wordpress
  labels:
    stack: sample-stack-wordpress
    core.crossplane.io/parent-kind: "wordpressinstances.wordpress.samples.stacks.crossplane.io"
    core.crossplane.io/parent-name: "my-wordpressinstance"
    core.crossplane.io/parent-namespace: "app-project1-dev"
    app.kubernetes.io/managed-by: stack-manager
...
```