---
title: Build your Composition
tocHidden: true 
weight: 260
indent: true
---

# Build your Composition

This is the third part of a four part tutorial to build an API for provisioning pre-configured VMs on top of the GCP Compute Engine service. If you have not already, you need to first [Build your XRD](build-xrd.md) first. In this part of the tutorial, you will build the Composition portion of your API.

## Getting Started 

The previous part of the tutorial had you build the first half of your API, the XRD, which defines the shape of your API. Now you are going to build the Composition, which is an implementation of the definition. Open the `composition.yaml` file created earlier. 

Similar to the XRD, it is useful to bootstrap the file by copying in from a “known good” `composition.yaml` and pruning it back to the basics. Using the same repo as before, copy the PostgreSQL instance composition and repurpose it for your needs, found [here](https://raw.githubusercontent.com/upbound/platform-ref-gcp/main/package/database/postgres/composition.yaml), and paste it into your `composition.yaml` config. You could also go to the marketplace listing looked at earlier, scroll down and select the “XRDs” tab, and select the Composition titled [xpostgresqlinstances.gcp.platformref.upbound.io](https://marketplace.upbound.io/configurations/upbound/platform-ref-gcp/v0.3.0/compositions/xpostgresqlinstances.gcp.platformref.upbound.io/gcp.platformref.upbound.io/XPostgreSQLInstance), and copy the `.yaml` from there.

![Composition yaml](/docs/media/basic-compositions/composition-yaml.png)

Next, prune out all the fields that are irrelevant to your use case and you will end up with this:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
 name: <todo>
 labels:
   provider: <todo>
spec:
 writeConnectionSecretsToNamespace: crossplane-system
 compositeTypeRef:
   apiVersion: <todo>
   kind: <todo>
 resources:
   - name: <todo>
     base:
       apiVersion: <todo>
       kind: <todo>
       spec:
         forProvider:
     patches:
```

Next, you will fill in the fields above `resources`:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
 name: xvminstances.compute.acme.co
 labels:
   provider: gcp
spec:
 writeConnectionSecretsToNamespace: upbound-system
 compositeTypeRef:
   apiVersion: compute.acme.co/v1alpha1
   kind: XVMInstance
...
```

Now to explain what you just wrote:

* Under `metadata`, the name convention is to use the composite resource name (which we defined in the XRD).
* It’s also convention to attach a label of whichever provider this API is built on top of (yours is GCP). This label can be used in `compositionSelector` in an XR or Claim.
* Each Composition must declare that it is compatible with a particular type of Composite Resource using its `compositeTypeRef` field. The referenced version must be marked `referenceable` in the XRD that defines the XR.

## Composing Managed Resources

This brings you to the main section of the composition, which is where you define the resources that will get composed by the composition and how the fields map (i.e. patch) between the composition and its composed resources. Resources are an array, so you can have anywhere from one to many resources that get composed together. 

Recall earlier how you looked at the Managed Resource for a GCP Compute Engine Instance and made note of the required fields? Remember, `boot disk`, `machineType`, `networkInterface`, and `zone` are required. We need to define each one.

### Compose the Compute Engine Instance

The first Managed Resource that needs to be composed is the `Instance` for Compute Engine. Add the following to your `composition.yaml` under `resources`.

```yaml
...
 resources:
    - name: vm-instance
      base:
        apiVersion: compute.gcp.upbound.io/v1beta1
        kind: Instance
        spec:
          forProvider:
            bootDisk:
              - initializeParams:
                  - image: debian-cloud/debian-11
            machineType: e2-medium
            networkInterface:
              - networkIp: 10.2.0.21
                networkSelector:
                  matchLabels:
                    network-name: network-endpoint
                subnetworkSelector:
                  matchLabels:
                    subnet-name: network-endpoint
            zone: us-west1-a
```

Now to explain what you just wrote:

* You gave this composed resource a name.
* The apiVersion and kind of the resource comes straight from what is shown in the Upbound marketplace for that CRD.
* The ‘spec.forProvider’ section is where you pass values for the required fields of the Managed Resource. This is also where you could set additional optional fields exposed by the Managed Resource.

If you stopped here, you would have an incomplete API. Right now the resource is getting composed without any input from the user for the size of the VM or the region it should run in. We address this with patches. Write the patches as follows:

```yaml
...
     patches:
       - fromFieldPath: "spec.parameters.region"
         toFieldPath: "spec.forProvider.zone"
         transforms:
         - type: map
           map:
             east: us-east1-b
             west: us-west1-a
       - fromFieldPath: "spec.parameters.size"
         toFieldPath: "spec.forProvider.machineType"
         transforms:
         - type: map
           map:
             small: e2-standard-2
             medium: e2-standard-4
             large: e2-standard-8
```

Now to explain what you just wrote:

* You have two patches, one for each field exposed in the XRD. 
* The fields exposed to the end user are string enums, and the values do not directly map to the values needed by the Managed Resource, so we set up a transform to mutate the value inputted by users to the value required by the Managed Resource. Because this is a Compute Engine instance, you can find the values GCP uses for [region](https://cloud.google.com/compute/docs/regions-zones) and [machineType](https://cloud.google.com/compute/docs/general-purpose-machines#e2_machine_types) respectively.

### Compose the Network objects

You will notice under the `networkInterface` object, there are two "selector" fields: `networkSelector` and `subnetworkSelector`. When GCP creates a new Compute Engine instance, it also creates network obejcts for each instance. Therefore, you must do the same in Crossplane. These objects, `Network` and `Subnetwork` are _also_ Managed Resources. Crossplane is using label matching to establish a relationship between these MRs--the `Instance` to each networking MR.

Therefore, you need two add two additional composed Managed Resources to this Composition: a `Network` MR and a `Subnetwork` MR. Just as you did for the `Instance` MR, you can look these up on the Marketplace to find which fields are required when creating these objects, look at examples, etc. Let's first add the `Network` as another object under the `resources` array:

```yaml
...
    - name: network-instance
      base:
        apiVersion: compute.gcp.upbound.io/v1beta1
        kind: Network
        metadata:
          labels:
            network-name: network-endpoint
          name: network-endpoint
        spec:
          forProvider:
            autoCreateSubnetworks: false
```

And now let's add the `Subnetwork` as another object, also under the `resources` array:

```yaml
...
    - name: subnet-instance
      base:
        apiVersion: compute.gcp.upbound.io/v1beta1
        kind: Subnetwork
        metadata:
          labels:
            subnet-name: network-endpoint
          name: network-endpoint
        spec:
          forProvider:
            ipCidrRange: 10.2.0.0/16
            networkSelector:
              matchLabels:
                network-name: network-endpoint
            region: us-west1
      patches:
        - fromFieldPath: "spec.parameters.region"
          toFieldPath: "spec.forProvider.region"
          transforms:
          - type: map
            map:
              east: us-east1
              west: us-west1
```

Now to explain what you just wrote:

* You defined two additional MRs as part of this Composition. In total, this Composition composes 3 Managed Resources: an `Instance`, a `Network`, and a `Subnetwork`.
* You created labels for the network objects and used those labels to establish a relationship with the `Instance` object so it can find the network objects that it requires in order to provision successfully.
* Since the `Subnetwork` requires a region input--and the API you defined has this as a settable parameter--you had to set up a patch to transform the `region` input to a form expected by GCP's `Subnetwork` object.

Save and close the `composition.yaml` file. The complete file should look like this:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: xvminstances.compute.acme.co
  labels:
    provider: gcp
spec:
  writeConnectionSecretsToNamespace: upbound-system
  compositeTypeRef:
    apiVersion: compute.acme.co/v1alpha1
    kind: XVMInstance
  resources:
    - name: vm-instance
      base:
        apiVersion: compute.gcp.upbound.io/v1beta1
        kind: Instance
        spec:
          forProvider:
            bootDisk:
              - initializeParams:
                  - image: debian-cloud/debian-11
            machineType: e2-medium
            networkInterface:
              - networkIp: 10.2.0.21
                networkSelector:
                  matchLabels:
                    network-name: network-endpoint
                subnetworkSelector:
                  matchLabels:
                    subnet-name: network-endpoint
            zone: us-west1-a
      patches:
        - fromFieldPath: "spec.parameters.region"
          toFieldPath: "spec.forProvider.zone"
          transforms:
          - type: map
            map:
              east: us-east1-b
              west: us-west1-a
        - fromFieldPath: "spec.parameters.size"
          toFieldPath: "spec.forProvider.machineType"
          transforms:
          - type: map
            map:
              small: e2-standard-2
              medium: e2-standard-4
              large: e2-standard-8
    - name: network-instance
      base:
        apiVersion: compute.gcp.upbound.io/v1beta1
        kind: Network
        metadata:
          labels:
            network-name: network-endpoint
          name: network-endpoint
        spec:
          forProvider:
            autoCreateSubnetworks: false
    - name: subnet-instance
      base:
        apiVersion: compute.gcp.upbound.io/v1beta1
        kind: Subnetwork
        metadata:
          labels:
            subnet-name: network-endpoint
          name: network-endpoint
        spec:
          forProvider:
            ipCidrRange: 10.2.0.0/16
            networkSelector:
              matchLabels:
                network-name: network-endpoint
            region: us-west1
      patches:
        - fromFieldPath: "spec.parameters.region"
          toFieldPath: "spec.forProvider.region"
          transforms:
          - type: map
            map:
              east: us-east1
              west: us-west1
```

## Next Steps

In this part of the tutorial, you completed the second half of your API. You authored the Composition, which implements the definition you created earlier.  The next step is to test the API to see if it behaves the way you want. Continue reading in [Test & Deploy your Composition](test-and-deploy.md). 

