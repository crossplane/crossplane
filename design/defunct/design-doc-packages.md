# Crossplane Packages
* Owner: Jared Watts (@jbw976)
* Reviewers: Crossplane Maintainers
* Status: Defunct

This document aims to provide details about the experience and implementation for Crossplane Packages, which can add new functionality/support, types, and controllers to Crossplane.

## Revisions

Packages include the version of this document that they follow within
their primary metadata file, currently the `apiVersion` field of `/.registry/app.yaml`.

* 1.1
  * Renamed Extensions concept to Stacks (`Stack` code references are unaffected) [#571](https://github.com/crossplane/crossplane/issues/571)
  * Added additional Questions and Open Issues
* 1.2 - Dan Mangum ([@hasheddan](https://github.com/hasheddan))
  * Renamed `ClusterPackageInstall` / `PackageInstall` / `Stack` to `ClusterPackageInstall` / `PackageInstall` / `Package`
  * Renamed stack manager to package manager

## Experience

The core experience for consuming new functionality in Crossplane is composed of 2 steps:

1. Create a package install object for the name of the Crossplane Package or one of the CRDs that it owns
    1. e.g., GitLab or `gitlabcluster.gitlab.com/v1alpha1`
1. Create a CRD instance that the custom controller owns
    1. e.g., GitLab CRD instance

After step 1, the required types and controllers are available in the Crossplane cluster and this step only needs to be done once.

After step 2, the controller (or other supported “runner”) from the package performs the necessary operations to create workloads, claims, etc. that bring the users desired state captured in the CRD to reality.
Step 2 can be repeated many times to provision multiple "instances" of the types that the package introduces to the cluster.

## Terminology

* **Custom Resource Definition** - A standard Kubernetes CRD, which defines a new type of resource that can be managed declaratively. This serves as the unit of management in Crossplane.  The CRD is composed of spec and status sections and supports API level versioning (e.g., v1alpha1)
  * Atomic / External CRDs - usually represent external resources, and cannot be broken down any further (leaves)
  * Composite CRDs - these are also resources that capture parent/child relationships. They have a selector that can help query/find children resources.
  * Claim CRDs - these are abstract resources that bind to managed resources.
* **Custom Controllers** -- this is the implementation of one or more CRDs. Can be implemented in different ways, such as golang code (controller-runtime), templates, functions/hooks, templates, a new DSL, etc. The implementation itself is versioned using semantic versioning (e.g., v1.0.4)
* **Package** -- this is the unit of extending Crossplane with new functionality.  It is comprised of the CRDs, Custom Controller, and metadata about the Package.  A Crossplane cluster can be queried for all installed Packages as a way of learning what functionality a particular Crossplane supports.
* **Package Registry** -- this is a registry for Packages where they can be published, downloaded, explored, categorized, etc. The registry understands a Package’s custom controller and its CRDs and indexes by both -- you could lookup a custom controller by the CRD name and vice versa.
* **Package Manager (PM)** -- this is the component that is responsible for installing a Package’s custom controllers and resources in Crossplane. It can download packages, resolve dependencies, install resources and execute controllers.  This component is also responsible for managing the complete life-cycle of Packages, including upgrading them as new versions become available.
* **unpacking** -- the process of extracting the Package files.

These concepts comprise the extensibility story for Crossplane.  With them, users will be able to add new supported functionality of all varieties to Crossplane.
The currently supported functionality, such as PostgreSQL, Redis, etc. can be packaged and published using the concepts described above, so that the initial installation of Crossplane is very sparse.
Only the user’s desired functionality needs to be added on as needed basis (a la carte).

When Crossplane is initially created, we should consider only having a few key components installed and running:

* Core API types (CRDs)
* Scheduler
* Workload and Kubernetes cluster Controllers
* Package Manager (PM)

This would enable a user to create Kubernetes clusters and define workloads to be scheduled on them without having to install any Packages.
All further functionality for Crossplane (databases, buckets, etc.) could then be added through additional Packages custom controllers and resources that are installed and managed by the PM.

## Installation Flow

This section describes the end to end installation flow implemented by the Package Manager:

* The PM starts up with a default “source” registry (e.g. `registry.crossplane.io`) that contains packages (bundles of a Package and its custom controllers and CRDs) published to it
* User creates a custom resource instance that represents their desire to install a new Package in the cluster, for example `ClusterPackageInstall` or `PackageInstall`.
The CRD type used here will depend on what type of Package they wish to install, but will always include everything needed to successfully run that Package, such as:
  * an optional source registry that can be any arbitrary registry location.
    If a registry source is provided and the controller image does not include a
    source, the controller image will use the registry source provided in the PackageInstall.
    If this field is not specified then the PM's default source registry
    will be used.
  * One of the following must be specified:
    * package name (`gitlab`) OR
    * CRD name (`gitlabcluster.gitlab.com/v1alpha1`)
      * Note: this code path is exactly the same as dependency resolution
* The PM performs dependency resolution that determines all CRDs and controllers that are required by this Package and any other dependencies (Not Implemented)
* The PM pulls all necessary Packages from the registry
* The PM creates an unpack job that sends the artifacts to `stdout` which the PM ingests to install the Package
  * Package metadata (`app.yaml`, `install.yaml`) is extracted and transformed to create an `Package` CRD instance that serves as a record of the install
  * All owned/defined CRDs are installed and annotated with their related metadata (`group.yaml`, `resource.yaml`, and icon file)
  * [RBAC rules](./one-pager-packages-security-isolation.md#allowed-resource-access) necessary for the controller or controller installer are installed
  * Package installation instructions (`install.yaml`), in the form of Kubernetes YAML state files, are parsed and sent to the Kubernetes API
* Kubernetes starts up the custom controller so that it is in the running state
* The PM marks the `PackageInstall` status as succeeded

## `ClusterPackageInstall` and `PackageInstall` CRDs

To commence the installation of new functionality into a Crossplane cluster, an instance of one of the package install CRDs should be created.
The currently supported package install types are `ClusterPackageInstall` and `PackageInstall`, which each have varying scope of permissions within the control plane.
More details can be read in the [security and isolation design doc](./one-pager-packages-security-isolation.md).
The PM will be watching for events on these types and it will begin the process of installing a Package during its reconcile loop.

Package install CRDs can be specified by either a package name or by a CRD type.
When given a CRD type, the controller will query the registry to find out what package owns that CRD and then it will download that package to proceed with the install.
This gives more flexibility to how Packages are installed and does not require the requestor to know what package a CRD is defined in.

```yaml
# Install a package into Crossplane from a package,
# using a specific version number.
# This package will be installed at the cluster scope.
apiVersion: packages.crossplane.io/v1alpha1
kind: ClusterPackageInstall
metadata:
  name: gcp-from-package
spec:
  source: registry.crossplane.io
  package: crossplane/provider-gcp:v0.1.0
status:
  conditions:
  - type: Ready
    status: "True"
---
# Install a package into Crossplane by specifying a CRD that
# the package defines/owns.
# This package will be installed at a namespace scope.
apiVersion: packages.crossplane.io/v1alpha1
kind: PackageInstall
metadata:
  name: wordpress-from-crd
spec:
  source: registry.crossplane.io
  crd: wordpressinstances.wordpress.samples.packages.crossplane.io/v1alpha1
status:
  conditions:
  - type: Creating
    status: "True"
```

## `Package` CRD

The `Package` CRD serves as a record of an installed Package (a custom controller and its CRDs).
These records make it so that a user or system can query Crossplane and learn all of the functionality that has been installed on it as well as their statuses.

Instances of this CRD can be generated from the filesystem based contents of a package, i.e. the metadata files contained inside the package.
This can be thought of as a translation operation, where the file based content is translated into a YAML based version that is stored in the `Package` CRD.

`Package` CRD instances can also be created directly by a user without any knowledge of packages at all.
They can directly create any CRDs that their Package requires and then create a `Package` CRD instance that describes their Package, its custom controller, etc.
The Package Manager will see this new instance and take the steps necessary to ensure the custom controller is running in the cluster and the Package’s functionality is available.

```yaml
apiVersion: packages.crossplane.io/v1alpha1
kind: Package
metadata:
 name: redis
spec:
 # these are references to CRDs for the resources exposed by this package
 # by convention they are bundled in the same Package as this package
 customresourcedefinitions:
 - kind: RedisCluster
   apiVersion: crossplane.redislabs.com/v1alpha1
 dependsOn: []
  # CRDs that this package depends on (required) are listed here
  # this data drives the dependency resolution process
 title: Redis package for Crossplane
 overviewShort: "One sentence in plain text about how Redis is a really cool database"
 overview: |
   Plain text about how Redis is a really cool database.
   This also describes how this package relates to Redis.
 readme: |
  ## README

  * Uses markdown
  * Describes the package
  * Describes features and how to get help
 category: Database
 version: 0.1.0
 icons:
 - base64data: iVBORw0KGgoAAAANSUhEUgAAAOEAAADZCAYAAADWmle6AAAACXBIWXMA
   mediatype: image/png
 maintainers:
 - name: Rick Kane
   email: rick@foo.io
 owners:
 - name: Chandler
   email: chandler@bar.io
 keywords:
 - "databases"
 website: "https://redislabs.com/"
 # the implementation of the package, i.e. a controller that will run
 # on the crossplane cluster
 controller:
  deployment:
    name: redis-controller
    spec:
      replicas: 1
      selector:
        matchLabels:
          core.crossplane.io/name: "redis"
      template:
        metadata:
          name: redis-controller
          labels:
            core.crossplane.io/name: "redis"
        spec:
          serviceAccountName: redis-controller
          containers:
          - name: redis-controller
            image: redis/redis-crossplane-controller:2.0.9
            imagePullPolicy: Always
            env:
```

### Modifying the Package via PackageInstall

The PackageInstall spec can include parameters to adjust the composition of the
Package resource and behaviors of the PackageManager and the Package's controller.

For example, a private registry can be used as the source of the Package
and the controller.  Both OCI images can be fetched using the necessary
registry secret and an appropriate pull policy using PackageInstall parameters.
The service account that is created to run the Package controller can also be
modified from the PackageInstall resource.

```yaml
apiVersion: packages.crossplane.io/v1alpha1
kind: PackageInstall
spec:
  source: private.registry.example.com
  package: private-user/image
  imagePullSecrets:
  - name: private-registry-secret
  imagePullPolicy: Always
  serviceAccount:
    annotations:
      iam-service-annotation: "special-value"
```

The PM, when handling this PackageInstall resource, will produce a Package resource
that has been modified in the following key ways:

```yaml
apiVersion: packages.crossplane.io/v1alpha1
kind: Package
spec:
  controller:
    deployment:
      spec:
        template:
          spec:
            containers:
            - image: private.registry.example.com/...
              imagePullPolicy: Always
            imagePullSecrets:
            - name: private-registry-secret
  serviceAccount:
    annotations:
      iam-service-annotation: "special-value"
```

The Package and controller image will be prefixed with the `PackageInstall`
`source` if provided. If the controller image already includes a registry, the
`source` will not be used for the controller image. `StackDefinition` resources
produced by the PM will also be affected. Their `behavior` images and
the generic templating-controller will be fetched from the same source.

The `ServiceAccount` created to run the controller deployment will include the
specified annotations when `serviceAccount.annotations` are provided in the
`PackageInstall`.

The `Deployment` created for the Package controller will include the
`PackageInstall` specified `imagePullPolicy` and `imagePullSecrets`.

The `initContainer` of the `Job` used by the PM, which copies the package files
to a shared volume, will use the `imagePullPolicy` and `imagePullSecrets`
specified in the `PackageInstall` to access the package image. The PM's package
processing (`package unpack`) container used in this `Job` will, however, rely on
the same `imagePullPolicy` used on the PM responsible for processing the
`PackageInstall`.

## Package Format

A Package is the bundle that contains the custom controller definition, CRDs, icons, and other metadata.

The Packaget is essentially just a tarball (e.g., a [container image](https://github.com/opencontainers/image-spec/blob/master/spec.md)).  All of the Package resources are brought together into this single unit which is understood and supported by the Package registry and Package manager.

As previously mentioned, after downloading and unpacking a Package, the Package Manager will not only install its contents into Crossplane, but it will also translate them into a `Package` record.

More details will be provided when a Package registry project is bootstrapped and launched.

### Package Filesystem Layout

Inside of a package, the filesystem layout shown below is expected for the best experience.  This layout will accommodate current and future Package consuming tools, such as client tools, back-end indexing and cataloging tools, and the Package Manager itself.

```text
.registry/
├── icon.svg
├── app.yaml # Application metadata.
├── install.yaml # Optional install metadata.
├── ui-schema.yaml #  Optional UI Metadata
└── resources
      └── databases.foocompany.io # Group directory
            ├── group.yaml # Optional Group metadata
            ├── icon.svg # Optional Group icon
            └── mysql # Kind directory by convention
                ├── v1alpha1
                │   ├── mysql.v1alpha1.crd.yaml # Required CRD
                │   ├── icon.svg # Optional resource icon
                │   └── resource.yaml # Resource level metadata.
                └── v1beta1
                    ├── mysql.v1beta1.crd.yaml
                    ├── ui-schema.yaml #  Optional UI Metadata, optionally prefixed with kind and version separated by periods
                    ├── icon.svg
                    └── resource.yaml
```

In this example, the directory names "databases.foocompany.io", "mysql", "v1alpha1", and "v1alpha2" are for human-readability and should be considered arbitrary.  The group, kind, and version data will be parsed from any leaf level CRD files ignoring the directory names.

### Package Files

* `app.yaml`: This file is the general metadata and information about the Package, such as its name, description, version, owners, etc.  This metadata will be saved in the `Package` record's spec fields.
* `install.yaml`: This file contains the information for how the custom controller for the Package should be installed into Crossplane.  Initially, only simple `Deployment` based controllers will be supported, but eventually other types of implementations will be supported as well, e.g., templates, functions/hooks, templates, a new DSL, etc.
* `icon.svg`: This file (or `icon.png`, `icon.jpg`, `icon.gif`, or potentially other supported filenames, TBD) will be used in a visual context when listing or describing this package.  The preferred formats/filenames are `svg`, `png`, `jpg`, `gif` in that order (if multiple files exist).  For bitmap formats, the width to height ratio should be 1:1. Limitations may be placed on the acceptable file dimensions and byte size (TBD).
* `resources` directory: This directory contains all the CRDs and optional metadata about them.
  * `group.yaml`: Related Package resources (CRDs) can be described at a high level within a group directory using this file.
  * `*resource.yaml` and `*icon.svg`: Files that describe the resource with descriptions, titles, or images, may be used to inform out-of-cluster or in-cluster Package managing tools.  This may affect the annotations of the `Package` record or the Resource CRD (TBD).
    * Multiple `*resource.yaml` files may exist alongside multiple CRD files in the same directory.  The `resource.yaml` files should include an `id:` referencing the `Kind` of their matching CRD.
    * Multiple `*icon.svg` files may be included in the same directory as the CRD yaml files they modify. In this case the filename should be prefixed to match the `Kind` of the associated CRD (`mytype.icon.svg`).
  * `*ui-schema.yaml`: UI metadata that will be transformed and annotated according to the [Package UI Metadata One Pager](one-pager-stack-ui-metadata.md)
    * Multiple `*ui-schema.yaml` files may be included in the same directory as the CRD yaml files they modify. In this case the filename should be prefixed to match the `Kind` of the associated CRD (`mytype.ui-schema.yaml`).
  * `*crd.yaml`: These CRDs are the types that the custom controller implements the logic for.  They will be directly installed into Crossplane so that users can create instances of them to start consuming their new Package functionality.  Notice that the filenames can vary from `very.descriptive.name.crd.yaml` to `crd.yaml`.
  Multiple CRDs can reside in the same file.  These CRDs may also be pre-annotated at build time with annotations describing the `resource.yaml`, `icon.svg`, and `ui-schema.yaml` files to avoid bundling additional files and incurring a minor processing penalty at runtime.

Examples of annotations that the Package manager produces are included in the [Example Package Files](example-package-files) section.  Icon annotations should be provided as [RFC-2397](https://tools.ietf.org/html/rfc2397) Data URIs and there is a strong preference that these URIs use base64 encoding.

The minimum required file tree for a single tool, such as the Package Manager, could be condensed to the following:

```text
.registry/
├── app.yaml
├── install.yaml
└── resources
      └── crd.yaml
```

Strictly speaking, `install.yaml` is optional, but a Package bereft of this file would only introduce a data storage CRD with no active controller to act on CRs.  A Package with no implementation could still be useful as a dependency of another Package if CRs or the CRD itself can influence the behavior of active Packages.

## Example Package Files

A concrete examples of this package format can be examined at <https://github.com/crossplane/sample-stack>.

A Git repository may choose to include the `.registry` directory with all of the files described above but that may not always be the case.  Packages are easy to create as build artifacts through a combination of shell scripting, Make, Docker, or any other tool-chain or process that can produce an OCI image.

An example project that processes the artifacts of Kubebuilder 2 to create a Package is available at <https://github.com/crossplane/app-wordpress>.

### Example `app.yaml`

```yaml
# This example conforms to version 0.1.0 of the package format
apiVersion: 0.1.0

# Version of project (optional)
# If omitted the version will be filled with the docker tag
# If set it must match the docker tag
version: 0.0.1

# Human readable title of application.
title: Sample Crossplane Stack

overviewShort: A one line plain text explanation of this package
overview: |
  Multiple line plain text description of this package.

# Markdown description of this entry
readme: |
 *Markdown* describing the sample Crossplane Stack project in more detail

# Maintainer names and emails.
maintainers:
- name: Jared Watts
  email: jared@upbound.io

# Owner names and emails.
owners:
- name: Bassam Tabbara
  email: bassam@upbound.io

# Human readable company name.
company: Upbound

# A single category that best fits this Package
# Arbitrary categories may be used but it is expected that a preferred set of categories will emerge among Package tools
category: Database

# Keywords that describe this application and help search indexing
keywords:
- "samples"
- "examples"
- "tutorials"

# Links to more information about the application (about page, source code, etc.)
website: "https://crossplane.io"
source: "https://github.com/crossplane/sample-stack"

# License SPDX name: https://spdx.org/licenses/
license: Apache-2.0

# Type of package represented. Supported values are:
#
# - Provider
# - Stack
# - Application
# - Addon
packageType: Provider


# Scope of roles needed by the package once installed in the control plane,
# current supported values are:
#
# - Cluster
# - Namespaced
permissionScope: Cluster

# Dependent CRDs will be coupled with Owned CRDs and core resources to produce
# RBAC rules. All verbs will be permitted.
dependsOn:
- crd: "mytype.mygroup.example.org/v1alpha1"
- crd: '*.yourstack.example.org/v1alpha1'

```

### Example `install.yaml`

The `install.yaml` file is expected to conform to a standard Kubernetes
YAML file describing a single `Deployment` object. The only exception is
that the `image` field in the container spec objects is optional. If it
is omitted, it will be filled in by the package manager with the value of
the image specified in the package install object used to install the
package. Leaving it out makes it easier to manage the versions of images
if the CRDs and the controller live in the same image.

Additionally, if an image is specified for the `Deployment` object, and
the package is installed with an explicit source registry, the registry
will be injected into the image field if it does not already specify a
registry.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: crossplane-sample-stack
  labels:
    core.crossplane.io/name: "crossplane-sample-stack"
spec:
  selector:
    matchLabels:
      core.crossplane.io/name: "crossplane-sample-stack"
  replicas: 1
  template:
    metadata:
      name: sample-stack-controller
      labels:
        core.crossplane.io/name: "crossplane-sample-stack"
    spec:
      containers:
      - name: sample-stack-controller
        # The `image:` field is optional. If omitted, it will be
        # filled in by the package manager, using the same image
        # name and tag as the package on the PackageInstall object.
        # We recommend omitting it unless you know you need it.
        #
        # image: crossplane/sample-stack:latest
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
```

### Example `group.yaml`

```yaml
group: databases.foocompany.io
title: Sample Controller Types
overviewShort: This is a one line plain text description of this resource group
overview: |
  This is a group description.
  Talk about the types of CRDs in this group.
readme: |
  # MySQL Resource Group by Foo Company

  This is a Crossplane Package resource group for FooCompany MySQL
```

### Example `resource.yaml`

```yaml
id: mysql
title: MySQL
titlePlural: MySQL Instances
category: Database
overviewShort: Overview of this resource in FooCompany MySQL
overview: |
  Longer plain text description.

  Some detail.
readme: |
  # MySQL Resource by FooCompany

  This is the Crossplane Package mysql crd for FooCompany MySQL

  ## Details

  More markdown.

  ## Examples

    apiVersion: mysql.stacks.foocompany.example/v1
    kind: mysql
```

### Example `crd.yaml`

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  labels:
    controller-tools.k8s.io: "1.0"
  name: mytypes.samples.crossplane.io
spec:
  group: samples.crossplane.io
  names:
    kind: Mytype
    plural: mytypes
  scope: Namespaced
  version: v1alpha1
```

#### Example CRD with Package metadata

It is the job of the PM or a Package build tool to process the Package metadata
files and the source `*.crd.yaml` files into a final CRD installed in the
cluster.  The annotations that are applied will assist Package tools and users
in the discovery and identification of cluster resources that are both currently
managed by and can be managed by Crossplane.

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: mytypes.samples.crossplane.io
  labels:
    controller-tools.k8s.io: "1.0"
    app.kubernetes.io/managed-by: package-manager
  annotations:
    packages.crossplane.io/package-title: "Crossplane Sample Stack"
    packages.crossplane.io/group-title: "Title of the Group"
    packages.crossplane.io/group-overview: "Overview of the Group"
    packages.crossplane.io/group-overview-short: "Short overview of the Group"
    packages.crossplane.io/group-readme: "Readme of the Group"
    packages.crossplane.io/resource-category: "Databases"
    packages.crossplane.io/resource-title: "Example Resource"
    packages.crossplane.io/resource-title-plural: "Example Resources"
    packages.crossplane.io/resource-overview: "Overview of the Resource"
    packages.crossplane.io/resource-overview-short: "Short overview of the Resource"
    packages.crossplane.io/resource-readme: "Readme of the Resource"
    packages.crossplane.io/icon-data-uri: data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciLz4K
    packages.crossplane.io/ui-schema: |
      version: 0.5
      configSections:
      - title: Configuration
        description: Enter information specific to the configuration you wish to create.
        items:
        - name: dbReplicas
          controlType: singleInput
          type: integer
          path: .spec.dbReplicas
          title: DB Replicas
          description: The number of DB Replicas
          default: 1
          validation:
          - minimum: 1
          - maximum: 3
      ---
      additionalSpec: example
      stillYaml: true
      usesSameSpecConvention: "not necessary"
  ownerReferences:
  - apiVersion: packages.crossplane.io/v1alpha1
    kind: PackageInstall
    name: stack-wordpress
    uid: 1eb30f04-afdf-4282-bfd2-d0fb924f65d9
spec:
  group: samples.crossplane.io
  names:
    kind: Mytype
    plural: mytypes
  scope: Namespaced
  version: v1alpha1
```

#### Package CRD Labels

The labels added by Crossplane tools serve the following purpose:

Label | Value | Purpose
---|--- | ---
`app.kubernetes.io/managed-by` | `package-manager` | This [recommended label](https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/) identifies the tool being used to manage the operation of the application.

_Labels and annotations provided in the source CRD are preserved, such as
`controller-tools.k8s.io` in the example above._

#### Package CRD Annotations

The annotations added by Crossplane tools reflect the following metadata values:

Annotation | Metadata Source | Value from Source
---|---|---
`packages.crossplane.io/package-title` | `app.yaml` | `title`
`packages.crossplane.io/group-title` | `group.yaml` | `title`
`packages.crossplane.io/group-overview` | `group.yaml` | `overview`
`packages.crossplane.io/group-overview-short` | `group.yaml` | `overviewShort`
`packages.crossplane.io/group-readme` | `group.yaml` | `readme`
`packages.crossplane.io/resource-category` | `resource.yaml` | `category`
`packages.crossplane.io/resource-title` | `resource.yaml` | `title`
`packages.crossplane.io/resource-title-plural` | `resource.yaml` | `titlePlural`
`packages.crossplane.io/resource-overview` | `resource.yaml` | `overview`
`packages.crossplane.io/resource-overview-short` | `resource.yaml` | `shortOverview`
`packages.crossplane.io/resource-readme` | `resource.yaml` | `readme`
`packages.crossplane.io/ui-schema` | `ui-schema.yaml` | Multi-document concatenation
`packages.crossplane.io/icon-data-uri` | `icon.svg` | Data URI

## Dependency Resolution

When a Package requires types and functionality defined by another Package, this dependency needs to be resolved and fulfilled.
All dependencies are expressed by the CRDs that are required, as opposed to the Package that defines them.
Packages are units of publishing, pulling and versioning, they are not a unit of consumption.

Therefore, If the required CRDs don’t exist, the registry must be queried for what package defines them.
The registry maintains an index of package contents so that it can easily answer questions like this.
The full set of Packages that another Package depends on will be downloaded, unpacked and installed before installation of the Package itself can proceed.

## Package Processing

The process of installing a Package involves downloading it, extracting the its contents into its relevant CRDs and `Package` record, and applying them to the Crossplane cluster.

See the [Installation Flow](#installation-flow) for a more complete view of the current package processing implementation.

The package manager uses a "Job initContainer and shared volume" approach which copies the package contents from the package initContainer to a shared volume.  The PM, using command arguments to the Crossplane container, performs processing logic over the shared volume contents. The artifacts of this are sent to `stdout` where the main entry-point of the Crossplane container parses the unpacking container's `stdout`.  The parsed artifacts are then sent to the Kubernetes API for install.

The processing/unpacking logic can easily move to its own image that can be used as a base layer in the future (or a CLI tool).  This approach is not very divergent from the current implementation which divides these functions through the use of image entry-point command arguments.

A key aspect of the current implementation is that it takes advantage of the existing machinery in Kubernetes around container image download, verification, and extraction.
This is much more efficient and reliable than writing new code within Crossplane to do this work.

### Packaging Considerations

All packages should strive to minimize the amount of YAML they require.  Packages should avoid including any files that are not necessary for the Package metadata or the controller, when including the controller in the same image.

Some packaging considerations do not need to be enforced by spec and are left to the Package developer.

* Package metadata may be bundled with the controller image, but this does not need to be the case nor should it be enforced one way or the other.
  * There may be benefits to metadata-only Package images and their small byte sizes.
  * There is a benefit to requiring less image fetching by the container runtime
  * There may be benefits to maintaining fewer images (combined Package metadata + controller image vs separate Package metadata and controller images)

### Alternate Packaging Designs

Alternative designs for package processing and their related considerations (pros and cons) are listed below.  These ideas or parts of them may surface in future implementations.

* Package base image
  * A base image containing the unpacking logic could be defined and all other Packages are based on it, e.g., `FROM crossplane/package`
  * The main entry point of the package would call this unpacking logic and send all artifacts to `stdout`
  * PRO: The knowledge to unpack an image is self-contained an external entity such as the PM does not need to know these details, package format is opaque to the PM.
  * CON: This likely significantly increases the size of the package if the logic is written in golang using Crossplane types for unmarshalling, increasing the KB size of the original package contents into an image that is many MB in size.
* Job initContainer and shared volume
  * Package images only contain the package contents, no processing logic is included in the package
  * The PM starts a job pod with an init container for the package image and copies all its contents to a shared volume in the job pod.  The Crossplane package processing logic runs in the main pod container and runs over the shared volume, sending all artifacts to `stdout`.
  * PRO: Package images are significantly smaller since they will only contain yaml files and icons, no binaries or libraries.
  * PRO: The processing/unpacking logic can easily move to its own image that can be used as a base layer in the future (or a CLI tool), so this approach is not very divergent.
  * CON: The PM needs to understand package content and layout.
* CLI tool
  * The unpacking logic could be built into a CLI tool that can be run over a package image.
  * PRO: This is the most versatile option as it can be used in contexts outside of a Crossplane cluster.
  * CON: Doesn't integrate very cleanly into the Crossplane mainline scenario with the PM.

Each of these designs offered a good place to start.  Through iteration over time we will learn more, hopefully without investing much effort that cannot be reused.

## Security and Isolation

Details on the installation and runtime security and isolation of packages can be read in the [security and isolation design doc](./one-pager-packages-security-isolation.md).

## Questions and Open Issues

* Offloading redundant Package Manager functionality to Package building tools
* Dependency resolution design: [#434](https://github.com/crossplane/crossplane/issues/434)
* Updating/Upgrading Package: [#435](https://github.com/crossplane/crossplane/issues/435)
* Support installation of packages from private registries [#505](https://github.com/crossplane/crossplane/issues/505)
* Figure out model for crossplane core vs packages [#531](https://github.com/crossplane/crossplane/issues/531)
* Prototype alternate package implementations [#548](https://github.com/crossplane/crossplane/issues/548)
* Is there a benefit to `kind.version.` prefixed `crd.yaml` filenames
  * Should this be the only name prefix?
  * Should this be the primary means of disambiguating related CRD, UI, and Resource files in the same directory to each other?
* What categories are valid? Is there a well-defined Category tree? Are arbitrary categories invalid or ignored?
* Should links be predefined (`website`, `source`) or freeform `links:[{description:"Website",url:"..."}, ...]`?
