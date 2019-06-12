# Crossplane Extensions
* Owner: Jared Watts (@jbw976)
* Reviewers: Crossplane Maintainers
* Status: Accepted, revision 1.0

This document aims to provide details about the experience and implementation for Crossplane “extensions”, which can add new functionality/support, types, and controllers to Crossplane.

## Experience

The core experience for consuming new functionality in Crossplane is composed of 2 steps:

1. Create an extension request for the name of the extension or one of the CRDs that it owns
    1. e.g., GitLab or `gitlabcluster.gitlab.com/v1alpha1`
1. Create a CRD instance that the custom controller owns
    1. e.g., GitLab CRD instance

After step 1, the required types and controllers are available in the Crossplane cluster and this step only needs to be done once.

After step 2, the controller (or other supported “runner”) from the package performs the necessary operations to create workloads, claims, etc. that bring the users desired state captured in the CRD to reality.
Step 2 can be repeated many times to provision multiple "instances" of the types that the package introduces to the cluster.

## Terminology

* **Custom Resource Definition** - A standard Kubernetes CRD, which defines a new type of resource that can be managed declaratively. This serves as the unit of management in Crossplane.  Composed of a spec and a status section, supports API level versioning (e.g., v1alpha1)
  * Atomic / External CRDs - usually represent external resources, cannot be broken down any further (leaves)
  * Composite CRDs - these are also resources that capture parent/child relationships. They have a selector that can help query/find children resources.
  * Claim CRDs - these are abstract resources that bind to concrete resources.
* **Custom Controllers** -- this is the implementation of one or more CRDs. Can be implemented in different ways, such as golang code (controller-runtime), templates, functions/hooks, templates, a new DSL, etc. The implementation itself is versioned using semantic versioning (e.g., v1.0.4)
* **Extension** -- this is the unit of extending Crossplane with new functionality.  It is comprised of the CRDs, Custom Controller, and metadata about the extension.  A Crossplane cluster can be queried for all installed extensions as a way of learning what functionality a particular Crossplane supports.
* **Extension Registry** -- this is a registry for extensions where they can be published, downloaded, explored, categorized, etc. The registry understands an extension’s custom controller and its CRDs and indexes by both -- you could lookup a custom controller by the CRD name and vice versa.
* **Extension Package** -- this is a package format for extensions that contains the custom controller definition, metadata, CRDs, etc. It is essentially just a tarball (container image) that bundles all these resources together into a single unit. This is the packaging format that is supported by the registry and installer.
* **Extension Manager (EM)** -- this is the component that is responsible for installing an extension’s custom controllers and resources in Crossplane. It can download packages, resolve dependencies, install resources and execute controllers.  This component is also responsible for managing the complete lifecycle of extensions, including upgrading them as new versions become available.

These concepts comprise the extensibility story for Crossplane.  With them, users will be able to add new supported functionality of all varieties to Crossplane.
The currently supported functionality, such as PostgreSQL, Redis, etc. can be packaged and published using the concepts described above, so that the initial installation of Crossplane is very sparse.
Only the user’s desired functionality needs to be added on as needed basis (a la carte).

When Crossplane is initially created, we should consider only having a few key components installed and running:

* Core API types (CRDs)
* Scheduler
* Workload and Kubernetes cluster Controllers
* Extension Manager (EM)

This would enable a user to create Kubernetes clusters and define workloads to be scheduled on them without having to install any extensions.
All further functionality for Crossplane (databases, buckets, etc.) could then be added through additional extensions custom controllers and resources that are installed and managed by the EM.

## Installation Flow

This section describes the end to end installation flow implemented by the Extension Manager:

* The EM starts up with a default “source” registry (e.g. `registry.crossplane.io`) that contains packages (bundles of an extension and its custom controllers and CRDs) published to it
* User creates an `ExtensionRequest` instance to request an extension be installed in the cluster, which includes everything needed to successfully run that extension.  The `ExtensionRequest` includes:
  * an optional source registry that can be any arbitrary registry location.  If this field is not specified then the EM's default source registry will be used.
  * One of the following must be specified:
    * package name (`gitlab`) OR
    * CRD name (`gitlabcluster.gitlab.com/v1alpha1`)
      * Note: this code path is exactly the same as dependency resolution
* Performs dependency resolution that determines all packages/extensions that are required by this extension and all of its dependent extensions
* Pulls all necessary extension packages from the registry
* Unpacks them and installs all owned/defined CRDs and transforms the unpackaged content into an `Extension` CRD instance that serves as a record of the install
* Starts up the custom controller so that it is in the running state
* Marks the `ExtensionRequest` status as succeeded

