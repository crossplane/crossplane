# e2e-function-revision-id

A minimal Crossplane composition function used only by Crossplane's e2e
tests. It writes a build-time constant, `Identity`, into the composite's
`status.servedBy` field.

Because the identity is baked into the image (not taken from the composition
step's input), the value a composite observes reflects the function revision
that actually served the request — independent of which composition revision
requested it. The `TestCompositionFunctionByOCIRef` e2e test relies on this to
prove that a pinned composite keeps being served by its original function
revision after the composition (and function) are updated.

Two versions are published so that a composition can reference the two versions
by OCI ref and get two distinct `FunctionRevision`s under one parent `Function`:

| Tag | `status.servedBy` |
|-----|-------------------|
| `:v1` | `revision-one` |
| `:v2` | `revision-two` |

## Build & push

Build the runtime images with `ko`:

```bash
KO_CONFIG_PATH=./ko.v1.yaml ko build --push=false --tarball v1-amd64.tgz --platform linux/amd64
KO_CONFIG_PATH=./ko.v1.yaml ko build --push=false --tarball v1-arm64.tgz --platform linux/arm64
KO_CONFIG_PATH=./ko.v2.yaml ko build --push=false --tarball v2-amd64.tgz --platform linux/amd64
KO_CONFIG_PATH=./ko.v2.yaml ko build --push=false --tarball v2-arm64.tgz --platform linux/arm64
```

Build the xpkgs:

```bash
crossplane xpkg build --package-root . --ignore ko.v1.yaml,ko.v2.yaml \
  --embed-runtime-image-tarball v1-amd64.tgz --embed-runtime-image-tarball v1-arm64.tgz \
  --package-file e2e-function-revision-id-v1.xpkg
crossplane xpkg build --package-root . --ignore ko.v1.yaml,ko.v2.yaml \
  --embed-runtime-image-tarball v2-amd64.tgz --embed-runtime-image-tarball v2-arm64.tgz \
  --package-file e2e-function-revision-id-v2.xpkg
```

Push the xpkgs:

```bash
crossplane xpkg push -f e2e-function-revision-id-v1.xpkg \
  ghcr.io/crossplane/e2e-function-revision-id:v1
crossplane xpkg push -f e2e-function-revision-id-v2.xpkg \
  ghcr.io/crossplane/e2e-function-revision-id:v2
```

Find the pushed digests for use in tests:

```bash
crane digest xpkg.crossplane.io/crossplane/e2e-function-revision-id:v1
crane digest xpkg.crossplane.io/crossplane/e2e-function-revision-id:v2
```
