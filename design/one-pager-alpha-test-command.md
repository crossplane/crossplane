# Add `crossplane alpha test` command

**Owner**: Kai Udo(@u-kai)
**Status**: Proposed

## Summary

Introduce `crossplane alpha test` and a minimal declarative test file (`apiVersion: test.crossplane.io/v1alpha1`, `kind: TestSuite`) to assert **rendered** outputs of:

- the **composite (XR)**
- a **composed** resource by **index**
- a **composed** resource by **GVKN** (`apiVersion`, `kind`, `name`, optional `namespace`)

## Problem / Motivation

- Today authors do `render` + manual inspection or generic linters; there are no first‑class assertions.
- Need a fast, CI‑friendly loop that uses Crossplane concepts (XR, composed) with stable addressing (GVKN, as in Kustomize).

## Proposal (alpha, minimal)

**Command**

```bash
# Single file or directory (MVP keeps options minimal)
crossplane alpha test -f tests/suite.yaml
crossplane alpha test -d tests/
```

**Rendering contract**

- Uses a rendering engine **behaviorally equivalent to `crossplane render`** (Functions are required).
- Implementation **may** call the CLI or reuse Go packages; the **contract is behavior**, not mechanism.

**Targets (MVP)**

- `resource: { composite: true }`
- `resource: { composed: { index: <0-based> } }`
- `resource: { composed: { apiVersion, kind, name, [namespace] } }` # GVKN

**Assertions (positive only)**

- Operators: `equal`, `exists`, `contains`, `regex`, `gt/gte/lt/lte`, `length`
- Paths: **JSONPath** (multi‑hit defaults to **any**)

**Inputs (expressed in the YAML test file)**

- Configure all render inputs **in the file**, not via CLI flags:
  - `functions` (required), `xr`, `composition`
  - `context` (`values`, `files`)
  - `observedResources`, `extraResources`
  - toggles: `includeFullXR`, `includeContext`, `includeFunctionResults`, `timeout`

- **Path resolution**: all relative paths are resolved **relative to the TestSuite file location** — the same approach as Kustomize, which resolves paths relative to its `kustomization.yaml`.

**Execution**

- **Directory mode**: recursively discover all `*.yaml` / `*.yml` files under the specified path; only files with `apiVersion: test.crossplane.io/*` and `kind: TestSuite` are executed; others are ignored.
- No pattern / filter / parallelism options in MVP.
- Exit codes: `0=pass`, `1=assertion-fail`, `2=input/target error`.

## MVP Scope (what ships)

- Single‑phase **render → assert → report**
- XR + composed(index/GVKN) selection
- Positive assertions only

## Non‑Goals (alpha)

- Negative/expect‑error tests, multi‑phase apply/patch/delete
- Live‑cluster E2E (KUTTL scope), policy engines, snapshots, selectors

## Tiny Example

```yaml
apiVersion: test.crossplane.io/v1alpha1
kind: TestSuite
spec:
  inputs:
    xr: ./examples/xr.yaml
    composition: ./examples/composition.yaml
    functions: ./examples/functions.yaml # required
  tests:
    - name: "first composed replicas = 3"
      resource: { composed: { index: 0 } }
      assertions:
        - path: "$.spec.replicas"
          operator: equal
          value: 3
```

## Versioning

- CLI subcommand ships as **alpha**; test file schema is `test.crossplane.io/v1alpha1`.
- CLI maturity label ≠ file schema version; both can evolve independently.
