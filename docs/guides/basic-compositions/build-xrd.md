---
title: Build your XRD
tocHidden: true 
weight: 260
indent: true
---

# Build your XRD

This is the second part of a four part tutorial to build an API for provisioning pre-configured VMs on top of the GCP Compute Engine service. If you have not already, it is recommended to read [Plan your API](plan-api.md) first. In this part of the tutorial, you will build the XRD portion of your API.

## Getting Started 

The previous part of the tutorial gave you a sense for the Managed Resources you need to compose for your API. It’s time to start authoring the Crossplane Composition (your API definition). Make a folder called `compute-platform`, which will host your entire platform definition. In that folder, make another folder called `VMInstance`, which is where the Composition’s files will live. Finally, create two files in that folder: `composition.yaml` and `definition.yaml`. Your workspace should look like this:

![Composition workspace setup](/docs/media/basic-compositions/composition-workspace-setup.png)

You’ll start by authoring the `definition.yaml` file (which will define your XRD). Open it in your favorite code editor. At this current time, the Crossplane authoring experience unfortunately involves writing some boilerplate text. It is fair practice to bootstrap a composition file by starting from a “known good composition”. 

The Upbound Marketplace is once again useful here: from the Upbound Marketplace, either scroll down on the main page to find `platform-ref-gcp` or use the search bar to search for this Configuration. 

>TIP: If you use the search bar, make sure to change the filter type to “Configurations”.

![Marketplace reference Configurations](/docs/media/basic-compositions/ref-configs.png)

Click into the `platform-ref-gcp` Configuration  and click the link on the next page under “Source Code” to go to the source repo.

![Reference Configuration source](/docs/media/basic-compositions/config-source.png)

Within this GitHub repo, you are going to copy an existing XRD’s definition. You are going to use the PostgreSQL instance XRD and repurpose it for your needs. Copy the contents of that file, found here, and paste it into your `definition.yaml` created earlier.

>TIP: An aside: these Configurations in the Marketplace are reference implementations. In some cases, you may want to build an API for which there is already an implementation (like a Kubernetes cluster or PostgreSQL instance). It is completely fair practice to use these as launching pads, but unfortunately for our VM scenario, it doesn’t exist and so we have to build it from scratch. Nevertheless, we can copy the .yaml and use some of it as boilerplate code.

## Build the XRD

Next, prune out all the fields that are irrelevant for your use case (i.e. they are specific to PostgreSQL) and you will end up with this. This is boilerplate.yaml that nearly all XRDs will want to have:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
 name: <todo>
spec:
 group: <todo>
 names:
   kind: <todo>
   plural: <todo>
 claimNames:
   kind: <todo>
   plural: <todo>
 versions:
 - name: v1alpha1
   served: true
   referenceable: true
   schema:
     openAPIV3Schema:
       type: object
       properties:
         spec:
           type: object
           properties:
             parameters:
               type: object
               properties:
               required:
           required:
             - parameters
```

>TIP: The Composition [reference docs](https://docs.crossplane.io/master/reference/composition/) are an invaluable reference at this time. 

Going by the conventions as described in the Composition reference docs, you will fill in values for each of the `todo`. Fill in the top half of the config as below:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: xvminstances.compute.acme.co
spec:
  group: compute.acme.co
  names:
    kind: XVMInstance
    plural: xvminstances
  claimNames:
    kind: VMInstance
    plural: vminstances
...
```

Now to explain what you just wrote:

* The `group` is the declaration of where this API will live (in the future you could define multiple APIs and have them all be a member of the ‘compute.acme.co’ group).
* The `names` field of the spec is where we define the name of the XRD. Crossplane convention says we prefix the XRD name with an ‘X’.
* The `claimNames` field of the spec is where we define the claim that our XR offers. If you don’t want a publicly callable API, omit having a claimName. You want app teams to issue claims to “call” your VM instance API, so you include it.

Next, you will also define the custom fields of your API. Since you are building a VMInstance XRD, use the following:


```yaml
...
  versions:
  - name: v1alpha1
    served: true
    referenceable: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            description: 'The specification for how this VM instance should be deployed.'
            properties:
              parameters:
                type: object
                description: 'The parameters indicating how this VM instance should be configured.'
                properties:
                  region:
                    type: string
                    enum:
                    - east
                    - west
                    description: 'The geographic region in which this VM instance should be deployed.'
                  size:
                    type: string
                    enum:
                    - small
                    - medium
                    - large
                    description: 'The machine size for this VM instance.'
                required:
                - size
            required:
            - parameters
```

Now to explain what you just wrote:

* Following convention, the parameters of the API get defined in `schema.openAPIV3Schema.properties.spec.properties.parameters.properties`. 
* As mentioned at the beginning of the blog, you are going to offer 2 fields: `size` and `region`. Only `size` is a required input, while `region` is optional. Hence, you only declare `size` as a required field.
* You have defined your custom API fields as `string enums`. To learn more about the range of what you could define here, look at the [Kubernetes docs](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#create-custom-objects) on authoring custom objects.

Save and close the `definition.yaml` file. The complete file should look like this:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: xvminstances.compute.acme.co
spec:
  group: compute.acme.co
  names:
    kind: XVMInstance
    plural: xvminstances
  claimNames:
    kind: VMInstance
    plural: vminstances
  versions:
  - name: v1alpha1
    served: true
    referenceable: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            description: 'The specification for how this VM instance should be deployed.'
            properties:
              parameters:
                type: object
                description: 'The parameters indicating how this VM instance should be configured.'
                properties:
                  region:
                    type: string
                    enum:
                    - east
                    - west
                    description: 'The geographic region in which this VM instance should be deployed.'
                  size:
                    type: string
                    enum:
                    - small
                    - medium
                    - large
                    description: 'The machine size for this VM instance.'
                required:
                - size
            required:
            - parameters
```

You are done creating your XRD!

## Next Steps

In this part of the tutorial, you completed the first half of your API. You authored the XRD, which defines the shape of your API. The next steps are to build the Composition, which is an implementation of the definition. Continue reading in [Build Composition](build-composition.md). 
