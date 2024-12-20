# Crossplane Security Self-Assessment

November 2024

* **Primary Author**: Jared Watts
* **Reviewers**: Nic Cope, Hasan Turken, Bob Haddleton, Philippe Scorsolini

## Metadata

|                   |                                                                                                                                           |
|-------------------|-------------------------------------------------------------------------------------------------------------------------------------------|
| Assessment Stage  | Incomplete                                                                                                                                |
| Software          | https://github.com/crossplane/crossplane                                                                                                  |
| Security Provider | No                                                                                                                                        |
| Languages         | Go                                                                                                                                        |
| SBOM              | A SBOM for the Crossplane v1.18.0 release has been generated manually at https://gist.github.com/jbw976/faddf69437451e1dd289545749738771. |

### Relevant Links

| Doc                                                 | URL                                                                                   |
|-----------------------------------------------------|---------------------------------------------------------------------------------------|
| Security Audit by Ada Logics, 2023                  | https://github.com/crossplane/crossplane/blob/main/security/ADA-security-audit-23.pdf |
| Fuzzing Audit by Ada Logics, 2022                   | https://github.com/crossplane/crossplane/blob/main/security/ADA-fuzzing-audit-22.pdf  |
| Security policy and vulnerability reporting process | https://github.com/crossplane/crossplane/blob/main/SECURITY.md                        |
| Security Advisories                                 | https://github.com/crossplane/crossplane/security/advisories                          |
| Graduation Issue                                    | https://github.com/cncf/toc/issues/1397                                               |

## Overview

