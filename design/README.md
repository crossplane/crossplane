# Crossplane Design Documents
This directory contains all documents informing Crossplane's design. Crossplane
design documents can be divided into two two kinds:

* One-pagers. A One-pager is a brief document pitching an idea for further
  experimentation. One-pagers, as the name suggests, are usually around one
  page long. They provide just enough context to achieve buy-in from Crossplane
  maintainers.
* Design documents. A design document is a longer, more detailed design. Design
  documents should typically be preceeded by a one-pager and/or a good amount of
  research and experimentation.

In some cases a one-pager is sufficient to move forward with a design. Complex
or controversial designs will require a more detailed design document. Please
name your documents appropriately when committing to this directory, i.e.
`one-pager-my-cool-design.md` or `design-doc-my-cool-design.md`.

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