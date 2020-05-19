---
title: FAQ
toc: true
weight: 1200
---

# Frequently Asked Questions (FAQs)

### Where did the name Crossplane come from?

Crossplane is the fusing of cross-cloud control plane. We wanted to use a noun
that refers to the entity responsible for connecting to different cloud providers
and acting as control plane across them. Cross implies “cross-cloud” and “plane”
brings in “control plane”.

### What's up with popsicle?

We believe in a multi-flavor cloud.

### Why is Upbound open sourcing this project? What are Upbound’s monetization plans?

Upbound’s mission is to create a more open cloud-computing platform, with more
choice and less lock-in. We believe that Crossplane as an important step towards
this vision and that it’s going to take a village to solve this problem. We
believe that control plane is a new category of open source software,
and it will ultimately disrupt closed source and proprietary models. Upbound
aspires to be a commercial provider of a more open cloud-computing platform.

### What kind of governance model will be used for Crossplane?

Crossplane will be an independent project and we plan on making Crossplane a
community driven project and not a vendor driven project. It will have an
independent brand, github organization, and an open governance model. It will
not be tied to single organization or individual.

### Will Crossplane be donated to an open source foundation?

We don’t know yet. We are open to doing so but we’d like to revisit this after
the project has gotten some end-user community traction.

### Does supporting multiple Providers mean you will impose the lowest common denominator across clouds?

Not at all. Crossplane supports defining, composing, and publishing your own
infrastructure resources to the Kubernetes API that can be composed using the
infrastructure primitives in each Provider.  We believe that by having an open
control plane for all cloud providers to integrate with using a common API,
CLI and idomatic Kubernetes experience will make it easier for everyone to 
define infrastructure abstractions that make sense for their organizations.

### How are resources and claims related to PersistentVolumes in Kubernetes?

Crossplane originally modeled resource claims and classes after
PersistentVolumes and PersistentVolumeClaims in Kubernetes. We believe many of
the lessons learned from managing volumes in Kubernetes apply to managing
resources within cloud providers. The separation of concerns afforded by claims
and classes has been moved forward and enhanced with the ability to define,
compose, and publish your own infrastructure resources in a no-code way.
They're not called claims and classes anymore, but the team-centered approach
is intact and improved, and infrastructure operators have more control over the
shape of the abstractions they provide for app operators to consume.

### Other attempts at building a higher level API on-top of a multitude of inconsistent lower level APIs have not been successful, will Crossplane not have the same issues?

We agree that building a consistent higher level API on top of multitudes of
inconsistent lower level API's is well known to be fraught with peril (e.g.
dumbing down to lowest common denominator, or resulting in so loosely defined an
API as to be impossible to practically develop real portable applications on top
of it).

That is why Crossplane offers the ability to define, compose, and publish
your own infrastructure APIs in Kubernetes, so you can choose the right mix of 
portable vs. cloud-specific abstractions for application teams to use. 

With Crossplane you can choose how much or little of the underlying
infrastructure configuration to export, to hide infrastructure complexity and
include policy guardrails, so applications can easily and safely consume the
infrastructure they need, using any tool that works with the Kubernetes API.

### Related Projects
See [Related Projects].

[Related Projects]: related_projects.md
