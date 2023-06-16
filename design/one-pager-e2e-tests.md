# Crossplane E2E Tests

* Owner: Nic Cope (@negz)
* Reviewers: Hasan Turken (@turkenh)
* Status: Accepted

## Background

Crossplane relies heavily on unit tests to ensure quality, but these alone are
not enough to ensure Crossplane functions correctly as a whole.

Thus far we have relied on an honor system of folks running manual black-box
tests to increase the likelihood that their changes work in real-world
conditions. These tests:

* Are not very repeatable, or portable for others to run.
* Are often only run on one environment - i.e. `kind` on the author's laptop.
* Are not re-run on future changes to catch regressions.
* Are likely to miss things like performance degradation.
* Are usually skipped for innocuous-seeming things like dependency bumps.
* Often don't consider interactions with features besides the one under test.
* Often don't cover upgrade scenarios.

Over the years this has lead to several (probably many) issues in released
versions of Crossplane that would _likely_ have been caught by E2E tests. Some
examples include:

* https://github.com/crossplane/crossplane/pull/3376
* https://github.com/crossplane/crossplane/issues/3113
* https://github.com/crossplane/crossplane/issues/3071

We've made several attempts to add integration and E2E tests to Crossplane in
the past, but none have flourished. Typically we've aligned on a pattern,
implemented the first test or two, then never touched them again. I believe this
is largely because we've relied on CNCF interns to kick-off the testing effort. 
The testing frameworks we've put into place are actually promising - it's just
hard to ensure contributors keep using them once your internship ends.

Examples include:

* https://github.com/crossplane/crossplane/blob/v1.12.2/cluster/local/integration_tests.sh
* https://github.com/crossplane/crossplane/tree/v1.12.2/test/e2e
* https://github.com/crossplane/test

`integration_tests.sh` is our most long-lived attempt at black-box testing. It
runs as part of every continuous integration (CI) job (i.e. on every PR) and
simply ensures Crossplane starts without crashing.

`crossplane/test` does run on a regular schedule, using a mix of tests defined
under `test/e2e` in c/c, and tests defined in its own repository. It covers:

* Upgrading Crossplane from stable to master builds, using the Helm chart.
* Upgrading a Provider from one version to another.
* Resolving Configuration package dependencies.
* Creating a claim using a very minimal Composition, without any patches or
  transforms.

Because it's run in a separate repository and isn't connected to PRs or releases
it's unclear whether anyone notices if/when tests break. It's also easy to
forget to update the tests when adding new functionality.

## Goals

The goal of this proposal is to:

* Automatically simulate real-world use of all of Crossplane's functionality.
* Catch the kind of bugs that unit tests miss, before they're merged.
* Ensure contributors actually add and maintain tests.
* Run reliably, with no flaky tests.

In this context we consider "Crossplane" to include only the functionality
defined in the crossplane/crossplane repo, e.g. the package manager, composition
engine, etc. Testing that any specific extension such as a provider or function
works as expected is out of scope.

Our "black box" is the artifacts produced from this repository - the Crossplane
containers and Helm chart. Testing their integration with the Kubernetes API
server is unavoidable, but it is not our goal to test that the API server
itself, or any particular client thereof, functions correctly. (Consider that
clients like `kubectl` don't interact _directly_ with Crossplane, they simply
write state to the API server that Crossplane reads).

## Proposal

To achieve our goals, I propose:

* We move all E2E testing code into this repo - c/c.
* We run all E2E tests on every PR, breaking the build when tests fail.
* We update the contributing guide and PR template to encourage E2E tests.
* We attempt to make E2E tests low effort to add and expand.

It will eventually become expensive and slow to run all E2E tests on every PR,
but we can face that problem when we come to it. We could add a GitHub action to
trigger specific tests on-command, or run them periodically (e.g. nightly).

### Checklists!

I propose the PR template be updated as follows:

```markdown
### Description of your changes

<!--
Briefly describe what this pull request does, and how it is covered by tests.
Be proactive - direct your reviewers' attention to anything that needs special
consideration.

You MUST either [x] check or ~strikethrough~ every item in the checklist below.

We love pull requests that fix an open issue. If yours does, use the below line
to indicate which issue it fixes, for example "Fixes #500".
-->

Fixes # 

I have:

- [ ] Read and followed Crossplane's [contribution process].
- [ ] Added or updated unit **and** E2E tests for my change.
- [ ] Run `make reviewable` to ensure this PR is ready for review.
- [ ] Added `backport release-x.y` labels to auto-backport this PR if necessary.

[contribution process]: https://git.io/fj2m9
```

Note that:

* The opening comment asks the author to describe how the contribution is
  covered by tests.
* The checklist contains a new entry pertaining to tests.
* The "explain how you (manually) tested this" section has been removed.
* https://github.com/mheap/require-checklist-action will make it impossible to
  merge PRs with an incomplete "I have" checklist.

### Writing E2E Tests

Contemporary E2E tests are either shell scripts or Go-heavy tests written using
the stdlib `testing` library and `client-go`. I propose that in order to make it
more likely for contributors to add or update E2E tests we attempt to reduce the
burden of doing so by:

* Using familiar Kubernetes YAML manifests as test fixtures.
* Building a library of common, reusable checks (e.g.) "does this resource have
  the desired condition?".
* Leveraging existing tools where possible; avoid reinventing the wheel.
* Sticking to tools that are familiar and unsurprising to contributors.

The overall goal being that eventually most tests will involve only adding or
updating YAML manifests, with a little Go plumbing to use existing functions to
check that Crossplane works correctly when they are applied.

To this end I propose that we:

* Use https://github.com/kubernetes-sigs/e2e-framework to write tests.
* Keep "assessment" functions (i.e. `features.Func` implementations) in a
  separate package from the test step definitions.

I believe e2e-framework is the best choice for us because it:

* Attempts to stay close to the familiar, idiomatic Go stdlib `testing` package.
* Is a Kubernetes SIG project showing early signs of broad ecosystem adoption.
* Is purpose-built for E2E testing "Kubernetes stuff" and "includes batteries"
  to help do that. (e.g. Has utilities for applying manifests, waiting for
  resources to be ready, etc.)

## Alternatives Considered

https://github.com/crossplane/crossplane/pull/4101 explored several other
potential E2E testing frameworks, including Godog (i.e. Cucumber), Terratest,
and kuttl. See that proposal for details.