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

* Code is reviewed and approved
* Code passes linting (use `make reviewable` to lint)
* All error conditions are reported into the log or as an event, as
  appropriate
* Code is tested

Additionally, the following criteria **SHOULD** be met for the work to
be considered done, as they are best practices:

* Code is tested with an integration test
* New behavior is documented in the documentation that will go onto the
  crossplane website
* Examples have been updated to reflect any changes

## Applying definition of done

The definition of done can be applied in the following ways:

* Pull requests should not be merged until the work contained in them is
  done.
* Issues tracking work should not be closed until the work being tracked
  is done.
