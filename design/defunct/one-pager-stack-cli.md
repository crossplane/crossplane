# Crossplane Stacks CLI
* Owner: Daniel Suskin (@suskin)
* Reviewers: Crossplane Maintainers
* Status: Defunct

## Introduction
*Why does this document exist?*

We're working on the first version of a user experience for working with
crossplane stacks. This document answers the questions of:
* What are our goals for this version?
* How do we measure success?
* What do we think the experience will look like?

## OKRs
*What are our goals for this version? How do we measure success?*

Objectives:
* Provide a complete and simple experience for creating stacks from scratch (for a single pattern of creating stacks)
* The stacks experience can be updated independently of the Crossplane releases

Key Results:
* Creating a Hello World stack from scratch takes 5 minutes, using the recommended approach
* Creating and publishing a Hello World stack from scratch takes 10 minutes, using the recommended approach
* All documentation linking from Crossplane 0.3 to the stacks experience will not need to be changed if we update the stacks experience

## Experience
*What do we think the experience will look like?*

To create a Hello World stack from scratch:
* Have access to a Crossplane
* Install kubebuilder v2 and its prerequisites
  - go
  - docker
  - kustomize (optional)
* Create a kubebuilder v2 project
* Add a "Hello World" log line to the kubebuilder controller
* Install kubectl plugins
  - `curl -s -o /usr/local/bin/kubectl-crossplane-stack-init /url/directly/to/the/script/somewhere >/dev/null`
  - More curls until all the scripts are downloaded
  - `chmod +x /usr/local/bin/kubectl-crossplane-*`
  - This could be wrapped into a script so it could be a one-liner if
    the block of commands gets too long.
* Navigate to the project directory
  - `cd myproject`
* Run init
  - `kubectl crossplane stack init mystackgroup/mystackname`
* Run build
  - `kubectl crossplane stack build`
* Run publish
  - (docker registry credentials should be set up before publishing)
  - `kubectl crossplane stack publish`
* Install
  - `kubectl crossplane stack install mystackgroup/mystackname`, OR
  - `kubectl apply -f config/stack/install.stack.yaml`
* Validate
  - `kubectl apply -f config/samples/*instance.yaml`
  - `kubectl logs mystackname-pod-name`

## FAQ
### How does configuration of the stack tool work?
The tool will make assumptions based on a single supported pattern for
now (we are imagining it as Kubebuilder v2), but that will be
configurable in the future. When it is configurable, it will also be
adjustable at init time.

### Should the stack development tool be a kubectl plugin?
Unclear, because it isn't interacting with any Kubernetes objects.
However, it makes sense to have it as a kubectl plugin for now so that
we can take advantage of kubectl's discoverability mechanism and
subcommand interpretation.

### Is the stack tool going to look different in the future?
Most likely. This is a very early imagining of it, and we will learn a
lot as we develop it, use it, and hear stories from the wild.

### Where are the plugins curled from?
They will be curled directly from the raw files in the [stack cli github
repo][stack cli github].

### How is plugin versioning handled?
Curling can be done against a tag or branch, so we can create tags for
each release, and update the documentation to reflect that.

### What if I want an even simpler and faster Hello World experience?
We can have a repo which is a complete sample hello world stack, and
which is periodically recreated from scratch using all of the init
steps.

### How do I tag my stack release with a version or other things?
The `init` command generates all of the scaffolding for building the
stack, including any tags or version numbers. The build configuration
can be edited to tag any published images as desired.

### For publish, how are registry credentials handled?
They will be handled in the same way that docker registry credentials
are handled; they should be set up before running the `publish` command.

### Could this be a single plugin instead of multiple?
Yes, it could be a single plugin instead of multiple. For this version,
the benefit of having multiple plugins is that the project can leverage
the subcommand dispatch functionality that kubectl provides, and that
saves some time. In the future, it may very well be a single plugin
instead.

[stack cli github]: https://github.com/crossplane/crossplane-cli
