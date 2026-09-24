# Remote e2e results: composed resource ordering

This document covers the results of running the prototype graph engine
against [Modelplane's](https://modelplane.ai) e2e test to validate ordered
creation and deletion of resources in PR [#7841](https://github.com/crossplane/crossplane/pull/7841).

The runbook that describes how to repeat it is
[notes-remote-e2e-plan.md](notes-remote-e2e-plan.md).

Both phases passed. The graph behaved correctly throughout: every failure
along the way was in test scaffolding or in a published artifact, and each is
recorded below with its fix.

## What was under test

Modelplane currently uses function code and Usages to control ordering. This
test was run on a fork that replaces both the logic and the compose-usages
function with functions that declare edges through the Python SDK's
`add_dependency`, which Crossplane records as `dependsOn` on the composite's
composed resource references.

Crossplane reads the accumulated edges, creates a graph, and then sequences
creation and deletion from the graph. This e2e test is validating that the
graph works on a larger project. Two questions mattered:

1. Does it work in isolation, against purpose-built fixtures?
2. Does it hold up as a replacement for a mechanism a real platform already
   depends on? Modelplane's serving stack was rewritten from `Usage`s to
   declared dependencies, so its e2e is the first real consumer.

## Environment

| | |
|---|---|
| Host | GCE `borrelli-modelplane`, `e2-standard-8` (8 vCPU, 31 GB), Debian 13 |
| Disk | 99 GB (grown from 10 GB; the original disk could not hold the run) |
| Nix | 2.35.2, multi-user, flakes enabled, run natively rather than via `./nix.sh` |
| Crossplane | built from the branch; the final runs used `f322547` |
| Args | `["core","start","--enable-dependency-version-upgrades","--enable-composed-resource-ordering"]` |
| SDK | `function-sdk-python` @ `8e5a0b2`, resolved into the function images from `uv.lock` |
| Modelplane | `composed-resource-ordering`, final runs at `38b3543` |

Ordering is alpha and off by default. Phase 1's suite sets the flag itself;
phase 2 needed it passed explicitly, because `crossplane project run`
installs a released Crossplane from a chart and takes no flag for an image.

## Phase 1: Crossplane's ordering suite

`nix run .#e2e -- --test-suite=composed-resource-ordering`

```
--- PASS: TestComposedResourceOrderingTeardownIsOrdered      (47.80s)
--- PASS: TestComposedResourceOrderingTeardownBlocks         (29.81s)
--- PASS: TestComposedResourceOrderingCreateBeforeDestroy    (23.22s)
--- PASS: TestComposedResourceOrderingCreatesInWaves        (159.49s)
--- PASS: TestComposedResourceOrderingTeardownSurvivesRestart (36.22s)

DONE 107 tests, 57 skipped in 350.449s
```

The suite selector runs the whole apiextensions file, so the circuit breaker,
required resources and required schemas tests ran alongside and also passed.

`TeardownSurvivesRestart` is the one worth calling out: it kills Crossplane
mid-teardown and the replacement process resumes in dependency order, ordering
by a graph it never computed and can only have read back from the XR's own
composed resource references.

## Phase 2: Modelplane's serving stack

`CROSSPLANE_IMAGE=... CROSSPLANE_ARGS=--enable-composed-resource-ordering nix
run .#e2e -- --verify`, two kind clusters, no cloud and no GPU.

```
==> verify attempt 1 (OpenAI): HTTP 200
==> verify (Anthropic /v1/messages): HTTP 200
==> End to end OK: http://172.18.255.200/ml-team/mock
```

About 15 minutes on the warm 8 vCPU box.

A green `--verify` proves the stack converged, not that it converged *in
order* - the previous implementation converged by racing and retrying. So the
ordering was checked directly.

### The graph reached the XR

18 composed resource references, 16 carrying `dependsOn`. The two without are
the ProviderConfigs, which depend on nothing. Edges read as intended:

```
kube-prometheus-stack  <- provider-config-helm
cert-manager           <- provider-config-helm
ai-gateway             <- ai-gateway-crds, provider-config-helm
envoy-gateway          <- cert-manager, provider-config-helm
```

### Creation ran in waves

Composed resource creation timestamps, relative to the first:

```
+22s   ProviderConfig  provider-config-helm, provider-config-kubernetes
+24s   Release         ai-gateway-crds, kube-prometheus-stack,
                       leader-worker-set, nvidia-dra-driver-gpu
+25s   Release         cert-manager, node-feature-discovery
+63s   Release         ai-gateway       (after ai-gateway-crds went Ready)
+131s  Release         envoy-gateway    (after cert-manager went Ready)
+208s  Object          gateway-class    (after envoy-gateway went Ready)
+212s  Object          gateway          (after gateway-class)
```

Nothing was created before what it depends on. The gaps are the dependency
waiting for its prerequisite to report Ready, not scheduling noise.

### Teardown ran in reverse

Deleting from the top (`InferenceCluster/local`), relative to the first
deletion:

```
+0s   leaves asked to delete: ai-gateway, kube-prometheus-stack,
      node-feature-discovery, leader-worker-set, nvidia-dra-driver-gpu,
      gateway, gateway-proxy, the GAIE CRD objects
      not deleting: cert-manager, envoy-gateway, gateway-class,
      ai-gateway-crds, both ProviderConfigs - each still has a live dependent
+16s  gateway-proxy gone  -> gateway-namespace starts deleting
+64s  gateway gone        -> gateway-class starts deleting
+96s  gateway-class gone  -> envoy-gateway starts deleting
      cert-manager still alive, holding for envoy-gateway
```

Teardown then ran to completion. Every one of the ServingStack's 18 composed
resources was deleted, its two ProviderConfigs last. The only objects left in
the cluster belonged to `InferenceGateway/default`, which was never deleted.

That is the guarantee the `Usage`s used to provide, now coming from the graph.

### Usages

Three `Usage`s remained, all owned by `InferenceGateway/default`, plus a
`ClusterUsage` holding the InferenceCluster while ModelReplicas are scheduled
to it - it refused the first delete, correctly. None is owned by the
ServingStack. `Usage`s are gone exactly where they were replaced and intact
everywhere else.

## Confirmation runs

The results above come from the first clean pass of each phase. Both were
then re-run on the fixed code, and phase 1 a third time, so nothing here
rests on a single run:

- Phase 1, three times green, the last two after the teardown fixes and the
  `setup/` cleanup, with the tolerant delete removed again.
- Phase 2, twice green. The second run also carried the
  `compose-inference-gateway` conversion, which moved that composition off
  `Usage`s the same way. Its graph reads as 16 composed resources across 4
  waves - the ProviderConfig and the Gateway API CRDs, then Traefik and
  MetalLB, then the GatewayClass and MetalLB's pool, then the Gateway - with
  no `Usage`s of its own, and the stack still served live traffic.

## Findings, and what fixed them

Four, none in the ordering graph itself: three in test scaffolding or a
published artifact, one in this document's own instructions. All are fixed,
and the confirmation runs above cover them.

**1. `provider-nop:timed-deletes.2` was published arm64-only.** Built on a
laptop, so on an amd64 cluster the provider pod crashlooped with an exec
format error. That surfaces as a Provider that never goes Healthy, several
minutes into a run, with nothing pointing at the architecture. Republished as
a manifest list (`timed-deletes.3`) and the reference bumped in `39310759b`.
The sibling `function-ordering` repo was unaffected because it has a
`build.sh` that builds both architectures; provider-nop had none, and now
does.

Moot since: `deleteAfter` and `deleteError` shipped in provider-nop v0.6.0,
so everything here now pins
`xpkg.crossplane.io/crossplane-contrib/provider-nop:v0.6.0` and there is no
fork image to publish. The lesson stands for any hand-built package - a
single-architecture image fails as a Provider that never goes Healthy, with
nothing naming the architecture.

**2. The blocked-teardown test lost a race with its own provider.** It cleared
`deleteError` with an unretried read-modify-write against a NopResource the
provider writes to continuously while refusing to delete, and got `the object
has been modified`. Fixed with `retry.RetryOnConflict` in `c3e0f4f`, matching
what `pkg_test.go` already does.

The fallout was worse than the failure. The test's teardown then deleted the
Provider while its NopResources were still live, which removed the controller
that would finalize them - so the Provider's own foreground deletion could
never complete, its MRDs went away, and the RBAC manager withdrew Crossplane's
grants on `nopresources`. Every later test in the suite ran against a wedged
cluster and failed with `forbidden` errors that pointed nowhere near the
cause.

**Worth knowing independently of this feature:** deleting a Provider before
its managed resources are gone deadlocks, and nothing in the failure message
says so. On a disposable cluster, rebuild. On one you care about, install a
new revision and move the CRDs' owner references onto it, so a controller
exists to finalize the stuck resources.

**3. The ordering teardown deleted one object seven times.** Seven manifests
under `setup/` declare the same Composition - they are alternatives each test
picks between - so deleting `setup/*.yaml` asks Kubernetes to delete that one
object seven times, and the decoder's delete handler treats the second
`NotFound` as fatal. It normally passes only because foreground deletion
leaves the Composition behind a finalizer long enough for the later manifests
to still find it; this run lost that race. Fixed in `e284c1b` with a
`DeleteResourcesIgnoreNotFound` helper.

This one is still worth fixing at the source: those seven variants are never
referenced by name, only swept up by the setup glob, and the per-test
compositions already live in `cbd/`, `create/`, `delete/` and `teardown/`.
Note that giving them distinct names would also require each test to pin
`spec.crossplane.compositionRef`, since `xr.yaml` does not and selection
depends on exactly one Composition existing for the type.

**4. The runbook's own verification steps were wrong** - the ServingStack is
in `modelplane-system` rather than `ml-team`, "no Usages at all" was the wrong
assertion, and teardown cannot start by deleting the ServingStack because
`InferenceCluster/local` composes it. Corrected in `fb9db6ae4`.

## What this does not establish

- **Scale.** The largest graph exercised here is 18 composed resources.
  Measured separately since - see
  [notes-scale-findings.md](notes-scale-findings.md) - graphs of up to 1000
  composed resources and 9,600 edges converge fine, but two things found
  there apply to any run of this kind. Teardown used to wedge, because it
  kept writing to resources it had already asked to delete; that is fixed.
  And ordering does not converge at all against the circuit breaker's shipped
  defaults once a graph is deep enough: a 50-link chain that takes 10s with
  the burst raised had not finished after 601s at `burst=100`. Every run in
  this document had the breaker raised.
- **Out-of-band deletion.** A `Usage` refuses a direct `kubectl delete` of a
  ProviderConfig; dependencies only order the deletes Crossplane performs.
  That trade was accepted deliberately and was not tested.
- **The capability guard.** `compose-serving-stack` fails the pipeline when
  Crossplane does not advertise `CAPABILITY_DEPENDENCIES`. The flag was set
  for every run here, so that path ran only in unit tests.
- **Real workloads.** Mock engine, fake DRA devices, no GPUs, no cloud
  provisioning, single-node clusters.
- **Modelplane beyond these three compositions.** All three composing
  functions have since been converted and `compose-usages` deleted outright -
  see [notes-modelplane-conversion.md](notes-modelplane-conversion.md) - but
  only the serving stack and the gateway were exercised end to end here. The
  inference cluster's conversion, including its required-resource edges, is
  covered by unit tests only.

## Reproducing

See [notes-remote-e2e-plan.md](notes-remote-e2e-plan.md) for the VM setup,
both phases, and the verification steps, all corrected against this run.
Branches, at the final run: crossplane `f322547`, function-sdk-python
`8e5a0b2`, modelplane `38b3543`, all on `github.com/stevendborrelli` forks.

`xpgraph` prints the graph off a live composite, which is the quickest way to
see whether the edges reached the XR and where a reconcile has got to:

```bash
go build -o /tmp/xpgraph ./cmd/xpgraph
/tmp/xpgraph inferencegateway/default
/tmp/xpgraph servingstack/<name> -n modelplane-system --dot | dot -Tpng -o graph.png
```
