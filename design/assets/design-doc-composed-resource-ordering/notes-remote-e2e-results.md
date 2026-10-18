# Remote e2e results: composed resource ordering

This document covers the results of running the prototype graph engine
against [Modelplane's](https://modelplane.ai) e2e test to validate ordered
creation and deletion of resources in PR [#7841](https://github.com/crossplane/crossplane/pull/7841).

The runbook that describes how to repeat it is
[notes-remote-e2e-plan.md](notes-remote-e2e-plan.md).

Both phases passed. Until the last round, every failure along the way was in
test scaffolding or in a published artifact. The last round, run after the
review fixes, found one defect in Crossplane itself: an XR stopped watching
composed resources that were leaving desired state, so a removal ordered by
carried-forward edges stalled. Each failure is recorded below with its fix.

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

## At the circuit breaker's defaults

Every run above raised `--circuit-breaker-burst` for phase 2. Once the
prototype exempted the events that release a wave from the realtime
compositions circuit breaker (finding 7 in
[notes-scale-findings.md](notes-scale-findings.md)), both phases were run
again with phase 2 passing only `--enable-composed-resource-ordering`, so the
breaker ran as Crossplane ships it.

| | |
|---|---|
| Crossplane | `f5d9ddf5b`, image `crossplane/crossplane:v0.0.0-1790779852-f5d9ddf` |
| Modelplane | `composed-resource-ordering` at `766915a`, all three functions converted, the inference cluster wired to its backend by reference |
| SDK | `function-sdk-python` @ `20115c7` |

Phase 1:

```
--- PASS: TestComposedResourceOrderingTeardownIsOrdered            (34.18s)
--- PASS: TestComposedResourceOrderingTeardownBlocks               (42.18s)
--- PASS: TestComposedResourceOrderingCreateBeforeDestroy          (38.67s)
--- PASS: TestComposedResourceOrderingCreatesInWaves              (109.26s)
--- PASS: TestComposedResourceOrderingTeardownSurvivesRestart      (38.18s)
--- PASS: TestComposedResourceOrderingTeardownIsOrderedOnLegacyXR  (54.22s)

DONE 118 tests, 57 skipped in 357.769s
```

`CreatesInWaves` took 109s against 159s in the first run, but the two are on
different code and one run each, so that isn't a measurement of the
exemption.

Phase 2 passed: the stack came up and the mock engine answered 200 on both
`/v1/chat/completions` and `/v1/messages`. The InferenceGateway reported
`Ready=True` with 16 composed resources, 14 edges and 4 waves, the same shape
as the confirmation run.

## Background deletion and issue #477

[modelplane#477](https://github.com/modelplaneai/modelplane/issues/477)
reports that deleting an InferenceCluster with background propagation, the
`kubectl delete` default, can hang its serving stack and orphan cloud
resources. An XR drops its finalizer as soon as it's deleted, so its
ServingStack disappears at once, the Usage holding the cluster for it
releases, and the cluster can go while the stack's Helm releases are still
uninstalling. ModelReplicas and InferenceGateways race the same way against
the provider-kubernetes Objects they compose.

`e2e/teardown-check.py` in the Modelplane fork tests for it. Run after
`--verify`, it deletes every ModelService and ModelDeployment, then every
InferenceGateway, then every InferenceCluster, each with a plain
`kubectl delete`, and watches every object each one composed, nested XRs
included. It fails if an XR is gone before something it composed, if a
resource starts deleting while something that depends on it still exists, or
if anything is left after ten minutes.

| | Upstream Modelplane `main` (`0f348ac`), Crossplane v2.4.0 | Forks: Crossplane `4a9bd3c36`, Modelplane `c2fee1a` |
| --- | --- | --- |
| ModelDeployment | The ModelReplica was gone at +1s, before 10 of its Objects | The ModelReplica stayed until its Objects were gone |
| InferenceGateway | Gone at +3s, before 15 of its Objects | Stayed until its releases, ProviderConfig and CRDs were gone, at +12s |
| InferenceCluster | It and its ServingStack were gone at +7s, before 53 composed resources that took until +24s to go | Both stayed until the last Helm release, cert-manager, had uninstalled |
| Result | 4 problems | OK |

The InferenceCluster row is the case the issue describes: for 17 seconds
upstream, nothing held the cluster while its Helm releases were uninstalling.
With a provisioned cluster, its API server could go in that window. Here it
can't, because the workload cluster is a kind cluster registered with
`source: Existing` that outlives the InferenceCluster, so upstream's releases
still finished and nothing was left behind. What the run shows is whether each
XR outlived what it composed, and with ordering on, each one did.

Three changes make that hold:

- A deleting XR keeps its finalizer until the composed resources it controls
  are gone, even when it declared no graph. That covers ModelReplicas, which
  don't.
- Crossplane deletes composed resources with background rather than
  foreground propagation, so a nested XR like the ServingStack orders its own
  teardown rather than being cascaded.
- The InferenceCluster's ServingStack depends on the cluster, in place of the
  Usage the issue cites.

Getting there turned up two more problems, both fixed:

- **A kind that's no longer served wedged teardown.** Teardown reads the
  composed resources the XR's references name, and doesn't prune them as they
  go. Once the InferenceGateway's MetalLB release was uninstalled, taking its
  CRDs with it, the references still named an IPAddressPool and an
  L2Advertisement. Reading them through the cache started informers that could
  never sync, and each read blocked until the reconcile timed out: the
  InferenceGateway reported `Timeout: failed waiting for
  *unstructured.Unstructured Informer to sync` and never finished deleting.
  Teardown now reads composed resources directly from the API server, and
  treats a kind that's no longer served like NotFound.
- **MetalLB's namespace was deleted alongside its release.** Nothing ordered
  them, so the namespace that holds the release, and Helm's record of it,
  started deleting in the same wave as the uninstall. The release, its
  IPAddressPool and its L2Advertisement now depend on the namespace.

## After the review fixes

The review of #7841 and #7842 changed how the graph behaves, so both phases
were run again from scratch on a new VM. The changes under test:

- Edges are carried forward for resources leaving desired state. A function
  that drops resources and stops declaring their edges in the same reconcile
  still has them removed in order, and the persisted graph no longer loses
  those edges.
- `pendingResources` follows one naming rule: `resourceName` is a
  composition resource name and `name` is an object name.
- Claims that ask for foreground deletion delete their XR in the background.

| | |
|---|---|
| Host | GCE `instance-20261017-174742`, `e2-highmem-4` (4 vCPU, 31 GB), Debian 13, 200 GB disk |
| Nix | 2.35.3, multi-user, run natively |
| Crossplane | phase 1 at `741a590dd`, then reruns at `20fd96e86` and `876824dff`; phase 2 on image `crossplane/crossplane:v0.0.0-1792261451-20fd96e` |
| Modelplane | `composed-resource-ordering`, ported onto upstream `main` (`0ee6ab4`); the first phase 2 run at `70a2c34c`, the second at `12ff4e32` |
| SDK | `function-sdk-python` @ `78346f4` |

The phase 2 image predates the last two Crossplane commits. `3d7d9f8aa`
changes only an e2e test. `876824dff` adds a warning that fires only with
ordering off. Both phases ran with ordering on, so the results hold for
`876824dff`.

### Phase 1

The suite now has ten ordering tests. `RemovalIsOrdered` is new. It drops
two resources and the edge between them in one reconcile, then asserts they
go in order. That is what carry-forward exists for.

```
--- PASS: TestComposedResourceOrderingTeardownIsOrdered             (33.96s)
--- FAIL: TestComposedResourceOrderingRemovalIsOrdered              (77.00s)
--- PASS: TestComposedResourceOrderingTeardownBlocks                (40.02s)
--- PASS: TestComposedResourceOrderingCreateBeforeDestroy           (26.94s)
--- PASS: TestComposedResourceOrderingCreatesInWaves                (98.25s)
--- PASS: TestComposedResourceOrderingTeardownSurvivesRestart       (40.56s)
--- PASS: TestComposedResourceOrderingTeardownIsOrderedOnLegacyXR   (27.68s)
--- PASS: TestComposedResourceOrderingTeardownIsOrderedInNestedXR   (52.52s)
--- PASS: TestComposedResourceOrderingWaitsForRequiredResource      (44.36s)
--- PASS: TestComposedResourceOrderingReportsContradiction          (30.08s)
```

`RemovalIsOrdered` failed for two reasons, fixed one at a time (findings 5
and 6 below). It passed on the third run:

```
--- PASS: TestComposedResourceOrderingRemovalIsOrdered              (48.55s)
```

### Phase 2, first run: stuck behind a Helm release

`--verify` failed after 25 minutes with 10 passed and 15 errors. The
ServingStack never went Ready because cert-manager never installed (finding
7). The graph held everything that depends on cert-manager, as it should.

The two stuck Helm releases were cleared by hand, and the stack converged
within minutes. `--teardown` then passed, but it showed that several gateway
objects had no edge to the controller that finalizes them (finding 8). They
finished first only because of timing.

### Phase 2, second run: clean

The second run included the harness fix for finding 7 and the edges for
finding 8. Clusters were rebuilt from scratch, with no intervention.

`--verify` passed:

```
======================== 25 passed in 481.52s (0:08:01) ========================
```

The harness waited 99s for provider-helm to finish rolling out onto its
runtime config. That is the window in which the first run's installs were
killed. No release reached `pending-install`.

Readiness, from the XRs' creation at 20:16:39 UTC:

| | |
|---|---|
| InferenceCluster `ClusterReady` | +9s |
| InferenceGateway `Ready` | +4m12s |
| ServingStack `Ready`, and with it the InferenceCluster | +5m15s |
| ModelService `RoutingReady` | +7m16s |

The only Usage left was the `ClusterUsage` that protects the InferenceCluster
while ModelReplicas are scheduled to it. It belongs to a different XR, so a
graph can't replace it.

`--teardown` printed `OK` and left nothing behind. The InferenceCluster
started deleting at +18s from the start of teardown and was gone at +40s.
The gateway PKI rows, against the same rows from the first run (`-` means
the deletion started between two polls):

| Object | First run: deleting / gone | Second run: deleting / gone |
|---|---|---|
| `gateway-serving-certificate` | `-` / +23s | `-` / +22s |
| `gateway-ca-issuer` | +19s / +23s | `-` / +26s |
| `gateway-ca-certificate` | +19s / +23s | `-` / +30s |
| `gateway-selfsigned-issuer` | +27s / +30s | +33s / +35s |
| `cert-manager` | +30s / +38s | +35s / +38s |
| `gateway-ca-bundle` | `-` / +23s | `-` / +22s |
| `trust-manager` | +19s / +27s | +26s / +30s |
| `gateway-proxy`, `gateway-class` | +19s / +23s | `-` / +22s |
| `envoy-gateway` | `-` / +27s | +26s / +30s |
| ProviderConfigs | `-` / +38s | `-` / +38s and +40s |

In the first run, trust-manager started uninstalling at +19s, while the
Bundle it reconciles was still there. In the second, it waited until the
Bundle was gone. cert-manager now waits for the last of its Certificates and
Issuers. envoy-gateway now waits for its EnvoyProxy and GatewayClass. Each
controller outlives the objects it has to finalize, and teardown took no
longer: the InferenceCluster took 22s, against 19s in the first run.

## Findings, and what fixed them

Eight. Four came from the earlier runs: three in test scaffolding or a
published artifact, and one in this document's own instructions. Four came
from the run after the review fixes. Of those four, one was in Crossplane,
one in a test, one in Modelplane's e2e harness and one in Modelplane's graph.
All are fixed, and the runs above cover them.

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

**5. A live removal stalled after its first wave.** This one was in
Crossplane. Compose registered only desired composed resources with the
tracker that maps realtime-composition watch events back to the XR. A
resource that had left desired state was no longer watched. That covers a
resource already deleting, and one held back from deletion behind a
dependent. So when its deletion finished, nothing enqueued the XR. With
realtime compositions nothing polls, so `RemovalIsOrdered` deleted its first
wave and then reported the next as waiting on a resource that was already
gone. XR teardown wasn't affected, because it requeues itself. Live removal
had no e2e coverage until this test, which is why this hadn't come up
before. Fixed in `ba2452838`: with ordering on, the tracker also records
observed resources that aren't desired, until they're gone.

**6. `RemovalIsOrdered` raced its own wave.** Its `FirstStays` step started
as soon as `second` began deleting, saw `second` still there partway
through deletion, and failed, though the waves were right. Fixed in
`3d7d9f8aa`, which waits for only `first` to remain before asserting it's
left alone.

**7. provider-helm was killed in the middle of a Helm install.** Bring-up
points provider-helm and provider-kubernetes at the
DeploymentRuntimeConfigs Modelplane defines. That creates a new
ProviderRevision for each, whose pod replaces the old one. Nothing waited
for the rollout. On the 4 vCPU box, the old provider-helm pod was killed in
the middle of `helm install --wait` for cert-manager and
kube-prometheus-stack. Helm records a release as `pending-install` until
the install finishes, so both stayed that way. Every later attempt failed
with `another operation (install/upgrade/rollback) is in progress`, and the
ServingStack waited behind cert-manager until the test timed out. Fixed in
the Modelplane fork's harness (`12ff4e32`): bring-up waits for each
provider's current revision to use its runtime config and report
`RuntimeHealthy`, and for the old revision's pods to go, before applying
anything.

That fixes the harness, not the exposure. A provider-helm restart during
an install, from an upgrade, a node drain or an out-of-memory kill, leaves
the release stuck the same way. provider-helm recovers a stuck release
only when the Release sets `spec.rollbackLimit`, and Modelplane's
Releases don't set it.

**8. Gateway objects had no edge to their controllers.** The serving stack's
hand-written gateway objects depended only on their ProviderConfig:

- the cert-manager Certificates and Issuers;
- the trust-manager Bundle;
- the EnvoyProxy.

So they were all created in the first wave and retried with
`ReconcileError` until their CRDs arrived. Upstream's create gate was the
same, so that is how it always converged. At teardown, though, nothing kept
cert-manager, trust-manager or envoy-gateway from uninstalling while their
objects still existed. Objects left without their controller can't
finalize. The first run's teardown passed only because of timing. Fixed in
the Modelplane fork (`eec480dd`), which declares each edge only while both
ends are composed:

- the PKI in chain order: self-signed issuer, CA certificate, CA issuer,
  serving certificate;
- the CA bundle on trust-manager and the CA certificate;
- the EnvoyProxy on envoy-gateway;
- the Gateway on its serving certificate.

The second run's teardown table above shows the result.

## What this does not establish

- **Scale.** The largest graph exercised here is 18 composed resources.
  Measured separately since - see
  [notes-scale-findings.md](notes-scale-findings.md) - graphs of up to 1000
  composed resources and 9,600 edges converge fine, but two things found
  there apply to any run of this kind. Teardown used to wedge, because it
  kept writing to resources it had already asked to delete; that is fixed.
  And ordering does not converge at all against the circuit breaker's shipped
  defaults once a graph is deep enough: a 50-link chain that takes 10s with
  the burst raised had not finished after 601s at `burst=100`. That is now
  addressed by exempting the events that release a wave, and the last run
  here had the breaker at its defaults; the runs before it had it raised.
- **Out-of-band deletion.** A `Usage` refuses a direct `kubectl delete` of a
  ProviderConfig; dependencies only order the deletes Crossplane performs.
  That trade was accepted deliberately and was not tested.
- **The capability guard.** `compose-serving-stack` and
  `compose-inference-cluster` fail the pipeline, through the SDK's
  `require_capabilities`, when Crossplane does
  not advertise `CAPABILITY_DEPENDENCIES`. The flag was set for every run
  here, so that path ran only in unit tests. So did the warning Crossplane
  raises when a step returns dependencies with ordering off.
- **Live removal in Modelplane.** `RemovalIsOrdered` covers carry-forward
  against purpose-built fixtures. Modelplane's runs only create and tear
  down, so no Modelplane function dropped resources from a live XR.
- **Claims.** Background deletion for claims that ask for foreground
  deletion has unit tests only; nothing here creates a claim.
- **Real workloads.** Mock engine, fake DRA devices, no GPUs, no cloud
  provisioning, single-node clusters. So #477's hang itself, which needs the
  InferenceCluster's deletion to take a provisioned cluster's API server with
  it, wasn't reproduced; only the ordering that prevents it was.
- **Modelplane beyond these three compositions.** All three composing
  functions have since been converted and `compose-usages` deleted outright,
  but only the serving stack and the gateway were exercised end to end at
  first.
  The teardown check has since exercised the InferenceCluster with
  `source: Existing`; its cloud cluster paths, including their required
  resource edges, are covered by unit tests only.

## Reproducing

See [notes-remote-e2e-plan.md](notes-remote-e2e-plan.md) for the VM setup,
both phases, and the verification steps, all corrected against this run.
Branches, at the last run: crossplane `876824dff`, function-sdk-python
`78346f4`, modelplane `12ff4e32`, all on `github.com/stevendborrelli` forks.

[crossplane-graph](https://github.com/stevendborrelli/crossplane-graph)
prints the graph off a live composite, which is the quickest way to see
whether the edges reached the XR and where a reconcile has got to:

```bash
crossplane-graph inferencegateway/local
crossplane-graph servingstack/<name> -n modelplane-system --dot | dot -Tpng -o graph.png
```
