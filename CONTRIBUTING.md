# Contributing to Crossplane

Welcome, and thank you for considering contributing to Crossplane. We encourage
you to help out by raising issues, improving documentation, fixing bugs, or
adding new features

If you're interested in contributing please start by reading this document. If
you have any questions at all, or don't know where to start, please reach out to
us on [Slack]. Please also take a look at our [code of conduct], which details
how contributors are expected to conduct themselves as part of the Crossplane
community.

## Establishing a Development Environment

The Crossplane project consists of several repositories under the crossplane and
crossplane-contrib GitHub organisations. Most of these projects use the Upbound
[build submodule]; a library of common Makefiles. Establishing a development
environment typically requires:

1. Forking and cloning the repository you wish to work on.
1. Installing development dependencies.
1. Running `make` to establish the build submodule.

Run `make help` for information on the available Make targets. Useful targets
include:

* `make test` - Run unit tests.
* `make e2e` - Run end-to-end tests.
* `make reviewable` - Run code generation and linters.
* `make` - Build Crossplane.

Once you've built Crossplane you can deploy it to a Kubernetes cluster of your
choice. [`kind`] (Kubernetes in Docker) is a good choice for development. The
`kind.sh` script contains several utilities to deploy and run a development
build of Crossplane to `kind`:

```bash
# Build Crossplane locally.
make

# See what commands are available.
./cluster/local/kind.sh help

# Start a new kind cluster. Specifying KUBE_IMAGE is optional.
KUBE_IMAGE=kindest/node:v1.16.15 ./cluster/local/kind.sh up

# Use Helm to deploy the local build of Crossplane.
./cluster/local/kind.sh helm-install

# Use Helm to upgrade the local build of Crossplane.
./cluster/local/kind.sh helm-upgrade
```

When iterating rapidly on a change it can be faster to run Crossplane as a local
process, rather than as a pod deployed by Helm to your Kubernetes cluster. Use
Helm to install your local Crossplane build per the above instructions, then:

```bash
# Stop the Helm-deployed Crossplane pod.
kubectl -n crossplane-system scale deploy crossplane --replicas=0

# Run Crossplane locally; it should connect to your kind cluster if said cluster
# is your active kubectl context. You can also go run cmd/crossplane/main.go.
make run
```

> Note that local development using minikube and microk8s is also possible.
> Simply use the `minikube.sh` or `microk8s.sh` variants of the above `kind.sh`
> script to do so. Their arguments and functionality are identical.

## Contributing Code

To contribute bug fixes or features to Crossplane:

1. Communicate your intent.
1. Make your changes.
1. Test your changes.
1. Update documentation and examples.
1. Open a Pull Request (PR).

Communicating your intent lets the Crossplane maintainers know that you intend
to contribute, and how. This sets you up for success - you can avoid duplicating
an effort that may already be underway, adding a feature that may be rejected,
or heading down a path that you would be steered away from at review time. The
best way to communicate your intent is via a detailed GitHub issue. Take a look
first to see if there's already an issue relating to the thing you'd like to
contribute. If there isn't, please raise a new one! Let us know what you'd like
to work on, and why. The Crossplane maintainers can't always triage new issues
immediately, but we encourage you to bring them to our attention via [Slack].

> NOTE: new features can only being merged during the active development period
> of a Crossplane release cycle. If implementation and review of a new feature
> cannot be accomplished prior to feature freeze, it may be bumped to the next
> release cycle. See the [Crossplane release cycle] documentation for more
> information.

Be sure to practice [good git commit hygiene] as you make your changes. All but
the smallest changes should be broken up into a few commits that tell a story.
Use your git commits to provide context for the folks who will review PR, and
the folks who will be spelunking the codebase in the months and years to come.
Ensure each of your commits is signed-off in compliance with the [Developer
Certificate of Origin] by using `git commit -s`. The Crossplane highly values
readable, idiomatic Go code. Familiarise yourself with common [code review
comments] that are left on Go PRs - try to preempt any that your reviewers would
otherwise leave. Run `make reviewable` to lint your change.

All Crossplane code must be covered by tests. Note that unlike many Kubernetes
projects Crossplane does not use gingko tests and will request changes to any PR
that uses gingko or any third party testing library, per the common Go [test
review comments]. Crossplane encourages the use of table driven unit tests. The
tests of the [crossplane-runtime] project are representative of the testing
style Crossplane encourages; new tests should follow their conventions. Note
that when opening a PR your reviewer will expect you to detail how you've tested
your work. For all but the smallest changes some manual testing is encouraged in
addition to unit tests.

All Crossplane documentation and examples are under revision control; see the
[docs] and [examples] directories of this repository. Any change that introduces
new behaviour or changes existing behaviour must include updates to any relevant
documentation and examples. Please keep documentation and example changes in
distinct commits.

Once your change is written, tested, and documented the final step is to have it
reviewed! You'll be presented with a template and a small checklist when you
open a PR. Please read the template and fill out the checklist. Please make all 
PR request changes in subsequent commits. This allows your reviewers to see what
has changed as you address their comments. Be mindful
of  your commit history as you do this - avoid commit messages like "Address
review feedback" if possible. If doing so is difficult a good alternative is to
rewrite your commit history to clean them up after your PR is approved but
before it is merged.

In summary, please:

* Discuss your change in a GitHub issue before you start.
* Use your Git commit messages to communicate your intent to your reviewers.
* Sign-off on all Git commits by running `git commit -s`
* Add or update tests for all changes.
* Preempt common [code review comments] and [test review comments].
* Update all relevant documentation and examples.
* Don't force push to address review feedback. Your commits should tell a story.
* If necessary, tidy up your git commit history once your PR is approved.

Thank you for reading through our contributing guide! We appreciate you taking
the time to ensure your contributions are high quality and easy for our
community to review and accept. Please don't hesitate to [reach out to
us][Slack] if you have any questions about contributing!

[Slack]: https://crossplane.slack.com/channels/dev
[code of conduct]: https://github.com/cncf/foundation/blob/master/code-of-conduct.md
[build submodule]: https://github.com/upbound/build/
[`kind`]: https://kind.sigs.k8s.io/
[Crossplane release cycle]: docs/reference/release-cycle.md
[good git commit hygiene]: https://www.futurelearn.com/info/blog/telling-stories-with-your-git-history
[Developer Certificate of Origin]: https://github.com/apps/dco
[code review comments]: https://github.com/golang/go/wiki/CodeReviewComments
[test review comments]: https://github.com/golang/go/wiki/TestComments
[crossplane-runtime]: https://github.com/crossplane/crossplane-runtime
[docs]: docs/
[examples]: examples/
