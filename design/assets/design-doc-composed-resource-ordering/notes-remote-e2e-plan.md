# Remote e2e plan: composed resource ordering

A runbook for validating composed resource ordering on a Linux VM, where
there's enough memory to run the full Modelplane stack. Written so a future
session can execute it without rediscovering how the pieces fit.

Two phases, cheapest first. Phase 1 tests the feature in isolation with
Crossplane's own e2e suite. Phase 2 tests it against a real platform —
Modelplane's serving stack, whose ordering was rewritten from `Usage`s to
declared dependencies.

Stop after phase 1 if it fails. Phase 2 can't pass if phase 1 doesn't.

## What's being validated

The feature lets a composition function return ordering constraints, and
Crossplane sequences creation and deletion from them. Phase 2 is the
interesting test: it's the first real platform to depend on it, and it
replaced a working mechanism (`Usage`s) rather than adding a new one.

Specifically, whether:

- a composed resource is created only once what it depends on is Ready;
- a dependency is deleted only once its dependents are gone;
- a stack that used to converge by retrying now converges by ordering, with
  no `Usage` left in the picture.

## State of the code

| Repo | Branch | Where |
|---|---|---|
| crossplane | `composed-resource-ordering-prototype` | pushed, `sb` remote (`github.com/stevendborrelli/crossplane`), at `f34945a` |
| function-sdk-python | `composed-resource-dependencies` | pushed, `origin` = fork (`github.com/stevendborrelli/function-sdk-python`), at `8e5a0b2` |
| modelplane | `composed-resource-ordering` | pushed, `sb` remote (`github.com/stevendborrelli/modelplane`), three commits on `main` |

Everything needed is pushed; the VM clones all three.

Note the remotes differ per repo: in the crossplane and modelplane checkouts
`origin` is upstream and `sb` is the fork; in function-sdk-python `origin`
*is* the fork. Check `git remote -v` rather than assuming.

## VM requirements

- x86_64 Linux, Docker installed, user in the `docker` group.
- **≥ 16 GB RAM** and ~100 GB disk. Phase 2 runs two kind clusters, 13
  provider packages, and the serving stack (cert-manager, Envoy Gateway,
  kube-prometheus-stack, NFD, the DRA driver). A full disk shows up as `no
  space left on device`; memory pressure shows up as pods stuck Pending.
  Phase 1 alone is much lighter — one kind cluster, ~8 GB is fine.
- Outbound network: Docker Hub, `charts.crossplane.io`, `xpkg.upbound.io`,
  ghcr, GitHub.
- Nix is not required on the host. Both repos carry `./nix.sh`, which runs
  Nix inside Docker. Installing Nix natively is faster if the VM will be
  reused, but `./nix.sh` is the tested path.

Watch for Docker Hub anonymous pull limits — several images come from there,
including the phase 1 function package. `docker login` first if the VM has no
pull-through cache.

## Phase 1: Crossplane's own ordering suite

This builds Crossplane from the branch, stands up a kind cluster, and runs
the e2e tests written for the feature.

```bash
git clone -b composed-resource-ordering-prototype \
  https://github.com/stevendborrelli/crossplane.git
cd crossplane
git rev-parse --short HEAD    # expect f34945a

./nix.sh run .#e2e -- --test-suite=composed-resource-ordering
```

The nix app already passes `-test.v`, so pass only the suite selector. The
suite is registered in `test/e2e/apiextensions_ordering_test.go`, and
installs Crossplane with the feature flag already set
(`--set args={--debug,--enable-composed-resource-ordering}`). Phase 2 is where
you have to pass it yourself.

It covers creation ordering, teardown ordering, a dependency on a required
(rather than composed) resource, nested XRs, and a blocked teardown.

**Dependency to check first.** The suite installs a Function package from
`index.docker.io/steve/function-ordering:dependency-test.4`
(`test/e2e/manifests/apiextensions/composition/ordering/setup/functions.yaml`).
If that tag is gone or private, rebuild and push it from the local
`function-ordering` checkout, then update the manifest to the new reference.
That repo is the demo function written for this feature; no published
function declares dependencies, which is the compatibility problem the
feature is about.

**Note:** the project `CLAUDE.md` says `./nix.sh run .#streamImage`. The real
attribute is `stream-image`; the documented spelling errors out.

## Optional: publish the build to your own registry

Not needed for either phase — both build locally and `kind load`. Worth doing
to install this Crossplane on a cluster that isn't kind, or to hand the build
to someone else.

```bash
# Images: multi-arch, tagged v<version>, manifest included.
./nix.sh run .#push-images -- ghcr.io/stevendborrelli/crossplane

# Chart: pushed as <namespace>/crossplane:<version>, no leading v.
./nix.sh run .#push-chart -- oci://ghcr.io/stevendborrelli/charts
```

Then anywhere with cluster access:

```bash
helm install crossplane \
  oci://ghcr.io/stevendborrelli/charts/crossplane --version <version> \
  -n crossplane-system --create-namespace \
  --set image.repository=ghcr.io/stevendborrelli/crossplane \
  --set args={--debug,--enable-composed-resource-ordering}
```

Only `image.repository` needs overriding: the chart's default tag is `v` +
`appVersion`, which is exactly how `push-images` tags what it pushes, so chart
and image stay in lockstep with the commit.

Three things to know before trying:

- The GitHub token needs the **`write:packages`** scope. The usual `repo,
  workflow, read:org, gist` set is not enough for ghcr, and both pushes fail
  without it: `gh auth refresh -s write:packages`, then `docker login ghcr.io`
  and `helm registry login ghcr.io`.
- ghcr packages are **private** on first push, so a cluster pulling them needs
  a pull secret until they're made public.
