# Build with Earthly

* Owner: Nic Cope (@negz)
* Status: Proposed

## Background

Crossplane uses a `Makefile` with several targets, like `make build`, to
automate tasks that developers frequently need to run when developing
Crossplane.

Crossplane also uses GitHub Actions for continous integration (CI), to validate
pull requests. Most of Crossplane's GitHub Actions workflows run the same Make
targets. This creates some consistency between local development and CI. For
example `make test` should have the same result whether run locally or in CI.

The `Makefile` includes a moderate library of other `Makefiles`. These are
imported from the `build/makelib` directory. The `build` directory is a Git
submodule. Its source is https://github.com/crossplane/build. Most maintainers
call it "the build submodule".

Crossplane uses the build submodule to:

- Install pinned versions of common tools (`helm`, `kubectl`, etc)
- Cross-compile Crossplane for several platforms
- Produce a multi-platform OCI image for Crossplane
- Run code generation - e.g. `go generate`
- Validate code by running linters, unit tests, and end-to-end (E2E) tests
- Automatically derive the semantic version of artifacts from git tags
- Publish OCI image artifacts to OCI registries
- Publish binary and Helm chart artifacts to AWS S3
- Promote artifacts to different distribution channels (i.e. tags, S3 dirs)

The build submodule is also used by Crossplane extensions, like Providers.
Providers use the build submodule to do more than core Crossplane - for example
they use it to spin up `kind` clusters and deploy Crossplane for testing.

In the 5+ years I've been a Crossplane maintainer, almost every new maintainer
(including myself) has expressed a dislike for the build submodule and a desire
to change build tooling.

I believe folks dislike the build submodule because:

- Make, as a language, has a high learning curve
- Few people have prior experience with advanced use of Make
- Needing to update a shared git submodule slows down changes to build logic

It's worth noting that builds using the submodule aren't fully hermetic. It
strives to be hermetic: for example it uses pinned versions of tools like `helm`
and uses a per-project Go module cache. However it doesn't manage the Go
toolchain, and uses the global Go build cache. I've never heard anyone complain
about this, but it's an area that could be improved.

## Proposal

I proposed we switch from Make to https://earthly.dev.

Earthly targets the 'glue' layer between language-specific tools like `go` and
CI systems like GitHub Actions. In Crossplane, Earthly would replace Make and
Docker. It's based on Docker's [BuildKit][buildkit], so all builds are
containerized and hermetic.

### Configuration

The Earthly equivalent of a `Makefile` is an `Earthfile`. An `Earthfile` is a
lot like a `Dockerfile`, but with Make-like targets:

```Dockerfile
VERSION 0.8
FROM golang:1.22
WORKDIR /go-workdir

deps:
    COPY go.mod go.sum ./
    RUN go mod download
    # Output these back in case go mod download changes them.
    SAVE ARTIFACT go.mod AS LOCAL go.mod
    SAVE ARTIFACT go.sum AS LOCAL go.sum

build:
    FROM +deps
    COPY main.go .
    RUN go build -o output/example main.go
    SAVE ARTIFACT output/example AS LOCAL local-output/go-example

docker:
    COPY +build/example .
    ENTRYPOINT ["/go-workdir/example"]
    SAVE IMAGE go-example:latest
```

You'd run `earthly +docker` to build the Docker target in this example.

At first glance Earthly looks very similar to a multi-stage Dockerfile. There's
a lot of overlap, but Earthly has a bunch of extra functionality that's useful
for a general purpose build tool, including:

* Invoking other Dockerized things ([`WITH DOCKER`][earthfile-with-docker]) -
  e.g. Crossplane's E2E tests
* Exporting files that changed in the build
  ([`SAVE ARTIFACT AS LOCAL`][earthfile-save-artifact])
* Targets that are simply aliases for a bunch of other targets.
* The ability to import Earthfiles from other repos without a submodule
  ([`IMPORT`][earthfile-import]).

I feel Earthly's key strength is its Dockerfile-like syntax. Before writing this
one-pager I ported 90% of Crossplane's build from Make to Earthly. I found it
much easier to pick up and iterate on than the build submodule.

### Performance

Earthly is as fast as Make when developing locally, but a little slower in CI.
CI is slower because the Go build cache doesn't persist across CI runs.

Here are a few local development comparisons using a Linux VM with 6 Apple M1
Max vCPUs and 20GiB of memory.

| Task | Make | Earthly |
| --- | --- | --- |
| Build with a cold cache | ~46 seconds | ~60 seconds |
| Build with a hot cache (no changes) | ~2 seconds | ~1 second |
| Build with a hot cache (one Go file changed) | ~8 seconds | ~8 seconds |
| Build for all platforms with a cold cache | ~4 minutes 10 seconds | ~4 minutes 40 seconds |
| Build for all platforms with a hot cache (one Go file changed) | ~42 seconds | ~32 seconds |

Here are some CI comparisons run on GitHub Actions standard workers.

| Task | Make | Earthly |
| --- | --- | --- |
| Run linters | ~3 minutes | ~4 minutes |
| Run unit tests | ~3 minutes | ~2.5 minutes |
| Publish artifacts | ~12 minutes | ~14 minutes |
| Run E2E tests | ~12 minutes | ~14 minutes |

