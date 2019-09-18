---
title: FAQs
toc: true
weight: 840
indent: true
---
# Frequently Asked Questions (FAQs)

### Where did the name Crossplane come from?

Crossplane is the fusing of cross-cloud control plane. We wanted to use a noun that refers to the entity responsible for connecting different cloud providers and acts as control plane across them. Cross implies “cross-cloud” and “plane” brings in “control plane”.

### What's up with popsicle?

We believe in a multi-flavor cloud.

### Why is Upbound open sourcing this project? What are Upbound’s monetization plans?

Upbound’s mission is to create a more open cloud-computing platform, with more choice and less lock-in. We believe the Crossplane as an important step towards this vision and that it’s going to take a village to solve this problem. We believe that multicloud control plane is a new category of open source software, and it will ultimately disrupt closed source and proprietary models. Upbound aspires to be a commercial provider of a more open cloud-computing platform.

### What kind of governance model will be used for Crossplane?

Crossplane will be an independent project and we plan on making a community driven project and not a vendor driven project. It will have an independent brand, github organization, and an open governance model. It will not be tied to single organization or individual.

### Will Crossplane be donated to an open source foundation?

We don’t know yet. We are open to doing so but we’d like to revisit this after the project has gotten some end-user community traction.

### Does using multicloud mean you will use the lowest common denominator across clouds?

Not necessarily. There are numerous best of breed cloud offerings that run on multiple clouds. For example, CockroachDB and ElasticSearch are world class implementations of platform software and run well on cloud providers. They compete with managed services offered by a cloud provider. We believe that by having an open control plane for them to integrate with, and providing a common API, CLI and UI for all of these services, that more of these offerings will exist and get first-class experience in the cloud.

### How are resources and claims related to PersistentVolumes in Kubernetes?

We modeled resource claims and classes after PersistentVolumes and PersistentVolumeClaims in Kubernetes. We believe many of the lessons learned from managing volumes in Kubernetes apply to managing resources within cloud providers. One notable exception is that we avoided creating a plugin model within Crossplane.

### How is workload scheduling related to pod scheduling in Kubernetes?

We modeled workload scheduling after the Pod scheduler in Kubernetes. We believe many of the lessons learned from Pod scheduling apply to scheduling workloads across cloud providers.

### Can I use Crossplane to consistently provision and manage multiple Kubernetes clusters?

Crossplane includes a portable API for Kubernetes clusters that will include common configuration including node pools, auto-scalers, taints, admission controllers, etc. These will be applied to the specific implementations within the cloud providers like EKS, GKE and AKS. We see the Kubernetes Cluster API to be something that will be used by administrators and not developers.

### Other attempts at building a higher level API on-top of a multitude of inconsistent lower level APIs have not been successful, will Crossplane not have the same issues?

We agree that building a consistent higher level API on top of multitudes of inconsistent lower level API's is well known to be fraught with peril (e.g. dumbing down to lowest common denominator, or resulting in so loosely defined an API as to be impossible to practically develop real portable applications on top of it).

Crossplane follows a different approach here. The portable API extracts the pieces that are common across all implementations, and from the perspective of the workload. The rest of the implementation details are captured in full fidelity by the admin in resource classes. The combination of the two is what results in full configuration that can be deployed. We believe this to be a reasonable tradeoff that avoids the dumbing down to lowest common denominator problem, while still enabling portability.