## `ExtensionRequest` CRD

To commence the installation of new functionality into a Crossplane cluster, an instance of the `ExtensionRequest` CRD should be created.
The EM will be watching for events on this type and it will begin the process of installing an extension during its reconcile loop.

`ExtensionRequests` can be specified by either a package name or by a CRD type.
When given a CRD type, the controller will query the registry to find out what package owns that CRD and then it will download that package to proceed with the install.
This gives more flexibility to how extensions are installed and does not require the requestor to know what package a CRD is defined in.

```yaml
# request to extend Crossplane with the redis package,
# using a specific version number
apiVersion: extensions.crossplane.io/v1alpha1
kind: ExtensionRequest
metadata:
  name: redis-from-package
spec:
  source: registry.crossplane.io
  package: redis:v0.1.0
status:
  conditions:
  - type: Ready
    status: "True"
---
# request to extend Crossplane with the package that defines/owns,
# the rediscluster CRD
apiVersion: extensions.crossplane.io/v1alpha1
kind: ExtensionRequest
metadata:
  name: redis-from-crd
spec:
  source: registry.crossplane.io
  crd: redisclusters.cache.crossplane.io/v1alpha1
status:
  conditions:
  - type: Creating
    status: "True"
```

## `Extension` CRD

The `Extension` CRD serves as a record of an installed extension (a custom controller and its CRDs).
These records make it so that a user or system can query Crossplane and learn all of the functionality that has been installed on it as well as their statuses.

Instances of this CRD can be generated from the filesystem based contents of a package, i.e. the metadata files contained inside the package.
This can be thought of as a translation operation, where the file based content is translated into a YAML based version that is stored in the `Extension` CRD.

`Extension` CRD instances can also be created directly by a user without any knowledge of packages at all.
They can directly create any CRDs that their extension requires and then create an `Extension` CRD instance that describes their extension, its custom controller, etc.
The Extension Manager will see this new instance and take the steps necessary to ensure the custom controller is running in the cluster and the extension’s functionality is available.

```yaml
apiVersion: extensions.crossplane.io/v1alpha1
kind: Extension
metadata:
 name: redis
spec:
 # these are references to CRDs for the resources exposed by this extension
 # by convention they are bundled in the same Package as this extension
 customresourcedefinitions:
  owns:
  - kind: RedisCluster
    apiVersion: crossplane.redislabs.com/v1alpha1
  dependsOn: []
  # CRDs that this extension depends on (required) are listed here
  # this data drives the dependency resolution process
 title: Redis extension for Crossplane
 description: "Markdown syntax about how Redis is a really cool database"
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
 links:
 - description: About
   url: "https://redislabs.com/"
 # the implementation of the extension, i.e. a controller that will run
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
 # the permissions needed by the controller
 permissions:
   rules:
   - apiGroups:
     - ""
     resources:
     - secrets
     - serviceaccounts
     - events
     - namespaces
     verbs:
     - get
     - list
     - watch
     - create
     - update
     - patch
     - delete
```

## Extension Package Format

An extension package is simply the bundle that contains the custom controller definition, metadata, CRDs, etc. for a given extension.
It is essentially just a tarball (e.g., a container image) that packages all these resources together into a single unit and is the format that is understood and supported by the extension registry and extension manager.
More details will be provided when an extension registry project is bootstrapped and launched.
This section can be thought of as the initial thinking for the format of an extension package to start generating discussion and iteration.

As previously mentioned, after downloading and unpacking an extension package, the extension manager will not only install its contents into Crossplane, but it will also translate them into an `Extension` record.

Inside of a package, the following filesystem layout is expected:

```
.registry/
├── icon.jpg
├── app.yaml # Application metadata.
├── install.yaml # Optional install metadata.
├── rbac.yaml # Optional RBAC permissions.
└── resources
      └── databases.foocompany.io # Group directory
            ├── group.yaml # Optional Group metadata
            ├── icon.jpg # Optional Group icon
            └── mysql # Kind directory by convention
                ├── v1alpha1
                │   ├── mysql.v1alpha1.crd.yaml # Required CRD
                │   ├── icon.jpg # Optional resource icon
                │   └── resource.yaml # Resource level metadata.
                └── v1beta1
                    ├── mysql.v1beta1.crd.yaml
                    ├── icon.jpg
                    └── resource.yaml
```

