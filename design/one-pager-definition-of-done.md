# Definition of done

* Owner: Daniel Suskin (@suskin)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Introduction

*Why does this document exist?*

The goal of this document is to answer the questions:

* As a developer, when should I consider a chunk of work to be complete?
* As a reviewer, what should I be looking for to verify that a chunk of work is
  complete?

## Definition of work

Work means a contribution to the project, typically code.

## Definition of done

The following criteria **MUST** be met for work to be considered done:

* Code is reviewed and approved.
* Code passes linting (use `make reviewable` to lint).
* Error conditions are reported as and where appropriate. For more about
  how to think about error reporting, see our [philosophical
  guide][error-reporting-philosophy].
* Code is tested to an extent such that: we are confident in its
  correctness; and future maintainers will be able to tell if they have
  introduced a regression.

Additionally, the following criteria **SHOULD** be met for the work to
be considered done, as they are best practices:

* Code is exercised with an integration test (existing or new).
* New behavior is documented in the documentation that will go onto the
  crossplane website.
* Examples have been updated to reflect any changes.

## Applying definition of done

The definition of done can be applied in the following ways:

* Pull requests should not be merged until the work contained in them is
  done. Generally, the best practice would be to include all relevant
  changes (such as code, tests, and documentation) in a single pull
  request, so that it is easier to keep track of.
* Issues tracking work should not be closed until the work being tracked
  is done.

<!-- Reference links -->
[error-reporting-philosophy]: https://github.com/crossplane/crossplane/blob/master/design/one-pager-error-and-event-reporting.md
