---
title: Quick Start Guide
toc: true
weight: 210
---
# Quick Start Guide

This quick start will demonstrate using Crossplane to deploy a portable stateful workload in the cloud provider of your choice.
It will first dynamically provision a Kubernetes cluster within the cloud provider environment, followed by a stateful application and its database to the same environment.
The database will also be dynamically provisioned using a managed service hosted by the cloud provider.
The Workload will be deployed into the target Kubernetes cluster, and be configured to consume the database resource in a completely portable way.

The general steps for this example are as follows:

1. Install Crossplane so it is ready to manage resources on your behalf: [Install Crossplane](install-crossplane.md)
1. Set up a cloud provider and add it to Crossplane: [Adding a Cloud Provider](cloud-providers.md)
1. Deploy a portable workload to the cloud provider: [Deploying Workloads](deploy.md)
