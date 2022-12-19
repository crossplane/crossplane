---
title: Basic Compositions Guide
toc: true
weight: 1
---

In this tutorial series, you will build an API for provisioning pre-configured VMs on top of the GCP Compute Engine service. It will walk you through reasoning about how to define the API through implementation. This tutorial does not spend time explaining core Crossplane concepts (visit [Concepts]({{<ref "master/concepts/_index.md" >}}) to learn more). 

## Prerequisites

Before you begin, you’ll need the following:

* A Kubernetes cluster, such as kind.
* Crossplane is installed on your Kubernetes cluster.
* Your preferred code editor, such as vscode.

## Getting Started 

It is a good practice to game plan the shape of your API based on inbound requirements. For the purposes of this tutorial, the following requirements are given:

* The target cloud service for abstraction is GCP.
* The goal is to deliver a platform that other teams can use to create VM instances which have already been configured to comply with company policy. 
* Other app teams don’t want to hassle with knowing which properties to set or what sizes are outside of company policy–they just need a VM to host their app. We’ll only ask the app team to specify whether they want ‘small’, ‘medium’, or ‘large’-sized VMs. They will have profiled their app’s performance and can pick a size accordingly. They also need the option of selecting a region: us-east or us-west. If they don’t specify the region, it defaults to us-west.

From these requirements, you know you are going to be building an API abstraction on top of GCP’s Compute Engine service. 

## Find the Crossplane Provider

The first objective is to check whether there is a Crossplane provider for the cloud service you intend to build on. [Upbound Marketplace](https://marketplace.upbound.io/) is the place to check.

{{< img src="../media/basic-compositions/marketplace.png" alt="Upbound Marketplace" size="large" >}}

While provider-gcp is listed on the frontpage, search the marketplace for “Google” and it should return you two results. Select the result that has the Upbound “official” label, which means this is an Upbound-backed implementation of the GCP Crossplane provider. This one should be chosen because it has the API types required for this tutorial.

{{< img src="../media/basic-compositions/marketplace-search.png" alt="Upbound Marketplace search results" size="large" >}}

> Tip: Generally speaking, If you cannot find the provider you are looking for in the Upbound marketplace, the next step you must evaluate is whether to build the provider yourself (or consult a vendor to build it for you). The Upbound Marketplace already has popular providers (like AWS or Azure) and as the community continues building Crossplane providers, this situation should become much less common. Building a new Provider is outside the scope of this tutorial.

## Confirm base APIs are available

Now that you’ve found the right provider, next you should confirm it contains the API types (CRDs) that you need. Since the objective is to build a platform on Compute Engine VMs, use the search bar in the CRDs pane to look for `Instance`. Note: you may have to try a couple search terms–the marketplace searches based on a few fields exposed by the CRD. For example, if you search for `VM`, you will get no results, but `Instance` gets accurate results. Click the CRD that has an `Instance` kind under the `compute.gcp.upbound.io` API group.

{{< img src="../media/basic-compositions/marketplace-crds.png" alt="provider-gcp CRD search results" size="large" >}}

You can read the description provided by the CRD and see that, “Instance is the Schema for the Instances API. Manages a VM instance resource within GCE.” We’ve confirmed the correct CRD exists in the provider! 

## Understand required fields

Next, you should familiarize yourself with the API documentation of the CRD, especially the `spec` field. Make a note of the required objects under the `forProvider` field–as you build your API abstraction, these are properties you must be sure to set. Scanning the fields, you can see that `boot disk`, `machineType`, `networkInterface`, and `zone` are required.

{{< img src="../media/basic-compositions/crd-required.png" alt="provider-gcp CRD required fields" size="large" >}}

If the required fields are an `object` type, you can recursively expand the object’s fields and see what are the required fields for that nested object. 

It’s at this point you should be considering:

* Which fields should I expose via my API to my users?
* Which fields should I set privately?

One last thing to look at during this planning phase is the “Examples” tab, which gives working samples of .yaml that could be passed to created instances of this CRD as a Managed Resource (which is not quite the goal of this tutorial–which is to build a Composition–but it gives you a good baseline to start from). 

{{< img src="../media/basic-compositions/crd-examples.png" alt="provider-gcp CRD examples" size="large" >}}

## Next Steps

You should be beginning to form a sense for what your API will look like. The next steps are to build the API--both the XRD and Composition. Continue reading in [Build XRD]({{<ref "build-xrd.md" >}}).
