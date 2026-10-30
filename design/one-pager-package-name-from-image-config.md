# Naming Packages with `ImageConfig`

* Owner: Cyrill Näf (@BigGold1310)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

When the Crossplane package manager installs a package that the user didn't ask
for by name — most commonly a dependency of a `Configuration` — it has to invent
a name for the `Provider`, `Configuration`, or `Function` object it creates. It
derives that name from the image's repository path:

```go
// internal/controller/pkg/resolver/reconciler.go
pack.SetName(xpkg.ToDNSLabel(ref.Context().RepositoryStr()))
```

`ToDNSLabel` flattens `/`, `.`, and `:` to `-`, drops every character that isn't
lowercase alphanumeric, and truncates at 63 characters. So
`xpkg.crossplane.io/crossplane-contrib/provider-aws` is always installed as
`crossplane-contrib-provider-aws`.

That is a reasonable default, but it is the only option, and it has real costs:

* **Long paths collide.** A mirrored or deeply nested repository path can be
  truncated to the point where two distinct packages produce the same name.
* **Names leak the registry layout.** Users who mirror packages internally end
  up with object names that encode their mirror's directory structure, even
  though `rewriteImage` exists precisely so the rest of the system doesn't have
  to care where an image physically lives.
* **Compositions have to guess.** A `functionRef.name` in a Composition has to
  match whatever the package manager decided to call the Function, so users must
  know the derivation rule to write a Composition against a dependency.

The only workaround today is to install the package yourself, under the name you
want, before the package manager gets to it. That works — the Lock and the
dependency DAG key on the package *source*, not on the object name, so a
pre-existing package satisfies the dependency — but it defeats the purpose of
dependency resolution.

`ImageConfig` is already where per-image package manager behavior is
configured: pull secrets, signature verification, path rewriting, and runtime
configuration. It already matches images by longest prefix. Naming belongs
there too.

## Goals

- Let users control the object name the package manager gives a package it
  installs on their behalf.
- Support both naming a single package explicitly and deriving names for a whole
  set of packages matched by a prefix.
- Keep the current derivation as the default, so nothing changes for users who
  don't opt in.

## Non-Goals

- Renaming packages that already exist. Names are assigned at creation time.
- Changing how package *revisions* are named, beyond their following their
  parent package's name as they already do.
- Changing how the package manager resolves dependencies. Resolution keys on the
  package source and is unaffected.

## Proposal

Add a `packageName` field to `ImageConfigSpec`. It is a single string, holding
either a literal name or a Go template:

```yaml
apiVersion: pkg.crossplane.io/v1beta1
kind: ImageConfig
metadata:
  name: provider-aws
spec:
  matchImages:
    - type: Prefix
      prefix: xpkg.crossplane.io/crossplane-contrib/provider-aws
  packageName: aws
```

There is no separate "literal" and "template" field, because `text/template`
doesn't distinguish them: a value with no template actions renders to itself.
`aws` is simply a template that happens to produce `aws`.

A CEL rule keeps the literal case honest — a value with no template delimiters
must be a valid DNS label, so `packageName: My_Provider` is rejected at apply
time rather than silently installed as `myprovider`:

```
self.contains('<<') || self.matches('^[a-z0-9]([-a-z0-9]*[a-z0-9])?$')
```

### Templates

A literal name only makes sense when `matchImages` selects a single repository.
For a prefix that matches many packages, a template derives a name per image:

```yaml
spec:
  matchImages:
    - type: Prefix
      prefix: registry.example.com/mirror/crossplane-contrib/
  packageName: "<< .Remainder >>"
```

Templates are delimited by `<<` and `>>` rather than Go's usual `{{` and `}}`.
`ImageConfig`s are routinely shipped inside Helm charts, and Helm consumes
`{{ ... }}` when it renders a chart — a user would otherwise have to write
`{{ "{{ .Remainder }}" }}` to get a template through intact.

A `packageName` written with Go's default delimiters is rejected at apply time:
`{{ .Remainder }}` contains no `<<`, so the CEL rule requires it to be a valid
DNS label, which it isn't. Reaching for the wrong delimiters is an error, not a
silently odd name.

Templates are rendered with both matcher-derived and generic values. For the
image `xpkg.crossplane.io/crossplane-contrib/provider-aws:v1` matched by the
prefix `xpkg.crossplane.io/crossplane-contrib/`:

