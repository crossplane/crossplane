# e2e-function-revision-id

A minimal Crossplane composition function used only by Crossplane's e2e tests. It
writes a build-time constant, `Identity`, into the composite's
`status.servedBy` field.

Because the identity is baked into the image (not taken from the composition
step's input), the value a composite observes reflects the **function revision**
that actually served the request — independent of which **composition revision**
requested it. The `TestCompositionFunctionByOCIRef` e2e test relies on this to
prove that a Manual-pinned composite keeps being served by its original function
revision after the composition (and function) are updated.

Two versions of the *same* package repository are published so that a
composition can reference the two versions by OCI ref and get two distinct
`FunctionRevision`s under one parent `Function`:

| Tag | `status.servedBy` |
|-----|-------------------|
| `:v1` | `revision-one` |
| `:v2` | `revision-two` |

Pipeline steps must reference function packages **by digest**, so the tags above
exist only for our convenience when pushing. The digest of each tag is what the
test manifests under
`test/e2e/manifests/apiextensions/composition/function-oci-ref` actually
reference, so they must be updated whenever a tag is re-pushed.

## Build & push

This is a self-contained nested Go module. From this directory:

```sh
# v1 -> revision-one
docker build --build-arg IDENTITY=revision-one -t e2e-function-revision-id-runtime:v1 .
crossplane xpkg build --package-root . \
  --embed-runtime-image e2e-function-revision-id-runtime:v1 \
  --package-file e2e-function-revision-id-v1.xpkg
crossplane xpkg push -f e2e-function-revision-id-v1.xpkg \
  xpkg.crossplane.io/crossplane/e2e-function-revision-id:v1

# v2 -> revision-two
docker build --build-arg IDENTITY=revision-two -t e2e-function-revision-id-runtime:v2 .
crossplane xpkg build --package-root . \
  --embed-runtime-image e2e-function-revision-id-runtime:v2 \
  --package-file e2e-function-revision-id-v2.xpkg
crossplane xpkg push -f e2e-function-revision-id-v2.xpkg \
  xpkg.crossplane.io/crossplane/e2e-function-revision-id:v2
```

`go mod tidy` is run inside the Docker build; run it locally too if you want an
up-to-date `go.sum` checked in.

Finally, record the pushed digests in the test manifests:

```sh
crane digest xpkg.crossplane.io/crossplane/e2e-function-revision-id:v1
crane digest xpkg.crossplane.io/crossplane/e2e-function-revision-id:v2
```
