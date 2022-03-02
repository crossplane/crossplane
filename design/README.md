# Crossplane Design Documents

This directory contains all documents informing Crossplane's design. Crossplane
designs must transition through up to three of the following phases:

1. Proposals. A proposal is simply a Github issue against this repository with
   the [`proposal` label][proposal-label]. Proposals need not be more than two
   to three paragraphs.
2. One-pagers. A One-pager is a brief document pitching an idea for further
   experimentation. One-pagers, as the name suggests, are usually around one
   page long. They provide just enough context to achieve buy-in from Crossplane
   maintainers.
3. Design documents. A design document is a longer, more detailed design. Design
   documents should typically be preceded by a one-pager and/or a good amount
   of research and experimentation.

All designs must start as a proposal. In some simple cases this proposal alone
is sufficient to move forward with a design. As the complexity or controversy of
the proposed design increases a one-pager and/or design document may be
required. Please name your documents appropriately when committing to this
directory, i.e. `one-pager-my-cool-design.md` or `design-doc-my-cool-design.md`.

All documents committed to this directory _must_ include the following header:

```markdown
# Document Title
* Owner: Some Person (@GithubUsername)
* Reviewers: Crossplane Maintainers
* Status: Accepted, revision 1.0
```

The __document owner__ is the person responsible for stewarding its lifecycle.
The owner will typically be the original document author, though ownership may
be handed over to another individual over time. The owner may choose to include
their email address, but doing so is not mandatory.

The __document reviewers__ are a small, targeted audience. Feedback is _always_
welcome from any member of the Crossplane community, but feedback from the
elected reviewers carries extra weight.

The __document status__ reflects the lifecycle of the design. Designs may be
committed to master at any stage in their lifecycle as long as the status is
indicated clearly. Use one of the following statuses:

* _Speculative_ designs explore an idea without _yet_ explicitly proposing a
  change be made. Typically only one-pagers will be speculative.
* _Draft_ designs strive toward acceptance. Designs may exist in draft for
  some time as we experiment and learn, but their ultimate goal is to become
  an _accepted_ design.
* _Accepted_ designs reflect the current or impending state of the codebase.
  Documents may transition from _draft_ to _accepted_ only after receiving
  approval from a majority of the document's elected reviewers. Once a
  document reaches _accepted_ status it should also include a major.minor
  style abbreviated semantic version number reflecting substantial updates to
  the design over time.
* _Defunct_ designs are kept for historical reasons, but do not reflect the
  current or impending state of the codebase either because they were never
  accepted or because they were accepted but are no longer reflective of the
  state of the codebase.

Note that in some cases designs are "semi defunct". These designs are deemed
relevant enough to keep in the "accepted" state rather than marking them
completely defunct, but may:

* Be only partially implemented.
* Reference defunct concepts, like resource classes or Crossplane workloads.
* Have iterated either conceptually, API wise, or code wise since initial
  implementation.

The current convention for such documents is to leave them in the "Accepted"
state, but add a prominent note at the top of the document explaining the way(s)
in which they are no longer accurate.

[proposal-label]: https://github.com/crossplane/crossplane/issues?utf8=%E2%9C%93&q=is%3Aissue+label%3Aproposal