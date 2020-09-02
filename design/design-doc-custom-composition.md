# Custom Composition Types

* Owner: Muvaffak Onuş (@muvaf)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

## Proposal

### Authorship Experience

Let's say we have the following `CompositeResourceDefinition` that defines a `PrivateNetwork`
type.

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: CompositeResourceDefinition
metadata:
  name: privatenetworks.aws.example.org
spec:
  crdSpecTemplate:
    group: aws.example.org
    version: v1alpha1
    names:
      kind: PrivateNetwork
      listKind: PrivateNetworkList
      plural: privatenetworks
      singular: privatenetwork
    validation:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              region:
                type: string
                description: Geographic region in which nodes will exist.
              providerConfigRef:
                type: object
                description: Crossplane AWS provider credentials to use.
                properties:
                  name:
                    type: string
                required:
                - name
            required:
            - region
            - providerConfigRef
```

User can have multiple `Composition` or custom compositions that target this new
type. Let's assume they would like to write a cdk8s app and use it as the composition
engine. They'd need to write the cdk8s app that will take the following resources
as input:
  * `PrivateNetwork` custom resource.
  * `CustomComposition` custom resource.
  It will return all the resulting resources as output.

This conversion function will be wrapped by an HTTP server and pushed as an image
whose `ENTRYPOINT` is to start that HTTP server. The input will come as POST
request and the body of the result will consist of all the output resources.

At this point, we have our cdk8s app that will act as composition engine. Now,
let's go through how the consumers of that app will be able to use it.

### Consumption Experience

Now that our composition engine is ready, we'll instruct Crossplane to install
that composition engine and send the appropriate requests. In order to do that,
the following `CustomComposition` will be created:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: CustomComposition
metadata:
  name: acmenetwork.acme.org
spec:
  writeConnectionSecretsToNamespace: crossplane-system
  reclaimPolicy: Delete
  compositeTypeRef:
    apiVersion: aws.example.org/v1alpha1
    kind: PrivateNetwork
  image: acme/network-cdk8s:1.0.0
```

Once this `CustomComposition` is created, a new `Deployment` will be created by
Crossplane with the following properties:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: acmenetwork.acme.org
  labels:
    app: acmenetwork.acme.org
spec:
  replicas: 1
  selector:
    matchLabels:
      app: acmenetwork.acme.org
  template:
    metadata:
      labels:
        app: acmenetwork.acme.org
    spec:
      containers:
      - name: custom-composition
        image: crossplane/custom-composition:0.0.1
        env:
        # COMPOSITE_TYPE will let the custom-composition controller know what type
        # it should watch.
        - name: COMPOSITE_TYPE
          value: privatenetworks.aws.example.org
        # CUSTOMCOMPOSITION_NAME is the name of the CustomComposition that will
        # be sent over to composition engine alongside composite instance. 
        - name: CUSTOMCOMPOSITION_NAME
          value: acmenetwork.acme.org
      - name: composition-engine
        image: acme/network-cdk8s:1.0.0
        ports:
        - containerPort: 80
```

After this `Deployment` is up, the container named `custom-composition` will start
watching `PrivateNetwork` types and it will reconcile only the ones with the following
reference:
```yaml
apiVersion: acme.example.org
kind: PrivateNetwork
metadata:
  name: something
spec:
  region: us-east-1
  providerConfigRef:
    name: my-creds
  compositionRef:
    apiVersion: apiextensions.crossplane.io/v1alpha1
    kind: CustomComposition
    name: acmenetwork.acme.org
```

In each of its reconciliation pass:
1. It will make an HTTP request to `cdk-app` container
with the `PrivateNetwork` instance and `CustomComposition` instance.
  * The reason we include `CustomComposition` is that the cdk8s app might be configured
    with some additional fields in the future or metadata of `CustomComposition`.
2. The response from the cdk-app container is expected to be a set of valid
    Kubernetes YAMLs.
3. `custom-composition` will add owner references to all objects and run the
   equivalent of `kubectl apply --prune -f <response YAMLs>`.

At this point, we got all of our output deployed to the cluster.

## Alternatives Considered

### Separate Type for Each Custom Composition Type

We could have `CDKComposition`, `HelmComposition` types with their own additional
fields. But since for each instance of those types, a new `Deployment` is created,
the entity that creates this `Deployment` couldn't be Crossplane because it'd not
know of those types. So, for each type, users would have to install a CRD and a
separate controller that will do this `Deployment` creation operation.

We could possibly include those custom composition types in Crossplane with the
assumption that the number of types will not be that big anyway. However, some users
would prefer the flexibility of use of their in-house custom compositions and having
to have it merged to Crossplane would be a big barrier.

Another possibility is simply have each type be almost completely decoupled from
Crossplane by having them install that `Deployment` creator entity, too. For instance,
they could install their CRD and a `Deployment` that has only a controller which creates
a `Deployment` for every instance of that custom composition type. But this approach
expands the attack surface since now they have to give `Deployment` permission to
yet another `ServiceAccount` besides from Crossplane itself. Also, they'd have to
write a controller to have a custom composition, which could be alleviated by
having a generic one since the operation itself is not a complex one, but that's
yet another thing to maintain.
