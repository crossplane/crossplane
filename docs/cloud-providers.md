---
title: Adding Your Cloud Providers
toc: true
weight: 230
indent: true
---

# Adding Your Cloud Providers

In order for Crossplane to be able to manage resources across all your clouds, you will need to add your cloud provider credentials to Crossplane.
Use the links below for specific instructions to add each of the following cloud providers:

* [Google Cloud Platform (GCP)](cloud-providers/gcp/gcp-provider.md)
    * Required for Quick Start
* [Microsoft Azure](cloud-providers/azure/azure-provider.md)
* [Amazon Web Services (AWS)](cloud-providers/aws/aws-provider.md)

## Examining Cloud Provider Configuration

When Crossplane is installed, you can list all of the available providers.

```console
$ kubectl api-resources  | grep providers.*crossplane | awk '{print $2}'
aws.crossplane.io
azure.crossplane.io
gcp.crossplane.io
```

After credentials have put in place for some of the Cloud providers, you can list those configurations.

```console
$ kubectl -n crossplane-system get providers.gcp.crossplane.io
NAME           PROJECT-ID                 AGE
gcp-provider   crossplane-example-10412   22h

$ kubectl -n crossplane-system get providers.aws.crossplane.io
NAME           REGION      AGE
aws-provider   eu-west-1   22h

$ kubectl -n crossplane-system get providers.azure.crossplane.io
NAME           AGE
azure-provider 22h
```
