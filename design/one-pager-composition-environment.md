# Patch from Generic Datasources

* Owner: Maximilian Blatt (maximilian.blatt@accenture.com, @mistermx)
* Reviewers: Crossplane Maintainers
* Status: Accepted

> **NOTE:** this document discussed the alpha version of this feature. The goals
> still apply, but for an updated discussion of its issues and the plan forward
> see [the beta one-pager](./one-pager-composition-environment-beta.md).

## Background

Crossplane currently supports patching from composite resources to composed
resources and vice versa. It is also possible to copy values between two
composed resources inside one composition by patching the value from one
composed back to the composite and then to the second composed resource.

However, Crossplane currently does not provide a way to patch from
environment-dependent data sources. 
Compositions behave the same regardless in which environment they are deployed.

This becomes a big issue once you want to use the same composition in
multiple environments. For example deploying AWS resources in two different 
accounts requires different subnet IDs, VPC Ids, OIDC issuers etc.

## Current workarounds

### Build one composition per environment

The simplest solution is to build one composition per environment.
For example by using a helm chart that renders differently for each `values.yaml`.
While this indeed works, it does not scale well with the number of environments
and becomes increasingly more complex.

### Use generic provider referencers

While compositions itself are not environment-aware, some providers are by
pulling data from generic resources like secrets. For example:

