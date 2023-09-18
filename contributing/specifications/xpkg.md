# xpkg Specification

Crossplane supports the following types of [packages]:

- Providers
- Configurations
- Functions

These packages are distributed as generic [OCI images], which contain [YAML]
content informing the Crossplane package manager how to alter the state of a
cluster by installing objects that configure new resource types, and starting
controllers to reconcile them. An OCI image that contains valid Crossplane
package content is commonly referred to as an `xpkg` ("ex-package"). This
document provides the specification for a valid `xpkg`, which can be considered
a superset of the requirements detailed in the [OCI image specification]. It is
divided into two broad sections: requirements related to OCI image format and
requirements related to Crossplane `package.yaml` contents.

- [OCI Image Format](#oci-image-format)
  - [Indexes](#indexes)
  - [Manifests](#manifests)
  - [Configuration](#configuration)
  - [Layers](#layers)
- [package.yaml Contents](#packageyaml-contents)
  - [Configuration Package Requirements](#configuration-package-requirements)
  - [Provider Package Requirements](#provider-package-requirements)
  - [Object Annotations](#object-annotations)

## OCI Image Format

OCI images are comprised of [manifests], [configuration], and [layers].
Additionally, an image reference could refer to an image [index], which may
reference multiple image manifests and is frequently used for multi-platform
images. A valid Crossplane `xpkg` imposes various requirements on the components
of an OCI, each of which are described in the following sections.

### Indexes

The components of an `xpkg` that Crossplane interacts with do not contain any
platform-specific information, so Crossplane is broadly agnostic to the
formatting of an image index. Crossplane does impose the following requirements
on an image index:

- At least one (1) manifest MUST be referenced in the manifest descriptor array
  for a package to be successfully fetched and processed.

> The OCI image specification allows for zero-length manifest descriptor arrays
> in an index.

The following default behavior when interacting with image indexes is
implemented in the Crossplane package manager:

- If one manifest is referenced in the image index, the image it points to will
  be used.
- If multiple manifests are referenced in the image index, Crossplane will use
  the `linux/amd64` variant by default.

> It is important to note that the platform of the package image that is used by
> Crossplane does not necessarily mean that the same platform will be used for
> the controller if the package is a Provider. The decision of selecting a
> platform for a Provider controller image is deferred to the configured
> container runtime.

### Manifests

A manifest defines the layers and configuration of a specific image. Crossplane
is only concerned with the layer descriptors array in an image manifest and does
not impose additional requirements on any other portions of the manifest. The
following requirements are imposed on the layer descriptors array:

- One (1) layer descriptor in the array MAY have an [annotation] with key
  `io.crossplane.xpkg` and value `base`.
- Any number of layer descriptors in the array MAY have an annotation with key
  `io.crossplane.xpkg` and arbitrary value. Whether multiple layer descriptors
  may have the same value is left to the specification of the consumer of those
  layers.

> As evidenced by the fact that annotations are provided as a map of
> _string-string_, no single descriptor will contain multiple
> `io.crossplane.xpkg` annotations.

Crossplane is only concerned with the layer with the `base` annotation, and any
other layers with the `io.crossplane.xpkg` key are used to signify to
third-party consumers that a layer contains content related to the `xpkg` that
may be specific to a given consumer.

If no layer descriptors have an annotation in the form `io.crossplane.xpkg:
base`, the resultant filesystem from [applying changesets] from all layers will
be used. It is preferred to use layer descriptor annotations.

**Motivation**

Crossplane prefers the usage of annotated layer descriptors because it allows
for fetching and processing individual layers, rather than all layers in the
image. In the event that the image contains a single layer, this overhead is
minimal. However, larger images with many layers, whether they contain
third-party `xpkg` content or unrelated data, will result in multiple network
calls and more data to process.

Crossplane also prefers the usage of annotated layer descriptors to define
additive package content (i.e. third-party `xpkg` content) as it provides a
clean mechanism to build an `xpkg` through a series of stages. A valid `xpkg`
can be produced and later modified while verifying that the integrity of the
existing content is not violated, which ensures that Crossplane's package
manager will process the resulting `xpkg` in the same manner as the it would
prior to modification.

While not explicitly forbidden, modifying content from a preceding layer with
the `io.crossplane.xpkg` annotation in any subsequent layers is discouraged, as
it may lead to confusion if a third-party is consuming content from the
flattened filesystem.

### Configuration

Crossplane imposes no additional requirements on image configuration and does
not consider its contents when processing a package.

### Layers

As described above, Crossplane is only concerned with the single layer
referenced by the descriptor containing `io.crossplane.xpkg: base` if
distinguished. Crossplane imposes no additional restrictions on any other
layers, including those with a `io.crossplane.xpkg` annotation but a value other
than `base`, but does require the following of the `xpkg` base layer:

- A single file with name `package.yaml` MUST exist in the root directory of the
  `xpkg` base layer if distinguished, or in the root of the image filesystem
  after all layer changesets are applied.
- The `package.yaml` file MUST contain a valid [YAML stream].
- All other content in either the `xpkg` base layer, or the full image
  filesystem is ignored by Crossplane.

> The ability to use the image's flattened filesystem is primarily for backwards
> compatibility and is not encouraged, especially in the event that an image
> contains more than just `xpkg` related content, due to the fact that
> accidentally overwriting or modifying the `xpkg` layer contents in subsequent
> layers when constructing an image could cause the package to be invalid.

## package.yaml Contents

Depending on the type of package, the YAML stream in the `xpkg` base layer
`package.yaml` may contain different content. Additionally, the objects in the
YAML stream may contain common annotations that are suitable for the given
object type.

### Configuration Package Requirements

The `package.yaml` for Configuration packages must adhere to the following
requirements:

- One (1) and only one `Configuration.meta.pkg.crossplane.io` object MUST be
  defined in the YAML stream.
- Zero (0) or more `CompositeResourceDefinition.apiextensions.crossplane.io`
  objects MAY be defined in the YAML stream.
- Zero (0) or more `Composition.apiextensions.crossplane.io` objects MAY be
  defined in the YAML stream.
- Zero (0) other object types may be defined in the YAML stream.

### Provider Package Requirements

The `package.yaml` for Provider packages must adhere to the following
requirements:

- One (1) and only one `Provider.meta.pkg.crossplane.io` object MUST be defined
  in the YAML stream.
- Zero (0) or more `CustomResourceDefinition.apiextensions.k8s.io` objects MAY
  be defined in the YAML stream.
- Zero (0) or more `AdmissionWebhookConfiguration.admissionregistration.k8s.io`
  objects MAY be defined in the YAML stream.
- Zero (0) or more `MutatingWebhookConfiguration.admissionregistration.k8s.io`
  objects MAY be defined in the YAML stream.
- Zero (0) other object types may be defined in the YAML stream.

### Function Package Requirements

The `package.yaml` for Function packages must adhere to the following
requirements:

- One (1) and only one `Function.meta.pkg.crossplane.io` object MUST be defined
  in the YAML stream.
- Zero (0) or more `CustomResourceDefinition.apiextensions.k8s.io` objects MAY
  be defined in the YAML stream.
- Zero (0) other object types may be defined in the YAML stream.

Note that Function packages use CustomResourceDefinitions (CRD) only to deliver
schema for their input type(s). The package manager will not actually create the
supplied CRDs in the API server.

### Object Annotations

Though not used directly by Crossplane, the following object metadata
annotations (not to be confused with descriptor annotations in an OCI image
manifest) are defined for `Configuration.meta.pkg.crossplane.io` and
`Provider.meta.pkg.crossplane.io` and should be honored over any competing
annotations by third-party consumers of Crossplane packages:

- `meta.crossplane.io/maintainer`: The package's maintainers, as a short opaque
  text string.
- `meta.crossplane.io/source`: The URL at which the package's source can be
  found.
- `meta.crossplane.io/license`: The license under which the package's source is
  released. This should be a valid [SPDX License Identifier].
- `meta.crossplane.io/description`: A one sentence description of the package.
- `meta.crossplane.io/readme`: A longer description, documentation, etc.

Third party consumers may define additional arbitrary annotations with any key
and value on any object in a package. All annotations on "meta" types (i.e.
`Configuration.meta.pkg.crossplane.io` and `Provider.meta.pkg.crossplane.io`)
are propagated to the respective package revision (i.e.
`ConfigurationRevision.pkg.crossplane.io` and
`ProviderRevision.pkg.crossplane.io`) on package install. Annotations on all
other objects in a package are propagated to their in-cluster representation
unmodified.

<!-- Named Links -->

[packages]: https://docs.crossplane.io/master/concepts/packages
[OCI images]: https://github.com/opencontainers/image-spec
[OCI image specification]: https://github.com/opencontainers/image-spec/blob/main/spec.md
[YAML]: https://yaml.org/spec/1.2.2/
[YAML stream]: https://yaml.org/spec/1.2.2/#92-streams
[manifests]: https://github.com/opencontainers/image-spec/blob/main/manifest.md
[configuration]: https://github.com/opencontainers/image-spec/blob/main/config.md
[layers]: https://github.com/opencontainers/image-spec/blob/main/layer.md
[index]: https://github.com/opencontainers/image-spec/blob/main/image-index.md
[annotation]: https://github.com/opencontainers/image-spec/blob/main/annotations.md
[applying changesets]: https://github.com/opencontainers/image-spec/blob/main/layer.md#applying-changesets
[SPDX License Identifier]: https://spdx.org/licenses/
