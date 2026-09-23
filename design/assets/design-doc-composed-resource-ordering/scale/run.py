#!/usr/bin/env python3
"""Measure how long an ordered composition takes to converge and tear down.

Generates a Composition for function-ordering in one of three shapes, applies a
fresh XR, and times both directions against the cluster:

  chain    n resources, each waiting on the one before. Depth n, n-1 edges.
           Maximally serial: convergence is bounded by depth, not by capacity.
  fanout   one root and n-1 dependents. Depth 2, n-1 edges. Isolates resource
           count from depth.
  layered  n resources in `--levels` levels, every resource waiting on every
           resource in the level before. The shape a wide composition of
           independent branches actually takes, and the one that generates the
           most edges.

Creation waves are read from composed resource creationTimestamps; teardown
from when each resource first carries a deletionTimestamp. Both come from the
API server rather than from logs, so a run can be repeated and compared.

A fresh XR name per run, always: the realtime compositions circuit breaker
keeps state per target for 24 hours, so reusing a name inherits the last run's
breaker state and measures the wrong thing.

Usage:
  ./run.py --shape fanout --n 50
  ./run.py --shape chain --n 10 --ready-after 2s
  ./run.py --shape layered --n 100 --levels 5 --no-teardown
  ./run.py --shape fanout --n 50 --unordered    # same resources, no edges
"""

import argparse
import datetime
import json
import subprocess
import sys
import time
import urllib.request

NAMESPACE = "default"
GROUP = "scale.crossplane.io/v1alpha1"
KIND = "XScale"

# What function-ordering composes, and how readiness is established for it.
#
# A NopResource goes through a provider, so a run measures Crossplane and the
# provider together - and the provider is the larger and more variable half.
# A ConfigMap is composed directly by core with no provider in the loop at
# all, so a ConfigMap run is the closer thing to measuring the engine.
# Readiness for one is existence, which also removes the readyAfter floor.
RESOURCES = {
    "nopresource": "nopresources.nop.crossplane.io",
    "configmap": "configmaps",
}


def kubectl(*args, check=True, stdin=None):
    """Run kubectl and return stdout."""
    p = subprocess.run(
        ["kubectl", *args],
        capture_output=True,
        text=True,
        input=stdin,
        check=False,
    )
    if check and p.returncode != 0:
        raise RuntimeError(f"kubectl {' '.join(args)}: {p.stderr.strip()}")
    return p.stdout