* `provider-helm`: using [`spec.forProvider.valuesFrom.secretKeyRef`](https://doc.crds.dev/github.com/crossplane-contrib/provider-helm/helm.crossplane.io/Release/v1beta1@v0.10.0#spec-forProvider-valuesFrom-secretKeyRef)
* `provider-kubernetes`: using [`spec.forProvider.references[].patchesFrom`](https://doc.crds.dev/github.com/crossplane-contrib/provider-kubernetes/kubernetes.crossplane.io/Object/v1alpha1@v0.3.0#spec-references-patchesFrom)

However, none of these methods is native to Crossplane native and they are not
available for every provider. Furthermore, they would require an additional
managed resource (here `Object` or `Release`) to deploy the composed resource
making the composition much more complex.

## Goals

The goal of this document is to define an API design for extracting environment
specific data considering the following requirements:

1. Allow the creation of generic compositions that render composed resources
based on the environment they are executed in.
2. Allows users to define this environment in Crossplane native way.
3. Prevent escalation of privileges by disallowing users to read out fields
from objects they shouldn't have access to.

## Proposed Solution

Introduce a new Crossplane resource `EnvironmentConfig` that can store any 
kind of  data similar to a K8s `ConfigMap` but supports complex data as well
using the [`JSON` struct](https://pkg.go.dev/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1#JSON)
which can store anything that serializes to valid JSON.

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: EnvironmentConfig
metadata:
  name: example-environment
data:
  simple: value
  int: 123
  bool: false
  complex:
    a: b
    c:
      d: e
  list:
    - a
    - b
    - c
```

`EnvironmentConfig`s are referenced in the composition spec. They can be
referenced directly as names or through labels in `spec.environment.environmentConfigs`.

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example
spec:
  # This feature will be introduced as alpha (behind a flag). Composition is a
  # v1 API so we'll need to add a note that the environment block is alpha and
  # may change without notice.
  environment:
    environmentConfigs:
    # The list of the EnvironmentConfig selectors used to generate
    # spec.environmentConfigRefs in the XR.
    # These selectors are only executed if spec.environmentConfigRefs is null,
    # similar to referencers in providers.
    # 
    # During reconcile, the data of the XR's EnvironmentConfigs is merged into
    # a single 'computed' environment object. This computed environment
    # is what all environment patches operate upon. For debugging, the computed
    # environment written to the controller logs. Note: It is possible to expand
    # this later and store the computed environment somewhere else, i.e. in the
    # XR's status.
    - type: Reference
      reference:
        name: example-environment
    - type: Reference
      reference:
        name: other-environment
    - type: Selector
      selector: 
      # Select one EnvironmentConfig with matching "stage" AND "my-label" labels.
      - matchLabels:
        # Matches an EnvironmentConfig whose "stage" label matches the value read
        # from the Composite's field path.
        - type: FromCompositeFieldPath
          key: stage
          valueFromFieldPath: spec.parameters.stage
        # Matches an EnvironmentConfig whose "my-label" label matches the supplied
        # static value.
        - type: Value
          key: my-label
          value: metadata.labels[my-label]
   # This is where we specify patches "between" the XR and (computed) environment
    patches:
    # A FromCompositeFieldPath patches from XR -> computed environment.
    - type: FromCompositeFieldPath
      fromFieldPath: spec.widgets
      toFieldPath: widgets.count
    # A FromEnvironmentFieldPath patches from computed environment -> XR.
    - type: FromEnvironmentFieldPath
      fromFieldPath: spec.widgets
      toFieldPath: widgets.count
```

All found `EnvironmentConfig`s are merged together using strategic merging
in the order they are listed. Similar to Helm value files.

The selected environment configs refs are stored in the XR under
`spec.resourceRefs`:

```yaml
apiVersion: demo.org/v1alpha1
kind: XExample
spec:
  environmentConfigRefs:
    - name: example-environment
    - name: other-environment
    - name: label-environment
```

Similar to referencers in providers, `environmentConfigRefs` is only going to be
updated if it is null. Otherwise the list is going to be reused on consecutive
reconciles.

Composed resourced can be patched using the new `FromEnvironmentFieldPath` and
`CombineFromEnvironment` patch types:

```yaml
        - type: FromEnvironmentFieldPath
          fromFieldPath: key
          toFieldPath: spec.forProvider.manifest.data.key
        - type: CombineFromEnvironment
          combine:
            variables:
              - fromFieldPath: key
              - fromFieldPath: key
          toFieldPath: spec.forProvider.manifest.data.key
```

It is also possible to patch the (in-memory) environment itself using 
`ToEnvironmentFieldPath` and `CombineToEnvironment` patches:

```yaml
        - type: ToEnvironmentFieldPath
          fromFieldPath: metadata.name
          toFieldPath: tmp.name
        - type: CombineToEnvironment
          combine:
            variables:
              - fromFieldPath: metadata.namespace
              - fromFieldPath: metadata.name
          toFieldPath: tmp.namespacedName
```

The described feature will be hidden behind a flag
(--enable-environment-configs) and disabled by default while it is in alpha
phase.

**Advantages:**

* Centralized solution that can be used with any provider.
* API extensions are purely additive. No breaking changes.
* No security risks through the usage of a new CRD. Composition writers cannot
use this solution to extract data from resources they shouldn't have access to
- except `EnvironmentConfig`, of course.
* `EnvironmentConfig` is more powerful than standard `ConfigMap` since they
can store complex data types instead of just `map[string]string`. This allows
patching whole objects or arrays during compose.

**Drawbacks:**

* Further extending the patching API making it more complex.

## Alternatives considered

Some other solutions to patch from generic data sources:

### Patch from any object

Introduce a new `fromObjectFieldPath` patch type that can extract values from
any object Crossplane has access to:

```yaml
      patches:
        - type: FromObjectFieldPath
          fromObjectRef:
            apiVersion: v1
            kind: ConfigMap
            name: sample-config
            namespace: sample-ns
          fromFieldPath: data.value
          toFieldPath: spec.forProvider.sampleField
          policy:
            fromFieldPath: Required # Dont render if referenced resource does not exist
```

See https://github.com/crossplane/crossplane/pull/2938 for more details.

**Drawbacks:**

* Further extends the patching API making it more complex.
* Possible security issue by allowing escalation of privileges:
A user without cluster admin access can create a composition to read out any value
from an object they do not have access to.

### Referencers on managed resource (MR) level

Generic resource referencers could be implemented on MR level. Here every
provider is responsible for implementing and supporting this feature.

See https://github.com/crossplane/crossplane/issues/1770 for details.

However, the security issues mentioned in [Patch from any Object](#patch-from-any-object)
would occur here as well.
One could potentially use a managed resource to extract data from a secret
within another namespace.

Additionally, this solution would require every provider to implement this
separately. It would therefore exclude every provider version that does not
implement this feature.

### Custom compositions

Custom compositions are a proposed way of generating compositions on-the-fly
using XRM functions which are similar to KRM function.

See https://github.com/crossplane/crossplane/pull/2886/files for details.

XRM functions are definitely a very powerful way to generate or modify
compositions and might become the standard way of using compositions in the
future.

However, they also come with a huge increase in complexity compared to plain
YAML which might be too high if you just want to patch some environment specific
fields. For this use case, Crossplane should provide a more simpler method
out-of-the-box. It would also allow teams to stick with their existing YAML
compositions without having to migrate to XRM functions and a (potentially
API breaking) composition v2.