[Crossplane](https://crossplane.io/) extends the Kubernetes API to enable
platform teams to manage a wide variety of infrastructure resources from
multiple vendors. These resources can be composed into higher level self-service
APIs for application teams to consume. Crossplane effectively enables platform
teams to quickly put together their own opinionated platform declaratively
without having to write any code, and offer it to their application teams as a
self-service Kubernetes-style declarative API.

### Background

Crossplane is a popular tool across the Cloud Native landscape for Platform
Engineers to build Internal Developer Platforms (IDPs) and manage infrastructure
resources. The core of Crossplane's runtime and composition machinery runs as a
set of controllers in the main Crossplane pod. These controllers reconcile the
API objects of the Crossplane resource model, facilitating Platform Engineers to
compose infrastructure resources from multiple vendors together and expose them
as higher level platform APIs for application teams to consume.

Crossplane has a rich extension model that enables its users to build platforms
to manage basically any resources that expose an API. The direct reconciliation
of these infrastructure resources with external entities such as cloud providers
occurs within
[Providers](https://docs.crossplane.io/latest/concepts/providers/), one of the
main extension points of Crossplane. Within the vibrant Provider ecosystem,
there exist Providers for AWS, GCP, Azure and a plethora of other environments.
Each of these Providers runs as an isolated pod, separate from the main
Crossplane pod.

Crossplane also has a package manager to install and manage these extensions within
the control plane. These packages are distributed as generic OCI images, which
contain YAML content informing the Crossplane package manager how to alter the
state of a cluster by installing objects that configure new resource types, and then
starting controllers to reconcile them. Details about the contents of these
packages can be found in the [`xpkg`
Specification](https://github.com/crossplane/crossplane/blob/main/contributing/specifications/xpkg.md).

The core Crossplane and composition machinery, the Crossplane runtime, and its
package manager are all in scope for this security assessment. Extensions to
Crossplane, such as
[Providers](https://docs.crossplane.io/latest/concepts/providers/) and
[Functions](https://docs.crossplane.io/latest/concepts/compositions/) are
considered out of scope.

### Actors

<table>
<thead>
  <tr>
    <th>Actor</th>
    <th>Description</th>
  </tr>
</thead>
<tbody>
  <tr>
    <td>Composition engine</td>
    <td>The set of controllers that reconcile Crossplane's composition model, e.g. Claims, <code>CompositeResources</code> (XRs), <code>CompositeResourceDefinitions</code> (XRDs), and <code>Compositions</code>.
        <ul>
          <li><code>XRD</code>: When a new <code>XRD</code> is created by a platform engineer, this controller reconciles this desired state of this definition to dynamically create a new platform API in the form of a CRD that extends the Kubernetes API.</li>
          <li><code>XR</code>: This controller reconciles instances of cluster scoped composite resources, executing the composition logic authored by the platform team to dynamically compose and configure a set of granular resources.</li>
          <li><code>Claim</code>: This controller reconciles the namespaced abstractions that application developers use to request infrastructure resources.</li>
          <li><code>Composition</code>: Contains the "blueprint" logic for how to compose resources together. The platform engineer defines a simple pipeline of functions to execute, with the end goal being to defined and customized the composed resources.
         </ul>
    </td>
  </tr>
  <tr>
    <td>RBAC manager</td>
    <td> The RBAC manager enables Crossplane to dynamically manage (and in some cases bind) RBAC roles that grant subjects access to use Crossplane as it is extended by installing providers and defining composite resources.
        Crossplane has a dynamic and user extensible type system, and this component essentially initializes newly created types with appropriate RBAC permissions to allow controllers to reconcile them and/or end user personas to access them.
    </td>
  </tr>
  <tr>
    <td>Package manager</td>
    </td>
    <td>The package manager is responsible for installing and managing extensions to Crossplane.
        It connects to OCI registries to download packages, applies their resources (CRDs) to the control plane, and starts a <code>Deployment</code> workload to run the package's controllers that reconcile these new resources.
  </tr>
  <tr>
    <td>Webhooks</td>
    <td>A set of webhooks that perform schema validation on dynamically created resources within Crossplane's composition model. These webhooks are optional and can be disabled by the user.
    </td>
  </tr>
</tbody>
</table>

### Actions

<table>
<thead>
  <tr>
    <th>Action</th>
    <th>Actor</th>
    <th>Description</th>
  </tr>
</thead>
<tbody>
  <tr>
    <td>Package installed to extend Crossplane's API</td>
    <td>Platform engineer, package manager, RBAC manager</td>
    <td>A platform engineer creates a cluster scoped <code>Provider</code>, <code>Function</code>, or <code>Configuration</code> object that points to a Crossplane package, i.e. an OCI image such as <code>xpkg.upbound.io/crossplane-contrib/provider-argocd:v0.9.1</code>.
        The Package Manager's controllers reconcile this desired state by downloading the OCI image from the given registry.
        The contents of the image are parsed and all objects of allowed types (e.g. CRDs, XRDs, Compositions, etc.) are applied to the API server.
        A <code>Deployment</code> manifest is constructed to run the OCI image as a workload in the control plane, and then applied to the API server.
        The RBAC manager process the new types defined in the package and creates RBAC permissions for the Provider's and Crossplane's service account to reconcile them and platform engineers/consumers to access them.</td>
  </tr>
  <tr>
    <td><code>CompositeResourceDefinition</code> (XRD) created</td>
    <td>Platform Engineer, composition engine, RBAC manager</td>
    <td>A platform engineer intends to add a new API to their platform. They start by creating a <code>XRD</code> instance to define the schema of the new API.
        A validating webhook is invoked by the API server and it performs validation on the values of the <code>XRD</code> according to the built-in defined schema.
        If accepted, the <code>XRD</code> controller reconciles the new instance and creates new CRDs to extend the Kubernetes API with the new abstraction that the <code>XRD</code> represents.
        A controller is also created that will reconcile instances of the new CRDs.
        The RBAC manager process these new CRDs and creates RBAC permissions for Crossplane's service account to reconcile them and platform engineers/consumers to access them.</td>
  </tr>
  <tr>
    <td><code>CompositeResource</code> (XR) or <code>Claim</code> created</td>
    <td>Platform Engineer (<code>XR</code>) or Application Developer (<code>Claim</code>), composition engine</td>
    <td>When a composite resource or claim object is created, Crossplane's dynamic controllers (creation of these was described in previous actions) respond to reconcile them.
        The corresponding/selected <code>Composition</code> is processed and its defined function pipeline logic is executed to render a set of composed resources.
        These composed resources are then applied to the API server, which will cause corresponding controllers to reconcile them in turn.</td>
  </tr>
</tbody>
</table>

### Goals

The main goal of Crossplane is to provision and manage infrastructure resources
on behalf of a centralized platform team, through the exposure and usage of
simplified abstraction APIs to the application developer teams that the platform
team services.

Crossplane intends to support a strong separation of concerns for these personas
and the infrastructure APIs in between. Crossplane creates these APIs (via CRDs)
on demand for the platform team, and then appropriately applies RBAC permissions
to secure their access if requested. This RBAC management feature is on by
default in Crossplane, but can be turned off if the platform team wants to
manually manage the permissions themselves.

Crossplane also intends to support an extension model for the platform team to
dynamically extend their control planes with support for new cloud providers and
environments. Crossplane's package manager handles the lifecycle management of
these extensions and intends to ensure that only the expected/allowed package
content is installed into the control plane, such as new CRDs and controllers to
reconcile them.

### Non-goals

Crossplane does not intend to provide further granular access control to the
infrastructure primitives that the platform team manages. These resources are
cluster scoped and Crossplane does not have built-in mechanisms or experiences
to further restrict their access. This can be configured outside of Crossplane
via manual RBAC or policy configurations.

Crossplane does not intend to exhaustively restrict the controller workloads
that are run as extensions in the control plane. Users have the ability to
configure the `Deployment` that manages the extension's pod (and therefore
internal controllers), but the specific execution and runtime of the extension
is not restricted further by Crossplane.


## Self-assessment Use

This self-assessment is created by the Crossplane team to perform an internal
analysis of the project’s security. It is not intended to provide a security
audit of Crossplane, or function as an independent assessment or attestation of
Crossplane’s security health.

This document serves to provide Crossplane users with

* an initial understanding of Crossplane’s security
* where to find existing security documentation
* Crossplane plans for security
* a general overview of Crossplane security practices, both for development of
  Crossplane as well as security of Crossplane.

This document provides the CNCF TAG-Security with an initial understanding of
Crossplane to assist in a joint-assessment, necessary for projects under
incubation. Taken together, this document and the joint-assessment serve as a
cornerstone for if and when Crossplane seeks graduation and is preparing for a
security audit.

## Security functions and features

**Critical Security Components**

* RBAC manager: This component dynamically manages (and in some cases bind) RBAC
  roles that grant subjects access to use Crossplane as it is extended by
  installing providers and defining composite resources. The most important
  aspect of the RBAC manager related to security is that while it runs as an
  isolated pod, separate from core Crossplane and other extensions, and with a
  service account limited to a specific set of resources and verbs, it is also
  granted the special `escalate` and `bind` verbs on the `Role` and
  `ClusterRole` resources, thus allowing it to grant access that it does not
  have. This component is optional and can be completely disabled for platform
  operators that want to manually manage the permissions for dynamically created
  resources. More details about how the RBAC manager works can be found in the
  [design
  doc](https://github.com/crossplane/crossplane/blob/main/design/design-doc-rbac-manager.md).
* Package manager: This component is responsible for installing and managing
  extensions to Crossplane. It connects to remote OCI registries to download
  arbitrary Crossplane packages (opinionated OCI images), parse their contents,
  and apply the resources contained within to the control plane. Because this
  component introduces new resources and controllers to the control plane from a
  remote source, it is noteworthy from a security perspective. Its code paths
  and functionality were a significant focus for the [third party security
  audit](https://github.com/crossplane/crossplane/blob/main/security/ADA-security-audit-23.pdf).
  The package manager [design
  doc](https://github.com/crossplane/crossplane/blob/main/design/design-doc-packages-v2.md)
  and `xpkg` package format
  [specification](https://github.com/crossplane/crossplane/blob/main/contributing/specifications/xpkg.md)
  are also useful resources.

**Security Relevant Security Components**

* Composition engine: This set of controllers reconciles Crossplane's
  composition model, e.g. `Compositions`, `CompositeResources` (XRs), and
  `CompositeResourceDefinitions`(XRDs). They take user provided input to define
  new abstractions (platform APIs) as well as how to generate and compose resources
  together. The general approach for the logic contained in a `Composition` is
  that the platform engineer defines a simple [pipeline of
  functions](https://docs.crossplane.io/latest/concepts/compositions/) to
  execute. At the end of this pipeline will be a set of resources that
  Crossplane then applies to the API server on behalf of the platform engineer.
  Since this component contains and executes simple user defined logic, it is a
  security relevant component. The Functions [design
  doc](https://github.com/crossplane/crossplane/blob/main/design/design-doc-composition-functions.md)
  and the Functions
  [specification](https://github.com/crossplane/crossplane/blob/main/contributing/specifications/functions.md)
  are useful related resources.
* Webhooks: Crossplane has a set of validating admission webhooks enabled by
  default that perform validation on user provided resources in the composition
  model, such as `XRDs` and `Compositions`. The provided objects are validated
  according to their schema and invalid objects will be rejected from the API
  server. These webhooks are optional and can be disabled.

## Project Compliance

The Crossplane project is not documented to comply with any specific security
standards or sub-sections at this time.

## Secure Development Practices

### Development Pipeline

* All PRs on the Crossplane project require at least one approval from a code
  owner or maintainer
* A DCO sign-off is required for all commits, but cryptographically signed
  commits are not currently required
* A CI/CD pipeline with extensive checks runs on all PRs, `main` branch, and
  release branches. The checks performed by this
  [pipeline](https://github.com/crossplane/crossplane/blob/main/.github/workflows/ci.yml)
  include:
  * A E2E test suite to validate all major scenarios and features
  * A unit test suite to validate all unit level functionality
  * `golangci-lint` runs with all linters turned on by default (`enable-all:
    true`), and a handful disabled with justifications provided. The
    [`gosec`](https://github.com/securego/gosec) linter is included to inspect
    the source code for security problems.
  * A CodeQL scan performs static analysis to identify potential vulnerabilities
  * The Trivy vulnerability scanner is run over the codebase in `fs` mode
  * Fuzz testing is performed on the codebase with multiple test cases developed
    during the [Fuzzing
    audit](https://github.com/crossplane/crossplane/blob/main/security/ADA-fuzzing-audit-22.pdf)
* The `crossplane/crossplane` image is based on the `distroless/static` base
  image to minimize surface area of the container image

### Communication Channels

* Internal: Most communication amongst the team occurs in the [Crossplane
  Slack](https://slack.crossplane.io/) dedicated workspace (in particular the
  `#core-development` channel), as well as the Github
  [`crossplane`](https://github.com/crossplane) and
  [`crossplane-contrib`](https://github.com/crossplane-contrib) organizations on
  issues, PRs, and discussions.
* Inbound: Communication from users of Crossplane most commonly occurs on the
  Crossplane Slack `#general` channel, in regular Community meetings, and other
  channels outlined in the main
  [README](https://github.com/crossplane/crossplane?tab=readme-ov-file#get-involved).
* Outbound: The project makes announcements on the Crossplane Slack
  `#announcements` channel, [Bluesky](https://bsky.app/profile/crossplane.io),
  [Twitter](https://twitter.com/crossplane_io),
  [LinkedIn](https://www.linkedin.com/company/crossplane), [blog
  posts](https://blog.crossplane.io/), and [release
  notes](https://github.com/crossplane/crossplane/releases).

### Ecosystem

Crossplane extends Kubernetes to enable cloud native control planes, so currently
Crossplane will always be running within a Kubernetes cluster. Crossplane is
very often integrated with a GitOps system such as ArgoCD or Flux.

The [Graduation application](https://github.com/cncf/toc/issues/1397) covers
many other integrations and collaborations with the cloud native ecosystem, for
example Helm, gRPC, Prometheus, Backstage, Dapr, and others.

## Security Issue Resolution

### Responsible Disclosures Process

The full process for reporting security vulnerabilities is defined in the
[security
policy](https://github.com/crossplane/crossplane/blob/main/SECURITY.md) page.

Anyone from the ecosystem can report a vulnerability privately using the GitHub
security vulnerability integration, or by emailing
crossplane-security@lists.cncf.io.

#### Vulnerability Response Process

As outlined in the vulnerability reporting process, the reporter(s) can
typically expect a response from the maintainer team within 24 hours
acknowledging the issue was received. If a response is not received within 24
hours, they are then asked to reach out to any maintainer directly to confirm
receipt of the issue.

### Incident Response

The full details of the incident response process are outlined in the
[vulnerability reporting
process](https://github.com/crossplane/crossplane/blob/main/SECURITY.md), but a
brief summary is provided here:

* When a vulnerability is initially reported, the core maintainers review it to
  triage and assess the severity of the issue
* A draft security advisory is created on GitHub to collaborate with the
  reporter(s)
* A fix is developed and tested, typically on a private fork, and then merged
  into the main/release branches
* A patch release is built and published for the affected versions
* The security advisory is then published to GitHub and announced to
  crossplane-security-announce@lists.cncf.io

## Appendix

### Known Issues Over Time

All vulnerabilities and security advisories are published publicly to the
[security center](https://github.com/crossplane/crossplane/security/advisories)
of the core crossplane repository.

To date, 4 security advisories have been published. 3 are the result of
security/fuzzing audits, and 1 is a result of a community reported
vulnerability in the version of Go that Crossplane depended on.

### [OpenSSF Best Practices](https://www.bestpractices.dev/en)

Crossplane current has a passing OpenSSF Best Practices badge: [![OpenSSF Best
Practices](https://www.bestpractices.dev/projects/3260/badge)](https://www.bestpractices.dev/projects/3260)

### Case Studies

There are a few common use cases for the cloud native control planes that
Crossplane enables.

* **Internal developer platform (IDP)**: Crossplane is often used to build an IDP
  that enables platform teams to offer infrastructure resources to application
  teams in a self-service manner. This allows the platform team to define
  opinionated abstractions with golden paths for their application teams to
  consume, and then the control plane manages the underlying infrastructure
  resources on their behalf. A wide variety of infrastructure resources are made
  available to application teams through an IDP, such as databases, buckets,
  caches, queues, networking, etc.
* **Cluster as a Service ([CaaS](https://blog.upbound.io/cluster-as-a-service))**:
  Crossplane is very often used to expose self-service Clusters on demand for
  application developers to request and consume for their workloads. Developers
  can focus on writing code and building applications without the need to worry
  about the underlying cluster setup, scaling, or maintenance. This abstraction
  reduces the learning curve for developers, allowing them to quickly leverage
  the power of Kubernetes for container orchestration without becoming
  Kubernetes experts.

Brief descriptions of the use cases for the many adopters of Crossplane are
described in the [public adopters
list](https://github.com/crossplane/crossplane/blob/main/ADOPTERS.md).

A few Crossplane community members have taken the time to outline their usage of
Crossplane in more details on the Crossplane blog:

* [How Imagine Learning is Building Crossplane Composition Functions to Empower
  Your Control
  Plane](https://blog.crossplane.io/building-crossplane-composition-functions-to-empower-your-control-plane/)
* [How VSHN uses Composition Functions in
  Production](https://blog.crossplane.io/composition-functions-in-production/)

Further cases studies can be found on the [CNCF
website](https://www.cncf.io/case-studies/?_sft_lf-project=crossplane) and
[Upbound's website](https://www.upbound.io/resources/case-studies).

### Related Projects and Vendors

Terraform is far and away the most common project that Crossplane is compared
to. Some key differences between the projects include:

* Crossplane takes a control plane approach (based on Kubernetes) that is always
  actively reconciling resources, which quickly identifies and corrects any
  configuration drift.
* Crossplane is API driven and allows sets of resources to be abstracted and
  exposed as self-service APIs that integrate with an entire ecosystem of tools
  and solutions.
* Crossplane manages individual resources at a granular level, whereas Terraform
  typically manages an entire workspace of resources as a single unit.
* More details about the comparison of Crossplane and Terraform can be found in
  this [blog post](https://blog.crossplane.io/crossplane-vs-terraform/).

Cluster API is sometimes compared to Crossplane as they can both be used to
provision and manage Kubernetes clusters and their underlying infrastructure.
However, Cluster API takes an exclusive focus on the Kubernetes cluster
provisioning use case, while Crossplane has a much more broad focus on managing
infrastructure in general and building general usage cloud native platforms.

The hyperscalers also offer projects that expose their resources as extensions
to the Kubernetes API, such as AWS Controllers for Kubernetes (ACK) and Google
Config Connector. While these projects inherently take a narrow focus of
exposing their own service offerings, Crossplane takes a much broader focus of
intending to be a universal control plane for anything with an API. More details
that compare Crossplane to Cloud Provider Infrastructure add-ons can be found in
this [blog
post](https://blog.crossplane.io/crossplane-vs-cloud-infrastructure-addons/).

Radius is a project that probably has more overlap than was indicated in
[sandbox#65](https://github.com/cncf/sandbox/issues/65) as both projects are
focusing on abstractions for developer self-service. However, Radius focuses
more on an application layer model with a narrow scope of Recipes that do not
appear to be user definable. Crossplane takes more of an infrastructure level
focus and is very flexible, allowing the platform team to completely define
their own custom abstractions and implementations.