def graph(shape, n, levels):
    """The resource names and edges for one shape."""
    names = [f"r{i:04d}" for i in range(n)]
    edges = []

    if shape == "chain":
        edges = [(names[i], names[i - 1]) for i in range(1, n)]
    elif shape == "fanout":
        edges = [(names[i], names[0]) for i in range(1, n)]
    elif shape == "layered":
        # Split as evenly as possible, then join every level to the one before.
        size = max(1, n // levels)
        tiers = [names[i : i + size] for i in range(0, n, size)]
        for lower, upper in zip(tiers, tiers[1:]):
            edges += [(u, low) for u in upper for low in lower]

    return names, edges


def composition(name, names, edges, ready_after, delete_after, engine="ordering"):
    """A Composition whose function-ordering input is this run's graph.

    With engine="sequencer" the same graph is expressed to
    function-sequencer instead, for comparison. The pipeline then has two
    steps: function-ordering composes the resources and declares no edges,
    and function-sequencer withholds each resource from desired state until
    the ones before it are ready. That is the mechanism this proposal
    replaces, so it is the baseline worth measuring against.

    function-sequencer takes ordered lists rather than edges, so a chain
    converts exactly and anything wider does not - see waves_to_sequence.
    """
    inp = {
        "apiVersion": "ordering.fn.crossplane.io/v1alpha1",
        "kind": "Input",
        "readyAfter": ready_after,
        "resources": names,
    }
    if delete_after:
        inp["deleteAfter"] = delete_after
    if edges and engine == "ordering":
        inp["edges"] = [{"resource": r, "dependsOn": d} for r, d in edges]

    pipeline = [
        {
            "step": "compose-in-order",
            "functionRef": {"name": "function-ordering"},
            "input": inp,
        }
    ]

    if engine == "sequencer" and edges:
        pipeline.append(
            {
                "step": "sequence",
                "functionRef": {"name": "function-sequencer"},
                "input": {
                    "apiVersion": "sequencer.fn.crossplane.io/v1beta1",
                    "kind": "Input",
                    "rules": [{"sequence": waves_to_sequence(names, edges)}],
                },
            }
        )

    return {
        "apiVersion": "apiextensions.crossplane.io/v1",
        "kind": "Composition",
        "metadata": {"name": name},
        "spec": {
            "compositeTypeRef": {"apiVersion": GROUP, "kind": KIND},
            "mode": "Pipeline",
            "pipeline": pipeline,
        },
    }


def waves_to_sequence(names, edges):
    """Flatten a graph into the ordered list function-sequencer takes.

    A sequence is strictly serial: each entry waits for the one before it to
    be ready, whether or not the graph says they are related. Flattening a
    graph into one therefore preserves every ordering constraint and adds
    constraints that were never declared - a fanout's hundred independent
    leaves become a hundred serial steps.

    That is not an artifact of this harness, it is what a list can express,
    and it is the comparison worth having: the same dependencies, stated as
    a sequence rather than as a graph. The walk below is in wave order, so
    the added serialization is the only difference.
    """
    deps = {n: set() for n in names}
    for r, d in edges:
        deps[r].add(d)

    done, seq = set(), []
    while len(done) < len(names):
        wave = sorted(n for n in names if n not in done and deps[n] <= done)
        if not wave:
            break
        seq.extend(wave)
        done.update(wave)

    return seq


def parse_ts(s):
    return datetime.datetime.strptime(s, "%Y-%m-%dT%H:%M:%SZ")


def composed(xr, kind="nopresource"):
    """Composed resources of this XR: name -> (created, ready_at, deleting).

    ready_at is the Ready condition's lastTransitionTime, which is what the
    ordering decision actually waits on. Without it a stalled wave can't be
    attributed: a gap between waves is either the dependency taking that long
    to become ready, or Crossplane taking that long to act on a dependency
    that already is. Those have different causes and different fixes.
    """
    out = kubectl(
        "get",
        RESOURCES[kind],
        "-n",
        NAMESPACE,
        "-o",
        "json",
        check=False,
    )
    if not out:
        return {}

    got = {}
    for it in json.loads(out).get("items", []):
        labels = it["metadata"].get("labels", {})
        if labels.get("crossplane.io/composite") != xr:
            continue

        ann = it["metadata"].get("annotations", {})
        name = ann.get("crossplane.io/composition-resource-name", it["metadata"]["name"])

        # A ConfigMap has no conditions, so existence is all the readiness
        # there is - which is what the function reports for it too.
        ready_at = parse_ts(it["metadata"]["creationTimestamp"]) if kind == "configmap" else None
        for c in it.get("status", {}).get("conditions", []):
            if c.get("type") == "Ready" and c.get("status") == "True":
                ready_at = parse_ts(c["lastTransitionTime"])

        got[name] = (
            parse_ts(it["metadata"]["creationTimestamp"]),
            ready_at,
            it["metadata"].get("deletionTimestamp") is not None,
        )

    return got


def attribute(created):
    """Split the elapsed time into waiting for readiness and waiting for Crossplane.

    For each wave after the first: when was the last resource of the previous
    wave ready, and when was this wave created? The difference is time
    Crossplane spent not acting on a graph that was ready to advance.
    """
    if not created:
        return []

    first = min(c for c, _, _ in created.values())
    by_wave = {}
    for name, (c, ready_at, _) in created.items():
        by_wave.setdefault(int((c - first).total_seconds()), []).append((name, ready_at))

    rows = []
    offsets = sorted(by_wave)
    for i, offset in enumerate(offsets):
        readies = [r for _, r in by_wave[offset] if r]
        last_ready = max(readies) if readies else None
        row = {
            "offset": offset,
            "count": len(by_wave[offset]),
            "ready_at": int((last_ready - first).total_seconds()) if last_ready else None,
        }

        if i > 0:
            prev = rows[i - 1]
            if prev["ready_at"] is not None:
                row["lag"] = offset - prev["ready_at"]

        rows.append(row)

    return rows


def waves(times):
    """Group timestamps into waves, one per distinct second offset.

    Wave boundaries are the gaps: resources created in the same second are one
    wave. With a readyAfter of a second or more, a gap is always larger than
    the spread within a wave.
    """
    if not times:
        return []

    first = min(times)
    buckets = {}
    for t in times:
        buckets.setdefault(int((t - first).total_seconds()), 0)
        buckets[int((t - first).total_seconds())] += 1

    return sorted(buckets.items())


def sample_usage(peak):
    """Record peak CPU and memory per pod, from metrics-server.

    Sampled rather than integrated: what matters for a scale run is whether
    anything sat at its limit, and a peak answers that. Silently does nothing
    if metrics-server isn't installed, because a missing sample should not
    fail a measurement that is otherwise fine.
    """
    out = kubectl("top", "pods", "-n", "crossplane-system", "--no-headers", check=False)
    for line in out.splitlines():
        parts = line.split()
        if len(parts) < 3:
            continue

        name, cpu, mem = parts[0], parts[1], parts[2]

        # Names carry a replicaset hash; the workload is the useful key.
        who = "crossplane" if name.startswith("crossplane-") and "rbac" not in name else name.split("-")[0]
        if name.startswith("provider-nop"):
            who = "provider-nop"

        try:
            millicores = int(cpu.rstrip("m")) if cpu.endswith("m") else int(float(cpu) * 1000)
            mebibytes = int(mem.rstrip("Mi"))
        except ValueError:
            continue

        was = peak.get(who, (0, 0))
        peak[who] = (max(was[0], millicores), max(was[1], mebibytes))


def core_metrics():
    """Crossplane's metrics, read through a port-forward.

    Not `kubectl exec`: the Crossplane image is distroless, so there is no
    shell or wget in it to run, and the attempt fails silently - which reads
    as "the breaker never opened" rather than "nothing was measured".
    """
    pf = subprocess.Popen(
        ["kubectl", "port-forward", "-n", "crossplane-system", "deploy/crossplane", ":8080"],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
    )

    try:
        # The chosen local port is only known from the first line it prints.
        line = pf.stdout.readline()
        if "->" not in line:
            return {}

        port = line.split(":")[1].split(" ")[0]

        got = {}
        for _ in range(10):
            try:
                with urllib.request.urlopen(f"http://127.0.0.1:{port}/metrics", timeout=5) as r:
                    body = r.read().decode()
                break
            except Exception:
                time.sleep(1)
        else:
            return {}

        for line in body.splitlines():
            if line.startswith(
                (
                    "circuit_breaker_opens_total",
                    "circuit_breaker_events_total",
                    "controller_runtime_reconcile_time_seconds_sum",
                    "controller_runtime_reconcile_time_seconds_count",
                    "controller_runtime_reconcile_total",
                )
            ):
                key, _, value = line.rpartition(" ")
                got[key] = value

        return got
    finally:
        pf.terminate()
        pf.wait(timeout=10)


def reconcile_cost(before, after, controller):
    """How long core spent per reconcile of this XR's controller.

    The question ordering raises is what a pass over the graph costs where it
    happens, and the honest in-cluster answer is the whole reconcile: rebuild
    the graph, decide, apply. Taking the difference of the histogram's sum and
    count across a run gives the mean reconcile for that controller over
    exactly the window measured, which can then be compared with the same run
    with --unordered.
    """
    def delta(metric):
        key = f'{metric}{{controller="{controller}"}}'
        try:
            return float(after[key]) - float(before[key])
        except (KeyError, ValueError):
            return None

    secs = delta("controller_runtime_reconcile_time_seconds_sum")
    count = delta("controller_runtime_reconcile_time_seconds_count")

    if secs is None or not count:
        return None

    return {"reconciles": int(count), "total_s": secs, "mean_ms": 1000 * secs / count}


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--shape", choices=["chain", "fanout", "layered"], default="fanout")
    ap.add_argument("--n", type=int, default=10, help="composed resources")
    ap.add_argument("--levels", type=int, default=4, help="levels, for --shape layered")
    ap.add_argument(
        "--kind",
        choices=sorted(RESOURCES),
        default="nopresource",
        help="what to compose: nopresource goes through a provider, configmap does not",
    )
    ap.add_argument(
        "--engine",
        choices=["ordering", "sequencer"],
        default="ordering",
        help="order with this proposal's graph, or with function-sequencer as it exists today",
    )
    ap.add_argument("--ready-after", default=None, help="how long each resource takes to report Ready")
    ap.add_argument("--delete-after", default="", help="how long each resource takes to delete")
    ap.add_argument("--unordered", action="store_true", help="compose the same resources with no edges, as a baseline")
    ap.add_argument("--no-teardown", action="store_true", help="leave the XR in place")
    ap.add_argument("--timeout", type=int, default=900, help="seconds to wait in each direction")
    args = ap.parse_args()

    # A ConfigMap has no conditions to flip, so readyAfter is a NopResource
    # setting: asking for it is what selects one.
    if args.ready_after is None:
        args.ready_after = "" if args.kind == "configmap" else "2s"

    if args.kind == "configmap" and (args.ready_after or args.delete_after):
        print("--kind configmap cannot use --ready-after or --delete-after", file=sys.stderr)
        return 2

    names, edges = graph(args.shape, args.n, args.levels)
    if args.unordered:
        edges = []

    run = f"{args.shape}-{args.n}-{int(time.time())}"
    ready = f", readyAfter={args.ready_after}" if args.ready_after else ""
    print(
        f"run {run}: {len(names)} resources, {len(edges)} edges, "
        f"{args.kind}s ordered by {args.engine}{ready}"
    )

    controller = f"composite/{KIND.lower()}s.{GROUP.split('/')[0]}"
    before = core_metrics()

    # Server-side apply, because a dense graph is a large manifest and
    # client-side apply stores a copy of it in an annotation, which a few
    # thousand edges pushes past the API server's 256KiB annotation limit.
    kubectl(
        "apply",
        "--server-side",
        "--field-manager=scale-harness",
        "-f",
        "-",
        stdin=json.dumps(
            composition(run, names, edges, args.ready_after, args.delete_after, args.engine)
        ),
    )
    kubectl(
        "apply",
        "--server-side",
        "--field-manager=scale-harness",
        "-f",
        "-",
        stdin=json.dumps(
            {
                "apiVersion": GROUP,
                "kind": KIND,
                "metadata": {
                    "name": run,
                    "namespace": NAMESPACE,
                    "annotations": {"crossplane.io/composition-ref": run},
                },
                "spec": {"crossplane": {"compositionRef": {"name": run}}},
            }
        ),
    )

    # --- Creation -----------------------------------------------------------

    start = time.time()
    created = {}
    peak = {}
    while time.time() - start < args.timeout:
        created = composed(run, args.kind)
        sample_usage(peak)
        if len(created) >= len(names):
            break
        time.sleep(2)

    elapsed = time.time() - start
    print(f"\ncreation: {len(created)}/{len(names)} in {elapsed:.0f}s")

    if created:
        for row in attribute(created):
            ready = f"ready +{row['ready_at']}s" if row["ready_at"] is not None else "not ready"
            lag = f"   waited {row['lag']}s after its dependencies were ready" if "lag" in row else ""
            print(f"  +{row['offset']:>4}s  {row['count']:>4} created, {ready}{lag}")

    if len(created) < len(names):
        print(f"  !! timed out with {len(names) - len(created)} never created")

    after = core_metrics()

    cost = reconcile_cost(before, after, controller)
    if cost:
        print(
            f"\ncore spent {cost['total_s']:.1f}s over {cost['reconciles']} reconciles "
            f"of this XR's controller, {cost['mean_ms']:.0f}ms mean"
        )

    opened = False
    for k, v in after.items():
        if not k.startswith("circuit_breaker"):
            continue

        print(f"  {k} {v}")
        if k.startswith("circuit_breaker_opens_total") and float(v) > 0:
            opened = True

    if opened:
        print("  !! the circuit breaker opened: these timings describe the breaker,")
        print("     not the ordering. Raise --circuit-breaker-burst and re-run.")

    if args.no_teardown:
        if peak:
            print("\npeak usage")
            for who, (cpu, mem) in sorted(peak.items()):
                print(f"  {who:<14} {cpu:>6}m cpu  {mem:>6}Mi")

        print(f"\nleft in place. Delete with: kubectl -n {NAMESPACE} delete xscale {run}")
        return 0

    # --- Teardown -----------------------------------------------------------

    kubectl("delete", "xscale", run, "-n", NAMESPACE, "--wait=false")

    start = time.time()
    first_seen = {}
    gone_at = None
    while time.time() - start < args.timeout:
        now = composed(run, args.kind)
        sample_usage(peak)
        for name, (_, _, deleting) in now.items():
            if deleting and name not in first_seen:
                first_seen[name] = time.time() - start
        if not now:
            gone_at = time.time() - start
            break
        time.sleep(2)

    print(f"\nteardown: {'all gone in %.0fs' % gone_at if gone_at else 'timed out'}")
    if first_seen:
        buckets = {}
        for name, at in first_seen.items():
            buckets.setdefault(int(at // 5) * 5, []).append(name)
        for offset in sorted(buckets):
            print(f"  +{offset:>4}s  {len(buckets[offset])} asked to delete")

    if peak:
        print("\npeak usage")
        for who, (cpu, mem) in sorted(peak.items()):
            print(f"  {who:<14} {cpu:>6}m cpu  {mem:>6}Mi")

    kubectl("delete", "composition", run, check=False)
    return 0


if __name__ == "__main__":
    sys.exit(main())
