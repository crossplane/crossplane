# Project Charter

## Overview

Crossplane is a framework for building cloud native control planes without
needing to write code. It has a highly extensible backend that enables you to
build a control plane that can orchestrate applications and infrastructure no
matter where they run, and a highly configurable frontend that puts you in
control of the schema of the declarative API it offers.

## Control Planes

Control planes first emerged in network packet switching, where devices must
forward packets as fast as possible, but also be highly configurable. Breaking
out the control plane from the data plane allows the former to be optimized for
flexibility while the latter is optimized for speed. It also decouples the
failure domains - a control plane failure does not prevent the data plane from
functioning. Control planes are rooted in control theory, in particular closed
loop control where the actual state of a system is observed in order to
determine how best to drive it toward a desired state.

Control planes power cloud computing. Cloud providers like AWS are built with
control planes, as are projects like Kubernetes. In cloud computing the data
plane is instead a “compute plane” consisting of applications and the
infrastructure they run on - VMs, containers, databases, caches, queues, etc.

Control planes are popular in cloud computing because they:

* Are “always-on”, constantly correcting drift by driving observed state toward
  desired state.
* Are API-driven, which enables discovery, collaboration, and integration with
  other systems.
* Don’t need to be installed on every laptop and CI/CD system, making them
  easier to operate.
* Keep control in a separate failure domain from the plane under control (e.g.
  compute).

Cloud native organizations increasingly prefer control planes to approaches such
as imperative scripts or Infrastructure-as-Code (IaC), but historically building
a control plane has required a significant investment to design and code it from
scratch. This is where Crossplane comes in.

## What Crossplane Is

Crossplane is a neutral place for vendors and individuals to come together in
enabling control planes. It offers a framework to build control planes for cloud
native computing without needing to write code (unless you want to). It shares a
foundation with Kubernetes, but supports many powerful extensions that enable it
to control anything - not just containers. The Crossplane project consists of:

* The Crossplane Resource Model (XRM). An API model for Crossplane extensions.
* The ability to compose bespoke control plane APIs for any compute plane.
* A package format and manager to extend your control plane with:
  * The ability to orchestrate any kind of compute plane resource.
  * New ways to compose control plane APIs.
  * New integration with supporting services such as secret stores.
* A set of providers and other integrations to cloud and infrastructure
  offerings that enable resources to be provisioned and managed.
* Tooling to create and test new Crossplane extensions.
* A conformance program for Crossplane extensions.
* Tooling, interfaces, and experiences that facilitate building, managing, and
  consuming a control plane.

As a framework, Crossplane:

* Avoids building features specific to any particular compute plane in its core.
  Instead features pertaining to any particular cloud provider, project, or tool
  are enabled through extension points.
* Focuses on APIs. Crossplane provides and recommends “golden path” tools to
  build conformant extensions but does not require them. Anything with
  conformant API schema and behavior may be considered a Crossplane extension -
  regardless of how it is built. 
* Optimizes for “vertical integration”. That is, Crossplane extensions are
  intended and designed to be used in conjunction with Crossplane and each
  other.
* Prioritizes enabling a “separation of concerns” between control plane curators
  and consumers. Curators deploy, extend, and configure control planes, while
  consumers use them to orchestrate compute planes.

## What Crossplane Is Not

Crossplane is not intended to be purpose built for any particular use case.
Instead it is a general, but complete, framework that facilitates building,
deploying, configuring, and managing a control plane for your own highly bespoke
use case, often without writing any code.

The project is a neutral place for vendors and individuals to come together in
enabling control planes. While any extension that passes the appropriate
Crossplane conformance suite may consider itself to be
“a Crossplane extension” - regardless of how it is built, licensed, or governed,
who maintains it, or where it is hosted - the project maintainers can endorse
a curated set of extensions to drive a high quality experience around them.

Concrete examples of things that are not in scope for the project include (but
are not limited to):

* Providing or mandating abstractions around specific use cases such as
  provisioning applications (e.g. a PaaS API) or provisioning Kubernetes
  clusters. 
* Providing or mandating a configuration language (for example, CUE). Crossplane
  provides a declarative API that may be targeted by arbitrary configuration
  languages, and extension points that may be powered by arbitrary languages.
* Enabling the use of Crossplane extensions without using Crossplane and vice
  versa; this is not supported and may result in undefined behavior.
* Providing or mandating opinions around control plane hosting, security and
  tenancy models, or multiple control plane management.
* Offering ‘in-tree’ support for particular compute planes - e.g. AWS. All
  features specific to any specific compute plane provider are built as
  extensions.
* Offering a package specification or manager for anything other than Crossplane
  extensions.

## Updating this Document

This is a living document. Changes to the scope, principles, or mission
statement of the Crossplane project require a [majority vote][sc-voting] of the
steering committee.

[sc-voting]: https://github.com/crossplane/crossplane/blob/main/GOVERNANCE.md#updating-the-governance