- Both apps use the ambient Docker daemon and Helm credentials. Under
  `./nix.sh` those are the ones *inside* the Nix container, which starts its
  own dockerd — so log in inside that container, or run these two with a
  native Nix install on the VM.

This does **not** remove the `CROSSPLANE_IMAGE` step from phase 2.
`crossplane project run` resolves its chart from `charts.crossplane.io` by
version and takes no flag for a chart repo, so a chart of your own can't be
installed through the CLI.

## Phase 2: Modelplane's e2e against local builds

Modelplane's serving stack used to sequence teardown with `Usage`s and gate
installs by re-composing only what it had already observed. Both were replaced
by dependency edges. This is the test that the replacement actually works.

### Build and load the Crossplane image

```bash
cd crossplane
./nix.sh run .#stream-image | docker load     # prints the tag it loaded
```

The tag looks like `crossplane/crossplane:v0.0.0-<epoch>-<sha>`, with `<sha>`
matching the branch HEAD. Confirm it carries the feature before going further
— a stale image is the most confusing possible failure:

```bash
cid=$(docker create crossplane/crossplane:<tag>)
docker export "$cid" | strings | grep -c CAPABILITY_DEPENDENCIES   # expect ≥1
docker rm -f "$cid"
```

### Run it

```bash
git clone -b composed-resource-ordering \
  https://github.com/stevendborrelli/modelplane.git
cd modelplane

CROSSPLANE_IMAGE=crossplane/crossplane:<tag> \
  CROSSPLANE_ARGS=--enable-composed-resource-ordering \
  ./nix.sh run .#e2e -- --verify
```

Both variables are required, for different reasons:

- `CROSSPLANE_IMAGE` — `crossplane project run` installs Crossplane from
  `charts.crossplane.io` by version, so a branch build can't come in through
  the CLI. `run.sh` loads the image into the control-plane cluster and
  repoints both Crossplane Deployments after the chart lands, before any
  Modelplane manifest is applied.
- `CROSSPLANE_ARGS` — ordering is an **alpha feature, off by default**.
  Without `--enable-composed-resource-ordering` Crossplane doesn't advertise
  `CAPABILITY_DEPENDENCIES`, and `compose-serving-stack` deliberately fails
  the pipeline rather than composing a stack whose ordering nothing will
  enforce.

The functions need no flag: they resolve the branch SDK from `uv.lock`, which
points at the `function-sdk-python` rev above.

`--verify` waits for readiness and asserts a live 200 from the mock engine.
Budget 30-45 minutes for a first run on a cold VM.

### What to check beyond a green run

`--verify` proves the stack converged. It does not prove it converged *in
order* — a stack that races and retries can still end up healthy. Check the
ordering directly, on the control-plane cluster
(`kubectl --context kind-modelplane-e2e-local`):

1. **No `Usage`s.** `kubectl get usages.protection.crossplane.io -A` should be
   empty. Anything here means an old build.
2. **Edges reached the XR.** The composed resource references should carry
   `dependsOn`:
   `kubectl get servingstack -n ml-team -o yaml | grep -A3 resourceRefs`.
   If nothing carries it, Crossplane ignored the dependencies — check the flag
   landed: `kubectl -n crossplane-system get deploy crossplane -o
   jsonpath='{.spec.template.spec.containers[0].args}'`.
3. **Creation ran in waves.** Sort composed resources by creation timestamp
   and confirm the ProviderConfigs precede everything, cert-manager precedes
   the Envoy Gateway release, and the GatewayClass follows it. Timestamps have
   one-second granularity, so treat ties as inconclusive rather than as
   failures.
4. **Teardown runs in reverse.** This is the part `Usage`s used to do, so it's
   the real regression risk. Delete the ServingStack and watch which resources
   get a deletion timestamp first:

   ```bash
   kubectl -n ml-team delete servingstack <name> --wait=false
   kubectl get managed -A -w   # or watch the XR's resourceRefs
   ```

   Expect the leaves to go first and the ProviderConfigs last. A
   `ProviderConfig` deleted while a Release still exists is the specific
   failure the `Usage`s used to prevent.

## If it fails

- **Pipeline fatal, "does not support composed resource dependencies"** —
  `CROSSPLANE_ARGS` didn't take, or the image is a release build. Check the
  deployment args and image, per step 2 above.
- **Nothing is created at all** — everything waits on the ProviderConfigs,
  which are marked Ready only once *observed*. If they never appear as
  observed, the whole graph stays blocked. Check they exist and reconcile.
- **A resource never gets created** — a cycle, or a dependency that never
  reports Ready. Crossplane logs name the resources it's holding back;
  `--debug` is already on.
- **Teardown hangs** — expected when something can't delete; that's the
  blocked-teardown case phase 1 covers. Confirm the blocker is a real stuck
  resource rather than a cycle.
- **Serving stack never goes Ready** — most likely resource exhaustion, not
  ordering. Check for Pending pods and disk before suspecting the feature.

Collect on failure: `kubectl -n crossplane-system logs deploy/crossplane`,
the XR's status and `resourceRefs`, and the function pod's logs from
`crossplane-system`.

## Known gaps

- Ordering only constrains deletes **Crossplane itself** performs. A
  `kubectl delete` of a ProviderConfig out from under a live Release is no
  longer refused — a `Usage` used to reject it. Don't read that as a
  regression in the run; it's a known trade of the change.
- The image swap replaces the image and args only. This branch changes no
  CRDs, so nothing else is needed today; a future build that changes
  `cluster/crds` would need them applied alongside.
- The git SDK source makes Nix build the SDK from source rather than take a
  wheel, which needs its `hatchling` backend named in `nix/checks.nix`. That
  override and the git source come out together when the SDK releases.
