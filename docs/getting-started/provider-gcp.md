---
title: GCP Quickstart
weight: 4
---

Connect Crossplane to Google GCP to create and manage cloud resources from Kubernetes with the [Upbound GCP Provider](https://marketplace.upbound.io/providers/upbound/provider-gcp/).

This guide walks you through the steps required to get started with the Upbound GCP Provider. This includes installing Crossplane, configuring the provider to authenticate to GCP and creating a _Managed Resource_ in GCP directly from your Kubernetes cluster.

- [Prerequisites](#prerequisites)
- [Install the GCP provider](#install-the-gcp-provider)
- [Create a Kubernetes secret for GCP](#create-a-kubernetes-secret-for-gcp)
  - [Generate a GCP service account JSON file](#generate-a-gcp-service-account-json-file)
  - [Create a Kubernetes secret with the GCP credentials](#create-a-kubernetes-secret-with-the-gcp-credentials)
- [Create a ProviderConfig](#create-a-providerconfig)
- [Create a managed resource](#create-a-managed-resource)
- [Delete the managed resource](#delete-the-managed-resource)
- [Next steps](#next-steps)

## Prerequisites
This quickstart requires:
* a Kubernetes cluster with at least 3 GB of RAM
* permissions to create pods and secrets in the Kubernetes cluster
* [Helm] version `v3.2.0` or later
* a GCP account with permissions to create a storage bucket
* GCP [account keys](https://cloud.google.com/iam/docs/creating-managing-service-account-keys)
* GCP [Project ID](https://support.google.com/googleapi/answer/7014113?hl=en)

{{< hint type="tip" >}}
If you don't have a Kubernetes cluster create one locally with [minikube](https://minikube.sigs.k8s.io/docs/start/) or [kind](https://kind.sigs.k8s.io/).
{{< /hint >}}


{{< hint type="note" >}}
All commands use the current `kubeconfig` context and configuration. 
{{< /hint >}}


{{< include file="install-crossplane.md" type="page" >}}

## Install the GCP provider

Install the provider into the Kubernetes cluster with a Kubernetes configuration file. 

```shell {label="provider",copy-lines="all"}
cat <<EOF | kubectl apply -f -
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: upbound-provider-gcp
spec:
  package: xpkg.upbound.io/upbound/provider-gcp:v0.15.0
EOF
```

The {{< hover label="provider" line="3">}}kind: Provider{{< /hover >}} uses the Crossplane `Provider` _Custom Resource Definition_ to connect your Kubernetes cluster to your cloud provider.  

Verify the provider installed with `kubectl get providers`. 

{{< hint type="note" >}}
It may take up to five minutes for the provider to list `HEALTHY` as `True`. 
{{< /hint >}}

```shell 
kubectl get providers
NAME                   INSTALLED   HEALTHY   PACKAGE                                        AGE
upbound-provider-gcp   True        False     xpkg.upbound.io/upbound/provider-gcp:v0.15.0   8s
```

A provider installs their own Kubernetes _Custom Resource Definitions_ (CRDs). These CRDs allow you to create GCP resources directly inside Kubernetes.

You can view the new CRDs with `kubectl get crds`. Every CRD maps to a unique GCP service Crossplane can provision and manage.

{{< hint type="tip" >}}
All the supported CRDs are also available in the [Upbound Marketplace](https://marketplace.upbound.io/providers/upbound/provider-gcp/latest/crds).
{{< /hint >}}

## Create a Kubernetes secret for GCP
The provider requires credentials to create and manage GCP resources. Providers use a Kubernetes _Secret_ to connect the credentials to the provider.

First generate a Kubernetes _Secret_ from a Google Cloud service account JSON file and then configure the Provider to use it.

{{< hint type="note" >}}
Other authentication methods exist and are beyond the scope of this guide. The [Provider documentation](https://marketplace.upbound.io/providers/upbound/provider-gcp/latest/docs/configuration) contains information on alternative authentication methods. 
{{< /hint >}}

### Generate a GCP service account JSON file
For basic user authentication, use a Google Cloud service account JSON file. 

{{< hint type="tip" >}}
The [GCP documentation](https://cloud.google.com/iam/docs/creating-managing-service-account-keys) provides information on how to generate a service account JSON file.
{{< /hint >}}

Save this JSON file as `gcp-credentials.json`

{{< hint type="note" >}}
The [Configuration](https://marketplace.upbound.io/providers/upbound/provider-gcp/latest/docs/configuration) section of the Provider documentation describes other authentication methods.
{{< /hint >}}

### Create a Kubernetes secret with the GCP credentials
<!-- vale gitlab.Substitutions = NO -->
<!-- ignore .json file name -->
A Kubernetes generic secret has a name and contents. Use {{< hover label="kube-create-secret" line="1">}}kubectl create secret{{< /hover >}} to generate the secret object named {{< hover label="kube-create-secret" line="2">}}gcp-secret{{< /hover >}} in the {{< hover label="kube-create-secret" line="3">}}crossplane-system{{</ hover >}} namespace.  
Use the {{< hover label="kube-create-secret" line="4">}}--from-file={{</hover>}} argument to set the value to the contents of the  {{< hover label="kube-create-secret" line="4">}}gcp-credentials.json{{< /hover >}} file.
<!-- vale gitlab.Substitutions = YES -->

```shell {label="kube-create-secret",copy-lines="all"}
kubectl create secret \
generic gcp-secret \
-n crossplane-system \
--from-file=creds=./gcp-credentials.json
```

View the secret with `kubectl describe secret`

{{< hint type="note" >}}
The size may be larger if there are extra blank spaces in your text file.
{{< /hint >}}

```shell
kubectl describe secret gcp-secret -n crossplane-system
Name:         gcp-secret
Namespace:    crossplane-system
Labels:       <none>
Annotations:  <none>

Type:  Opaque

Data
====
creds:  2330 bytes
```

## Create a ProviderConfig
A `ProviderConfig` customizes the settings of the GCP Provider.  

Apply the {{< hover label="providerconfig" line="2">}}ProviderConfig{{</ hover >}}. Include your {{< hover label="providerconfig" line="7" >}}GCP project ID{{< /hover >}}.

{{< hint type="warning" >}}
Add your GCP `project ID` into the output below. 
{{< /hint >}}

```yaml {label="providerconfig",copy-lines="all"}
cat <<EOF | kubectl apply -f -
apiVersion: gcp.upbound.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  projectID: <PROJECT_ID>
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: gcp-secret
      key: creds
EOF
```

This attaches the GCP credentials, saved as a Kubernetes secret, as a {{< hover label="providerconfig" line="9">}}secretRef{{</ hover>}}.

The {{< hover label="providerconfig" line="12">}}spec.credentials.secretRef.name{{< /hover >}} value is the name of the Kubernetes secret containing the GCP credentials in the {{< hover label="providerconfig" line="11">}}spec.credentials.secretRef.namespace{{< /hover >}}.


## Create a managed resource
A _managed resource_ is anything Crossplane creates and manages outside of the Kubernetes cluster. This creates a GCP storage bucket with Crossplane. The storage bucket is a _managed resource_.

This generates a random name for the storage bucket starting with {{< hover label="xr" line="1" >}}crossplane-bucket{{< /hover >}}

```yaml {label="xr",copy-lines="all"}
bucket=$(echo "crossplane-bucket-"$(head -n 4096 /dev/urandom | openssl sha1 | tail -c 10))
cat <<EOF | kubectl apply -f -
apiVersion: storage.gcp.upbound.io/v1beta1
kind: Bucket
metadata:
  name: $bucket
spec:
  forProvider:
    location: US
    storageClass: MULTI_REGIONAL
  providerConfigRef:
    name: default
  deletionPolicy: Delete
EOF
```

Notice the {{< hover label="xr" line="3">}}apiVersion{{< /hover >}} and {{< hover label="xr" line="4">}}kind{{</hover >}} are from the `Provider's` CRDs.


The {{< hover label="xr" line="6">}}metadata.name{{< /hover >}} value is the name of the created GCP storage bucket.  
This example uses the generated name `crossplane-bucket-<hash>` in the {{< hover label="xr" line="6">}}$bucket{{</hover >}} variable.

{{< hover label="xr" line="10" >}}spec.storageClass{{< /hover >}} defines the GCP storage bucket is [single-region, dual-region or multi-region](https://cloud.google.com/storage/docs/locations#key-concepts). 

{{< hover label="xr" line="9">}}spec.forProvider.location{{< /hover >}} is a [GCP location based](https://cloud.google.com/storage/docs/locations) on the {{< hover label="xr" line="10" >}}storageClass{{< /hover >}}. 

Use `kubectl get buckets` to verify Crossplane created the bucket.

{{< hint type="tip" >}}
Crossplane created the bucket when the values `READY` and `SYNCED` are `True`.  
This may take up to 5 minutes.  
{{< /hint >}}

```shell
kubectl get bucket
NAME                          READY   SYNCED   EXTERNAL-NAME                 AGE
crossplane-bucket-cf2b6d853   True    True     crossplane-bucket-cf2b6d853   3m3s
```

Optionally, log into the [GCP Console](https://console.cloud.google.com/) and see the storage bucket inside GCP.

## Delete the managed resource
Before shutting down your Kubernetes cluster, delete the S3 bucket just created.

Use `kubectl delete bucket <bucketname>` to remove the bucket.

```shell
kubectl delete bucket $bucket
bucket.storage.gcp.upbound.io "crossplane-bucket-b7cf6b590" deleted
```

Look in the [GCP Console](https://console.cloud.google.com/) to confirm Crossplane deleted the bucket from GCP.


## Next steps 
* Explore GCP resources that can Crossplane can configure in the [Provider CRD reference](https://marketplace.upbound.io/providers/upbound/provider-gcp/latest/crds).
* Join the [Crossplane Slack](https://slack.crossplane.io/) and connect with Crossplane users and contributors.