Earthly uses caching to run containerized builds as fast as Make's "native"
builds. For Crossplane this primarily means two things:

* It caches Go modules, and will only redownload them if `go.mod` changes.
* It stores the Go build cache in a cache volume that's reused across builds.

This caching requires the BuildKit cache to persist across runs. The BuildKit
cache doesn't persist across GitHub Actions runs, because every job runs in a
clean runner environment.

Crossplane's Make based GitHub actions use the [cache] GitHub Action to save the
Go module cache and build cache after each run, and load it before the next.
There's no good way to do this in Earthly today, per
https://github.com/earthly/earthly/issues/1540.

Earthly's recommended approach to caching in CI is to use their Earthly
Satellite remote runners, or host your own remote BuildKit that persists across
runs. Neither are good fits for Crossplane. Satellites are a paid product, and
hosting BuildKit would mean paying for and operating build infrastructure.

Earthly supports 'remote caching' of build layers in an OCI registry, but this
doesn't include `CACHE` volumes (i.e. the Go build cache). Typically CI is
configured to push the cache using the `--max-remote-cache` on main builds, then
PR builds use the `--remote-cache` flag to load the cache.

My testing indicates remote caching would have little impact for our builds. For
example building Crossplane for all platforms, with one changed Go file, a cold
local cache, and a hot remote cache was only a second faster than building with
a cold cache. This is because the difference is mostly whether Go modules are
downloaded from the Go module proxy via `go mod download` or downdloaded from an
OCI registry as a cached layer. It's possible GitHub Actions caching to GitHub
Container Registry would have a more significant impact on build times.

## Risks

Earthly is an early product, currently at v0.8.11. In my testing it's been
mostly stable, though I've had to restart BuildKit a small handful of times due
to errors like https://github.com/earthly/earthly/issues/2454.

Earthly also appears to be owned and primarily staffed by a single vendor, who
presumably would like to build a business around it. This could create conflicts
of interest - for example Earthly probably isn't incentivised to make CI caching
better given they're selling a CI caching solution (Satellites). It's worth
noting that Earthly switched _from_ BSL to MPL already.

## Alternatives Considered

I considered the following alternatives.

### Make, but simpler

Make isn't so bad when you only have a small handful of really simple targets.
In theory, this could be a nice alternative - strip everything down into one
streamlined `Makefile`.

Unfortunately I don't think there's much in `makelib` that we can eliminate to
achieve this. The functionality (pinning tools, building for multiple platforms,
etc) has to be implemented somewhere.

### Multistage docker builds

This is the closest alternative to Earthly. It has the notable advantage that
Docker is able to leverage bind mounts and/or [native GitHub Actions cache
support][docker-actions-cache] to cache the Go build cache across runs.

The main reason to avoid this route is that Docker doesn't make a great general
purpose build tool. For example there's no great way to invoke our (`kind`
based) E2E tests, or even output build artifacts. Earthly makes this point
pretty well in [this article][earthly-repeatable-builds].

### Dagger

[Dagger][dagger] is architecturally similar to Earthly, in that it's built on
BuildKit and all builds are containerized. It differs significantly in how you
configure your build.

In Dagger, you install one or more Dagger Functions. You then invoke these
Functions via the `dagger` CLI. There's no equivalent of a `Makefile` or an
`Earthfile` - if you need to string multiple functions together you write a new
function that calls them, and call that function.

The result is you end up defining your build logic in a language like Go, for
example:

* https://docs.dagger.io/quickstart/822194/daggerize
* https://docs.dagger.io/quickstart/428201/custom-function

I could see this becoming useful if our build logic became _really_ complex, but
for our current use cases I prefer the simpler `Earthfile` syntax.

### Bazel and friends

[Bazel][bazel] and similar Google-Blaze-inspired tools like Pants and Buck focus
on fast, correct builds. They're especially well suited to large monorepos using
multiple languages, where building the entire monorepo for every change isn't
feasible. Bazel uses `BUILD` files with rules written in Starlark, a Pythonic
language.

Bazel doesn't wrap tools like `go`, it completely replaces them. It's not
compatible with Go modules for example, and instead offers tools like `gazelle`
to generate a `BUILD` file from a module-based third party dependency.

Bazel has a pretty large learning curve and tends to require a lot of care and
feeding to keep its `BUILD` files up-to-date. I don't feel it's a great fit for
a medium sized, single language, manyrepo project like Crossplane.

[buildkit]: https://github.com/moby/buildkit
[earthfile-with-docker]: https://docs.earthly.dev/docs/earthfile#with-docker
[earthfile-save-artifact]: https://docs.earthly.dev/docs/earthfile#save-artifact
[earthfile-import]: https://docs.earthly.dev/docs/earthfile#import
[cache]: https://github.com/actions/cache
[docker-actions-cache]: https://docs.docker.com/build/cache/backends/gha/
[earthly-repeatable-builds]: https://earthly.dev/blog/repeatable-builds-every-time/
[dagger]: https://dagger.io
[bazel]: https://bazel.build