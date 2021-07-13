# Stack Parent/Child Relationship Label Spec

- Owner: Steven Rathbauer ([@rathpc](https://github.com/rathpc))
- Reviewers: Crossplane Maintainers
- Status: Defunct

**_NOTE: The focus for this design is long term and may not be implemented immediately_**

## Revisions

* 1.1 - Dan Mangum ([@hasheddan](https://github.com/hasheddan))
  * Removed references to `core.crossplane.io/parent-uid` label as it no longer
    applied in order to enable backup and restore of stacks
    ([crossplane/crossplane#1389](https://github.com/crossplane/crossplane/issues/1389)).

## Proposal

[crossplane/crossplane#752](https://github.com/crossplane/crossplane/issues/752)

#### These are the labels we are proposing to add:

- **`core.crossplane.io/parent-group`**
- **`core.crossplane.io/parent-version`**
- **`core.crossplane.io/parent-kind`**
- **`core.crossplane.io/parent-name`**
- **`core.crossplane.io/parent-namespace`**

## Problem

There is currently no easy way to get a list of all resources that are owned by a parent resource.
We can leverage CRD labels to make this easier, as long as we have a standard format for doing so.
Additionally as long as we define this label on all children CRD's, we should then be able to query
all children of a given parent object using those labels.

Per the proposal, it seems that the closest example of what I am referring to is shown on
[this page][common-labels] within the `app.kubernetes.io/part-of` label. However since we are not
exactly using these labels for the intentions defined within that page, we should use something
following those patterns instead.

We had discussed using the _common labels_ as they are defined however after some deliberation we
decided the best course of action was to make our own. _<sup>[Additional Context &rarr;](#additionalContext)</sup>_

The primary reason for this was stated on [that page][common-labels]:

> ### _The metadata is organized around the concept of an application._

## Design

The group, version, and kind is captured as three individual fields to provide the most flexibility
for consumers of this metadata. Since consumers will have access to all 3 fields, they can on-demand
build any of the numerous GVK related formats available in the ecosystem. It was initially
considered to combine the group and version in the typical format of `my.group/v1alpha1`, but labels
cannot contain any slash characters. Therefore, storing the group, version, and kind as separate
fields seems reasonable.

> _**Important Note**: These files are post processed yaml outputs, **NOT** static package files.
> The purpose of showing these is only to better outline and explain the relationships with respect
> to this specific example._

Wordpress example file tree structure of the example files below:

```text
stackinstall.yaml
|
`-- crd.yaml
|
`-- stack.yaml
    |
    `-- wordpress.yaml
        |-- kubernetesapplication.yaml
        |-- kubernetescluster.yaml
        |-- mysqlinstance.yaml
        |-- ...
```

### Example wordpress `crd.yaml` (parented by `StackInstall`)

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  creationTimestamp: null
  name: wordpressinstances.wordpress.samples.stacks.crossplane.io
  labels:
    core.crossplane.io/parent-group: "stacks.crossplane.io"
    core.crossplane.io/parent-version: "v1alpha1"
    core.crossplane.io/parent-kind: "StackInstall"
    core.crossplane.io/parent-name: "sample-stack-wordpress"
    core.crossplane.io/parent-namespace: "app-project1-dev"
    app.kubernetes.io/managed-by: stack-manager
...
```

### Example wordpress `stack.yaml` (parented by `StackInstall`)

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
    core.crossplane.io/parent-group: "stacks.crossplane.io"
    core.crossplane.io/parent-version: "v1alpha1"
    core.crossplane.io/parent-kind: "StackInstall"
    core.crossplane.io/parent-name: "sample-stack-wordpress"
    core.crossplane.io/parent-namespace: "app-project1-dev"
...
```

### Example wordpress `wordpress.yaml` (parented by `Stack`)

```yaml
apiVersion: wordpress.samples.stacks.crossplane.io/v1alpha1
kind: WordpressInstance
metadata:
  creationTimestamp: "2019-10-23T23:47:40Z"
  generation: 1
  name: my-wordpressinstance
  namespace: app-project1-dev
  resourceVersion: "6445"
  selfLink: /apis/wordpress.samples.stacks.crossplane.io/v1alpha1/namespaces/app-project1-dev/wordpressinstances/my-wordpressinstance
  uid: f2d13a15-1f9b-40a7-a173-a40abefa61bf
  labels:
    core.crossplane.io/parent-kind: "Stack"
    core.crossplane.io/parent-group: "stacks.crossplane.io"
    core.crossplane.io/parent-version: "v1alpha1"
    core.crossplane.io/parent-name: "sample-stack-wordpress"
    core.crossplane.io/parent-namespace: "app-project1-dev"
    app.kubernetes.io/managed-by: stack-manager
...
```

### Example wordpress `kubernetesapplication.yaml` (parented by `WordpressInstance`)

```yaml
apiVersion: workload.crossplane.io/v1alpha1
kind: KubernetesApplication
metadata:
  name: wordpress-app-wordpress
  labels:
    stack: sample-stack-wordpress
    core.crossplane.io/parent-kind: "WordpressInstance"
    core.crossplane.io/parent-group: "wordpress.samples.stacks.crossplane.io"
    core.crossplane.io/parent-version: "v1alpha1"
    core.crossplane.io/parent-name: "my-wordpressinstance"
    core.crossplane.io/parent-namespace: "app-project1-dev"
    app.kubernetes.io/managed-by: stack-manager
...
```

### Example wordpress `kubernetescluster.yaml` (parented by `WordpressInstance`)

```yaml
apiVersion: compute.crossplane.io/v1alpha1
kind: KubernetesCluster
metadata:
  name: wordpress-cluster-wordpress
  labels:
    stack: sample-stack-wordpress
    core.crossplane.io/parent-kind: "WordpressInstance"
    core.crossplane.io/parent-group: "wordpress.samples.stacks.crossplane.io"
    core.crossplane.io/parent-version: "v1alpha1"
    core.crossplane.io/parent-name: "my-wordpressinstance"
    core.crossplane.io/parent-namespace: "app-project1-dev"
    app.kubernetes.io/managed-by: stack-manager
...
```

### Example wordpress `mysqlinstance.yaml` (parented by `WordpressInstance`)

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: wordpress-mysql-wordpress
  labels:
    stack: sample-stack-wordpress
    core.crossplane.io/parent-kind: "WordpressInstance"
    core.crossplane.io/parent-group: "wordpress.samples.stacks.crossplane.io"
    core.crossplane.io/parent-version: "v1alpha1"
    core.crossplane.io/parent-name: "my-wordpressinstance"
    core.crossplane.io/parent-namespace: "app-project1-dev"
    app.kubernetes.io/managed-by: stack-manager
...
```

-----

#### Footnotes

<a name="additionalContext">Additional Context</a>:
> _Citing **Daniel Suskin ([@suskin](https://github.com/suskin))**_

Essentially, because the labels are organized around the concept of an application, they do not
really fit our concept of tracing multiple levels of parent/child relationships. Consider
[the label descriptions][common-labels]. They are missing the concept of a hierarchy like what we're
trying to build. The label which comes the closest is `part-of`, which reads `The name of a higher
level application this one is part of` (example: `wordpress`). None of the other labels are for
describing a hierarchical relationship.

Let's make things more concrete with an example. We **could use** `part-of` and/or `instance` to
point to parents if we wanted to. But this is not ideal. Consider the following example hierarchy,
where `A -> B` means A is a parent of B:

```text
WordpressInstance -> KubernetesCluster -> KubernetesApplication -> KubernetesApplicationResource
```

Semantically, the `app.kubernetes.io` labels _"are organized around the concept of an application"_.
Since the resources in the example hierarchy are all part of the same application (Wordpress), they
should all have the same `instance` and `part-of` labels:

```yaml
# WordpressInstance labels
app.kubernetes.io/name: wordpress
app.kubernetes.io/instance: wordpress-instance-efafe342
app.kubernetes.io/component: application-claim
app.kubernetes.io/part-of: wordpress

# KubernetesCluster labels
app.kubernetes.io/name: kubernetes
app.kubernetes.io/instance: wordpress-instance-efafe342
app.kubernetes.io/component: cluster
app.kubernetes.io/part-of: wordpress

# KubernetesApplication labels
app.kubernetes.io/name: kubernetes-application
app.kubernetes.io/instance: wordpress-instance-efafe342
app.kubernetes.io/component: workload
app.kubernetes.io/part-of: wordpress

# KubernetesApplicationResource labels
app.kubernetes.io/name: kubernetes-resource
app.kubernetes.io/instance: wordpress-instance-efafe342
app.kubernetes.io/component: workload-resource
app.kubernetes.io/part-of: wordpress
```

However, what we want is for each one to point back one step in the hierarchy. And that would
contradict organizing the labels around an application. The `app.kubernetes.io` labels are still
useful, just not for describing a multi-level hierarchy.

-----

<!-- Recurring Links -->

[common-labels]: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels
