# Converting a real platform to declared ordering

[Modelplane](https://modelplane.ai) orders its compositions today the way
Crossplane users are told to: functions gate desired state on observed state,
and `Usage`s hold resources through teardown. Its three composing functions
were moved onto declared edges to see what changes when ordering becomes data
Crossplane holds rather than control flow each function implements.

The branch is
[`stevendborrelli/modelplane@composed-resource-ordering`][branch]. What it
proves is not that the graph is faster - it is not, for creation - but that
the same ordering can be expressed as edges, that expressing it that way
deletes a large amount of code, and that the code it deletes is the kind that
is easy to get wrong.

[branch]: https://github.com/modelplaneai/modelplane/compare/main...stevendborrelli:modelplane:composed-resource-ordering

## What it cost, in lines

| | Added | Removed | Net |
| --- | --- | --- | --- |
| Function code | 332 | 720 | **-388** |
| Tests | 571 | 821 | **-250** |
| `compose-usages`, deleted outright | 0 | 541 | **-541** |

A whole function left the pipeline. `compose-usages` existed only to compose
`Usage` resources so that teardown happened in an order, and there is nothing
for it to do once the order is declared.

Per composition:

| Function | Before | After |
| --- | --- | --- |
| `compose-serving-stack` | 14 `Usage`s, 22 observed-state reads | 5 `add_dependency` calls, 10 reads |
| `compose-inference-gateway` | 11 `Usage`s, 23 reads | 5 calls, 12 reads |
| `compose-inference-cluster` | 15 `Usage`s | 12 calls, 2 required resources |

## What the removed code was doing

Four kinds of thing disappeared, and they are worth naming separately because
they fail differently.

**Readiness gates.** `compose-serving-stack` carried a `deps_ready()` that
walked each component's `depends_on` list and checked the `Ready` condition of
every document each dependency renders - a wave computation, in a function,
over the platform's own component model.

**A gate that existed to dodge a race.** `provider_configs_observed()` kept
composed resources from being created against a ProviderConfig that Crossplane
had accepted but not yet persisted. It is pure defensive plumbing: it encodes
"do not create this yet" because there was no way to say "this depends on
that."

**Per-cloud `Usage` builders.** `compose-inference-cluster` had five
near-identical methods - `compose_nebius_usage`, `compose_vultr_usage`,
`compose_eks_usage`, `compose_aks_usage`, `compose_gke_usage` - each composing
a `Usage` so the backend outlived its cluster. All five became one line each:

```python
response.add_dependency(self.rsp, BACKEND_RESOURCE_KEY, f"{cloud}-cluster")
```

**The escape hatches.** This is the one that matters most. Every gate was
written as:

```python
if not (gate or c.key in self.req.observed.resources):
    continue
```

The `or key in observed` is not an optimization. Desired state expresses "not
yet" and "never again" with the same absence, so a resource withheld after it
exists is a resource Crossplane deletes. Every gate needs that clause, on
every path, forever, and forgetting it deletes live infrastructure. A graph
has no such ambiguity: **an edge says when, desired state says whether.**

## What replaced it

One method per function, and it reads as a description of the platform rather
than as control flow. From `compose-serving-stack`:

```python
for c in components:
    pc = _PC_HELM if isinstance(c, stacks.Chart) else _PC_KUBERNETES
    for key in docs[c.key]:
        response.add_dependency(self.rsp, key, pc)
    for dep in c.depends_on:
        for dependency_key in docs[dep]:
            for key in docs[c.key]:
                response.add_dependency(self.rsp, key, dependency_key)
```

The platform already had a `depends_on` field in its component model. It had
been used to drive gating; now it is translated directly into edges, and the
ordering logic is the translation.

Note what one edge buys. `add_dependency(key, pc)` constrains *both*
directions: the resource isn't created until the ProviderConfig is Ready, and
the ProviderConfig isn't deleted until the resource is gone. Under the old
code those were two separate mechanisms - a gate for creation, a `Usage` for
deletion - that had to be kept consistent with each other by hand.

## Required resources replaced a `Usage` pattern

`compose-inference-cluster` supports attaching an existing cluster, whose
kubeconfig the user supplies as a Secret. The old code could not wait for a
resource it does not compose, so it used `Usage` with
`matchControllerRef: false`.

Declared as a requirement instead:

```python
response.require_resources(
    self.rsp, name=_REQUIRED_KUBECONFIG, api_version="v1", kind="Secret",
    match_name=existing.secretRef.name, namespace=_NAMESPACE_SYSTEM,
)
...
response.add_required_resource_dependency(self.rsp, key, _REQUIRED_KUBECONFIG)
```

The edge names the requirement rather than a resource inside it, which is what
makes the semantics right: a requirement matching nothing is a *wait*, which
is what a Secret the user has not created yet should be. Naming a specific
resource that nothing matched would be a contradiction, because waiting cannot
resolve it.

These edges order creation only - Crossplane never deletes a resource it did
not compose - so nothing here constrains teardown.

## What it does not show

* **Creation is not faster.** Measured separately, a graph creates a chain no
  faster than `function-sequencer` does. The gain here is expressiveness and
  deleted code, not throughput.
* **Out-of-band protection is genuinely lost.** A `Usage` refuses a direct
  `kubectl delete` of a ProviderConfig. Edges order only the deletions
  Crossplane performs. That trade was made deliberately, and `Usage` keeps its
  role for protecting critical resources.
* **One platform, converted by someone who wanted it to work.** Three
  compositions, 18 composed resources at the largest. It is evidence that the
  conversion is possible and that it simplifies, not a controlled study.

## Why this bears on the comparison with #7242

[#7242][7242] generalizes the gating pattern, for deletion, by running the
pipeline during teardown. Modelplane is the closest thing available to a
controlled comparison, because it had already hand-written that pattern for
creation and can say what it costs to maintain.

What the conversion deleted was not the platform's complexity. The platform
does exactly what it did before. It deleted the machinery of expressing order
*as control flow* - the wave computation, the readiness walks, the race-dodging
gates, and the `or key in observed` clause on every one of them. Sequence
stopped being something each function implements and became something the
composition declares.

That is available to a templating function too, which is the part of the
ecosystem #7242 expects to decline the capability: edges are data, and
`function-go-templating` can emit data.

[7242]: https://github.com/crossplane/crossplane/pull/7242
