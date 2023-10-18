# Contributing to Crossplane

Welcome, and thank you for considering contributing to Crossplane. We encourage
you to help out by raising issues, improving documentation, fixing bugs, or
adding new features

If you're interested in contributing please start by reading this document. If
you have any questions at all, or don't know where to start, please reach out to
us on [Slack]. Please also take a look at our [code of conduct], which details
how contributors are expected to conduct themselves as part of the Crossplane
community.

## Checklist Cheat Sheet

Wondering whether something on the pull request checklist applies to your PR?
Generally:

* Everyone must read and follow this contribution process.
* Every PR must run (and pass) `make reviewable`.
* Most PRs that touch code should touch unit tests. We want ~80% coverage.
* Any significant feature should be covered by E2E tests. If you're adding a new
  feature, you should probably be adding or updating E2Es.
* Any significant feature should be documented. If you're adding a new feature,
  you should probably be opening a docs PR or tracking issue. If you make a
  change it's your responsibility to document it before it's released.
* Most PRs that (only) fix a bug should have a backport label.

If you're still unsure, just leave the checklist box unticked (and
un-struck-through). This will cause the `checklist-completed` CI job to fail
until you and your reviewer figure out what to do.

## Contributing Code

To contribute bug fixes or features to Crossplane:

1. Communicate your intent.
1. Make your changes.
1. Test your changes.
1. Update documentation and examples where appropriate.
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
Certificate of Origin] by using `git commit -s`. The Crossplane project highly
values readable, idiomatic Go code. Familiarise yourself with the
[Coding Style](#coding-style) section below and try to preempt any comments your
reviewers would otherwise leave. Run `make reviewable` to lint your change.

All Crossplane features must be covered by unit **and** end-to-end (E2E) tests.

Crossplane uses table driven unit tests - you can find an example
[below](#prefer-table-driven-tests). Crossplane does not use third-party test
libraries (e.g. Ginkgo, Gomega, Testify) for unit tests and will request changes
to any PR that introduces one. See the Go [test review comments] for our
rationale.

E2E tests live under `test/e2e`. Refer to the [E2E readme] for information on
adding and updating E2E tests. They are considered to be expensive,
therefore add them only for important use cases that cannot be verified by
unit tests. If in a doubt, check with the maintainers for guidance.

All Crossplane documentation is under revision control; see the [docs]
repository. Any change that introduces new behaviour or changes existing
behaviour must include updates to any relevant documentation. Please keep
documentation changes in distinct commits.

Once your change is written, tested, and documented the final step is to have it
reviewed! You'll be presented with a template and a small checklist when you
open a PR. Please read the template and fill out the checklist. Please make all
requested changes in subsequent commits. This allows your reviewers to see what
has changed as you address their comments. Be mindful of  your commit history as
you do this - avoid commit messages like "Address review feedback" if possible.
If doing so is difficult a good alternative is to rewrite your commit history to
clean them up after your PR is approved but before it is merged.

In summary, please:

* Discuss your change in a GitHub issue before you start.
* Use your Git commit messages to communicate your intent to your reviewers.
* Sign-off on all Git commits by running `git commit -s`
* Add or update unit and E2E tests for all changes.
* Preempt [coding style](#coding-style) review comments.
* Update all relevant documentation.
* Don't force push to address review feedback. Your commits should tell a story.
* If necessary, tidy up your git commit history once your PR is approved.

Thank you for reading through our contributing guide! We appreciate you taking
the time to ensure your contributions are high quality and easy for our
community to review and accept. Please don't hesitate to [reach out to
us][Slack] if you have any questions about contributing!

## Certificate of Origin

By contributing to this project you agree to the Developer Certificate of Origin
(DCO). This document was created by the Linux Kernel community and is a simple
statement that you, as a contributor, have the legal right to make the
contribution. See the [DCO](../DCO) file for details.

Contributors sign-off that they adhere to these requirements by adding a
Signed-off-by line to commit messages. For example:

```text
This is my commit message

Signed-off-by: Random J Developer <random@developer.example.org>
```

Git even has a -s command line option to append this automatically to your
commit message:

```bash
git commit -s -m 'This is my commit message'
```

If you have already made a commit and forgot to include the sign-off, you can
amend your last commit to add the sign-off with the following command, which can
then be force pushed.

```bash
git commit --amend -s
```

We use a [DCO bot] to enforce the DCO on all commits in every pull request.

## Code Review Process

All Pull Requests (PR), whether written by a Crossplane maintainer or a
community member, must go through code review.

Not only do code reviews ensure that the code is correct, maintainable, and
secure, but more importantly it allows us to use code reviews as an educational
tool.

With this in mind, all efforts around code reviews should be seen through the
lens of educating the author and be accompanied by kind, detailed feedback
that will help authors understand the context the reviewer is coming from.

We encourage anyone in the community to conduct a code review on a PR.
In most situations we prefer to have the following approvals before
merging a PR:

* At least one approval from [Reviewers]
* At least one approval from [Maintainers]

When opening a PR, GitHub will assign reviewers based on the project's
code review settings and who gets assigned depends on what code the contributor
changed, per [CODEOWNERS]. In most cases we expect that someone from
[crossplane-reviewers] and a subject matter expert from
[crossplane-maintainers] will be assigned.

We encourage reviews from the community and [Reviewers] to take place before
someone from the [Maintainers] group reviews a PR. This helps reduce the load
on the project maintainers and ensures they can be more efficient in their
reviews. In addition, reviewing PRs is a path to becoming a maintainer on the
project.

### Expectation of PR Authors

While preparing your PR, be mindful of the instructions and requirements in the
[contributing code](#contributing-code) section.

Once your PR is ready for review please notify the assigned reviewers by
mentioning them in a comment.

After implementing review feedback, the PR author should notify the reviewer
by mentioning them in a comment when the PR is ready for another
review.

If you are not getting a response within a reasonable timeframe, remembering
that reviewers and maintainers offer their time free of charge and have other
obligations, you can reach out to the `#crossplane-owners` channel in the
Crossplane community [Slack] workspace.

### Expectation of Reviewers

If you are assigned as a reviewer on a PR and you are unable to commit to
reviewing the PR within a reasonable timeframe, you are encouraged to
communicate this and manage the PR author's expectation.

All reviewers are encouraged to consider the following aspects and to provide
guidance to the PR author before giving their approval:

* Is the code functionally correct?
* Are the changes well documented, and their intent explained sufficiently for
current and future readers?
* Is the code written according to the [Coding Style](#coding-style)?
* Is the solution idiomatically aligned with existing Crossplane APIs?
* Is the code sufficiently covered by tests?
* Has the PR author signed the DCO?
* Are all CI jobs passing?

When providing feedback please consider the following guidelines:

* It helps the recipient a lot to be able to understand the context and
intention behind comments.
* Aim to provide feedback in a conversational style, rather than terse
instructions.
* Clearly articulate if you are sharing an opinion or instruction that
needs to be complied with (i.e. a contribution guide rule). Proactively clarify
what needs to change for you to feel comfortable to approve a PR.
* Default to asking questions when things are not how we would expect them to
be. Suggesting rather than demanding changes.
* Proactively provide context when asking people to change things. Refer to
where rules are defined or existing precedent exists, where possible.
* Allow the author to “win some battles”. Particularly if they’re pushing
back on something that isn't crucial.

Examples:

* “What do you think about changing X? I think it would be an improvement
because Y”.
* "Do we need X at all in this scenario? My thinking is: ..."
* "I like the direction this is going. I think adding X would be useful
to Y.""
* "Please would you make sure your commits are signed (see the DCO check) and
update the PR description per the template (in particular detail how
you've tested this change)."
* "I'm not 100% sure I follow why this is needed - can you add a comment
(to the code) explaining?"
* "I might be wrong, but I think what you're actually trying to do here is X"

Being specific with your intention and expectation can save hours of
undercommunication and misunderstandings.

## Coding Style

The Crossplane project prefers not to maintain its own style guide, but we do
enforce the style and best practices established by the Go project and its
community. This means contributors should:

* Follow the guidelines set out by the [Effective Go] document.
* Preempt common Go [code review comments] and [test review comments].
* Follow Crossplane's [Observability Developer Guide].

These coding style guidelines apply to all https://github.com/crossplane and
https://github.com/crossplane-contrib repositories unless stated otherwise.

Below we cover some of the feedback we most frequently leave on pull requests.
Most of these are covered by the documents above, but may be subtle or easily
missed and thus warrant closer attention.

### Explain 'nolint' Directives

We use [golangci-lint] on all our repositories to enforce many style and safety
rules that are not covered here. We prefer to tolerate false positives from our
linter configuration in order to make sure we catch as many issues as possible.
This means it's sometimes necessary to override the linter to make a build pass.

You can override the linter using a `//nolint` comment directive. When you do so
you must:

1. Be specific. Apply `//nolint:nameoflinter` at as tight a scope as possible.
1. Include a comment explaining why you're disabling the linter.

For example:

```go
func hash(s string) string {
        h := fnv.New32()
        _ = h.Write([]byte(s)) //nolint:errcheck // Writing to a hash never returns an error.
        return fmt.Sprintf("%x", h.Sum32())
}
```

Here we only disable the specific linter that would emit a warning (`errcheck`),
for the specific line where that warning would be emitted.

### Use Descriptive Variable Names Sparingly

Quoting the Go [code review comments]:

> Variable names in Go should be short rather than long. This is especially true
> for local variables with limited scope. Prefer `c` to `lineCount`. Prefer `i`
> to `sliceIndex`.
>
> The basic rule: the further from its declaration that a name is used, the more
> descriptive the name must be. For a method receiver, one or two letters is
> sufficient. Common variables such as loop indices and readers can be a single
> letter (`i`, `r`). More unusual things and global variables need more
> descriptive names.

Another way to frame the above is that we prefer to use short variables in all
cases where a (human) reader could easily infer what the variable was from its
source. For example:

```go

// NumberOfGeese might be used outside this package, or many many lines further
// down the file so it needs a descriptive name. It's also just an int, which
// doesn't give the reader much clue about what it's for.
const NumberOfGeese = 42

// w is plenty for the first argument here. Naming it gooseWrangler is redundant
// because readers can tell what it is from its type. looseGeese on the other
// hand warrants a descriptive name. It's short lived (lines wise), and its type
// doesn't give us any context about what it's for.
func capture(w goose.Wrangler, looseGeese int) error {
        // Important goose capturing logic.
        for looseGeese > 0 {
                // It's not obvious from the w.Wrangle method name what the
                // return value is, so a descriptive name names sense here too.
                captured, err := w.Wrangle()
                if err != nil {
                        return errors.Wrap(err, "defeated by geese")
                }
                looseGeese = looseGeese - captured
        }

        // We prefer 'y' to 'yard' here because 'yard' is implied by 'NewYard'.
        y := goose.NewYard(w)
        return y.Secure()
}
```

### Don't Wrap Function Signatures

Quoting again from the Go [code review comments]:

> Most of the time when people wrap lines "unnaturally" (in the middle of
> function calls or function declarations, more or less, say, though some
> exceptions are around), the wrapping would be unnecessary if they had a
> reasonable number of parameters and reasonably short variable names. Long
> lines seem to go with long names, and getting rid of the long names helps a
> lot.

```go
func capture(gooseWrangler goose.Wrangler, looseGeese int, gooseYard goose.Yard,
        duckWrangler duck.Wrangler, looseDucks, duckYard duck.Yard) error {
        // Important fowl wrangling logic.
}
```

If you find the need to wrap a function signature like the above it's almost
always a sign that your argument names are superfluously verbose, or that your
function is doing too much. If your function needs to take many optional
arguments, perhaps to enable dependency injection, use variadic functions as
options. In this case we usually make an exception for wrapped function calls.
For example:

```go
type Wrangler struct {
        fw fowl.Wrangler
        loose int
}

type Option func(w *Wrangler)

func WithFowlWrangler(fw fowl.Wrangler) Option {
        return func(w *Wrangler) {
                w.fw = fw
        }
}

func NewWrangler(looseGeese int, o ...Option) *Wrangler {
        w := &Wrangler{
                fw: fowl.DefaultWrangler{}
                loose: 
        }

        for _, fn := range o {
                fn(w)
        }

        return w
}

func example() {
        w := NewWrangler(42,
                WithFowlWrangler(chicken.NewWrangler()),
                WithSomeOtherOption(),
                WithYetAnotherOption())
        
        w.Wrangle()
}
```

You can read more about this pattern on [Dave Cheney's blog].

### Return Early

We prefer to return early. Another way to think about this is that we prefer to
handle terminal cases (e.g. errors) early. So for example instead of:

```go
func example() error {
        v := fetch()
        if v == 42 {
                // Really important business logic.
                b := embiggen(v)
                for k, v := range lookup(b) {
                        if v == true {
                                store(k)
                        } else {
                                remove(k)
                        }
                }
                return nil
        }
        return errors.New("v was a bad number")
}
```

We prefer:

```go
func example() error {
        v := fetch()
        if v != 42 {
                return errors.New("v was a bad number")
        }
        // Really important business logic.
        b := embiggen(v)
        for k, v := range lookup(b) {
                // "Continue early" is a variant of "return early".
                if v == false {
                        remove(k)
                        continue
                }
                store(k)
        }
        return nil
}
```

This approach gets error handling out of the way first, allowing the 'core' of
the function to follow at the scope of the function, not a conditional. Or put
otherwise, with the least amount of indentation. An interesting side effect of
this approach is that it's rare to find an `else` in Crossplane code (at the
time of writing there are four uses of `else` in `crossplane/crossplane`).
Quoting [Effective Go]:

> In the Go libraries, you'll find that when an if statement doesn't flow into
> the next statement—that is, the body ends in break, continue, goto, or
> return—the unnecessary else is omitted.

### Wrap Errors

Use [`crossplane-runtime/pkg/errors`] to wrap errors with context. This allows
us to emit logs and events with useful, specific errors that can be related to
deeper parts of the codebase without having to actually plumb loggers and event
sources deep down into the codebase. For example:

```go
import "github.com/crossplane/crossplane-runtime/pkg/errors"

func example() error {
        v, err := fetch()
        if err != nil {
                return errors.Wrap(err, "could not fetch the thing")
        }

        store(embiggen(v))
        return nil
}
```

### Scope Errors

Where possible, keep errors as narrowly scoped as possible. This avoids bugs
that can appear due to 'shadowed' errors, i.e. accidental re-use of an existing
`err` variable, as code is refactored over time. Keeping errors scoped to the
error handling conditional block can help protect against this. So for example
instead of:

```go
func example() error {
        err := enable()
        if err != nil {
                return errors.Wrap(err, "could not enable the thing")
        }

        // 'err' still exists here at the function scope.

        return errors.Wrap(emit(), "could not emit the thing")
}
```

We prefer:

```go
func example() error {
        if err := enable(); err != nil {
                // 'err' exists here inside the conditional block.
                return errors.Wrap(err, "could not enable the thing")
        }

        // 'err' does not exist here at the function scope. It's scoped to the
        // above conditional block.

        return errors.Wrap(emit(), "could not emit the thing")
}
```

Note that the 'return early' advice above trumps this rule - it's okay to
declare errors at the function scope if it lets you keep business logic less
nested. That is, instead of:

```go
func example() error {
        if v, err := fetch(); err != nil {
                return errors.Wrap(err, "could not enable the thing")
        } else {
                store(embiggen(v))
        }
        
        return nil
}
```

We prefer:

```go
func example() error {
        v, err := fetch()
        if err != nil {
                return errors.Wrap(err, "could not enable the thing")
        }

        store(embiggen(v))
        return nil
}
```

### Actionable Conditions

Conditions should be actionable for a user of Crossplane. This implies:

1. conditions are made for users, not for developers.
2. conditions should contain enough information to know where to look next.
3. conditions are part of UX.

Conditions have a `type`, a `reason` and a `message`:

- The type is fixed by type, e.g. `Ready` or `Synced`. Keep the number low.
  Uniform condition types across related kinds are preferred.

  `Ready` is common in Crossplane to indicate that a resource is ready to be 
  used by the user. Do not signal `Ready=True` earlier, e.g. do not signal
  a claim as ready before the credential secret has been created and has
  valid and working credentials.
  
- The reason is for machines and uses CamelCase. Reasons should be documented in 
  the API docs.
- The message is for humans and is written in plain English, without newlines,
  and with the first letter capitalized and no trailing punctuation. It might 
  end in an error string, e.g. `Cannot create all resources: foo, bar, and 3 more failed: condiguration.pkg.crossplane.io "foo" is invalid: package.spec is required`.
  Keep the message reasonable short although there is no hard limit. 1000
  characters is probably too long, 100 characters is fine.

Conditions must not flap, including the reason and the message. Make sure that
the reason and message are deterministic and stable. For example, sort in case
of maps as maps iteration is not deterministic in Golang.

Avoid timestamps and in particular relative times in condition messages as
these change on repeated reconciliation. Rule of thumb: if another reconcile
shows the same problem, the condition message must not change.

Transient issues, e.g. apiserver conflict errors like `the object has been modified; please apply your changes to the latest version and try again`
must not be shown in condition messages, but rather the reconciliation should
silently requeue.

### Events when something happens, no events if nothing happens

Events are for users, not for Crossplane developers. Events should matter for
a human.

Events are about changes or actions. If nothing changes or no action happens, do
not emit an event. For example, if no new composition is selected, do not emit an
event. Successful idem-potent actions should only emit an event once. Erroring
actions should emit an event for each error.

Events should aim at telling what has changed and to which value, e.g. 
`Successfully selected composition: eks.clusters.caas.com`, don't omit the
composition name here.

Events should not be used to tell what is going to happen, but what **has**
happened. In reconcile functions with an update at the end, it is fine to emit
an event before the update, in the assumption that the update will succeed.

Transient issues, e.g. apiserver conflict errors like `the object has been modified; please apply your changes to the latest version and try again`
should not be emitted as an event, but rather the reconciliation should silently
requeue.

Events are not a replacements for conditions. As a rule of thumb: the last event
showing a problem should show up as condition message too.

To keep the value for the user up, keep the number of events low. Events are for
humans and humans will read 10, but not 1000 events per object. Emit events
valuable for the user. Use logs instead of events for higher volume information.

Examples for good events:
- `Successfully selected composition: eks.clusters.caas.com` – the message
  is stable, and this is an action (selecting) that succeeded. Hence, it is fine
  to emit one event for it.
- `Readiness probe failed: Get "https://192.168.139.246:8443/readyz": net/http: request canceled (Client.Timeout exceeded while awaiting headers)`
  – the error string is stable, and this is an actions (probing) that failed.
  Hence, it is fine to repeat the event.

Examples for bad events:
- `Applied RBAC ClusterRoles` – it's lacking which ClusterRoles.
- `Bound system ClusterRole to provider ServiceAccount(s)` – it's lacking which 
  ClusterRole, which service accounts and what this cluster role enables.
- `(Re)started composite resource controller` – controllers are not user-facing,
  but just an implementation detail of how APIs are implemented.
- `Update failed: the object has been modified; please apply your changes to the latest version and try again`
  – it's lacking which update failed. Moreover, this is a transient apiserver
  error. The controller should silently requeue instead of emitting an event.

### Prefer Table Driven Tests

As mentioned in [Contributing Code](#contributing-code) Crossplane diverges from
common controller-runtime patterns in that it follows the advice laid out in the
Go project's [test review comments] documents. This means we prefer table driven
tests, and avoid test frameworks like Ginkgo. The most common form of Crossplane
test is as follows:

```go
// Example is the function we're testing.
func Example(ctx context.Context, input string) (int, error) {
        // ...
}

// Test function names are always PascalCase. No underscores.
func TestExample(t *testing.T) {
        type args struct {
                ctx   context.Context
                input string
        }

        type want struct {
                output int
                err    error
        }

        cases := map[string]struct{
                reason string
                args   args
                want   want
        }{
                // The summary is always PascalCase. No spaces, hyphens, or underscores.
                "BriefTestCaseSummary": {
                        reason: "A longer summary of what we're testing - printed if the test fails.",
                        args: args{
                                ctx: context.Background(),
                                input: "some input value",
                        }
                        want: want{
                                output: "the expected output",
                                err: nil,
                        }
                },
        }
        
        for name, tc := range cases {
                t.Run(name, func(t *testing.T) {
                        got, err := Example(tc.args.ctx, tc.args.input)

                        // We prefer to use https://github.com/google/go-cmp/
                        // even for simple comparisons to keep test output
                        // consistent. Some Crossplane specific cmp options can
                        // be found in crossplane-runtime/pkg/test.
                        if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
                                t.Errorf("%s\nExample(...): -want, +got:\n%s", tc.reason, diff)
                        }

                        if diff := cmp.Diff(tc.want.output, got); diff != "" {
                                t.Errorf("%s\nExample(...): -want, +got:\n%s", tc.reason, diff)
                        }
                })
        }
}
```

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

* `make reviewable` - Run code generation, linters, and unit tests.
* `make e2e` - Run end-to-end tests.
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
KUBE_IMAGE=kindest/node:v1.27.1 ./cluster/local/kind.sh up

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

[Slack]: https://slack.crossplane.io/
[code of conduct]: https://github.com/cncf/foundation/blob/master/code-of-conduct.md
[build submodule]: https://github.com/upbound/build/
[`kind`]: https://kind.sigs.k8s.io/
[Crossplane release cycle]: https://docs.crossplane.io/knowledge-base/guides/release-cycle
[good git commit hygiene]: https://www.futurelearn.com/info/blog/telling-stories-with-your-git-history
[Developer Certificate of Origin]: https://github.com/apps/dco
[code review comments]: https://github.com/golang/go/wiki/CodeReviewComments
[test review comments]: https://github.com/golang/go/wiki/TestComments
[E2E readme]: ../test/e2e/README.md
[docs]: https://github.com/crossplane/docs
[Effective Go]: https://golang.org/doc/effective_go
[Observability Developer Guide]: guide-observability.md
[Dave Cheney's blog]: https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis
[`crossplane-runtime/pkg/errors`]: https://pkg.go.dev/github.com/crossplane/crossplane-runtime/pkg/errors
[golangci-lint]: https://golangci-lint.run/
[DCO bot]: https://probot.github.io/apps/dco/
[crossplane-maintainers]: https://github.com/orgs/crossplane/teams/crossplane-maintainers/members
[crossplane-reviewers]: https://github.com/orgs/crossplane/teams/crossplane-reviewers/members
[CODEOWNERS]: ../CODEOWNERS
[Reviewers]: ../OWNERS.md#reviewers
[Maintainers]: ../OWNERS.md#maintainers