| Field         | Value                                                |
| ------------- | ---------------------------------------------------- |
| `.Image`      | `xpkg.crossplane.io/crossplane-contrib/provider-aws` |
| `.Registry`   | `xpkg.crossplane.io`                                 |
| `.Repository` | `crossplane-contrib/provider-aws`                    |
| `.Path`       | `crossplane-contrib`                                 |
| `.Name`       | `provider-aws`                                       |
| `.Match`      | `xpkg.crossplane.io/crossplane-contrib/`             |
| `.Remainder`  | `provider-aws`                                       |
| `.ImageConfig`| the name of the matching `ImageConfig`               |

`.Match` and `.Remainder` mirror the semantics of `rewriteImage`: `.Match` is the
longest `matchImages` prefix that matched, and `.Remainder` is the image with
that prefix trimmed.

The functions `lower`, `trimPrefix`, `trimSuffix`, and `replace` are available.
The string being operated on comes last so it reads well at the end of a
pipeline:

```yaml
  packageName: '<< trimPrefix "provider-" .Name >>'
```

The rendered result is normalized with `xpkg.ToDNSLabel`, so a template can't
produce an invalid object name. A template that renders to something that
normalizes to nothing is a configuration error, and dependency resolution fails
with `ResolutionFailed` rather than silently falling back.

### Matching

Matching follows the existing `ImageConfig` rules exactly: the longest matching
`matchImages` prefix wins, globally, across all `ImageConfig`s that configure a
`packageName`. `ImageConfig`s without a `packageName` are ignored for naming,
just as those without a pull secret are ignored when resolving pull secrets.

Matching is done against the package's **original** source — the source before
any `rewriteImage` was applied. The package manager deliberately works in
original sources everywhere that names and identities are derived, so that
configuring a mirror doesn't change what things are called.

### Where it applies

Two places derive package names from an image and must agree, or the same image
ends up installed as two different objects:

1. `internal/controller/pkg/resolver` — names the package created for a
   dependency.
2. `internal/controller/apiextensions/composite.FunctionName` — names the
   `Function` created for a Composition or Operation pipeline step that
   references a function by OCI reference. Its existing doc comment already
   states that it must match the package manager's scheme so the two share
   Functions.

Both honor `packageName`. `FunctionRevision` names continue to be derived from
their parent `Function`'s name, so revisions follow whatever the Function is
called.

`internal/initializer` — which installs the packages named by the core
`--provider`, `--configuration`, and `--function` flags — is deliberately left
alone. Those packages are named by a user who is already naming them explicitly,
and the initializer already reuses the name of any existing package with the
same source.

### Caveats

**Renaming is not a migration.** Names are assigned when the object is created.
Adding or changing `packageName` on an `ImageConfig` does not rename anything
that already exists: the existing package keeps running and keeps satisfying the
dependency, because the Lock and the DAG key on source. The new name takes
effect the next time the package is installed from scratch.

**A broad prefix plus a literal `name` will collide.** Every package matched by
that `ImageConfig` wants the same object name, and only the first one installed
gets it. This is what `template` is for, and the field documentation says so.

## Alternatives Considered

### Go's default `{{ }}` delimiters

Using Go's defaults would be one less thing to explain, but `ImageConfig`s are
commonly shipped in Helm charts, and Helm consumes the template when it renders
the chart. It doesn't fail — it evaluates the template against its own context
and renders the field empty:

```console
$ helm template ./chart          # packageName: "{{ .Remainder }}" in the chart
  packageName: ""
```

Escaping (`{{ "{{ .Remainder }}" }}`) works but is hostile. `<<` and `>>`
sidestep it entirely and survive `helm template` verbatim. They also have the
side benefit that `<` can't appear in a DNS label, so the CEL rule can tell a
template from a literal name with a plain `contains` check — and a `packageName`
mistakenly written with `{{ }}` is rejected at apply time rather than installed
under an odd name.

### A new `ConfigStore` method upstream

The `ImageConfig` matching helpers now live in `crossplane-runtime`'s
`xpkg.ConfigStore`. Adding `PackageNameFor` there would keep all matching in one
place. It was not done for this change because naming is consumed by Crossplane
alone, and the prefix matching involved is already duplicated in the package
manager's watch handlers. Folding it into `ConfigStore` later is a mechanical
change.