* `app.yaml`: This file is the general metadata and information about the extension, such as its name, description, version, owners, etc.  This metadata will be saved in the `Extension` record's spec fields.
* `install.yaml`: This file contains the information for how the custom controller for the extension should be installed into Crossplane.  Initially, only simple `Deployment` based controllers will be supported, but eventually other types of implementations will be supported as well, e.g., templates, functions/hooks, templates, a new DSL, etc.
* `resources` directory: This directory contains all the CRDs and optional metadata about them.  These CRDs are the types that the custom controller implements the logic for.  They will be directly installed into Crossplane so that users can create instances of them to start consuming their new extension functionality.

### Example `app.yaml`

```yaml
# Human readable title of application.
title: Sample Crossplane Extension

# Markdown description of this entry
description: |
 Markdown describing this sample Crossplane extension project.

# Version of project (optional)
# If omitted the version will be filled with the docker tag
# If set it must match the docker tag
version: 0.0.1

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

# Keywords that describe this application and help search indexing
keywords:
- "samples"
- "examples"
- "tutorials"

# Links to more information about the application (about page, source code, etc.)
links:
- description: Website
  url: "https://upbound.io"
- description: Source Code
  url: "https://github.com/crossplaneio/sample-extension"

# License SPDX name: https://spdx.org/licenses/
license: Apache-2.0
```

## Dependency Resolution

When an extension requires types and functionality defined by another extension, this dependency needs to be resolved and fulfilled.
All dependencies are expressed by the CRDs that are required, as opposed to the extension or package that defines them.
Packages are units of publishing, pulling and versioning, they are not a unit of consumption.

Therefore, If the required CRDs don’t exist, the registry must be queried for what package defines them.
The registry maintains an index of package contents so that it can easily answer questions like this.
The full set of packages and their extensions that the extension depends on will be downloaded, unpacked and installed before installation of the extension itself can proceed.

## Package Processing

The process of installing an extension involves downloading its package, extracting the package contents into its relevant CRDs and `Extension` record, and applying them to the Crossplane cluster.
There are a few different options for this flow, which are described below, but a key aspect of the proposed implementation is to take advantage of the existing machinery in Kubernetes around container image download, verification, and extraction.
This will be much more efficient and reliable than writing new code within Crossplane to do this work ourselves.

Possible solutions for package processing are listed below, along with their pros and cons.

* Extension package base image
  * A base image containing the unpacking logic could be defined and all other extension packages are based on it, e.g., `FROM crossplane-extension`
  * The main entry point of the package would call this unpacking logic and send all artifacts to `stdout`
  * PRO: The knowledge to unpack an image is self-contained an external entity such as the EM does not need to know these details, package format is opaque to the EM.
  * CON: This likely significantly increases the size of the package if the logic is written in golang using Crossplane types for unmarshalling, increasing the KB size of the original package contents into an image that is many MB in size.
* Job initContainer and shared volume
  * Package images only contain the package contents, no processing logic is included in the package
  * The EM starts a job pod with an init container for the package image and copies all its contents to a shared volume in the job pod.  The Crossplane package processing logic runs in the main pod container and runs over the shared volume, sending all artifacts to `stdout`.
  * PRO: Package images are significantly smaller since they will only contain yaml files and icons, no binaries or libraries.
  * CON: The EM needs to understand package content and layout.
* CLI tool
  * The unpacking logic could be built into a CLI tool that can be run over a package image.
  * PRO: This is the most versatile option as it can be used in contexts outside of a Crossplane cluster.
  * CON: Doesn't integrate very cleanly into the Crossplane mainline scenario with the EM.

**Recommendation:** It is recommended to start with the "Job initContainer and shared volume" approach that copies the package contents from the package initContainer to a shared volume and has a Crossplane container main entry point perform the processing logic over the shared volume contents, sending out the artifacts to `stdout`.
Extension packages should only have a few KB of yaml files, so it's a significant concern to blow that image size up to many MB right now.
The processing/unpacking logic can easily move to its own image that can be used as a base layer in the future (or a CLI tool), so this approach is not very divergent.
It is a good place to start and iterate over as we learn more without investing much effort that cannot be reused.

## Questions and Open Issues

* Dependency resolution design: [#434](https://github.com/crossplaneio/crossplane/issues/434)
* Updating/Upgrading extensions: [#435](https://github.com/crossplaneio/crossplane/issues/435)
