---
tocHidden: true
---

# Test & Deploy your Composition

This is the fourth part of a four part tutorial to build an API for provisioning pre-configured VMs on top of the GCP Compute Engine service. This part depends on having already completed building the [XRD]({{<ref "build-xrd.md" >}}) and [Composition]({{<ref "build-composition.md" >}}). In this part of the tutorial, you will package, deploy and test your API.

## Build the Configuration

The final step is to create a Configuration to bundle the API, push it to an OCI registry, and then you can deploy it to your Crossplane cluster. To create the Configuration, you need to define a `crossplane.yaml` at the root of your platform folder (in the `compute-platform` folder if you are following along from the beginning).

In the newly created `crossplane.yaml` file, copy in the following:

```yaml
apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: configuration-computeplatform
  annotations:
    provider: gcp
spec:
  crossplane:
    version: ">=v1.10.1-0"
  dependsOn:
    - provider: xpkg.upbound.io/upbound/provider-gcp
      version: ">=v0.21.0"
```

Now to explain what you just wrote:

* You gave your Configuration a name.
* You declared what version of Crossplane is provided to run your Configuration (in cases where your Configuration uses new capabilities released in a certain version of Crossplane). For convenience, you just declared the latest release version of Crossplane.
* You declared provider-gcp as a dependency, since your API is an abstraction sitting on top of a Managed Resource in provider-gcp. The API reference you looked at in the Upbound Marketplace (as of the time of this writing) was v0.21.

Run the following command in the root of your folder structure to build the Configuration:

```bash
kubectl crossplane build configuration
```

Now push that package to an OCI registry of your choice (it can be Docker Hub or elsewhere):

```bash
REG=my-package-repo
kubectl crossplane push configuration ${REG}/configuration-computeplatform:v0.0.1
```

## Deploy your API

With the Configuration successfully pushed, you are now ready to deploy it to a Crossplane cluster and test your API. As mentioned in the prerequisites, you should have Crossplane already installed into a Kubernetes cluster. Install your Configuration on your control plane:

```bash
kubectl crossplane install configuration ${REG}/configuration-computeplatform:v0.0.1
```

Confirm the package installed by confirming it’s `healthy` status shows `true`. You can also see it pulled in the provider-gcp dependency automatically by confirming its existence:

```bash
kubectl get configurations
kubectl get providers
```

>Important: The last thing to do is configure provider-gcp with credentials. Those instructions can be found in the [Getting Started]({{<ref "master/getting-started/install-configure.md" >}}) guide

## Test your API

You are now ready to test your API by submitting a claim to your control plane and observing whether the claim becomes healthy & the resource shows up in the GCP console. This is the claim you will submit:

```bash
echo "apiVersion: compute.acme.co/v1alpha1
kind: VMInstance
metadata:
  name: my-vm
  namespace: default
spec:
  parameters:
    region: west
    size: small" | kubectl apply -f -
```

Query for the status of your resource by running the following:

```bash
kubectl get VMInstances
```

Likewise, you can see the Managed Resources created under the covers with:

```bash
kubectl get managed
```

{{< img src="../media/basic-compositions/deployment-proof.png" alt="Demonstrating deployed MRs" size="large" >}}

Finally, visit the GCP console and notice the VM has been created along with it's required network and subnetwork objects.

{{< img src="../media/basic-compositions/gcp-deployment-proof.png" alt="Demonstrating API in GCP" size="large" >}}

If you want to clean up the resources, all you need to do is delete the claim and it will clean up the underlying Managed Resources:

```bash
kubectl delete VMInstance my-vm
```

## Conclusion

In this tutorial, you built an API step-by-step. Starting with using Upbound Marketplace to understand how to define the API, you learned what Managed Resources must be created, and authored the XRD and Composition config files accordingly. Finally, you deployed and validated that your API behaves as expected.
