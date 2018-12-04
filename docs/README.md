# Crossplane

Crossplane is an open source multicloud control plane. It introduces workload and resource abstractions on-top of existing managed services that enables a high degree of workload portability across cloud providers. A single crossplane enables the provisioning and full-lifecycle management of services and infrastructure across a wide range of providers, offerings, vendors, regions, and clusters. Crossplane offers a universal API for cloud computing, a workload scheduler, and a set of smart controllers that can automate work across clouds.

<h4 align="center"><img src="media/arch.png" alt="Crossplane" height="400"></h4>

Crossplane presents a declarative management style API that covers a wide range of portable abstractions including databases, message queues, buckets, data pipelines, serverless, clusters, and many more coming. It’s based on the declarative resource model of the popular [Kubernetes](https://github.com/kubernetes/kubernetes) project, and applies many of the lessons learned in container orchestration to multicloud workload and resource orchestration.

Crossplane supports a clean separation of concerns between developers and administrators. Developers define workloads without having to worry about implementation details, environment constraints, and policies. Administrators can define environment specifics, and policies. The separation of concern leads to a higher degree of reusability and reduces complexity.

Crossplane includes a workload scheduler that can factor a number of criteria including capabilities, availability, reliability, cost, regions, and performance while deploying workloads and their resources. The scheduler works alongside specialized resource controllers to ensure policies set by administrators are honored.

For a deeper dive into Crossplane, see the [architecture](https://docs.google.com/document/d/1whncqdUeU2cATGEJhHvzXWC9xdK29Er45NJeoemxebo/edit?usp=sharing) document.

## Table of Contents

* [Quick Start Guide](quick-start.md)
* [Getting Started](getting-started.md)
  * [Installing Crossplane](install-crossplane.md)
  * [Adding Your Cloud Providers](cloud-providers.md)
  * [Deploying Workloads](deploy.md)
  * [Running Resources](running-resources.md)
  * [Troubleshooting](troubleshoot.md)
* [Concepts](concepts.md)
* [FAQs](faqs.md)
* [Contributing](contributing.md)
