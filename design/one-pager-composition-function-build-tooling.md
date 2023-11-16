# Building A Composition Function

* Owner: Nic Cope (@negz)
* Reviewers: Jared Watts (@jbw976)
* Status: Draft

## Background

A Composition Function extends Crossplane to support a new way of configuring
how to reconcile a composite resource (XR). Each Function is a gRPC server.
Crossplane sends state to a Function via a `RunFunctionRequest` RPC. The
Function is intended to return updated state via a `RunFunctionResponse` RPC.

Part of the goal of Composition Functions is to allow Crossplane users to build
Functions using their general purpose programming language (GPL) of choice. I
think there will be broadly two types of Function:

1. Generic, reusable Functions. These Functions support a common use case.
   They're intended to work with any kind of XR, and often expose a domain
   specific language (DSL) that their users must use to express Composition
   logic.
   [`function-patch-and-transform`][function-patch-and-transform] and
   [`function-auto-ready`][function-auto-ready] are examples of generic
   Functions.
2. Purpose-build Functions. These Functions are strongly coupled to a the schema
   of a specific kind of XR. Rather that exposing a configuration DSL, the
   Function's GPL code _is_ the Composition logic.

Take a look at the [Composition Functions design document][design-doc] for more
context on Functions. Quoting from that document:

> I believe it's critical that Composition Functions are easy to develop - much
> easier than developing a Crossplane Provider.

Another way to think about this is that the developer experience must scale.
Some Functions will be software engineering projects. They'll be maintained by a
team of contributors, have unit and end-to-end tests, release branches,
continuous integration (CI), etc.

At the other, simpler, end of the scale many Functions will be more like
configuration that happens to be expressed using a GPL. For someone writing a
Function like this, needing to learn and use a new set of build and CI tools
is a potentially huge barrier to entry.

The emergent process for developing a Composition Function is:

1. Scaffold a new Function from a template using `crossplane beta xpkg init`.
2. Add your logic to the scaffolded project (e.g. edit `fn.go`).
3. Optionally, add and run unit tests for your Function logic.
4. Optionally, test your Function end-to-end using `crossplane beta render`

Once you're satisfied that your Function works end-to-end it's time to make it
available to install and use in your Crossplane control plane. To do this you
must:

1. Build your Function's "runtime" - the OCI image used to deploy it.
2. Build a package from the OCI image runtime using `crossplane xpkg build`.
3. Push your package to an OCI registry using `crossplane xpkg push`

This document proposes a set of guiding principles for the Composition Function
developer experience, with a particular focus on build and CI. Ultimately
Functions are open-ended enough that a developer could choose whatever path best
suits them, so consider this a proposed "golden path". This golden path will
inform what choices we make in Function template repositories like
[`function-template-go`][function-template-go], and thus establish patterns for
the broader Function ecosystem.

## Proposal

I propose that we strive to keep the set of tools and technologies a Crossplane
user must learn to write a Function as small at possible. Learning Crossplane
alone is hard enough - we don't want users to also need to learn a new build
tool if we can avoid it.

I propose that the minimum set of tools required to build a Function be:

1. Your programming language of choice - e.g. Go, Python.
2. Docker.
3. The `crossplane` CLI.

When it comes to templates used to scaffold a new Function I think we'll want to
offer a little more than the minimum required set of tools. For example I
believe linting, testing, and CI should be part of the golden path we establish.
Most language runtimes aren't opinionated about these things so we'll need to
make some tooling choices that will affect Function developers.

Where we must include a tool in a Function template, I propose we bias for tools
that:

* Where possible, are the de-facto standard for their language or ecosystem.
* Are idiomatic and widely adopted within their language ecosystem.
* Have great documentation and educational resources available.

Put otherwise, we should bias for tools that a developer likely already knows
and uses. For example if you're a Python developer there's a good chance you're
familiar with the `pylint` linter.

For CI, I propose we stick with GitHub Actions and avoid 'intermediary'
scripting or automation layers such as `make`. For example, a Function runtime
should build in CI using the [`docker/build-push-action`][docker-build-push-action]. 

## An Example

Today only one Function template exists -
[`function-template-go`][function-template-go] for the Go programming language.
Go is an interesting example for two reasons:

* The language is more opinionated than most when it comes to tooling. For
  example it includes tools like `go test` and `go generate`.
* The Crossplane is written in Go, so we have established patterns and
  practices, like heavy use of Makefiles and the [build submodule].

In `function-template-go`, I propose that Function developers:

* Use native tools like `go run`, `go generate`, and `go test` to develop and
  (unit) test their Function. 
* Use the language's defacto standard linter, `golangci-lint`, to lint their
  Function.
* Use a GitHub Actions workflow consisting only of steps that either invoke
  widely used and documented actions (e.g. `docker/build-push-action`), run a
  `go` command, or run a `crossplane` command.

For example, to run unit tests in CI:

```yaml
jobs:
  unit-test:
    runs-on: ubuntu-22.04
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: ${{ env.GO_VERSION }}

      - name: Run Unit Tests
        run: go test -v -cover ./...
```

[design-doc]: ./design-doc-composition-functions.md
[function-patch-and-transform]: https://github.com/crossplane-contrib/function-patch-and-transform
[function-auto-ready]: https://github.com/crossplane-contrib/function-auto-ready
[function-template-go]: https://github.com/crossplane/function-template-go
[docker-build-push-action]: https://github.com/docker/build-push-action
[build submodule]: https://github.com/upbound/build
